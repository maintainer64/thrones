package server

import (
	"context"
	"net/http"
	"os"
	"testing"

	"westeros-web/internal/game"
)

func TestRavenBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in real-browser Messenger Raven")
	}
	s := testServer(t)
	type fixture struct {
		ID      string                  `json:"id"`
		Cookies map[string]*http.Cookie `json:"cookies"`
	}
	fixtures := map[string]fixture{}
	for _, name := range []string{"privacy", "replacement", "decline", "no-orders"} {
		r, cookies := seedRavenRoom(t, s)
		if name == "replacement" {
			// The same restriction must constrain the UI and command validation.
			r.Westeros.Run = &game.WesterosRun{Round: r.Round, Stage: game.WesterosFinished, Restriction: game.WesterosNoRaid}
		}
		if name == "no-orders" {
			r.Orders = map[string]string{"pyke": "march"}
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
	runDecisionBrowser(t, s, fixtures, "raven.spec.ts", "WESTEROS_RAVEN_FIXTURES")
}
