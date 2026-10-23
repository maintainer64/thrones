import { readFile, writeFile, copyFile, stat } from "node:fs/promises";
import { join } from "node:path";
import {
  readGLB,
  writeGLB,
  appendView,
  floatAccessor,
  compactGLB,
} from "./gltf.mjs";
import { appendAnimation } from "./animation.mjs";
import { matchRegions } from "./regions.mjs";
import { readRegionColliders } from "./colliders.mjs";
import { importLandmarks } from "./landmarks.mjs";

export async function importGameAssets({
  unity,
  exported,
  output,
  board,
  world,
  offset,
  unityObjects,
  Vector3,
  dimensions,
}) {
  const catalog = JSON.parse(
    await readFile(
      new URL("../../internal/game/catalog.json", import.meta.url),
      "utf8",
    ),
  );
  const areaRoot = board.json.nodes.find(
    (n) => n.name === "AreaEntities" && n.children?.length === 58,
  );
  if (!areaRoot) throw Error("Missing original area hierarchy");
  const nodes = areaRoot.children.map((index) => ({
    index,
    name: board.json.nodes[index].name,
  }));
  const matched = matchRegions(catalog.boards.westeros.regions, nodes);
  const colliders = await readRegionColliders(unity);
  const hits = {
    json: {
      asset: { version: "2.0" },
      nodes: [],
      meshes: [],
      accessors: [],
      bufferViews: [],
    },
    bin: Buffer.alloc(0),
  };
  const hitRoots = [],
    regions = {};
  hits.json.materials = [
    {
      name: "Region picking surface",
      doubleSided: true,
      pbrMetallicRoughness: {
        baseColorFactor: [1, 1, 1, 1],
        metallicFactor: 0,
        roughnessFactor: 1,
      },
    },
  ];
  function position(index) {
    const p = new Vector3().setFromMatrixPosition(
      offset.clone().multiply(world.get(index)),
    );
    return [p.x, p.z];
  }
  for (const [id, { index }] of matched) {
    const source = board.json.nodes[index];
    const hint = source.children.find(
      (i) => board.json.nodes[i].name === "RegionHint",
    );
    const hit = board.json.nodes[hint]?.children?.find(
      (i) => board.json.nodes[i].mesh !== undefined,
    );
    if (hit === undefined) throw Error(`Missing picking mesh for ${id}`);
    // RegionHighlight is an extruded wall effect, NOT the filled picking surface.
    // Original *_Hint meshes are the flat collider polygons in the same local frame.
    const meshNames = colliders.get(source.name);
    if (!meshNames?.length)
      throw Error(`Missing original MeshCollider for ${id}`);
    const geometry = { primitives: [] };
    for (const meshName of meshNames) {
      const polygon = await readGLB(join(exported, `Mesh/${meshName}.glb`));
      for (const primitive of polygon.json.meshes[0].primitives) {
        const a = polygon.json.accessors[primitive.attributes.POSITION],
          v = polygon.json.bufferViews[a.bufferView];
        const start = (v.byteOffset || 0) + (a.byteOffset || 0),
          stride = v.byteStride || 12,
          values = [];
        for (let i = 0; i < a.count; i++)
          values.push(
            polygon.bin.readFloatLE(start + i * stride),
            0,
            polygon.bin.readFloatLE(start + i * stride + 8),
          );
        const position = floatAccessor(
          hits,
          values,
          "VEC3",
          a.count,
          [a.min[0], 0, a.min[2]],
          [a.max[0], 0, a.max[2]],
        );
        const p = { attributes: { POSITION: position }, material: 0 };
        if (primitive.indices !== undefined) {
          const a = polygon.json.accessors[primitive.indices],
            v = polygon.json.bufferViews[a.bufferView],
            bytes = { 5121: 1, 5123: 2, 5125: 4 }[a.componentType];
          const start = (v.byteOffset || 0) + (a.byteOffset || 0);
          p.indices = hits.json.accessors.length;
          hits.json.accessors.push({
            componentType: a.componentType,
            count: a.count,
            type: "SCALAR",
            bufferView: appendView(
              hits,
              polygon.bin.subarray(start, start + a.count * bytes),
            ),
          });
        }
        geometry.primitives.push(p);
      }
    }
    hitRoots.push(hits.json.nodes.length);
    hits.json.nodes.push({
      name: id,
      extras: { regionId: id },
      mesh: hits.json.meshes.length,
      matrix: offset.clone().multiply(world.get(hint)).toArray(),
    });
    hits.json.meshes.push(geometry);
    const anchor = source.children.find((i) =>
      /^(UnitAnchor|ShipAnchor)$/.test(board.json.nodes[i].name),
    );
    const token = source.children.find(
      (i) => board.json.nodes[i].name === "OrderToken",
    );
    regions[id] = {
      position: position(anchor ?? index),
      label: position(index),
      order: position(token ?? index),
    };
  }
  await writeGLB(join(output, "regions.glb"), compactGLB(hits, hitRoots));
  const units = {};
  for (const [type, prefab, meshFile, rootName, clips] of [
    ["footman", "Footman", null, null, []],
    [
      "knight",
      "Knight",
      "msh_knight",
      "Unit",
      [
        ["A_Unit_Knight_Idle1.anim", "idle"],
        ["A_Unit_Knight_Trot.anim", "run"],
      ],
    ],
    ["ship", "Ship", "Ship", "G_unit_Ship", [["ShipIdle.anim", "idle"]]],
    ["siege-engine", "SiegeEngine", null, "Unit", []],
  ]) {
    let document;
    if (type === "footman")
      document = await readGLB(join(output, "stark-footman.glb"));
    else {
      document = await readGLB(
        join(exported, `PrefabHierarchyObject/P_unit_${prefab}.glb`),
      );
      const names = new Map(
        document.json.nodes.map((node, index) => [node.name, index]),
      );
      const root = names.get(rootName);
      if (root === undefined) throw Error(`Missing ${type} root`);
      if (meshFile) {
        const objects = unityObjects(
          await readFile(
            join(unity, `GameObject/P_unit_${prefab}.prefab`),
            "utf8",
          ),
        );
        const renderer = [...objects.values()]
          .map((o) => o.SkinnedMeshRenderer)
          .find((r) => r?.m_Bones?.length > 0);
        if (!renderer) throw Error(`Missing ${type} renderer`);
        const transforms = new Map(
          [...objects]
            .filter(([, o]) => o.Transform)
            .map(([id, o]) => [id, o.Transform]),
        );
        const nameOf = (ref) =>
          objects.get(
            String(transforms.get(String(ref.fileID)).m_GameObject.fileID),
          ).GameObject.m_Name;
        const bodyName = objects.get(String(renderer.m_GameObject.fileID))
          .GameObject.m_Name;
        const body = names.get(bodyName);
        const joints = renderer.m_Bones.map((ref) => names.get(nameOf(ref)));
        if (body === undefined || joints.some((i) => i === undefined))
          throw Error(`Unmapped ${type} bones`);
        const mesh = await readGLB(join(exported, `Mesh/${meshFile}.glb`));
        const viewOffset = document.json.bufferViews.length,
          accessorOffset = document.json.accessors.length;
        const start =
          document.bin.length + ((4 - (document.bin.length % 4)) % 4);
        document.bin = Buffer.concat([
          document.bin,
          Buffer.alloc(start - document.bin.length),
          mesh.bin,
        ]);
        mesh.json.bufferViews.forEach((v) =>
          document.json.bufferViews.push({
            ...v,
            byteOffset: start + (v.byteOffset || 0),
          }),
        );
        mesh.json.accessors.forEach((a) =>
          document.json.accessors.push({
            ...a,
            bufferView: a.bufferView + viewOffset,
          }),
        );
        const geometry = structuredClone(mesh.json.meshes[0]);
        const material = document.json.materials.findIndex((m) =>
          m.name?.startsWith(`M_unit_${prefab}_`),
        );
        for (const primitive of geometry.primitives) {
          for (const key of Object.keys(primitive.attributes))
            primitive.attributes[key] += accessorOffset;
          if (primitive.indices !== undefined)
            primitive.indices += accessorOffset;
          primitive.material = material;
        }
        document.json.nodes[body].mesh = document.json.meshes.length;
        document.json.meshes.push(geometry);
        const original = [
          ...unityObjects(
            await readFile(join(unity, `Mesh/${meshFile}.asset`), "utf8"),
          ).values(),
        ][0].Mesh;
        const signs = [-1, 1, 1, 1],
          binds = [];
        for (const pose of original.m_BindPose)
          for (let col = 0; col < 4; col++)
            for (let row = 0; row < 4; row++)
              binds.push(
                Number(pose[`e${row}${col}`]) * signs[row] * signs[col],
              );
        if (binds.length !== joints.length * 16)
          throw Error(`Invalid ${type} bind poses`);
        document.json.nodes[body].skin = 0;
        document.json.skins = [
          {
            joints,
            skeleton: names.get(nameOf(renderer.m_RootBone)),
            inverseBindMatrices: floatAccessor(
              document,
              binds,
              "MAT4",
              joints.length,
            ),
          },
        ];
      }
      document.json.animations = [];
      for (const [file, label] of clips) {
        const clip = [
          ...unityObjects(
            await readFile(join(unity, "AnimationClip", file), "utf8"),
          ).values(),
        ][0].AnimationClip;
        appendAnimation(document, root, clip, label);
      }
      document = compactGLB(document, [root]);
    }
    const textures = {};
    for (const house of [
      "stark",
      "lannister",
      "greyjoy",
      "baratheon",
      "tyrell",
      "martell",
    ]) {
      const name = `${type}-${house}.png`;
      await copyFile(
        join(
          exported,
          `Texture2D/T_unit_${prefab}_${house[0].toUpperCase() + house.slice(1)}.png`,
        ),
        join(output, name),
      );
      textures[house] = `/local-assets/preview3d/${name}`;
    }
    const name = `game-${type}.glb`;
    await writeGLB(join(output, name), document);
    units[type] = {
      url: `/local-assets/preview3d/${name}`,
      textures,
      bytes: (await stat(join(output, name))).size,
      bones: document.json.skins?.[0]?.joints.length || 0,
      clips: (document.json.animations || []).map((a) => a.name),
    };
  }
  const manifest = {
    version: 1,
    localOnly: true,
    board: {
      url: "/local-assets/preview3d/westeros.glb",
      width: dimensions.width,
      depth: dimensions.depth,
    },
    picking: "/local-assets/preview3d/regions.glb",
    regions,
    units,
    landmarks: await importLandmarks({ exported, output }),
  };
  await writeFile(
    join(output, "game-manifest.json"),
    JSON.stringify(manifest, null, 2),
  );
  console.log(
    `Game assets: ${matched.size} exact regions; ${Object.keys(units).length} unit types; six house textures each`,
  );
}
