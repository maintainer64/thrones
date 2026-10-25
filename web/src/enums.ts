// JSON values are shared with internal/game/tides.go and card_effects.go.
// Keep these stable: saved rooms and the WebSocket protocol use these IDs.
export enum TidesCardID {
  Zero = "zero",
  Sword = "sword",
  Fortification = "fortification",
  Two = "two",
  Three = "three",
  Skull = "skull",
}

export enum CardChoiceKind {
  Card = "card",
  Replace = "replace",
  Region = "region",
  Track = "track",
  Confirm = "confirm",
  TidesRedraw = "tides_redraw",
  TidesBlade = "tides_blade",
  Blade = "blade",
}
export enum FortificationKind {
  Garrison = "garrison",
  Neutral = "neutral",
  Blocked = "blocked",
}

export enum TidesDecision {
  Keep = "keep",
  Redraw = "redraw",
  UseBlade = "strength",
}

export enum InfluenceTrack {
  IronThrone = "ironThrone",
  Fiefdoms = "fiefdoms",
  KingsCourt = "kingsCourt",
}

export enum AuctionStage {
  Bidding = "bidding",
  Ties = "ties",
}

export enum UnitType {
  Footman = "footman",
  Knight = "knight",
  Ship = "ship",
  Siege = "siege-engine",
}

export enum WildlingCardID {
  Silence = "silence-at-the-wall",
  Preemptive = "preemptive-raid",
  CrowKillers = "crow-killers",
  Raiders = "rattleshirts-raiders",
  Milkwater = "massing-on-the-milkwater",
  King = "a-king-beyond-the-wall",
  Mammoths = "mammoth-riders",
  Horde = "the-horde-descends",
  Skinchanger = "skinchanger-scout",
}

export enum WesterosCardID {
  Supply = "supply",
  Mustering = "mustering",
  Throne = "a-throne-of-blades",
  Summer = "last-days-of-summer",
  Winter = "winter-is-coming",
  Thrones = "game-of-thrones",
  Clash = "clash-of-kings",
  Wings = "dark-wings-dark-words",
  Sword = "put-to-the-sword",
  Storm = "storm-of-swords",
  Rains = "rains-of-autumn",
  Sea = "sea-of-storms",
  Lies = "web-of-lies",
  Feast = "feast-for-crows",
  Wildlings = "wildlings-attack",
}

export enum WildlingChoiceKind {
  Track = "influence-track",
  Discard = "discard-card",
  Upgrade = "upgrade-footman",
  Downgrade = "downgrade-knight",
  DestroyKnight = "destroy-knight",
  Destroy = "destroy-unit",
  Retrieve = "retrieve-card",
  Castle = "muster-region",
  Penalty = "preemptive-penalty",
}

export enum WesterosDecision {
  Nothing = "nothing",
  Supply = "supply",
  Muster = "mustering",
  Clash = "clash-of-kings",
  Power = "game-of-thrones",
  NoDefense = "no-defense",
  NoMarch = "no-march-plus-one",
}

export enum RavenStage {
  Choose = "choose",
  Peek = "peek",
}

export enum RavenDecision {
  Skip = "skip",
  Replace = "replace",
  Peek = "peek",
  Keep = "keep",
  Bottom = "bottom",
}
export enum CombatLineKind {
  Unit = "unit",
  UnitEffect = "unit_effect",
  SupportUnit = "support_unit",
  SupportOrder = "support_order",
  March = "march",
  Defense = "defense",
  Garrison = "garrison",
  HouseCard = "house_card",
  CardEffect = "card_effect",
  Blade = "blade",
  Tides = "tides",
  Minimum = "minimum",
}
