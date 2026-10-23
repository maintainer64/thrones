import { useEffect, useMemo } from "react";
import { DoubleSide, Float32BufferAttribute } from "three";
import type { BufferGeometry } from "three";

// Dragonstone is printed over sea-coloured map art. Give its actual land collider
// an explicit volcanic surface instead of merely lifting that blue map texture.
// This is authored terrain, not a recovered mesh/height map from the original game.
export function islandGeometry(source?: BufferGeometry) {
  if (!source) return undefined;
  const geometry = source.clone().translate(0, -0.006, 0),
    positions = geometry.getAttribute("position");
  const colors: number[] = [];
  for (let i = 0; i < positions.count; i++) {
    const x = positions.getX(i),
      z = positions.getZ(i),
      h = positions.getY(i);
    const grain = 0.025 * Math.sin(x * 29 + z * 17) * Math.cos(z * 23);
    const rock = 0.32 + Math.min(0.15, h * 0.35) + grain;
    colors.push(rock + 0.07, rock + 0.055, rock);
  }
  geometry.setAttribute("color", new Float32BufferAttribute(colors, 3));
  geometry.computeVertexNormals();
  return geometry;
}
export function IslandTerrain({ source }: { source?: BufferGeometry }) {
  const geometry = useMemo(() => islandGeometry(source), [source]);
  useEffect(() => () => geometry?.dispose(), [geometry]);
  if (!geometry) return null;
  return (
    <mesh geometry={geometry} receiveShadow raycast={() => {}}>
      <meshStandardMaterial
        vertexColors
        side={DoubleSide}
        roughness={0.97}
        metalness={0}
      />
    </mesh>
  );
}
