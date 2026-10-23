import { ArrowRight, Map, SkipForward, Swords } from "lucide-react";
import { useId } from "react";
import { t } from "../i18n/core";
import {
  regionName,
  getUnitNames,
  type Assets,
  type Catalog,
  type CombatRecord,
} from "../types";
import { Modal } from "../ui/Modal";
import { StrengthBreakdown } from "./StrengthBreakdown";
import "./combat.css";

export function CombatIntro({
  record,
  catalog,
  assets,
  motion,
  onNext,
  onMap,
  onDecision,
  approachViewed = false,
}: {
  record: CombatRecord;
  catalog: Catalog;
  assets: Assets;
  motion: boolean;
  onNext: () => void;
  onMap: () => void;
  onDecision?: () => void;
  approachViewed?: boolean;
}) {
  const titleID = useId();
  const name = (id: string) =>
    regionName(catalog.board.regions.find((region) => region.id === id));
  return (
    <Modal
      titleID={titleID}
      className={`combat-intro ${motion ? "" : "combat-static"}`}
      onClose={onNext}
    >
      <p className="eyebrow">
        {t("combat.attackAnnouncement", { p0: record.round, p1: record.id })}
      </p>
      <h2 id={titleID}>
        <Swords size={26} />
        {t("combat.attackOn", { p0: name(record.to) })}
      </h2>
      <div className="combat-route" aria-label={t("combat.route")}>
        <div>
          <img src={`/media/crests/${record.attacker}.svg`} alt="" />
          <span>
            <small>{catalog.houseNames[record.attacker]}</small>
            <b>{name(record.from)}</b>
          </span>
        </div>
        <ArrowRight className="combat-route-arrow" size={30} />
        <div>
          <img src={`/media/crests/${record.defender}.svg`} alt="" />
          <span>
            <small>{catalog.houseNames[record.defender]}</small>
            <b>{name(record.to)}</b>
          </span>
        </div>
      </div>
      <StrengthBreakdown
        value={record.initial}
        catalog={catalog}
        assets={assets}
      />
      <p className="combat-remaining">
        {t("combat.leftBehind", { p0: name(record.from) })}:{" "}
        {record.remaining.length
          ? record.remaining
              .map(
                (unit) => `${getUnitNames()[unit.unitType]} × ${unit.quantity}`,
              )
              .join(", ")
          : t("board.noUnits")}
      </p>
      <p className="combat-secret-note">{t("combat.readingDoesNotPause")}</p>
      <footer className="combat-intro-actions">
        <button onClick={onMap}>
          <Map size={17} />
          {t("combat.showOnMap")}
        </button>
        {onDecision && (
          <button onClick={onDecision}>{t("combat.yourDecision")}</button>
        )}
        {!approachViewed && (
          <button onClick={onNext}>{t("animation.skipIntro")}</button>
        )}
        <button
          className="primary"
          data-initial-focus
          onClick={approachViewed ? onNext : onMap}
        >
          <SkipForward size={17} />
          {approachViewed ? t("ui.continue") : t("animation.watchAttack")}
        </button>
      </footer>
    </Modal>
  );
}
