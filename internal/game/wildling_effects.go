package game

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

type WildlingEffectStage string
type WildlingChoiceKind string

const (
	WildlingSupplyEffect    WildlingEffectStage = "supply"
	WildlingTrackEffect     WildlingEffectStage = "tracks"
	WildlingHandEffect      WildlingEffectStage = "hand"
	WildlingUnitsEffect     WildlingEffectStage = "units"
	WildlingMusterEffect    WildlingEffectStage = "muster"
	WildlingPenaltyEffect   WildlingEffectStage = "penalty"
	WildlingChoosePenalty   WildlingChoiceKind  = "preemptive-penalty"
	WildlingChooseCastle    WildlingChoiceKind  = "muster-region"
	WildlingChooseTrack     WildlingChoiceKind  = "influence-track"
	WildlingDiscardCard     WildlingChoiceKind  = "discard-card"
	WildlingUpgradeFootman  WildlingChoiceKind  = "upgrade-footman"
	WildlingDowngradeKnight WildlingChoiceKind  = "downgrade-knight"
	WildlingDestroyKnight   WildlingChoiceKind  = "destroy-knight"
	WildlingDestroyUnit     WildlingChoiceKind  = "destroy-unit"
	WildlingRetrieveCard    WildlingChoiceKind  = "retrieve-card"
	WildlingFinishReward                        = "finish"
)

// The queue is consumed before entering a nested resolver, so a reload cannot
// repeat a track penalty. Lowest bidder first, then everyone else in turn order.
type WildlingEffect struct {
	Stage          WildlingEffectStage `json:"stage"`
	Queue          []string            `json:"queue"`
	Choice         *WildlingChoice     `json:"choice,omitempty"`
	RemainingCards []string            `json:"remainingCards,omitempty"`
	Units          *WildlingUnitEffect `json:"units,omitempty"`
	MusterRegion   string              `json:"musterRegion,omitempty"`
}

type WildlingChoice struct {
	House       string               `json:"house"`
	Kind        WildlingChoiceKind   `json:"kind"`
	Options     []string             `json:"options"`
	Count       int                  `json:"count,omitempty"`
	UnitOptions []WildlingUnitOption `json:"unitOptions,omitempty"`
}

func (a *WildlingAttack) otherHouses() []string {
	others := []string{}
	for _, house := range a.Bids.Participants {
		if house != a.SelectedHouse {
			others = append(others, house)
		}
	}
	return others
}

func (r *Room) advanceWildlingEffects(c Catalog, now time.Time) bool {
	if !r.validWildlingEffects() {
		return false
	}
	a := r.Wildlings.Attack
	switch a.Card {
	case WildlingSilence:
		return r.finishWildlingAttack()
	case WildlingSkinchanger:
		if a.WatchWon {
			r.gainPower(a.SelectedHouse, a.Bids.Values[a.SelectedHouse])
		} else {
			r.Match.Power[a.SelectedHouse] = 0
			for _, house := range a.otherHouses() {
				r.gainPower(house, -2)
			}
		}
		return r.finishWildlingAttack()
	case WildlingRaiders:
		if a.WatchWon {
			r.changeWildlingSupply(a.SelectedHouse, 1)
			return r.finishWildlingAttack()
		}
		if a.Effect == nil {
			a.Effect = &WildlingEffect{Stage: WildlingSupplyEffect, Queue: append([]string{a.SelectedHouse}, a.otherHouses()...)}
		}
		if a.Effect.Stage != WildlingSupplyEffect {
			return false
		}
		if len(a.Effect.Queue) == 0 {
			return r.finishWildlingAttack()
		}
		house := a.Effect.Queue[0]
		a.Effect.Queue = slices.Clone(a.Effect.Queue[1:])
		loss := -1
		if house == a.SelectedHouse {
			loss = -2
		}
		r.changeWildlingSupply(house, loss)
		r.Supply = &SupplyResolution{Queue: []string{house}, Source: SupplyFromWildlings}
		r.advanceSupply(c, now)
		return true
	case WildlingKing:
		if err := r.validateInfluenceTracks(); err != nil {
			return false
		}
		if a.Effect == nil {
			queue := []string{a.SelectedHouse}
			if !a.WatchWon {
				for _, track := range influenceTrackOrder {
					r.moveWildlingInfluence(a.SelectedHouse, track, false)
				}
				queue = a.otherHouses()
			}
			a.Effect = &WildlingEffect{Stage: WildlingTrackEffect, Queue: queue}
		}
		if a.Effect.Stage != WildlingTrackEffect || a.Effect.Choice != nil {
			return false
		}
		if len(a.Effect.Queue) == 0 {
			return r.finishWildlingAttack()
		}
		house := a.Effect.Queue[0]
		options := []string{string(TrackFiefdoms), string(TrackKingsCourt)}
		if a.WatchWon {
			options = append([]string{string(TrackIronThrone)}, options...)
		}
		a.Effect.Choice = &WildlingChoice{House: house, Kind: WildlingChooseTrack, Options: options}
		r.Match.ActiveHouse = house
		r.startClock(house, now)
		return true
	case WildlingMilkwater:
		return r.advanceMilkwater(c, now)
	case WildlingCrowKillers:
		return r.advanceCrowKillers(c, now)
	case WildlingMammoths:
		return r.advanceMammothRiders(c, now)
	case WildlingHorde:
		return r.advanceHorde(c, now)
	case WildlingPreemptive:
		return r.advancePreemptiveRaid(c, now)
	}
	return false
}

func (r *Room) changeWildlingSupply(house string, delta int) {
	r.Match.Supply[house] = max(0, min(6, r.Match.Supply[house]+delta))
	r.event("wildling-supply", house, "", "", fmt.Sprintf("%s: одичалые изменили снабжение — %d", HouseNames[house], r.Match.Supply[house]))
}

func (r *Room) moveWildlingInfluence(house string, track InfluenceTrack, top bool) {
	key := string(track)
	ranking := slices.Clone(r.Match.Tracks[key])
	index := slices.Index(ranking, house)
	ranking = append(ranking[:index], ranking[index+1:]...)
	position := "низ"
	if top {
		ranking = append([]string{house}, ranking...)
		position = "верх"
	} else {
		ranking = append(ranking, house)
	}
	r.Match.Tracks[key] = ranking
	// Dominance ownership derives from the tracks. Never reset BladeUsed (nor
	// RavenUsed) when a token changes hands outside the next round.
	r.event("wildling-influence", house, "", "", HouseNames[house]+": одичалые переместили дом — "+influenceTrackName(track)+", "+position)
}

func (r *Room) chooseWildlingEffect(house, option string, c Catalog) error {
	if !r.validWildlingEffects() {
		return errors.New("Сейчас нет вашего решения по карте одичалых")
	}
	a := r.Wildlings.Attack
	e := a.Effect
	if e == nil || e.Choice == nil || e.Choice.House != house || len(e.Queue) == 0 || r.Match.ActiveHouse != house {
		return errors.New("Сейчас нет вашего решения по карте одичалых")
	}
	actor := e.Queue[0]
	if a.Card == WildlingPreemptive && e.Stage == WildlingTrackEffect {
		if err := r.validateInfluenceTracks(); err != nil {
			return err
		}
		actor = r.Match.Tracks[string(TrackIronThrone)][0]
	}
	if house != actor {
		return errors.New("Сейчас нет вашего решения по карте одичалых")
	}
	switch {
	case a.Card == WildlingPreemptive:
		if err := r.choosePreemptiveRaid(option, c); err != nil {
			return err
		}
	case a.Card == WildlingHorde:
		if err := r.chooseHorde(house, option, c); err != nil {
			return err
		}
	case a.Card == WildlingMammoths:
		if err := r.chooseMammothRiders(house, option, c); err != nil {
			return err
		}
	case a.Card == WildlingCrowKillers && e.Stage == WildlingUnitsEffect:
		if err := r.chooseCrowKillers(house, option, c); err != nil {
			return err
		}
	case a.Card == WildlingKing && e.Stage == WildlingTrackEffect && e.Choice.Kind == WildlingChooseTrack:
		if !slices.Contains(e.Choice.Options, option) || (!a.WatchWon && option == string(TrackIronThrone)) || !slices.Contains(influenceTrackOrder, InfluenceTrack(option)) {
			return errors.New("Выберите доступный трек влияния на карте одичалых")
		}
		if err := r.validateInfluenceTracks(); err != nil {
			return err
		}
		r.moveWildlingInfluence(house, InfluenceTrack(option), a.WatchWon)
		e.Queue = slices.Clone(e.Queue[1:])
	case a.Card == WildlingMilkwater && !a.WatchWon && e.Stage == WildlingHandEffect && e.Choice.Kind == WildlingDiscardCard:
		if !slices.Contains(e.Choice.Options, option) || !slices.Contains(e.RemainingCards, option) || !slices.Contains(r.availableCards(house, c), option) {
			return errors.New("Выберите доступную для сброса карту дома")
		}
		r.discardCard(house, option, c)
		r.event("wildling-discard", house, "", "", HouseNames[house]+": одичалые вынудили сбросить карту — "+c.Houses[house].Cards[option].Name)
		index := slices.Index(e.RemainingCards, option)
		e.RemainingCards = slices.Delete(e.RemainingCards, index, index+1)
		if house != a.SelectedHouse || len(e.RemainingCards) == 0 {
			e.Queue, e.RemainingCards = slices.Clone(e.Queue[1:]), nil
		}
	default:
		return errors.New("Сейчас нет вашего решения по карте одичалых")
	}
	e.Choice = nil
	r.Match.ActiveHouse, r.Clock = "", Clock{}
	return nil
}

func (r *Room) advanceMilkwater(c Catalog, now time.Time) bool {
	a := r.Wildlings.Attack
	if a.WatchWon {
		returned := len(r.Match.Discard[a.SelectedHouse])
		r.Match.Discard[a.SelectedHouse] = []string{}
		r.event("wildling-hand", a.SelectedHouse, "", "", fmt.Sprintf("%s: одичалые вернули карт в руку — %d", HouseNames[a.SelectedHouse], returned))
		return r.finishWildlingAttack()
	}
	if a.Effect == nil {
		a.Effect = &WildlingEffect{Stage: WildlingHandEffect, Queue: append([]string{a.SelectedHouse}, a.otherHouses()...)}
	}
	e := a.Effect
	if e.Stage != WildlingHandEffect || e.Choice != nil {
		return false
	}
	if len(e.Queue) == 0 {
		return r.finishWildlingAttack()
	}
	house := e.Queue[0]
	if len(e.RemainingCards) == 0 {
		hand := r.availableCards(house, c)
		if len(hand) <= 1 {
			e.Queue = slices.Clone(e.Queue[1:])
			return true
		}
		e.RemainingCards = hand
		if house == a.SelectedHouse {
			strength := -1
			for _, id := range hand {
				strength = max(strength, c.Houses[house].Cards[id].Strength)
			}
			e.RemainingCards = slices.DeleteFunc(hand, func(id string) bool { return c.Houses[house].Cards[id].Strength != strength })
		}
	}
	// Snapshot the ORIGINAL highest-strength set. If discarding the last card
	// restores the hand, returned cards are not new penalty targets (FAQ p. 4).
	e.Choice = &WildlingChoice{House: house, Kind: WildlingDiscardCard, Options: slices.Clone(e.RemainingCards)}
	r.Match.ActiveHouse = house
	r.startClock(house, now)
	return true
}

func (r *Room) wildlingBotOption(seat *Seat, c Catalog) string {
	choice := r.Wildlings.Attack.Effect.Choice
	if len(choice.Options) == 0 {
		return ""
	}
	if choice.Kind == WildlingChooseTrack {
		return r.wildlingBotTrack(seat)
	}
	if choice.Kind == WildlingChoosePenalty {
		if seat.Difficulty == "easy" {
			return choice.Options[randomIndex(len(choice.Options))]
		}
		_, total := r.wildlingLossOptions(seat.House)
		if total < 2 {
			return string(PreemptiveDestroyUnits)
		}
		// Prefer losing influence to destroying two pieces; only public
		// tracks and this house's army are consulted.
		return string(PreemptiveLoseInfluence)
	}
	if choice.Kind == WildlingChooseCastle {
		if seat.Difficulty == "easy" {
			return choice.Options[randomIndex(len(choice.Options))]
		}
		best, points := WildlingFinishReward, -1
		for _, id := range choice.Options {
			if id != WildlingFinishReward && r.definition(id, c).Castle > points {
				best, points = id, r.definition(id, c).Castle
			}
		}
		return best
	}
	if choice.Kind == WildlingRetrieveCard || choice.Kind == WildlingDestroyUnit {
		return r.mammothBotOption(seat, c)
	}
	if choice.Kind == WildlingUpgradeFootman || choice.Kind == WildlingDowngradeKnight || choice.Kind == WildlingDestroyKnight {
		return r.crowKillersBotOption(seat, c)
	}
	if choice.Kind != WildlingDiscardCard {
		return ""
	}
	if seat.Difficulty == "easy" {
		return choice.Options[randomIndex(len(choice.Options))]
	}
	best := choice.Options[0]
	for _, id := range choice.Options[1:] {
		if c.Houses[seat.House].Cards[id].Strength < c.Houses[seat.House].Cards[best].Strength {
			best = id
		}
	}
	return best
}

func (r *Room) wildlingBotTrack(seat *Seat) string {
	a := r.Wildlings.Attack
	options := a.Effect.Choice.Options
	if seat.Difficulty == "easy" {
		return options[randomIndex(len(options))]
	}
	// Improve the weakest ranking on a reward; sacrifice the weakest ranking on
	// a penalty too. Only public tracks are consulted, never another hidden bid.
	best, rank := options[0], -1
	for _, option := range options {
		if index := slices.Index(r.Match.Tracks[option], seat.House); index > rank {
			best, rank = option, index
		}
	}
	return best
}
