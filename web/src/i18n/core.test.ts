import { afterEach, describe, expect, it } from "vitest";
import en from "../locales/en.json";
import ru from "../locales/ru.json";
import { getLanguage, setLanguage, subscribe, t } from "./core";
import { getPhases, getUnitNames, regionName } from "../types";
import type { Region } from "../types";
import { describeOrder } from "../orders";

afterEach(() => setLanguage("ru"));
describe("personal RU/EN localization", () => {
  it("has complete dictionaries and preserves each interpolation parameter", () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(ru).sort());
    for (const key of Object.keys(ru) as (keyof typeof ru)[]) {
      expect(en[key].trim(), key).not.toBe("");
      expect(en[key].match(/\{p\d+\}/g)?.sort() || [], key).toEqual(
        ru[key].match(/\{p\d+\}/g)?.sort() || [],
      );
      expect(key).toMatch(/^[a-z]+\.[a-zA-Z0-9]+$/);
      expect(en[key], key).not.toMatch(/[А-Яа-яЁё]/);
    }
  });
  it("switches reactively without retaining module-level Russian labels", () => {
    let changes = 0;
    const unsubscribe = subscribe(() => changes++);
    expect(getPhases().planning).toBe("Тайное планирование");
    setLanguage("en");
    expect(getLanguage()).toBe("en");
    expect(getPhases().planning).toBe("Secret planning");
    expect(getUnitNames().footman).toBe("Footmen");
    expect(t("settings.changeLanguage")).toBe("Change language");
    expect(changes).toBe(1);
    unsubscribe();
    setLanguage("ru");
    expect(changes).toBe(1);
  });
  it("interpolates zero and user input as text without executing it", () => {
    setLanguage("en");
    expect(t("march.unitsCount", { p0: 0 })).toBe("March: 0");
    expect(t("march.unitsCount", { p0: "<script>test</script>" })).toBe(
      "March: <script>test</script>",
    );
  });
  it("changes area labels, not canonical IDs", () => {
    const region = { id: "winterfell", name: "Winterfell" } as Region;
    expect(regionName(region)).toBe("Винтерфелл");
    setLanguage("en");
    expect(regionName(region)).toBe("Winterfell");
    expect(region.id).toBe("winterfell");
  });
  it("translates order rules including nested special-order explanations", () => {
    setLanguage("en");
    const text = describeOrder({
      id: "raid-star",
      kind: "raid",
      name: "Raid",
      modifier: 0,
      star: true,
    });
    expect(text).toContain("special Raid order");
    expect(text).not.toMatch(/[А-Яа-яЁё]/);
  });
});
