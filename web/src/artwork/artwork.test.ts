import { describe, expect, it } from "vitest";
import { artwork, artworkURL, originalPortrait } from "./artwork";
import { textureKey } from "../cards/card-art";
import media from "../../public/media/manifest.json" with { type: "json" };

describe("original HD artwork", () => {
  it("does not publish the obsolete public PNG card collection", () => {
    expect(media).not.toHaveProperty("cards");
  });
  it("indexes all 42 portraits, 24 events, six Tides, five order bases and the shield", () => {
    const paths = Object.keys(artwork);
    expect(paths.filter((path) => path.startsWith("T_crd_"))).toHaveLength(42);
    expect(paths.filter((path) => path.startsWith("events/"))).toHaveLength(24);
    expect(
      paths.filter((path) => path.startsWith("tides-of-battle/")),
    ).toHaveLength(6);
    expect(artwork["ui/strength-shield.png"]).toBeDefined();
    expect(
      paths.filter((path) => path.startsWith("media/orders/")),
    ).toHaveLength(5);
    for (const [path, image] of Object.entries(artwork)) {
      expect(image.revision).toMatch(/^[a-f0-9]{12}$/);
      expect(image.sourceRevision).toMatch(/^[a-f0-9]{12}$/);
      expect(image.file).toBe(path.replace(/\.png$/, ".webp"));
      expect(image.bytes).toBeGreaterThan(0);
      expect(image.sourceBytes).toBeGreaterThan(0);
      expect(image.width).toBeGreaterThanOrEqual(1000);
      // Event illustrations include wide landscapes; don't force a card ratio.
      expect(image.height).toBeGreaterThan(0);
      expect(artworkURL(path)).toBe(
        `${path.startsWith("media/") ? "/" : "/local-assets/"}${image.file}?v=${image.revision}`,
      );
      if (path.startsWith("T_crd_")) {
        expect(image.height).toBeGreaterThanOrEqual(1000);
        expect(originalPortrait(path.replace(/\.png$/, ""))).toBe(
          artworkURL(path),
        );
      }
    }
  });
  it("uses HD originals for all six leaders and differently shaped portraits", () => {
    for (const [house, card] of [
      ["stark", "eddard-stark"],
      ["greyjoy", "balon-greyjoy"],
      ["lannister", "tywin-lannister"],
      ["baratheon", "stannis-baratheon"],
      ["tyrell", "mace-tyrell"],
      ["martell", "doran-martell"],
      ["baratheon", "brienne-of-tarth"],
      ["greyjoy", "victarion-greyjoy"],
    ]) {
      expect(originalPortrait(textureKey(card, house))).toMatch(
        /\.webp\?v=[a-f0-9]{12}$/,
      );
    }
  });
  it("does not invent missing textures or break unindexed local URLs", () => {
    expect(originalPortrait("T_crd_missing_person")).toBeUndefined();
    expect(artworkURL("optional.png")).toBe("/local-assets/optional.png");
  });
});
