import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  appendView,
  compactGLB,
  floatAccessor,
  readGLB,
  writeGLB,
} from "./gltf.mjs";
import { hermite } from "./unity.mjs";
import { matchRegions, regionKey } from "./regions.mjs";
import { appendAnimation } from "./animation.mjs";

test("GLB round trip aligns binary views without changing float values", async (t) => {
  const directory = await mkdtemp(join(tmpdir(), "westeros-gltf-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const document = {
    json: { asset: { version: "2.0" } },
    bin: Buffer.alloc(0),
  };
  appendView(document, Buffer.from([1, 2, 3]));
  floatAccessor(document, [1.25, -2, 3.5], "VEC3", 1);
  const file = join(directory, "synthetic.glb");
  await writeGLB(file, document);
  const loaded = await readGLB(file);
  assert.equal(loaded.json.bufferViews[1].byteOffset, 4);
  assert.equal(loaded.bin.readFloatLE(4), 1.25);
  assert.equal(loaded.bin.readFloatLE(8), -2);
  assert.equal(loaded.bin.readFloatLE(12), 3.5);
  assert.deepEqual(loaded.json.accessors[0], document.json.accessors[0]);
});

test("GLB reader rejects unrelated file data", async (t) => {
  const directory = await mkdtemp(join(tmpdir(), "westeros-gltf-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const file = join(directory, "invalid.glb");
  await writeFile(file, Buffer.alloc(32));
  await assert.rejects(readGLB(file), /Invalid GLB/);
});

test("compaction retains skin, texture and relevant animation, dropping unrelated scene", () => {
  const doc = {
    json: {
      nodes: [
        { name: "unused", mesh: 0 },
        { name: "unit", mesh: 1, skin: 0, children: [2] },
        { name: "hip" },
      ],
      meshes: [
        { primitives: [{ attributes: { POSITION: 0 } }] },
        { primitives: [{ attributes: { POSITION: 1 }, material: 0 }] },
      ],
      skins: [{ joints: [2], skeleton: 2, inverseBindMatrices: 2 }],
      materials: [{ pbrMetallicRoughness: { baseColorTexture: { index: 0 } } }],
      textures: [{ source: 0 }],
      images: [],
      animations: [
        {
          name: "idle",
          channels: [
            { sampler: 0, target: { node: 0, path: "translation" } },
            { sampler: 1, target: { node: 2, path: "translation" } },
          ],
          samplers: [
            { input: 3, output: 4 },
            { input: 3, output: 5 },
          ],
        },
      ],
    },
    bin: Buffer.alloc(0),
  };
  floatAccessor(doc, [99, 99, 99], "VEC3", 1);
  floatAccessor(doc, [1, 2, 3], "VEC3", 1);
  floatAccessor(
    doc,
    Array.from({ length: 16 }, (_, i) => (i % 5 === 0 ? 1 : 0)),
    "MAT4",
    1,
  );
  floatAccessor(doc, [0, 1], "SCALAR", 2);
  floatAccessor(doc, [99, 99, 99, 99, 99, 99], "VEC3", 2);
  floatAccessor(doc, [0, 0, 0, 1, 0, 0], "VEC3", 2);
  doc.json.images.push({
    bufferView: appendView(doc, Buffer.from([137, 80, 78, 71])),
    mimeType: "image/png",
  });
  const before = structuredClone(doc.json);
  const compact = compactGLB(doc, [1]);
  assert.deepEqual(doc.json, before, "source metadata remains unchanged");
  assert.deepEqual(
    compact.json.nodes.map((n) => n.name),
    ["unit", "hip"],
  );
  assert.equal(compact.json.nodes[0].mesh, 0);
  assert.deepEqual(compact.json.skins[0].joints, [1]);
  assert.equal(compact.json.skins[0].skeleton, 1);
  assert.equal(compact.json.animations[0].channels.length, 1);
  assert.equal(compact.json.animations[0].channels[0].target.node, 1);
  assert.equal(compact.json.animations[0].channels[0].sampler, 0);
  assert.equal(compact.json.animations[0].samplers.length, 1);
  assert.equal(compact.json.accessors.length, 4);
  assert.equal(compact.json.meshes.length, 1);
  const image = compact.json.bufferViews[compact.json.images[0].bufferView];
  assert.deepEqual(
    [
      ...compact.bin.subarray(
        image.byteOffset,
        image.byteOffset + image.byteLength,
      ),
    ],
    [137, 80, 78, 71],
  );
  assert.ok(compact.bin.length < doc.bin.length);
});

test("Hermite interpolation clamps ends and respects per-second tangents", () => {
  const keys = [
    { time: 0, value: { x: 2 }, outSlope: { x: 2 } },
    { time: 2, value: { x: 6 }, inSlope: { x: 2 } },
  ];
  assert.equal(hermite(keys, -1, "x"), 2);
  assert.equal(hermite(keys, 3, "x"), 6);
  assert.equal(hermite(keys, 0.5, "x"), 3);
  assert.equal(hermite(keys, 1, "x"), 4);
});

test("Hermite interpolation handles stepped and single-key curves", () => {
  const keys = [
    { time: 0, value: { x: 1 }, outSlope: { x: Infinity } },
    { time: 1, value: { x: 3 }, inSlope: { x: Infinity } },
  ];
  assert.equal(hermite(keys, 0.5, "x"), 1);
  assert.equal(hermite(keys, 1, "x"), 3);
  assert.equal(hermite([keys[0]], 5, "x"), 1);
  assert.throws(() => hermite([], 0, "x"), /no keys/);
});

test("original region names match catalog IDs including ports and spelling differences", () => {
  assert.equal(regionKey("Winterfell_Port"), regionKey("Port of Winterfell"));
  const regions = [
    { id: "redwyne-straights", name: "Redwyne Straights" },
    { id: "seagard", name: "Seagard" },
  ];
  const nodes = [
    { name: "RedwyneStraits", index: 1 },
    { name: "Seaguard", index: 2 },
  ];
  assert.equal(matchRegions(regions, nodes).get("seagard").index, 2);
  assert.throws(
    () => matchRegions([{ id: "missing", name: "Missing" }], nodes),
    /no region missing/,
  );
});

test("animation export mirrors Unity X, normalizes rotations and maps bone paths", () => {
  const document = {
    json: { nodes: [{ name: "Unit", children: [1] }, { name: "hip" }] },
    bin: Buffer.alloc(0),
  };
  const key = (time, value) => ({ time, value });
  const clip = {
    m_Name: "Synthetic",
    m_SampleRate: 60,
    m_AnimationClipSettings: { m_StopTime: 1 },
    m_PositionCurves: [
      {
        path: "hip",
        curve: {
          m_Curve: [key(0, { x: 1, y: 2, z: 3 }), key(1, { x: 2, y: 2, z: 3 })],
        },
      },
    ],
    m_RotationCurves: [
      {
        path: "hip",
        curve: {
          m_Curve: [
            key(0, { x: 1, y: 2, z: 3, w: 4 }),
            key(1, { x: 2, y: 4, z: 6, w: 8 }),
          ],
        },
      },
    ],
  };
  const result = appendAnimation(document, 0, clip, "idle");
  assert.equal(result.tracks, 2);
  const animation = document.json.animations[0];
  for (const channel of animation.channels) {
    assert.equal(channel.target.node, 1);
    const accessor =
      document.json.accessors[animation.samplers[channel.sampler].output];
    const offset = document.json.bufferViews[accessor.bufferView].byteOffset;
    assert.equal(accessor.count, 61);
    if (channel.target.path === "translation")
      assert.equal(document.bin.readFloatLE(offset), -1);
    else {
      const value = Array.from({ length: 4 }, (_, i) =>
        document.bin.readFloatLE(offset + i * 4),
      );
      assert.ok(Math.abs(Math.hypot(...value) - 1) < 1e-6);
      assert.ok(value[0] > 0 && value[1] < 0 && value[2] < 0 && value[3] > 0);
    }
  }
  assert.throws(
    () =>
      appendAnimation(
        document,
        0,
        {
          ...clip,
          m_PositionCurves: [{ ...clip.m_PositionCurves[0], path: "unknown" }],
        },
        "invalid",
      ),
    /Unmapped animation target/,
  );
});
