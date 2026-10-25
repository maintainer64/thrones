import { describe, it, expect } from "vitest";
import { TouchSelection } from "./touch-selection";
describe("touch selection versus orbit gestures", () => {
  it("allows a tap with a small finger drift", () => {
    const touch = new TouchSelection(() => 0);
    touch.down(1, 10, 20);
    touch.move(1, 13, 24);
    touch.up(1);
    expect(touch.allowsClick()).toBe(true);
  });
  it("does not turn dragging into a region selection", () => {
    let now = 0;
    const touch = new TouchSelection(() => now);
    touch.down(1, 0, 0);
    touch.move(1, 40, 0);
    touch.up(1);
    expect(touch.allowsClick()).toBe(false);
    now = 400;
    expect(touch.allowsClick()).toBe(true);
    touch.down(2, 40, 0);
    touch.up(2);
    expect(touch.allowsClick()).toBe(true);
  });
  it("blocks pinch/rotation even if the fingers return to their initial points", () => {
    const touch = new TouchSelection(() => 0);
    touch.down(1, 0, 0);
    touch.down(2, 0, 30);
    touch.up(2);
    touch.up(1);
    expect(touch.allowsClick()).toBe(false);
  });
  it("blocks cancelled touches", () => {
    const touch = new TouchSelection(() => 0);
    touch.down(1, 0, 0);
    touch.up(1, true);
    expect(touch.allowsClick()).toBe(false);
  });
});
