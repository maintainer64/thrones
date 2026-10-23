package store

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestWesterosContinuationSurvivesReopenAtEveryPause(t *testing.T) {
	for _, kind := range []string{"threat", "choice", "supply", "mustering", "auction", "attack"} {
		t.Run(kind, func(t *testing.T) {
			c := game.LoadCatalog()
			r := game.NewRoom("resume", "Title", "host", "Name")
			if err := r.Apply("host", game.Command{Type: "start"}, c); err != nil {
				t.Fatal(err)
			}
			for i := range r.Seats {
				r.Seats[i].Bot = false
			}
			cards := [3]game.WesterosCardID{game.WesterosThroneOfBlades, game.WesterosLastDays, game.WesterosSeaOfStorms}
			steps := 2
			switch kind {
			case "threat":
				steps = 0
			case "supply":
				cards[0] = game.WesterosSupply
				r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "footman", House: "stark", Count: 5}}}
				r.Match.Control = map[string]string{"winterfell": "stark"}
			case "mustering":
				cards[0] = game.WesterosMustering
			case "auction":
				cards[0], cards[1], steps = game.WesterosLastDays, game.WesterosClashOfKings, 3
			case "attack":
				cards[0], r.Westeros.Threat, steps = game.WesterosLastDays, 10, 1
			}
			for i, card := range cards {
				deck := &r.Westeros.Decks[i]
				index := slices.Index(deck.Draw, card)
				deck.Draw[0], deck.Draw[index] = deck.Draw[index], deck.Draw[0]
			}
			r.Round, r.Phase = 2, game.PhaseWesteros
			now := time.Now().UTC()
			if err := r.BeginWesteros(now); err != nil {
				t.Fatal(err)
			}
			for step := 0; step < steps; step++ {
				if !r.AdvanceBot(c, now) {
					t.Fatal("test did not reach the requested pause", kind)
				}
			}
			filename := filepath.Join(t.TempDir(), "resume.sqlite")
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
			if err != nil || !reflect.DeepEqual(saved.Westeros, r.Westeros) || saved.Phase != r.Phase || !saved.Clock.Deadline.Equal(r.Clock.Deadline) {
				t.Fatal("reopen lost cursor, threat, pending choice, nested phase or deadline", err)
			}
			active, err := db.ActiveActions()
			if err != nil || len(active) != 1 || active[0].ID != r.ID {
				t.Fatal("scheduler cannot find the saved continuation", err)
			}
			if changed := saved.AdvanceBot(c, now); changed != (kind == "threat" || kind == "attack") {
				t.Fatal("restart skipped a human decision/attack or did not resume automatic threat")
			}
			if kind == "attack" && (saved.Phase != game.PhaseWildlingBidding || saved.Wildlings.Attack == nil) {
				t.Fatal("pending attack did not resume into real sealed contributions")
			}
			if !reflect.DeepEqual(saved.Westeros.Current, r.Westeros.Current) {
				t.Fatal("restart rerolled the revealed cards")
			}
		})
	}
}
