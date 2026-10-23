export function regionKey(name) {
  if (name.endsWith("_Port")) name = `Port of ${name.slice(0, -5)}`;
  const key = name.toLowerCase().replace(/[^a-z]/g, "");
  return (
    { seaguard: "seagard", redwynestraits: "redwynestraights" }[key] || key
  );
}

export function matchRegions(regions, nodes) {
  const byName = new Map(nodes.map((node) => [regionKey(node.name), node]));
  const matched = new Map();
  for (const region of regions) {
    const node = byName.get(regionKey(region.name));
    if (!node) throw Error(`Original map has no region ${region.id}`);
    matched.set(region.id, node);
  }
  if (new Set(matched.values()).size !== regions.length)
    throw Error("Ambiguous region mapping");
  return matched;
}
