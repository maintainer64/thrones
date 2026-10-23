package server

import (
	"net/http"
	"os"
	"testing"
	"time"
	"westeros-web/internal/game"
)

func TestCombatBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in real combat presentation")
	}
	s := testServer(t)
	type fixture struct {
		ID      string                  `json:"id"`
		Cookies map[string]*http.Cookie `json:"cookies"`
	}
	fixtures := map[string]fixture{}
	for _, name := range []string{"live", "tablet", "english", "queue", "pinned", "priority", "retreat"} {
		r, cookies := seedInfluenceRoom(t, s)
		r.Influence = nil
		r.Phase = "marches"
		r.Match.ActiveHouse = "stark"
		r.Match.BladeUsed = true
		for _, seat := range r.Seats {
			r.Match.Supply[seat.House] = 6
		}
		r.Match.Armies = map[string][]game.Unit{
			"winterfell":   {{Type: "footman", House: "stark", Count: 3}, {Type: "knight", House: "stark", Count: 1}, {Type: "siege-engine", House: "stark", Count: 1}},
			"white-harbor": {{Type: "footman", House: "lannister", Count: 1}, {Type: "knight", House: "lannister", Count: 1}},
			"widows-watch": {{Type: "footman", House: "stark", Count: 1}},
		}
		r.Match.Control = map[string]string{"winterfell": "stark", "white-harbor": "lannister", "widows-watch": "stark"}
		r.Match.Garrisons = map[string]int{"white-harbor": 2}
		r.Orders = map[string]string{"winterfell": "march-minus", "white-harbor": "defend-star", "widows-watch": "support-star"}
		command := func(house string, cmd game.Command) {
			t.Helper()
			player := ""
			for _, seat := range r.Seats {
				if seat.House == house {
					player = seat.Player
				}
			}
			if err := r.ApplyAt(player, cmd, s.Catalog, time.Now().UTC()); err != nil {
				t.Fatalf("%s %s: %v", name, cmd.Type, err)
			}
		}
		if name == "queue" || name == "pinned" || name == "priority" {
			r.Match.Armies["seagard"] = []game.Unit{{Type: "footman", House: "greyjoy", Count: 3}}
			r.Match.Armies["riverrun"] = []game.Unit{{Type: "footman", House: "baratheon", Count: 1}}
			r.Match.Control["seagard"], r.Match.Control["riverrun"] = "greyjoy", "baratheon"
			r.Orders["seagard"] = "march"
		}
		command("stark", game.Command{Type: "march", Region: "winterfell", Target: "white-harbor", Units: map[string]int{"footman": 2, "knight": 1, "siege-engine": 1}})
		if name == "retreat" {
			command("stark", game.Command{Type: "card", Card: "catelyn-stark"})
			command("lannister", game.Command{Type: "card", Card: "tywin-lannister"})
			command("stark", game.Command{Type: "retreat", Target: "winterfell"})
		}
		if name == "queue" || name == "pinned" || name == "priority" {
			command("stark", game.Command{Type: "card", Card: "eddard-stark"})
			command("lannister", game.Command{Type: "card", Card: "ser-kevan-lannister"})
			command("greyjoy", game.Command{Type: "march", Region: "seagard", Target: "riverrun", Units: map[string]int{"footman": 3}})
			if name == "queue" {
				command("greyjoy", game.Command{Type: "card", Card: "dagmar-cleftjaw"})
				command("baratheon", game.Command{Type: "card", Card: "ser-davos-seaworth"})
			}
			if len(r.Match.Combats) != 2 || name == "queue" && (r.Round != 2 || r.Phase != game.PhaseWesteros) {
				t.Fatalf("fixture did not finish two battles: %d %s", r.Round, r.Phase)
			}
		}
		r.Revision++
		if err := s.Store.Save(r, r.Revision-1); err != nil {
			t.Fatal(err)
		}
		fixtures[name] = fixture{r.ID, cookies}
	}
	runDecisionBrowser(t, s, fixtures, "combat.spec.ts", "WESTEROS_COMBAT_FIXTURES")
}
