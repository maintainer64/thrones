// Read-only import from the provided Unity export; only ignored local assets change.
import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { homedir } from "node:os";
import { importLandmarks } from "./lib/landmarks.mjs";
const exported = resolve(
  process.argv[2] ||
    `${homedir()}/Desktop/Resources.out/AssetRipper_export_20261018_045740/Assets`,
);
const output = fileURLToPath(
  new URL("../local-assets/preview3d/", import.meta.url),
);
const manifest = JSON.parse(
  await readFile(`${output}/game-manifest.json`, "utf8"),
);
manifest.landmarks = await importLandmarks({ exported, output });
await writeFile(
  `${output}/game-manifest.json`,
  JSON.stringify(manifest, null, 2),
);
console.log(
  "Prepared original Castle, Stronghold and textured Port models; source export unchanged.",
);
