package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"westeros-web/internal/assets"
	"westeros-web/internal/game"
)

// Opt-in real-browser coverage of a deterministic Clash of Kings. Seed through
// the engine only in this test process; production has no fixture API, debug
// command or way for a player to force an auction.
// First run: cd web && pnpm build; then node scripts/sync-web.mjs from repo root.
func TestInfluenceBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("build/sync the frontend and set WESTEROS_BROWSER_TEST=1 for real-browser tests")
	}
	s := testServer(t)
	type fixture struct {
		ID      string                  `json:"id"`
		Cookies map[string]*http.Cookie `json:"cookies"`
	}
	fixtures := map[string]fixture{}
	for _, name := range []string{"privacy", "complete", "tablet", "conflict"} {
		r, cookies := seedInfluenceRoom(t, s)
		if name == "tablet" {
			// Exercise zero power and an all-zero tie with the human as ruler.
			for _, seat := range r.Seats {
				r.Match.Power[seat.House] = 0
			}
			r.Match.Tracks["ironThrone"] = []string{"stark", "greyjoy", "baratheon", "lannister", "tyrell", "martell"}
			r.Influence, r.Phase = nil, game.PhaseWesteros
			if err := r.BeginClashOfKings(time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			zero := 0
			for _, seat := range r.Seats {
				if seat.House != "stark" {
					if err := r.ApplyAt(seat.Player, game.Command{Type: "bid", Bid: &zero}, s.Catalog, time.Now().UTC()); err != nil {
						t.Fatal(err)
					}
				}
			}
			r.Revision++
			if err := s.Store.Save(r, r.Revision-1); err != nil {
				t.Fatal(err)
			}
		}
		fixtures[name] = fixture{ID: r.ID, Cookies: cookies}
	}
	runDecisionBrowser(t, s, fixtures, "influence.spec.ts", "WESTEROS_INFLUENCE_FIXTURES")
}

func runDecisionBrowser(t *testing.T, s *Server, fixtures any, spec, fixtureEnv string) {
	t.Helper()
	s.mux.Handle("GET /", http.FileServer(http.FS(assets.Web())))
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	s.LocalAssets(filepath.Join(root, "local-assets"))
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	data, err := json.Marshal(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("pnpm", "test:e2e", spec)
	if grep := os.Getenv("WESTEROS_BROWSER_GREP"); grep != "" {
		command.Args = append(command.Args, "--grep", grep)
	}
	command.Dir = filepath.Join(root, "web")
	output := os.Getenv("PLAYWRIGHT_OUTPUT_DIR")
	if output == "" {
		output = "test-results"
	}
	// Keep earlier suite failures/screenshots when the next spec starts.
	command.Env = append(os.Environ(), "BASE_URL="+httpServer.URL, fixtureEnv+"="+string(data), "PLAYWRIGHT_OUTPUT_DIR="+filepath.Join(output, spec))
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal("real decision browser tests:", err)
	}
}
