import { useState } from "react";
import { artworkURL } from "../artwork/artwork";

export function StrengthShield({
  value,
  label,
}: {
  value: number;
  label: string;
}) {
  const [missing, setMissing] = useState(false);
  return (
    <span
      className={`strength-shield ${missing ? "shield-fallback" : ""}`}
      aria-label={`${label.replace(/:\s*$/, "")}: ${value}`}
    >
      {!missing && (
        <img
          src={artworkURL("ui/strength-shield.png")}
          alt=""
          onError={() => setMissing(true)}
        />
      )}
      <span>{value}</span>
    </span>
  );
}
