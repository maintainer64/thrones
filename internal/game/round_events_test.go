package game

import (
	"slices"
	"testing"
)

func TestNormalRoundBoundaryResolvesEveryWildlingCardBeforePlanning(t *testing.T) {
	for _, card := range baseWildlingCards() {
		for _, rich := range []bool{false, true} {
			r, c, now := fixture(t)
			for i := range r.Seats {
				r.Seats[i].Bot, r.Seats[i].Difficulty = true, "normal"
				r.Match.Power[r.Seats[i].House] = 0
				if rich {
					r.Match.Power[r.Seats[i].House] = 20
				}
			}
			r.Westeros.Threat = 12
			for i, event := range [3]WesterosCardID{WesterosLastDays, WesterosLastDays, WesterosFeastForCrows} {
				deck := &r.Westeros.Decks[i]
				index := slices.Index(deck.Draw, event)
				deck.Draw[0], deck.Draw[index] = deck.Draw[index], deck.Draw[0]
			}
			index := slices.Index(r.Wildlings.Deck, card)
			r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
			if card == WildlingPreemptive {
				index = slices.Index(r.Wildlings.Deck, WildlingSilence)
				r.Wildlings.Deck[1], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[1]
			}
			r.completeRound(c, now)
			if r.Phase != PhaseWesteros || r.Round != 2 || r.Westeros.Current == nil || r.Match.BladeUsed {
				t.Fatal("round boundary skipped events", card, rich)
			}
			for step := 0; step < 512 && r.Phase != "orders_revealed"; step++ {
				if !r.AdvanceBot(c, now) {
					t.Fatal("normal flow stalled", card, rich, r.Phase)
				}
				r = r.Clone()
			}
			if r.Phase != "orders_revealed" || r.Round != 2 || r.Westeros.Run.Stage != WesterosFinished || r.Westeros.Run.NextCard != 3 || r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Westeros.Run.Restriction != WesterosNoPower {
				t.Fatal("normal game skipped a card or failed to finish events", card, rich)
			}
			if card != WildlingPreemptive && r.Wildlings.Last.Card != card {
				t.Fatal("wrong resolved card")
			}
		}
	}
}

func TestDamagedDeckCannotSilentlyAdvanceToPlanningAtRoundEnd(t *testing.T) {
	r, c, now := fixture(t)
	r.Westeros.Decks[0].Draw = nil
	r.completeRound(c, now)
	if r.Phase != PhaseWesteros || r.Westeros.Run != nil || r.Events[len(r.Events)-1].Type != "rules-error" || r.AdvanceBot(c, now) {
		t.Fatal("damaged deck was silently bypassed")
	}
}
