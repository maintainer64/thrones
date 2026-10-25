package server

import (
	"context"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"westeros-web/internal/game"
)

// Real HTTP/WebSocket/SQLite browser coverage; no production fixture endpoint.
func TestEventsBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in real-browser Westeros and wildling decisions")
	}
	s := testServer(t)
	type fixture struct {
		ID      string                  `json:"id"`
		Cookies map[string]*http.Cookie `json:"cookies"`
	}
	fixtures := map[string]fixture{}
	for _, name := range []string{"privacy", "zero", "units", "cards", "tracks", "castle", "event", "repeat"} {
		r, cookies := seedInfluenceRoom(t, s)
		r.Influence, r.Phase, r.Round = nil, game.PhaseWesteros, 2
		r.Westeros.Threat = 12
		r.Match.Armies = map[string][]game.Unit{
			"winterfell":   {{Type: "footman", House: "stark", Count: 1}, {Type: "knight", House: "stark", Count: 1}},
			"white-harbor": {{Type: "footman", House: "stark", Count: 2}},
		}
		r.Match.Control = map[string]string{"winterfell": "stark", "white-harbor": "stark"}
		r.Match.Supply["stark"] = 6
		r.Match.Discard["stark"] = []string{"eddard-stark", "robb-stark"}
		r.Match.Tracks["ironThrone"] = slices.Clone(game.BaseHouses)
		cards := [3]game.WesterosCardID{game.WesterosLastDays, game.WesterosLastDays, game.WesterosFeastForCrows}
		wildling := game.WildlingSilence
		switch name {
		case "units", "castle":
			wildling = game.WildlingHorde
		case "cards":
			wildling = game.WildlingMammoths
		case "tracks", "repeat":
			wildling = game.WildlingPreemptive
		case "event":
			cards[0], r.Westeros.Threat = game.WesterosThroneOfBlades, 0
		}
		for i, card := range cards {
			deck := &r.Westeros.Decks[i]
			index := slices.Index(deck.Draw, card)
			deck.Draw[0], deck.Draw[index] = deck.Draw[index], deck.Draw[0]
		}
		index := slices.Index(r.Wildlings.Deck, wildling)
		r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
		if name == "repeat" {
			index = slices.Index(r.Wildlings.Deck, game.WildlingSilence)
			r.Wildlings.Deck[1], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[1]
		}
		now := time.Now().UTC()
		if err := r.BeginWesteros(now); err != nil {
			t.Fatal(err)
		}
		for step := 0; step < 3 && r.Phase == game.PhaseWesteros && r.Westeros.Run.Choice == nil; step++ {
			if !r.AdvanceBot(s.Catalog, now) {
				t.Fatal("could not reach event", name)
			}
		}
		if name == "zero" {
			r.Wildlings.Attack.Strength, r.Westeros.Run.Attack.Strength, r.Westeros.Threat = 0, 0, 0
			for _, house := range game.BaseHouses {
				r.Match.Power[house] = 0
			}
		}
		if name != "event" && name != "privacy" {
			for _, seat := range r.Seats {
				if name == "zero" && seat.House == "stark" {
					continue
				}
				bid := 0
				if name == "units" || name == "tracks" {
					bid = 1
					if seat.House == "stark" {
						bid = 0
					}
				} else if name != "zero" && seat.House == "stark" {
					bid = 12
				}
				if err := r.ApplyAt(seat.Player, game.Command{Type: "wildling_bid", Bid: &bid}, s.Catalog, now); err != nil {
					t.Fatal(err)
				}
			}
			if name != "zero" && !r.AdvanceBot(s.Catalog, now) {
				t.Fatal("could not reach card effect", name)
			}
		}
		if name == "tracks" {
			r.Match.Tracks["ironThrone"] = []string{"greyjoy", "stark", "lannister", "baratheon", "tyrell", "martell"}
			r.Match.Tracks["fiefdoms"] = slices.Clone(r.Match.Tracks["ironThrone"])
			r.Match.Tracks["kingsCourt"] = []string{"greyjoy", "lannister", "stark", "baratheon", "tyrell", "martell"}
		}
		r.Revision++
		if err := s.Store.Save(r, r.Revision-1); err != nil {
			t.Fatal(err)
		}
		fixtures[name] = fixture{ID: r.ID, Cookies: cookies}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.RunTimers(ctx) }()
	defer func() { cancel(); <-done }()
	runDecisionBrowser(t, s, fixtures, "events.spec.ts", "WESTEROS_EVENTS_FIXTURES")
}
