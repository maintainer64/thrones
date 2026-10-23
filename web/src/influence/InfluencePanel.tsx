import { Crown, Feather, Swords, X } from "lucide-react";
import { InfluenceTrack } from "../enums";
import { influenceTrackName } from "./InfluenceDialog";
import type { Catalog, Room } from "../types";
import { t } from "../i18n/core";

export function InfluencePanel({
  room,
  catalog,
  onClose,
}: {
  room: Room;
  catalog: Catalog;
  onClose: () => void;
}) {
  return (
    <aside
      className="influence-panel"
      aria-label={t("tracks.title")}
      data-tour="influence"
    >
      <button
        className="hud-close-panel"
        aria-label={t("ui.closeInfluence")}
        onClick={onClose}
      >
        <X size={16} />
      </button>
      <p className="eyebrow">{t("ui.smallCouncil")}</p>
      <h2>{t("tracks.title")}</h2>
      {room.match || room.setup ? (
        <div className="influence-columns">
          {[
            { track: InfluenceTrack.IronThrone, Emblem: Crown },
            { track: InfluenceTrack.Fiefdoms, Emblem: Swords },
            { track: InfluenceTrack.KingsCourt, Emblem: Feather },
          ].map(({ track, Emblem }) => {
            const used =
              (track === InfluenceTrack.KingsCourt && room.match?.ravenUsed) ||
              (track === InfluenceTrack.Fiefdoms && room.match?.bladeUsed);
            return (
              <section key={track} aria-label={influenceTrackName(track)}>
                <div className="track-emblem" title={influenceTrackName(track)}>
                  <Emblem aria-hidden="true" size={25} />
                </div>
                <h3>{influenceTrackName(track)}</h3>
                <div className="track-status" data-used={!!used}>
                  {track !== InfluenceTrack.IronThrone &&
                    (used
                      ? t("tracks.tokenSpent")
                      : t("tracks.tokenAvailable"))}
                </div>
                <ol>
                  {(
                    (room.match?.tracks || room.setup?.tracks)?.[track] || []
                  ).map((house, index) => (
                    <li
                      key={house}
                      data-yours={house === room.yourHouse}
                      title={`${index + 1}. ${catalog.houseNames[house]}`}
                    >
                      <small>{index + 1}</small>
                      {track === InfluenceTrack.KingsCourt && (
                        <small
                          className="track-stars"
                          aria-label={t("setup.stars", {
                            p0: room.courtStars?.[index] || 0,
                          })}
                        >
                          {"★".repeat(room.courtStars?.[index] || 0) || "—"}
                        </small>
                      )}
                      <img
                        src={`/media/crests/${house}.svg`}
                        alt={catalog.houseNames[house]}
                        width={32}
                        height={32}
                      />
                      {index === 0 && (
                        <span
                          className="track-holder"
                          aria-label={t("ui.tokenHolder")}
                        />
                      )}
                    </li>
                  ))}
                </ol>
              </section>
            );
          })}
        </div>
      ) : (
        <p className="influence-empty">{t("ui.tracksAfterStart")}</p>
      )}
      {room.match && (
        <div className="influence-panel-note">
          <Crown size={16} />
          <p>{t("ui.tracksHint")}</p>
        </div>
      )}
    </aside>
  );
}
