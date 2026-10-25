package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func westerosCounts(cards []WesterosCardID) map[WesterosCardID]int {
	counts := map[WesterosCardID]int{}
	for _, card := range cards {
		counts[card]++
	}
	return counts
}

func assertWesterosCopies(t *testing.T, s *WesterosState) {
	t.Helper()
	for i, expected := range baseWesterosDecks() {
		actual := append(slices.Clone(s.Decks[i].Draw), s.Decks[i].Discard...)
		if !reflect.DeepEqual(westerosCounts(actual), westerosCounts(expected)) {
			t.Fatalf("deck %d lost or duplicated physical cards: %v", i+1, actual)
		}
	}
}

func TestWesterosThirtyCopiesAndNoWinterInDeckThree(t *testing.T) {
	want := [3]map[WesterosCardID]int{
		{WesterosSupply: 3, WesterosMustering: 3, WesterosThroneOfBlades: 2, WesterosLastDays: 1, WesterosWinter: 1},
		{WesterosGameOfThrones: 3, WesterosClashOfKings: 3, WesterosDarkWings: 2, WesterosLastDays: 1, WesterosWinter: 1},
		{WesterosWildlings: 3, WesterosPutToSword: 2, WesterosStormOfSwords: 1, WesterosRainsOfAutumn: 1, WesterosSeaOfStorms: 1, WesterosWebOfLies: 1, WesterosFeastForCrows: 1},
	}
	for repeat := 0; repeat < 25; repeat++ {
		state := newWesterosState()
		for i, deck := range state.Decks {
			if len(deck.Draw) != 10 || len(deck.Discard) != 0 || !reflect.DeepEqual(westerosCounts(deck.Draw), want[i]) {
				t.Fatalf("wrong deck %d: %+v", i+1, deck)
			}
		}
		if state.Current != nil || state.Threat != 2 {
			t.Fatal("new decks must be unrevealed with the threat marker at 2")
		}
	}
}

func TestWesterosShuffleUsesEveryFisherYatesBound(t *testing.T) {
	cards := baseWesterosDecks()[0]
	original := slices.Clone(cards)
	bounds := []int{}
	shuffleWesteros(cards, func(n int) int {
		bounds = append(bounds, n)
		return 0
	})
	if !slices.Equal(bounds, []int{10, 9, 8, 7, 6, 5, 4, 3, 2}) || slices.Equal(cards, original) || !reflect.DeepEqual(westerosCounts(cards), westerosCounts(original)) {
		t.Fatal("shuffle changed weights or used an incorrect random range", cards, bounds)
	}
	if !slices.Equal(baseWesterosDecks()[0], original) {
		t.Fatal("shuffle mutated the base catalogue")
	}
}

func TestWesterosNormalDrawAndSameRoundIsIdempotent(t *testing.T) {
	s := &WesterosState{}
	for i, cards := range baseWesterosDecks() {
		s.Decks[i].Draw = cards
	}
	noShuffle := func(int) int { t.Fatal("normal draws must not shuffle"); return 0 }
	if err := s.reveal(2, noShuffle); err != nil {
		t.Fatal(err)
	}
	if s.Current.Cards != [3]WesterosCardID{WesterosSupply, WesterosGameOfThrones, WesterosWildlings} || s.Current.WinterDraws != [3]int{} || s.Current.Round != 2 {
		t.Fatal("wrong reveal", s.Current)
	}
	for _, deck := range s.Decks {
		if len(deck.Draw) != 9 || len(deck.Discard) != 1 {
			t.Fatal("ordinary draw should consume exactly one card", deck)
		}
	}
	before, _ := json.Marshal(s)
	if err := s.reveal(2, noShuffle); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("same-round retry rerolled events")
	}
	if err := s.reveal(1, noShuffle); err == nil {
		t.Fatal("allowed rewinding the event deck")
	}
	assertWesterosCopies(t, s)
}

func TestWesterosWinterRecombinesDiscardDrawAndItself(t *testing.T) {
	cards := baseWesterosDecks()[0]
	deck := WesterosDeck{Draw: append([]WesterosCardID{WesterosWinter}, cards[5:9]...), Discard: slices.Clone(cards[:5])}
	// An identity permutation makes it explicit which old piles are retained.
	calls := 0
	card, winters := deck.draw(func(n int) int { calls++; return n - 1 })
	if winters != 1 || card != cards[5] || calls != 9 || !slices.Equal(deck.Discard, []WesterosCardID{card}) || len(deck.Draw) != 9 || !slices.Contains(deck.Draw, WesterosWinter) {
		t.Fatal("Winter failed to shuffle all ten cards and redraw", deck, card, winters, calls)
	}
	if !reflect.DeepEqual(westerosCounts(append(slices.Clone(deck.Draw), deck.Discard...)), westerosCounts(cards)) {
		t.Fatal("old discards were lost or Winter was removed")
	}
}

func TestWesterosWinterCanBeDrawnAgainImmediately(t *testing.T) {
	cards := baseWesterosDecks()[0]
	deck := WesterosDeck{Draw: append([]WesterosCardID{WesterosWinter}, cards[:9]...)}
	calls := 0
	card, winters := deck.draw(func(n int) int {
		calls++
		if calls == 1 {
			return 0 // First shuffle moves the just-discarded Winter back to top.
		}
		return n - 1 // Second shuffle leaves the next non-Winter on top.
	})
	if winters != 2 || card == WesterosWinter || calls != 18 || len(deck.Draw) != 9 || len(deck.Discard) != 1 {
		t.Fatal("repeated Winter was skipped or treated as an event", card, winters, calls, deck)
	}
	if !reflect.DeepEqual(westerosCounts(append(slices.Clone(deck.Draw), deck.Discard...)), westerosCounts(cards)) {
		t.Fatal("repeated Winter changed physical copies")
	}
}

func TestWesterosEmptyDeckReshufflesItsOwnDiscard(t *testing.T) {
	cards := baseWesterosDecks()[2]
	deck := WesterosDeck{Draw: slices.Clone(cards)}
	calls := 0
	random := func(n int) int { calls++; return n - 1 }
	for _, expected := range cards {
		card, winters := deck.draw(random)
		if card != expected || winters != 0 {
			t.Fatal("deck III contains Winter or reordered unshuffled cards")
		}
	}
	if len(deck.Draw) != 0 || len(deck.Discard) != 10 || calls != 0 {
		t.Fatal("reshuffled before exhaustion")
	}
	card, _ := deck.draw(random)
	if card != cards[0] || calls != 9 || len(deck.Draw) != 9 || len(deck.Discard) != 1 {
		t.Fatal("exhausted deck not recycled", deck)
	}
}

func TestWesterosRepeatedRevealsKeepAllPhysicalCards(t *testing.T) {
	s := newWesterosState()
	for round := 2; round < 150; round++ {
		if err := s.reveal(round, randomIndex); err != nil {
			t.Fatal(err)
		}
		assertWesterosCopies(t, s)
		for i, card := range s.Current.Cards {
			if card == WesterosWinter || s.Decks[i].Discard[len(s.Decks[i].Discard)-1] != card {
				t.Fatal("final event was not discarded or is unresolved Winter")
			}
		}
	}
}

func TestWesterosCorruptDeckRejectsWithoutPartialDraws(t *testing.T) {
	for _, corrupt := range []func(*WesterosState){
		func(s *WesterosState) { s.Decks[2].Draw = s.Decks[2].Draw[:9] },
		func(s *WesterosState) { s.Decks[2].Draw[0] = WesterosWinter },
	} {
		s := newWesterosState()
		corrupt(s)
		before, _ := json.Marshal(s)
		if err := s.reveal(2, randomIndex); err == nil {
			t.Fatal("accepted invalid deck")
		} else if hasCyrillic(TranslateSystemText(err.Error(), "en")) {
			t.Fatal("untranslated deck validation error", err)
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("invalid deck consumed cards from an earlier deck")
		}
	}
}

func TestWesterosHookOnlyAtValidPhaseBoundary(t *testing.T) {
	for _, setup := range []func(*Room){
		func(r *Room) { r.Phase = "planning" },
		func(r *Room) { r.Round = 1 },
		func(r *Room) { r.Round = 11 },
		func(r *Room) { r.Match = nil },
		func(r *Room) { r.Battle = &Battle{} },
		func(r *Room) { r.Supply = &SupplyResolution{} },
		func(r *Room) { r.Mustering = &Mustering{} },
		func(r *Room) { r.Influence = &InfluenceAuction{} },
	} {
		r, _ := started(t)
		r.Phase, r.Round = PhaseWesteros, 2
		setup(r)
		before, _ := json.Marshal(r)
		if err := r.RevealWesterosCards(); err == nil {
			t.Fatal("opened Westeros events at an invalid boundary")
		}
		after, _ := json.Marshal(r)
		if string(before) != string(after) {
			t.Fatal("invalid hook mutated the room")
		}
	}
	r, _ := started(t)
	r.Phase, r.Round, r.Westeros = PhaseWesteros, 2, nil
	if err := r.RevealWesterosCards(); err != nil || r.Westeros == nil || r.Westeros.Current.Round != 2 || r.Phase != PhaseWesteros {
		t.Fatal("valid boundary not initialized or unresolved effects were skipped", err)
	}
}

func TestWesterosViewsHideFutureAndDoNotAliasState(t *testing.T) {
	r, c := started(t)
	if r.Westeros == nil || r.Westeros.Current != nil || r.Phase != "planning" || r.Round != 1 {
		t.Fatal("start must prepare decks without revealing round-one events")
	}
	r.Phase, r.Round = PhaseWesteros, 2
	if err := r.RevealWesterosCards(); err != nil {
		t.Fatal(err)
	}
	for _, player := range []string{"host-secret", "outsider"} {
		view := r.View(player, c).Westeros
		data, _ := json.Marshal(view)
		if strings.Contains(string(data), `"draw"`) || view.Current == nil || view.Current.Round != 2 {
			t.Fatal("future cards leaked or revealed events are missing", string(data))
		}
		before := r.Clone()
		view.Current.Cards[0] = "changed"
		view.Decks[0].Discard[0] = "changed"
		if !reflect.DeepEqual(before.Westeros, r.Westeros) {
			t.Fatal("view aliases hidden deck state")
		}
	}
	clone := r.Clone()
	clone.Westeros.Decks[0].Draw[0] = "changed"
	if r.Westeros.Decks[0].Draw[0] == "changed" {
		t.Fatal("cloned room aliases another room's future deck")
	}
}
