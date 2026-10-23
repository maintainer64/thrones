// Keep optional expansion/research imports beside the project, not in the
// directory served to browsers. Dry-run by default; --apply never overwrites.
import { readdir, readFile, writeFile, mkdir, rename, access } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { join, dirname, basename } from "node:path";
import { createHash } from "node:crypto";

const root = fileURLToPath(new URL("../", import.meta.url));
const runtime = join(root, "local-assets");
const archive = join(dirname(root), `${basename(root)}-workbench`, "unused-imports");
const names = (await readdir(runtime)).filter(name => /^T_crd_(2_|3_|Wildling_|westeros_|vassal)/.test(name) || name === "tutorial-source-en.txt");
const plan = await Promise.all(names.sort().map(async name => {
  const data = await readFile(join(runtime, name));
  return { name, bytes: data.length, sha256: createHash("sha256").update(data).digest("hex") };
}));
console.log(JSON.stringify({ dryRun: !process.argv.includes("--apply"), archive, count: plan.length, bytes: plan.reduce((n,f)=>n+f.bytes,0), files: plan.map(f=>f.name) }, null, 2));
if (!process.argv.includes("--apply") || !plan.length) process.exit(0);
await mkdir(archive, { recursive: true });
for (const {name} of plan) {
  try { await access(join(archive, name)); } catch { continue; }
  throw new Error(`Archive file already exists; refusing to overwrite: ${name}`);
}
const manifestPath = join(runtime,"manifest.json");
const manifest = JSON.parse(await readFile(manifestPath,"utf8"));
const removed = {};
// Preserve original lookup metadata and checksums for a reversible restore.
for (const {name} of plan) for (const [key,url] of Object.entries(manifest.textures)) {
  if (url === `/local-assets/${name}`) {removed[key]=url; delete manifest.textures[key];}
}
await writeFile(join(archive,"archive-manifest.json"), JSON.stringify({files:plan,textures:removed},null,2), {flag:"wx"});
for (const {name} of plan) await rename(join(runtime,name), join(archive,name));
await writeFile(manifestPath,JSON.stringify(manifest,null,2)+"\n");
