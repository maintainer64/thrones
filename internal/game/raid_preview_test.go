package game

import (
	"encoding/json"
	"testing"
)

func TestEmptyRaidPreviewCanBePassedWithoutMovingArmies(t *testing.T) {
	r, c, now := fixture(t)
	r.Phase = "raids"
	r.Match.Armies = map[string][]Unit{"moat-cailin": {{Type: "footman", House: "stark", Count: 1}}, "white-harbor": {{Type: "footman", House: "stark", Count: 1}}}
	r.Match.Control = map[string]string{"moat-cailin": "stark", "white-harbor": "stark"}
	r.Orders = map[string]string{"moat-cailin": "raid-1", "white-harbor": "march"}
	before, _ := json.Marshal(r)
	preview, err := r.PreviewAction("host", Command{Type: "raid", Region: "moat-cailin"}, c, now)
	if err != nil || len(preview.Targets) != 0 {
		t.Fatalf("expected valid empty preview: %+v, %v", preview, err)
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("empty preview modified the game")
	}
	armies, _ := json.Marshal(r.Match.Armies)
	if err := r.ApplyAt("host", Command{Type: "pass", Region: "moat-cailin"}, c, now); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Orders["moat-cailin"]; ok {
		t.Fatal("raid order still present")
	}
	if r.Phase != "marches" || r.Match.ActiveHouse != "stark" || r.Orders["white-harbor"] != "march" {
		t.Fatalf("turn did not progress: %s %s %+v", r.Phase, r.Match.ActiveHouse, r.Orders)
	}
	unchanged, _ := json.Marshal(r.Match.Armies)
	if string(armies) != string(unchanged) {
		t.Fatal("passing moved or removed troops")
	}
}

func TestRaidPreviewOnlyOffersRemovableEnemyOrders(t *testing.T) {
	for _, tc := range []struct {
		name, raid, target string
		valid              bool
	}{
		{"support", "raid-1", "support-1", true},
		{"march is immune", "raid-1", "march", false},
		{"normal cannot remove defense", "raid-1", "defend-1", false},
		{"star can remove defense", "raid-star", "defend-1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, c, now := fixture(t)
			r.Phase = "raids"
			r.Match.Armies = map[string][]Unit{"winterfell": {{Type: "footman", House: "stark", Count: 1}}, "white-harbor": {{Type: "footman", House: "lannister", Count: 1}}}
			r.Match.Control = map[string]string{"winterfell": "stark", "white-harbor": "lannister"}
			r.Orders = map[string]string{"winterfell": tc.raid, "white-harbor": tc.target}
			preview, err := r.PreviewAction("host", Command{Type: "raid", Region: "winterfell"}, c, now)
			if err != nil {
				t.Fatal(err)
			}
			if contains(preview.Targets, "white-harbor") != tc.valid {
				t.Fatalf("wrong raid targets: %+v", preview.Targets)
			}
		})
	}
}
