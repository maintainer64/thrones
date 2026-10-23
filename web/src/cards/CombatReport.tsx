import { useEffect, useState } from "react";
import { ArrowRight, Pause, Play, Swords, X } from "lucide-react";
import type { Assets, Catalog, CombatRecord, Match } from "../types";
import { getUnitNames, regionName } from "../types";
import { t } from "../i18n/core";
import { Modal } from "../ui/Modal";
import { StrengthShield } from "../ui/StrengthShield";
import { HouseCard } from "./HouseCard";
import { TidesCard } from "./TidesCard";
import { StrengthBreakdown } from "../combat/StrengthBreakdown";
import {
  ReportStage,
  strengthAtStep,
  strengthStepCount,
} from "../combat/animation";

export function CombatReport({
  result,
  catalog,
  art,
  motion,
  onClose,
  assets,
  record,
  onMap,
  onDecision,
  stage = ReportStage.Outcome,
  onStage,
}: {
  result: NonNullable<Match["lastCombat"]>;
  catalog: Catalog;
  art: (card: string, house: string) => string | undefined;
  motion: boolean;
  onClose: () => void;
  assets: Assets;
  record?: CombatRecord;
  onMap?: () => void;
  onDecision?: () => void;
  stage?: ReportStage;
  onStage?: (stage: ReportStage) => void;
}) {
  const [turned, setTurned] = useState(false);
  const [step, setStep] = useState(0);
  const [playing, setPlaying] = useState(true);
  const revealed = turned || stage !== ReportStage.Cards;
  const outcome = stage === ReportStage.Outcome;
  const count = result.strength ? strengthStepCount(result.strength) : 0;
  const visibleCount = outcome || !motion ? count : step;
  const strength = result.strength
    ? strengthAtStep(result.strength, visibleCount)
    : undefined;
  useEffect(() => {
    if (stage !== ReportStage.Strength || !playing || !motion || step >= count)
      return;
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible")
        setStep((value) => Math.min(count, value + 1));
    }, 650);
    return () => window.clearInterval(timer);
  }, [stage, playing, motion, count, step]);
  return (
    <Modal
      titleID="report-title"
      focusKey={stage}
      className={`combat-report ${motion ? "" : "combat-static"}`}
      onClose={onClose}
    >
      <button
        className="modal-close"
        aria-label={t("combat.closeResult")}
        onClick={onClose}
      >
        <X />
      </button>
      <div className="combat-scroll">
        <Swords
          className="combat-crossed-swords"
          size={112}
          aria-hidden="true"
        />
        <p className="eyebrow">{t("combat.areaHeading")}</p>
        <h2 id="report-title">
          {regionName(
            catalog.board.regions.find((region) => region.id === result.region),
          )}
        </h2>
        <span>{t("phases.combat")}</span>
      </div>
      {onStage && (
        <ol className="combat-stages" aria-label={t("animation.stages")}>
          {[ReportStage.Cards, ReportStage.Strength, ReportStage.Outcome].map(
            (item, index) => (
              <li key={item} aria-current={stage === item ? "step" : undefined}>
                <span>{index + 1}</span>
                {item === ReportStage.Cards
                  ? t("animation.cards")
                  : item === ReportStage.Strength
                    ? t("combat.breakdown")
                    : t("animation.outcome")}
              </li>
            ),
          )}
        </ol>
      )}
      <div className="combat-result-cards">
        {[result.attacker, result.defender].map((house, index) => {
          const winner = result.winner === house;
          const cardID =
            index === 0 ? result.attackerCard : result.defenderCard;
          const card = catalog.houses[house].houseCards[cardID];
          return (
            <div
              key={house}
              className={
                outcome
                  ? winner
                    ? "victorious"
                    : "defeated"
                  : "combat-unresolved"
              }
            >
              <div className="combat-house-plaque">
                <StrengthShield
                  value={
                    outcome
                      ? index === 0
                        ? result.attack
                        : result.defense
                      : stage === ReportStage.Strength && strength
                        ? index === 0
                          ? strength.attacker.total
                          : strength.defender.total
                        : record
                          ? index === 0
                            ? record.initial.attacker.total
                            : record.initial.defender.total
                          : 0
                  }
                  label={
                    outcome
                      ? t("combat.total")
                      : stage === ReportStage.Strength
                        ? t("animation.intermediate")
                        : t("combat.publicStrength")
                  }
                />
                <div>
                  <small>
                    {index === 0 ? t("combat.attacker") : t("combat.defender")}
                  </small>
                  <h3>
                    <img
                      src={`/media/crests/${house}.svg`}
                      width={24}
                      height={24}
                      alt=""
                    />
                    {catalog.houseNames[house]}
                  </h3>
                </div>
              </div>
              <div className="combat-card-tableau">
                <div className="combat-card-flip" data-revealed={revealed}>
                  {!revealed && (
                    <div
                      className="combat-card-back"
                      aria-label={t("animation.hiddenCard")}
                    >
                      <img src={`/media/crests/${house}.svg`} alt="" />
                      <span>{catalog.houseNames[house]}</span>
                    </div>
                  )}
                  {revealed &&
                    (card ? (
                      <HouseCard
                        card={card}
                        src={art(cardID, house)}
                        color={catalog.houses[house].color}
                        motion={motion}
                      />
                    ) : (
                      <p className="no-combat-card">
                        {t("combat.cancelledCard")}
                      </p>
                    ))}
                </div>
                {outcome &&
                  (winner ? (
                    <span className="combat-victory-banner">
                      {t("ui.victory")}
                    </span>
                  ) : (
                    <span
                      className="combat-defeat-slash"
                      aria-label={t("ui.defeat")}
                    />
                  ))}
              </div>
              {revealed && result.modifiers?.[house] && (
                <TidesCard
                  card={result.modifiers[house]}
                  houseName={catalog.houseNames[house]}
                />
              )}
              {revealed && result.bladeBonus === house && (
                <small className="tides-blade-note">
                  {t("tides.bladeBonus")}
                </small>
              )}
              {revealed && result.bladeRedraw === house && (
                <small className="tides-blade-note">
                  {t("tides.bladeRedrawn")}
                </small>
              )}
            </div>
          );
        })}
      </div>
      {record && (
        <p className="combat-remaining">
          {regionName(
            catalog.board.regions.find((region) => region.id === record.from),
          )}{" "}
          →{" "}
          {regionName(
            catalog.board.regions.find((region) => region.id === record.to),
          )}
        </p>
      )}
      {stage === ReportStage.Strength && (
        <div className="combat-calculation-controls">
          <p role="status">
            {t("animation.step", { p0: visibleCount, p1: count })} ·{" "}
            {t("animation.intermediate")}
          </p>
          <button
            onClick={() => setPlaying(!playing)}
            disabled={!motion || visibleCount >= count}
            aria-label={playing ? t("animation.pause") : t("animation.play")}
          >
            {playing ? <Pause size={17} /> : <Play size={17} />}
          </button>
          <button
            onClick={() => setStep(count)}
            disabled={visibleCount >= count}
          >
            {t("animation.skipCalculation")}
          </button>
        </div>
      )}
      {stage !== ReportStage.Cards && strength && (
        <details className="combat-breakdown-details" open>
          <summary>{t("combat.breakdown")}</summary>
          <StrengthBreakdown
            value={outcome ? result.strength! : strength}
            catalog={catalog}
            assets={assets}
            visibleCount={outcome ? undefined : visibleCount}
          />
        </details>
      )}
      {outcome && result.losses && (
        <section
          className="combat-casualties"
          aria-label={t("combat.casualties")}
        >
          <h3>{t("combat.casualties")}</h3>
          {[result.attacker, result.defender].map((house) => (
            <p key={house}>
              <b>{catalog.houseNames[house]}</b>:{" "}
              {result.losses?.[house]?.length
                ? result.losses[house]
                    .map(
                      (unit) =>
                        `${getUnitNames()[unit.unitType]} × ${unit.quantity}`,
                    )
                    .join(", ")
                : t("combat.noCasualties")}
            </p>
          ))}
          {result.retreatHouse && (
            <p>
              {result.retreatTo
                ? t("combat.retreatTo", {
                    p0: catalog.houseNames[result.retreatHouse],
                    p1: regionName(
                      catalog.board.regions.find(
                        (region) => region.id === result.retreatTo,
                      ),
                    ),
                  })
                : t("combat.retreatPending", {
                    p0: catalog.houseNames[result.retreatHouse],
                  })}
            </p>
          )}
          {result.returnTo && (
            <p>
              {t("combat.returnTo", {
                p0: catalog.houseNames[result.attacker],
                p1: regionName(
                  catalog.board.regions.find(
                    (region) => region.id === result.returnTo,
                  ),
                ),
              })}
            </p>
          )}
        </section>
      )}
      {outcome && !!result.effects?.length && (
        <details className="combat-effects">
          <summary>{t("cards.effects")}</summary>
          <ul>
            {result.effects.map((text, index) => (
              <li key={index}>{text}</li>
            ))}
          </ul>
        </details>
      )}
      <button
        className="primary combat-continue"
        onClick={
          stage === ReportStage.Cards
            ? () =>
                revealed ? onStage?.(ReportStage.Strength) : setTurned(true)
            : stage === ReportStage.Strength
              ? onMap || (() => onStage?.(ReportStage.Outcome))
              : onClose
        }
      >
        {stage === ReportStage.Cards
          ? revealed
            ? t("animation.toStrength")
            : t("animation.reveal")
          : stage === ReportStage.Strength
            ? onMap
              ? t("animation.watchLosses")
              : t("animation.toOutcome")
            : t("ui.continue")}
        <ArrowRight size={18} />
      </button>
      <div className="combat-intro-actions">
        {onMap && outcome && (
          <button onClick={onMap}>{t("animation.watchLosses")}</button>
        )}
        {!outcome && onStage && (
          <button onClick={() => onStage(ReportStage.Outcome)}>
            {t("animation.toOutcome")}
          </button>
        )}
        {onDecision && (
          <button onClick={onDecision}>{t("combat.yourDecision")}</button>
        )}
      </div>
    </Modal>
  );
}
