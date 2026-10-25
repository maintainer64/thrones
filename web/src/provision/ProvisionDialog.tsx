import { useState } from "react";
import { Coins, Flag, MapPin, Shield, Users, X } from "lucide-react";
import { setLanguage, t } from "../i18n/core";
import { useLanguage } from "../i18n/useLanguage";
import { getUnitNames, regionName } from "../types";
import type { Assets, Catalog, Room } from "../types";
import { Modal } from "../ui/Modal";
import "./provision.css";

export function ProvisionDialog({
  room,
  catalog,
  assets,
  busy,
  error,
  onCommand,
  onClose,
  onShowRegion,
}: {
  room: Room;
  catalog: Catalog;
  assets?: Assets;
  busy: boolean;
  error: string;
  onCommand: (command: Record<string, unknown>, onSuccess?: () => void) => void;
  onClose: () => void;
  onShowRegion: (id: string) => void;
}) {
  const language = useLanguage();
  const names = getUnitNames();
  const supply = room.supply,
    muster = room.mustering;
  const house = supply?.house || muster?.house || room.match!.activeHouse;
  const ours = house === room.yourHouse;
  const title = supply
    ? t("provision.supplyTitle")
    : muster
      ? t("provision.musterTitle")
      : t("provision.powerTitle");
  const regions = supply
    ? supply.armies.map((a) => a.region)
    : muster
      ? Object.keys(muster.points)
      : Object.keys(room.orders).filter(
          (id) =>
            room.orders[id] === "power-star" &&
            room.match?.armies[id]?.some((u) => u.house === house),
        );
  const [selected, setSelected] = useState(regions[0] || "");
  const [quantities, setQuantities] = useState<Record<string, number>>({});
  const [confirm, setConfirm] = useState(false);
  const id = regions.includes(selected) ? selected : regions[0];
  const name = (id: string) =>
    regionName(catalog.board.regions.find((r) => r.id === id));
  const army = supply?.armies.find((a) => a.region === id);
  const counts = Object.fromEntries(
    (army?.units || []).map((u) => [
      u.unitType,
      Math.min(u.quantity, quantities[u.unitType] || 0),
    ]),
  );
  const total = Object.values(counts).reduce((a, b) => a + b, 0);
  const collect = Math.max(
    0,
    Math.min(
      1 + (catalog.board.regions.find((r) => r.id === id)?.crownIcons || 0),
      20 -
        Object.values(room.match?.controlTokens || {}).filter(
          (owner) => owner === house,
        ).length -
        (room.match?.power[house] || 0),
    ),
  );
  return (
    <Modal
      titleID="provision-title"
      className="provision-modal"
      focusKey={`${id}:${confirm}`}
      focusReady={!busy}
      onClose={onClose}
    >
      <div className="modal-top">
        <div>
          <p className="eyebrow">{catalog.houseNames[house]}</p>
          <h2 id="provision-title">{title}</h2>
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
      {!ours && (
        <p className="provision-wait" role="status">
          {t("provision.waiting", { p0: catalog.houseNames[house] })}
        </p>
      )}
      {supply && (
        <div className="provision-supply-summary">
          <Shield />
          <span>
            {t("resources.supply")}: <b>{supply.level}</b>
          </span>
          <span>
            {t("provision.allowedArmies", { p0: supply.limits.join(" / ") })}
          </span>
        </div>
      )}
      <p className="subtle">
        {supply
          ? t("provision.disbandHint")
          : muster
            ? t("provision.musterHint")
            : t("provision.powerHint")}
      </p>
      {muster && (
        <ol className="muster-steps" aria-label={t("provision.musterSteps")}>
          <li>{t("provision.stepArea")}</li>
          <li>{t("provision.stepUnit")}</li>
          <li>{t("provision.stepFinish")}</li>
        </ol>
      )}
      <div className="provision-layout">
        <nav aria-label={t("provision.areas")} className="provision-areas">
          {regions.map((region) => (
            <button
              type="button"
              key={region}
              aria-pressed={region === id}
              onClick={() => {
                setSelected(region);
                setQuantities({});
                setConfirm(false);
              }}
            >
              <MapPin size={16} />
              {name(region)}
              <small>
                {supply
                  ? t("provision.armySize", {
                      p0: supply.armies.find((a) => a.region === region)?.size,
                    })
                  : muster
                    ? t("provision.points", { p0: muster.points[region] })
                    : ""}
              </small>
            </button>
          ))}
        </nav>
        <section className="provision-decision">
          <h3>{name(id)}</h3>
          <button
            className="subtle-button"
            type="button"
            onClick={() => onShowRegion(id)}
          >
            <MapPin size={15} />
            {t("provision.showOnMap")}
          </button>
          {supply && army && (
            <>
              <div className="provision-unit-list">
                {army.units.map((unit) => (
                  <label key={unit.unitType}>
                    <span>
                      {names[unit.unitType]}
                      <small>
                        {t("provision.available", { p0: unit.quantity })}
                      </small>
                    </span>
                    {ours && (
                      <input
                        type="number"
                        min={0}
                        max={unit.quantity}
                        step={1}
                        value={counts[unit.unitType]}
                        disabled={busy || confirm}
                        aria-label={t("provision.disbandUnit", {
                          p0: names[unit.unitType],
                        })}
                        onChange={(event) =>
                          setQuantities((old) => ({
                            ...old,
                            [unit.unitType]: Math.max(
                              0,
                              Math.min(
                                unit.quantity,
                                Math.trunc(Number(event.target.value)),
                              ),
                            ),
                          }))
                        }
                      />
                    )}
                  </label>
                ))}
              </div>
              {ours &&
                (confirm ? (
                  <div className="provision-confirm">
                    <p>{t("provision.disbandConfirm", { p0: total })}</p>
                    <button
                      className="primary"
                      type="button"
                      data-initial-focus
                      disabled={busy || total === 0}
                      onClick={() =>
                        onCommand(
                          { type: "disband", region: id, units: counts },
                          () => {
                            setQuantities({});
                            setConfirm(false);
                          },
                        )
                      }
                    >
                      {t("provision.confirmDisband")}
                    </button>
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => setConfirm(false)}
                    >
                      {t("provision.cancel")}
                    </button>
                  </div>
                ) : (
                  <button
                    className="primary"
                    type="button"
                    disabled={busy || total === 0}
                    onClick={() => setConfirm(true)}
                  >
                    {t("provision.removeSelected")}
                  </button>
                ))}
            </>
          )}
          {muster && (
            <>
              <div className="muster-area-state">
                <p className="provision-points">
                  <Flag size={18} />
                  {t("provision.pointsRemaining", {
                    p0: muster.points[id] || 0,
                    p1:
                      catalog.board.regions.find((region) => region.id === id)
                        ?.castleLevel || 0,
                  })}
                </p>
                <div className="muster-point-tokens" aria-hidden="true">
                  {Array.from(
                    {
                      length:
                        catalog.board.regions.find((region) => region.id === id)
                          ?.castleLevel || 0,
                    },
                    (_, index) => (
                      <span
                        key={index}
                        data-available={index < (muster.points[id] || 0)}
                      >
                        ◆
                      </span>
                    ),
                  )}
                </div>
                <h4>{t("provision.unitsHere")}</h4>
                <div className="muster-current-units">
                  {(room.match?.armies[id] || []).map((unit) => (
                    <span key={unit.unitType}>
                      {assets && (
                        <img
                          src={assets.units[house]?.[unit.unitType]}
                          alt=""
                        />
                      )}
                      {names[unit.unitType]} <b>× {unit.quantity}</b>
                    </span>
                  ))}
                  {!(room.match?.armies[id] || []).length && (
                    <span>{t("board.noUnits")}</span>
                  )}
                </div>
              </div>
              <p className="subtle">{t("provision.immediateMusterHint")}</p>
              <div className="provision-options">
                {muster.options
                  .filter((o) => o.region === id)
                  .map((o) => (
                    <button
                      type="button"
                      key={`${o.region}:${o.target}:${o.unit}:${o.upgrade}`}
                      disabled={busy || !ours}
                      onClick={() =>
                        onCommand({
                          type: "muster",
                          region: o.region,
                          target: o.target,
                          unit: o.unit,
                          upgrade: o.upgrade,
                        })
                      }
                    >
                      <span>
                        {o.upgrade && assets && (
                          <>
                            <img
                              className="muster-unit-icon"
                              src={assets.units[house]?.footman}
                              alt=""
                            />
                            <span aria-hidden="true">→</span>
                          </>
                        )}
                        {assets ? (
                          <img
                            className="muster-unit-icon"
                            src={assets.units[house]?.[o.unit]}
                            alt=""
                          />
                        ) : (
                          <Users size={17} />
                        )}
                        {o.upgrade
                          ? t("provision.upgradeTo", { p0: names[o.unit] })
                          : names[o.unit]}
                      </span>
                      <small>
                        {name(o.target)} ·{" "}
                        {t("provision.points", { p0: o.cost })}
                      </small>
                    </button>
                  ))}
              </div>
              {!muster.options.some((o) => o.region === id) && (
                <p className="subtle">
                  {t("provision.noOptions")} {t("provision.checkConstraints")}
                </p>
              )}
            </>
          )}
          {!supply && !muster && ours && (
            <div className="provision-power-options">
              <button
                className="primary"
                type="button"
                disabled={busy}
                onClick={() =>
                  onCommand({
                    type: "consolidate",
                    region: id,
                    choice: "power",
                  })
                }
              >
                <Coins size={18} />
                {t("provision.collectPower", {
                  p0: collect,
                })}
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() =>
                  onCommand({
                    type: "consolidate",
                    region: id,
                    choice: "muster",
                  })
                }
              >
                <Users size={18} />
                {t("provision.chooseMustering")}
              </button>
            </div>
          )}
        </section>
      </div>
      {muster && (
        <>
          <div
            className="provision-reserve"
            aria-label={t("provision.reserveTitle")}
          >
            <b>{t("provision.reserveTitle")}</b>
            {Object.entries(muster.reserve).map(([type, value]) => (
              <span key={type}>
                {names[type]}: <strong>{value}</strong>
              </span>
            ))}
          </div>
          {ours &&
            (confirm ? (
              <div className="provision-confirm">
                <p>{t("provision.finishConfirm")}</p>
                <button
                  type="button"
                  className="primary"
                  data-initial-focus
                  disabled={busy}
                  onClick={() => onCommand({ type: "finish_muster" })}
                >
                  {t("provision.confirmFinish")}
                </button>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => setConfirm(false)}
                >
                  {t("provision.cancel")}
                </button>
              </div>
            ) : (
              <button
                type="button"
                disabled={busy}
                onClick={() => setConfirm(true)}
              >
                {t("provision.finishMuster")}
              </button>
            ))}
        </>
      )}
      {error && (
        <p className="influence-error" role="alert">
          {error}
        </p>
      )}
      <p className="subtle">
        {room.settings.hardTimer
          ? t("provision.hardTimer")
          : t("provision.softTimer")}
      </p>
    </Modal>
  );
}
