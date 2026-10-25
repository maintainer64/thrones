import { useState } from "react";
import type { Order } from "../types";
import { orderModifier, orderTokenArt } from "../artwork/order-art";
import { GameSymbol } from "./GameSymbol";
import "./order-token.css";

export function OrderToken({
  order,
  className = "",
  decorative = false,
}: {
  order: Order;
  className?: string;
  decorative?: boolean;
}) {
  const src = orderTokenArt(order);
  const [failedSource, setFailedSource] = useState<string>();
  const modifier = orderModifier(order);
  return (
    <span
      className={`order-token ${className}`}
      data-order-id={order.id}
      data-kind={order.kind}
      data-special={order.star}
      role={decorative ? undefined : "img"}
      aria-hidden={decorative || undefined}
      aria-label={decorative ? undefined : order.name}
    >
      {src && src !== failedSource ? (
        <img
          src={src}
          alt=""
          draggable={false}
          onError={() => setFailedSource(src)}
        />
      ) : (
        <span className="order-token-fallback">{order.name.slice(0, 1)}</span>
      )}
      {modifier !== undefined && (
        <b className="order-token-modifier">{modifier}</b>
      )}
      {order.star && (
        <span className="order-token-star">
          <GameSymbol kind="star" />
        </span>
      )}
    </span>
  );
}
