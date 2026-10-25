import type { Card } from "../types";

// Width of a complete 10:17 card inside an already padded viewport. Zoom is
// applied separately, so 100% always means fit rather than a fixed pixel size.
export function fittedCardWidth(width: number, height: number) {
  return Math.max(
    1,
    Math.floor(Math.min(440, width - 14, ((height - 14) * 10) / 17)),
  );
}

export function cardRulesTop(description: string) {
  return description.length > 230 ? 54 : description.length > 140 ? 60 : 66;
}

export function orderedCards(cards: Record<string, Card>) {
  return Object.entries(cards).sort(
    ([, a], [, b]) => (b.combatStrength || 0) - (a.combatStrength || 0),
  );
}

export function textureKey(card: string, house: string) {
  const aliases: Record<string, string> = {
    "ser-rodrick-cassel": "rodrik",
    "greatjon-umber": "greatjon",
    "the-blackfish": "blackfish",
    "the-hound": "sandor",
    "the-red-viper": "theredviper",
    "queen-of-thorns": "queenofthorns",
    "dagmar-cleftjaw": "dagmer",
    "ser-davos-seaworth": "davos",
    "ser-gregor-clegane": "gregor",
    "ser-kevan-lannister": "kevan",
    "ser-jaime-lannister": "jaime",
    "ser-garlan-tyrell": "garlan",
    "ser-loras-tyrell": "loras",
    "areo-hotah": "areo",
  };
  return `T_crd_${house}_${aliases[card] || card.split("-").filter((part) => !["ser", "the"].includes(part))[0]}`;
}

export function pointerPose(x: number, y: number) {
  const clamp = (value: number) => Math.max(0, Math.min(1, value));
  return {
    x: (clamp(y) - 0.5) * -10,
    y: (clamp(x) - 0.5) * 12,
    light: clamp(x) * 100,
  };
}
