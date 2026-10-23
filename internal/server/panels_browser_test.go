package server

import (
	"os"
	"testing"
)

func TestPanelsBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in real houses and sidebar UI")
	}
	s := testServer(t)
	runDecisionBrowser(t, s, nil, "panels.spec.ts", "WESTEROS_PANELS_FIXTURES")
}
