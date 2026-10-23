import { TidesCardID } from "../enums";
import { artwork, artworkURL, type Artwork } from "../artwork/artwork";

// Full-resolution WebP faces generated from untouched user-supplied PNGs.
// A content revision prevents old faces staying in browser cache.
export const tidesArt = Object.fromEntries(
  Object.values(TidesCardID).map((id) => [
    id,
    artwork[`tides-of-battle/${id}.png`],
  ]),
) as Record<TidesCardID, Artwork>;

export function tidesArtURL(id: TidesCardID) {
  return artworkURL(`tides-of-battle/${id}.png`);
}
