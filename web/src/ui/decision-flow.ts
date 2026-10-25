import { useEffect, useState, createContext } from "react";
import { eventsIdentity } from "../events/model";
import type { Room } from "../types";

export type DecisionKind = "events" | "provision" | "influence" | "raven";
export type Decision = { kind: DecisionKind; id: string; room: Room };
export type DecisionState = { active?: Decision; dismissed?: string };
export const DecisionFlowContext = createContext<{
  stale: boolean;
  next: () => void;
  urgent?: () => void;
} | null>(null);

export function decisionFor(
  room: Room | undefined,
  manual?: DecisionKind,
): Decision | undefined {
  if (!room) return;
  const kind =
    manual ||
    (room.raven && room.phase === "raven"
      ? "raven"
      : room.influence
        ? "influence"
        : ["supply", "mustering", "consolidate_power"].includes(room.phase)
          ? "provision"
          : eventsIdentity(room)
            ? "events"
            : undefined);
  if (!kind) return;
  const group =
    kind === "events"
      ? room.phase.startsWith("wildling_")
        ? `wildlings:${room.wildlings?.attack?.strength}:${room.wildlings?.attack?.excludedHouse || ""}`
        : "westeros"
      : kind === "provision"
        ? `${room.phase}:${room.match?.activeHouse}`
        : kind;
  return { kind, id: `${room.id}:${room.round}:${group}`, room };
}

type FlowAction =
  | { type: "observe"; room?: Room; blocked?: boolean }
  | { type: "open"; room: Room; kind: DecisionKind }
  | { type: "close"; room?: Room; blocked?: boolean }
  | { type: "next"; room?: Room; blocked?: boolean }
  | { type: "commit"; room?: Room };

export function updateDecision(
  state: DecisionState,
  action: FlowAction,
): DecisionState {
  const room = action.room;
  const incoming = decisionFor(room);
  if (!room) return {};
  if (action.type === "open") return { active: decisionFor(room, action.kind) };
  if (action.type === "close")
    return { dismissed: action.blocked ? state.active?.id : incoming?.id };
  if (action.type === "next") {
    if (action.blocked) return { dismissed: state.active?.id };
    const overview =
      !incoming &&
      state.active?.kind === "events" &&
      state.active.id.includes(":wildlings:") &&
      room.westeros?.current
        ? decisionFor(room, "events")
        : undefined;
    return { active: incoming || overview, dismissed: incoming?.id };
  }
  if (action.type === "commit") {
    // An explicit completed choice may advance its own window. Other players'
    // updates must never dismiss the event currently being read.
    if (
      state.active?.kind === "influence" &&
      !room.influence &&
      room.match?.lastInfluence?.length
    )
      return { active: { ...state.active, room } };
    if (
      state.active?.kind === "events" &&
      state.active.id.endsWith(":westeros") &&
      !incoming &&
      room.westeros?.current?.round === state.active.room.round
    )
      return { active: { ...state.active, room } };
    return { active: incoming, dismissed: incoming?.id };
  }
  const active = state.active;
  if (active && active.room.id !== room.id)
    return { active: action.blocked ? undefined : incoming };
  if (active) {
    if (room.revision < active.room.revision) return state;
    const same = incoming?.id === active.id;
    const completedEvents =
      active.kind === "events" &&
      active.id.endsWith(":westeros") &&
      !room.phase.startsWith("wildling_") &&
      !incoming &&
      room.westeros?.current?.round === active.room.round;
    const auctionReport =
      active.kind === "influence" &&
      !room.influence &&
      !!room.match?.lastInfluence?.length;
    if ((same || completedEvents || auctionReport) && active.room !== room)
      return { ...state, active: { ...active, room } };
    return state; // Pin the previous snapshot until the reader chooses Next.
  }
  return !action.blocked && incoming && incoming.id !== state.dismissed
    ? { active: incoming }
    : state;
}

export function useDecisionFlow(room: Room | undefined, blocked: boolean) {
  const [state, setState] = useState<DecisionState>({});
  useEffect(
    () =>
      setState((previous) =>
        updateDecision(previous, { type: "observe", room, blocked }),
      ),
    [room, blocked],
  );
  const active = state.active?.room.id === room?.id ? state.active : undefined;
  const latest = decisionFor(room);
  const dispatch = (action: FlowAction) =>
    setState((previous) => updateDecision(previous, action));
  return {
    active,
    stale: !!active && latest?.id !== active.id,
    open: (kind: DecisionKind) => {
      if (room) dispatch({ type: "open", room, kind });
    },
    close: () => dispatch({ type: "close", room, blocked }),
    next: () => dispatch({ type: "next", room, blocked }),
    commit: (next: Room) =>
      dispatch({
        type: "commit",
        room: room && room.revision > next.revision ? room : next,
      }),
  };
}
