import { t } from "../i18n/core";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import {
  ChevronLeft,
  ChevronRight,
  Minus,
  Plus,
  RotateCcw,
  X,
} from "lucide-react";
import type { Card } from "../types";
import { HouseCard } from "./HouseCard";
import { fittedCardWidth, orderedCards } from "./card-art";
export function CardInspector({
  cards,
  selected,
  houseName,
  color,
  art,
  motion,
  onSelect,
  onClose,
}: {
  cards: Record<string, Card>;
  selected: string;
  houseName: string;
  color: string;
  art: (id: string) => string | undefined;
  motion: boolean;
  onSelect: (id: string) => void;
  onClose: () => void;
}) {
  const [zoom, setZoom] = useState(1);
  const [fitWidth, setFitWidth] = useState(280);
  const dialog = useRef<HTMLElement>(null);
  const close = useRef<HTMLButtonElement>(null);
  const stage = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const viewport = stage.current;
    if (!viewport) return;
    const resize = () => {
      const style = getComputedStyle(viewport);
      setFitWidth(
        fittedCardWidth(
          viewport.clientWidth -
            parseFloat(style.paddingLeft) -
            parseFloat(style.paddingRight),
          viewport.clientHeight -
            parseFloat(style.paddingTop) -
            parseFloat(style.paddingBottom),
        ),
      );
    };
    resize();
    const observer = new ResizeObserver(resize);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, []);
  const ids = orderedCards(cards).map(([id]) => id);
  const index = ids.indexOf(selected);
  const card = cards[selected];
  const latest = useRef({ ids, index, onSelect, onClose });
  latest.current = { ids, index, onSelect, onClose };
  useEffect(() => {
    stage.current?.scrollTo(0, 0);
  }, [selected]);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    close.current?.focus();
    const keydown = (event: KeyboardEvent) => {
      const { ids, index, onSelect, onClose } = latest.current;
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
      }
      if (["ArrowLeft", "ArrowRight"].includes(event.key)) {
        event.preventDefault();
        onSelect(
          ids[
            (index + (event.key === "ArrowLeft" ? -1 : 1) + ids.length) %
              ids.length
          ],
        );
      }
      if (event.key === "Tab") {
        const buttons = [
          ...(dialog.current?.querySelectorAll<HTMLButtonElement>(
            "button:not(:disabled)",
          ) || []),
        ];
        const first = buttons[0],
          last = buttons.at(-1);
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }
    };
    document.addEventListener("keydown", keydown);
    return () => {
      document.removeEventListener("keydown", keydown);
      previous?.focus();
    };
  }, []);
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <section
        ref={dialog}
        className="card-inspector-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="card-title"
        onClick={(event) => event.stopPropagation()}
      >
        <button
          ref={close}
          className="modal-close"
          aria-label={t("cards.close")}
          onClick={onClose}
        >
          <X />
        </button>
        <div
          ref={stage}
          className="card-inspector-stage"
          style={
            {
              "--card-zoom": zoom,
              "--card-fit-width": `${fitWidth}px`,
            } as CSSProperties
          }
        >
          <HouseCard
            card={card}
            src={art(selected)}
            color={color}
            motion={motion}
            cinematic
          />
        </div>
        <div className="card-inspector-copy">
          <p className="eyebrow">
            {t("cards.inspectorTitle")}
            {houseName}
          </p>
          <h2 id="card-title">{card.name}</h2>
          <p>
            {t("cards.strengthPrefix")}
            <b>{card.combatStrength || 0}</b>
            {t("cards.swordsSuffix")} {card.swordIcons || 0}
            {t("cards.fortificationsSuffix")}
            {card.towerIcons || 0}
          </p>
          <div className="card-inspector-controls" aria-label={t("cards.zoom")}>
            <button
              aria-label={t("cards.zoomOut")}
              disabled={zoom <= 0.8}
              onClick={() =>
                setZoom((value) =>
                  Math.max(0.8, Math.round((value - 0.2) * 10) / 10),
                )
              }
            >
              <Minus size={18} />
            </button>
            <output aria-label={t("cards.currentZoom")}>
              {Math.round(zoom * 100)}%
            </output>
            <button
              aria-label={t("cards.zoomIn")}
              disabled={zoom >= 2}
              onClick={() =>
                setZoom((value) =>
                  Math.min(2, Math.round((value + 0.2) * 10) / 10),
                )
              }
            >
              <Plus size={18} />
            </button>
            <button
              aria-label={t("cards.resetZoom")}
              onClick={() => setZoom(1)}
            >
              <RotateCcw size={16} />
            </button>
          </div>
          <p className="card-help">{t("cards.inspectionHint")}</p>
          <section className="card-rules" aria-label={t("cards.effect")}>
            <h3>
              {card.ability ? t("phases.cardEffect") : t("cards.combatIcons")}
            </h3>
            <p>{card.description || t("cards.standardIconsHint")}</p>
            {card.ability && (
              <small>
                {card.implemented
                  ? t("cards.abilityResolutionHint")
                  : t("cards.expansionAbilityUnavailable")}
              </small>
            )}
          </section>
          <div className="card-inspector-navigation">
            <button
              aria-label={t("cards.previous")}
              onClick={() =>
                onSelect(ids[(index - 1 + ids.length) % ids.length])
              }
            >
              <ChevronLeft size={18} />
            </button>
            <span>
              {index + 1} / {ids.length}
            </span>
            <button
              aria-label={t("cards.next")}
              onClick={() => onSelect(ids[(index + 1) % ids.length])}
            >
              <ChevronRight size={18} />
            </button>
          </div>
          <p className="card-help">{t("cards.keyboardHint")}</p>
        </div>
      </section>
    </div>
  );
}
