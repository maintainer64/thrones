import { copyFile, mkdir, readFile } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const source = resolve(process.argv[2] || resolve(root, "../westeros-web-references/agot-bg-game-server/public/images"));
const cards = JSON.parse(await readFile(resolve(root, "web/src/events/art.json"), "utf8"));
const destination = resolve(root, "local-assets/events");
await mkdir(destination, { recursive: true });
for (const [id, file] of Object.entries(cards)) {
  const directory = id in {
    "silence-at-the-wall": 1, "preemptive-raid": 1, "crow-killers": 1,
    "rattleshirts-raiders": 1, "massing-on-the-milkwater": 1, "a-king-beyond-the-wall": 1,
    "mammoth-riders": 1, "the-horde-descends": 1, "skinchanger-scout": 1,
  } ? "wildling-cards" : "westeros-cards";
  await copyFile(resolve(source, directory, file), resolve(destination, file));
}
console.log(`Imported ${Object.keys(cards).length} local event card images; no external upload.`);
