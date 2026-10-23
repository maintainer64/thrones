import { useState } from "react";
import { Eye, Feather, LockKeyhole, MapPin, RefreshCw, X } from "lucide-react";
import { RavenDecision, RavenStage } from "../enums";
import { t, setLanguage } from "../i18n/core";
import { useLanguage } from "../i18n/useLanguage";
import { Modal } from "../ui/Modal";
import { OrderPalette } from "../OrderPalette";
import { EventArt } from "../events/EventDialog";
import { cardName, wildlingDescription } from "../events/model";
import { regionName } from "../types";
import type { Catalog, Room } from "../types";
import "./raven.css";

function confirmation(choice: RavenDecision) {
  switch (choice) {
    case RavenDecision.Peek:
      return t("raven.confirmPeek");
    case RavenDecision.Skip:
      return t("raven.confirmSkip");
    case RavenDecision.Keep:
      return t("raven.confirmKeep");
    case RavenDecision.Bottom:
      return t("raven.confirmBottom");
    default:
      return "";
  }
}

export function RavenDialog({
  room,
  catalog,
  busy,
  error,
  onCommand,
  onClose,
  onShowRegion,
}: {
  room: Room;
  catalog: Catalog;
  busy: boolean;
  error: string;
  onCommand: (value: Record<string, unknown>) => void;
  onClose: () => void;
  onShowRegion: (id: string) => void;
}) {
  const language = useLanguage();
  const step = room.raven!;
  const ours = step.house === room.yourHouse;
  const peeking = step.stage === RavenStage.Peek;
  const areas = Object.keys(step.options || {}).sort();
  const [replacing, setReplacing] = useState(false);
  const [region, setRegion] = useState(areas[0] || "");
  const [order, setOrder] = useState("");
  const [pending, setPending] = useState<RavenDecision>();
  const options = step.options?.[region] || [];
  const legal = catalog.orders.filter((value) => options.includes(value.id));
  const current = catalog.orders.find(
    (value) => value.id === room.orders[region],
  );
  const selected = legal.find((value) => value.id === order);
  const area = catalog.board.regions.find((value) => value.id === region);
  const holder = catalog.houseNames[step.house] || step.house;
  return (
    <Modal
      titleID="raven-title"
      className="raven-modal"
      focusKey={`${step.stage}:${replacing}:${pending || ""}`}
      focusReady={!busy}
      onClose={onClose}
    >
      <div className="modal-top">
        <div>
          <p className="eyebrow">{t("raven.eyebrow")}</p>
          <h2 id="raven-title">
            <Feather size={25} /> {t("raven.title")}
          </h2>
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
      <p className="raven-holder">
        <img
          src={`/media/crests/${step.house}.svg`}
          alt=""
          width={30}
          height={30}
        />
        {holder} ·{" "}
        {room.match?.ravenUsed ? t("raven.used") : t("raven.available")}
      </p>
      {!ours ? (
        <section className="raven-wait" role="status">
          <LockKeyhole size={32} />
          <h3>{t("raven.waiting", { p0: holder })}</h3>
          <p>{t("raven.privacy")}</p>
        </section>
      ) : peeking ? (
        <>
          <p className="raven-warning">
            <LockKeyhole size={19} /> {t("raven.private")}
          </p>
          {step.card && (
            <article
              className="events-wildling-card"
              data-testid="raven-private-card"
            >
              <EventArt id={step.card} />
              <div>
                <h3>{cardName(step.card)}</h3>
                <h4>{t("raven.watchWins")}</h4>
                <p>{wildlingDescription(step.card, true)}</p>
                <h4>{t("raven.watchLoses")}</h4>
                <p>{wildlingDescription(step.card, false)}</p>
              </div>
            </article>
          )}
          <p>{t("raven.placementHint")}</p>
          {!pending && (
            <div className="raven-actions">
              <button
                type="button"
                disabled={busy}
                data-initial-focus=""
                onClick={() => setPending(RavenDecision.Keep)}
              >
                {t("raven.keep")}
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() => setPending(RavenDecision.Bottom)}
              >
                {t("raven.bottom")}
              </button>
            </div>
          )}
        </>
      ) : replacing ? (
        <section>
          <h3>{t("raven.replace")}</h3>
          <p>{t("raven.replaceHint")}</p>
          <label htmlFor="raven-region">{t("raven.region")}</label>
          <div className="raven-region">
            <select
              id="raven-region"
              value={region}
              disabled={busy}
              onChange={(event) => {
                setRegion(event.target.value);
                setOrder("");
              }}
            >
              {areas.map((id) => (
                <option key={id} value={id}>
                  {regionName(
                    catalog.board.regions.find((value) => value.id === id),
                  )}
                </option>
              ))}
            </select>
            <button type="button" onClick={() => onShowRegion(region)}>
              <MapPin size={17} />
              {t("raven.showRegion")}
            </button>
          </div>
          <p>{t("raven.current", { p0: current?.name || "—" })}</p>
          <OrderPalette
            key={region}
            orders={legal}
            used={[]}
            selected={order}
            onChoose={setOrder}
            locked={busy ? t("common.sending") : undefined}
          />
          {selected && (
            <div className="events-confirm" role="status">
              <p>
                {t("raven.swapSummary", {
                  p0: regionName(area),
                  p1: current?.name,
                  p2: selected.name,
                })}
              </p>
              <button
                type="button"
                className="primary"
                disabled={busy}
                onClick={() =>
                  onCommand({
                    type: "raven",
                    choice: RavenDecision.Replace,
                    region,
                    order: selected.id,
                  })
                }
              >
                {t("raven.confirmReplace")}
              </button>
            </div>
          )}
          <button
            type="button"
            disabled={busy}
            className="subtle-button"
            onClick={() => {
              setReplacing(false);
              setOrder("");
            }}
          >
            {t("raven.back")}
          </button>
        </section>
      ) : !pending ? (
        <>
          <p>{t("raven.intro")}</p>
          <div className="raven-actions raven-modes">
            <button
              type="button"
              disabled={busy || !areas.length}
              aria-label={t("raven.replace")}
              onClick={() => setReplacing(true)}
            >
              <RefreshCw />
              {t("raven.replace")}
              <small>{t("raven.replaceHint")}</small>
            </button>
            <button
              type="button"
              disabled={busy}
              data-initial-focus=""
              aria-label={t("raven.peek")}
              onClick={() => setPending(RavenDecision.Peek)}
            >
              <Eye />
              {t("raven.peek")}
              <small>{t("raven.peekHint")}</small>
            </button>
          </div>
          {!areas.length && <p>{t("raven.noReplacements")}</p>}
          <button
            type="button"
            disabled={busy}
            className="subtle-button"
            onClick={() => setPending(RavenDecision.Skip)}
          >
            {t("raven.skip")}
          </button>
        </>
      ) : null}
      {ours && pending && (
        <section className="events-confirm" role="status">
          <h3>{confirmation(pending)}</h3>
          <p>{t("raven.irreversible")}</p>
          <div className="raven-actions">
            <button
              type="button"
              className="primary"
              disabled={busy}
              data-initial-focus=""
              onClick={() => onCommand({ type: "raven", choice: pending })}
            >
              {t("raven.confirm")}
            </button>
            <button
              type="button"
              disabled={busy}
              onClick={() => setPending(undefined)}
            >
              {t("raven.back")}
            </button>
          </div>
        </section>
      )}
      {error && (
        <p role="alert" className="influence-error">
          {error}
        </p>
      )}
      <p className="subtle">
        {room.settings.hardTimer ? t("raven.hardTimer") : t("raven.softTimer")}
      </p>
    </Modal>
  );
}
