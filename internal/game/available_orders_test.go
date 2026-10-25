package game

import (
	"encoding/json"
	"testing"
)

func TestAvailableOrdersUsesCommandValidationWithoutMutation(t *testing.T) {
	r, c := started(t)
	if err := r.Apply("host-secret", Command{Type: "order", Region: "winterfell", Order: "march"}, c); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(r)
	options := r.View("host-secret", c).AvailableOrders
	if !contains(options["winterfell"], "march") || contains(options["white-harbor"], "march") {
		t.Fatal("current token or used token incorrect")
	}
	if _, ok := options["pyke"]; ok {
		t.Fatal("enemy options leaked")
	}
	for region, army := range r.armies(c) {
		owned := false
		for _, unit := range army {
			if unit.House == "stark" && unit.Count > 0 {
				owned = true
			}
		}
		if !owned {
			continue
		}
		for _, order := range Orders {
			copy := r.Clone()
			err := copy.Apply("host-secret", Command{Type: "order", Region: region, Order: order.ID}, c)
			if contains(options[region], order.ID) != (err == nil) {
				t.Fatalf("UI and command disagree on %s: %s: %v", region, order.ID, err)
			}
		}
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("options mutated the room")
	}
}

func TestAvailableOrdersTerrainStarsAndPrivateScope(t *testing.T) {
	r, c := started(t)
	options := r.View("host-secret", c).AvailableOrders
	for _, id := range []string{"power-1", "power-2", "power-star"} {
		if contains(options["the-shivering-sea"], id) {
			t.Fatal("power available at sea")
		}
		if !contains(options["winterfell"], id) {
			t.Fatal("power unavailable on land")
		}
	}
	if err := r.Apply("host-secret", Command{Type: "order", Region: "the-shivering-sea", Order: "power-1"}, c); err == nil {
		t.Fatal("sea power accepted by command")
	}
	// Stark occupies the sixth place: no starred order is available.
	r.Match.Tracks["kingsCourt"] = []string{"greyjoy", "lannister", "baratheon", "tyrell", "martell", "stark"}
	for _, ids := range r.View("host-secret", c).AvailableOrders {
		for _, id := range ids {
			o, _ := FindOrder(id)
			if o.Star {
				t.Fatal("star limit ignored")
			}
		}
	}
	if len(r.View("unknown", c).AvailableOrders) != 0 {
		t.Fatal("options exposed without membership")
	}
	r.Seat("host-secret").Ready = true
	if len(r.View("host-secret", c).AvailableOrders) != 0 {
		t.Fatal("ready player can edit")
	}
	r.Phase = "marches"
	if len(r.View("host-secret", c).AvailableOrders) != 0 {
		t.Fatal("options outside planning")
	}
}
