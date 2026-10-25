import { t } from "./i18n/core";
import { useId, useState } from "react";
import type { Order } from "./types";
import { describeOrder, shortOrderName } from "./orders";
import { OrderToken } from "./ui/OrderToken";
export function OrderDescription({ order }: { order: Order }) {
  return (
    <p className="order-description">
      {describeOrder(order)}
      {order.star && <small>{t("orders.specialLimitHint")}</small>}
    </p>
  );
}
export function OrderPalette({
  orders,
  selected,
  used,
  locked,
  available,
  onChoose,
}: {
  orders: Order[];
  selected?: string;
  used: string[];
  locked?: string;
  available?: string[];
  onChoose: (id: string) => void;
}) {
  const [inspected, setInspected] = useState(selected || orders[0]?.id);
  const descriptionID = useId();
  const order = orders.find((order) => order.id === inspected) || orders[0];
  const reason = (id: string) =>
    locked ||
    (used.includes(id)
      ? t("orders.alreadyAssignedHint")
      : available && !available.includes(id) && id !== selected
        ? t("orders.unavailableHint")
        : "");
  return (
    <div className="order-picker">
      {order && (
        <div
          id={descriptionID}
          className="order-help"
          role="status"
          aria-live="polite"
        >
          <strong>{order.name}</strong>
          <OrderDescription order={order} />
          {reason(order.id) && (
            <p className="order-unavailable">{reason(order.id)}</p>
          )}
        </div>
      )}
      <div className="order-palette" aria-label={t("orders.tokens")}>
        {orders.map((order) => (
          <button
            key={order.id}
            type="button"
            title={
              order.name +
              (used.includes(order.id) ? t("orders.inUseSuffix") : "")
            }
            aria-label={order.name}
            aria-pressed={selected === order.id}
            aria-disabled={!!reason(order.id)}
            data-used={used.includes(order.id)}
            aria-describedby={descriptionID}
            className={`${selected === order.id ? "picked" : ""} ${inspected === order.id ? "inspected" : ""}`}
            onMouseEnter={() => setInspected(order.id)}
            onFocus={() => setInspected(order.id)}
            onClick={() => {
              setInspected(order.id);
              if (!reason(order.id))
                onChoose(selected === order.id ? "" : order.id);
            }}
          >
            <OrderToken order={order} decorative />
            <span>{shortOrderName(order)}</span>
          </button>
        ))}
      </div>
      {!orders.length && (
        <p className="order-picker-hint">{t("orders.noTokensHint")}</p>
      )}
      <p className="order-picker-hint">{t("orders.assignmentHint")}</p>
    </div>
  );
}
