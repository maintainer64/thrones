import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { appendView, compactGLB, readGLB, writeGLB } from "./gltf.mjs";

export async function importLandmarks({ exported, output }) {
  const result = {};
  for (const [kind, name] of [
    ["castle", "Castle"],
    ["stronghold", "Stronghold"],
    ["port", "Port"],
  ]) {
    const document = await readGLB(
      join(
        exported,
        kind === "port"
          ? `Mesh/${name}.glb`
          : `PrefabHierarchyObject/P_tkn_${name}.glb`,
      ),
    );
    if (kind !== "port") {
      // Only the architecture: omit garrison UI, blast, smoke and Unity scripts.
      document.json.nodes[0].children = [];
    } else {
      const image = appendView(
        document,
        await readFile(join(exported, "Texture2D/T_env_Port.png")),
      );
      document.json.images = [{ bufferView: image, mimeType: "image/png" }];
      document.json.textures = [{ source: 0 }];
      document.json.materials = [
        {
          name: "Original port",
          pbrMetallicRoughness: { baseColorTexture: { index: 0 } },
        },
      ];
    }
    for (const material of document.json.materials) {
      material.pbrMetallicRoughness ??= {};
      material.pbrMetallicRoughness.metallicFactor = 0.08;
      material.pbrMetallicRoughness.roughnessFactor = 0.8;
    }
    const file = `landmark-${kind}.glb`;
    await writeGLB(join(output, file), compactGLB(document, [0]));
    result[kind] = `/local-assets/preview3d/${file}`;
  }
  return result;
}
