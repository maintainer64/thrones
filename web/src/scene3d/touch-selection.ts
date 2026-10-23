// Touch orbit gestures must never select a region/target on the synthetic click.
export class TouchSelection {
  private points = new Map<number, { x: number; y: number }>();
  private gesture = false;
  private blockedUntil = 0;
  constructor(private now = () => performance.now()) {}
  down(id: number, x: number, y: number) {
    if (!this.points.size) this.gesture = false;
    this.points.set(id, { x, y });
    if (this.points.size > 1) this.gesture = true;
  }
  move(id: number, x: number, y: number) {
    const start = this.points.get(id);
    if (start && Math.hypot(x - start.x, y - start.y) > 8) this.gesture = true;
  }
  up(id: number, cancelled = false) {
    if (!this.points.has(id)) return;
    if (cancelled) this.gesture = true;
    this.points.delete(id);
    if (this.gesture) this.blockedUntil = this.now() + 300;
  }
  allowsClick() {
    return (
      this.now() >= this.blockedUntil &&
      (this.points.size === 0 || (!this.gesture && this.points.size < 2))
    );
  }
}
