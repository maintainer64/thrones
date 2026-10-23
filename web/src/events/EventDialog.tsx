import { useState } from "react";
import {
  Check,
  Coins,
  LockKeyhole,
  MapPin,
  ScrollText,
  Shield,
  X,
} from "lucide-react";
import { InfluenceTrack, WildlingChoiceKind } from "../enums";
import type { WildlingCardID, WesterosCardID } from "../enums";
import { t, setLanguage } from "../i18n/core";
import { useLanguage } from "../i18n/useLanguage";
import { influenceTrackName } from "../influence/InfluenceDialog";
import { parseBid } from "../influence/bid";
import { HouseCard } from "../cards/HouseCard";
import { Modal } from "../ui/Modal";
import { getUnitNames, regionName } from "../types";
import type { Catalog, Room, SealedBidsView, WildlingChoice } from "../types";
import {
  cardImage,
  cardName,
  cardDescription,
  wildlingDescription,
  choiceTitle,
  decisionName,
  eventText as et,
  finishReward,
  readableEvent,
  eventProgress,
} from "./model";
import type { EventKey } from "./model";
import "./events.css";

type Command = (value: Record<string, unknown>) => void;

export function EventArt({ id }: { id: WesterosCardID | WildlingCardID }) {
  return (
    <div className="events-art" aria-hidden="true">
      <img
        src={cardImage(id)}
        alt=""
        onError={(event) => {
          event.currentTarget.hidden = true;
        }}
      />
    </div>
  );
}

function House({ id, catalog }: { id: string; catalog: Catalog }) {
  return (
    <span className="influence-house">
      <img src={`/media/crests/${id}.svg`} width={26} height={26} alt="" />
      {catalog.houseNames[id] || id}
    </span>
  );
}

function BidLedger({
  bids,
  catalog,
}: {
  bids: SealedBidsView;
  catalog: Catalog;
}) {
  return (
    <section className="events-ledger" aria-label={et("biddingTitle")}>
      <h3>
        {et("ready", {
          p0: Object.values(bids.submitted).filter(Boolean).length,
          p1: Object.keys(bids.submitted).length,
        })}
      </h3>
      <ul>
        {Object.entries(bids.submitted).map(([house, ready]) => (
          <li key={house}>
            <House id={house} catalog={catalog} />
            <span data-testid={`watch-bid-${house}`}>
              {bids.values?.[house] !== undefined ? (
                bids.values[house]
              ) : ready ? (
                <>
                  <Check size={15} />
                  {et("readyHouse")}
                </>
              ) : (
                et("pendingHouse")
              )}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function WatchBid({
  room,
  catalog,
  busy,
  onCommand,
}: {
  room: Room;
  catalog: Catalog;
  busy: boolean;
  onCommand: Command;
}) {
  const [draft, setDraft] = useState("0");
  const [confirm, setConfirm] = useState(false);
  const attack = room.wildlings!.attack!;
  const balance = room.match?.power[room.yourHouse] ?? 0;
  const amount = parseBid(draft, balance);
  const own = attack.bids.yourBid;
  const participant = Object.hasOwn(attack.bids.submitted, room.yourHouse);
  return (
    <div className="events-bid-layout">
      <section className="events-decision">
        <p className="influence-balance">
          <Coins size={19} />
          {et("available")}
          <strong data-testid="watch-power">{balance}</strong>
        </p>
        {!participant ? (
          <p role="status">{et("automatic")}</p>
        ) : own !== undefined ? (
          <div className="influence-committed" role="status">
            <LockKeyhole />
            <h3>{et("bidLocked")}</h3>
            <strong data-testid="watch-your-bid">{own}</strong>
            <p>{et("bidHint")}</p>
          </div>
        ) : (
          <form
            onSubmit={(event) => {
              event.preventDefault();
              if (amount !== undefined && !busy) setConfirm(true);
            }}
          >
            <label htmlFor="watch-bid">{et("bid")}</label>
            <input
              id="watch-bid"
              type="number"
              inputMode="numeric"
              min={0}
              max={balance}
              step={1}
              value={draft}
              disabled={busy || confirm}
              data-initial-focus={!confirm ? "" : undefined}
              aria-describedby="watch-bid-hint"
              onChange={(event) => setDraft(event.target.value)}
            />
            <div className="influence-presets">
              {[...new Set([0, Math.floor(balance / 2), balance])].map(
                (value) => (
                  <button
                    key={value}
                    type="button"
                    disabled={busy || confirm}
                    onClick={() => setDraft(String(value))}
                  >
                    {value}
                  </button>
                ),
              )}
            </div>
            <p id="watch-bid-hint" className="subtle">
              {amount === undefined
                ? et("bidInvalid", { p0: balance })
                : et("bidHint")}
            </p>
            {confirm ? (
              <div className="influence-confirm">
                <h3>{et("bidConfirmTitle", { p0: amount })}</h3>
                <button
                  type="button"
                  className="primary"
                  data-initial-focus
                  autoFocus
                  disabled={busy || amount === undefined}
                  onClick={() => {
                    if (amount !== undefined)
                      onCommand({ type: "wildling_bid", bid: amount });
                  }}
                >
                  {et("bidConfirm")}
                </button>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => setConfirm(false)}
                >
                  {et("bidEdit")}
                </button>
              </div>
            ) : (
              <button
                className="primary"
                type="submit"
                disabled={busy || amount === undefined}
              >
                {et("bidContinue")}
              </button>
            )}
          </form>
        )}
      </section>
      <BidLedger bids={attack.bids} catalog={catalog} />
    </div>
  );
}

type Option = {
  id: string;
  title: string;
  region?: string;
  content?: React.ReactNode;
  detail?: string;
};
function Options({
  options,
  ours,
  busy,
  waiting,
  onChoose,
  onShowRegion,
}: {
  options: Option[];
  ours: boolean;
  busy: boolean;
  waiting: string;
  onChoose: (value: string) => void;
  onShowRegion: (id: string) => void;
}) {
  const [selected, setSelected] = useState("");
  const option = options.find((value) => value.id === selected);
  return (
    <>
      {!ours && (
        <p className="events-wait" role="status">
          {waiting}
        </p>
      )}
      <div className="events-options" role="group">
        {options.map((item) => (
          <div
            key={item.id}
            className={`events-option ${item.content ? "events-card-option" : ""}`}
          >
            <button
              type="button"
              aria-pressed={selected === item.id}
              disabled={!ours || busy}
              onClick={() => setSelected(item.id)}
              aria-label={item.title}
            >
              {item.content}
              <span>{item.title}</span>
              {item.detail && <small>{item.detail}</small>}
            </button>
            {item.region && (
              <button
                className="subtle-button events-show-region"
                type="button"
                aria-label={`${et("region")}: ${item.title}`}
                onClick={() => onShowRegion(item.region!)}
              >
                <MapPin size={15} />
                {et("region")}
              </button>
            )}
          </div>
        ))}
      </div>
      {ours && option && (
        <div className="events-confirm" role="status">
          <strong>{option.title}</strong>
          <p>{et("choiceHint")}</p>
          <button
            className="primary"
            type="button"
            disabled={busy}
            onClick={() => onChoose(option.id)}
          >
            {et("choiceConfirm")}
          </button>
        </div>
      )}
    </>
  );
}

function EffectChoice({
  room,
  catalog,
  choice,
  busy,
  onCommand,
  onShowRegion,
  art,
}: {
  room: Room;
  catalog: Catalog;
  choice: WildlingChoice;
  busy: boolean;
  onCommand: Command;
  onShowRegion: (id: string) => void;
  art: (id: string, house: string) => string | undefined;
}) {
  const names = getUnitNames();
  const cardChoice = [
    WildlingChoiceKind.Discard,
    WildlingChoiceKind.Retrieve,
  ].includes(choice.kind);
  const options: Option[] = choice.options.map((id) => {
    if (id === finishReward) return { id, title: et("finish") };
    if (choice.kind === WildlingChoiceKind.Track)
      return { id, title: influenceTrackName(id as InfluenceTrack) };
    if (choice.kind === WildlingChoiceKind.Penalty)
      return { id, title: et(id as "destroy-units" | "lose-influence") };
    if (cardChoice) {
      const card = catalog.houses[choice.house]?.houseCards[id];
      return {
        id,
        title: card?.name || id,
        content: card && (
          <HouseCard
            card={card}
            src={art(id, choice.house)}
            color={catalog.houses[choice.house].color}
            motion={false}
          />
        ),
      };
    }
    const unit = choice.unitOptions?.find((value) => value.id === id);
    const region = catalog.board.regions.find(
      (value) => value.id === (unit?.region || id),
    );
    const title = regionName(region);
    return {
      id,
      region: region?.id,
      title: unit
        ? et("unitOption", { p0: title, p1: names[unit.unit] })
        : title,
      detail: unit ? et("unitsAvailable", { p0: unit.count }) : undefined,
    };
  });
  return (
    <section className="events-decision" data-testid="watch-choice">
      <h3>{choiceTitle(choice.kind)}</h3>
      {choice.count !== undefined && (
        <p>{et("remaining", { p0: choice.count })}</p>
      )}
      {choice.kind === WildlingChoiceKind.Penalty && (
        <p className="subtle">{et("partialPenalty")}</p>
      )}
      <Options
        options={options}
        ours={choice.house === room.yourHouse}
        busy={busy}
        waiting={et("waiting", { p0: catalog.houseNames[choice.house] })}
        onShowRegion={onShowRegion}
        onChoose={(choice) => onCommand({ type: "wildling_choice", choice })}
      />
    </section>
  );
}

function RevealedAttack({
  room,
  catalog,
  art,
  busy,
  onCommand,
  onShowRegion,
}: {
  room: Room;
  catalog: Catalog;
  busy: boolean;
  onCommand: Command;
  onShowRegion: (id: string) => void;
  art: (id: string, house: string) => string | undefined;
}) {
  const attack = room.wildlings!.attack!;
  const won = attack.outcome?.watchWon || false;
  return (
    <>
      {attack.outcome && (
        <div className={`events-outcome ${won ? "won" : "lost"}`}>
          <Shield size={24} />
          <div>
            <h3>{et(won ? "won" : "lost")}</h3>
            <p>
              {et("total", { p0: attack.outcome.total, p1: attack.strength })}
            </p>
            {attack.card && (
              <p>
                {et(won ? "reward" : "penalty", {
                  p0: catalog.houseNames[attack.outcome.selectedHouse],
                })}
              </p>
            )}
          </div>
        </div>
      )}
      {room.phase === "wildling_ties" && (
        <section className="events-decision">
          <p>{et(won ? "highestTie" : "lowestTie")}</p>
          <Options
            options={attack.tied.map((id) => ({
              id,
              title: et("chooseHouse", { p0: catalog.houseNames[id] }),
              content: <House id={id} catalog={catalog} />,
            }))}
            ours={attack.tieBreakHouse === room.yourHouse}
            busy={busy}
            waiting={et("waiting", {
              p0: catalog.houseNames[attack.tieBreakHouse],
            })}
            onShowRegion={onShowRegion}
            onChoose={(target) => onCommand({ type: "wildling_tie", target })}
          />
        </section>
      )}
      {attack.card ? (
        <article
          className="events-wildling-card"
          data-testid="watch-revealed-card"
        >
          <EventArt id={attack.card} />
          <div>
            <p className="eyebrow">{et("effectsTitle")}</p>
            <h3>{cardName(attack.card)}</h3>
            <p>{wildlingDescription(attack.card, won)}</p>
          </div>
        </article>
      ) : (
        <div className="events-secret" data-testid="watch-hidden-card">
          <LockKeyhole />
          <h3>{et("secretCard")}</h3>
          <p>{et("secretHint")}</p>
        </div>
      )}
      {attack.choice && (
        <EffectChoice
          room={room}
          catalog={catalog}
          choice={attack.choice}
          busy={busy}
          art={art}
          onCommand={onCommand}
          onShowRegion={onShowRegion}
        />
      )}
      {room.phase === "wildling_effects" && !attack.choice && (
        <p role="status">{et("automatic")}</p>
      )}
      {room.phase !== "wildling_bidding" && (
        <BidLedger bids={attack.bids} catalog={catalog} />
      )}
    </>
  );
}

export function EventDialog({
  room,
  catalog,
  busy,
  error,
  onCommand,
  onClose,
  onShowRegion,
  art,
}: {
  room: Room;
  catalog: Catalog;
  busy: boolean;
  error: string;
  onCommand: Command;
  onClose: () => void;
  onShowRegion: (id: string) => void;
  art: (id: string, house: string) => string | undefined;
}) {
  const language = useLanguage();
  const current = room.westeros?.current;
  const run = room.westeros?.run;
  const attack = room.wildlings?.attack;
  const last = room.wildlings?.last;
  const choice = room.phase === "westeros" ? run?.choice : undefined;
  const title = attack
    ? et(
        room.phase === "wildling_bidding"
          ? "biddingTitle"
          : room.phase === "wildling_ties"
            ? "tieTitle"
            : "effectsTitle",
      )
    : et("title");
  return (
    <Modal
      titleID="events-title"
      className="events-modal"
      focusKey={`${room.phase}:${choice?.house}:${attack?.choice?.kind}:${attack?.choice?.count}:${attack?.bids.yourBid}`}
      focusReady={!busy}
      onClose={onClose}
    >
      <div className="modal-top">
        <div>
          <p className="eyebrow">{et("eyebrow")}</p>
          <h2 id="events-title">{title}</h2>
        </div>
        <div className="influence-header-actions">
          <button
            type="button"
            aria-label={t("settings.changeLanguage")}
            onClick={() => setLanguage(language === "ru" ? "en" : "ru")}
          >
            {language.toUpperCase()}
          </button>
          <button
            type="button"
            aria-label={t("common.close")}
            onClick={onClose}
          >
            <X size={18} />
          </button>
        </div>
      </div>
      <div className="events-threat">
        <Shield />
        <span>{et("threat")}</span>
        <meter
          min={0}
          max={12}
          value={room.westeros?.threat ?? 0}
          aria-label={et("threat")}
        />
        <strong>{room.westeros?.threat ?? 0} / 12</strong>
      </div>
      {attack?.excludedHouse && (
        <p className="events-excluded" role="status">
          {et("excluded", { p0: catalog.houseNames[attack.excludedHouse] })}
        </p>
      )}
      {attack && (
        <p className="events-strength">
          {et("attackStrength", { p0: attack.strength })}
        </p>
      )}
      {room.phase === "wildling_bidding" && attack && (
        <>
          <div className="events-secret">
            <LockKeyhole />
            <h3>{et("secretCard")}</h3>
            <p>{et("secretHint")}</p>
          </div>
          <WatchBid
            room={room}
            catalog={catalog}
            busy={busy}
            onCommand={onCommand}
          />
        </>
      )}
      {attack && room.phase !== "wildling_bidding" && (
        <RevealedAttack
          room={room}
          catalog={catalog}
          busy={busy}
          art={art}
          onCommand={onCommand}
          onShowRegion={onShowRegion}
        />
      )}
      {choice && (
        <section className="events-decision">
          <h3>{cardName(choice.card)}</h3>
          <Options
            options={choice.options.map((id) => ({
              id,
              title: decisionName(id),
            }))}
            ours={choice.house === room.yourHouse}
            busy={busy}
            waiting={et("waiting", { p0: catalog.houseNames[choice.house] })}
            onShowRegion={onShowRegion}
            onChoose={(value) =>
              onCommand({ type: "westeros_choice", choice: value })
            }
          />
        </section>
      )}
      {!attack && room.phase === "westeros" && !choice && (
        <p role="status">{et("automatic")}</p>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {room.events.at(-1)?.type === "rules-error" && (
        <p className="error" role="alert">
          {room.events.at(-1)?.text}
        </p>
      )}
      <section className="events-westeros" aria-label={et("title")}>
        {current ? (
          current.cards.map((id, index) => (
            <article key={`${index}:${id}`}>
              <EventArt id={id} />
              <div>
                <p className="eyebrow">
                  {et("deck", { p0: ["I", "II", "III"][index] })}
                </p>
                <h3>{cardName(id)}</h3>
                <p>{cardDescription(id)}</p>
                {eventProgress(room, index) !== "pending" && (
                  <small>
                    {et(
                      eventProgress(room, index) === "current"
                        ? "current"
                        : "resolved",
                    )}
                  </small>
                )}
                {current.winterDraws[index] > 0 && (
                  <small>
                    {et("winter", { p0: current.winterDraws[index] })}
                  </small>
                )}
              </div>
            </article>
          ))
        ) : (
          <p>{et("noHistory")}</p>
        )}
      </section>
      {run?.restriction && (
        <p className="events-restriction">
          <strong>{et("restriction")}: </strong>
          {et(`restrictions.${run.restriction}` as EventKey)}
        </p>
      )}
      {last && (
        <details className="events-last">
          <summary>
            {et("history")}: {cardName(last.card)}
          </summary>
          <p>
            {et(last.outcome.watchWon ? "won" : "lost")} ·{" "}
            {et("total", { p0: last.outcome.total, p1: last.strength })}
          </p>
          <p>{wildlingDescription(last.card, last.outcome.watchWon)}</p>
          <ul>
            {Object.entries(last.bids).map(([id, value]) => (
              <li key={id}>
                <House id={id} catalog={catalog} />
                <strong>{value}</strong>
              </li>
            ))}
          </ul>
        </details>
      )}
      <details className="events-history">
        <summary>
          <ScrollText size={16} />
          {et("history")}
        </summary>
        <ol>
          {room.events.map((event) => (
            <li key={event.id}>{readableEvent(event)}</li>
          ))}
        </ol>
      </details>
      <p className="subtle events-timer">
        {et(room.settings.hardTimer ? "hardTimer" : "softTimer")}
      </p>
      <button type="button" className="subtle-button" onClick={onClose}>
        {et("back")}
      </button>
    </Modal>
  );
}
