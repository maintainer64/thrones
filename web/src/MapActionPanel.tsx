import { ArrowRight, Flag, LoaderCircle, Swords } from "lucide-react";
import { t } from "./i18n/core";
import { regionName } from "./types";
import type { Assets, Catalog, Room } from "./types";
import type { MapAction } from "./scene3d/game-state";

export function MapActionPanel({
  action,
  room,
  catalog,
  assets,
  selected,
  target,
  onTarget,
  onFrame,
  leaveToken,
  onLeaveToken,
}: {
  action: MapAction;
  room: Room;
  catalog: Catalog;
  assets: Assets;
  selected: string;
  target: string;
  onTarget: (id: string) => void;
  onFrame: () => void;
  leaveToken: boolean;
  onLeaveToken: (value: boolean) => void;
}) {
  const name = (id: string) =>
    regionName(catalog.board.regions.find((region) => region.id === id));
  const march = action.kind === "march";
  const land =
    catalog.board.regions.find((region) => region.id === selected)?.type ===
    "land";
  const power = room.match?.power[room.yourHouse] || 0;
  return (
    <section
      className="sidebar-action"
      data-testid="map-move-panel"
      data-selecting={action.selecting}
    >
      <h3>
        <Flag size={19} />
        {march ? t("march.heading") : t("raid.heading")}
      </h3>
      <p className="action-route">
        <b>{name(selected)}</b>
        <ArrowRight size={18} />
        <b>{target ? name(target) : t("actions.selectTarget")}</b>
      </p>
      {march && (
        <fieldset disabled={action.busy}>
          <legend>{t("actions.stepUnits")}</legend>
          {action.units.map((unit) => (
            <label className="sidebar-action-unit" key={unit.type}>
              <img src={assets.units[room.yourHouse]?.[unit.type]} alt="" />
              <span>
                {unit.label}
                <small>
                  {t("provision.available", { p0: unit.available })}
                </small>
              </span>
              <input
                aria-label={t("march.unitsCount", { p0: unit.label })}
                type="number"
                min={0}
                max={unit.available}
                step={1}
                value={unit.quantity}
                onChange={(event) =>
                  action.onQuantity(
                    unit.type,
                    Math.max(
                      0,
                      Math.min(
                        unit.available,
                        Math.trunc(Number(event.target.value) || 0),
                      ),
                    ),
                  )
                }
              />
            </label>
          ))}
        </fieldset>
      )}
      <h4>{march ? t("actions.stepTarget") : t("actions.selectTarget")}</h4>
      <p className="subtle">{t("actions.targetHelp")}</p>
      <div className="sidebar-action-status" role="status">
        {!action.selecting ? (
          t("actions.selectionCancelled")
        ) : action.loading ? (
          <>
            <LoaderCircle size={17} />
            {t("actions.findingTargets")}
          </>
        ) : action.error ? (
          <>
            {action.error}
            <button onClick={action.onRetry}>{t("common.retry")}</button>
          </>
        ) : action.targets.length ? (
          t("board.availableAreasHint", { p0: action.targets.length })
        ) : march ? (
          t("march.noTargetsHint")
        ) : (
          t("raid.noTargetsHint")
        )}
      </div>
      <select
        aria-label={t("actions.targetLabel")}
        value={target}
        disabled={action.busy || action.loading || !action.selecting}
        onChange={(event) => onTarget(event.target.value)}
      >
        <option value="">{t("board.selectArea")}</option>
        {action.targets.map((id) => (
          <option key={id} value={id}>
            {name(id)}
          </option>
        ))}
      </select>
      <button
        disabled={action.loading || !action.targets.length}
        onClick={onFrame}
      >
        {t("actions.showTargets")}
      </button>
      {!action.selecting && (
        <button onClick={action.onBegin}>
          {t("actions.chooseTargetOnMap")}
        </button>
      )}
      {action.selecting && (
        <button className="subtle-button" onClick={action.onCancel}>
          {t("actions.cancelTarget")}
        </button>
      )}
      {march && land && (
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={leaveToken}
            disabled={action.busy || power < 1}
            onChange={(event) => onLeaveToken(event.target.checked)}
          />
          {t("march.leaveControlToken")}
          <small>{t("actions.tokenCost", { p0: power })}</small>
        </label>
      )}
      {target && (
        <p className="action-recap">
          {t("actions.confirmRoute", {
            p0: name(selected),
            p1: name(target),
            p2: action.units.reduce((sum, unit) => sum + unit.quantity, 0),
          })}
        </p>
      )}
      <button
        className="primary"
        disabled={
          action.busy ||
          action.loading ||
          !!action.error ||
          !action.targets.includes(target)
        }
        onClick={action.onConfirm}
      >
        <Swords size={17} />
        {march ? t("march.confirm") : t("raid.confirm")}
      </button>
      <button
        disabled={action.busy}
        className="subtle-button"
        onClick={action.onPass}
      >
        {t("orders.removeWithoutAction")}
      </button>
    </section>
  );
}
