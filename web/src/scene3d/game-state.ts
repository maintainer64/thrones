import type { Event, Room, Unit } from "../types";

export type Point2 = [number, number];
export type RetreatAction = {
  from: string;
  targets: string[];
  target: string;
  busy: boolean;
  onTarget: (id: string) => void;
  onConfirm: () => void;
};
export type MapAction = {
  kind: "march" | "raid";
  selecting: boolean;
  loading: boolean;
  error: string;
  targets: string[];
  busy: boolean;
  units: { type: string; label: string; available: number; quantity: number }[];
  onQuantity: (type: string, value: number) => void;
  onBegin: () => void;
  onCancel: () => void;
  onConfirm: () => void;
  onPass: () => void;
  onRetry: () => void;
};
export type GameManifest = {
  version: 1;
  board: { url: string; width: number; depth: number };
  picking: string;
  landmarks?: { castle: string; stronghold: string; port: string };
  regions: Record<string, { position: Point2; label: Point2; order: Point2 }>;
  units: Record<
    string,
    {
      url: string;
      textures: Record<string, string>;
      bones: number;
      clips: string[];
    }
  >;
};
export type Piece = {
  id: number;
  region: string;
  house: string;
  type: string;
  slot: number;
  battle: boolean;
};
export type Presentation = {
  pieces: Piece[];
  nextId: number;
  eventId: number;
  roomId: string;
};

export function isGameManifest(value: unknown): value is GameManifest {
  if (!value || typeof value !== "object") return false;
  const m = value as GameManifest;
  const url = (value: unknown) =>
    typeof value === "string" && value.startsWith("/local-assets/preview3d/");
  const point = (value: unknown) =>
    Array.isArray(value) &&
    value.length === 2 &&
    value.every((n) => typeof n === "number" && Number.isFinite(n));
  if (
    m.version !== 1 ||
    !m.board ||
    !url(m.board.url) ||
    !url(m.picking) ||
    !Number.isFinite(m.board.width) ||
    !Number.isFinite(m.board.depth) ||
    !(m.board.width > 0) ||
    !(m.board.depth > 0) ||
    !m.regions ||
    Object.keys(m.regions).length !== 58 ||
    !m.units
  )
    return false;
  if (
    !Object.values(m.regions).every(
      (region) =>
        region &&
        point(region.position) &&
        point(region.label) &&
        point(region.order),
    )
  )
    return false;
  if (
    m.landmarks &&
    ![m.landmarks.castle, m.landmarks.stronghold, m.landmarks.port].every(url)
  )
    return false;
  for (const type of ["footman", "knight", "ship", "siege-engine"]) {
    const unit = m.units[type];
    if (!unit || !url(unit.url) || !unit.textures || !Array.isArray(unit.clips))
      return false;
    if (
      ![
        "stark",
        "lannister",
        "greyjoy",
        "baratheon",
        "tyrell",
        "martell",
      ].every((house) => url(unit.textures[house]))
    )
      return false;
  }
  return true;
}

// Presentation identity only. All quantities and destinations come from the server.
// Reuse a removed figure only when a new public move event explains its arrival.
export function reconcilePieces(
  previous: Presentation | undefined,
  room: Room,
  initial: Record<string, Unit[]>,
): Presentation {
  const sameRoom = previous?.roomId === room.id;
  const old = sameRoom ? previous!.pieces : [];
  const publicEvents = room.events || [];
  const events = sameRoom
    ? publicEvents.filter(
        (e) =>
          e.id > previous!.eventId &&
          e.from &&
          e.to &&
          ["march", "combat", "retreat"].includes(e.type),
      )
    : [];
  const wanted: Omit<Piece, "id">[] = [];
  for (const [region, army] of Object.entries(
    room.match?.armies || room.setup?.armies || initial,
  )) {
    for (const unit of army || [])
      for (let slot = 0; slot < unit.quantity; slot++)
        wanted.push({
          region,
          house: unit.house,
          type: unit.unitType,
          slot,
          battle: false,
        });
  }
  if (room.battle)
    for (const unit of room.battle.army || []) {
      for (let slot = 0; slot < unit.quantity; slot++)
        wanted.push({
          region: room.battle.to,
          house: unit.house,
          type: unit.unitType,
          slot,
          battle: true,
        });
    }
  const remaining = new Set(old);
  const result: (Piece | undefined)[] = wanted.map((unit) => {
    const existing = old.find(
      (piece) =>
        remaining.has(piece) &&
        piece.region === unit.region &&
        piece.house === unit.house &&
        piece.type === unit.type &&
        piece.slot === unit.slot &&
        piece.battle === unit.battle,
    );
    if (existing) {
      remaining.delete(existing);
      return { ...unit, id: existing.id };
    }
  });
  let nextId = sameRoom ? previous!.nextId : 1;
  for (let i = 0; i < wanted.length; i++) {
    if (result[i]) continue;
    const unit = wanted[i];
    const transfer = [...remaining].find(
      (piece) =>
        piece.house === unit.house &&
        piece.type === unit.type &&
        events.some(
          (event) =>
            event.house === unit.house &&
            event.from === piece.region &&
            event.to === unit.region,
        ),
    );
    // A battle resolving in place is not a fresh march.
    const battleSurvivor = [...remaining].find(
      (piece) =>
        piece.battle &&
        !unit.battle &&
        piece.house === unit.house &&
        piece.type === unit.type &&
        piece.region === unit.region,
    );
    const existing = transfer || battleSurvivor;
    if (existing) remaining.delete(existing);
    result[i] = { ...unit, id: existing?.id ?? nextId++ };
  }
  return {
    pieces: result as Piece[],
    nextId,
    eventId: publicEvents.at(-1)?.id || 0,
    roomId: room.id,
  };
}

export function formation(
  piece: Piece,
  pieces: Piece[],
  origin: Point2,
): Point2 {
  const group = pieces.filter(
    (p) => p.region === piece.region && p.house === piece.house,
  );
  const index = group.findIndex((p) => p.id === piece.id);
  const columns = Math.min(3, group.length);
  const row = Math.floor(index / columns),
    column = index % columns;
  return [
    origin[0] +
      (column - (Math.min(columns, group.length - row * columns) - 1) / 2) *
        0.38,
    origin[1] + row * 0.38 + (piece.battle ? 0.65 : 0),
  ];
}

export function lastRoute(events: Event[]) {
  return (events || [])
    .filter(
      (e) => e.from && e.to && ["march", "combat", "retreat"].includes(e.type),
    )
    .at(-1);
}
