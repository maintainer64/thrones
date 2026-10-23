import { useEffect, useRef } from "react";
import type { ComponentRef } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import { OrbitControls } from "@react-three/drei";
import { MOUSE, TOUCH, PerspectiveCamera, Vector3 } from "three";
import type { Point2 } from "./game-state";
import type { Relief } from "./terrain";

export type CameraPose = {
  polar: string;
  azimuth: string;
  target: string;
  distance: number;
};
export type CameraRequest = {
  view: "table" | "focus" | "action" | "in" | "out";
  serial: number;
  rotation: number;
};
export const TABLE_POLAR = Math.PI / 4;

export default function TableCamera({
  width,
  depth,
  relief,
  focus,
  points,
  request,
  onPose,
  instant = false,
}: {
  width: number;
  depth: number;
  relief: Relief;
  focus: Point2;
  points: Point2[];
  request: CameraRequest;
  onPose: (pose: CameraPose) => void;
  instant?: boolean;
}) {
  const controls = useRef<ComponentRef<typeof OrbitControls>>(null);
  const { camera, size } = useThree();
  const goal = useRef<{ eye: Vector3; target: Vector3 } | undefined>(undefined);
  const lastReport = useRef(0);
  useEffect(() => {
    if ((request.view === "in" || request.view === "out") && controls.current) {
      const target = controls.current.target.clone(),
        offset = camera.position.clone().sub(target);
      const distance = Math.max(
        2.2,
        Math.min(65, offset.length() * (request.view === "in" ? 0.72 : 1.38)),
      );
      goal.current = {
        target,
        eye: target.clone().addScaledVector(offset.normalize(), distance),
      };
      return;
    }
    const angle = Math.atan2(12, -15) + request.rotation;
    const direction = new Vector3(
      Math.sin(angle) * Math.sin(TABLE_POLAR),
      Math.cos(TABLE_POLAR),
      Math.cos(angle) * Math.sin(TABLE_POLAR),
    );
    let minX = -width / 2 - 0.3,
      maxX = width / 2 + 0.3,
      minZ = -depth / 2 - 0.3,
      maxZ = depth / 2 + 0.3;
    if (request.view === "focus") {
      minX = focus[0] - 2;
      maxX = focus[0] + 2;
      minZ = focus[1] - 2;
      maxZ = focus[1] + 2;
    }
    if (request.view === "action") {
      const all = [focus, ...points];
      minX = Math.min(...all.map((p) => p[0])) - 1;
      maxX = Math.max(...all.map((p) => p[0])) + 1;
      minZ = Math.min(...all.map((p) => p[1])) - 1;
      maxZ = Math.max(...all.map((p) => p[1])) + 1;
    }
    const target = new Vector3((minX + maxX) / 2, 0.15, (minZ + maxZ) / 2);
    target.y = Math.max(0.15, relief.height(target.x, target.z) * 0.5);
    const right = new Vector3(0, 1, 0).cross(direction).normalize(),
      up = direction.clone().cross(right).normalize();
    const tanV = Math.tan(((camera as PerspectiveCamera).fov * Math.PI) / 360),
      tanH = (tanV * size.width) / size.height;
    let distance = 2.2;
    for (const x of [minX, maxX])
      for (const z of [minZ, maxZ])
        for (const y of [0, 0.8]) {
          const corner = new Vector3(x, y, z).sub(target);
          distance = Math.max(
            distance,
            corner.dot(direction) + Math.abs(corner.dot(right)) / (tanH * 0.92),
            corner.dot(direction) + Math.abs(corner.dot(up)) / (tanV * 0.9),
          );
        }
    goal.current = {
      target,
      eye: target.clone().addScaledVector(direction, distance),
    };
    // Coordinates are sampled on explicit requests/resizes, never on hover.
  }, [request, size.width, size.height, camera, width, depth]);
  useFrame(({ clock }, delta) => {
    const orbit = controls.current;
    if (!orbit) return;
    if (goal.current) {
      const amount = instant ? 1 : 1 - Math.exp(-delta * 7);
      camera.position.lerp(goal.current.eye, amount);
      orbit.target.lerp(goal.current.target, amount);
      orbit.update();
      if (camera.position.distanceTo(goal.current.eye) < 0.01)
        goal.current = undefined;
    }
    const old = orbit.target.clone();
    orbit.target.x = Math.max(
      -width / 2 - 1,
      Math.min(width / 2 + 1, orbit.target.x),
    );
    orbit.target.z = Math.max(
      -depth / 2 - 1,
      Math.min(depth / 2 + 1, orbit.target.z),
    );
    const minimum = relief.height(orbit.target.x, orbit.target.z) * 0.5;
    orbit.target.y = Math.max(minimum, Math.min(0.5, orbit.target.y));
    camera.position.add(orbit.target.clone().sub(old));
    if (clock.elapsedTime - lastReport.current > 0.3) {
      lastReport.current = clock.elapsedTime;
      onPose({
        polar: orbit.getPolarAngle().toFixed(3),
        azimuth: orbit.getAzimuthalAngle().toFixed(3),
        target: orbit.target
          .toArray()
          .map((n) => n.toFixed(3))
          .join(","),
        distance: orbit.getDistance(),
      });
    }
  });
  return (
    <OrbitControls
      ref={controls}
      makeDefault
      minDistance={2.2}
      maxDistance={65}
      minPolarAngle={TABLE_POLAR}
      maxPolarAngle={TABLE_POLAR}
      screenSpacePanning={false}
      zoomToCursor
      enableDamping
      dampingFactor={0.12}
      mouseButtons={{
        LEFT: MOUSE.PAN,
        MIDDLE: MOUSE.DOLLY,
        RIGHT: MOUSE.ROTATE,
      }}
      touches={{ ONE: TOUCH.PAN, TWO: TOUCH.DOLLY_ROTATE }}
      onStart={() => {
        goal.current = undefined;
      }}
    />
  );
}
