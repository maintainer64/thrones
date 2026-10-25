import { getLanguage, t } from "./i18n/core";
import { eventText } from "./events/model";
import type {
  AuctionStage,
  CardChoiceKind,
  InfluenceTrack,
  TidesCardID,
  UnitType,
  WildlingCardID,
  WildlingChoiceKind,
  WesterosCardID,
  WesterosDecision,
  RavenStage,
  FortificationKind,
  CombatLineKind,
} from "./enums";
export type Fortification = {
  kind: FortificationKind;
  strength: number;
  house?: string;
};
export type Unit = {
  unitType: string;
  house: string;
  quantity: number;
};
export type Order = {
  id: string;
  name: string;
  kind: string;
  star: boolean;
  modifier: number;
};
export type Region = {
  id: string;
  name: string;
  type: string;
  x: number;
  y: number;
  supplyIcons: number;
  crownIcons: number;
  castleLevel: number;
  neighbors: string[];
};
export type Card = {
  name: string;
  combatStrength: number;
  swordIcons: number;
  towerIcons: number;
  ability?: string;
  description?: string;
  implemented?: boolean;
};
export type Settings = {
  turnMinutes: number;
  hardTimer: boolean;
  scenario: string;
  tidesOfBattle: boolean;
  playerCount?: number;
};
export type Catalog = {
  baseSetups: { playerCount: number; houses: string[] }[];
  board: {
    width: number;
    height: number;
    image: string;
    regions: Region[];
  };
  houses: Record<
    string,
    {
      name: string;
      color: string;
      supplyLevel: number;
      houseCards: Record<string, Card>;
    }
  >;
  houseNames: Record<string, string>;
  units: Record<string, Unit[]>;
  orders: Order[];
};
export type Event = {
  id: number;
  type: string;
  house: string;
  from?: string;
  to?: string;
  text: string;
};
export type Match = {
  combats?: CombatRecord[];
  bladeUsed: boolean;
  ravenUsed: boolean;
  lastInfluence?: InfluenceResult[];
  armies: Record<string, Unit[]>;
  control: Record<string, string>;
  controlTokens?: Record<string, string>;
  power: Record<string, number>;
  supply: Record<string, number>;
  tracks: Record<string, string[]>;
  discard: Record<string, string[]>;
  activeHouse: string;
  winner?: string;
  lastCombat?: {
    id?: number;
    strength?: CombatStrength;
    attacker: string;
    defender: string;
    attackerCard: string;
    defenderCard: string;
    attack: number;
    defense: number;
    winner: string;
    region: string;
    effects?: string[];
    modifiers?: Record<string, TidesCard>;
    bladeBonus?: string;
    bladeRedraw?: string;
    losses?: Record<string, Unit[]>;
    retreatHouse?: string;
    retreatTo?: string;
    returnTo?: string;
    final?: {
      armies: Record<string, Unit[]>;
      control: Record<string, string>;
      garrisons: Record<string, number>;
    };
  };
};
export type Room = {
  setup?: {
    playerCount: number;
    armies: Record<string, Unit[]>;
    control: Record<string, string>;
    tracks: Record<string, string[]>;
  };
  fortifications?: Record<string, Fortification>;
  courtStars?: number[];
  id: string;
  name: string;
  revision: number;
  phase: string;
  round: number;
  owner: boolean;
  yourHouse: string;
  seats: {
    house: string;
    name: string;
    bot: boolean;
    difficulty: string;
    ready: boolean;
    you: boolean;
    occupied: boolean;
  }[];
  orders: Record<string, string>;
  orderBudget?: { power: number; starsAllowed: number; starsUsed: number };
  log: string[];
  settings: Settings;
  clock: {
    house: string;
    startedAt: string;
    deadline: string;
  };
  serverTime: string;
  match?: Match;
  influence?: InfluenceAuction;
  raven?: {
    round: number;
    house: string;
    stage: RavenStage;
    card?: WildlingCardID;
    options?: Record<string, string[]>;
  };
  westeros?: {
    threat: number;
    decks: { remaining: number; discard: WesterosCardID[] }[];
    current?: { round: number; cards: WesterosCardID[]; winterDraws: number[] };
    run?: {
      round: number;
      stage: string;
      nextCard: number;
      restriction: string;
      choice?: {
        house: string;
        card: WesterosCardID;
        options: WesterosDecision[];
      };
      attack?: { trigger: string; strength: number };
    };
  };
  wildlings?: {
    remaining: number;
    attack?: {
      strength: number;
      bids: SealedBidsView;
      tieBreakHouse: string;
      tied: string[];
      excludedHouse?: string;
      outcome?: { total: number; watchWon: boolean; selectedHouse: string };
      card?: WildlingCardID;
      effectsImplemented: boolean;
      choice?: WildlingChoice;
    };
    last?: {
      strength: number;
      outcome: { total: number; watchWon: boolean; selectedHouse: string };
      card: WildlingCardID;
      bids: Record<string, number>;
      threatAfter: number;
      excludedHouse?: string;
    };
  };
  supply?: {
    house: string;
    level: number;
    limits: number[];
    armies: { region: string; units: Unit[]; size: number }[];
  };
  mustering?: {
    house: string;
    points: Record<string, number>;
    reserve: Record<string, number>;
    options: MusterOption[];
    powerRegion?: string;
  };
  battle?: {
    id?: number;
    strength?: CombatStrength;
    modifiers?: Record<string, TidesCard>;
    modifiersRevealed: boolean;
    from: string;
    to: string;
    attacker: string;
    defender: string;
    army: Unit[];
    chosen: Record<string, boolean>;
    yourCard?: string;
    retreats: string[];
    loser?: string;
    retreatChooser?: string;
    cards?: Record<string, string>;
    choice?: {
      house: string;
      card: string;
      kind: CardChoiceKind;
      options: string[] | null;
      optional: boolean;
    };
  };
  events: Event[];
  availableCards: string[];
  legalRegions: string[];
  availableOrders: Record<string, string[]>;
};
export type Assets = {
  units: Record<string, Record<string, string>>;
  orders: Record<string, string>;
};
export const defaultSettings: Settings = {
  playerCount: 6,
  turnMinutes: 5,
  hardTimer: false,
  scenario: "base",
  tidesOfBattle: false,
};

export type TidesCard = {
  id: TidesCardID;
  strength: number;
  swords: number;
  towers: number;
  skulls: number;
};
export type InfluenceResult = {
  track: InfluenceTrack;
  bids: Record<string, number>;
  ranking: string[];
};
export type MusterOption = {
  region: string;
  target: string;
  unit: UnitType;
  upgrade: boolean;
  cost: number;
};
export type InfluenceAuction = {
  track: InfluenceTrack;
  stage: AuctionStage;
  bids: SealedBidsView;
  tieBreakHouse: string;
  ranking: string[];
  tied: string[];
  results: InfluenceResult[];
};
export type SealedBidsView = {
  submitted: Record<string, boolean>;
  yourBid?: number;
  values?: Record<string, number>;
};
export type WildlingChoice = {
  house: string;
  kind: WildlingChoiceKind;
  options: string[];
  count?: number;
  unitOptions?: { id: string; region: string; unit: UnitType; count: number }[];
};
export function getPhases(): Record<string, string> {
  return {
    lobby: t("phases.lobby"),
    planning: t("phases.planning"),
    orders_revealed: t("phases.revealed"),
    raven: t("raven.title"),
    raids: t("phases.raids"),
    marches: t("phases.marches"),
    combat: t("phases.combat"),
    retreat: t("phases.retreat"),
    card_effect: t("phases.cardEffect"),
    westeros: t("phases.westeros"),
    supply: t("phases.supply"),
    mustering: t("phases.mustering"),
    consolidate_power: t("phases.consolidatePower"),
    influence_bidding: t("phases.influenceBidding"),
    influence_ties: t("phases.influenceTies"),
    wildling_bidding: eventText("biddingTitle"),
    wildling_ties: eventText("tieTitle"),
    wildling_effects: eventText("effectsTitle"),
    finished: t("phases.finished"),
  };
}
export function getUnitNames(): Record<string, string> {
  return {
    footman: t("units.footman"),
    knight: t("units.knight"),
    ship: t("units.ship"),
    "siege-engine": t("units.siegeEngine"),
  };
}
const regions: Record<string, string> = {
  "sunset-sea": "Закатное море",
  seagard: "Сигард",
  "flints-finger": "Кремневый Палец",
  "searoad-marches": "Марки Морской дороги",
  "widows-watch": "Вдовий Дозор",
  "the-arbor": "Арбор",
  "east-summer-sea": "Восточное Летнее море",
  "west-summer-sea": "Западное Летнее море",
  "port-of-white-harbor": "Порт Белой Гавани",
  "port-of-dragonstone": "Порт Драконьего Камня",
  "port-of-sunspear": "Порт Солнечного Копья",
  "port-of-storms-end": "Порт Штормового Предела",
  "port-of-oldtown": "Порт Староместа",
  winterfell: "Винтерфелл",
  "white-harbor": "Белая Гавань",
  "the-shivering-sea": "Дрожащее море",
  "the-narrow-sea": "Узкое море",
  dragonstone: "Драконий Камень",
  "blackwater-bay": "Черноводный залив",
  kingswood: "Королевский лес",
  lannisport: "Ланниспорт",
  "the-golden-sound": "Золотой пролив",
  "stoney-sept": "Каменная Септа",
  pyke: "Пайк",
  "ironmans-bay": "Залив Железных Людей",
  "greywater-watch": "Сероводье",
  highgarden: "Хайгарден",
  "dornish-marches": "Дорнийские марки",
  "redwyne-straights": "Пролив Редвина",
  sunspear: "Солнечное Копьё",
  "salt-shore": "Солёный Берег",
  "sea-of-dorne": "Дорнийское море",
  "kings-landing": "Королевская Гавань",
  "the-reach": "Простор",
  blackwater: "Черноводная",
  riverrun: "Риверран",
  "the-eyrie": "Орлиное Гнездо",
  harrenhal: "Харренхолл",
  "the-twins": "Близнецы",
  "moat-cailin": "Ров Кейлин",
  "the-sunset-sea": "Закатное море",
  "bay-of-ice": "Ледовый залив",
  "the-stony-shore": "Каменный берег",
  "port-of-winterfell": "Порт Винтерфелла",
  "port-of-pyke": "Порт Пайка",
  "port-of-lannisport": "Порт Ланниспорта",
  karhold: "Кархолд",
  "castle-black": "Чёрный замок",
  "shipbreaker-bay": "Губительный залив",
  "storms-end": "Штормовой Предел",
  oldtown: "Старомест",
  starfall: "Звездопад",
  yronwood: "Айронвуд",
  "three-towers": "Три Башни",
  "princes-pass": "Принцев перевал",
  "the-boneway": "Костяной Путь",
  searoad: "Морская дорога",
  "crackclaw-point": "Расколотый мыс",
  "the-fingers": "Персты",
  "the-mountains-of-the-moon": "Лунные горы",
};
export function regionName(region: Region | undefined) {
  return region
    ? getLanguage() === "en"
      ? region.name
      : regions[region.id] || region.name
    : t("board.selectArea");
}
export function castleCount(room: Room, house: string, catalog: Catalog) {
  return catalog.board.regions.filter(
    (r) =>
      r.castleLevel > 0 &&
      (room.match?.control || room.setup?.control)?.[r.id] === house,
  ).length;
}
export async function api<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    credentials: "same-origin",
    method: body ? "POST" : "GET",
    headers: {
      "Accept-Language": getLanguage(),
      ...(body ? { "Content-Type": "application/json" } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  const result = await response.json();
  if (!response.ok)
    throw Object.assign(new Error(result.error || `HTTP ${response.status}`), {
      status: response.status,
    });
  return result;
}
export type CombatStrengthLine = {
  kind: CombatLineKind;
  region?: string;
  unit?: string;
  count?: number;
  each?: number;
  value: number;
  card?: string;
};
export type CombatSideStrength = {
  house: string;
  lines: CombatStrengthLine[];
  total: number;
};
export type CombatStrength = {
  attacker: CombatSideStrength;
  defender: CombatSideStrength;
  revealed: boolean;
  castle: boolean;
};
export type CombatRecord = {
  id: number;
  round: number;
  from: string;
  to: string;
  attacker: string;
  defender: string;
  attackers: Unit[];
  defenders: Unit[];
  remaining: Unit[];
  initial: CombatStrength;
  result?: NonNullable<Match["lastCombat"]>;
};
