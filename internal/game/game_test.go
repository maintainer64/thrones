package game

import (
	"encoding/json"
	"strings"
	"testing"
)

func started(t *testing.T) (*Room, Catalog) {
	t.Helper()
	c := LoadCatalog()
	r := NewRoom("test-room", "Тест", "host-secret", "Игрок")
	if err := r.Apply("host-secret", Command{Type: "start"}, c); err != nil {
		t.Fatal(err)
	}
	return r, c
}

func TestCatalogGraphAndArmies(t *testing.T) {
	c := LoadCatalog()
	b := c.Boards["westeros"]
	ids := map[string]bool{}
	for _, region := range b.Regions {
		ids[region.ID] = true
	}
	for _, region := range b.Regions {
		seen := map[string]bool{}
		for _, neighbor := range region.Neighbors {
			if !ids[neighbor] || seen[neighbor] || neighbor == region.ID {
				t.Errorf("invalid edge %s -> %q", region.ID, neighbor)
			}
			seen[neighbor] = true
		}
	}
	for region, army := range c.Units {
		if !ids[region] {
			t.Errorf("unknown army region %s", region)
		}
		for _, u := range army {
			if u.Count <= 0 || u.House == "arryn" || u.House == "targaryen" {
				t.Errorf("non-base army: %+v", u)
			}
		}
	}
	if len(c.Houses["stark"].Cards) != 7 {
		t.Fatal("expected seven Stark house cards")
	}
}

func TestOrdersArePrivate(t *testing.T) {
	r, c := started(t)
	if len(r.Orders) == 0 {
		t.Fatal("bots did not plan")
	}
	v := r.View("host-secret", c)
	if len(v.Orders) != 0 {
		t.Fatal("other houses' private orders leaked")
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "host-secret") || strings.Contains(string(b), `"player":`) {
		t.Fatal("private player identity leaked")
	}
}

func TestCannotOrderEnemyArmy(t *testing.T) {
	r, c := started(t)
	if err := r.Apply("host-secret", Command{Type: "order", Region: "pyke", Order: "march"}, c); err == nil {
		t.Fatal("enemy order accepted")
	}
}

func TestTokenCannotBeUsedTwice(t *testing.T) {
	r, c := started(t)
	if err := r.Apply("host-secret", Command{Type: "order", Region: "winterfell", Order: "march"}, c); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply("host-secret", Command{Type: "order", Region: "white-harbor", Order: "march"}, c); err == nil {
		t.Fatal("duplicate token accepted")
	}
}

func TestSpecialOrdersRespectCourt(t *testing.T) {
	r, c := started(t)
	if StarLimit("greyjoy", c) != 0 {
		t.Fatal("wrong base court limit")
	}
	s := &r.Seats[1]
	s.Ready = false
	if err := r.placeOrder(s, "pyke", "march-star", c); err == nil {
		t.Fatal("starred order accepted at rank six")
	}
}

func TestRevealOnlyAfterAllReady(t *testing.T) {
	r, c := started(t)
	for region, order := range map[string]string{"winterfell": "march", "white-harbor": "defend-1", "the-shivering-sea": "support-1"} {
		if err := r.Apply("host-secret", Command{Type: "order", Region: region, Order: order}, c); err != nil {
			t.Fatal(err)
		}
	}
	if r.Phase != "planning" {
		t.Fatal("revealed too early")
	}
	if err := r.Apply("host-secret", Command{Type: "ready"}, c); err != nil {
		t.Fatal(err)
	}
	if r.Phase != "orders_revealed" {
		t.Fatalf("phase %s", r.Phase)
	}
	if len(r.View("host-secret", c).Orders) != len(r.Orders) {
		t.Fatal("revealed orders missing")
	}
	if err := r.Apply("host-secret", Command{Type: "order", Region: "winterfell", Order: ""}, c); err == nil {
		t.Fatal("edited revealed order")
	}
}

func TestHostCannotReplaceHuman(t *testing.T) {
	c := LoadCatalog()
	r := NewRoom("test", "Test", "host", "Host")
	if err := r.Apply("host", Command{Type: "configure", House: "greyjoy", Bot: false, Difficulty: "normal"}, c); err != nil {
		t.Fatal(err)
	}
	if err := r.Join("guest", "Guest", "greyjoy"); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply("host", Command{Type: "configure", House: "greyjoy", Bot: true, Difficulty: "hard"}, c); err == nil {
		t.Fatal("human replaced")
	}
	if err := r.Apply("guest", Command{Type: "start"}, c); err == nil {
		t.Fatal("non-host started game")
	}
}

func TestBotsUseLegalOrdersAtEveryDifficulty(t *testing.T) {
	for _, difficulty := range []string{"easy", "normal", "hard"} {
		t.Run(difficulty, func(t *testing.T) {
			c := LoadCatalog()
			r := NewRoom("test", "Test", "host", "Host")
			for i := range r.Seats {
				r.Seats[i].Difficulty = difficulty
			}
			if err := r.Apply("host", Command{Type: "start"}, c); err != nil {
				t.Fatal(err)
			}
			for i, s := range r.Seats {
				if !s.Bot {
					continue
				}
				copy := r.Clone()
				copy.Seats[i].Ready = false
				orders := map[string]string{}
				for region, army := range c.Units {
					for _, u := range army {
						if u.House == s.House {
							orders[region] = r.Orders[region]
							delete(copy.Orders, region)
						}
					}
				}
				for region, order := range orders {
					if order == "" {
						t.Fatalf("bot %s left %s empty", s.House, region)
					}
					if err := copy.placeOrder(&copy.Seats[i], region, order, c); err != nil {
						t.Fatalf("bot %s: %v", s.House, err)
					}
				}
			}
		})
	}
}
