import { t } from "./i18n/core";
import type { Order } from "./types";
export function describeOrder(order: Order) {
  const modifier =
    order.modifier > 0 ? `+${order.modifier}` : `${order.modifier}`;
  switch (order.kind) {
    case "march":
      return t("orders.marchDescription", { p0: modifier });
    case "defend":
      return t("orders.defendDescription", { p0: modifier });
    case "support":
      return t("orders.supportDescription", {
        p0: order.modifier ? t("orders.supportBonus", { p0: modifier }) : "",
      });
    case "raid":
      return t("orders.raidDescription", {
        p0: order.star
          ? t("orders.raidStarException")
          : t("orders.raidExcludedOrders"),
      });
    case "power":
      return t("orders.powerDescription", {
        p0: order.star ? t("orders.powerMusteringChoice") : "",
      });
    default:
      return t("orders.descriptionUnavailable");
  }
}
export function shortOrderName(order: Order) {
  return (
    (
      {
        march: t("orders.march"),
        defend: t("orders.defend"),
        support: t("orders.support"),
        raid: t("orders.raid"),
        power: t("resources.power"),
      } as Record<string, string>
    )[order.kind] || order.name
  );
}
