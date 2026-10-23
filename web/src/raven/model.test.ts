import { describe, expect, it } from "vitest";
import { RavenStage } from "../enums";
import { ravenIdentity } from "./model";

describe("Raven dialog identity", () => {
  const room = {
    id: "room",
    phase: "raven",
    round: 2,
    raven: { round: 2, house: "stark", stage: RavenStage.Choose },
  };
  it("does not invent a Raven step in another phase or round", () => {
    expect(ravenIdentity({ ...room, phase: "planning" })).toBe("");
    expect(ravenIdentity({ ...room, round: 3 })).toBe("");
    expect(ravenIdentity({ ...room, raven: undefined })).toBe("");
  });
  it("is stable until the real private decision advances", () => {
    expect(ravenIdentity(room)).toBe("room:2:stark:choose");
    expect(
      ravenIdentity({
        ...room,
        raven: { ...room.raven, stage: RavenStage.Peek },
      }),
    ).not.toBe(ravenIdentity(room));
  });
});
