import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { OrderToken } from "./OrderToken";
import { orderModifier, orderTokenArt } from "../artwork/order-art";
import { GameSymbol } from "./GameSymbol";
import type { Order } from "../types";
import media from "../../public/media/manifest.json" with { type: "json" };

const order = (kind: string, modifier = 0, star = false): Order => ({
  id: `${kind}${star ? "-star" : ""}`,
  kind,
  name: kind,
  modifier,
  star,
});
describe("composed game tokens", () => {
  it("reuses exactly five HD bases, without missing P/M variants", () => {
    expect(new Set(Object.values(media.orders)).size).toBe(5);
    for (const [id, path] of Object.entries(media.orders)) {
      const value = order(id.split("-")[0], 0, id.endsWith("-star"));
      expect(orderTokenArt(value)).toMatch(
        /^\/media\/orders\/[A-Za-z]+\.webp\?v=[a-f0-9]{12}$/,
      );
      expect(orderTokenArt(value)!.split("?")[0]).toBe(path);
    }
  });
  it("renders both the star and strength, not one or the other", () => {
    const html = renderToStaticMarkup(
      <OrderToken order={order("defend", 2, true)} />,
    );
    expect(html).toContain('class="order-token-modifier">+2</b>');
    expect(html).toContain('data-symbol="star"');
    expect(html).toContain('data-special="true"');
    expect(html).not.toContain("DefendP");
  });
  it("distinguishes normal marches, starred support and non-combat orders", () => {
    expect(orderModifier(order("march", -1))).toBe("-1");
    expect(orderModifier(order("march"))).toBe("0");
    expect(orderModifier(order("march", 1, true))).toBe("+1");
    expect(orderModifier(order("support", 1, true))).toBe("+1");
    for (const kind of ["power", "raid", "support"]) {
      expect(orderModifier(order(kind))).toBeUndefined();
    }
    const normal = renderToStaticMarkup(
      <OrderToken order={order("power")} decorative />,
    );
    expect(normal).not.toContain('data-symbol="star"');
    expect(normal).not.toContain("order-token-modifier");
    expect(normal).toContain('aria-hidden="true"');
    const special = renderToStaticMarkup(
      <OrderToken order={order("power", 0, true)} />,
    );
    expect(special).toContain('data-symbol="star"');
    expect(special).not.toContain("order-token-modifier");
  });
  it("has distinct scalable sword and battlement tower symbols", () => {
    const sword = renderToStaticMarkup(<GameSymbol kind="sword" />);
    const tower = renderToStaticMarkup(<GameSymbol kind="tower" />);
    expect(sword).toContain('data-symbol="sword"');
    expect(tower).toContain('data-symbol="tower"');
    expect(sword).not.toBe(tower);
    expect(sword).toContain('viewBox="0 0 64 64"');
    expect(tower).toContain('aria-hidden="true"');
  });
});
