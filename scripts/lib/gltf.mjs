import { readFile, writeFile } from "node:fs/promises";

export async function readGLB(file) {
  const bytes = await readFile(file);
  if (
    bytes.toString("ascii", 0, 4) !== "glTF" ||
    bytes.readUInt32LE(4) !== 2 ||
    bytes.readUInt32LE(8) !== bytes.length
  )
    throw Error(`Invalid GLB: ${file}`);
  let json,
    bin = Buffer.alloc(0);
  for (let offset = 12; offset < bytes.length;) {
    const size = bytes.readUInt32LE(offset),
      type = bytes.readUInt32LE(offset + 4);
    if (type === 0x4e4f534a)
      json = JSON.parse(bytes.toString("utf8", offset + 8, offset + 8 + size));
    if (type === 0x004e4942)
      bin = bytes.subarray(offset + 8, offset + 8 + size);
    offset += 8 + size;
  }
  if (!json) throw Error(`Missing GLB JSON: ${file}`);
  return { json, bin };
}
export async function writeGLB(file, { json, bin }) {
  json.buffers = [{ byteLength: bin.length }];
  let text = Buffer.from(JSON.stringify(json));
  text = Buffer.concat([text, Buffer.alloc((4 - (text.length % 4)) % 4, 0x20)]);
  const data = Buffer.concat([bin, Buffer.alloc((4 - (bin.length % 4)) % 4)]);
  const out = Buffer.alloc(28 + text.length + data.length);
  out.write("glTF");
  out.writeUInt32LE(2, 4);
  out.writeUInt32LE(out.length, 8);
  out.writeUInt32LE(text.length, 12);
  out.writeUInt32LE(0x4e4f534a, 16);
  text.copy(out, 20);
  out.writeUInt32LE(data.length, 20 + text.length);
  out.writeUInt32LE(0x004e4942, 24 + text.length);
  data.copy(out, 28 + text.length);
  await writeFile(file, out);
}
export function appendView(doc, bytes, target) {
  const start = doc.bin.length + ((4 - (doc.bin.length % 4)) % 4);
  doc.bin = Buffer.concat([
    doc.bin,
    Buffer.alloc(start - doc.bin.length),
    bytes,
  ]);
  const index = (doc.json.bufferViews ??= []).length;
  doc.json.bufferViews.push({
    buffer: 0,
    byteOffset: start,
    byteLength: bytes.length,
    ...(target ? { target } : {}),
  });
  return index;
}
export function floatAccessor(doc, values, type, count, min, max) {
  const bytes = Buffer.alloc(values.length * 4);
  values.forEach((v, i) => bytes.writeFloatLE(v, i * 4));
  const accessor = {
    bufferView: appendView(doc, bytes),
    componentType: 5126,
    count,
    type,
  };
  if (min) accessor.min = min;
  if (max) accessor.max = max;
  const index = (doc.json.accessors ??= []).length;
  doc.json.accessors.push(accessor);
  return index;
}
export function textureReferences(material, visitor) {
  function visit(object) {
    if (!object || typeof object !== "object") return;
    for (const [key, value] of Object.entries(object)) {
      if (/Texture$/.test(key) && value && typeof value.index === "number")
        visitor(value);
      else if (value && typeof value === "object") visit(value);
    }
  }
  visit(material);
}
// Rebuild only reachable GLTF resources and binary ranges. No Unity scripts or UI
// are retained in the browser asset; sources are never rewritten.
export function compactGLB(doc, roots) {
  const g = structuredClone(doc.json);
  const used = {
    nodes: new Set(),
    meshes: new Set(),
    skins: new Set(),
    materials: new Set(),
    textures: new Set(),
    images: new Set(),
    samplers: new Set(),
    accessors: new Set(),
    bufferViews: new Set(),
  };
  function node(id) {
    if (used.nodes.has(id)) return;
    used.nodes.add(id);
    const n = g.nodes[id];
    for (const child of n.children || []) node(child);
    if (n.mesh !== undefined) used.meshes.add(n.mesh);
    if (n.skin !== undefined) used.skins.add(n.skin);
  }
  roots.forEach(node);
  for (const id of used.skins) {
    const s = g.skins[id];
    s.joints.forEach(node);
    if (s.skeleton !== undefined) node(s.skeleton);
    if (s.inverseBindMatrices !== undefined)
      used.accessors.add(s.inverseBindMatrices);
  }
  for (const id of used.meshes) {
    for (const p of g.meshes[id].primitives) {
      Object.values(p.attributes).forEach((i) => used.accessors.add(i));
      if (p.indices !== undefined) used.accessors.add(p.indices);
      if (p.material !== undefined) used.materials.add(p.material);
      for (const t of p.targets || [])
        Object.values(t).forEach((i) => used.accessors.add(i));
    }
  }
  const animations = (g.animations || [])
    .map((a) => ({
      ...a,
      channels: a.channels.filter((ch) => used.nodes.has(ch.target.node)),
    }))
    .filter((a) => a.channels.length);
  for (const a of animations) {
    const samplers = new Set(a.channels.map((ch) => ch.sampler));
    for (const i of samplers) {
      used.accessors.add(a.samplers[i].input);
      used.accessors.add(a.samplers[i].output);
    }
  }
  for (const id of used.materials)
    textureReferences(g.materials[id], (v) => used.textures.add(v.index));
  for (const id of used.textures) {
    const t = g.textures[id];
    if (t.source !== undefined) used.images.add(t.source);
    if (t.sampler !== undefined) used.samplers.add(t.sampler);
  }
  for (const id of used.images) {
    const img = g.images[id];
    if (img.bufferView !== undefined) used.bufferViews.add(img.bufferView);
  }
  for (const id of used.accessors) {
    const a = g.accessors[id];
    if (a.bufferView !== undefined) used.bufferViews.add(a.bufferView);
    if (a.sparse) {
      used.bufferViews.add(a.sparse.indices.bufferView);
      used.bufferViews.add(a.sparse.values.bufferView);
    }
  }
  const maps = {};
  for (const [key, set] of Object.entries(used)) {
    const ids = [...set].sort((a, b) => a - b);
    maps[key] = new Map(ids.map((id, i) => [id, i]));
  }
  const out = {
    json: {
      asset: {
        version: "2.0",
        generator: "Westeros local-only asset preparation",
      },
      scene: 0,
      scenes: [{ nodes: roots.map((i) => maps.nodes.get(i)) }],
    },
    bin: Buffer.alloc(0),
  };
  for (const [key, map] of Object.entries(maps))
    out.json[key] = [...map.keys()].map((id) => g[key][id]);
  for (const n of out.json.nodes) {
    if (n.children) n.children = n.children.map((i) => maps.nodes.get(i));
    if (n.mesh !== undefined) n.mesh = maps.meshes.get(n.mesh);
    if (n.skin !== undefined) n.skin = maps.skins.get(n.skin);
    delete n.camera;
  }
  for (const m of out.json.meshes)
    for (const p of m.primitives) {
      for (const k of Object.keys(p.attributes))
        p.attributes[k] = maps.accessors.get(p.attributes[k]);
      if (p.indices !== undefined) p.indices = maps.accessors.get(p.indices);
      if (p.material !== undefined) p.material = maps.materials.get(p.material);
      for (const t of p.targets || [])
        for (const k of Object.keys(t)) t[k] = maps.accessors.get(t[k]);
    }
  for (const s of out.json.skins) {
    s.joints = s.joints.map((i) => maps.nodes.get(i));
    if (s.skeleton !== undefined) s.skeleton = maps.nodes.get(s.skeleton);
    if (s.inverseBindMatrices !== undefined)
      s.inverseBindMatrices = maps.accessors.get(s.inverseBindMatrices);
  }
  for (const m of out.json.materials)
    textureReferences(m, (v) => {
      v.index = maps.textures.get(v.index);
    });
  for (const t of out.json.textures) {
    if (t.source !== undefined) t.source = maps.images.get(t.source);
    if (t.sampler !== undefined) t.sampler = maps.samplers.get(t.sampler);
  }
  for (const image of out.json.images)
    if (image.bufferView !== undefined)
      image.bufferView = maps.bufferViews.get(image.bufferView);
  for (const a of out.json.accessors) {
    if (a.bufferView !== undefined)
      a.bufferView = maps.bufferViews.get(a.bufferView);
    if (a.sparse) {
      a.sparse.indices.bufferView = maps.bufferViews.get(
        a.sparse.indices.bufferView,
      );
      a.sparse.values.bufferView = maps.bufferViews.get(
        a.sparse.values.bufferView,
      );
    }
  }
  out.json.bufferViews = [];
  for (const id of maps.bufferViews.keys()) {
    const view = g.bufferViews[id],
      start = view.byteOffset || 0;
    const index = appendView(
      out,
      doc.bin.subarray(start, start + view.byteLength),
      view.target,
    );
    if (view.byteStride)
      out.json.bufferViews[index].byteStride = view.byteStride;
  }
  out.json.animations = animations.map((a) => {
    const indices = [...new Set(a.channels.map((ch) => ch.sampler))].sort(
      (x, y) => x - y,
    );
    const map = new Map(indices.map((id, i) => [id, i]));
    return {
      ...a,
      channels: a.channels.map((ch) => ({
        ...ch,
        sampler: map.get(ch.sampler),
        target: { ...ch.target, node: maps.nodes.get(ch.target.node) },
      })),
      samplers: indices.map((i) => ({
        ...a.samplers[i],
        input: maps.accessors.get(a.samplers[i].input),
        output: maps.accessors.get(a.samplers[i].output),
      })),
    };
  });
  for (const key of Object.keys(out.json))
    if (Array.isArray(out.json[key]) && out.json[key].length === 0)
      delete out.json[key];
  return out;
}
