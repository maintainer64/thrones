import type { Order } from "../types";
import { artworkURL } from "./artwork";
import bases from "./order-bases.json" with { type: "json" };

export function orderTokenArt(order: Order) {
  const filename = (bases as Record<string, string>)[order.kind];
  return filename ? artworkURL(`media/orders/${filename}`) : undefined;
}

export function orderModifier(order: Order) {
  return order.kind === "march" || order.modifier !== 0
    ? `${order.modifier > 0 ? "+" : ""}${order.modifier}`
    : undefined;
}
