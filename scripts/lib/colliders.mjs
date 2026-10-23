import { readFile, readdir } from "node:fs/promises";
import { join } from "node:path";

// Read only the identifiers needed to resolve the original collider components.
// Parsing Unity behaviour/UI documents is unnecessary and would retain a huge scene.
export async function readRegionColliders(unity) {
  const names = new Map(),
    transforms = new Map(),
    byObject = new Map(),
    colliders = [];
  const text = await readFile(join(unity, "Scenes/Playmat.unity"), "utf8");
  for (const match of text.matchAll(
    /--- !u!(\d+) &(\d+)\r?\n([\s\S]*?)(?=--- !u!|$)/g,
  )) {
    const [, type, id, record] = match;
    if (type === "1") names.set(id, record.match(/^  m_Name: (.*)$/m)?.[1]);
    if (type === "4") {
      const go = record.match(/m_GameObject: \{fileID: (\d+)\}/)?.[1],
        parent = record.match(/m_Father: \{fileID: (\d+)\}/)?.[1];
      transforms.set(id, { go, parent });
      byObject.set(go, id);
    }
    if (type === "64" && !/m_Enabled: 0/.test(record)) {
      const go = record.match(/m_GameObject: \{fileID: (\d+)\}/)?.[1],
        guid = record.match(/m_Mesh: \{fileID: \d+, guid: ([a-f0-9]+)/)?.[1];
      if (go && guid) colliders.push({ go, guid });
    }
  }
  const result = new Map(),
    needed = new Set();
  for (const { go, guid } of colliders) {
    if (names.get(go) !== "RegionHint") continue;
    const transform = transforms.get(byObject.get(go)),
      parent = transforms.get(transform.parent);
    const area = names.get(parent.go);
    if (!result.has(area)) result.set(area, []);
    result.get(area).push(guid);
    needed.add(guid);
  }
  const meshes = new Map();
  for (const file of await readdir(join(unity, "Mesh"))) {
    if (!file.endsWith(".asset.meta")) continue;
    const metadata = await readFile(join(unity, "Mesh", file), "utf8"),
      guid = metadata.match(/^guid: ([a-f0-9]+)/m)?.[1];
    if (needed.has(guid)) meshes.set(guid, file.slice(0, -11));
  }
  for (const [area, guids] of result)
    result.set(
      area,
      guids.map((guid) => {
        if (!meshes.has(guid))
          throw Error(`Unresolved collider mesh ${guid} for ${area}`);
        return meshes.get(guid);
      }),
    );
  return result;
}
