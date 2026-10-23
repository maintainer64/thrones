package server

import (
	"os"
	"testing"
)

func TestArtworkBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in original PNGs and house portrait UI")
	}
	runDecisionBrowser(t, testServer(t), nil, "artwork.spec.ts", "WESTEROS_ARTWORK_FIXTURES")
}
