package server

import (
	"net/http"
	"os"
	"testing"
	"time"
	"westeros-web/internal/game"
)

func TestSetupBladeBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in real setup and ordinary Blade UI")
	}
	s := testServer(t)
	type fixture struct {
		ID      string                  `json:"id"`
		Cookies map[string]*http.Cookie `json:"cookies"`
	}
	fixtures := map[string]fixture{}
	for _, name := range []string{"use", "keep", "used"} {
		r, cookies := seedInfluenceRoom(t, s)
		r.Influence = nil
		r.Phase = "marches"
		r.Settings.TidesOfBattle = false
		r.Match.Armies = map[string][]game.Unit{"pyke": {{Type: "footman", House: "greyjoy", Count: 1}}, "white-harbor": {{Type: "footman", House: "greyjoy", Count: 2}}}
		r.Match.ControlTokens = map[string]string{"white-harbor": "greyjoy"}
		r.Match.Control = map[string]string{"white-harbor": "greyjoy", "winterfell": "stark"}
		r.Match.Garrisons = map[string]int{"winterfell": 2}
		r.Match.Tracks["fiefdoms"] = []string{"stark", "greyjoy", "lannister", "baratheon", "tyrell", "martell"}
		r.Match.ActiveHouse = "greyjoy"
		r.Match.BladeUsed = name == "used"
		// Keep this combat isolated from the separate end-of-round presentation
		// queue (PLAN G), with another real march left to resolve.
		r.Orders = map[string]string{"pyke": "march-minus", "white-harbor": "march"}
		for _, seat := range r.Seats {
			if seat.House == "greyjoy" {
				if err := r.ApplyAt(seat.Player, game.Command{Type: "march", Region: "white-harbor", Target: "winterfell", Units: map[string]int{"footman": 2}}, s.Catalog, time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			}
		}
		r.Revision++
		if err := s.Store.Save(r, r.Revision-1); err != nil {
			t.Fatal(err)
		}
		fixtures[name] = fixture{r.ID, cookies}
	}
	runDecisionBrowser(t, s, fixtures, "setup-blade.spec.ts", "WESTEROS_SETUP_BLADE_FIXTURES")
}
