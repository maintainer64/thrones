import type { Room } from "../types";

// Revision and translated prose do not reopen a deliberately closed dialog.
export function ravenIdentity(
  room: Pick<Room, "id" | "phase" | "round" | "raven">,
) {
  const step = room.raven;
  return room.phase === "raven" && step && step.round === room.round
    ? `${room.id}:${room.round}:${step.house}:${step.stage}`
    : "";
}
