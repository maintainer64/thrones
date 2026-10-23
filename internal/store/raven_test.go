package store

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestRavenSecretAndDeadlineSurviveSQLiteReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raven.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := game.LoadCatalog()
	r := game.NewRoom("raven", "Raven", "host", "Host")
	now := time.Date(2026, 10, 21, 1, 0, 0, 0, time.UTC)
	if err := r.ApplyAt("host", game.Command{Type: "start"}, c, now); err != nil {
		t.Fatal(err)
	}
	r.Phase = "orders_revealed"
	r.Match.Tracks[string(game.TrackKingsCourt)] = slices.Clone(game.BaseHouses)
	r.Orders = map[string]string{"winterfell": "march", "white-harbor": "march-minus"}
	if err := r.ApplyAt("host", game.Command{Type: "continue"}, c, now); err != nil {
		t.Fatal(err)
	}
	if err := r.ApplyAt("host", game.Command{Type: "raven", Choice: string(game.RavenPeek)}, c, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	deck, deadline, card := slices.Clone(r.Wildlings.Deck), r.Clock.Deadline, r.Raven.Card
	if err := db.Create(r); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r, err = db.Load(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Raven == nil || r.Raven.Card != card || !r.Match.RavenUsed || !r.Clock.Deadline.Equal(deadline) || !slices.Equal(deck, r.Wildlings.Deck) {
		t.Fatal("SQLite reopen lost private Raven state")
	}
	if r.View("host", c).Raven.Card != card || r.View("outsider", c).Raven.Card != "" {
		t.Fatal("reopen changed projection ownership")
	}
	if err := r.ApplyAt("host", game.Command{Type: "raven", Choice: string(game.RavenBottom)}, c, deadline.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err := db.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	final, err := db.Load(r.ID)
	if err != nil || final.Raven != nil || final.Phase == game.PhaseRaven || !final.Match.RavenUsed || final.Wildlings.Deck[len(deck)-1] != card {
		t.Fatal("placement did not survive save")
	}
}
