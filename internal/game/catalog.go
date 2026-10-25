package game

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed catalog.json
var catalogJSON []byte

//go:embed locales/ru.json
var russianJSON []byte

//go:embed locales/en.json
var englishJSON []byte

type Region struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	X         float64  `json:"x"`
	Y         float64  `json:"y"`
	Supply    int      `json:"supplyIcons"`
	Crowns    int      `json:"crownIcons"`
	Castle    int      `json:"castleLevel"`
	Neighbors []string `json:"neighbors"`
	Garrison  int      `json:"garrison"`
	Home      string   `json:"superController"`
}
type Board struct {
	ID      string   `json:"id"`
	Width   float64  `json:"width"`
	Height  float64  `json:"height"`
	Image   string   `json:"image"`
	Regions []Region `json:"regions"`
}
type Card struct {
	Name        string `json:"name"`
	Strength    int    `json:"combatStrength"`
	Swords      int    `json:"swordIcons"`
	Towers      int    `json:"towerIcons"`
	Ability     string `json:"ability,omitempty"`
	Description string `json:"description,omitempty"`
	Implemented bool   `json:"implemented"`
}
type House struct {
	Name   string          `json:"name"`
	Color  string          `json:"color"`
	Supply int             `json:"supplyLevel"`
	Cards  map[string]Card `json:"houseCards"`
	Limits map[string]int  `json:"unitLimits"`
}
type Unit struct {
	Type  string `json:"unitType"`
	House string `json:"house"`
	Count int    `json:"quantity"`
}
type Catalog struct {
	Houses       map[string]House         `json:"houses"`
	Units        map[string][]Unit        `json:"units"`
	Tracks       map[string][]string      `json:"tracks"`
	Boards       map[string]Board         `json:"boards"`
	SupplyLimits [][]int                  `json:"supplyRestrictions"`
	Setups       map[string]ScenarioSetup `json:"setups"`
}

func LoadCatalog() Catalog {
	return LoadCatalogForLanguage("ru")
}
func LoadCatalogForLanguage(language string) Catalog {
	var c Catalog
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		panic(fmt.Errorf("load game data: %w", err))
	}
	locale := localeForLanguage(language)
	for _, house := range c.Houses {
		for id, card := range house.Cards {
			if text, ok := locale.Cards[id]; ok {
				card.Name, card.Description = text.Name, text.Description
				card.Implemented = card.Ability == "" || implementedCardEffect(card.Ability)
				house.Cards[id] = card
			}
		}
	}
	return c
}

var HouseNames = map[string]string{"stark": "Старк", "greyjoy": "Грейджой", "lannister": "Ланнистер", "baratheon": "Баратеон", "tyrell": "Тирелл", "martell": "Мартелл", "arryn": "Аррен", "targaryen": "Таргариен"}
var BaseHouses = []string{"stark", "greyjoy", "lannister", "baratheon", "tyrell", "martell"}

type Order struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Star     bool   `json:"star"`
	Modifier int    `json:"modifier"`
}

var Orders = []Order{
	{"march-minus", "Поход −1", "march", false, -1}, {"march", "Поход", "march", false, 0}, {"march-star", "Поход +1 ★", "march", true, 1},
	{"defend-1", "Оборона +1", "defend", false, 1}, {"defend-2", "Оборона +1", "defend", false, 1}, {"defend-star", "Оборона +2 ★", "defend", true, 2},
	{"support-1", "Поддержка", "support", false, 0}, {"support-2", "Поддержка", "support", false, 0}, {"support-star", "Поддержка +1 ★", "support", true, 1},
	{"raid-1", "Набег", "raid", false, 0}, {"raid-2", "Набег", "raid", false, 0}, {"raid-star", "Набег ★", "raid", true, 0},
	{"power-1", "Сбор власти", "power", false, 0}, {"power-2", "Сбор власти", "power", false, 0}, {"power-star", "Сбор власти ★", "power", true, 0},
}

func FindOrder(id string) (Order, bool) {
	for _, o := range Orders {
		if o.ID == id {
			return o, true
		}
	}
	return Order{}, false
}
