import { t } from "../i18n/core";
import { useEffect, useRef, useState } from "react";
import { GameSymbol } from "../ui/GameSymbol";
import type { CSSProperties } from "react";
import type { Card } from "../types";
import { cardRulesTop, pointerPose } from "./card-art";
import { StrengthShield } from "../ui/StrengthShield";
export function HouseCard({
  card,
  src,
  color,
  motion,
  cinematic = false,
}: {
  card: Card;
  src?: string;
  color: string;
  motion: boolean;
  cinematic?: boolean;
}) {
  const element = useRef<HTMLSpanElement>(null);
  const [failedArt, setFailedArt] = useState(false);
  const [reduced, setReduced] = useState(
    () => matchMedia("(prefers-reduced-motion: reduce)").matches,
  );
  useEffect(() => {
    const query = matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(query.matches);
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    setFailedArt(false);
  }, [src]);
  const animated = motion && !reduced;
  const description = card.description || t("cards.standardIconsHint");
  const reset = () => {
    for (const [name, value] of Object.entries({
      "--tilt-x": "0deg",
      "--tilt-y": "0deg",
      "--light-x": "50%",
    }))
      element.current?.style.setProperty(name, value);
  };
  useEffect(reset, [animated, src]);
  return (
    <span
      ref={element}
      className={`house-card ${cinematic ? "cinematic" : ""} ${animated ? "animated" : ""}`}
      style={
        {
          "--house-ink": color,
          "--card-rules-top": `${cardRulesTop(description)}%`,
        } as CSSProperties
      }
      onPointerMove={
        cinematic && animated
          ? (event) => {
              if (event.pointerType === "touch") return;
              const box = event.currentTarget.getBoundingClientRect();
              const pose = pointerPose(
                (event.clientX - box.left) / box.width,
                (event.clientY - box.top) / box.height,
              );
              element.current?.style.setProperty("--tilt-x", `${pose.x}deg`);
              element.current?.style.setProperty("--tilt-y", `${pose.y}deg`);
              element.current?.style.setProperty("--light-x", `${pose.light}%`);
            }
          : undefined
      }
      onPointerLeave={reset}
    >
      <span className="house-card-portrait">
        {src && !failedArt ? (
          <img
            className="house-card-art"
            src={src}
            alt={card.name}
            onError={() => setFailedArt(true)}
          />
        ) : (
          <span className="house-card-missing">{card.name}</span>
        )}
      </span>
      <span className="house-card-foil" aria-hidden="true" />
      <span className="house-card-strength">
        <StrengthShield
          value={card.combatStrength || 0}
          label={t("cards.strengthPrefix")}
        />
      </span>
      <span className="house-card-name">{card.name}</span>
      <span className="house-card-parchment">
        <span className="house-card-description">{description}</span>
        {!!(card.swordIcons || card.towerIcons) && (
          <span className="house-card-stats">
            {!!card.swordIcons && (
              <span aria-label={t("cards.swordsSuffix")}>
                <GameSymbol kind="sword" />
                {card.swordIcons}
              </span>
            )}
            {!!card.towerIcons && (
              <span aria-label={t("cards.fortificationsSuffix")}>
                <GameSymbol kind="tower" />
                {card.towerIcons}
              </span>
            )}
          </span>
        )}
      </span>
      {cinematic && (
        <span className="house-card-motes" aria-hidden="true">
          {Array.from({ length: 8 }, (_, index) => (
            <i key={index} style={{ "--i": index } as CSSProperties} />
          ))}
        </span>
      )}
    </span>
  );
}
