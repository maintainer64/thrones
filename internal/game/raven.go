package game

import (
	"errors"
	"slices"
	"time"
)

const PhaseRaven = "raven"

type RavenStage string
type RavenDecision string

const (
	RavenChoosing RavenStage = "choose"
	RavenPeeking  RavenStage = "peek"

	RavenSkip    RavenDecision = "skip"
	RavenReplace RavenDecision = "replace"
	RavenPeek    RavenDecision = "peek"
	RavenKeep    RavenDecision = "keep"
	RavenBottom  RavenDecision = "bottom"
)

// Private persisted state. Card must never be projected to another participant,
// a public Match, event or error; looking is already an irrevocable use.
type RavenState struct {
	Round int            `json:"round"`
	House string         `json:"house"`
	Stage RavenStage     `json:"stage"`
	Card  WildlingCardID `json:"card,omitempty"`
}

type RavenView struct {
	Round   int                 `json:"round"`
	House   string              `json:"house"`
	Stage   RavenStage          `json:"stage"`
	Card    WildlingCardID      `json:"card,omitempty"`
	Options map[string][]string `json:"options,omitempty"`
}

// The revealed-order Continue command enters this step, never the raids
// directly. No public command can start/skip someone else's Raven decision.
func (r *Room) beginRaven(now time.Time) error {
	if r.Phase != "orders_revealed" || r.Match == nil || r.Raven != nil || r.Match.RavenUsed ||
		r.Battle != nil || r.Influence != nil || r.Supply != nil || r.Mustering != nil ||
		(r.Wildlings != nil && r.Wildlings.Attack != nil) {
		return errors.New("Сейчас нельзя начать шаг Посыльного ворона")
	}
	if err := r.validateInfluenceTracks(); err != nil {
		return err
	}
	house := r.Match.Tracks[string(TrackKingsCourt)][0]
	r.Raven = &RavenState{Round: r.Round, House: house, Stage: RavenChoosing}
	r.Phase, r.Match.ActiveHouse = PhaseRaven, house
	r.startClock(house, now)
	r.event("raven", house, "", "", "Посыльный ворон: решение принимает дом "+HouseNames[house])
	return nil
}

func (r *Room) validRaven() bool {
	if r.Phase != PhaseRaven || r.Match == nil || r.Raven == nil || r.Raven.Round != r.Round {
		return false
	}
	track := r.Match.Tracks[string(TrackKingsCourt)]
	stageValid := (r.Raven.Stage == RavenChoosing && !r.Match.RavenUsed && r.Raven.Card == "") ||
		(r.Raven.Stage == RavenPeeking && r.Match.RavenUsed && r.Raven.Card != "")
	return stageValid && len(track) > 0 && track[0] == r.Raven.House && r.houseSeat(r.Raven.House) != nil &&
		r.Match.ActiveHouse == r.Raven.House && r.Battle == nil && r.Influence == nil &&
		r.Supply == nil && r.Mustering == nil && (r.Wildlings == nil || r.Wildlings.Attack == nil)
}

func (r *Room) validateRavenReplacement(house, region, orderID string, c Catalog) error {
	if r.Orders[region] == "" || orderID == "" || r.Orders[region] == orderID {
		return errors.New("Замените существующий приказ другим неиспользованным жетоном")
	}
	seat := r.houseSeat(house)
	if seat == nil {
		return errors.New("Сейчас нет вашего решения Посыльного ворона")
	}
	return r.validateOrderToken(seat, region, orderID, c)
}

func (r *Room) ravenReplacements(house string, c Catalog) map[string][]string {
	result := map[string][]string{}
	for _, region := range r.regionIDs(house) {
		for _, order := range Orders {
			if r.validateRavenReplacement(house, region, order.ID, c) == nil {
				result[region] = append(result[region], order.ID)
			}
		}
	}
	return result
}

func (r *Room) ravenView(house string, c Catalog) *RavenView {
	if !r.validRaven() {
		return nil
	}
	a := r.Raven
	v := &RavenView{Round: a.Round, House: a.House, Stage: a.Stage}
	if house == a.House {
		if a.Stage == RavenPeeking {
			v.Card = a.Card
		} else if a.Stage == RavenChoosing && !r.Match.RavenUsed {
			v.Options = r.ravenReplacements(house, c)
		}
	}
	return v
}

func (r *Room) chooseRaven(house string, choice RavenDecision, region, order string, c Catalog, now time.Time) error {
	if !r.validRaven() || house != r.Raven.House {
		return errors.New("Сейчас нет вашего решения Посыльного ворона")
	}
	a := r.Raven
	if a.Stage == RavenChoosing && !r.Match.RavenUsed {
		switch choice {
		case RavenSkip:
			r.event("raven-skip", house, "", "", HouseNames[house]+": Посыльный ворон не использован")
			r.finishRaven(c, now)
			return nil
		case RavenReplace:
			if err := r.validateRavenReplacement(house, region, order, c); err != nil {
				return err
			}
			r.Orders[region] = order
			r.Match.RavenUsed = true
			r.event("raven-order", house, region, "", HouseNames[house]+": Посыльный ворон заменил приказ")
			r.finishRaven(c, now)
			return nil
		case RavenPeek:
			if r.Wildlings == nil || !validWildlingDeck(r.Wildlings.Deck) {
				return errors.New("Некорректный состав колоды одичалых")
			}
			a.Card, a.Stage = r.Wildlings.Deck[0], RavenPeeking
			r.Match.RavenUsed = true
			r.startClock(house, now)
			r.event("raven-peek", house, "", "", HouseNames[house]+": Посыльный ворон посмотрел карту одичалых")
			return nil
		}
	} else if a.Stage == RavenPeeking && r.Match.RavenUsed {
		if choice != RavenKeep && choice != RavenBottom {
			return errors.New("После просмотра выберите верх или низ колоды")
		}
		if r.Wildlings == nil || !validWildlingDeck(r.Wildlings.Deck) || r.Wildlings.Deck[0] != a.Card {
			return errors.New("Некорректный состав колоды одичалых")
		}
		if choice == RavenBottom {
			r.Wildlings.Deck = append(slices.Clone(r.Wildlings.Deck[1:]), a.Card)
		}
		// Do not reveal the card, its destination or its rules in public history.
		r.event("raven-finish", house, "", "", HouseNames[house]+": Посыльный ворон завершил решение по колоде")
		r.finishRaven(c, now)
		return nil
	}
	return errors.New("Выберите допустимое действие Посыльного ворона")
}

func (r *Room) finishRaven(c Catalog, now time.Time) {
	r.Raven = nil
	r.beginActions(c, now)
}

func (r *Room) advanceRaven(c Catalog, now time.Time, safe bool) bool {
	if !r.validRaven() {
		return false
	}
	a := r.Raven
	seat := r.houseSeat(a.House)
	if !safe && !seat.Bot {
		return false
	}
	choice, region, order := RavenSkip, "", ""
	if a.Stage == RavenPeeking {
		choice = RavenKeep // Only the privately seen top card is available here.
		if !safe && seat.Difficulty == "easy" && randomIndex(2) == 1 {
			choice = RavenBottom
		}
	} else if !safe {
		choice = RavenPeek
		context := c
		context.Units = r.armies(c)
		best := 0
		options := r.ravenReplacements(a.House, c)
		for _, id := range sortedKeys(options) {
			old, _ := FindOrder(r.Orders[id])
			for _, candidate := range options[id] {
				next, _ := FindOrder(candidate)
				gain := planningScore(next, id, a.House, context, seat.Difficulty) - planningScore(old, id, a.House, context, seat.Difficulty)
				if gain > best {
					best, region, order, choice = gain, id, candidate, RavenReplace
				}
			}
		}
		if seat.Difficulty == "easy" {
			choice = []RavenDecision{RavenSkip, RavenPeek, choice}[randomIndex(3)]
		}
	}
	return r.chooseRaven(a.House, choice, region, order, c, now) == nil
}
