package game

import (
	"encoding/json"
	"testing"
)

func TestChooseAnyHouseAndStartSolo(t *testing.T) {
	c := LoadCatalog()
	for _, house := range BaseHouses {
		t.Run(house, func(t *testing.T) {
			r := NewRoom("lobby", "Solo", "host", "Player")
			if err := r.Apply("host", Command{Type: "choose_house", House: house}, c); err != nil {
				t.Fatal(err)
			}
			if r.Seat("host").House != house || r.Host != "host" {
				t.Fatal("wrong house or host changed")
			}
			for _, seat := range r.Seats {
				if seat.House != house && (!seat.Bot || seat.Player != "" || seat.Name == "Player") {
					t.Fatalf("old seat not returned to bot: %+v", seat)
				}
			}
			if err := r.Apply("host", Command{Type: "start"}, c); err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(r)
			if err := r.Apply("host", Command{Type: "choose_house", House: "stark"}, c); err == nil {
				t.Fatal("changed house after start")
			}
			after, _ := json.Marshal(r)
			if string(before) != string(after) {
				t.Fatal("rejected choice mutated the room")
			}
		})
	}
}

func TestGuestsChooseHousesWithoutDisplacingHumans(t *testing.T) {
	c := LoadCatalog()
	r := NewRoom("lobby", "Multi", "host", "Host")
	if err := r.Join("guest", "Guest", "martell"); err != nil {
		t.Fatal(err)
	}
	if r.Seat("guest").House != "martell" || r.Seat("guest").Bot {
		t.Fatal("did not take selected bot seat")
	}
	for _, house := range []string{"stark", "unknown"} {
		before, _ := json.Marshal(r)
		if err := r.Apply("guest", Command{Type: "choose_house", House: house}, c); err == nil {
			t.Fatal("accepted occupied or unknown house")
		}
		after, _ := json.Marshal(r)
		if string(before) != string(after) {
			t.Fatal("failed choice mutated room")
		}
	}
	if err := r.Join("intruder", "Intruder", "martell"); err == nil {
		t.Fatal("displaced guest")
	}
	if err := r.Apply("guest", Command{Type: "choose_house", House: "tyrell"}, c); err != nil {
		t.Fatal(err)
	}
	if r.Seat("guest").House != "tyrell" || !r.houseSeat("martell").Bot {
		t.Fatal("guest move did not free original seat")
	}
	if err := r.Apply("host", Command{Type: "configure", House: "greyjoy", Bot: false, Difficulty: "hard"}, c); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply("guest", Command{Type: "choose_house", House: "greyjoy"}, c); err != nil {
		t.Fatal(err)
	}
	if r.houseSeat("tyrell").Bot || r.houseSeat("tyrell").Player != "" {
		t.Fatal("open seat policy not preserved")
	}
	if err := r.Apply("outsider", Command{Type: "choose_house", House: "martell"}, c); err == nil {
		t.Fatal("outsider chose house")
	}
}
