// Narrow, recoverable cleanup: old public card PNGs and the unused blank face.
// Never moves PNG sources listed by the current WebP pipeline. Dry-run by default.
import {
  readdir,
  readFile,
  mkdir,
  writeFile,
  rename,
  lstat,
} from "node:fs/promises";
import { createHash } from "node:crypto";

const root = new URL("../", import.meta.url);
const archive = new URL(
  "../../westeros-web-workbench/unused-cards-2026-10-23/",
  import.meta.url,
);
const index = JSON.parse(
  await readFile(new URL("web/src/artwork/index.json", root), "utf8"),
);
const protectedPaths = new Set(
  Object.keys(index).map((path) =>
    path.startsWith("media/") ? `web/public/${path}` : `local-assets/${path}`,
  ),
);
const candidates = ["local-assets/T_crd_none_blank.png"];
try {
  for (const name of await readdir(new URL("web/public/media/cards/", root))) {
    if (/^[a-z0-9-]+\.png$/.test(name))
      candidates.push(`web/public/media/cards/${name}`);
  }
} catch (error) {
  if (error.code !== "ENOENT") throw error;
}
const files = [];
for (const path of candidates.sort()) {
  if (protectedPaths.has(path))
    throw new Error(`Refusing to move a WebP source: ${path}`);
  try {
    if (!(await lstat(new URL(path, root))).isFile())
      throw new Error(`Not a regular file: ${path}`);
    const bytes = await readFile(new URL(path, root));
    files.push({
      path,
      bytes: bytes.length,
      sha256: createHash("sha256").update(bytes).digest("hex"),
    });
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}
console.log(
  JSON.stringify(
    {
      dryRun: !process.argv.includes("--apply"),
      archive: archive.pathname,
      count: files.length,
      bytes: files.reduce((n, file) => n + file.bytes, 0),
      files: files.map((f) => f.path),
    },
    null,
    2,
  ),
);
if (!process.argv.includes("--apply") || !files.length) process.exit(0);
for (const file of files) {
  try {
    await lstat(new URL(file.path, archive));
    throw new Error(`Archive target exists: ${file.path}`);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}
await mkdir(archive, { recursive: true });
const media = JSON.parse(
  await readFile(new URL("web/public/media/manifest.json", root), "utf8"),
);
const local = JSON.parse(
  await readFile(new URL("local-assets/manifest.json", root), "utf8"),
);
await writeFile(
  new URL("archive-manifest.json", archive),
  JSON.stringify(
    {
      originalRoot: root.pathname,
      files,
      removedLookups: {
        mediaCards: media.cards,
        blankTexture: local.textures?.T_crd_none_blank,
      },
      restore:
        "Move each file.path from this archive back beneath originalRoot; all paths and SHA-256 checksums are listed above.",
    },
    null,
    2,
  ) + "\n",
  { flag: "wx" },
);
for (const file of files) {
  await mkdir(new URL("./", new URL(file.path, archive)), { recursive: true });
  await rename(new URL(file.path, root), new URL(file.path, archive));
}
delete media.cards;
if (local.textures) delete local.textures.T_crd_none_blank;
await writeFile(
  new URL("web/public/media/manifest.json", root),
  JSON.stringify(media, null, 2) + "\n",
);
await writeFile(
  new URL("local-assets/manifest.json", root),
  JSON.stringify(local, null, 2) + "\n",
);
console.log(
  `Archived ${files.length} unused PNGs; ${protectedPaths.size} PNG sources kept intact.`,
);
