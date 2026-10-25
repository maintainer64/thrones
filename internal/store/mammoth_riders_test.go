package store

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestMammothCasualtiesVacatedPortsAndCardRewardSurviveSQLiteReopen(t *testing.T) {
	for _, stage := range []string{"loss-choice", "after-land-loss", "last-loss", "next-house", "reward-choice", "reward-complete", "reward-declined"} {
		t.Run(stage, func(t *testing.T) {
			won := strings.HasPrefix(stage, "reward")
			r, c, now := storeWildlingOutcome(t, game.WildlingMammoths, won)
			r.Match.Discard["stark"] = []string{"eddard-stark", "robb-stark"}
			r.Match.Armies = map[string][]game.Unit{
				"white-harbor":         {{Type: "footman", House: "stark", Count: 1}},
				"port-of-white-harbor": {{Type: "ship", House: "stark", Count: 2}},
				"winterfell":           {{Type: "knight", House: "stark", Count: 1}},
			}
			advance := func() {
				t.Helper()
				if !r.AdvanceBot(c, now) {
					t.Fatal("expected next decision")
				}
			}
			choose := func(id string) {
				t.Helper()
				if err := r.ApplyAt("stark", game.Command{Type: "wildling_choice", Choice: id}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			advance()
			if !won && stage != "loss-choice" {
				choose("white-harbor:footman")
			}
			if stage == "last-loss" || stage == "next-house" {
				advance()
				choose("port-of-white-harbor:ship")
				advance()
			}
			if stage == "next-house" {
				choose("port-of-white-harbor:ship")
			}
			if stage == "reward-complete" {
				choose("eddard-stark")
			}
			if stage == "reward-declined" {
				choose(game.WildlingFinishReward)
			}
			filename := filepath.Join(t.TempDir(), "mammoth.sqlite")
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
			if err != nil || !reflect.DeepEqual(r.Wildlings, saved.Wildlings) || !reflect.DeepEqual(r.Match, saved.Match) || !r.Clock.Deadline.Equal(saved.Clock.Deadline) {
				t.Fatal("reopen lost targets, vacant land, quota, card or deadline", err)
			}
			active, err := db.ActiveActions()
			if err != nil || len(active) != 1 {
				t.Fatal("scheduler lost saved decision", err)
			}
			before := saved.Clone()
			pending := stage == "loss-choice" || stage == "last-loss" || stage == "reward-choice"
			if changed := saved.AdvanceBot(c, now.Add(time.Minute)); changed == pending || (!changed && !reflect.DeepEqual(before, saved)) || !reflect.DeepEqual(before.Match.Armies, saved.Match.Armies) {
				t.Fatal("reopen repeated a casualty or skipped a human choice")
			}
			for i := range saved.Seats {
				saved.Seats[i].Bot, saved.Seats[i].Difficulty = true, "normal"
			}
			for step := 0; step < 64 && saved.Wildlings.Attack != nil; step++ {
				if !saved.AdvanceBot(c, now) {
					t.Fatal("restored effect stalled")
				}
			}
			if saved.Wildlings.Attack != nil || saved.Wildlings.Last == nil || saved.Wildlings.Last.Card != game.WildlingMammoths {
				t.Fatal("final report missing")
			}
			if won {
				want := []string{"robb-stark"}
				if stage == "reward-declined" {
					want = []string{"eddard-stark", "robb-stark"}
				}
				if !reflect.DeepEqual(saved.Match.Discard["stark"], want) {
					t.Fatal("reward replayed or ignored a decline")
				}
			} else if len(saved.Match.Armies["winterfell"]) != 1 || len(saved.Match.Armies["port-of-white-harbor"]) != 0 || len(saved.Match.Armies["white-harbor"]) != 0 {
				t.Fatal("port cleanup caused extra casualties on reload")
			}
		})
	}
}
