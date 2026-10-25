package store

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"westeros-web/internal/game"
)

func TestHordeCastleLockAndNestedMusterSurviveSQLiteReopen(t *testing.T) {
	for _, stage := range []string{"loss-choice", "loss-locked", "reward-choice", "reward-selected", "reward-mustering", "reward-spent", "reward-declined"} {
		t.Run(stage, func(t *testing.T) {
			won := strings.HasPrefix(stage, "reward")
			r, c, now := storeWildlingOutcome(t, game.WildlingHorde, won)
			r.Match.Armies = map[string][]game.Unit{
				"winterfell": {{Type: "footman", House: "stark", Count: 2}},
			}
			r.Match.Control = map[string]string{"winterfell": "stark"}
			r.Match.Supply["stark"] = 6
			advance := func() {
				t.Helper()
				if !r.AdvanceBot(c, now) {
					t.Fatal("decision did not advance")
				}
			}
			choose := func(option string) {
				t.Helper()
				if err := r.ApplyAt("stark", game.Command{Type: "wildling_choice", Choice: option}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			advance()
			if stage == "loss-locked" {
				choose("winterfell:footman")
				advance()
			}
			if stage == "reward-selected" || stage == "reward-mustering" || stage == "reward-spent" {
				choose("winterfell")
			}
			if stage == "reward-mustering" || stage == "reward-spent" {
				advance()
			}
			if stage == "reward-spent" {
				if err := r.ApplyAt("stark", game.Command{Type: "muster", Region: "winterfell", Target: "winterfell", Unit: game.UnitKnight, Upgrade: true}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "reward-declined" {
				choose(game.WildlingFinishReward)
			}
			filename := filepath.Join(t.TempDir(), "horde.sqlite")
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
			if err != nil || !reflect.DeepEqual(saved.Wildlings, r.Wildlings) || !reflect.DeepEqual(saved.Mustering, r.Mustering) || !reflect.DeepEqual(saved.Match, r.Match) || !saved.Clock.Deadline.Equal(r.Clock.Deadline) {
				t.Fatal("reopen lost region lock, reward, points or deadline", err)
			}
			active, err := db.ActiveActions()
			if err != nil || len(active) != 1 {
				t.Fatal("scheduler lost nested phase", err)
			}
			before := saved.Clone()
			waiting := stage != "reward-selected" && stage != "reward-declined"
			if waiting && (saved.AdvanceBot(c, now) || saved.Expire(saved.Clock.Deadline, c) || !reflect.DeepEqual(before, saved)) {
				t.Fatal("reload bypassed a human decision")
			}
			for i := range saved.Seats {
				saved.Seats[i].Bot, saved.Seats[i].Difficulty = true, "normal"
			}
			for i := 0; i < 64 && saved.Wildlings.Attack != nil; i++ {
				if !saved.AdvanceBot(c, now) {
					t.Fatal("restored effect stalled", saved.Phase)
				}
			}
			if saved.Wildlings.Attack != nil || saved.Wildlings.Last == nil || saved.Wildlings.Last.Card != game.WildlingHorde || saved.Mustering != nil {
				t.Fatal("nested effect did not return to original attack")
			}
			if !won && len(saved.Match.Armies["winterfell"]) != 0 {
				t.Fatal("wrong casualty quota after reopen")
			}
			if stage == "reward-declined" && !reflect.DeepEqual(before.Match.Armies, saved.Match.Armies) {
				t.Fatal("declined reward repeated")
			}
		})
	}
}
