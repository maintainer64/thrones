import { useEffect, useMemo } from "react";
import { useGLTF } from "@react-three/drei";
import { Box3, Mesh, MeshStandardMaterial, Vector3 } from "three";
import type { GameManifest } from "./game-state";
import type { Landmark } from "./landmarks";
import type { Relief } from "./terrain";

export function LandmarkFigure({
  landmark,
  manifest,
  relief,
  onSelect,
}: {
  landmark: Landmark;
  manifest: GameManifest;
  relief: Relief;
  onSelect: () => void;
}) {
  const source = useGLTF(manifest.landmarks![landmark.kind]);
  const { model, scale, center, floor, materials } = useMemo(() => {
    const model = source.scene.clone(true);
    const materials: MeshStandardMaterial[] = [];
    model.traverse((object) => {
      if (!(object instanceof Mesh)) return;
      object.castShadow = true;
      object.receiveShadow = true;
      // Share immutable geometry/textures, own only the material instances.
      const own = (material: MeshStandardMaterial) => {
        const copy = material.clone();
        copy.metalness = 0.08;
        copy.roughness = 0.8;
        materials.push(copy);
        return copy;
      };
      object.material = Array.isArray(object.material)
        ? object.material.map(own)
        : own(object.material as MeshStandardMaterial);
    });
    const bounds = new Box3().setFromObject(model),
      size = bounds.getSize(new Vector3());
    const scale =
      (landmark.kind === "port"
        ? 0.55
        : landmark.kind === "stronghold"
          ? 0.62
          : 0.5) / Math.max(size.x, size.z);
    return {
      model,
      scale,
      center: bounds.getCenter(new Vector3()),
      floor: bounds.min.y,
      materials,
    };
  }, [source.scene, landmark.kind]);
  useEffect(
    () => () => materials.forEach((material) => material.dispose()),
    [materials],
  );
  const [x, z] = landmark.point;
  return (
    <group
      position={[x, relief.height(x, z) + 0.012, z]}
      rotation={[0, landmark.rotation || 0, 0]}
      onClick={(event) => {
        if (event.delta < 5) {
          event.stopPropagation();
          onSelect();
        }
      }}
    >
      <group
        scale={scale}
        position={[-center.x * scale, -floor * scale, -center.z * scale]}
      >
        <primitive object={model} />
      </group>
    </group>
  );
}
