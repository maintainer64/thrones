package game

import (
	"encoding/json"
	"testing"
)

func TestRetreatUsesLoserNotActiveHouseAndRejectsInvalidTargets(t *testing.T) {
	r, c, now := fixture(t)
	r.Phase = "retreat"
	r.Match.ActiveHouse = "greyjoy"
	r.Match.Armies["moat-cailin"] = []Unit{{Type: "footman", House: "greyjoy", Count: 1}}
	r.Match.Control["moat-cailin"] = "greyjoy"
	r.Battle = &Battle{From: "greywater-watch", To: "moat-cailin", Attacker: "greyjoy", Defender: "stark", Loser: "stark", Survivors: []Unit{{Type: "footman", House: "stark", Count: 1}}}
	view := r.View("host", c)
	if !contains(view.Battle.Retreats, "winterfell") || contains(view.Battle.Retreats, "greywater-watch") {
		t.Fatalf("wrong legal retreat targets: %+v", view.Battle.Retreats)
	}
	for _, cmd := range []struct{ house, target string }{{"stark", "sunspear"}, {"stark", "greywater-watch"}, {"greyjoy", "winterfell"}} {
		before, _ := json.Marshal(r)
		if err := r.action(cmd.house, Command{Type: "retreat", Target: cmd.target}, c, now); err == nil {
			t.Fatal("invalid retreat accepted", cmd)
		}
		after, _ := json.Marshal(r)
		if string(before) != string(after) {
			t.Fatal("invalid retreat changed the room")
		}
	}
	if err := r.ApplyAt("host", Command{Type: "retreat", Target: "winterfell"}, c, now); err != nil {
		t.Fatal(err)
	}
	if count(r.Match.Armies["winterfell"]) != 3 || r.Battle != nil || r.Phase == "retreat" {
		t.Fatal("survivors did not retreat or turn did not finish")
	}
}
