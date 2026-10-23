package server

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
	"westeros-web/internal/game"
)

// Only this test process seeds positions. Bot turns use the production timer,
// commands, persistence and WebSocket broadcast; there is no fixture HTTP API.
func TestCombatMatrixBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in real complex combat UI")
	}
	s := testServer(t)
	type fixture struct {
		ID      string                  `json:"id"`
		Cookies map[string]*http.Cookie `json:"cookies"`
	}
	fixtures := map[string]fixture{}
	for _, name := range []string{"bot-human", "bot-bot", "catelyn", "plain", "arianne", "renly", "trapped", "tides-blade", "tides-redraw", "skull"} {
		r, cookies := seedInfluenceRoom(t, s)
		r.Influence = nil
		r.Phase = "marches"
		r.Match.ActiveHouse, r.Match.BladeUsed = "stark", true
		a, d := "stark", "lannister"
		ac, dc := "eddard-stark", "ser-kevan-lannister"
		if name == "catelyn" {
			a, d, ac, dc = "lannister", "stark", "ser-kevan-lannister", "catelyn-stark"
		}
		if name == "arianne" {
			a, d, ac, dc = "baratheon", "martell", "stannis-baratheon", "arianne-martell"
		}
		if name == "trapped" {
			ac = "roose-bolton"
		}
		if name == "renly" {
			a, d, ac, dc = "baratheon", "stark", "renly-baratheon", "catelyn-stark"
		}
		for _, seat := range r.Seats {
			r.Match.Supply[seat.House] = 6
		}
		to := "white-harbor"
		if name == "plain" {
			to = "widows-watch"
		}
		if name == "skull" {
			to = "moat-cailin"
		}
		r.Match.Armies = map[string][]game.Unit{
			"winterfell":        {{Type: "footman", House: a, Count: 3}, {Type: "knight", House: a, Count: 1}, {Type: "siege-engine", House: a, Count: 1}},
			to:                  {{Type: "footman", House: d, Count: 1}, {Type: "knight", House: d, Count: 1}},
			"the-shivering-sea": {{Type: "ship", House: a, Count: 1}},
		}
		r.Match.Control = map[string]string{"winterfell": a, to: d, "the-shivering-sea": a}
		r.Match.Garrisons = map[string]int{to: 2}
		r.Orders = map[string]string{"winterfell": "march-minus", to: "defend-star", "the-shivering-sea": "support-star"}
		if name == "skull" {
			r.Match.Armies["winterfell"] = []game.Unit{{Type: "footman", House: a, Count: 1}}
			r.Match.Armies[to] = []game.Unit{{Type: "footman", House: d, Count: 1}}
			delete(r.Match.Armies, "the-shivering-sea")
			r.Match.Garrisons, r.Orders = map[string]int{}, map[string]string{"winterfell": "march"}
		}
		player := func(house string) string {
			for _, seat := range r.Seats {
				if seat.House == house {
					return seat.Player
				}
			}
			t.Fatal("missing seat", house)
			return ""
		}
		command := func(house string, cmd game.Command) {
			t.Helper()
			if err := r.ApplyAt(player(house), cmd, s.Catalog, time.Now().UTC()); err != nil {
				t.Fatalf("%s %s: %v", name, cmd.Type, err)
			}
		}
		if name == "bot-human" || name == "bot-bot" {
			if name == "bot-bot" {
				r.Westeros.Current = &game.WesterosReveal{Round: r.Round, Cards: [3]game.WesterosCardID{game.WesterosSupply, game.WesterosGameOfThrones, game.WesterosStormOfSwords}}
			}
			for i := range r.Seats {
				if r.Seats[i].House == a || name == "bot-bot" && r.Seats[i].House == d {
					r.Seats[i].Bot, r.Seats[i].Difficulty = true, "hard"
				}
			}
			for _, region := range s.Catalog.Boards["westeros"].Regions {
				if region.ID != "winterfell" && region.ID != to && region.ID != "the-shivering-sea" && region.ID != "highgarden" {
					r.Match.BlockedRegions = append(r.Match.BlockedRegions, region.ID)
				}
			}
			r.Match.ActiveHouse = "tyrell" // Human command releases the real bot scheduler.
			r.Match.Armies["highgarden"] = []game.Unit{{Type: "footman", House: "tyrell", Count: 1}}
			r.Match.Control["highgarden"], r.Orders["highgarden"] = "tyrell", "march"
			r.Match.Tracks["ironThrone"] = []string{"tyrell", a, d, "greyjoy", "baratheon", "martell"}
		} else {
			r.Match.ActiveHouse = a
			units := map[string]int{"footman": 2, "knight": 1, "siege-engine": 1}
			if name == "skull" {
				units = map[string]int{"footman": 1}
			}
			command(a, game.Command{Type: "march", Region: "winterfell", Target: to, Units: units})
			if name == "tides-blade" || name == "tides-redraw" {
				r.Settings.TidesOfBattle, r.Match.BladeUsed = true, false
				r.Match.Tracks["fiefdoms"] = []string{a, d, "greyjoy", "baratheon", "tyrell", "martell"}
			} else {
				if name == "trapped" {
					for _, region := range s.Catalog.Boards["westeros"].Regions {
						if region.ID != to && region.ID != "winterfell" {
							r.Match.BlockedRegions = append(r.Match.BlockedRegions, region.ID)
						}
					}
				}
				if name == "skull" {
					r.Settings.TidesOfBattle = true
					r.Battle.Tides = &game.TidesState{Cards: map[string]game.TidesCard{a: {ID: game.TideZero}, d: {ID: game.TideSkull, Skulls: 1}}, RedrawDone: true, BonusDone: true}
				}
				command(a, game.Command{Type: "card", Card: ac})
				command(d, game.Command{Type: "card", Card: dc})
				for step := 0; r.Battle != nil && step < 8; step++ {
					if r.Phase == "card_effect" {
						choice := r.Battle.Choice
						value := choice.Options[0]
						if choice.Card == "renly-baratheon" {
							value = to
						}
						command(choice.House, game.Command{Type: "ability", Choice: value})
					} else if r.Phase == "retreat" {
						chooser := r.Match.ActiveHouse
						view := r.View(player(chooser), s.Catalog)
						if len(view.Battle.Retreats) == 0 {
							t.Fatal("retreat without targets")
						}
						command(chooser, game.Command{Type: "retreat", Target: view.Battle.Retreats[0]})
					} else {
						t.Fatalf("unhandled fixture phase %s", r.Phase)
					}
				}
				if len(r.Match.Combats) != 1 || r.Match.Combats[0].Result.Final == nil {
					t.Fatalf("%s: no final snapshot", name)
				}
			}
		}
		r.Revision++
		if err := s.Store.Save(r, r.Revision-1); err != nil {
			t.Fatal(err)
		}
		fixtures[name] = fixture{r.ID, cookies}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.RunTimers(ctx) }()
	defer func() { cancel(); <-done }()
	runDecisionBrowser(t, s, fixtures, "combat-matrix.spec.ts", "WESTEROS_COMBAT_MATRIX_FIXTURES")
}
