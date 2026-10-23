import type { CSSProperties } from "react";
import { Bot, Castle, Check, Coins, Package, UserRound } from "lucide-react";
import { t } from "../i18n/core";
import { castleCount, type Catalog, type Room } from "../types";
import { HousePortrait } from "./HousePortrait";

export function HouseSeat({
  seat,
  room,
  catalog,
  art,
  busy,
  onConfigure,
}: {
  seat: Room["seats"][number];
  room: Room;
  catalog: Catalog;
  art: (card: string, house: string) => string | undefined;
  busy: boolean;
  onConfigure: (bot: boolean, difficulty: string) => void;
}) {
  const name = catalog.houseNames[seat.house];
  const player = seat.name || t("players.emptySeat");
  return (
    <section
      aria-label={name}
      className={`seat ${seat.you ? "you" : ""} ${room.match?.activeHouse === seat.house ? "taking-turn" : ""}`}
      style={{ "--house": catalog.houses[seat.house].color } as CSSProperties}
    >
      <div className="seat-title">
        <HousePortrait house={seat.house} art={art} />
        <div className="seat-identity">
          <b title={name}>{name}</b>
          <span className="seat-name" title={player}>
            {seat.bot ? <Bot size={12} /> : <UserRound size={12} />}
            <span>{player}</span>
          </span>
        </div>
        {seat.ready && room.phase === "planning" && (
          <Check size={18} aria-label={t("players.ordersReady")} />
        )}
      </div>
      <dl className="seat-resources">
        <div title={t("resources.power")}>
          <dt>
            <Coins size={13} />
            <span className="sr-only">{t("resources.power")}</span>
          </dt>
          <dd>{room.match?.power[seat.house] ?? 5}</dd>
        </div>
        <div title={t("resources.supply")}>
          <dt>
            <Package size={13} />
            <span className="sr-only">{t("resources.supply")}</span>
          </dt>
          <dd>
            {room.match?.supply[seat.house] ??
              catalog.houses[seat.house].supplyLevel}
          </dd>
        </div>
        <div title={t("resources.castles")}>
          <dt>
            <Castle size={13} />
            <span className="sr-only">{t("resources.castles")}</span>
          </dt>
          <dd>
            {castleCount(room, seat.house, catalog)}
            <small>/7</small>
          </dd>
        </div>
      </dl>
      {room.phase === "lobby" && room.owner && !seat.you && (
        <div className="seat-controls">
          <label>
            <span>{t("players.controller")}</span>
            <select
              aria-label={t("players.housePlayer", { p0: name })}
              disabled={busy || (seat.occupied && !seat.bot)}
              value={seat.bot ? "bot" : "human"}
              onChange={(event) =>
                onConfigure(event.target.value === "bot", seat.difficulty)
              }
            >
              <option value="human">{t("players.human")}</option>
              <option value="bot">{t("players.bot")}</option>
            </select>
          </label>
          {seat.bot && (
            <label>
              <span>{t("players.difficulty")}</span>
              <select
                aria-label={t("players.botDifficulty")}
                disabled={busy}
                value={seat.difficulty}
                onChange={(event) => onConfigure(true, event.target.value)}
              >
                <option value="easy">{t("difficulty.easy")}</option>
                <option value="normal">{t("difficulty.normal")}</option>
                <option value="hard">{t("difficulty.hard")}</option>
              </select>
            </label>
          )}
        </div>
      )}
    </section>
  );
}
