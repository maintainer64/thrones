import { describe, expect, it } from "vitest";
import { boardFrameLoop, sameCameraPose } from "./render-policy";

describe("idle 3D rendering", () => {
  const active = {
    paused: false,
    visible: true,
    motion: true,
    animatedPlayback: false,
  };
  it("keeps normal animated play continuous", () => {
    expect(boardFrameLoop(active)).toBe("always");
  });
  it("does not continuously render when animations are disabled", () => {
    expect(boardFrameLoop({ ...active, motion: false })).toBe("demand");
  });
  it("pauses hidden and modal-covered boards even during playback", () => {
    expect(
      boardFrameLoop({ ...active, paused: true, animatedPlayback: true }),
    ).toBe("demand");
    expect(boardFrameLoop({ ...active, visible: false })).toBe("demand");
  });
  it("still renders an explicitly animated replay", () => {
    expect(
      boardFrameLoop({ ...active, motion: false, animatedPlayback: true }),
    ).toBe("always");
  });
  it("does not restart demand rendering for identical camera reports", () => {
    const pose = {
      polar: "0.785",
      azimuth: "1.000",
      target: "0,0,0",
      distance: 20,
    };
    expect(sameCameraPose(pose, { ...pose })).toBe(true);
    expect(sameCameraPose(pose, { ...pose, distance: 20.00001 })).toBe(true);
    expect(sameCameraPose(pose, { ...pose, distance: 19.9 })).toBe(false);
    expect(sameCameraPose(pose, { ...pose, target: "1,0,0" })).toBe(false);
    expect(sameCameraPose(pose, { ...pose, azimuth: "1.100" })).toBe(false);
  });
});
