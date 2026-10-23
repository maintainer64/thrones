package game

import "errors"

type PlayerSetup struct {
	PlayerCount    int            `json:"playerCount"`
	Houses         []string       `json:"houses"`
	BlockedRegions []string       `json:"blockedRegions,omitempty"`
	Garrisons      map[string]int `json:"garrisons,omitempty"`
}
type ScenarioSetup struct {
	PlayerSetups []PlayerSetup `json:"playerSetups"`
}
type FortificationKind string

const (
	FortificationGarrison FortificationKind = "garrison"
	FortificationNeutral  FortificationKind = "neutral"
	FortificationBlocked  FortificationKind = "blocked"
)

type Fortification struct {
	Kind     FortificationKind `json:"kind"`
	Strength int               `json:"strength"`
	House    string            `json:"house,omitempty"`
}
type SetupView struct {
	PlayerCount int                 `json:"playerCount"`
	Armies      map[string][]Unit   `json:"armies"`
	Control     map[string]string   `json:"control"`
	Tracks      map[string][]string `json:"tracks"`
}

func baseSetup(c Catalog, count int) PlayerSetup {
	for _, setup := range c.Setups["base-game"].PlayerSetups {
		if setup.PlayerCount == count {
			return setup
		}
	}
	panic("base setup missing from catalog")
}

func (s Settings) houseCount() int {
	if s.PlayerCount == 0 {
		return 6
	}
	return s.PlayerCount
}

// Only unoccupied seats/bots may disappear when selecting a smaller setup.
// Never silently move or evict an already connected human.
func (r *Room) configureSettings(settings Settings, c Catalog) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	setup := baseSetup(c, settings.houseCount())
	for _, seat := range r.Seats {
		if seat.Player != "" && !contains(setup.Houses, seat.House) {
			return errors.New("В выбранной расстановке нет занятого дома: сначала выберите доступный дом")
		}
	}
	seats := []Seat{}
	for _, house := range BaseHouses {
		if !contains(setup.Houses, house) {
			continue
		}
		seat := Seat{House: house, Bot: true, Difficulty: "normal", Name: "Бот · " + HouseNames[house]}
		if current := r.houseSeat(house); current != nil {
			seat = *current
		}
		seats = append(seats, seat)
	}
	r.Seats, r.Settings = seats, settings
	return nil
}

func courtStars(count int) []int {
	if count <= 4 {
		return []int{3, 2, 1, 0}
	}
	return []int{3, 3, 2, 1, 0, 0}
}

func (r *Room) startingMatch(c Catalog) *Match {
	setup := baseSetup(c, r.Settings.houseCount())
	m := &Match{Armies: map[string][]Unit{}, Control: map[string]string{}, Power: map[string]int{}, Supply: map[string]int{}, Tracks: map[string][]string{}, Discard: map[string][]string{}, Garrisons: map[string]int{}, BlockedRegions: append([]string(nil), setup.BlockedRegions...)}
	for region, army := range c.Units {
		for _, unit := range army {
			if contains(setup.Houses, unit.House) && !contains(setup.BlockedRegions, region) {
				m.Armies[region] = append(m.Armies[region], unit)
				m.Control[region] = unit.House
			}
		}
	}
	// Apply the official FAQ 2.0 Lannister starting fleet in every base setup.
	// The imported community catalog's 3-player removedUnits variant is not used.
	for _, region := range c.Boards["westeros"].Regions {
		if contains(setup.BlockedRegions, region.ID) {
			continue
		}
		if contains(setup.Houses, region.Home) {
			m.Control[region.ID] = region.Home
		}
		if region.Garrison > 0 && (region.Home == "" || contains(setup.Houses, region.Home)) {
			m.Garrisons[region.ID] = region.Garrison
		}
	}
	for id, strength := range setup.Garrisons {
		m.Garrisons[id] = strength
	}
	for _, house := range setup.Houses {
		m.Power[house], m.Supply[house], m.Discard[house] = 5, c.Houses[house].Supply, []string{}
	}
	for name, track := range c.Tracks {
		m.Tracks[name] = []string{}
		for _, house := range track {
			if contains(setup.Houses, house) {
				m.Tracks[name] = append(m.Tracks[name], house)
			}
		}
	}
	return m
}

func (r *Room) fortifications(m *Match, c Catalog) map[string]Fortification {
	out := map[string]Fortification{}
	for id, strength := range m.Garrisons {
		if strength <= 0 {
			continue
		}
		value := Fortification{Kind: FortificationNeutral, Strength: strength}
		if house := r.definition(id, c).Home; house != "" && r.houseSeat(house) != nil {
			value.Kind, value.House = FortificationGarrison, house
		}
		out[id] = value
	}
	for _, id := range m.BlockedRegions {
		out[id] = Fortification{Kind: FortificationBlocked}
	}
	return out
}
