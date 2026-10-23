import { useEffect } from "react";
import { Pause, Play, SkipForward } from "lucide-react";
import { ReplayPhase, type CombatPlayback } from "./animation";
import { t } from "../i18n/core";
import { regionName, type Catalog } from "../types";

export function PlaybackControls({
  value,
  catalog,
  motion,
  onPause,
  onSkip,
  onContinue,
  onDecision,
}: {
  value: CombatPlayback;
  catalog: Catalog;
  motion: boolean;
  onPause: () => void;
  onSkip: () => void;
  onContinue: () => void;
  onDecision?: () => void;
}) {
  useEffect(() => {
    if (!motion && !value.complete) onSkip();
  }, [motion, value.serial, value.complete]);
  const name = (id: string) =>
    regionName(catalog.board.regions.find((region) => region.id === id));
  return (
    <section
      className="combat-map-banner combat-playback-controls"
      aria-label={t("animation.replay")}
      data-state={
        value.complete ? "complete" : value.playing ? "playing" : "paused"
      }
    >
      <div>
        <strong>
          {t("combat.attackAnnouncement", {
            p0: value.record.round,
            p1: value.record.id,
          })}{" "}
          ·{" "}
          {value.phase === ReplayPhase.Approach
            ? t("animation.movement")
            : t("animation.losses")}
        </strong>
        <span>
          {name(value.record.from)} → {name(value.record.to)}
        </span>
        <small>{t("animation.snapshotOnly")}</small>
        {value.complete && (
          <small role="status">{t("animation.finished")}</small>
        )}
      </div>
      <button
        onClick={onPause}
        disabled={value.complete}
        aria-label={value.playing ? t("animation.pause") : t("animation.play")}
      >
        {value.playing ? <Pause size={17} /> : <Play size={17} />}
      </button>
      <button onClick={onSkip} disabled={value.complete}>
        <SkipForward size={17} />
        {t("animation.skip")}
      </button>
      {onDecision && (
        <button onClick={onDecision}>{t("combat.yourDecision")}</button>
      )}
      <button className="primary" onClick={onContinue}>
        {t("combat.resumePresentation")}
      </button>
    </section>
  );
}
