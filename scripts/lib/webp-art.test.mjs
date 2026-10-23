import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, writeFile, stat, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { prepareWebP, sharp, revision } from "./webp-art.mjs";

async function fixture(t) {
  const directory = await mkdtemp(join(tmpdir(), "westeros-webp-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const source = pathToFileURL(join(directory, "source.png"));
  const destination = pathToFileURL(join(directory, "face.webp"));
  const width = 47,
    height = 31;
  const rgba = Buffer.alloc(width * height * 4);
  for (let i = 0; i < width * height; i++) {
    rgba.set([i % 255, (i * 7) % 255, (i * 13) % 255, i % 256], i * 4);
  }
  await writeFile(
    source,
    await sharp(rgba, { raw: { width, height, channels: 4 } })
      .png()
      .toBuffer(),
  );
  return { source, destination, width, height };
}

test("WebP preserves full dimensions, exact transparency and untouched PNG source", async (t) => {
  const { source, destination, width, height } = await fixture(t);
  const before = await readFile(source);
  const result = await prepareWebP(source, destination);
  const output = await readFile(destination);
  const decoded = await sharp(output).metadata();
  assert.equal(decoded.format, "webp");
  assert.equal(decoded.width, width);
  assert.equal(decoded.height, height);
  assert.equal(result.sourceRevision, revision(before));
  assert.equal(result.revision, revision(output));
  assert.equal(result.sourceBytes, before.length);
  assert.equal(result.bytes, output.length);
  assert.deepEqual(await readFile(source), before);
  const alpha = (image) =>
    sharp(image).ensureAlpha().extractChannel("alpha").raw().toBuffer();
  assert.deepEqual(await alpha(before), await alpha(output));
});

test("WebP cache reuses unchanged output and repairs corrupt or stale derivatives", async (t) => {
  const { source, destination } = await fixture(t);
  const first = await prepareWebP(source, destination);
  const timestamp = (await stat(destination)).mtimeMs;
  const cached = await prepareWebP(source, destination, first);
  assert.equal(cached.cached, true);
  assert.equal((await stat(destination)).mtimeMs, timestamp);
  await writeFile(destination, "corrupt derivative");
  const repaired = await prepareWebP(source, destination, first);
  assert.equal(repaired.cached, false);
  assert.equal(repaired.revision, first.revision);
  await writeFile(
    source,
    await sharp({
      create: { width: 19, height: 23, channels: 4, background: "#80aacc80" },
    })
      .png()
      .toBuffer(),
  );
  const changed = await prepareWebP(source, destination, first);
  assert.equal(changed.cached, false);
  assert.notEqual(changed.sourceRevision, first.sourceRevision);
  assert.equal(changed.width, 19);
  assert.equal(changed.height, 23);
});

test("WebP conversion rejects overwriting originals and non-PNG source data", async (t) => {
  const { source, destination } = await fixture(t);
  const before = await readFile(source);
  await assert.rejects(prepareWebP(source, source), /must not overwrite/);
  assert.deepEqual(await readFile(source), before);
  await writeFile(source, await sharp(before).flatten().jpeg().toBuffer());
  await assert.rejects(prepareWebP(source, destination), /single-frame PNG/);
});
