package server

import (
	"net/http"
	"os"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestProvisionBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in real-browser provision tests")
	}
	s := testServer(t)
	type fixture struct {
		ID      string                  `json:"id"`
		Cookies map[string]*http.Cookie `json:"cookies"`
	}
	fixtures := map[string]fixture{}
	for _, name := range []string{"supply", "muster", "power", "power-collect"} {
		r, cookies := seedInfluenceRoom(t, s)
		r.Influence = nil
		r.Phase = game.PhaseWesteros
		r.Orders = map[string]string{}
		r.Match.Control = map[string]string{"winterfell": "stark", "white-harbor": "stark"}
		r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "footman", House: "stark", Count: 2}}}
		r.Match.Supply["stark"] = 6
		r.Match.Power["stark"] = 5
		now := time.Now().UTC()
		var err error
		switch name {
		case "supply":
			r.Match.Armies["winterfell"] = []game.Unit{{Type: "footman", House: "stark", Count: 3}, {Type: "knight", House: "stark", Count: 1}}
			r.Match.Armies["white-harbor"] = []game.Unit{{Type: "knight", House: "stark", Count: 2}}
			err = r.BeginSupply(s.Catalog, now)
		case "muster":
			r.Match.Armies["the-narrow-sea"] = []game.Unit{{Type: "ship", House: "greyjoy", Count: 1}}
			err = r.BeginMustering(s.Catalog, now)
		case "power", "power-collect":
			// Reach the real end-of-actions flow, not a synthetic power decision.
			r.Phase = "marches"
			r.Match.ActiveHouse = "stark"
			r.Match.Armies["white-harbor"] = []game.Unit{{Type: "footman", House: "stark", Count: 1}}
			r.Orders = map[string]string{"white-harbor": "march", "winterfell": "power-star"}
			err = r.ApplyAt(r.Host, game.Command{Type: "pass", Region: "white-harbor"}, s.Catalog, now)
		}
		if err != nil {
			t.Fatal(err)
		}
		r.Revision++
		if err = s.Store.Save(r, r.Revision-1); err != nil {
			t.Fatal(err)
		}
		fixtures[name] = fixture{ID: r.ID, Cookies: cookies}
	}
	runDecisionBrowser(t, s, fixtures, "provision.spec.ts", "WESTEROS_PROVISION_FIXTURES")
}
