package store

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"westeros-web/internal/game"
)

func TestPreemptivePenaltyAndRepeatAttackSurviveSQLiteReopen(t *testing.T) {
	for _, stage := range []string{"penalty", "units-selected", "unit-choice", "after-unit", "track-selected", "track-tie", "track-applied", "repeat-bidding", "repeat-commit", "repeat-tie", "repeat-effects"} {
		t.Run(stage, func(t *testing.T) {
			won := strings.HasPrefix(stage, "repeat")
			r, c, now := storeWildlingOutcome(t, game.WildlingPreemptive, won)
			r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "footman", House: "stark", Count: 3}}}
			r.Match.Tracks["ironThrone"] = []string{"greyjoy", "stark", "lannister", "baratheon", "tyrell", "martell"}
			r.Match.Tracks["fiefdoms"] = slices.Clone(r.Match.Tracks["ironThrone"])
			r.Match.Tracks["kingsCourt"] = []string{"greyjoy", "lannister", "stark", "baratheon", "tyrell", "martell"}
			// The repeat card has a deterministic, automatic effect for reopen.
			index := slices.Index(r.Wildlings.Deck, game.WildlingSilence)
			r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
			advance := func() {
				t.Helper()
				if !r.AdvanceBot(c, now) {
					t.Fatal("expected one durable step", r.Phase)
				}
			}
			choose := func(house, option string) {
				t.Helper()
				if err := r.ApplyAt(house, game.Command{Type: "wildling_choice", Choice: option}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			advance()
			if !won && stage != "penalty" {
				option := string(game.PreemptiveDestroyUnits)
				if strings.HasPrefix(stage, "track") {
					option = string(game.PreemptiveLoseInfluence)
				}
				choose("stark", option)
			}
			if stage == "unit-choice" || stage == "after-unit" || stage == "track-tie" || stage == "track-applied" {
				advance()
			}
			if stage == "after-unit" {
				choose("stark", "winterfell:footman")
			}
			if stage == "track-applied" {
				choose("greyjoy", "fiefdoms")
			}
			if stage == "repeat-commit" || stage == "repeat-tie" || stage == "repeat-effects" {
				for _, house := range slices.Clone(r.Wildlings.Attack.Bids.Participants) {
					zero := 0
					if err := r.ApplyAt(house, game.Command{Type: "wildling_bid", Bid: &zero}, c, now); err != nil {
						t.Fatal(err)
					}
					if stage == "repeat-commit" {
						break
					}
				}
			}
			if stage == "repeat-effects" {
				if err := r.ApplyAt("greyjoy", game.Command{Type: "wildling_tie", Target: "greyjoy"}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			filename := filepath.Join(t.TempDir(), "preemptive.sqlite")
			db, err := Open(filename)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Create(r); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = Open(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			saved, err := db.Load(r.ID)
			if err != nil || !reflect.DeepEqual(saved.Wildlings, r.Wildlings) || !reflect.DeepEqual(saved.Westeros, r.Westeros) || !reflect.DeepEqual(saved.Match, r.Match) || !saved.Clock.Deadline.Equal(r.Clock.Deadline) {
				t.Fatal("reopen lost quota, actor, exclusion, bids, cursor or clock", err)
			}
			active, err := db.ActiveActions()
			if err != nil || len(active) != 1 {
				t.Fatal("scheduler lost pending phase", err)
			}
			before := saved.Clone()
			waiting := stage == "penalty" || stage == "unit-choice" || stage == "track-tie" || stage == "repeat-bidding" || stage == "repeat-commit" || stage == "repeat-tie"
			if waiting && (saved.AdvanceBot(c, now) || saved.Expire(saved.Clock.Deadline, c) || !reflect.DeepEqual(saved, before)) {
				t.Fatal("reopening bypassed a human decision")
			}
			for i := range saved.Seats {
				saved.Seats[i].Bot, saved.Seats[i].Difficulty = true, "normal"
			}
			for step := 0; step < 128 && saved.Wildlings.Attack != nil; step++ {
				if !saved.AdvanceBot(c, now) {
					t.Fatal("restored continuation stalled", saved.Phase)
				}
			}
			if saved.Wildlings.Attack != nil || saved.Wildlings.Last == nil || saved.Round != before.Round || saved.Westeros.Run.NextCard != before.Westeros.Run.NextCard || len(saved.Wildlings.Deck) != 9 {
				t.Fatal("continuation advanced the cursor or lost the deck")
			}
			if won && (saved.Wildlings.Last.ExcludedHouse != "stark" || saved.Match.Power["stark"] != before.Match.Power["stark"]) {
				t.Fatal("exemption lost or first bids debited again")
			}
			if stage == "unit-choice" || stage == "units-selected" || stage == "after-unit" {
				total := 0
				for _, unit := range saved.Match.Armies["winterfell"] {
					total += unit.Count
				}
				if total != 1 {
					t.Fatal("casualty quota restarted after reopen")
				}
			}
			if stage == "track-applied" && !reflect.DeepEqual(before.Match.Tracks, saved.Match.Tracks) {
				t.Fatal("track penalty applied twice")
			}
		})
	}
}
