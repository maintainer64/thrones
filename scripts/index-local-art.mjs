// Preserve PNG originals; build full-resolution WebP derivatives for the browser.
// Docker/CI retain local metadata when the optional asset volume is absent.
import { readdir, readFile, writeFile } from "node:fs/promises";
import { prepareWebP } from "./lib/webp-art.mjs";

const root = new URL("../local-assets/", import.meta.url);
const output = new URL("../web/src/artwork/index.json", import.meta.url);
let filenames;
const index = JSON.parse(await readFile(output, "utf8"));
try {
  filenames = await readdir(root);
} catch (error) {
  if (error.code !== "ENOENT") throw error;
  filenames = [];
  console.log("Local artwork absent; retaining its checked-in WebP metadata.");
}
const events = JSON.parse(
  await readFile(
    new URL("../web/src/events/art.json", import.meta.url),
    "utf8",
  ),
);
const paths = [
  ...filenames.filter((name) =>
    /^T_crd_(stark|lannister|baratheon|greyjoy|tyrell|martell)_[a-z]+\.png$/.test(
      name,
    ),
  ),
  ...Object.values(events).map((name) => `events/${name}`),
  ...["zero", "sword", "fortification", "two", "three", "skull"].map(
    (id) => `tides-of-battle/${id}.png`,
  ),
  "ui/strength-shield.png",
];
const orderBases = JSON.parse(
  await readFile(
    new URL("../web/src/artwork/order-bases.json", import.meta.url),
    "utf8",
  ),
);
const sources = [
  ...paths.map((path) => [path, new URL(path, root)]),
  ...Object.values(orderBases).map((name) => [
    `media/orders/${name}`,
    new URL(`../web/public/media/orders/${name}`, import.meta.url),
  ]),
];
let converted = 0,
  cached = 0;
for (const [path, source] of sources) {
  try {
    const file = path.replace(/\.png$/, ".webp");
    const result = await prepareWebP(
      source,
      new URL(source.href.replace(/\.png$/, ".webp")),
      index[path],
    );
    const { cached: reused, ...metadata } = result;
    index[path] = { file, ...metadata };
    if (reused) cached++;
    else converted++;
    if ((converted + cached) % 10 === 0)
      console.log(`Prepared ${converted + cached}/${sources.length} images...`);
  } catch (error) {
    if (error.code === "ENOENT" && !path.startsWith("media/")) continue;
    throw error;
  }
}
const sorted = Object.fromEntries(
  Object.entries(index).sort(([a], [b]) => a.localeCompare(b, "en")),
);
await writeFile(output, JSON.stringify(sorted, null, 2) + "\n");
const originals = Object.values(index).reduce(
  (sum, image) => sum + (image.sourceBytes || 0),
  0,
);
const webp = Object.values(index).reduce(
  (sum, image) => sum + (image.bytes || 0),
  0,
);
console.log(
  `${converted} converted, ${cached} cached. PNG originals unchanged. ${(originals / 1048576).toFixed(1)} MiB → ${(webp / 1048576).toFixed(1)} MiB WebP (${(100 * (1 - webp / originals)).toFixed(1)}% smaller).`,
);
