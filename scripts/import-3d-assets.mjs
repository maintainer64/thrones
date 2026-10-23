import { readFile, readdir, mkdir, writeFile, stat } from "node:fs/promises";
import { resolve, join } from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { homedir } from "node:os";
import {
  readGLB,
  writeGLB,
  appendView,
  floatAccessor,
  compactGLB,
} from "./lib/gltf.mjs";
import { hermite } from "./lib/unity.mjs";
import { importGameAssets } from "./lib/game-assets.mjs";
const require = createRequire(new URL("../web/package.json", import.meta.url));
const YAML = require("yaml");
const { Matrix4, Vector3, Quaternion, Box3 } = await import(
  new URL("./three.module.js", pathToFileURL(require.resolve("three"))).href
);
const sharp = require("sharp");
const root = resolve(
  process.argv[2] || join(homedir(), "Desktop/Resources.out"),
);
const directories = (await readdir(root))
  .filter((n) => n.startsWith("AssetRipper"))
  .sort()
  .reverse();
async function exists(p) {
  try {
    return (await stat(p)).isDirectory();
  } catch {
    return false;
  }
}
let unity, exported;
for (const name of directories) {
  const directory = join(root, name);
  if (await exists(join(directory, "ExportedProject/Assets")))
    unity ??= join(directory, "ExportedProject/Assets");
  if (await exists(join(directory, "Assets/PrefabHierarchyObject")))
    exported ??= join(directory, "Assets");
}
if (!unity || !exported)
  throw Error(
    "Both Unity project and GLB AssetRipper exports are required. Sources remain read-only.",
  );
const output = fileURLToPath(
  new URL("../local-assets/preview3d/", import.meta.url),
);
await mkdir(output, { recursive: true });
function unityObjects(text) {
  const documents = text.split(/--- !u!\d+ &(\d+)\r?\n/);
  const result = new Map();
  for (let i = 1; i < documents.length; i += 2)
    result.set(
      documents[i],
      YAML.parse(documents[i + 1], { intAsBigInt: true, uniqueKeys: false }),
    );
  return result;
}
function unitySingle(text) {
  return [...unityObjects(text).values()][0];
}
function matrix(node) {
  if (node.matrix) return new Matrix4().fromArray(node.matrix);
  return new Matrix4().compose(
    new Vector3(...(node.translation || [0, 0, 0])),
    new Quaternion(...(node.rotation || [0, 0, 0, 1])),
    new Vector3(...(node.scale || [1, 1, 1])),
  );
}
function worlds(g) {
  const parents = new Map();
  g.nodes.forEach((n, i) =>
    (n.children || []).forEach((c) => parents.set(c, i)),
  );
  const cache = new Map();
  function world(i) {
    if (cache.has(i)) return cache.get(i);
    const m = matrix(g.nodes[i]);
    if (parents.has(i)) m.premultiply(world(parents.get(i)));
    cache.set(i, m);
    return m;
  }
  g.nodes.forEach((_, i) => world(i));
  return cache;
}

// Keep only the original northern/southern map planes and their embedded textures.
const board = await readGLB(join(exported, "SceneHierarchyObject/Playmat.glb"));
const world = worlds(board.json);
const boardRoots = [];
const bounds = new Box3();
for (const [i, node] of board.json.nodes.entries()) {
  if (!/^Map(North|South)$/.test(node.name || "") || node.mesh === undefined)
    continue;
  boardRoots.push(i);
  for (const primitive of board.json.meshes[node.mesh].primitives) {
    const a = board.json.accessors[primitive.attributes.POSITION];
    bounds.union(
      new Box3(new Vector3(...a.min), new Vector3(...a.max)).applyMatrix4(
        world.get(i),
      ),
    );
    const material = board.json.materials[primitive.material];
    material.doubleSided = true;
    material.pbrMetallicRoughness = {
      ...material.pbrMetallicRoughness,
      metallicFactor: 0,
      roughnessFactor: 1,
    };
  }
}
if (boardRoots.length !== 2)
  throw Error("Expected exactly two original map planes");
const center = bounds.getCenter(new Vector3());
const offset = new Matrix4().makeTranslation(
  -center.x,
  -bounds.max.y,
  -center.z,
);
for (const i of boardRoots) {
  const node = board.json.nodes[i];
  node.matrix = offset.clone().multiply(world.get(i)).toArray();
  delete node.rotation;
  delete node.translation;
  delete node.scale;
  delete node.children;
}
const mapImages = new Set();
for (const i of boardRoots)
  for (const primitive of board.json.meshes[board.json.nodes[i].mesh]
    .primitives) {
    const texture =
      board.json.materials[primitive.material].pbrMetallicRoughness
        .baseColorTexture;
    mapImages.add(board.json.textures[texture.index].source);
  }
for (const i of mapImages) {
  const image = board.json.images[i],
    view = board.json.bufferViews[image.bufferView],
    offset = view.byteOffset || 0;
  const bytes = await sharp(
    board.bin.subarray(offset, offset + view.byteLength),
  )
    .jpeg({ quality: 90, mozjpeg: true })
    .toBuffer();
  image.bufferView = appendView(board, bytes);
  image.mimeType = "image/jpeg";
}
const smallBoard = compactGLB(board, boardRoots);
await writeGLB(join(output, "westeros.glb"), smallBoard);

// Restore the original animated mesh, using source bind poses (not the prefab's posed matrices).
const unit = await readGLB(
  join(exported, "PrefabHierarchyObject/P_unit_Footman.glb"),
);
const mesh = await readGLB(join(exported, "Mesh/msh_Footman.glb"));
const objects = unityObjects(
  await readFile(join(unity, "GameObject/P_unit_Footman.prefab"), "utf8"),
);
const body = [...objects.values()]
  .map((o) => o.SkinnedMeshRenderer)
  .find((r) => r && r.m_Bones?.length === 23);
if (!body) throw Error("Expected original 23-bone footman renderer");
const transforms = new Map(
  [...objects]
    .filter(([, o]) => o.Transform)
    .map(([id, o]) => [id, o.Transform]),
);
function boneName(ref) {
  const transform = transforms.get(String(ref.fileID));
  return objects.get(String(transform.m_GameObject.fileID)).GameObject.m_Name;
}
const names = new Map();
unit.json.nodes.forEach((n, i) => {
  if (n.name) names.set(n.name, i);
});
const joints = body.m_Bones.map((ref) => names.get(boneName(ref)));
if (joints.some((i) => i === undefined))
  throw Error("Missing GLB bone corresponding to original renderer");
const rootNode = names.get("Unit"),
  bodyNode = names.get("msh_Footman");
if (rootNode === undefined || bodyNode === undefined)
  throw Error("Missing unit hierarchy");
const viewOffset = unit.json.bufferViews.length,
  accessorOffset = unit.json.accessors.length;
const binaryStart = unit.bin.length + ((4 - (unit.bin.length % 4)) % 4);
unit.bin = Buffer.concat([
  unit.bin,
  Buffer.alloc(binaryStart - unit.bin.length),
  mesh.bin,
]);
for (const view of mesh.json.bufferViews)
  unit.json.bufferViews.push({
    ...view,
    byteOffset: (view.byteOffset || 0) + binaryStart,
  });
for (const a of mesh.json.accessors)
  unit.json.accessors.push({
    ...a,
    bufferView:
      a.bufferView === undefined ? undefined : a.bufferView + viewOffset,
  });
const geometry = structuredClone(mesh.json.meshes[0]);
for (const p of geometry.primitives) {
  for (const k of Object.keys(p.attributes)) p.attributes[k] += accessorOffset;
  if (p.indices !== undefined) p.indices += accessorOffset;
}
const texture = await readFile(
  join(exported, "Texture2D/T_unit_Footman_Stark.png"),
);
const imageIndex = (unit.json.images ??= []).length;
unit.json.images.push({
  bufferView: appendView(unit, texture),
  mimeType: "image/png",
});
const textureIndex = (unit.json.textures ??= []).length;
unit.json.textures.push({ source: imageIndex });
const materialIndex = (unit.json.materials ??= []).length;
unit.json.materials.push({
  name: "Original Stark footman",
  pbrMetallicRoughness: {
    baseColorTexture: { index: textureIndex },
    metallicFactor: 0.404,
    roughnessFactor: 0.552,
  },
});
for (const p of geometry.primitives) p.material = materialIndex;
unit.json.nodes[bodyNode].mesh = unit.json.meshes.length;
unit.json.meshes.push(geometry);
const originalMesh = unitySingle(
  await readFile(join(unity, "Mesh/msh_Footman.asset"), "utf8"),
).Mesh;
const matrices = [];
const signs = [-1, 1, 1, 1];
for (const pose of originalMesh.m_BindPose)
  for (let col = 0; col < 4; col++)
    for (let row = 0; row < 4; row++)
      matrices.push(Number(pose[`e${row}${col}`]) * signs[row] * signs[col]);
if (matrices.length !== joints.length * 16)
  throw Error("Inverse bind matrix count mismatch");
unit.json.nodes[bodyNode].skin = 0;
unit.json.skins = [
  {
    joints,
    skeleton: names.get(boneName(body.m_RootBone)),
    inverseBindMatrices: floatAccessor(unit, matrices, "MAT4", joints.length),
  },
];
const paths = new Map();
function walk(i, path) {
  paths.set(path, i);
  for (const child of unit.json.nodes[i].children || [])
    walk(
      child,
      path
        ? path + "/" + unit.json.nodes[child].name
        : unit.json.nodes[child].name,
    );
}
walk(rootNode, "");
unit.json.animations = [];
const clipStats = [];
for (const [filename, label] of [
  ["A_Unit_Footman_Idle2.anim", "idle"],
  ["A_Unit_Footman_Run.anim", "run"],
]) {
  const clip = unitySingle(
    await readFile(join(unity, "AnimationClip", filename), "utf8"),
  ).AnimationClip;
  const groups = [
    ["m_RotationCurves", "rotation", ["x", "y", "z", "w"]],
    ["m_PositionCurves", "translation", ["x", "y", "z"]],
    ["m_ScaleCurves", "scale", ["x", "y", "z"]],
  ];
  const duration = Number(clip.m_AnimationClipSettings.m_StopTime);
  const frames = Math.ceil(duration * 60);
  const times = Array.from({ length: frames + 1 }, (_, i) =>
    Math.min(i / 60, duration),
  );
  const input = floatAccessor(
    unit,
    times,
    "SCALAR",
    times.length,
    [0],
    [duration],
  );
  const animation = {
    name: label,
    channels: [],
    samplers: [],
    extras: {
      sourceClip: clip.m_Name,
      sourceFrameRate: Number(clip.m_SampleRate),
      loop: true,
    },
  };
  for (const [section, path, components] of groups)
    for (const curve of clip[section] || []) {
      const target = paths.get(curve.path);
      if (target === undefined)
        throw Error(`Unmapped animation target: ${curve.path}`);
      const values = [];
      let previous;
      for (const time of times) {
        let v = components.map((k) => hermite(curve.curve.m_Curve, time, k));
        if (path === "translation") v[0] *= -1;
        if (path === "rotation") {
          v[1] *= -1;
          v[2] *= -1;
          const length = Math.hypot(...v);
          v = v.map((n) => n / length);
          if (previous && v.reduce((sum, n, i) => sum + n * previous[i], 0) < 0)
            v = v.map((n) => -n);
          previous = v;
        }
        values.push(...v);
      }
      const output = floatAccessor(
        unit,
        values,
        path === "rotation" ? "VEC4" : "VEC3",
        times.length,
      );
      animation.channels.push({
        sampler: animation.samplers.length,
        target: { node: target, path },
      });
      animation.samplers.push({ input, output, interpolation: "LINEAR" });
    }
  unit.json.animations.push(animation);
  clipStats.push({ name: label, duration, tracks: animation.channels.length });
}
const footman = compactGLB(unit, [rootNode]);
await writeGLB(join(output, "stark-footman.glb"), footman);
const size = bounds.getSize(new Vector3());
const manifest = {
  localOnly: true,
  board: {
    url: "/local-assets/preview3d/westeros.glb",
    width: size.x,
    depth: size.z,
    sourceBytes: (
      await stat(join(exported, "SceneHierarchyObject/Playmat.glb"))
    ).size,
    bytes: (await stat(join(output, "westeros.glb"))).size,
    nodes: smallBoard.json.nodes.length,
  },
  footman: {
    url: "/local-assets/preview3d/stark-footman.glb",
    bytes: (await stat(join(output, "stark-footman.glb"))).size,
    bones: joints.length,
    clips: clipStats,
  },
  source:
    "Read-only local AssetRipper exports; original geometry, textures, bind poses and motion curves",
};
await writeFile(
  join(output, "manifest.json"),
  JSON.stringify(manifest, null, 2),
);
console.log(JSON.stringify(manifest, null, 2));
await importGameAssets({
  unity,
  exported,
  output,
  board,
  world,
  offset,
  unityObjects,
  Vector3,
  dimensions: { width: size.x, depth: size.z },
});
