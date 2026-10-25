import { floatAccessor } from "./gltf.mjs";
import { hermite } from "./unity.mjs";

// Sample original Unity curves into normalized, continuous glTF tracks.
export function appendAnimation(document, root, clip, name) {
  const paths = new Map();
  function walk(index, path) {
    paths.set(path, index);
    for (const child of document.json.nodes[index].children || []) {
      walk(
        child,
        path
          ? `${path}/${document.json.nodes[child].name}`
          : document.json.nodes[child].name,
      );
    }
  }
  walk(root, "");
  const duration = Number(clip.m_AnimationClipSettings.m_StopTime);
  if (!(duration > 0)) throw Error(`Invalid duration for ${name}`);
  const times = Array.from({ length: Math.ceil(duration * 60) + 1 }, (_, i) =>
    Math.min(i / 60, duration),
  );
  const input = floatAccessor(
    document,
    times,
    "SCALAR",
    times.length,
    [0],
    [duration],
  );
  const animation = {
    name,
    channels: [],
    samplers: [],
    extras: {
      sourceClip: clip.m_Name,
      sourceFrameRate: Number(clip.m_SampleRate),
    },
  };
  for (const [section, path, components] of [
    ["m_RotationCurves", "rotation", ["x", "y", "z", "w"]],
    ["m_PositionCurves", "translation", ["x", "y", "z"]],
    ["m_ScaleCurves", "scale", ["x", "y", "z"]],
  ]) {
    for (const curve of clip[section] || []) {
      const target = paths.get(curve.path);
      if (target === undefined)
        throw Error(`Unmapped animation target: ${curve.path}`);
      const values = [];
      let previous;
      for (const time of times) {
        let value = components.map((key) =>
          hermite(curve.curve.m_Curve, time, key),
        );
        if (path === "translation") value[0] *= -1;
        if (path === "rotation") {
          value[1] *= -1;
          value[2] *= -1;
          const length = Math.hypot(...value);
          if (length < 1e-8) throw Error(`Zero quaternion in ${name}`);
          value = value.map((v) => v / length);
          if (
            previous &&
            value.reduce((sum, v, i) => sum + v * previous[i], 0) < 0
          )
            value = value.map((v) => -v);
          previous = value;
        }
        values.push(...value);
      }
      const output = floatAccessor(
        document,
        values,
        path === "rotation" ? "VEC4" : "VEC3",
        times.length,
      );
      animation.channels.push({
        sampler: animation.samplers.length,
        target: { node: target, path },
      });
      animation.samplers.push({ input, output, interpolation: "LINEAR" });
    }
  }
  (document.json.animations ??= []).push(animation);
  return { name, duration, tracks: animation.channels.length };
}
