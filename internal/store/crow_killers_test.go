package store

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestCrowKillersRewardAndLossQuotasSurviveSQLiteReopen(t *testing.T) {
	for _, stage := range []string{"destroy-choice", "between-losses", "replace-choice", "next-house", "reward-choice", "after-upgrade", "reward-finished"} {
		t.Run(stage, func(t *testing.T) {
			won := strings.HasPrefix(stage, "reward") || stage == "after-upgrade"
			r, c, now := storeWildlingOutcome(t, game.WildlingCrowKillers, won)
			knights, footmen := 3, 9
			if won {
				knights, footmen = 0, 3
			}
			r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "knight", House: "stark", Count: knights}, {Type: "footman", House: "stark", Count: footmen}}}
			advance := func() {
				t.Helper()
				if !r.AdvanceBot(c, now) {
					t.Fatal("expected the next saved choice")
				}
			}
			choose := func(option string) {
				t.Helper()
				if err := r.ApplyAt("stark", game.Command{Type: "wildling_choice", Choice: option}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			advance()
			if stage != "destroy-choice" && stage != "reward-choice" {
				choose("winterfell")
			}
			if stage == "replace-choice" || stage == "next-house" {
				advance()
				choose("winterfell")
				advance()
			}
			if stage == "next-house" {
				choose("winterfell")
			}
			if stage == "reward-finished" {
				advance()
				choose(game.WildlingFinishReward)
			}
			filename := filepath.Join(t.TempDir(), "crow-killers.sqlite")
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
			if err != nil || !reflect.DeepEqual(r.Wildlings, saved.Wildlings) || !reflect.DeepEqual(r.Match, saved.Match) || !reflect.DeepEqual(r.Westeros, saved.Westeros) || !r.Clock.Deadline.Equal(saved.Clock.Deadline) {
				t.Fatal("reopen lost unit quota, shortage, armies, queue or deadline", err)
			}
			active, err := db.ActiveActions()
			if err != nil || len(active) != 1 {
				t.Fatal("scheduler lost the pending unit resolution", err)
			}
			before := saved.Clone()
			changed := saved.AdvanceBot(c, now.Add(time.Minute))
			pending := stage == "destroy-choice" || stage == "replace-choice" || stage == "reward-choice"
			if changed == pending || (!changed && !reflect.DeepEqual(before, saved)) || !reflect.DeepEqual(before.Match.Armies, saved.Match.Armies) {
				t.Fatal("reopen bypassed a human, repeated a unit effect or lost its continuation")
			}
			for i := range saved.Seats {
				saved.Seats[i].Bot, saved.Seats[i].Difficulty = true, "normal"
			}
			for step := 0; step < 64 && saved.Wildlings.Attack != nil; step++ {
				if !saved.AdvanceBot(c, now) {
					t.Fatal("restored unit effect could not finish")
				}
			}
			gotKnights, gotFootmen := 0, 0
			for _, unit := range saved.Match.Armies["winterfell"] {
				if unit.Type == "knight" {
					gotKnights += unit.Count
				} else if unit.Type == "footman" {
					gotFootmen += unit.Count
				}
			}
			wantKnights, wantFootmen := 0, 10
			if won {
				wantKnights, wantFootmen = 2, 1
				if stage == "reward-finished" {
					wantKnights, wantFootmen = 1, 2
				}
			}
			if saved.Wildlings.Attack != nil || saved.Wildlings.Last == nil || saved.Wildlings.Last.Card != game.WildlingCrowKillers || gotKnights != wantKnights || gotFootmen != wantFootmen {
				t.Fatal("quota was reset on reload or a declined reward resumed", gotKnights, gotFootmen)
			}
		})
	}
}
