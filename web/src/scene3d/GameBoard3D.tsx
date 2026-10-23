import { t } from "../i18n/core";
import { OrderToken } from "../ui/OrderToken";
import {
  Component,
  Suspense,
  useEffect,
  useMemo,
  useRef,
  useState,
  useCallback,
} from "react";
import type { ReactNode, RefObject } from "react";
import {
  ReplayPhase,
  advanceReplay,
  replayActors,
  replayPieces,
  replayPose,
  replayRoom,
  type CombatPlayback,
  type ReplayActor,
} from "../combat/animation";
import { Canvas, useFrame, useThree } from "@react-three/fiber";
import { Html, Line, RoundedBox, useGLTF, useTexture } from "@react-three/drei";
import {
  AnimationMixer,
  Box3,
  DoubleSide,
  Group,
  Mesh,
  MeshStandardMaterial,
  SRGBColorSpace,
  SkinnedMesh,
  Vector3,
} from "three";
import { clone } from "three/addons/utils/SkeletonUtils.js";
import {
  Focus,
  Layers,
  LocateFixed,
  RotateCw,
  Plus,
  Minus,
  ArrowRight,
  Flag,
  Shield,
  Tag,
} from "lucide-react";
import type { Assets, Catalog, Room } from "../types";
import { FortificationBadge } from "../fortifications";
import { regionName } from "../types";
import type {
  GameManifest,
  Piece,
  Point2,
  Presentation,
  MapAction,
  RetreatAction,
} from "./game-state";
import { formation, lastRoute, reconcilePieces } from "./game-state";
import TableCamera from "./TableCamera";
import type { CameraPose, CameraRequest } from "./TableCamera";
import { createSurfaceData, terrainPlane } from "./surface-data";
import type { SurfaceData } from "./surface-data";
import type { Relief } from "./terrain";
import { TouchSelection } from "./touch-selection";
import { placeLandmarks } from "./landmarks";
import { LandmarkFigure } from "./LandmarkFigure";
import { IslandTerrain } from "./IslandTerrain";
import "./game-board.css";
type Props = {
  manifest: GameManifest;
  catalog: Catalog;
  room: Room;
  assets: Assets;
  selected: string;
  target?: string;
  onSelect: (id: string) => void;
  onTarget?: (id: string) => void;
  motion: boolean;
  onRetry: () => void;
  frameRequest?: number;
  paused?: boolean;
  action?: MapAction;
  retreat?: RetreatAction;
  battleFocus?: { from: string; to: string; house: string };
  combatPlayback?: CombatPlayback & {
    motion: boolean;
    onComplete: (serial: number) => void;
  };
};
class BoardBoundary extends Component<
  {
    children: ReactNode;
    onRetry: () => void;
    onError: () => void;
  },
  {
    error: string;
  }
> {
  state = { error: "" };
  static getDerivedStateFromError(error: Error) {
    return { error: error.message };
  }
  componentDidCatch() {
    this.props.onError();
  }
  render() {
    return this.state.error ? (
      <div className="game3d-failure">
        <Shield />
        <h3>{t("assets.boardUnavailable")}</h3>
        <p>{this.state.error}</p>
        <button onClick={this.props.onRetry}>{t("assets.retry3d")}</button>
      </div>
    ) : (
      this.props.children
    );
  }
}
function ArmyFigure({
  piece,
  point,
  manifest,
  color,
  motion,
  onSelect,
  relief,
  onFocus,
  replay,
}: {
  piece: Piece;
  point: Point2;
  manifest: GameManifest;
  color: string;
  motion: boolean;
  onSelect: () => void;
  relief: Relief;
  onFocus: () => void;
  replay?: {
    actor: ReplayActor;
    progress: RefObject<number>;
    phase: ReplayPhase;
    start: Point2;
    end: Point2;
    retreat?: Point2;
    playing: boolean;
  };
}) {
  const definition = manifest.units[piece.type];
  const gltf = useGLTF(definition.url);
  const texture = useTexture(definition.textures[piece.house]);
  const { actor, scale, center, floor, materials } = useMemo(() => {
    const actor = clone(gltf.scene);
    const materials: MeshStandardMaterial[] = [];
    texture.flipY = false;
    texture.colorSpace = SRGBColorSpace;
    texture.needsUpdate = true;
    actor.updateMatrixWorld(true);
    actor.traverse((object) => {
      if (object instanceof Mesh) {
        object.castShadow = true;
        object.receiveShadow = true;
        object.frustumCulled = false;
        const replace = (material: MeshStandardMaterial) => {
          const own = material.clone();
          if (/footman|knight|ship|siegeengine/i.test(own.name))
            own.map = texture;
          // Missing glTF factors default to metalness=1, which darkens these painted pieces.
          own.metalness = Math.min(own.metalness, 0.12);
          own.roughness = 0.65;
          if (replay) own.transparent = true;
          own.userData.replayOpacity = own.opacity;
          materials.push(own);
          return own;
        };
        object.material = Array.isArray(object.material)
          ? object.material.map(replace)
          : replace(object.material as MeshStandardMaterial);
      }
      if (object instanceof SkinnedMesh) object.skeleton.update();
    });
    const box = new Box3().setFromObject(actor, true),
      size = box.getSize(new Vector3()),
      center = box.getCenter(new Vector3());
    const scale =
      (piece.type === "ship" ? 0.7 : 0.65) / Math.max(size.x, size.y, size.z);
    return { actor, scale, center, floor: box.min.y, materials };
  }, [gltf.scene, texture, piece.type, !!replay]);
  const mixer = useMemo(() => new AnimationMixer(actor), [actor]);
  const actions = useMemo(
    () =>
      Object.fromEntries(
        gltf.animations.map((clip) => [clip.name, mixer.clipAction(clip)]),
      ),
    [gltf.animations, mixer],
  );
  const group = useRef<Group>(null);
  const location = useRef(
    new Vector3(point[0], relief.height(...point) + 0.025, point[1]),
  );
  const current = useRef("");
  useEffect(
    () => () => {
      mixer.stopAllAction();
      mixer.uncacheRoot(actor);
      materials.forEach((m) => m.dispose());
      actor.traverse((object) => {
        if (object instanceof SkinnedMesh) object.skeleton.dispose();
      });
    },
    [actor, mixer, materials],
  );
  useFrame((_, delta) => {
    const pose = replay
      ? replayPose(replay.actor, replay.phase, replay.progress.current)
      : undefined;
    let destination: Point2 = point;
    if (replay && pose) {
      const x =
        replay.start[0] + (replay.end[0] - replay.start[0]) * pose.travel;
      const z =
        replay.start[1] + (replay.end[1] - replay.start[1]) * pose.travel;
      destination = replay.retreat
        ? [
            x + (replay.retreat[0] - x) * pose.retreat,
            z + (replay.retreat[1] - z) * pose.retreat,
          ]
        : [x, z];
    }
    const target = new Vector3(
      destination[0],
      location.current.y,
      destination[1],
    );
    const distance = Math.hypot(
        location.current.x - destination[0],
        location.current.z - destination[1],
      ),
      moving =
        motion &&
        distance > (replay ? 0.00001 : 0.015) &&
        (!replay || replay.playing);
    const name = moving && actions.run ? "run" : "idle";
    if (current.current !== name) {
      if (current.current) actions[current.current]?.fadeOut(0.15);
      actions[name]?.reset().fadeIn(0.15).play();
      current.current = name;
    }
    if (moving) {
      const direction = target.clone().sub(location.current).normalize();
      if (replay) location.current.copy(target);
      else
        location.current.addScaledVector(
          direction,
          Math.min(distance, Math.min(delta, 0.06) * 2.4),
        );
      if (group.current)
        group.current.rotation.y =
          Math.atan2(direction.x, direction.z) + Math.PI;
    } else location.current.copy(target);
    location.current.y =
      relief.height(location.current.x, location.current.z) + 0.025;
    if (group.current) group.current.position.copy(location.current);
    if (replay && pose && group.current) {
      group.current.visible = pose.fade < 0.999;
      group.current.scale.setScalar(1 - pose.fade * 0.65);
      group.current.rotation.z = -pose.fade * 0.9;
      group.current.position.y -= pose.fade * 0.12;
      for (const material of materials)
        material.opacity = material.userData.replayOpacity * (1 - pose.fade);
    }
    if (motion) mixer.update(Math.min(delta, 0.06));
  });
  return (
    <group
      ref={group}
      rotation={[0, Math.PI, 0]}
      onDoubleClick={(e) => {
        e.stopPropagation();
        onFocus();
      }}
      onClick={(e) => {
        if (e.delta < 5) {
          e.stopPropagation();
          onSelect();
        }
      }}
    >
      <group
        scale={scale}
        position={[-center.x * scale, -floor * scale, -center.z * scale]}
      >
        <primitive object={actor} />
      </group>
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.006, 0]}>
        <ringGeometry args={[0.14, 0.175, 32]} />
        <meshBasicMaterial
          color={piece.battle ? "#e87563" : color}
          transparent
          opacity={0.85}
          depthWrite={false}
        />
      </mesh>
    </group>
  );
}
function RegionSurfaces({
  data,
  selected,
  target,
  active,
  candidates,
  hovered,
  borders,
  blocked,
  onSelect,
  onHover,
  onFocus,
}: {
  data: SurfaceData;
  selected: string;
  target?: string;
  active: Set<string>;
  candidates: Set<string>;
  hovered: string;
  borders: boolean;
  blocked: Set<string>;
  onSelect: (id: string) => void;
  onHover: (id: string) => void;
  onFocus: (id: string) => void;
}) {
  return (
    <group>
      {Object.entries(data.surfaces).map(([id, geometry]) => {
        const chosen = id === selected,
          goal = id === target;
        const over = id === hovered,
          eligible = candidates.has(id);
        const color = blocked.has(id)
          ? "#9b4840"
          : goal
            ? "#83f0c9"
            : chosen
              ? "#f2cc78"
              : eligible
                ? "#59d6b6"
                : over
                  ? "#e5dfc7"
                  : "#819a97";
        return (
          <group key={id}>
            <mesh
              geometry={geometry}
              onClick={(e) => {
                if (e.delta < 5) {
                  e.stopPropagation();
                  onSelect(id);
                }
              }}
              onPointerOver={(e) => {
                e.stopPropagation();
                onHover(id);
              }}
              onPointerOut={() => onHover("")}
              onDoubleClick={(e) => {
                e.stopPropagation();
                onFocus(id);
              }}
            >
              <meshBasicMaterial
                color={color}
                transparent
                opacity={
                  goal
                    ? 0.24
                    : chosen
                      ? 0.1
                      : blocked.has(id)
                        ? 0.13
                        : eligible
                          ? 0.07
                          : over
                            ? 0.09
                            : 0
                }
                depthWrite={false}
                side={DoubleSide}
                polygonOffset
                polygonOffsetFactor={-2}
                polygonOffsetUnits={-2}
              />
            </mesh>
            {(borders || chosen || goal || eligible || over) &&
              data.borders[id].length > 0 && (
                <Line
                  points={data.borders[id]}
                  segments
                  color={color}
                  lineWidth={
                    chosen || goal ? 2.4 : eligible || over ? 1.7 : 0.75
                  }
                  transparent
                  opacity={
                    chosen || goal || eligible || over
                      ? 0.95
                      : active.has(id)
                        ? 0.6
                        : 0.3
                  }
                  depthWrite={false}
                  raycast={() => {}}
                />
              )}
          </group>
        );
      })}
    </group>
  );
}
function MapModel({
  manifest,
  relief,
}: {
  manifest: GameManifest;
  relief: Relief;
}) {
  const { scene } = useGLTF(manifest.board.url);
  const map = useMemo(() => {
    const result = clone(scene);
    result.updateMatrixWorld(true);
    result.traverse((n) => {
      if (n instanceof Mesh) {
        const minZ = n.name.includes("North") ? 0 : -manifest.board.depth / 2;
        const maxZ = n.name.includes("North") ? manifest.board.depth / 2 : 0;
        const geometry = terrainPlane(
          manifest.board.width,
          minZ,
          maxZ,
          relief.height,
        );
        // Generated positions are already in world space; remove the source plane transform.
        n.geometry = geometry;
        n.position.set(0, 0, 0);
        n.quaternion.identity();
        n.scale.set(1, 1, 1);
        n.matrix.identity();
        n.matrixAutoUpdate = true;
        const material = (n.material as MeshStandardMaterial).clone();
        material.roughness = 0.93;
        material.metalness = 0;
        n.material = material;
        n.receiveShadow = true;
        n.castShadow = true;
      }
    });
    return result;
  }, [scene, relief, manifest.board.depth, manifest.board.width]);
  useEffect(
    () => () => {
      map.traverse((object) => {
        if (object instanceof Mesh) {
          object.geometry.dispose();
          (object.material as MeshStandardMaterial).dispose();
        }
      });
    },
    [map],
  );
  return (
    <group>
      <primitive object={map} />
      <RoundedBox
        args={[manifest.board.width + 0.18, 0.28, manifest.board.depth + 0.18]}
        radius={0.05}
        position={[0, -0.16, 0]}
        receiveShadow
      >
        <meshStandardMaterial color="#332d23" roughness={0.75} />
      </RoundedBox>
      <mesh position={[0, -0.32, 0]} receiveShadow>
        <boxGeometry
          args={[manifest.board.width + 0.3, 0.05, manifest.board.depth + 0.3]}
        />
        <meshStandardMaterial color="#b39a61" metalness={0.6} roughness={0.4} />
      </mesh>
    </group>
  );
}
function SceneContent({
  props,
  presentation,
  labels,
  request,
  onHover,
  onLoaded,
  hovered,
  borders,
  onFocus,
  onPose,
}: {
  props: Props;
  presentation: Presentation;
  labels: boolean;
  request: CameraRequest;
  onHover: (id: string) => void;
  onLoaded: (stats: { height: number; borders: number }) => void;
  hovered: string;
  borders: boolean;
  onFocus: (id: string) => void;
  onPose: (pose: CameraPose) => void;
}) {
  const {
    manifest,
    catalog,
    room,
    selected,
    target: moveTarget,
    onSelect,
    onTarget,
    motion,
    assets,
    action,
    retreat,
  } = props;
  const playback = props.combatPlayback;
  const actors = useMemo(
    () => (playback ? replayActors(playback.record) : []),
    [playback?.record],
  );
  const visiblePieces = playback
    ? replayPieces(playback.record, playback.phase, playback.complete)
    : presentation.pieces;
  const progress = useRef(0);
  const replayKey = playback ? `${room.id}:${playback.serial}` : "";
  const clock = useRef({
    key: replayKey,
    done: false,
    last: undefined as number | undefined,
  });
  if (clock.current.key !== replayKey) {
    clock.current = { key: replayKey, done: false, last: undefined };
    progress.current = 0;
  }
  useEffect(() => {
    clock.current.last = undefined;
  }, [replayKey, playback?.playing, props.paused]);
  useEffect(() => {
    const reset = () => {
      clock.current.last = undefined;
    };
    document.addEventListener("visibilitychange", reset);
    return () => document.removeEventListener("visibilitychange", reset);
  }, []);
  useFrame(() => {
    if (!playback) return;
    const now = performance.now();
    const elapsed =
      clock.current.last === undefined ? 0 : (now - clock.current.last) / 1000;
    clock.current.last = now;
    if (playback.complete || !playback.motion) progress.current = 1;
    else if (
      playback.playing &&
      !props.paused &&
      document.visibilityState === "visible"
    )
      progress.current = advanceReplay(
        progress.current,
        elapsed,
        playback.phase,
      );
    if (progress.current >= 1 && !clock.current.done) {
      clock.current.done = true;
      if (!playback.complete) playback.onComplete(playback.serial);
    }
  }, -1);
  const starts = actors.map((actor) => ({
    ...actor.piece,
    region: actor.origin,
    battle: false,
  }));
  const ends = actors.map((actor) => actor.piece);
  const retreats = actors
    .filter((actor) => !actor.lost)
    .map((actor) => ({
      ...actor.piece,
      region: actor.retreatTo || actor.destination,
      battle: false,
    }));
  const target = retreat ? retreat.target : moveTarget;
  const { scene: polygons } = useGLTF(manifest.picking);
  const data = useMemo(
    () => createSurfaceData(polygons, manifest, catalog.board.regions),
    [polygons, manifest, catalog.board.regions],
  );
  useEffect(
    () => () => {
      Object.values(data.surfaces).forEach((geometry) => geometry.dispose());
    },
    [data],
  );
  const candidates = new Set(
    retreat
      ? retreat.targets
      : action?.selecting && !action.loading && !action.error
        ? action.targets
        : [],
  );
  const landmarks = useMemo(
    () =>
      manifest.landmarks
        ? placeLandmarks(catalog.board.regions, manifest, data.triangles)
        : [],
    [manifest, catalog.board.regions, data],
  );
  const choose = (id: string) => {
    if (playback) return;
    if (retreat) {
      if (candidates.has(id)) retreat.onTarget(id);
      else onSelect(id);
    } else if (onTarget && id !== selected) {
      if (candidates.has(id)) onTarget(id);
      else if (active.has(id)) onSelect(id);
    } else onSelect(id);
  };
  const active = new Set(
    room.match?.activeHouse === room.yourHouse ? room.legalRegions || [] : [],
  );
  const controls = useThree((state) => state.controls);
  const [detail, setDetail] = useState(false);
  const detailRef = useRef(false);
  useFrame(({ camera }) => {
    const target = (
      controls as unknown as {
        target?: Vector3;
      }
    )?.target;
    const next = target ? camera.position.distanceTo(target) < 14 : false;
    if (next !== detailRef.current) {
      detailRef.current = next;
      setDetail(next);
    }
  });
  const previewTarget = retreat
    ? retreat.target || (candidates.has(hovered) ? hovered : "")
    : action?.selecting
      ? target || (candidates.has(hovered) ? hovered : "")
      : "";
  const route = previewTarget
    ? {
        from: retreat?.from || selected,
        to: previewTarget,
        house: room.yourHouse,
      }
    : props.battleFocus
      ? props.battleFocus
      : room.battle
        ? {
            from: room.battle.from,
            to: room.battle.to,
            house: room.battle.attacker,
          }
        : lastRoute(room.events);
  const from = route?.from ? manifest.regions[route.from]?.position : undefined,
    to = route?.to ? manifest.regions[route.to]?.position : undefined;
  useEffect(() => {
    onLoaded({
      height: data.relief.values.reduce((max, n) => Math.max(max, n), 0),
      borders: Object.keys(data.borders).length,
    });
  }, [onLoaded, data]);
  const routePoints = useMemo(() => {
    if (!from || !to) return undefined;
    return Array.from({ length: 33 }, (_, i) => {
      const t = i / 32,
        x = from[0] + (to[0] - from[0]) * t,
        z = from[1] + (to[1] - from[1]) * t;
      return [
        x,
        data.relief.height(x, z) + 0.15 + Math.sin(t * Math.PI) * 0.35,
        z,
      ] as [number, number, number];
    });
  }, [from, to, data]);
  return (
    <>
      <color attach="background" args={["#201d19"]} />
      <fog attach="fog" args={["#201d19", 65, 105]} />
      <ambientLight intensity={1.05} />
      <hemisphereLight args={["#e6f0f1", "#8e806a", 1.6]} />
      <directionalLight
        castShadow
        position={[-7, 15, 4]}
        intensity={2.8}
        shadow-mapSize={[2048, 2048]}
        shadow-camera-left={-13}
        shadow-camera-right={13}
        shadow-camera-top={15}
        shadow-camera-bottom={-15}
        shadow-normalBias={0.012}
      />
      <directionalLight position={[7, 6, -8]} intensity={1.2} color="#e8c58d" />
      <directionalLight
        position={[0, 9, -12]}
        intensity={1.4}
        color="#e4efff"
      />
      <TableCamera
        width={manifest.board.width}
        depth={manifest.board.depth}
        relief={data.relief}
        focus={
          manifest.regions[props.battleFocus?.from || retreat?.from || selected]
            ?.position || [0, 0]
        }
        request={request}
        instant={!!playback && !playback.motion}
        points={(props.battleFocus
          ? [
              props.battleFocus.to,
              ...(playback?.phase === ReplayPhase.Losses &&
              playback.record.result?.retreatTo
                ? [playback.record.result.retreatTo]
                : []),
            ]
          : retreat?.targets || action?.targets || []
        ).map((id) => manifest.regions[id].position)}
        onPose={onPose}
      />
      <MapModel manifest={manifest} relief={data.relief} />
      <IslandTerrain source={data.surfaces.dragonstone} />
      {landmarks.map((landmark) => (
        <LandmarkFigure
          key={landmark.region}
          landmark={landmark}
          manifest={manifest}
          relief={data.relief}
          onSelect={() => choose(landmark.region)}
        />
      ))}
      <RegionSurfaces
        blocked={
          new Set(
            Object.entries(room.fortifications || {})
              .filter(([, value]) => value.kind === "blocked")
              .map(([id]) => id),
          )
        }
        data={data}
        selected={props.battleFocus?.from || selected}
        target={props.battleFocus?.to || target}
        active={active}
        candidates={candidates}
        hovered={hovered}
        borders={borders}
        onSelect={choose}
        onHover={onHover}
        onFocus={onFocus}
      />
      {routePoints && (
        <Line
          points={routePoints}
          color={
            previewTarget
              ? "#8be7c3"
              : catalog.houses[route!.house]?.color || "#ddc18b"
          }
          lineWidth={previewTarget ? 2.5 : 1.8}
          dashed={!previewTarget}
          dashSize={0.12}
          gapSize={0.08}
          raycast={() => {}}
        />
      )}
      {!playback &&
        presentation.pieces.map((piece) => (
          <ArmyFigure
            key={piece.id}
            piece={piece}
            point={formation(
              piece,
              presentation.pieces,
              manifest.regions[piece.region].position,
            )}
            manifest={manifest}
            color={catalog.houses[piece.house]?.color || "#d7bd83"}
            motion={motion}
            onSelect={() => choose(piece.region)}
            onFocus={() => onFocus(piece.region)}
            relief={data.relief}
          />
        ))}
      {playback &&
        !(
          playback.phase === ReplayPhase.Losses &&
          playback.complete &&
          playback.record.result?.final
        ) &&
        actors.map((actor) => {
          const start = formation(
            starts.find((piece) => piece.id === actor.piece.id)!,
            starts,
            manifest.regions[actor.origin].position,
          );
          const end = formation(
            actor.piece,
            ends,
            manifest.regions[actor.destination].position,
          );
          const retreatPiece = retreats.find(
            (piece) => piece.id === actor.piece.id,
          );
          return (
            <ArmyFigure
              key={`replay:${playback.serial}:${actor.piece.id}`}
              piece={actor.piece}
              point={playback.phase === ReplayPhase.Approach ? start : end}
              manifest={manifest}
              color={catalog.houses[actor.piece.house]?.color || "#d7bd83"}
              motion={playback.motion}
              onSelect={() => {}}
              onFocus={() => {}}
              relief={data.relief}
              replay={{
                actor,
                progress,
                phase: playback.phase,
                start,
                end,
                retreat:
                  actor.retreatTo && retreatPiece
                    ? formation(
                        retreatPiece,
                        retreats,
                        manifest.regions[actor.retreatTo].position,
                      )
                    : undefined,
                playing: playback.playing,
              }}
            />
          );
        })}
      {playback?.phase === ReplayPhase.Losses &&
        playback.complete &&
        playback.record.result?.final &&
        visiblePieces.map((piece) => (
          <ArmyFigure
            key={`final:${playback.serial}:${piece.id}`}
            piece={piece}
            point={formation(
              piece,
              visiblePieces,
              manifest.regions[piece.region].position,
            )}
            manifest={manifest}
            color={catalog.houses[piece.house]?.color || "#d7bd83"}
            motion={playback.motion}
            onSelect={() => {}}
            onFocus={() => {}}
            relief={data.relief}
          />
        ))}
      {catalog.board.regions.map((region) => {
        const anchor = manifest.regions[region.id],
          order = room.orders[region.id];
        const orderDefinition = catalog.orders.find(
          (value) => value.id === order,
        );
        const units = visiblePieces.filter((p) => p.region === region.id),
          house =
            units[0]?.house ||
            (room.match?.control || room.setup?.control)?.[region.id];
        const show =
          (labels && detail) ||
          hovered === region.id ||
          props.battleFocus?.from === region.id ||
          props.battleFocus?.to === region.id ||
          selected === region.id ||
          target === region.id;
        return (
          <Html
            key={region.id}
            position={[
              anchor.label[0],
              data.relief.height(...anchor.label) + 0.08,
              anchor.label[1],
            ]}
            center
            zIndexRange={
              candidates.has(region.id)
                ? [9, 8]
                : active.has(region.id)
                  ? [7, 6]
                  : [5, 1]
            }
            style={{ pointerEvents: "none" }}
          >
            <button
              data-region3d={region.id}
              data-eligible={candidates.has(region.id)}
              aria-label={regionName(region)}
              aria-pressed={
                retreat ? target === region.id : selected === region.id
              }
              className={`game3d-region ${props.battleFocus?.from === region.id ? "battle-from" : ""} ${props.battleFocus?.to === region.id ? "battle-to" : ""} ${selected === region.id ? "selected" : ""} ${target === region.id ? "target" : ""} ${active.has(region.id) ? "actionable" : ""} ${candidates.has(region.id) ? "eligible" : ""} ${show || candidates.has(region.id) ? "" : "quiet"} ${!show && !units.length && !order && !room.fortifications?.[region.id] && !candidates.has(region.id) ? "empty" : ""}`}
              style={{
                borderColor:
                  house &&
                  show &&
                  ![props.battleFocus?.from, props.battleFocus?.to].includes(
                    region.id,
                  )
                    ? catalog.houses[house]?.color
                    : undefined,
              }}
              onClick={(e) => {
                e.stopPropagation();
                choose(region.id);
              }}
              onDoubleClick={(e) => {
                e.stopPropagation();
                onFocus(region.id);
              }}
            >
              {(show || candidates.has(region.id)) && (
                <span>{regionName(region)}</span>
              )}
              {[...new Set(units.map((unit) => unit.house))].map(
                (unitHouse) => (
                  <small
                    key={unitHouse}
                    data-unit-house={unitHouse}
                    title={catalog.houseNames[unitHouse]}
                  >
                    <img src={`/media/crests/${unitHouse}.svg`} alt="" />
                    {units.filter((unit) => unit.house === unitHouse).length}
                  </small>
                ),
              )}
              {orderDefinition && (
                <OrderToken className="game3d-order" order={orderDefinition} />
              )}
              {room.fortifications?.[region.id] && (
                <FortificationBadge value={room.fortifications[region.id]} />
              )}
            </button>
          </Html>
        );
      })}
      <mesh
        rotation={[-Math.PI / 2, 0, 0]}
        position={[0, -0.4, 0]}
        receiveShadow
      >
        <planeGeometry args={[100, 100]} />
        <meshStandardMaterial color="#28241c" roughness={1} />
      </mesh>
    </>
  );
}
export default function GameBoard3D(props: Props) {
  const { room, catalog, manifest, selected, onRetry, action, retreat } = props;
  const touch = useRef(new TouchSelection());
  const [visible, setVisible] = useState(
    document.visibilityState === "visible",
  );
  useEffect(() => {
    const changed = () => setVisible(document.visibilityState === "visible");
    document.addEventListener("visibilitychange", changed);
    return () => document.removeEventListener("visibilitychange", changed);
  }, []);
  const safeProps = {
    ...props,
    room: props.combatPlayback ? replayRoom(room, props.combatPlayback) : room,
    action: props.combatPlayback ? undefined : props.action,
    selected: props.combatPlayback?.record.from || props.selected,
    target: props.combatPlayback ? undefined : props.target,
    retreat:
      !props.combatPlayback && retreat
        ? {
            ...retreat,
            onTarget: (id: string) => {
              if (touch.current.allowsClick()) retreat.onTarget(id);
            },
          }
        : undefined,
    onSelect: (id: string) => {
      if (!props.combatPlayback && touch.current.allowsClick())
        props.onSelect(id);
    },
    onTarget: props.onTarget
      ? (id: string) => {
          if (!props.combatPlayback && touch.current.allowsClick())
            props.onTarget?.(id);
        }
      : undefined,
  };
  const [request, setRequest] = useState<CameraRequest>({
      view: "table",
      serial: 0,
      rotation: 0,
    }),
    [labels, setLabels] = useState(true),
    [borders, setBorders] = useState(
      localStorage.getItem("westeros-borders") !== "false",
    ),
    [hovered, setHovered] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [stats, setStats] = useState({ height: 0, borders: 0 });
  const [pose, setPose] = useState<CameraPose>({
    polar: "",
    azimuth: "",
    target: "",
    distance: 0,
  });
  const handleLoaded = useCallback(
    (stats: { height: number; borders: number }) => {
      setLoaded(true);
      setStats(stats);
    },
    [],
  );
  const [presentation, setPresentation] = useState(() =>
    reconcilePieces(undefined, room, catalog.units),
  );
  useEffect(
    () =>
      setPresentation((previous) =>
        reconcilePieces(previous, room, catalog.units),
      ),
    [room, catalog],
  );
  const camera = (next: CameraRequest["view"]) => {
    setRequest((old) => ({
      ...old,
      view: next,
      serial: old.serial + 1,
      rotation: next === "table" ? 0 : old.rotation,
    }));
  };
  const focusedAction = useRef("");
  useEffect(() => {
    if (!props.combatPlayback && props.frameRequest) camera("action");
  }, [props.frameRequest]);
  useEffect(() => {
    if (props.battleFocus) camera("action");
  }, [
    props.battleFocus?.from,
    props.battleFocus?.to,
    props.combatPlayback?.serial,
  ]);
  useEffect(() => {
    if (!props.combatPlayback && retreat) camera("action");
  }, [room.id, room.phase, retreat?.from]);
  useEffect(() => {
    const key = `${room.round}:${room.phase}:${selected}`;
    if (
      !props.combatPlayback &&
      action?.selecting &&
      !action.loading &&
      !action.error &&
      action.targets.length &&
      focusedAction.current !== key
    ) {
      focusedAction.current = key;
      camera("action");
    }
  }, [
    action?.selecting,
    action?.loading,
    action?.error,
    room.round,
    room.phase,
    selected,
  ]);
  const chooseRegion = catalog.board.regions.find(
    (r) => r.id === (hovered || selected),
  );
  return (
    <section
      className="game-board game3d-board"
      data-tour="board"
      data-testid="game3d-board"
      data-ready={loaded}
      data-revision={room.revision}
      data-piece-count={
        props.combatPlayback
          ? replayPieces(
              props.combatPlayback.record,
              props.combatPlayback.phase,
              props.combatPlayback.complete,
            ).length
          : presentation.pieces.length
      }
      data-playback={props.combatPlayback?.phase || ""}
      data-playback-serial={props.combatPlayback?.serial}
      data-playback-pieces={
        props.combatPlayback
          ? replayPieces(
              props.combatPlayback.record,
              props.combatPlayback.phase,
              props.combatPlayback.complete,
            ).length
          : undefined
      }
      data-landmark-count={
        manifest.landmarks
          ? catalog.board.regions.filter(
              (region) => region.type === "port" || region.castleLevel > 0,
            ).length
          : 0
      }
      data-relief-max={stats.height.toFixed(3)}
      data-border-count={stats.borders}
      data-borders={borders}
      data-camera-polar={pose.polar}
      data-camera-azimuth={pose.azimuth}
      data-camera-target={pose.target}
      data-camera-distance={pose.distance.toFixed(3)}
    >
      <div
        className="game3d-stage"
        onContextMenu={(e) => e.preventDefault()}
        onPointerDownCapture={(event) => {
          if (event.pointerType === "touch")
            touch.current.down(event.pointerId, event.clientX, event.clientY);
        }}
        onPointerMoveCapture={(event) => {
          if (event.pointerType === "touch")
            touch.current.move(event.pointerId, event.clientX, event.clientY);
        }}
        onPointerUpCapture={(event) => {
          if (event.pointerType === "touch") touch.current.up(event.pointerId);
        }}
        onPointerCancelCapture={(event) => {
          if (event.pointerType === "touch")
            touch.current.up(event.pointerId, true);
        }}
      >
        <BoardBoundary onRetry={onRetry} onError={() => setFailed(true)}>
          <Canvas
            frameloop={props.paused || !visible ? "demand" : "always"}
            shadows="percentage"
            dpr={[1, 1.5]}
            camera={{
              position: [12, Math.hypot(12, 15), -15],
              fov: 40,
              near: 0.05,
              far: 150,
            }}
            gl={{ antialias: true, powerPreference: "high-performance" }}
            fallback={
              <div className="game3d-failure">
                <p>{t("assets.webglUnsupported")}</p>
                <button onClick={onRetry}>{t("assets.retry3d")}</button>
              </div>
            }
          >
            <Suspense fallback={null}>
              <SceneContent
                props={safeProps}
                presentation={presentation}
                labels={labels}
                request={request}
                onHover={setHovered}
                onLoaded={handleLoaded}
                hovered={hovered}
                borders={borders}
                onFocus={(id) => {
                  if (props.combatPlayback || !touch.current.allowsClick())
                    return;
                  props.onSelect(id);
                  camera("focus");
                }}
                onPose={setPose}
              />
            </Suspense>
          </Canvas>
        </BoardBoundary>
        {!loaded && !failed && (
          <div className="game3d-loading">
            <Shield size={26} />
            <b>{t("loading.armies")}</b>
            <span>{t("assets.originalModels")}</span>
            <small>{t("assets.hardwareAccelerationHint")}</small>
          </div>
        )}
        <div className="game3d-tools">
          <button title={t("board.fit")} onClick={() => camera("table")}>
            <LocateFixed size={16} />
            {t("board.table")}
          </button>
          <button
            title={t("board.zoomSelected")}
            onClick={() => camera("focus")}
          >
            <Focus size={16} />
            {t("panels.area")}
          </button>
          <button title={t("board.zoomIn")} onClick={() => camera("in")}>
            <Plus size={16} />
          </button>
          <button title={t("board.zoomOut")} onClick={() => camera("out")}>
            <Minus size={16} />
          </button>
          <button
            title={t("board.rotate")}
            onClick={() =>
              setRequest((old) => ({
                ...old,
                view: old.view === "focus" ? "focus" : "table",
                serial: old.serial + 1,
                rotation: old.rotation + Math.PI / 2,
              }))
            }
          >
            <RotateCw size={16} />
          </button>
          <button
            title={t("board.borders")}
            aria-pressed={borders}
            onClick={() => {
              setBorders(!borders);
              localStorage.setItem("westeros-borders", String(!borders));
            }}
          >
            <Shield size={16} />
          </button>
          <button
            title={t("board.labels")}
            aria-pressed={labels}
            onClick={() => setLabels(!labels)}
          >
            <Tag size={16} />
          </button>
        </div>
        <div className="game3d-caption">
          <span>
            <Layers size={13} />
            3D · {presentation.pieces.length}
            {t("board.piecesUnit")}
          </span>
          <b>{regionName(chooseRegion)}</b>
          <small>
            {retreat
              ? t("retreat.mapHint")
              : props.onTarget
                ? t("march.mapHint")
                : t("board.controlsHint")}
          </small>
        </div>
      </div>
      {retreat && (
        <div className="map-move-dock" data-testid="map-retreat-panel">
          <div className="map-move-heading">
            <span>
              <Flag size={14} />
              {t("retreat.heading")}
            </span>
            <b>
              {regionName(
                catalog.board.regions.find((r) => r.id === retreat.from),
              )}
              <ArrowRight size={16} />
              {retreat.target
                ? regionName(
                    catalog.board.regions.find((r) => r.id === retreat.target),
                  )
                : t("board.selectAreaOnMap")}
            </b>
          </div>
          <div className="map-move-footer">
            <p role="status">{t("retreat.legalAreasHint")}</p>
            <button onClick={() => camera("action")}>
              {t("actions.showTargets")}
            </button>
            <button
              className="map-confirm"
              disabled={
                retreat.busy || !retreat.targets.includes(retreat.target)
              }
              onClick={retreat.onConfirm}
            >
              {retreat.busy ? t("retreat.inProgress") : t("retreat.confirm")}
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
