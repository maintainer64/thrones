/** HTTP responses and WebSocket messages may arrive out of order. */
import type { Match } from "./types";

/** Translated effect prose must not make an already seen battle look new. */
export function combatIdentity(
  round: number,
  result: NonNullable<Match["lastCombat"]>,
) {
  if (result.id) return String(result.id);
  const { effects: _effects, ...state } = result;
  return JSON.stringify({ round, ...state });
}

export function keepNewest<T extends { id: string; revision: number }>(
  previous: T | undefined,
  incoming: T,
): T {
  return previous?.id === incoming.id && previous.revision > incoming.revision
    ? previous
    : incoming;
}
