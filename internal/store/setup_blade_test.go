package store

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"westeros-web/internal/game"
)

func TestThreeHouseSetupAndOrdinaryBladeSurviveSQLiteReopen(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "setup-blade.sqlite")
	db, err := Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	r := game.NewRoom("setup-blade", "Setup", "host", "Host")
	c := game.LoadCatalog()
	now := time.Now().UTC()
	s := game.DefaultSettings()
	s.PlayerCount = 3
	if err := r.ApplyAt("host", game.Command{Type: "settings", Settings: &s}, c, now); err != nil {
		t.Fatal(err)
	}
	if err := r.ApplyAt("host", game.Command{Type: "start"}, c, now); err != nil {
		t.Fatal(err)
	}
	for i := range r.Seats {
		r.Seats[i].Bot = false
		if r.Seats[i].House == "lannister" {
			r.Seats[i].Player = "guest"
		}
	}
	r.Match.Supply["lannister"] = 6
	r.Match.Armies["winterfell"] = nil
	r.Phase = "combat"
	r.Match.ActiveHouse = "lannister"
	r.Orders = map[string]string{"stoney-sept": "march-minus"}
	r.Battle = &game.Battle{From: "white-harbor", To: "winterfell", Attacker: "lannister", Defender: "stark", Army: []game.Unit{{Type: "footman", House: "lannister", Count: 2}}, Cards: map[string]string{}}
	for player, card := range map[string]string{"host": "catelyn-stark", "guest": "ser-jaime-lannister"} {
		if err := r.ApplyAt(player, game.Command{Type: "card", Card: card}, c, now); err != nil {
			t.Fatal(err)
		}
	}
	if r.Battle.Choice == nil || r.Battle.Choice.Kind != game.ChoiceBlade {
		t.Fatal("missing ordinary Blade choice")
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
	if saved.Settings.PlayerCount != 3 || len(saved.Seats) != 3 || !reflect.DeepEqual(r.Battle, saved.Battle) || !saved.Clock.Deadline.Equal(r.Clock.Deadline) || !reflect.DeepEqual(r.Match.BlockedRegions, saved.Match.BlockedRegions) {
		t.Fatal("setup/Blade/deadline changed after restart")
	}
	if err := saved.ApplyAt("host", game.Command{Type: "ability", Choice: "strength"}, c, now); err != nil {
		t.Fatal(err)
	}
	if !saved.Match.BladeUsed || saved.Match.LastCombat.BladeBonus != "stark" || saved.Match.LastCombat.Defense != 3 || saved.Match.Garrisons["winterfell"] != 0 {
		t.Fatal("Blade or garrison result lost", saved.Match.LastCombat)
	}
	saved.Revision++
	if err := db.Save(saved, r.Revision); err != nil {
		t.Fatal(err)
	}
	reloaded, err := db.Load(saved.ID)
	want, _ := json.Marshal(saved.Match.LastCombat)
	got, _ := json.Marshal(reloaded.Match.LastCombat)
	if err != nil || string(want) != string(got) || reloaded.Match.Garrisons["winterfell"] != 0 {
		t.Fatal("defeated garrison or result did not persist", err)
	}
}
