package server

import (
	"os"
	"testing"
)

func TestTidesBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in Tides setup and full play UI")
	}
	runDecisionBrowser(t, testServer(t), nil, "tides.spec.ts", "WESTEROS_TIDES_FIXTURES")
}
