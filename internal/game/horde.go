package game

import (
	"errors"
	"slices"
	"time"
)

// The lowest bidder must lose both pieces in ONE castle/stronghold if
// possible. Persist that region after the first casualty: recomputing the
// >=2 condition after a reload would incorrectly allow the second elsewhere.
func (r *Room) hordeLossOptions(house string, c Catalog) []WildlingUnitOption {
	options, _ := r.wildlingLossOptions(house)
	u := r.Wildlings.Attack.Effect.Units
	if u.CastleOnly {
		options = slices.DeleteFunc(options, func(o WildlingUnitOption) bool {
			if u.Region != "" {
				return o.Region != u.Region
			}
			return r.definition(o.Region, c).Castle == 0 || count(r.Match.Armies[o.Region]) < 2
		})
	}
	return options
}

func (r *Room) advanceHorde(c Catalog, now time.Time) bool {
	a := r.Wildlings.Attack
	stage := WildlingUnitsEffect
	if a.WatchWon {
		stage = WildlingMusterEffect
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
	if e.MusterRegion != "" {
		region := e.MusterRegion
		e.MusterRegion = ""
		r.Mustering = &Mustering{Queue: []string{a.SelectedHouse}, Source: MusterFromWildlings, Region: region}
		r.advanceMuster(c, now)
		return true
	}
	if len(e.Queue) == 0 {
		return r.finishWildlingAttack()
	}
	house := e.Queue[0]
	if a.WatchWon {
		options := []string{}
		for _, region := range c.Boards["westeros"].Regions {
			if region.Castle > 0 && r.owner(region.ID) == house {
				options = append(options, region.ID)
			}
		}
		if len(options) == 0 {
			e.Queue = nil
			return true
		}
		slices.Sort(options)
		e.Choice = &WildlingChoice{House: house, Kind: WildlingChooseCastle, Options: append(options, WildlingFinishReward)}
	} else {
		units, total := r.wildlingLossOptions(house)
		if e.Units == nil {
			quota := 1
			castleOnly := false
			if house == a.SelectedHouse {
				quota = 2
				for _, unit := range units {
					if r.definition(unit.Region, c).Castle > 0 && count(r.Match.Armies[unit.Region]) >= 2 {
						castleOnly = true
					}
				}
			}
			e.Units = &WildlingUnitEffect{Remaining: min(quota, total), CastleOnly: castleOnly}
		}
		if e.Units.Remaining == 0 {
			r.finishWildlingLossHouse(c)
			return true
		}
		units = r.hordeLossOptions(house, c)
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

func (r *Room) chooseHorde(house, option string, c Catalog) error {
	a, e := r.Wildlings.Attack, r.Wildlings.Attack.Effect
	if !a.WatchWon {
		return r.chooseWildlingUnitLoss(house, option, c)
	}
	if e.Stage != WildlingMusterEffect || e.Choice.Kind != WildlingChooseCastle || !slices.Contains(e.Choice.Options, option) {
		return errors.New("Сейчас нет вашего решения по карте одичалых")
	}
	if option != WildlingFinishReward {
		region := r.definition(option, c)
		if region.Castle == 0 || r.owner(option) != house {
			return errors.New("Сейчас нет вашего решения по карте одичалых")
		}
		// Consume the reward before entering the shared resolver. It resumes the
		// same attack, not Westeros or consolidate power, after finish/reload.
		e.MusterRegion = option
	}
	e.Queue = nil
	return nil
}
