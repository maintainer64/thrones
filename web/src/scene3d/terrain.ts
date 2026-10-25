export type XZ = [number, number];
export type Triangle = [XZ, XZ, XZ];
export type Ridge = {
  x: number;
  z: number;
  height: number;
  width: number;
  length: number;
  angle?: number;
};
export type Relief = {
  width: number;
  depth: number;
  columns: number;
  rows: number;
  values: Float32Array;
  height: (x: number, z: number) => number;
};

export function containsTriangle(point: XZ, triangle: Triangle) {
  const [a, b, c] = triangle;
  const cross = (a: XZ, b: XZ, p: XZ) =>
    (b[0] - a[0]) * (p[1] - a[1]) - (b[1] - a[1]) * (p[0] - a[0]);
  if (Math.abs(cross(a, b, c)) < 1e-10) return false;
  const signs = [cross(a, b, point), cross(b, c, point), cross(c, a, point)];
  return (
    signs.every((value) => value >= -1e-7) ||
    signs.every((value) => value <= 1e-7)
  );
}

// Stylized topography, not a recovered height map. Coastlines use original collider
// polygons; broad authored ridges are registered to geographic landmarks.
export function createRelief(
  width: number,
  depth: number,
  land: Triangle[],
  ports: Triangle[],
  ridges: Ridge[],
  columns = 192,
  rows = 384,
): Relief {
  const stride = columns + 1,
    size = stride * (rows + 1),
    mask = new Float32Array(size);
  const dx = width / columns,
    dz = depth / rows;
  function raster(triangles: Triangle[], value: number) {
    for (const triangle of triangles) {
      const minX = Math.max(
        0,
        Math.ceil((Math.min(...triangle.map((p) => p[0])) + width / 2) / dx),
      );
      const maxX = Math.min(
        columns,
        Math.floor((Math.max(...triangle.map((p) => p[0])) + width / 2) / dx),
      );
      const minZ = Math.max(
        0,
        Math.ceil((Math.min(...triangle.map((p) => p[1])) + depth / 2) / dz),
      );
      const maxZ = Math.min(
        rows,
        Math.floor((Math.max(...triangle.map((p) => p[1])) + depth / 2) / dz),
      );
      for (let z = minZ; z <= maxZ; z++)
        for (let x = minX; x <= maxX; x++)
          if (
            containsTriangle([x * dx - width / 2, z * dz - depth / 2], triangle)
          )
            mask[z * stride + x] = value;
    }
  }
  raster(land, 1);
  raster(ports, 0);
  let coast = mask.slice();
  for (let pass = 0; pass < 3; pass++) {
    const next = new Float32Array(size);
    for (let z = 0; z <= rows; z++)
      for (let x = 0; x <= columns; x++) {
        let sum = 0;
        for (let oz = -1; oz <= 1; oz++)
          for (let ox = -1; ox <= 1; ox++)
            sum +=
              coast[
                Math.min(rows, Math.max(0, z + oz)) * stride +
                  Math.min(columns, Math.max(0, x + ox))
              ];
        next[z * stride + x] = sum / 9;
      }
    coast = next;
  }
  const values = new Float32Array(size);
  for (let iz = 0; iz <= rows; iz++)
    for (let ix = 0; ix <= columns; ix++) {
      const index = iz * stride + ix;
      if (!mask[index]) continue;
      const x = ix * dx - width / 2,
        z = iz * dz - depth / 2;
      let height = 0.09;
      for (const ridge of ridges) {
        const angle = ridge.angle || 0,
          c = Math.cos(angle),
          s = Math.sin(angle),
          rx = x - ridge.x,
          rz = z - ridge.z;
        const a = (rx * c + rz * s) / ridge.width,
          b = (-rx * s + rz * c) / ridge.length;
        height += ridge.height * Math.exp(-(a * a + b * b) * 1.7);
      }
      height +=
        0.012 * (1 + Math.sin(x * 6.7 + z * 2.3) * Math.cos(z * 5.1 - x * 1.2));
      values[index] = Math.min(0.8, height) * coast[index];
    }
  const height = (x: number, z: number) => {
    const gx = Math.max(0, Math.min(columns, (x + width / 2) / dx)),
      gz = Math.max(0, Math.min(rows, (z + depth / 2) / dz));
    const x0 = Math.min(columns - 1, Math.floor(gx)),
      z0 = Math.min(rows - 1, Math.floor(gz)),
      tx = gx - x0,
      tz = gz - z0;
    const a =
      values[z0 * stride + x0] * (1 - tx) + values[z0 * stride + x0 + 1] * tx;
    const b =
      values[(z0 + 1) * stride + x0] * (1 - tx) +
      values[(z0 + 1) * stride + x0 + 1] * tx;
    return a * (1 - tz) + b * tz;
  };
  return { width, depth, columns, rows, values, height };
}

// Sea colliders in Unity continue beyond the decorative board. Clip both
// interaction and outlines to the table before subdividing them for relief.
export function clipTriangle(
  triangle: Triangle,
  width: number,
  depth: number,
): Triangle[] {
  let polygon: XZ[] = [...triangle];
  for (const [axis, limit, sign] of [
    [0, -width / 2, 1],
    [0, width / 2, -1],
    [1, -depth / 2, 1],
    [1, depth / 2, -1],
  ] as const) {
    const next: XZ[] = [];
    for (let i = 0; i < polygon.length; i++) {
      const a = polygon[i],
        b = polygon[(i + 1) % polygon.length],
        insideA = (a[axis] - limit) * sign >= -1e-9,
        insideB = (b[axis] - limit) * sign >= -1e-9;
      if (insideA) next.push(a);
      if (insideA !== insideB) {
        const t = (limit - a[axis]) / (b[axis] - a[axis]);
        next.push([a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t]);
      }
    }
    polygon = next;
    if (polygon.length < 3) return [];
  }
  const result: Triangle[] = [];
  for (let i = 1; i < polygon.length - 1; i++) {
    const triangle: Triangle = [polygon[0], polygon[i], polygon[i + 1]],
      [a, b, c] = triangle;
    if (
      Math.abs((b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0])) >
      1e-10
    )
      result.push(triangle);
  }
  return result;
}

export function boundaryEdges(triangles: Triangle[]): [XZ, XZ][] {
  const edges = new Map<string, { a: XZ; b: XZ; count: number }>();
  const key = (p: XZ) => p.map((value) => Math.round(value * 10000)).join(",");
  for (const triangle of triangles)
    for (let i = 0; i < 3; i++) {
      const a = triangle[i],
        b = triangle[(i + 1) % 3],
        ka = key(a),
        kb = key(b);
      if (ka === kb) continue;
      const id = ka < kb ? `${ka}/${kb}` : `${kb}/${ka}`,
        existing = edges.get(id);
      if (existing) existing.count++;
      else edges.set(id, { a, b, count: 1 });
    }
  return [...edges.values()]
    .filter((edge) => edge.count === 1)
    .map((edge) => [edge.a, edge.b]);
}

export function subdivideTriangle(
  triangle: Triangle,
  emit: (point: XZ) => void,
  maxEdge = 0.24,
  depth = 0,
) {
  const lengths = triangle.map((a, i) => {
    const b = triangle[(i + 1) % 3];
    return (a[0] - b[0]) ** 2 + (a[1] - b[1]) ** 2;
  });
  const longest = Math.max(...lengths);
  if (longest <= maxEdge * maxEdge || depth >= 16) {
    triangle.forEach(emit);
    return;
  }
  const index = lengths.indexOf(longest),
    a = triangle[index],
    b = triangle[(index + 1) % 3],
    c = triangle[(index + 2) % 3];
  const middle: XZ = [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
  subdivideTriangle([a, middle, c], emit, maxEdge, depth + 1);
  subdivideTriangle([middle, b, c], emit, maxEdge, depth + 1);
}
