import { t } from "../i18n/core";
import type { Card, Room } from "../types";
import { HouseCard } from "./HouseCard";
import { ChevronDown, Swords } from "lucide-react";
import { orderedCards } from "./card-art";
export function HouseHand({
  room,
  cards,
  houseName,
  color,
  art,
  motion,
  open,
  onToggle,
  onInspect,
}: {
  room: Room;
  cards: Record<string, Card>;
  houseName: string;
  color: string;
  art: (id: string) => string | undefined;
  motion: boolean;
  open: boolean;
  onToggle: (open: boolean) => void;
  onInspect: (id: string) => void;
}) {
  return (
    <footer className="house-hand-dock" data-tour="cards">
      <details
        className="hand"
        open={open}
        onToggle={(event) => onToggle(event.currentTarget.open)}
      >
        <summary
          aria-label={open ? t("cards.collapseHand") : t("cards.showHand")}
        >
          <span className="hand-heading">
            <Swords size={16} />
            {t("cards.handTitle")}
            {houseName}
          </span>
          <span className="hand-count">
            {room.availableCards?.length ?? 7} / 7 <ChevronDown size={16} />
          </span>
        </summary>
        <div className="hand-cards" aria-label={t("cards.handLabel")}>
          {orderedCards(cards).map(([id, card]) => {
            const discarded = room.match?.discard[room.yourHouse]?.includes(id);
            return (
              <button
                key={id}
                className={discarded ? "discarded" : ""}
                aria-label={t("cards.accessibleLabel", {
                  p0: card.name,
                  p1: card.combatStrength || 0,
                  p2: discarded ? t("cards.discardedSuffix") : "",
                })}
                title={t("cards.accessibleLabel", {
                  p0: card.name,
                  p1: card.combatStrength || 0,
                  p2: discarded ? t("cards.discardedSuffix") : "",
                })}
                onClick={() => onInspect(id)}
              >
                <HouseCard
                  card={card}
                  src={art(id)}
                  color={color}
                  motion={motion}
                />
                {discarded && (
                  <span className="hand-discard-label">
                    {t("cards.discard")}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      </details>
    </footer>
  );
}
