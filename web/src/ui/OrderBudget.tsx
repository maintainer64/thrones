import { Coins, Star } from "lucide-react";
import { t } from "../i18n/core";
import type { Room } from "../types";

export function OrderBudget({
  value,
}: {
  value: NonNullable<Room["orderBudget"]>;
}) {
  const remaining = Math.max(0, value.starsAllowed - value.starsUsed);
  const starsLabel = t("orders.starsLeft", {
    p0: remaining,
    p1: value.starsAllowed,
  });
  return (
    <section className="order-budget" aria-label={t("orders.budget")}>
      <dl>
        <div>
          <dt>
            <Coins size={14} />
            {t("resources.power")}
          </dt>
          <dd>{value.power}</dd>
        </div>
        <div title={starsLabel} aria-label={starsLabel}>
          <dt>
            <Star size={14} />
            {t("orders.specialShort")}
          </dt>
          <dd>
            {remaining}
            <small> / {value.starsAllowed}</small>
          </dd>
        </div>
      </dl>
      <details>
        <summary>{t("orders.limitHelp")}</summary>
        <p>
          {starsLabel}. {t("orders.starsNotPower")}
        </p>
      </details>
    </section>
  );
}
