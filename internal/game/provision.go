package game

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"
)

type UnitType string

const (
	UnitFootman UnitType = "footman"
	UnitKnight  UnitType = "knight"
	UnitShip    UnitType = "ship"
	UnitSiege   UnitType = "siege-engine"
)

const (
	PhaseSupply    = "supply"
	PhaseMustering = "mustering"
	PhasePower     = "consolidate_power"
)

type SupplySource string

const SupplyFromWildlings SupplySource = "wildlings"

type MusterSource string

const MusterFromWildlings MusterSource = "wildlings"

type SupplyResolution struct {
	Queue  []string     `json:"queue"`
	Source SupplySource `json:"source,omitempty"`
}
type SupplyArmy struct {
	Region string `json:"region"`
	Units  []Unit `json:"units"`
	Size   int    `json:"size"`
}
type SupplyView struct {
	House  string       `json:"house"`
	Level  int          `json:"level"`
	Limits []int        `json:"limits"`
	Armies []SupplyArmy `json:"armies"`
}
type Mustering struct {
	Queue       []string       `json:"queue"`
	Points      map[string]int `json:"points"`
	PowerRegion string         `json:"powerRegion,omitempty"`
	Source      MusterSource   `json:"source,omitempty"`
	Region      string         `json:"region,omitempty"`
}
type MusterOption struct {
	Region  string   `json:"region"`
	Target  string   `json:"target"`
	Unit    UnitType `json:"unit"`
	Upgrade bool     `json:"upgrade"`
	Cost    int      `json:"cost"`
}
type MusterView struct {
	House       string         `json:"house"`
	Points      map[string]int `json:"points"`
	Reserve     map[string]int `json:"reserve"`
	Options     []MusterOption `json:"options"`
	PowerRegion string         `json:"powerRegion,omitempty"`
}

// Occupation and an established power token are different: disbanding the last
// unit must not manufacture a token, nor discard one that was already placed.
func (r *Room) restoreEmptyControl(id string, c Catalog) {
	region := r.definition(id, c)
	if region.Type != "land" {
		delete(r.Match.Control, id)
		return
	}
	if token := r.Match.ControlTokens[id]; token != "" {
		r.Match.Control[id] = token
	} else if region.Home != "" && r.houseSeat(region.Home) != nil {
		r.Match.Control[id] = region.Home
	} else {
		delete(r.Match.Control, id)
	}
	for _, neighbor := range region.Neighbors {
		if r.definition(neighbor, c).Type != "port" {
			continue
		}
		ships := r.Match.Armies[neighbor]
		if count(ships) > 0 && ships[0].House != r.Match.Control[id] {
			r.Match.Armies[neighbor] = nil
			delete(r.Orders, neighbor)
		}
	}
}

func (r *Room) canStartProvision() bool {
	return r.Phase == PhaseWesteros && r.Match != nil && r.Battle == nil && r.Influence == nil && r.Supply == nil && r.Mustering == nil
}

// Rule-effect hooks, not public commands: a player cannot force a Supply or
// Mustering event. The Westeros automation resumes after these complete.
func (r *Room) BeginSupply(c Catalog, now time.Time) error {
	if !r.canStartProvision() {
		return errors.New("Снабжение доступно только по событию Вестероса")
	}
	r.Supply = &SupplyResolution{Queue: append([]string{}, r.Match.Tracks["ironThrone"]...)}
	r.advanceSupply(c, now)
	return nil
}
func (r *Room) advanceSupply(c Catalog, now time.Time) {
	for len(r.Supply.Queue) > 0 {
		house := r.Supply.Queue[0]
		// Wildling penalties change the track directly, not the map's icons.
		if r.Supply.Source != SupplyFromWildlings {
			level := 0
			for _, region := range c.Boards["westeros"].Regions {
				if r.owner(region.ID) == house {
					level += region.Supply
				}
			}
			r.Match.Supply[house] = min(6, level)
			r.event("supply", house, "", "", fmt.Sprintf("%s: снабжение пересчитано — %d", HouseNames[house], r.Match.Supply[house]))
		}
		if !r.supplyOK(house, r.Match.Armies, c) {
			r.Phase, r.Match.ActiveHouse = PhaseSupply, house
			r.startClock(house, now)
			return
		}
		r.Supply.Queue = append([]string{}, r.Supply.Queue[1:]...)
	}
	phase := PhaseWesteros
	if r.Supply.Source == SupplyFromWildlings {
		phase = PhaseWildlingEffects
	}
	r.Supply = nil
	r.Phase, r.Match.ActiveHouse, r.Clock = phase, "", Clock{}
}
func (r *Room) disband(house, region string, quantities map[string]int, c Catalog, now time.Time) error {
	if r.Phase != PhaseSupply || r.Supply == nil || r.Match.ActiveHouse != house {
		return errors.New("Сейчас не ваше расформирование")
	}
	army := r.Match.Armies[region]
	if count(army) == 0 || army[0].House != house {
		return errors.New("Выберите область со своими отрядами")
	}
	removed := 0
	for typ, amount := range quantities {
		available := 0
		for _, unit := range army {
			if unit.Type == typ {
				available += unit.Count
			}
		}
		if amount < 0 || amount > available {
			return errors.New("Недопустимое количество для расформирования")
		}
		removed += amount
	}
	if removed == 0 {
		return errors.New("Выберите отряды для расформирования")
	}
	remaining := []Unit{}
	for _, unit := range army {
		unit.Count -= quantities[unit.Type]
		if unit.Count > 0 {
			remaining = append(remaining, unit)
		}
	}
	r.Match.Armies[region] = remaining
	if len(remaining) == 0 {
		delete(r.Orders, region)
		r.restoreEmptyControl(region, c)
	}
	r.event("disband", house, region, "", fmt.Sprintf("%s: расформировано отрядов — %d", HouseNames[house], removed))
	if r.supplyOK(house, r.Match.Armies, c) {
		r.Supply.Queue = append([]string{}, r.Supply.Queue[1:]...)
		r.advanceSupply(c, now)
	}
	return nil
}
func (r *Room) supplyView(c Catalog) *SupplyView {
	if r.Supply == nil || r.Phase != PhaseSupply {
		return nil
	}
	house := r.Match.ActiveHouse
	v := &SupplyView{House: house, Level: r.Match.Supply[house], Limits: append([]int{}, c.SupplyLimits[r.Match.Supply[house]]...), Armies: []SupplyArmy{}}
	for _, id := range r.regionIDs(house) {
		v.Armies = append(v.Armies, SupplyArmy{Region: id, Units: append([]Unit{}, r.Match.Armies[id]...), Size: count(r.Match.Armies[id])})
	}
	return v
}

func (r *Room) BeginMustering(c Catalog, now time.Time) error {
	if !r.canStartProvision() {
		return errors.New("Сбор войск доступен только по событию Вестероса")
	}
	r.Mustering = &Mustering{Queue: append([]string{}, r.Match.Tracks["ironThrone"]...)}
	r.advanceMuster(c, now)
	return nil
}
func (r *Room) advanceMuster(c Catalog, now time.Time) {
	m := r.Mustering
	for len(m.Queue) > 0 {
		house := m.Queue[0]
		m.Points = map[string]int{}
		for _, region := range c.Boards["westeros"].Regions {
			if region.Castle > 0 && r.owner(region.ID) == house && (m.PowerRegion == "" || m.PowerRegion == region.ID) && (m.Region == "" || m.Region == region.ID) {
				m.Points[region.ID] = region.Castle
			}
		}
		r.Phase, r.Match.ActiveHouse = PhaseMustering, house
		if len(r.musterOptions(c)) > 0 {
			r.startClock(house, now)
			return
		}
		m.Queue = append([]string{}, m.Queue[1:]...)
	}
	fromPower := m.PowerRegion != ""
	r.Mustering = nil
	r.Phase, r.Match.ActiveHouse, r.Clock = PhaseWesteros, "", Clock{}
	if m.Source == MusterFromWildlings {
		r.Phase = PhaseWildlingEffects
		return
	}
	if fromPower {
		r.advancePower(c, now)
	}
}
func (r *Room) reserve(house string, c Catalog) map[string]int {
	result := copyBids(c.Houses[house].Limits)
	for _, army := range r.Match.Armies {
		for _, unit := range army {
			if unit.House == house {
				result[unit.Type] -= unit.Count
			}
		}
	}
	return result
}
func (r *Room) musterCandidate(house string, option MusterOption, c Catalog) (map[string][]Unit, int, error) {
	invalid := errors.New("Недопустимый набор войск")
	if r.Phase != PhaseMustering || r.Mustering == nil || r.Match.ActiveHouse != house {
		return nil, 0, invalid
	}
	source, target := r.definition(option.Region, c), r.definition(option.Target, c)
	if contains(r.Match.BlockedRegions, source.ID) || contains(r.Match.BlockedRegions, target.ID) {
		return nil, 0, invalid
	}
	if r.Mustering.Region != "" && source.ID != r.Mustering.Region {
		return nil, 0, invalid
	}
	if source.Castle == 0 || r.owner(source.ID) != house || r.Mustering.Points[source.ID] <= 0 {
		return nil, 0, invalid
	}
	cost := 1
	switch option.Unit {
	case UnitFootman, UnitShip:
		if option.Upgrade {
			return nil, 0, invalid
		}
	case UnitKnight, UnitSiege:
		if !option.Upgrade {
			cost = 2
		}
	default:
		return nil, 0, invalid
	}
	if cost > r.Mustering.Points[source.ID] || r.reserve(house, c)[string(option.Unit)] < 1 {
		return nil, 0, invalid
	}
	if option.Unit == UnitShip {
		if !slices.Contains(source.Neighbors, target.ID) || (target.Type != "port" && target.Type != "sea") {
			return nil, 0, invalid
		}
		if count(r.Match.Armies[target.ID]) > 0 && r.Match.Armies[target.ID][0].House != house {
			return nil, 0, invalid
		}
		if target.Type == "port" && count(r.Match.Armies[target.ID]) >= 3 {
			return nil, 0, invalid
		}
	} else if target.ID != source.ID || target.Type != "land" {
		return nil, 0, invalid
	}
	candidate := map[string][]Unit{}
	for id, army := range r.Match.Armies {
		candidate[id] = append([]Unit{}, army...)
	}
	if option.Upgrade {
		found := false
		for i := range candidate[source.ID] {
			unit := &candidate[source.ID][i]
			if unit.House == house && unit.Type == string(UnitFootman) && unit.Count > 0 {
				unit.Count--
				found = true
				break
			}
		}
		if !found {
			return nil, 0, invalid
		}
		candidate[source.ID] = slices.DeleteFunc(candidate[source.ID], func(u Unit) bool { return u.Count == 0 })
	}
	candidate[target.ID] = merge(candidate[target.ID], []Unit{{Type: string(option.Unit), House: house, Count: 1}})
	if !r.supplyOK(house, candidate, c) {
		return nil, 0, invalid
	}
	return candidate, cost, nil
}
func (r *Room) musterOptions(c Catalog) []MusterOption {
	options := []MusterOption{}
	if r.Mustering == nil {
		return options
	}
	house := r.Match.ActiveHouse
	for _, region := range sortedKeys(r.Mustering.Points) {
		for _, unit := range []UnitType{UnitFootman, UnitKnight, UnitSiege, UnitShip} {
			targets := []string{region}
			if unit == UnitShip {
				targets = r.definition(region, c).Neighbors
			}
			for _, target := range targets {
				for _, upgrade := range []bool{false, true} {
					o := MusterOption{Region: region, Target: target, Unit: unit, Upgrade: upgrade}
					if _, cost, err := r.musterCandidate(house, o, c); err == nil {
						o.Cost = cost
						options = append(options, o)
					}
				}
			}
		}
	}
	return options
}
func (r *Room) muster(house string, option MusterOption, c Catalog, now time.Time) error {
	candidate, cost, err := r.musterCandidate(house, option, c)
	if err != nil {
		return err
	}
	r.Match.Armies = candidate
	r.Mustering.Points[option.Region] -= cost
	r.event("muster", house, option.Region, option.Target, HouseNames[house]+": выполнен набор войск")
	if len(r.musterOptions(c)) == 0 {
		return r.finishMuster(house, c, now)
	}
	return nil
}
func (r *Room) finishMuster(house string, c Catalog, now time.Time) error {
	if r.Phase != PhaseMustering || r.Mustering == nil || r.Match.ActiveHouse != house {
		return errors.New("Сейчас не ваш сбор войск")
	}
	r.Mustering.Queue = append([]string{}, r.Mustering.Queue[1:]...)
	r.advanceMuster(c, now)
	return nil
}
func (r *Room) musterView(c Catalog) *MusterView {
	if r.Mustering == nil {
		return nil
	}
	house := r.Match.ActiveHouse
	return &MusterView{House: house, Points: copyBids(r.Mustering.Points), Reserve: r.reserve(house, c), Options: r.musterOptions(c), PowerRegion: r.Mustering.PowerRegion}
}

// Normal orders are automatic; a starred order in a castle pauses for its owner
// to choose power or local mustering. Each house resolves its orders in turn order.
func (r *Room) advancePower(c Catalog, now time.Time) {
	r.Phase = PhasePower
	for _, house := range r.Match.Tracks["ironThrone"] {
		for _, region := range r.regionIDs(house) {
			order, ok := FindOrder(r.Orders[region])
			if !ok || order.Kind != "power" {
				continue
			}
			definition := r.definition(region, c)
			if order.Star && definition.Castle > 0 {
				r.Match.ActiveHouse = house
				r.startClock(house, now)
				return
			}
			if definition.Type == "land" {
				r.gainPower(house, 1+definition.Crowns)
			}
			delete(r.Orders, region)
		}
	}
	r.completeRound(c, now)
}
func (r *Room) consolidate(house, region, choice string, c Catalog, now time.Time) error {
	if r.Phase != PhasePower || r.Match.ActiveHouse != house || r.owner(region) != house || r.Orders[region] != "power-star" {
		return errors.New("Сейчас не ваш особый сбор власти")
	}
	if choice != "power" && choice != "muster" {
		return errors.New("Выберите власть или сбор войск")
	}
	delete(r.Orders, region)
	if choice == "power" {
		r.gainPower(house, 1+r.definition(region, c).Crowns)
		r.advancePower(c, now)
	} else {
		r.Mustering = &Mustering{Queue: []string{house}, PowerRegion: region}
		r.advanceMuster(c, now)
	}
	return nil
}

func (r *Room) advanceProvisionBot(c Catalog, now time.Time, safe bool) bool {
	house := r.Match.ActiveHouse
	seat := r.houseSeat(house)
	if seat == nil || (!safe && !seat.Bot) {
		return false
	}
	switch r.Phase {
	case PhaseSupply:
		ids := r.regionIDs(house)
		sort.SliceStable(ids, func(i, j int) bool { return count(r.Match.Armies[ids[i]]) > count(r.Match.Armies[ids[j]]) })
		if len(ids) == 0 {
			return false
		}
		army := r.Match.Armies[ids[0]]
		typ := army[0].Type
		for _, unit := range army {
			if unit.Type == string(UnitFootman) {
				typ = unit.Type
				break
			}
		}
		return r.disband(house, ids[0], map[string]int{typ: 1}, c, now) == nil
	case PhaseMustering:
		options := r.musterOptions(c)
		if safe || len(options) == 0 {
			return r.finishMuster(house, c, now) == nil
		}
		o := options[0]
		if seat.Difficulty == "easy" {
			o = options[randomIndex(len(options))]
		} else {
			for _, option := range options {
				if option.Upgrade {
					o = option
					break
				}
			}
		}
		return r.muster(house, o, c, now) == nil
	case PhasePower:
		for _, id := range r.regionIDs(house) {
			if r.Orders[id] == "power-star" {
				choice := "muster"
				if safe {
					choice = "power"
				}
				return r.consolidate(house, id, choice, c, now) == nil
			}
		}
	}
	return false
}
