package game

import (
	"errors"
	"fmt"
	"slices"
)

type WesterosCardID string

const (
	WesterosSupply         WesterosCardID = "supply"
	WesterosMustering      WesterosCardID = "mustering"
	WesterosThroneOfBlades WesterosCardID = "a-throne-of-blades"
	WesterosLastDays       WesterosCardID = "last-days-of-summer"
	WesterosWinter         WesterosCardID = "winter-is-coming"
	WesterosGameOfThrones  WesterosCardID = "game-of-thrones"
	WesterosClashOfKings   WesterosCardID = "clash-of-kings"
	WesterosDarkWings      WesterosCardID = "dark-wings-dark-words"
	WesterosPutToSword     WesterosCardID = "put-to-the-sword"
	WesterosStormOfSwords  WesterosCardID = "storm-of-swords"
	WesterosRainsOfAutumn  WesterosCardID = "rains-of-autumn"
	WesterosSeaOfStorms    WesterosCardID = "sea-of-storms"
	WesterosWebOfLies      WesterosCardID = "web-of-lies"
	WesterosFeastForCrows  WesterosCardID = "feast-for-crows"
	WesterosWildlings      WesterosCardID = "wildlings-attack"
)

// Physical copies, not equally weighted event types. Deck III has no Winter.
// Return fresh slices so neither shuffling nor a room can mutate the catalogue.
func baseWesterosDecks() [3][]WesterosCardID {
	return [3][]WesterosCardID{
		{WesterosSupply, WesterosSupply, WesterosSupply,
			WesterosMustering, WesterosMustering, WesterosMustering,
			WesterosThroneOfBlades, WesterosThroneOfBlades, WesterosLastDays, WesterosWinter},
		{WesterosGameOfThrones, WesterosGameOfThrones, WesterosGameOfThrones,
			WesterosClashOfKings, WesterosClashOfKings, WesterosClashOfKings,
			WesterosDarkWings, WesterosDarkWings, WesterosLastDays, WesterosWinter},
		{WesterosWildlings, WesterosWildlings, WesterosWildlings,
			WesterosPutToSword, WesterosPutToSword, WesterosStormOfSwords,
			WesterosRainsOfAutumn, WesterosSeaOfStorms, WesterosWebOfLies, WesterosFeastForCrows},
	}
}

// WesterosState is private persisted Room state, never embedded in public Match.
// Draw[0] is the next card. Discard includes the most recently revealed card.
type WesterosState struct {
	Decks   [3]WesterosDeck `json:"decks"`
	Current *WesterosReveal `json:"current,omitempty"`
	Threat  int             `json:"threat"`
	Run     *WesterosRun    `json:"run,omitempty"`
}

type WesterosDeck struct {
	Draw    []WesterosCardID `json:"draw"`
	Discard []WesterosCardID `json:"discard"`
}

// Winter is handled immediately, including another Winter drawn after shuffling.
// Cards holds the final three events, not the intervening Winter cards. Their
// effects and wildling threat are resolved later, by the Westeros phase runner.
type WesterosReveal struct {
	Round       int               `json:"round"`
	Cards       [3]WesterosCardID `json:"cards"`
	WinterDraws [3]int            `json:"winterDraws"`
}

type WesterosDeckView struct {
	Remaining int              `json:"remaining"`
	Discard   []WesterosCardID `json:"discard"`
}

type WesterosView struct {
	Decks   [3]WesterosDeckView `json:"decks"`
	Current *WesterosReveal     `json:"current,omitempty"`
	Threat  int                 `json:"threat"`
	Run     *WesterosRun        `json:"run,omitempty"`
}

func newWesterosState() *WesterosState {
	// Base rules, setup step 2: the marker starts at strength 2, not zero.
	state := &WesterosState{Threat: 2}
	for i, cards := range baseWesterosDecks() {
		shuffleWesteros(cards, randomIndex)
		state.Decks[i] = WesterosDeck{Draw: cards, Discard: []WesterosCardID{}}
	}
	return state
}

func shuffleWesteros(cards []WesterosCardID, random func(int) int) {
	for i := len(cards) - 1; i > 0; i-- {
		j := random(i + 1)
		cards[i], cards[j] = cards[j], cards[i]
	}
}

// RevealWesterosCards is a rule-effect hook, NOT a player/host command.
// BeginWesteros calls it once on entry; revealing alone does not apply effects.
func (r *Room) RevealWesterosCards() error {
	if !r.canStartProvision() || r.Round < 2 || r.Round > 10 {
		return errors.New("Карты Вестероса открываются только в фазе Вестероса раундов 2–10")
	}
	state := r.Westeros
	if state == nil {
		// Saves made before decks existed acquire them only at a valid boundary.
		state = newWesterosState()
	}
	if err := state.reveal(r.Round, randomIndex); err != nil {
		return err
	}
	r.Westeros = state
	return nil
}

func (s *WesterosState) reveal(round int, random func(int) int) error {
	if s.Current != nil {
		if s.Current.Round == round {
			return nil // Re-entering/reloading the same phase cannot reroll events.
		}
		if s.Current.Round > round {
			return errors.New("Нельзя повторно открывать карты прошедшего раунда")
		}
	}
	// Validate ALL decks first: a damaged snapshot must not partially consume one.
	for i, expected := range baseWesterosDecks() {
		cards := append(slices.Clone(s.Decks[i].Draw), s.Decks[i].Discard...)
		if len(cards) != len(expected) {
			return fmt.Errorf("Некорректный состав колоды Вестероса %d", i+1)
		}
		slices.Sort(cards)
		slices.Sort(expected)
		if !slices.Equal(cards, expected) {
			return fmt.Errorf("Некорректный состав колоды Вестероса %d", i+1)
		}
	}
	result := &WesterosReveal{Round: round}
	for i := range s.Decks {
		result.Cards[i], result.WinterDraws[i] = s.Decks[i].draw(random)
	}
	s.Current = result
	return nil
}

func (d *WesterosDeck) draw(random func(int) int) (WesterosCardID, int) {
	winters := 0
	for {
		if len(d.Draw) == 0 {
			d.reshuffle(random)
		}
		card := d.Draw[0]
		d.Draw = slices.Clone(d.Draw[1:])
		d.Discard = append(d.Discard, card)
		if card != WesterosWinter {
			return card, winters
		}
		winters++
		// Include the Winter just drawn, all previous discards AND the draw pile.
		d.reshuffle(random)
	}
}

func (d *WesterosDeck) reshuffle(random func(int) int) {
	d.Draw = append(slices.Clone(d.Draw), d.Discard...)
	d.Discard = []WesterosCardID{}
	shuffleWesteros(d.Draw, random)
}

func (s *WesterosState) view() *WesterosView {
	if s == nil {
		return nil
	}
	v := &WesterosView{Threat: s.Threat, Run: copyWesterosRun(s.Run)}
	for i, deck := range s.Decks {
		v.Decks[i] = WesterosDeckView{Remaining: len(deck.Draw), Discard: append([]WesterosCardID{}, deck.Discard...)}
	}
	if s.Current != nil {
		current := *s.Current
		v.Current = &current
	}
	return v
}
