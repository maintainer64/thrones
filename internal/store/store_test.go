package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestRoomSurvivesRestartAndRejectsStaleSave(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "game.sqlite")
	db, err := Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	r := game.NewRoom("room", "Title", "host", "Name")
	if err = db.Create(r); err != nil {
		t.Fatal(err)
	}
	r.Revision = 2
	r.Orders["winterfell"] = "march"
	if err = db.Save(r, 1); err != nil {
		t.Fatal(err)
	}
	if err = db.Save(r, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale save: %v", err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	saved, err := db.Load("room")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 2 || saved.Orders["winterfell"] != "march" {
		t.Fatalf("room lost: %+v", saved)
	}
}

func TestUnfinishedInfluenceAuctionSurvivesDatabaseReopen(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "auction.sqlite")
	db, err := Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	r := game.NewRoom("auction", "Title", "host", "Name")
	c := game.LoadCatalog()
	now := time.Now().UTC()
	if err = r.ApplyAt("host", game.Command{Type: "start"}, c, now); err != nil {
		t.Fatal(err)
	}
	r.Phase = game.PhaseWesteros
	if err = r.BeginClashOfKings(now); err != nil {
		t.Fatal(err)
	}
	amount := 3
	if err = r.ApplyAt("host", game.Command{Type: "bid", Bid: &amount}, c, now); err != nil {
		t.Fatal(err)
	}
	deadline := r.Clock.Deadline
	if err = db.Create(r); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	saved, err := db.Load(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	view := saved.View("host", c)
	if view.Influence == nil || view.Influence.Bids.YourBid == nil || *view.Influence.Bids.YourBid != 3 || !saved.Clock.Deadline.Equal(deadline) || saved.Match.Power["stark"] != 5 {
		t.Fatal("restart lost secret bid, balance or deadline")
	}
	if saved.View("outsider", c).Influence.Bids.YourBid != nil || saved.View("outsider", c).Influence.Bids.Values != nil {
		t.Fatal("restart exposed secret bid")
	}
	if err := saved.ApplyAt("host", game.Command{Type: "bid", Bid: &amount}, c, now); err == nil {
		t.Fatal("restart allowed bid replacement")
	}
	active, err := db.ActiveActions()
	if err != nil || len(active) != 1 || active[0].Influence.Bids.Values["stark"] != 3 {
		t.Fatal("restarted auction not found by scheduler", err)
	}
}
