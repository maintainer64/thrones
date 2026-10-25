package server

import (
	"os"
	"testing"
)

func TestOrdersBrowser(t *testing.T) {
	if os.Getenv("WESTEROS_BROWSER_TEST") != "1" {
		t.Skip("opt-in composed order tokens and game symbols")
	}
	runDecisionBrowser(t, testServer(t), nil, "orders.spec.ts", "WESTEROS_ORDERS_FIXTURES")
}
