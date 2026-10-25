package game

import (
	"encoding/json"
	"testing"
)

func TestActionPreviewMatchesValidatorWithoutChangingRoom(t *testing.T) {
	r, catalog, now := fixture(t)
	command := Command{Type: "march", Region: "winterfell", Units: map[string]int{"footman": 1}}
	before, _ := json.Marshal(r)
	result, err := r.PreviewAction("host", command, catalog, now)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Targets, "castle-black") || contains(result.Targets, "sunspear") || contains(result.Targets, "the-shivering-sea") {
		t.Fatal("wrong destinations", result.Targets)
	}
	for _, target := range catalog.Boards["westeros"].Regions {
		candidate := command
		candidate.Target = target.ID
		valid := r.Clone().ApplyAt("host", candidate, catalog, now) == nil
		if valid != contains(result.Targets, target.ID) {
			t.Fatal("preview differs from validator", target.ID)
		}
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("preview mutated live room")
	}
}

func TestActionPreviewRejectsUnknownPlayerSecretsAndOtherCommands(t *testing.T) {
	r, catalog, now := fixture(t)
	if _, err := r.PreviewAction("stranger", Command{Type: "march"}, catalog, now); err == nil {
		t.Fatal("outsider accepted")
	}
	for _, command := range []Command{{Type: "settings"}, {Type: "pass"}, {Type: "march", Region: "sunspear"}} {
		if _, err := r.PreviewAction("host", command, catalog, now); err == nil {
			t.Fatal("invalid preview accepted", command)
		}
	}
	r.Phase = "planning"
	if _, err := r.PreviewAction("host", Command{Type: "raid", Region: "winterfell"}, catalog, now); err == nil {
		t.Fatal("secret planning phase can be probed")
	}
}

func TestActionPreviewUsesChosenQuantitiesAndFleetTransport(t *testing.T) {
	r, catalog, now := fixture(t)
	for _, amount := range []int{-1, 0, 3} {
		result, err := r.PreviewAction("host", Command{Type: "march", Region: "winterfell", Units: map[string]int{"footman": amount}}, catalog, now)
		if err != nil || len(result.Targets) != 0 {
			t.Fatal("impossible troop selection has destinations", amount, result, err)
		}
	}
	r.Match.Armies["the-shivering-sea"] = []Unit{{Type: "ship", House: "stark", Count: 1}}
	r.Match.Armies["the-narrow-sea"] = []Unit{{Type: "ship", House: "stark", Count: 1}}
	result, err := r.PreviewAction("host", Command{Type: "march", Region: "winterfell", Units: map[string]int{"footman": 1}}, catalog, now)
	if err != nil || !contains(result.Targets, "the-fingers") {
		t.Fatal("fleet transport missing from preview", result, err)
	}
}
