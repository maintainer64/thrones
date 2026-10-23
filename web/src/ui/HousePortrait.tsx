import { useState } from "react";

const leaders: Record<string, string> = {
  stark: "eddard-stark",
  greyjoy: "balon-greyjoy",
  lannister: "tywin-lannister",
  baratheon: "stannis-baratheon",
  tyrell: "mace-tyrell",
  martell: "doran-martell",
};

export function HousePortrait({
  house,
  art,
}: {
  house: string;
  art: (card: string, house: string) => string | undefined;
}) {
  const [failedSource, setFailedSource] = useState<string>();
  const leader = leaders[house];
  const src = leader ? art(leader, house) : undefined;
  return (
    <span className="seat-seal" aria-hidden="true">
      {src && src !== failedSource && (
        <img
          className="seat-portrait"
          src={src}
          alt=""
          onError={() => setFailedSource(src)}
        />
      )}
      <img
        className="seat-heraldry"
        src={`/media/crests/${house}.svg`}
        alt=""
      />
    </span>
  );
}
