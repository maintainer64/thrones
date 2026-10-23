import { useId } from "react";
import "./game-symbols.css";

// Vector game pieces, not interface outline icons. Shared by house cards,
// combat captions and order tokens; remain sharp at hand and inspector sizes.
export function GameSymbol({ kind }: { kind: "sword" | "tower" | "star" }) {
  const id = useId().replace(/:/g, "");
  return (
    <svg
      className={`game-symbol game-symbol-${kind}`}
      data-symbol={kind}
      viewBox="0 0 64 64"
      aria-hidden="true"
      focusable="false"
    >
      <defs>
        <linearGradient id={`${id}-steel`} x1="0" y1="0" x2="1" y2="0">
          <stop stopColor="#555c62" />
          <stop offset=".35" stopColor="#f0eee0" />
          <stop offset=".52" stopColor="#bac1be" />
          <stop offset="1" stopColor="#69747a" />
        </linearGradient>
        <linearGradient id={`${id}-gold`} x1="0" y1="0" x2="1" y2="1">
          <stop stopColor="#fff0ae" />
          <stop offset=".4" stopColor="#dcb467" />
          <stop offset="1" stopColor="#79501f" />
        </linearGradient>
      </defs>
      {kind === "sword" ? (
        <g
          transform="rotate(32 32 32)"
          stroke="#342b20"
          strokeWidth="1.8"
          strokeLinejoin="round"
        >
          <path d="M32 3 39 15 36 43 28 43 25 15Z" fill={`url(#${id}-steel)`} />
          <path d="M32 7V42" stroke="#f9f5e2" strokeWidth="1" />
          <path
            d="m19 41 4-3 9 3 9-3 4 3-1 5-12-2-12 2Z"
            fill={`url(#${id}-gold)`}
          />
          <path d="M29 45h6v12h-6Z" fill="#583a25" />
          <path d="m29 48 6 2m-6 2 6 2" stroke="#ba9357" strokeWidth="1.5" />
          <path d="m32 56 5 3-5 3-5-3Z" fill={`url(#${id}-gold)`} />
        </g>
      ) : kind === "tower" ? (
        <g stroke="#392e21" strokeWidth="1.8" strokeLinejoin="round">
          <path
            d="M12 5h10v10h5V5h10v10h5V5h10v19l-5 5 3 24h5v7H9v-7h5l3-24-5-5Z"
            fill={`url(#${id}-steel)`}
          />
          <path
            d="M13 24h38M17 30h30M16 39h10m12 0h10M14 52h36M32 25v10m-11 5v10m22-10v10"
            stroke="#747a75"
          />
          <path d="M26 59V46a6 6 0 0 1 12 0v13" fill="#302c26" />
          <path d="M29 28h6v7h-6Z" fill="#343b39" />
          <path d="M10 55h44" stroke="#e0c488" strokeWidth="2" />
        </g>
      ) : (
        <>
          <path
            d="m32 3 8.6 18 19.8 2.8L46 37.7l3.4 20L32 48.3l-17.4 9.4 3.4-20L3.6 23.8 23.4 21Z"
            fill={`url(#${id}-gold)`}
            stroke="#604114"
            strokeWidth="3"
            strokeLinejoin="round"
          />
          <path
            d="m32 8 0 26-11 17 3-17-14-8 18-2Z"
            fill="#fff1bb"
            opacity=".6"
          />
        </>
      )}
    </svg>
  );
}
