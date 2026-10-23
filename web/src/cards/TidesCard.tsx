import { useState } from "react";
import { Shield, Skull } from "lucide-react";
import { GameSymbol } from "../ui/GameSymbol";
import type { TidesCard as TidesData } from "../types";
import { t } from "../i18n/core";
import { tidesArt, tidesArtURL } from "./tides-art";

export function TidesCard({
  card,
  houseName,
}: {
  card?: TidesData;
  houseName: string;
}) {
  const source = card ? tidesArtURL(card.id) : undefined;
  const [failedSource, setFailedSource] = useState<string>();
  const missing = failedSource === source;
  const label = card
    ? t("tides.cardLabel", {
        p0: houseName,
        p1: card.strength,
        p2: card.swords,
        p3: card.towers,
        p4: card.skulls,
      })
    : t("tides.hiddenCard", { p0: houseName });
  return (
    <figure
      className={`tides-card ${card ? "revealed" : "hidden-card"}`}
      aria-label={label}
      data-testid="tides-card"
      data-card-id={card?.id || "hidden"}
    >
      {card && !missing ? (
        <img
          src={source}
          width={tidesArt[card.id].width}
          height={tidesArt[card.id].height}
          alt=""
          onError={() => setFailedSource(source)}
        />
      ) : (
        <div className="tides-card-fallback">
          <Shield aria-hidden="true" />
          <b>{card ? `+${card.strength}` : "?"}</b>
        </div>
      )}
      <figcaption>
        <b>{houseName}</b>
        {card ? (
          <span className="tides-card-icons">
            <Shield size={13} aria-hidden="true" /> +{card.strength}
            {card.swords > 0 && (
              <>
                <GameSymbol kind="sword" /> {card.swords}
              </>
            )}
            {card.towers > 0 && (
              <>
                <GameSymbol kind="tower" /> {card.towers}
              </>
            )}
            {card.skulls > 0 && (
              <>
                <Skull size={13} aria-hidden="true" /> {card.skulls}
              </>
            )}
          </span>
        ) : (
          <span>{t("tides.hidden")}</span>
        )}
      </figcaption>
    </figure>
  );
}
