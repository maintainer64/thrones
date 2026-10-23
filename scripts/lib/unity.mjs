// Unity animation curves use Hermite tangents in units per second.
// This pure helper deliberately has no dependency on the local asset exports.
export function hermite(keys, time, component) {
  if (!keys.length) throw new Error("Animation curve has no keys");
  if (time <= Number(keys[0].time)) return Number(keys[0].value[component]);
  if (time >= Number(keys.at(-1).time))
    return Number(keys.at(-1).value[component]);
  const end = keys.findIndex((key) => Number(key.time) > time);
  const a = keys[end - 1],
    b = keys[end];
  const duration = Number(b.time) - Number(a.time);
  const t = (time - Number(a.time)) / duration;
  const av = Number(a.value[component]),
    bv = Number(b.value[component]);
  const out = Number(a.outSlope?.[component] || 0);
  const incoming = Number(b.inSlope?.[component] || 0);
  if (!Number.isFinite(out) || !Number.isFinite(incoming)) return av;
  return (
    (2 * t * t * t - 3 * t * t + 1) * av +
    (t * t * t - 2 * t * t + t) * duration * out +
    (-2 * t * t * t + 3 * t * t) * bv +
    (t * t * t - t * t) * duration * incoming
  );
}
