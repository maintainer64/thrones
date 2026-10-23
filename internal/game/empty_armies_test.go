package game

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicEmptyArmiesAreArraysWithoutMutatingSavedState(t *testing.T) {
	r, c, _ := fixture(t)
	r.Match.Armies["winterfell"] = nil
	view := r.View("host", c)
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"winterfell":null`) || view.Match.Armies["winterfell"] == nil {
		t.Fatal("empty army must serialize as an iterable array")
	}
	if r.Match.Armies["winterfell"] != nil {
		t.Fatal("projection changed persisted state")
	}
}

func TestOrderBudgetUsesCourtStarsNotPowerAndCountsOwnTokensOnly(t *testing.T) {
	r, c := started(t)
	power := r.Match.Power["stark"]
	if err := r.Apply("host-secret", Command{Type: "order", Region: "winterfell", Order: "defend-star"}, c); err != nil {
		t.Fatal(err)
	}
	budget := r.View("host-secret", c).OrderBudget
	if budget == nil || budget.Power != power || budget.StarsUsed != 1 || budget.StarsAllowed != StarLimit("stark", c) || r.Match.Power["stark"] != power {
		t.Fatal("incorrect power/star allowance", budget)
	}
}
