package game

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

type WesterosStage string

const (
	WesterosThreatStep WesterosStage = "threat"
	WesterosEventsStep WesterosStage = "events"
	WesterosChoiceStep WesterosStage = "choice"
	WesterosAttackStep WesterosStage = "wildlings"
	WesterosFinished   WesterosStage = "complete"
)

type WesterosDecision string

const (
	WesterosChooseNothing  WesterosDecision = "nothing"
	WesterosChooseSupply   WesterosDecision = "supply"
	WesterosChooseMuster   WesterosDecision = "mustering"
	WesterosChooseClash    WesterosDecision = "clash-of-kings"
	WesterosChoosePower    WesterosDecision = "game-of-thrones"
	WesterosChooseNoDefend WesterosDecision = "no-defense"
	WesterosChooseNoMarch  WesterosDecision = "no-march-plus-one"
)

type WesterosRestriction string

const (
	WesterosNoRestriction WesterosRestriction = ""
	WesterosNoRaid        WesterosRestriction = "no-raid"
	WesterosNoSupport     WesterosRestriction = "no-support"
	WesterosNoPower       WesterosRestriction = "no-power"
	WesterosNoDefense     WesterosRestriction = "no-defense"
	WesterosNoMarchPlus   WesterosRestriction = "no-march-plus-one"
)

type WildlingTrigger string

const (
	WildlingsFromThreat     WildlingTrigger = "threat"
	WildlingsFromCard       WildlingTrigger = "card"
	WildlingsFromPreemptive WildlingTrigger = "preemptive-raid"
)

// All fields here are public: no bid amounts or future wildling card belong in
// this continuation. The attack handoff is NOT a completed wildling attack.
type WesterosRun struct {
	Round       int                 `json:"round"`
	Stage       WesterosStage       `json:"stage"`
	NextCard    int                 `json:"nextCard"`
	Restriction WesterosRestriction `json:"restriction"`
	Choice      *WesterosChoice     `json:"choice,omitempty"`
	Attack      *WesterosAttack     `json:"attack,omitempty"`
}

type WesterosChoice struct {
	House   string             `json:"house"`
	Card    WesterosCardID     `json:"card"`
	Options []WesterosDecision `json:"options"`
}

type WesterosAttack struct {
	Trigger  WildlingTrigger `json:"trigger"`
	Strength int             `json:"strength"`
}

func copyWesterosRun(run *WesterosRun) *WesterosRun {
	if run == nil {
		return nil
	}
	copy := *run
	if run.Choice != nil {
		choice := *run.Choice
		choice.Options = slices.Clone(choice.Options)
		copy.Choice = &choice
	}
	if run.Attack != nil {
		attack := *run.Attack
		copy.Attack = &attack
	}
	return &copy
}

// BeginWesteros is the internal round-boundary hook for rounds 2–10. Events,
// wildling outcomes and their UI decisions must complete before planning.
// Each AdvanceBot tick commits at most one event/automatic step; nested rule
// decisions return to PhaseWesteros and resume the saved cursor on the next tick.
func (r *Room) BeginWesteros(now time.Time) error {
	if !r.canStartProvision() || r.Round < 2 || r.Round > 10 {
		return errors.New("Карты Вестероса открываются только в фазе Вестероса раундов 2–10")
	}
	if r.Westeros != nil && r.Westeros.Run != nil {
		run := r.Westeros.Run
		if run.Round == r.Round {
			return nil
		}
		if run.Stage != WesterosFinished {
			return errors.New("Предыдущие события Вестероса ещё не завершены")
		}
	}
	if err := r.validateInfluenceTracks(); err != nil {
		return err
	}
	if r.Westeros != nil && (r.Westeros.Threat < 0 || r.Westeros.Threat > 12 || r.Westeros.Threat%2 != 0) {
		return errors.New("Некорректная угроза одичалых")
	}
	if err := r.RevealWesterosCards(); err != nil {
		return err
	}
	r.Westeros.Run = &WesterosRun{Round: r.Round, Stage: WesterosThreatStep}
	r.Orders = map[string]string{}
	r.Match.ActiveHouse, r.Clock = "", Clock{}
	r.Match.BladeUsed = false
	r.Match.RavenUsed = false
	for i := range r.Seats {
		r.Seats[i].Ready = false
	}
	r.event("westeros", "", "", "", fmt.Sprintf("Вестерос: раскрыты карты раунда %d", r.Round))
	return nil
}

// The printed track is 0,2,...,12: advancing one space means +2 strength.
func westerosWildlingIcon(card WesterosCardID) bool {
	switch card {
	case WesterosLastDays, WesterosThroneOfBlades, WesterosDarkWings,
		WesterosStormOfSwords, WesterosRainsOfAutumn, WesterosSeaOfStorms,
		WesterosWebOfLies, WesterosFeastForCrows:
		return true
	}
	return false
}

func (r *Room) advanceWesteros(c Catalog, now time.Time) bool {
	if !r.canStartProvision() || r.Westeros == nil || r.Westeros.Current == nil || r.Westeros.Run == nil {
		return false
	}
	s, run := r.Westeros, r.Westeros.Run
	if run.Round != r.Round || s.Current.Round != r.Round {
		return false
	}
	switch run.Stage {
	case WesterosThreatStep:
		icons := 0
		for _, card := range s.Current.Cards {
			if westerosWildlingIcon(card) {
				icons++
			}
		}
		s.Threat = min(12, s.Threat+2*icons)
		run.Stage = WesterosEventsStep
		r.event("wildling-threat", "", "", "", fmt.Sprintf("Угроза одичалых: %d", s.Threat))
		if s.Threat == 12 {
			r.pauseForWildlings(WildlingsFromThreat)
		}
		return true
	case WesterosEventsStep:
		if run.NextCard == 3 {
			run.Stage = WesterosFinished
			r.beginPlanningRound(c, now)
			return true
		}
		if run.NextCard < 0 || run.NextCard > 3 {
			return false
		}
		card := s.Current.Cards[run.NextCard]
		if err := r.resolveWesterosCard(card, c, now); err != nil {
			return false
		}
		run.NextCard++
		r.event("westeros-event", "", "", "", "Событие Вестероса: "+string(card))
		return true
	}
	return false
}

func (r *Room) resolveWesterosCard(card WesterosCardID, c Catalog, now time.Time) error {
	run := r.Westeros.Run
	switch card {
	case WesterosSupply:
		return r.BeginSupply(c, now)
	case WesterosMustering:
		return r.BeginMustering(c, now)
	case WesterosClashOfKings:
		return r.BeginClashOfKings(now)
	case WesterosGameOfThrones:
		r.westerosGainPower(c)
	case WesterosLastDays:
		// No effect, but its wildling icon was counted before all three events.
	case WesterosThroneOfBlades:
		r.offerWesterosChoice(card, TrackIronThrone, []WesterosDecision{WesterosChooseSupply, WesterosChooseMuster, WesterosChooseNothing}, now)
	case WesterosDarkWings:
		r.offerWesterosChoice(card, TrackKingsCourt, []WesterosDecision{WesterosChooseClash, WesterosChoosePower, WesterosChooseNothing}, now)
	case WesterosPutToSword:
		r.offerWesterosChoice(card, TrackFiefdoms, []WesterosDecision{WesterosChooseNoDefend, WesterosChooseNoMarch, WesterosChooseNothing}, now)
	case WesterosSeaOfStorms:
		run.Restriction = WesterosNoRaid
	case WesterosWebOfLies:
		run.Restriction = WesterosNoSupport
	case WesterosFeastForCrows:
		run.Restriction = WesterosNoPower
	case WesterosStormOfSwords:
		run.Restriction = WesterosNoDefense
	case WesterosRainsOfAutumn:
		run.Restriction = WesterosNoMarchPlus
	case WesterosWildlings:
		r.pauseForWildlings(WildlingsFromCard)
	default:
		return errors.New("Неизвестное событие Вестероса")
	}
	return nil
}

func (r *Room) offerWesterosChoice(card WesterosCardID, track InfluenceTrack, options []WesterosDecision, now time.Time) {
	holder := r.Match.Tracks[string(track)][0]
	run := r.Westeros.Run
	run.Stage, run.Choice = WesterosChoiceStep, &WesterosChoice{House: holder, Card: card, Options: options}
	r.Match.ActiveHouse = holder
	r.startClock(holder, now)
}

func (r *Room) chooseWesteros(house string, choice WesterosDecision, c Catalog, now time.Time) error {
	if r.Phase != PhaseWesteros || r.Westeros == nil || r.Westeros.Run == nil || r.Westeros.Run.Round != r.Round {
		return errors.New("Сейчас нет решения по событию Вестероса")
	}
	run := r.Westeros.Run
	if run.Stage != WesterosChoiceStep || run.Choice == nil || run.Choice.House != house {
		return errors.New("Решение принимает владелец соответствующего жетона")
	}
	if !slices.Contains(run.Choice.Options, choice) {
		return errors.New("Выберите допустимый эффект события Вестероса")
	}
	// Validate/apply the nested rule before clearing the saved choice. In case
	// of a rejected command the authoritative cursor and deadline stay intact.
	switch choice {
	case WesterosChooseSupply:
		if err := r.BeginSupply(c, now); err != nil {
			return err
		}
	case WesterosChooseMuster:
		if err := r.BeginMustering(c, now); err != nil {
			return err
		}
	case WesterosChooseClash:
		if err := r.BeginClashOfKings(now); err != nil {
			return err
		}
	case WesterosChoosePower:
		r.westerosGainPower(c)
	case WesterosChooseNoDefend:
		run.Restriction = WesterosNoDefense
	case WesterosChooseNoMarch:
		run.Restriction = WesterosNoMarchPlus
	}
	r.event("westeros-choice", house, "", "", HouseNames[house]+": выбран эффект Вестероса — "+string(choice))
	run.Stage, run.Choice = WesterosEventsStep, nil
	if r.Phase == PhaseWesteros {
		r.Match.ActiveHouse, r.Clock = "", Clock{}
	}
	return nil
}

func (r *Room) westerosGainPower(c Catalog) {
	for _, seat := range r.Seats {
		gain := 0
		for _, region := range c.Boards["westeros"].Regions {
			if region.Type == "land" && r.owner(region.ID) == seat.House {
				gain += region.Crowns
			}
			if region.Type != "port" {
				continue
			}
			ships := r.Match.Armies[region.ID]
			if count(ships) == 0 || ships[0].House != seat.House {
				continue
			}
			eligible := true
			for _, neighbor := range region.Neighbors {
				if r.definition(neighbor, c).Type == "sea" {
					enemy := r.Match.Armies[neighbor]
					if count(enemy) > 0 && enemy[0].House != seat.House {
						eligible = false
					}
				}
			}
			if eligible {
				gain++
			}
		}
		before := r.Match.Power[seat.House]
		r.gainPower(seat.House, gain)
		r.event("westeros-power", seat.House, "", "", fmt.Sprintf("%s: получено власти за Вестерос — %d", HouseNames[seat.House], r.Match.Power[seat.House]-before))
	}
}

func (r *Room) pauseForWildlings(trigger WildlingTrigger) {
	run := r.Westeros.Run
	run.Stage = WesterosAttackStep
	run.Attack = &WesterosAttack{Trigger: trigger, Strength: r.Westeros.Threat}
	r.Match.ActiveHouse, r.Clock = "", Clock{}
	r.event("wildlings-pending", "", "", "", "Ожидается разрешение нападения одичалых")
}

// Called ONLY by the D resolver after bids, tie decisions and rewards /
// penalties have really completed. Never expose a host 'skip attack' command.
func (r *Room) resumeWesterosAfterWildlings(watchWon bool) error {
	if !r.canStartProvision() || r.Westeros == nil || r.Westeros.Run == nil {
		return errors.New("Нет незавершённого нападения одичалых")
	}
	run := r.Westeros.Run
	if run.Stage != WesterosAttackStep || run.Attack == nil || run.Round != r.Round {
		return errors.New("Нет незавершённого нападения одичалых")
	}
	if watchWon {
		r.Westeros.Threat = 0
	} else {
		r.Westeros.Threat = max(0, r.Westeros.Threat-4)
	}
	run.Stage, run.Attack = WesterosEventsStep, nil
	return nil
}

func (r *Room) westerosOrderRestricted(order Order) bool {
	if r.Westeros == nil || r.Westeros.Run == nil || r.Westeros.Run.Round != r.Round {
		return false
	}
	switch r.Westeros.Run.Restriction {
	case WesterosNoRaid:
		return order.Kind == "raid"
	case WesterosNoSupport:
		return order.Kind == "support"
	case WesterosNoPower:
		return order.Kind == "power"
	case WesterosNoDefense:
		return order.Kind == "defend"
	case WesterosNoMarchPlus:
		return order.ID == "march-star"
	}
	return false
}

func (r *Room) advanceWesterosChoice(c Catalog, now time.Time, safe bool) bool {
	if r.Westeros == nil || r.Westeros.Run == nil || r.Westeros.Run.Stage != WesterosChoiceStep || r.Westeros.Run.Choice == nil {
		return false
	}
	choice := r.Westeros.Run.Choice
	seat := r.houseSeat(choice.House)
	if seat == nil || (!safe && !seat.Bot) {
		return false
	}
	decision := WesterosChooseNothing
	if !safe {
		if seat.Difficulty == "easy" {
			decision = choice.Options[randomIndex(len(choice.Options))]
		} else {
			// Only public board/tracks and our own reserves inform this policy.
			switch choice.Card {
			case WesterosThroneOfBlades:
				decision = WesterosChooseMuster
			case WesterosDarkWings:
				decision = WesterosChoosePower
			case WesterosPutToSword:
				decision = WesterosChooseNoDefend
			}
		}
	}
	return r.chooseWesteros(choice.House, decision, c, now) == nil
}
