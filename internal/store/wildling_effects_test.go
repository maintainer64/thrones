package store

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func storeWildlingEffect(t *testing.T, card game.WildlingCardID) (*game.Room, game.Catalog, time.Time) {
	t.Helper()
	return storeWildlingOutcome(t, card, false)
}

func storeWildlingOutcome(t *testing.T, card game.WildlingCardID, won bool) (*game.Room, game.Catalog, time.Time) {
	t.Helper()
	c, now := game.LoadCatalog(), time.Now().UTC()
	r := game.NewRoom("wildling-effects", "Title", "host", "Name")
	if err := r.Apply("host", game.Command{Type: "start"}, c); err != nil {
		t.Fatal(err)
	}
	for i := range r.Seats {
		r.Seats[i].Bot, r.Seats[i].Player = false, r.Seats[i].House
		r.Match.Power[r.Seats[i].House] = 10
	}
	for i, event := range [3]game.WesterosCardID{game.WesterosSupply, game.WesterosGameOfThrones, game.WesterosWildlings} {
		deck := &r.Westeros.Decks[i]
		index := slices.Index(deck.Draw, event)
		deck.Draw[0], deck.Draw[index] = deck.Draw[index], deck.Draw[0]
	}
	index := slices.Index(r.Wildlings.Deck, card)
	r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
	r.Phase, r.Round, r.Westeros.Threat = game.PhaseWesteros, 2, 12
	if err := r.BeginWesteros(now); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 4 && r.Phase != game.PhaseWildlingBidding; step++ {
		if !r.AdvanceBot(c, now) {
			t.Fatal("fixture could not start the actual attack")
		}
	}
	for _, house := range game.BaseHouses {
		bid := 1
		if house == "stark" {
			bid = 0
			if won {
				bid = 7
			}
		}
		if err := r.ApplyAt(house, game.Command{Type: "wildling_bid", Bid: &bid}, c, now); err != nil {
			t.Fatal(err)
		}
	}
	if r.Phase != game.PhaseWildlingEffects || r.Wildlings.Attack.WatchWon != won {
		t.Fatal("fixture did not reach the intended outcome")
	}
	return r, c, now
}

func TestWildlingSupplyAndInfluenceContinuationsSurviveDatabaseReopen(t *testing.T) {
	for _, stage := range []string{"raiders-disband", "raiders-after-disband", "king-choice", "king-between-choices"} {
		t.Run(stage, func(t *testing.T) {
			card := game.WildlingKing
			if stage == "raiders-disband" || stage == "raiders-after-disband" {
				card = game.WildlingRaiders
			}
			r, c, now := storeWildlingEffect(t, card)
			if card == game.WildlingRaiders {
				for _, house := range game.BaseHouses {
					r.Match.Supply[house] = 1
				}
				r.Match.Supply["stark"] = 2
				r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "footman", House: "stark", Count: 3}}}
			}
			r.Match.BladeUsed = true
			if !r.AdvanceBot(c, now) {
				t.Fatal("effect could not start")
			}
			if stage == "raiders-after-disband" {
				if err := r.ApplyAt("stark", game.Command{Type: "disband", Region: "winterfell", Units: map[string]int{"footman": 1}}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "king-between-choices" {
				choice := r.Wildlings.Attack.Effect.Choice
				if err := r.ApplyAt(choice.House, game.Command{Type: "wildling_choice", Choice: string(game.TrackFiefdoms)}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			filename := filepath.Join(t.TempDir(), "effects.sqlite")
			db, err := Open(filename)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Create(r); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = Open(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			saved, err := db.Load(r.ID)
			if err != nil || !reflect.DeepEqual(saved.Wildlings, r.Wildlings) || !reflect.DeepEqual(saved.Supply, r.Supply) || !reflect.DeepEqual(saved.Match, r.Match) || !saved.Clock.Deadline.Equal(r.Clock.Deadline) {
				t.Fatal("reopen lost supply source, remaining queue, choice, tracks or clock", err)
			}
			active, err := db.ActiveActions()
			if err != nil || len(active) != 1 || active[0].Phase != r.Phase {
				t.Fatal("headless automation cannot find the nested phase", err)
			}
			before := saved.Clone()
			changed := saved.AdvanceBot(c, now.Add(time.Minute))
			wantChanged := stage == "raiders-after-disband" || stage == "king-between-choices"
			if changed != wantChanged || (!changed && !reflect.DeepEqual(before, saved)) || !reflect.DeepEqual(before.Match.Tracks, saved.Match.Tracks) {
				t.Fatal("reload replayed the lowest penalty, bypassed a human or lost the continuation")
			}
			if card == game.WildlingRaiders && saved.Match.Supply["stark"] != 0 {
				t.Fatal("reload recounted supply or repeated a delta")
			}
			for i := range saved.Seats {
				saved.Seats[i].Bot = true
			}
			for step := 0; step < 24 && saved.Wildlings.Attack != nil; step++ {
				if !saved.AdvanceBot(c, now) {
					t.Fatal("restored effect failed to complete")
				}
			}
			if saved.Wildlings.Attack != nil || saved.Wildlings.Last == nil || saved.Wildlings.Last.Card != card || len(saved.Wildlings.Deck) != 9 || saved.Westeros.Threat != 8 || !saved.Match.BladeUsed {
				t.Fatal("restored effect lost its final report, continuation or spent token")
			}
			penalties := 0
			for _, event := range saved.Events {
				if event.House == "stark" && (event.Type == "wildling-supply" || event.Type == "wildling-influence") {
					penalties++
				}
			}
			wantPenalties := 1
			if card == game.WildlingKing {
				wantPenalties = 3
			}
			if penalties != wantPenalties {
				t.Fatal("reload repeated the lowest bidder penalty", penalties)
			}
		})
	}
}
