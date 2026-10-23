package store

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestWildlingBiddingTieEffectsAndReportSurviveDatabaseReopen(t *testing.T) {
	for _, stage := range []string{"bidding", "tie", "effects", "complete"} {
		t.Run(stage, func(t *testing.T) {
			c, now := game.LoadCatalog(), time.Now().UTC()
			r := game.NewRoom("wildlings", "Title", "host", "Name")
			if err := r.Apply("host", game.Command{Type: "start"}, c); err != nil {
				t.Fatal(err)
			}
			for i := range r.Seats {
				r.Seats[i].Bot, r.Seats[i].Player = false, r.Seats[i].House
			}
			for i, card := range [3]game.WesterosCardID{game.WesterosSupply, game.WesterosGameOfThrones, game.WesterosWildlings} {
				deck := &r.Westeros.Decks[i]
				index := slices.Index(deck.Draw, card)
				deck.Draw[0], deck.Draw[index] = deck.Draw[index], deck.Draw[0]
			}
			card := game.WildlingHorde
			if stage == "complete" {
				card = game.WildlingSilence
			}
			index := slices.Index(r.Wildlings.Deck, card)
			r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
			r.Phase, r.Round, r.Westeros.Threat = game.PhaseWesteros, 2, 0
			if err := r.BeginWesteros(now); err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 10 && r.Phase != game.PhaseWildlingBidding; step++ {
				if !r.AdvanceBot(c, now) {
					t.Fatal("fixture failed to enter the real attack")
				}
			}
			zero := 0
			for _, house := range game.BaseHouses {
				if err := r.ApplyAt(house, game.Command{Type: "wildling_bid", Bid: &zero}, c, now); err != nil {
					t.Fatal(err)
				}
				if stage == "bidding" {
					break
				}
			}
			if stage == "effects" || stage == "complete" {
				if err := r.ApplyAt(r.Wildlings.Attack.TieBreakHouse, game.Command{Type: "wildling_tie", Target: "stark"}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "complete" && !r.AdvanceBot(c, now) {
				t.Fatal("automatic Silence effect did not complete")
			}
			if stage == "effects" && !r.AdvanceBot(c, now) {
				t.Fatal("expected a durable human reward decision")
			}
			filename := filepath.Join(t.TempDir(), "wildlings.sqlite")
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
			if err != nil || !reflect.DeepEqual(saved.Wildlings, r.Wildlings) || !reflect.DeepEqual(saved.Westeros, r.Westeros) || !saved.Clock.Deadline.Equal(r.Clock.Deadline) {
				t.Fatal("reopen lost a private deck, bid, selected bidder, effect or deadline", err)
			}
			rooms, err := db.ActiveActions()
			if err != nil || len(rooms) != 1 || rooms[0].Phase != r.Phase {
				t.Fatal("scheduler cannot find the persisted wildling stage", err)
			}
			if stage == "bidding" {
				if saved.View("greyjoy", c).Wildlings.Attack.Bids.YourBid != nil || saved.View("greyjoy", c).Wildlings.Attack.Card != "" {
					t.Fatal("reopening disclosed another contribution or future card")
				}
			}
			if changed := saved.AdvanceBot(c, now); changed != (stage == "complete") {
				t.Fatal("reopen bypassed a human decision")
			}
			if !reflect.DeepEqual(saved.Wildlings, r.Wildlings) {
				t.Fatal("reload rerolled or reapplied an attack")
			}
		})
	}
}
