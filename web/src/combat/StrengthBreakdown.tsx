import { Shield, Swords } from "lucide-react";
import { CombatLineKind } from "../enums";
import { t } from "../i18n/core";
import {
  getUnitNames,
  regionName,
  type Assets,
  type Catalog,
  type CombatStrength,
  type CombatStrengthLine,
} from "../types";
import { StrengthShield } from "../ui/StrengthShield";

function lineName(line: CombatStrengthLine, catalog: Catalog) {
  const card = line.card
    ? Object.values(catalog.houses)
        .map((house) => house.houseCards[line.card!])
        .find(Boolean)?.name
    : undefined;
  switch (line.kind) {
    case CombatLineKind.Unit:
      return getUnitNames()[line.unit || ""];
    case CombatLineKind.SupportUnit:
      return t("combat.supportUnit", { p0: getUnitNames()[line.unit || ""] });
    case CombatLineKind.UnitEffect:
      return t("combat.unitEffect", { p0: card });
    case CombatLineKind.SupportOrder:
      return t("combat.supportOrder");
    case CombatLineKind.March:
      return t("combat.marchBonus");
    case CombatLineKind.Defense:
      return t("combat.defenseBonus");
    case CombatLineKind.Garrison:
      return t("fortifications.garrison");
    case CombatLineKind.HouseCard:
      return card || t("combat.cancelledCard");
    case CombatLineKind.CardEffect:
      return t("combat.cardAdjustment", { p0: card });
    case CombatLineKind.Blade:
      return t("tides.bladeTitle");
    case CombatLineKind.Tides:
      return t("tides.title");
    case CombatLineKind.Minimum:
      return t("combat.minimumStrength");
  }
}
export function StrengthBreakdown({
  value,
  catalog,
  assets,
  visibleCount,
}: {
  value: CombatStrength;
  catalog: Catalog;
  assets: Assets;
  visibleCount?: number;
}) {
  return (
    <section
      className="combat-strength"
      aria-label={t("combat.breakdown")}
      data-revealed={value.revealed}
    >
      {!value.revealed && (
        <p className="combat-secret-note">{t("combat.beforeCards")}</p>
      )}
      <div className="combat-strength-columns">
        {[value.attacker, value.defender].map((side, index) => (
          <section
            className="combat-strength-side"
            key={side.house}
            data-side={index === 0 ? "attacker" : "defender"}
          >
            <div className="combat-strength-heading">
              <img
                src={`/media/crests/${side.house}.svg`}
                width={32}
                height={32}
                alt=""
              />
              <div>
                <small>
                  {index === 0 ? t("combat.attacker") : t("combat.defender")}
                </small>
                <h3>{catalog.houseNames[side.house]}</h3>
              </div>
              <StrengthShield
                value={side.total}
                label={
                  visibleCount !== undefined
                    ? t("animation.intermediate")
                    : value.revealed
                      ? t("combat.total")
                      : t("combat.publicStrength")
                }
              />
            </div>
            <ul>
              {side.lines.map((line, position) => (
                <li
                  key={position}
                  data-kind={line.kind}
                  data-value={line.value}
                  data-pending={
                    visibleCount !== undefined && position >= visibleCount
                  }
                >
                  {line.unit ? (
                    <img
                      src={assets.units[side.house]?.[line.unit]}
                      width={28}
                      height={28}
                      alt=""
                    />
                  ) : index === 0 ? (
                    <Swords size={19} />
                  ) : (
                    <Shield size={19} />
                  )}
                  <div>
                    <b>{lineName(line, catalog)}</b>
                    <small>
                      {line.region &&
                        regionName(
                          catalog.board.regions.find(
                            (region) => region.id === line.region,
                          ),
                        )}
                      {line.unit && (
                        <span>
                          {" "}
                          · {line.count} × {line.each || 0}
                        </span>
                      )}
                    </small>
                    {line.unit === "siege-engine" &&
                      line.kind !== CombatLineKind.UnitEffect && (
                        <small className="combat-siege-note">
                          {index === 0 && value.castle
                            ? t("combat.siegeCastle")
                            : t("combat.siegeZero")}
                        </small>
                      )}
                  </div>
                  <strong>
                    {line.value > 0 ? "+" : ""}
                    {line.value}
                  </strong>
                </li>
              ))}
            </ul>
            <footer>
              <span>
                {visibleCount !== undefined
                  ? t("animation.intermediate")
                  : value.revealed
                    ? t("combat.total")
                    : t("combat.publicStrength")}
              </span>
              <b data-testid="strength-total">{side.total}</b>
            </footer>
          </section>
        ))}
      </div>
      <p className="combat-secret-note">{t("combat.ownSupportOnly")}</p>
    </section>
  );
}
