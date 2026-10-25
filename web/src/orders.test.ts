import { describe, expect, it } from "vitest";
import { describeOrder } from "./orders";
import type { Order } from "./types";
const order = (kind: string, modifier = 0, star = false): Order => ({
  id: kind,
  name: kind,
  kind,
  modifier,
  star,
});
describe("order help matches implemented rules", () => {
  it("explains combat modifiers and separate march confirmation", () => {
    expect(describeOrder(order("march", -1))).toContain("-1");
    expect(describeOrder(order("march"))).toContain("подтверждается отдельно");
    expect(describeOrder(order("defend", 2, true))).toContain("+2");
  });
  it("distinguishes unimplemented foreign support from the implemented muster choice", () => {
    expect(describeOrder(order("support"))).toContain(
      "других домов ещё не реализована",
    );
    expect(describeOrder(order("power", 0, true))).toContain(
      "Особый приказ позволяет вместо власти собрать войска",
    );
  });
  it("explains the starred raid exception", () => {
    expect(describeOrder(order("raid"))).toContain("Не снимает оборону");
    expect(describeOrder(order("raid", 0, true))).toContain(
      "также снимает оборону",
    );
  });
});
