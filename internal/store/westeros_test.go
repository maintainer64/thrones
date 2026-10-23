package store

import (
	"path/filepath"
	"reflect"
	"testing"

	"westeros-web/internal/game"
)

func TestWesterosDecksAndRevealSurviveDatabaseReopen(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "westeros.sqlite")
	db, err := Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	r := game.NewRoom("westeros", "Title", "host", "Name")
	if err := r.Apply("host", game.Command{Type: "start"}, game.LoadCatalog()); err != nil {
		t.Fatal(err)
	}
	r.Phase, r.Round = game.PhaseWesteros, 2
	if err := r.RevealWesterosCards(); err != nil {
		t.Fatal(err)
	}
	// Prepare a known non-Winter next draw before saving: round three should
	// consume the persisted order without an independent new random shuffle.
	for i := range r.Westeros.Decks {
		deck := &r.Westeros.Decks[i]
		if deck.Draw[0] == game.WesterosWinter {
			deck.Draw[0], deck.Draw[1] = deck.Draw[1], deck.Draw[0]
		}
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
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.Westeros, r.Westeros) {
		t.Fatal("restart changed unrevealed order, discard or current cards")
	}
	if err := saved.RevealWesterosCards(); err != nil || !reflect.DeepEqual(saved.Westeros, r.Westeros) {
		t.Fatal("re-entering the saved round rerolled cards", err)
	}
	// Continue from the exact saved piles, with no shuffle needed. This also
	// verifies that saving a later round rejects a stale writer.
	expected := [3]game.WesterosCardID{}
	for i, deck := range saved.Westeros.Decks {
		expected[i] = deck.Draw[0]
	}
	saved.Round, saved.Revision = 3, saved.Revision+1
	if err := saved.RevealWesterosCards(); err != nil || saved.Westeros.Current.Cards != expected {
		t.Fatal("next round did not consume the persisted order", err)
	}
	if err := db.Save(saved, r.Revision); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err := db.Save(r, r.Revision-1); err != ErrConflict {
		t.Fatal("stale writer overwrote the deck order", err)
	}
	latest, err := db.Load(r.ID)
	if err != nil || !reflect.DeepEqual(latest.Westeros, saved.Westeros) {
		t.Fatal("persisted next round was overwritten", err)
	}
}
