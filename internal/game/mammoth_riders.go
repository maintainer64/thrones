package game

import (
	"errors"
	"slices"
	"time"
)

type WildlingUnitOption struct {
	ID     string   `json:"id"`
	Region string   `json:"region"`
	Unit   UnitType `json:"unit"`
	Count  int      `json:"count"`
}

func (r *Room) wildlingLossOptions(house string) ([]WildlingUnitOption, int) {
	options, total := []WildlingUnitOption{}, 0
	for _, region := range r.regionIDs(house) {
		for _, typ := range []UnitType{UnitFootman, UnitKnight, UnitShip, UnitSiege} {
			n := 0
			for _, unit := range r.Match.Armies[region] {
				if unit.House == house && unit.Type == string(typ) && unit.Count > 0 {
					n += unit.Count
				}
			}
			if n > 0 {
				options, total = append(options, WildlingUnitOption{ID: region + ":" + string(typ), Region: region, Unit: typ, Count: n}), total+n
			}
		}
	}
	return options, total
}

func (r *Room) advanceMammothRiders(c Catalog, now time.Time) bool {
	a := r.Wildlings.Attack
	stage := WildlingUnitsEffect
	if a.WatchWon {
		stage = WildlingHandEffect
	}
	if a.Effect == nil {
		queue := []string{a.SelectedHouse}
		if !a.WatchWon {
			queue = append(queue, a.otherHouses()...)
		}
		a.Effect = &WildlingEffect{Stage: stage, Queue: queue}
	}
	e := a.Effect
	if e.Stage != stage || e.Choice != nil {
		return false
	}
	if len(e.Queue) == 0 {
		return r.finishWildlingAttack()
	}
	house := e.Queue[0]
	if a.WatchWon {
		cards := slices.Clone(r.Match.Discard[house])
		slices.Sort(cards)
		if len(cards) == 0 {
			e.Queue = slices.Clone(e.Queue[1:])
			return true
		}
		e.Choice = &WildlingChoice{House: house, Kind: WildlingRetrieveCard, Options: append(cards, WildlingFinishReward), Count: 1}
	} else {
		units, total := r.wildlingLossOptions(house)
		if e.Units == nil {
			quota := 2
			if house == a.SelectedHouse {
				quota = 3
			}
			e.Units = &WildlingUnitEffect{Remaining: min(quota, total)}
		}
		if e.Units.Remaining == 0 {
			r.finishWildlingLossHouse(c)
			return true
		}
		ids := make([]string, 0, len(units))
		for _, unit := range units {
			ids = append(ids, unit.ID)
		}
		e.Choice = &WildlingChoice{House: house, Kind: WildlingDestroyUnit, Options: ids, UnitOptions: units, Count: e.Units.Remaining}
	}
	r.Match.ActiveHouse = house
	r.startClock(house, now)
	return true
}

func (r *Room) chooseMammothRiders(house, option string, c Catalog) error {
	a, e := r.Wildlings.Attack, r.Wildlings.Attack.Effect
	if a.WatchWon && e.Stage == WildlingHandEffect && e.Choice.Kind == WildlingRetrieveCard {
		if !slices.Contains(e.Choice.Options, option) {
			return errors.New("Выберите карту из своего сброса или откажитесь от награды")
		}
		if option != WildlingFinishReward {
			index := slices.Index(r.Match.Discard[house], option)
			_, known := c.Houses[house].Cards[option]
			if index < 0 || !known {
				return errors.New("Выберите карту из своего сброса или откажитесь от награды")
			}
			r.Match.Discard[house] = slices.Delete(r.Match.Discard[house], index, index+1)
			r.event("wildling-retrieve", house, "", "", HouseNames[house]+": одичалые вернули карту — "+c.Houses[house].Cards[option].Name)
		}
		e.Queue = slices.Clone(e.Queue[1:])
		return nil
	}
	return r.chooseWildlingUnitLoss(house, option, c)
}

func (r *Room) chooseWildlingUnitLoss(house, option string, c Catalog) error {
	a, e := r.Wildlings.Attack, r.Wildlings.Attack.Effect
	invalid := errors.New("Выберите доступный отряд для эффекта одичалых")
	if a.WatchWon || e.Stage != WildlingUnitsEffect || e.Choice.Kind != WildlingDestroyUnit || e.Units == nil || e.Units.Remaining < 1 || !slices.Contains(e.Choice.Options, option) {
		return invalid
	}
	units, _ := r.wildlingLossOptions(house)
	if a.Card == WildlingHorde {
		units = r.hordeLossOptions(house, c)
	}
	index := slices.IndexFunc(units, func(unit WildlingUnitOption) bool { return unit.ID == option })
	if index < 0 {
		return invalid
	}
	selected := units[index] // Resolve server-generated ID, never parse a client command.
	if e.Units.CastleOnly && e.Units.Region == "" {
		e.Units.Region = selected.Region
	}
	army := slices.Clone(r.Match.Armies[selected.Region])
	for i := range army {
		if army[i].House == house && army[i].Type == string(selected.Unit) && army[i].Count > 0 {
			army[i].Count--
			break
		}
	}
	r.Match.Armies[selected.Region] = slices.DeleteFunc(army, func(unit Unit) bool { return unit.Count == 0 })
	if count(r.Match.Armies[selected.Region]) == 0 {
		delete(r.Orders, selected.Region)
		// Resolve port cleanup only after ALL chosen casualties of this house.
		// A student may select both the last land unit and its port's ships.
		e.Units.Vacated = append(e.Units.Vacated, selected.Region)
	}
	r.event("wildling-casualty", house, selected.Region, "", HouseNames[house]+": одичалые уничтожили отряд — "+string(selected.Unit))
	e.Units.Remaining--
	if e.Units.Remaining == 0 {
		r.finishWildlingLossHouse(c)
	}
	return nil
}

func (r *Room) finishWildlingLossHouse(c Catalog) {
	e := r.Wildlings.Attack.Effect
	for _, region := range e.Units.Vacated {
		r.restoreEmptyControl(region, c)
	}
	e.Queue, e.Units = slices.Clone(e.Queue[1:]), nil
}

func (r *Room) mammothBotOption(seat *Seat, c Catalog) string {
	choice := r.Wildlings.Attack.Effect.Choice
	if seat.Difficulty == "easy" {
		return choice.Options[randomIndex(len(choice.Options))]
	}
	best, score := "", 0
	if choice.Kind == WildlingRetrieveCard {
		for _, id := range choice.Options {
			if id != WildlingFinishReward && (best == "" || c.Houses[seat.House].Cards[id].Strength > score) {
				best, score = id, c.Houses[seat.House].Cards[id].Strength
			}
		}
	} else {
		for _, unit := range choice.UnitOptions {
			value := 100
			if unit.Unit == UnitKnight || unit.Unit == UnitSiege {
				value = 200
			}
			region := r.definition(unit.Region, c)
			value += region.Castle*4 + region.Supply*2 + region.Crowns
			if count(r.Match.Armies[unit.Region]) == 1 {
				value += 20
			}
			if best == "" || value < score {
				best, score = unit.ID, value
			}
		}
	}
	return best
}
