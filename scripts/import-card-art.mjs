// Local-only derivative assets. Never modifies or uploads the PSD/PDF originals.
import { readdir, mkdir, writeFile, stat } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { join, resolve } from "node:path";

const require = createRequire(new URL("../web/package.json", import.meta.url));
const sharp = require("sharp");
const run = promisify(execFile);
const local = fileURLToPath(new URL("../local-assets/", import.meta.url));
const output = join(local, "cards");
const source = resolve(
  process.argv[2] || fileURLToPath(new URL("../../westeros-web-references/asset-sources/cards/", import.meta.url)),
);
const psd = join(source, "исходники/карты домов лицо.psd");
await stat(psd);
await mkdir(output, { recursive: true });
const manifest = { localOnly: true, frames: {}, portraits: {} };
// PSD layer names checked against the supplied source, not the blank merged preview.
for (const [house, layer] of Object.entries({
  tyrell: 2,
  stark: 4,
  martell: 5,
  lannister: 6,
  greyjoy: 7,
  baratheon: 8,
})) {
  const { stdout } = await run(
    "magick",
    [`${psd}[${layer}]`, "+repage", "png:-"],
    { encoding: "buffer", maxBuffer: 16 * 1024 * 1024 },
  );
  await sharp(stdout)
    .resize(709, 1111, { fit: "fill" })
    .webp({ lossless: true })
    .toFile(join(output, `${house}-frame.webp`));
  manifest.frames[house] = `/local-assets/cards/${house}-frame.webp`;
}
for (const filename of await readdir(local)) {
  if (
    !/^T_crd_(stark|lannister|baratheon|greyjoy|tyrell|martell)_[a-z]+\.png$/.test(
      filename,
    )
  )
    continue;
  const image = sharp(join(local, filename));
  const { width, height } = await image.metadata();
  // The installed game's 512² atlas has a narrow portrait between flat side gutters.
  if (width !== 512 || height !== 512)
    throw Error(`Unexpected portrait atlas: ${filename}`);
  const name = filename.replace(/\.png$/, "");
  await image
    .extract({ left: 96, top: 0, width: 344, height: 512 })
    .webp({ quality: 92 })
    .toFile(join(output, `${name}.webp`));
  manifest.portraits[name] = `/local-assets/cards/${name}.webp`;
}
if (Object.keys(manifest.portraits).length !== 42)
  throw Error("Import the installed game's 42 base house portraits first.");
await writeFile(
  join(output, "manifest.json"),
  JSON.stringify(manifest, null, 2),
);
console.log(
  `Prepared 6 PSD frames and 42 portraits in ${output}. Source files unchanged. PDFs describe alternate decks and are not game rules.`,
);
