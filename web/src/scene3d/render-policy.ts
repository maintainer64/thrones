export type CameraPose = {
  polar: string;
  azimuth: string;
  target: string;
  distance: number;
};

export function boardFrameLoop({
  paused,
  visible,
  motion,
  animatedPlayback,
}: {
  paused: boolean;
  visible: boolean;
  motion: boolean;
  animatedPlayback: boolean;
}): "demand" | "always" {
  return paused || !visible || (!motion && !animatedPlayback)
    ? "demand"
    : "always";
}

// The HUD displays millimetre precision. Re-reporting identical poses from a
// demand frame would itself invalidate another frame and keep an idle GPU busy.
export function sameCameraPose(a: CameraPose, b: CameraPose) {
  return (
    a.polar === b.polar &&
    a.azimuth === b.azimuth &&
    a.target === b.target &&
    Math.abs(a.distance - b.distance) < 0.0005
  );
}
