package game

import (
	"errors"
	"slices"
	"time"
)

// Remaining counts the house's original penalty/reward, not its changing army.
// Destroy is resolved first; the rest of Remaining must be replaced 1:1.
type WildlingUnitEffect struct {
	Remaining  int      `json:"remaining"`
	Destroy    int      `json:"destroy"`
	Vacated    []string `json:"vacated,omitempty"`
	CastleOnly bool     `json:"castleOnly,omitempty"`
	Region     string   `json:"region,omitempty"`
}

func (r *Room) wildlingUnitRegions(house string, typ UnitType, c Catalog) ([]string, int) {
	regions, total := []string{}, 0
	for _, id := range r.regionIDs(house) {
		if r.definition(id, c).Type != "land" {
			continue
		}
		found := 0
		for _, unit := range r.Match.Armies[id] {
			if unit.House == house && unit.Type == string(typ) {
				found += unit.Count
			}
		}
		if found > 0 {
			regions, total = append(regions, id), total+found
		}
	}
	return regions, total
}

func (r *Room) advanceCrowKillers(c Catalog, now time.Time) bool {
	a := r.Wildlings.Attack
	if a.Effect == nil {
		queue := []string{a.SelectedHouse}
		if !a.WatchWon {
			queue = append(queue, a.otherHouses()...)
		}
		a.Effect = &WildlingEffect{Stage: WildlingUnitsEffect, Queue: queue}
	}
	e := a.Effect
	if e.Stage != WildlingUnitsEffect || e.Choice != nil {
		return false
	}
	if len(e.Queue) == 0 {
		return r.finishWildlingAttack()
	}
	house := e.Queue[0]
	if e.Units == nil {
		_, knights := r.wildlingUnitRegions(house, UnitKnight, c)
		quota := knights
		if house != a.SelectedHouse {
			quota = min(2, quota)
		}
		destroy := max(0, quota-max(0, r.reserve(house, c)[string(UnitFootman)]))
		if a.WatchWon {
			_, footmen := r.wildlingUnitRegions(house, UnitFootman, c)
			quota, destroy = min(2, footmen, max(0, r.reserve(house, c)[string(UnitKnight)])), 0
		}
		e.Units = &WildlingUnitEffect{Remaining: quota, Destroy: destroy}
	}
	if e.Units.Remaining == 0 {
		e.Queue, e.Units = slices.Clone(e.Queue[1:]), nil
		return true
	}
	kind, typ, amount := WildlingDowngradeKnight, UnitKnight, e.Units.Remaining
	if a.WatchWon {
		kind, typ = WildlingUpgradeFootman, UnitFootman
	} else if e.Units.Destroy > 0 {
		kind, amount = WildlingDestroyKnight, e.Units.Destroy
	}
	options, _ := r.wildlingUnitRegions(house, typ, c)
	if a.WatchWon {
		options = append(options, WildlingFinishReward)
	}
	e.Choice = &WildlingChoice{House: house, Kind: kind, Options: options, Count: amount}
	r.Match.ActiveHouse = house
	r.startClock(house, now)
	return true
}

func (r *Room) chooseCrowKillers(house, option string, c Catalog) error {
	a, e := r.Wildlings.Attack, r.Wildlings.Attack.Effect
	invalid := errors.New("Выберите доступный отряд для эффекта одичалых")
	if e.Units == nil || e.Units.Remaining < 1 || !slices.Contains(e.Choice.Options, option) {
		return invalid
	}
	if a.WatchWon && e.Choice.Kind == WildlingUpgradeFootman && option == WildlingFinishReward {
		e.Queue, e.Units = slices.Clone(e.Queue[1:]), nil
		r.event("wildling-upgrade-finished", house, "", "", HouseNames[house]+": улучшения за одичалых завершены")
		return nil
	}
	from, to := UnitKnight, UnitFootman
	switch {
	case a.WatchWon && e.Choice.Kind == WildlingUpgradeFootman:
		from, to = UnitFootman, UnitKnight
	case !a.WatchWon && e.Units.Destroy > 0 && e.Choice.Kind == WildlingDestroyKnight:
		to = ""
	case !a.WatchWon && e.Units.Destroy == 0 && e.Choice.Kind == WildlingDowngradeKnight:
	default:
		return invalid
	}
	regions, _ := r.wildlingUnitRegions(house, from, c)
	if !slices.Contains(regions, option) || (to != "" && r.reserve(house, c)[string(to)] < 1) {
		return invalid
	}
	army := slices.Clone(r.Match.Armies[option])
	for i := range army {
		if army[i].House == house && army[i].Type == string(from) && army[i].Count > 0 {
			army[i].Count--
			break
		}
	}
	army = slices.DeleteFunc(army, func(unit Unit) bool { return unit.Count == 0 })
	text := "рыцарь заменён пехотинцем"
	if to != "" {
		army = merge(army, []Unit{{Type: string(to), House: house, Count: 1}})
	} else {
		text = "рыцарь уничтожен из-за нехватки пехоты"
		e.Units.Destroy--
	}
	if a.WatchWon {
		text = "пехотинец заменён рыцарем"
	}
	r.Match.Armies[option] = army
	if len(army) == 0 {
		delete(r.Orders, option)
		r.restoreEmptyControl(option, c)
	}
	r.event("wildling-unit", house, option, "", HouseNames[house]+": одичалые — "+text)
	e.Units.Remaining--
	if e.Units.Remaining == 0 {
		e.Queue, e.Units = slices.Clone(e.Queue[1:]), nil
	}
	return nil
}

func (r *Room) crowKillersBotOption(seat *Seat, c Catalog) string {
	choice := r.Wildlings.Attack.Effect.Choice
	if seat.Difficulty == "easy" {
		return choice.Options[randomIndex(len(choice.Options))]
	}
	best, bestScore := "", 0
	for _, id := range choice.Options {
		if id == WildlingFinishReward {
			continue
		}
		region := r.definition(id, c)
		score := region.Castle*4 + region.Supply*2 + region.Crowns
		if choice.Kind == WildlingDestroyKnight && count(r.Match.Armies[id]) == 1 {
			score += 10 // Avoid abandoning valuable control when another unit can go.
		}
		if best == "" || (choice.Kind == WildlingUpgradeFootman && score > bestScore) || (choice.Kind != WildlingUpgradeFootman && score < bestScore) {
			best, bestScore = id, score
		}
	}
	if best == "" {
		return WildlingFinishReward
	}
	return best
}
