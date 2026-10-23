package store

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestMilkwaterFAQOriginalTargetsAndRecoveredHandSurviveSQLiteReopen(t *testing.T) {
	for _, stage := range []string{"first-choice", "between-discards", "last-choice", "recovered-hand"} {
		t.Run(stage, func(t *testing.T) {
			r, c, now := storeWildlingEffect(t, game.WildlingMilkwater)
			for id := range c.Houses["stark"].Cards {
				if id != "roose-bolton" && id != "greatjon-umber" {
					r.Match.Discard["stark"] = append(r.Match.Discard["stark"], id)
				}
			}
			slices.Sort(r.Match.Discard["stark"])
			if !r.AdvanceBot(c, now) {
				t.Fatal("initial discard choice was not offered")
			}
			if stage != "first-choice" {
				if err := r.ApplyAt("stark", game.Command{Type: "wildling_choice", Choice: "roose-bolton"}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "last-choice" || stage == "recovered-hand" {
				if !r.AdvanceBot(c, now.Add(time.Minute)) {
					t.Fatal("last original target was skipped")
				}
			}
			if stage == "recovered-hand" {
				if err := r.ApplyAt("stark", game.Command{Type: "wildling_choice", Choice: "greatjon-umber"}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			filename := filepath.Join(t.TempDir(), "milkwater.sqlite")
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
				t.Fatal("reopen lost original penalty targets, recovered cards, cursor or deadline", err)
			}
			active, err := db.ActiveActions()
			if err != nil || len(active) != 1 || active[0].Phase != game.PhaseWildlingEffects {
				t.Fatal("scheduler lost the saved hand decision", err)
			}
			before := saved.Clone()
			changed := saved.AdvanceBot(c, now.Add(2*time.Minute))
			wantChanged := stage == "between-discards" || stage == "recovered-hand"
			if changed != wantChanged || (!changed && !reflect.DeepEqual(before, saved)) || !reflect.DeepEqual(before.Match.Discard, saved.Match.Discard) {
				t.Fatal("reload re-discarded cards, bypassed a human or failed to offer the next decision")
			}
			if stage == "first-choice" {
				if err := saved.ApplyAt("stark", game.Command{Type: "wildling_choice", Choice: "roose-bolton"}, c, now); err != nil {
					t.Fatal(err)
				}
			}
			for i := range saved.Seats {
				saved.Seats[i].Bot = true
			}
			for step := 0; step < 32 && saved.Wildlings.Attack != nil; step++ {
				if !saved.AdvanceBot(c, now) {
					t.Fatal("restored effect could not finish")
				}
			}
			if saved.Wildlings.Attack != nil || !slices.Equal(saved.Match.Discard["stark"], []string{"greatjon-umber"}) || saved.Wildlings.Last.Card != game.WildlingMilkwater || saved.Westeros.Threat != 8 {
				t.Fatal("recovered hand was penalized again or final report was lost")
			}
			saved.Revision++
			if err := db.Save(saved, saved.Revision-1); err != nil {
				t.Fatal(err)
			}
			final, err := db.Load(r.ID)
			if err != nil || !reflect.DeepEqual(saved.Wildlings, final.Wildlings) || !reflect.DeepEqual(saved.Match.Discard, final.Match.Discard) {
				t.Fatal("final report and hand did not persist", err)
			}
		})
	}
}
