import { describe, expect, it } from "vitest";
import { combatIdentity, keepNewest } from "./state";

describe("authoritative room snapshots", () => {
  it("does not reopen a battle result when only its effect language changes", () => {
    const result = {
      attacker: "stark",
      defender: "greyjoy",
      attackerCard: "eddard-stark",
      defenderCard: "asha-greyjoy",
      attack: 6,
      defense: 4,
      winner: "stark",
      region: "winterfell",
      effects: ["Русский текст"],
    };
    expect(combatIdentity(1, result)).toBe(
      combatIdentity(1, { ...result, effects: ["English text"] }),
    );
    expect(combatIdentity(1, result)).not.toBe(combatIdentity(2, result));
  });
  it("accepts the first snapshot", () => {
    const room = { id: "a", revision: 1 };
    expect(keepNewest(undefined, room)).toBe(room);
  });
  it("rejects an HTTP response older than a WebSocket update", () => {
    const current = { id: "a", revision: 5 };
    expect(keepNewest(current, { id: "a", revision: 4 })).toBe(current);
  });
  it("accepts a newer or equal personal snapshot", () => {
    const incoming = { id: "a", revision: 6 };
    expect(keepNewest({ id: "a", revision: 5 }, incoming)).toBe(incoming);
    expect(keepNewest({ ...incoming }, incoming)).toBe(incoming);
  });
  it("does not compare revisions between different rooms", () => {
    const incoming = { id: "b", revision: 1 };
    expect(keepNewest({ id: "a", revision: 50 }, incoming)).toBe(incoming);
  });
});
