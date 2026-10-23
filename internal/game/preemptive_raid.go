package game

import (
	"errors"
	"slices"
	"time"
)

type PreemptiveDecision string

const (
	PreemptiveDestroyUnits  PreemptiveDecision = "destroy-units"
	PreemptiveLoseInfluence PreemptiveDecision = "lose-influence"
)

func (r *Room) preemptiveHighestTracks(house string) ([]string, int) {
	options, best := []string{}, len(r.Seats)
	for _, track := range influenceTrackOrder {
		index := slices.Index(r.Match.Tracks[string(track)], house)
		if index < best {
			options, best = []string{}, index
		}
		if index == best {
			options = append(options, string(track))
		}
	}
	return options, best
}

func (r *Room) advancePreemptiveRaid(c Catalog, now time.Time) bool {
	a := r.Wildlings.Attack
	if err := r.validateInfluenceTracks(); err != nil {
		return false
	}
	if a.WatchWon {
		return r.repeatPreemptiveAttack(now)
	}
	if a.Effect == nil {
		a.Effect = &WildlingEffect{Stage: WildlingPenaltyEffect, Queue: []string{a.SelectedHouse}}
	}
	e := a.Effect
	if e.Choice != nil {
		return false
	}
	if len(e.Queue) == 0 {
		return r.finishWildlingAttack()
	}
	house := a.SelectedHouse
	switch e.Stage {
	case WildlingPenaltyEffect:
		e.Choice = &WildlingChoice{House: house, Kind: WildlingChoosePenalty,
			Options: []string{string(PreemptiveDestroyUnits), string(PreemptiveLoseInfluence)}}
	case WildlingTrackEffect:
		options, rank := r.preemptiveHighestTracks(house)
		// A sole highest track needs no tie decision. At the bottom of every
		// track either choice is legal (FAQ); no pointless human choice remains.
		if len(options) == 1 || rank == len(r.Seats)-1 {
			r.reducePreemptiveInfluence(house, options[0])
			e.Queue = nil
			return true
		}
		house = r.Match.Tracks[string(TrackIronThrone)][0]
		e.Choice = &WildlingChoice{House: house, Kind: WildlingChooseTrack, Options: options}
	case WildlingUnitsEffect:
		if e.Units == nil {
			return false
		}
		if e.Units.Remaining == 0 {
			r.finishWildlingLossHouse(c)
			return true
		}
		units, _ := r.wildlingLossOptions(house)
		ids := make([]string, 0, len(units))
		for _, unit := range units {
			ids = append(ids, unit.ID)
		}
		e.Choice = &WildlingChoice{House: house, Kind: WildlingDestroyUnit, Options: ids, UnitOptions: units, Count: e.Units.Remaining}
	default:
		return false
	}
	r.Match.ActiveHouse = house
	r.startClock(house, now)
	return true
}

func (r *Room) choosePreemptiveRaid(option string, c Catalog) error {
	a, e := r.Wildlings.Attack, r.Wildlings.Attack.Effect
	invalid := errors.New("Сейчас нет вашего решения по карте одичалых")
	if a.WatchWon || !slices.Contains(e.Choice.Options, option) {
		return invalid
	}
	switch e.Stage {
	case WildlingPenaltyEffect:
		if e.Choice.Kind != WildlingChoosePenalty {
			return invalid
		}
		switch PreemptiveDecision(option) {
		case PreemptiveDestroyUnits:
			_, total := r.wildlingLossOptions(a.SelectedHouse)
			e.Stage, e.Units = WildlingUnitsEffect, &WildlingUnitEffect{Remaining: min(2, total)}
		case PreemptiveLoseInfluence:
			if err := r.validateInfluenceTracks(); err != nil {
				return err
			}
			e.Stage = WildlingTrackEffect
		default:
			return invalid
		}
	case WildlingTrackEffect:
		if e.Choice.Kind != WildlingChooseTrack {
			return invalid
		}
		if err := r.validateInfluenceTracks(); err != nil {
			return err
		}
		options, _ := r.preemptiveHighestTracks(a.SelectedHouse)
		if !slices.Contains(options, option) {
			return invalid
		}
		r.reducePreemptiveInfluence(a.SelectedHouse, option)
		e.Queue = nil
	case WildlingUnitsEffect:
		return r.chooseWildlingUnitLoss(a.SelectedHouse, option, c)
	default:
		return invalid
	}
	return nil
}

func (r *Room) reducePreemptiveInfluence(house, track string) {
	ranking := slices.Clone(r.Match.Tracks[track])
	index := slices.Index(ranking, house)
	ranking = slices.Delete(ranking, index, index+1)
	ranking = slices.Insert(ranking, min(index+2, len(ranking)), house)
	r.Match.Tracks[track] = ranking
	// Ownership follows the updated tracks; BladeUsed is not reset.
	r.event("wildling-preemptive-track", house, "", "", HouseNames[house]+": одичалые снизили влияние на две позиции — "+influenceTrackName(InfluenceTrack(track)))
}

func (r *Room) repeatPreemptiveAttack(now time.Time) bool {
	a := r.Wildlings.Attack
	participants := slices.DeleteFunc(slices.Clone(r.Match.Tracks[string(TrackIronThrone)]), func(house string) bool {
		return house == a.SelectedHouse
	})
	if len(participants) == 0 {
		return false
	}
	excluded := a.SelectedHouse
	// One durable transition: finish this card, put it on the bottom, clear
	// threat, and start fresh bids. Do NOT resume/advance the Westeros cursor.
	r.Westeros.Threat = 0
	r.recordWildlingResult()
	r.Westeros.Run.Attack = &WesterosAttack{Trigger: WildlingsFromPreemptive, Strength: 6}
	r.event("wildling-complete", "", "", "", "Нападение одичалых завершено")
	r.event("wildling-preemptive-attack", excluded, "", "", HouseNames[excluded]+": освобождён от повторного нападения одичалых силой 6")
	r.startWildlingBidding(6, participants, excluded, now)
	return true
}
