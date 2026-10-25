import index from "./index.json" with { type: "json" };

export type Artwork = {
  width: number;
  height: number;
  revision: string;
  file: string;
  sourceRevision: string;
  sourceBytes: number;
  bytes: number;
  encoding: string;
};
export const artwork: Record<string, Artwork> = index;

export function artworkURL(path: string) {
  const image = artwork[path];
  const revision = image?.revision;
  const file = image?.file || path;
  const prefix = path.startsWith("media/") ? "/" : "/local-assets/";
  return `${prefix}${file}${revision ? `?v=${revision}` : ""}`;
}

// Legacy Unity atlases are only 512px with gutters. HD originals include a
// square Brienne portrait, so resolution, not aspect ratio, distinguishes them.
export function originalPortrait(key: string) {
  const path = `${key}.png`;
  const image = artwork[path];
  return image && image.width >= 1000 && image.height >= 1000
    ? artworkURL(path)
    : undefined;
}
