package server

import (
	"testing"
	"time"
	"westeros-web/internal/game"
)

func TestBotsAdvanceOneDecisionWithoutBrowsers(t *testing.T) {
	s := testServer(t)
	view, _ := createRoom(t, s)
	room, _ := s.Store.Load(view.ID)
	now := time.Now().UTC()
	for _, command := range []game.Command{{Type: "start"}, {Type: "ready"}, {Type: "continue"}} {
		if err := room.ApplyAt(room.Host, command, s.Catalog, now); err != nil {
			t.Fatal(err)
		}
	}
	if room.Match.ActiveHouse == "stark" {
		t.Fatal("fixture expected first action from a bot")
	}
	room.Revision++
	if err := s.Store.Save(room, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Store.Load(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 3 {
		t.Fatalf("wanted exactly one persisted bot decision: revision %d", saved.Revision)
	}
	// Human decisions must never be supplied by the bot scheduler.
	saved.Phase = "marches"
	saved.Battle = nil
	saved.Match.ActiveHouse = "stark"
	saved.Orders = map[string]string{"winterfell": "march"}
	saved.Revision++
	if err = s.Store.Save(saved, 3); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	saved, err = s.Store.Load(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 4 {
		t.Fatal("scheduler played for a human")
	}
}

func TestSchedulerFindsAndPersistsCardEffectDecisions(t *testing.T) {
	s := testServer(t)
	view, _ := createRoom(t, s)
	r, _ := s.Store.Load(view.ID)
	now := time.Now().UTC()
	if err := r.ApplyAt(r.Host, game.Command{Type: "start"}, s.Catalog, now); err != nil {
		t.Fatal(err)
	}
	r.Phase = "card_effect"
	r.Match.ActiveHouse = "lannister"
	r.Battle = &game.Battle{From: "lannisport", To: "white-harbor", Attacker: "lannister", Defender: "stark", Cards: map[string]string{"lannister": "tyrion-lannister", "stark": "eddard-stark"}, Revealed: true, Stage: "before", Done: map[string]bool{"tyrion": true}, Choice: &game.CardChoice{House: "lannister", Card: "tyrion-lannister", Kind: "confirm", Options: []string{"cancel"}, Optional: true}}
	r.Revision++
	if err := s.Store.Save(r, view.Revision); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Store.Load(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != r.Revision+1 || saved.Battle.Choice == nil || saved.Battle.Choice.House != "stark" || saved.Battle.Choice.Kind != "replace" {
		t.Fatal("scheduler skipped card effect", saved.Phase, saved.Revision)
	}
}
