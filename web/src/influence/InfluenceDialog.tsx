import { useState } from "react";
import {
  Check,
  Coins,
  Crown,
  Feather,
  LockKeyhole,
  Swords,
  X,
} from "lucide-react";
import { AuctionStage, InfluenceTrack } from "../enums";
import { setLanguage, t } from "../i18n/core";
import { useLanguage } from "../i18n/useLanguage";
import type { Catalog, InfluenceResult, Room } from "../types";
import { Modal } from "../ui/Modal";
import { parseBid } from "./bid";
import "./influence.css";

export function influenceTrackName(track: InfluenceTrack) {
  switch (track) {
    case InfluenceTrack.IronThrone:
      return t("tracks.ironThrone");
    case InfluenceTrack.Fiefdoms:
      return t("tracks.fiefdoms");
    case InfluenceTrack.KingsCourt:
      return t("tracks.kingsCourt");
  }
}

function TrackIcon({ track }: { track: InfluenceTrack }) {
  return track === InfluenceTrack.IronThrone ? (
    <Crown />
  ) : track === InfluenceTrack.Fiefdoms ? (
    <Swords />
  ) : (
    <Feather />
  );
}

function House({ id, catalog }: { id: string; catalog: Catalog }) {
  return (
    <span className="influence-house">
      <img src={`/media/crests/${id}.svg`} alt="" width={28} height={28} />
      {catalog.houseNames[id]}
    </span>
  );
}

function DialogHeader({
  title,
  onClose,
}: {
  title: string;
  onClose: () => void;
}) {
  const language = useLanguage();
  return (
    <div className="modal-top">
      <div>
        <p className="eyebrow">{t("influence.eyebrow")}</p>
        <h2 id="influence-title">{title}</h2>
      </div>
      <div className="influence-header-actions">
        <button
          type="button"
          aria-label={t("settings.changeLanguage")}
          onClick={() => setLanguage(language === "ru" ? "en" : "ru")}
        >
          {language.toUpperCase()}
        </button>
        <button type="button" aria-label={t("common.close")} onClick={onClose}>
          <X size={18} />
        </button>
      </div>
    </div>
  );
}

function InfluenceResults({
  results,
  catalog,
}: {
  results: InfluenceResult[];
  catalog: Catalog;
}) {
  return (
    <div className="influence-results">
      {results.map((result) => (
        <section key={result.track}>
          <h3>
            <TrackIcon track={result.track} />
            {influenceTrackName(result.track)}
          </h3>
          <ol>
            {result.ranking.map((house, index) => (
              <li key={house}>
                <span className="influence-rank">{index + 1}</span>
                <House id={house} catalog={catalog} />
                <span className="influence-result-bid">
                  <Coins size={15} />
                  {t("influence.bidValue", { p0: result.bids[house] })}
                </span>
              </li>
            ))}
          </ol>
        </section>
      ))}
    </div>
  );
}

export function InfluenceReport({
  room,
  catalog,
  onClose,
}: {
  room: Room;
  catalog: Catalog;
  onClose: () => void;
}) {
  useLanguage();
  return (
    <Modal
      titleID="influence-title"
      className="influence-report"
      onClose={onClose}
    >
      <DialogHeader title={t("influence.resultsTitle")} onClose={onClose} />
      <p className="subtle">{t("influence.resultsHint")}</p>
      <InfluenceResults
        results={room.match?.lastInfluence || []}
        catalog={catalog}
      />
      <button className="primary" type="button" onClick={onClose}>
        {t("influence.backToBoard")}
      </button>
    </Modal>
  );
}

export function InfluenceDialog({
  room,
  catalog,
  busy,
  error,
  onCommand,
  onClose,
}: {
  room: Room;
  catalog: Catalog;
  busy: boolean;
  error: string;
  onCommand: (command: Record<string, unknown>) => void;
  onClose: () => void;
}) {
  useLanguage();
  const [draft, setDraft] = useState("0");
  const [confirming, setConfirming] = useState(false);
  const auction = room.influence!;
  const available = room.match?.power[room.yourHouse] ?? 0;
  const own = auction.bids.yourBid;
  const committed = own !== undefined;
  const bidding = auction.stage === AuctionStage.Bidding;
  const amount = parseBid(draft, available);
  const ready = Object.values(auction.bids.submitted).filter(Boolean).length;
  const houses = room.seats.map((seat) => seat.house);
  const total = houses.length;
  const rankedHouses = [
    ...auction.ranking,
    ...houses.filter((house) => !auction.ranking.includes(house)),
  ];
  const canBreakTie = !bidding && auction.tieBreakHouse === room.yourHouse;
  const holder = catalog.houseNames[auction.tieBreakHouse];
  const steps = [
    InfluenceTrack.IronThrone,
    InfluenceTrack.Fiefdoms,
    InfluenceTrack.KingsCourt,
  ];
  const step = steps.indexOf(auction.track);

  return (
    <Modal
      titleID="influence-title"
      focusKey={`${auction.stage}:${committed}:${confirming}:${auction.ranking.length}`}
      focusReady={!busy}
      onClose={onClose}
    >
      <DialogHeader
        title={influenceTrackName(auction.track)}
        onClose={onClose}
      />
      <ol className="influence-steps" aria-label={t("influence.trackOrder")}>
        {steps.map((track, index) => (
          <li
            key={track}
            aria-current={index === step ? "step" : undefined}
            data-complete={index < step}
          >
            {index < step ? <Check /> : <TrackIcon track={track} />}
            <span>{influenceTrackName(track)}</span>
          </li>
        ))}
      </ol>
      <div className="influence-layout">
        <div className="influence-decision">
          <p className="influence-balance">
            <Coins size={19} />
            {t("influence.availablePower")}
            <strong data-testid="available-power">{available}</strong>
          </p>
          {bidding ? (
            committed ? (
              <div className="influence-committed" role="status">
                <LockKeyhole size={32} />
                <h3>{t("influence.committedTitle")}</h3>
                <strong data-testid="your-bid">{own}</strong>
                <p>{t("influence.committedHint")}</p>
              </div>
            ) : (
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  if (amount !== undefined && !busy) setConfirming(true);
                }}
              >
                <label htmlFor="influence-bid">{t("influence.yourBid")}</label>
                <input
                  id="influence-bid"
                  type="number"
                  inputMode="numeric"
                  min={0}
                  max={available}
                  step={1}
                  value={draft}
                  disabled={busy || confirming}
                  data-initial-focus={!confirming ? "" : undefined}
                  aria-describedby="influence-bid-hint"
                  onChange={(event) => setDraft(event.target.value)}
                />
                <div className="influence-presets">
                  {[...new Set([0, Math.floor(available / 2), available])].map(
                    (value) => (
                      <button
                        key={value}
                        type="button"
                        disabled={busy || confirming}
                        aria-label={t("influence.preset", { p0: value })}
                        onClick={() => setDraft(String(value))}
                      >
                        {value}
                      </button>
                    ),
                  )}
                </div>
                <p id="influence-bid-hint" className="subtle">
                  {t("influence.secretHint")}
                </p>
                {confirming ? (
                  <div
                    className="influence-confirm"
                    role="group"
                    aria-label={t("influence.confirmTitle")}
                  >
                    <h3>{t("influence.confirmAmount", { p0: amount })}</h3>
                    <p>{t("influence.irreversibleHint")}</p>
                    <button
                      type="button"
                      className="primary"
                      data-initial-focus
                      disabled={busy || amount === undefined}
                      onClick={() => {
                        if (amount !== undefined)
                          onCommand({ type: "bid", bid: amount });
                      }}
                    >
                      <LockKeyhole size={16} />
                      {busy
                        ? t("influence.sending")
                        : t("influence.confirmBid")}
                    </button>
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => setConfirming(false)}
                    >
                      {t("influence.editBid")}
                    </button>
                  </div>
                ) : (
                  <button
                    className="primary"
                    type="submit"
                    disabled={busy || amount === undefined}
                  >
                    {t("influence.submitBid")}
                  </button>
                )}
              </form>
            )
          ) : (
            <div className="influence-tie-decision">
              <Crown size={30} />
              <h3>
                {canBreakTie
                  ? t("influence.chooseRank", {
                      p0: auction.ranking.length + 1,
                    })
                  : t("influence.waitingForHolder", { p0: holder })}
              </h3>
              <p>{t("influence.tieRule", { p0: holder })}</p>
              {canBreakTie && (
                <div className="influence-tie-options">
                  {auction.tied.map((house, index) => (
                    <button
                      type="button"
                      key={house}
                      data-initial-focus={index === 0 ? "" : undefined}
                      disabled={busy}
                      aria-label={t("influence.rankHouse", {
                        p0: catalog.houseNames[house],
                      })}
                      onClick={() =>
                        onCommand({ type: "influence_tie", target: house })
                      }
                    >
                      <House id={house} catalog={catalog} />
                      <span>
                        {t("influence.assignRank", {
                          p0: auction.ranking.length + 1,
                        })}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
          {error && (
            <p className="influence-error" role="alert">
              {error}
            </p>
          )}
          <p className="subtle influence-clock-hint">
            {room.settings.hardTimer
              ? t("influence.hardTimerHint")
              : t("influence.softTimerHint")}
          </p>
        </div>
        <section
          className="influence-participants"
          aria-label={t("influence.participants")}
        >
          <h3>
            {bidding
              ? t("influence.readyCount", { p0: ready, p1: total })
              : t("influence.revealedTitle")}
          </h3>
          {bidding && (
            <progress
              max={total}
              value={ready}
              aria-label={t("influence.readyCount", { p0: ready, p1: total })}
            />
          )}
          <ul>
            {rankedHouses.map((house) => {
              const rank = auction.ranking.indexOf(house);
              const value = auction.bids.values?.[house];
              return (
                <li
                  key={house}
                  data-house={house}
                  data-ranked={rank >= 0}
                  data-tied={auction.tied.includes(house)}
                >
                  {rank >= 0 && <b className="influence-rank">{rank + 1}</b>}
                  <House id={house} catalog={catalog} />
                  {value !== undefined ? (
                    <strong data-testid={`revealed-bid-${house}`}>
                      {value}
                    </strong>
                  ) : (
                    <span className="influence-bid-status">
                      {auction.bids.submitted[house] ? (
                        <Check size={16} />
                      ) : (
                        <LockKeyhole size={14} />
                      )}
                      {auction.bids.submitted[house]
                        ? t("influence.bidReady")
                        : t("influence.bidWaiting")}
                    </span>
                  )}
                </li>
              );
            })}
          </ul>
          <p className="subtle">{t("influence.holderLabel", { p0: holder })}</p>
        </section>
      </div>
      {auction.results.length > 0 && (
        <details className="influence-previous">
          <summary>{t("influence.previousTracks")}</summary>
          <InfluenceResults results={auction.results} catalog={catalog} />
        </details>
      )}
    </Modal>
  );
}
