import { createHash, randomUUID } from "node:crypto";
import { readFile, writeFile, rename, unlink } from "node:fs/promises";
import { createRequire } from "node:module";

const require = createRequire(
  new URL("../../web/package.json", import.meta.url),
);
export const sharp = require("sharp");
export const encoding = `webp-q92-alpha100-effort6-v1-libwebp${sharp.versions.webp}`;
export const revision = (bytes) =>
  createHash("sha256").update(bytes).digest("hex").slice(0, 12);

// Source bytes are read only. Derivatives keep the full dimensions and alpha;
// quality 92 is deliberately lossy for RGB, not for the transparency channel.
export async function prepareWebP(source, destination, previous) {
  if (source.href === destination.href)
    throw new Error("WebP destination must not overwrite its PNG source");
  const input = await readFile(source);
  const sourceRevision = revision(input);
  if (
    previous?.encoding === encoding &&
    previous.sourceRevision === sourceRevision
  ) {
    try {
      const cached = await readFile(destination);
      if (revision(cached) === previous.revision)
        return { ...previous, cached: true };
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
  const metadata = await sharp(input).metadata();
  if (
    metadata.format !== "png" ||
    !metadata.width ||
    !metadata.height ||
    metadata.pages > 1
  ) {
    throw new Error(`Expected a single-frame PNG: ${source}`);
  }
  const { data, info } = await sharp(input)
    .webp({ quality: 92, alphaQuality: 100, effort: 6, smartSubsample: true })
    .toBuffer({ resolveWithObject: true });
  if (info.width !== metadata.width || info.height !== metadata.height) {
    throw new Error(`WebP dimensions changed: ${source}`);
  }
  const temporary = new URL(`.${randomUUID()}.webp.tmp`, destination);
  try {
    await writeFile(temporary, data, { flag: "wx" });
    await rename(temporary, destination);
  } finally {
    await unlink(temporary).catch((error) => {
      if (error.code !== "ENOENT") throw error;
    });
  }
  return {
    width: info.width,
    height: info.height,
    revision: revision(data),
    sourceRevision,
    sourceBytes: input.length,
    bytes: data.length,
    encoding,
    cached: false,
  };
}
