package game

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Room, Catalog, time.Time) {
	t.Helper()
	c := LoadCatalog()
	r := NewRoom("fixture", "Test", "host", "Host")
	r.initializeMatch(c)
	for i := range r.Seats {
		r.Seats[i].Bot = false
		r.Match.Supply[r.Seats[i].House] = 6
	}
	r.Match.Armies = map[string][]Unit{"winterfell": {{Type: "footman", House: "stark", Count: 2}}}
	r.Match.Control = map[string]string{"winterfell": "stark"}
	r.Phase = "marches"
	r.Match.ActiveHouse = "stark"
	r.Orders = map[string]string{"winterfell": "march"}
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	r.startClock("stark", now)
	return r, c, now
}

func TestDefaultSoftFiveMinuteClock(t *testing.T) {
	r, c, now := fixture(t)
	if r.Settings.TurnMinutes != 5 || r.Settings.HardTimer {
		t.Fatal("default must be soft five minutes")
	}
	if !r.Clock.Deadline.Equal(now.Add(5 * time.Minute)) {
		t.Fatal("wrong deadline")
	}
	before, _ := json.Marshal(r)
	if r.Expire(now.Add(24*time.Hour), c) {
		t.Fatal("soft timeout mutated game")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("soft timeout changed state")
	}
	if err := r.action("stark", Command{Type: "pass"}, c, now.Add(24*time.Hour)); err != nil {
		t.Fatal("soft timeout prevented action:", err)
	}
}

func TestHardPlanningDeadlineSurvivesSerialization(t *testing.T) {
	c := LoadCatalog()
	r := NewRoom("timer", "Timer", "host", "Host")
	r.Settings = Settings{TurnMinutes: 1, HardTimer: true, Scenario: "base"}
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	if err := r.ApplyAt("host", Command{Type: "start"}, c, now); err != nil {
		t.Fatal(err)
	}
	deadline := r.Clock.Deadline
	r = r.Clone()
	if !r.Clock.Deadline.Equal(deadline) {
		t.Fatal("reload reset deadline")
	}
	if r.Expire(now.Add(59*time.Second), c) {
		t.Fatal("expired early")
	}
	if !r.Expire(deadline, c) {
		t.Fatal("deadline not enforced")
	}
	if r.Phase != "orders_revealed" {
		t.Fatalf("phase %s", r.Phase)
	}
	for _, s := range r.Seats {
		if !s.Ready {
			t.Fatal("unfinished player after hard deadline")
		}
	}
	if r.Expire(deadline.Add(time.Second), c) {
		t.Fatal("same deadline enforced twice")
	}
}

func TestHardActionTimeoutPassesWithoutMovingArmy(t *testing.T) {
	r, c, now := fixture(t)
	r.Settings.HardTimer = true
	before := count(r.Match.Armies["winterfell"])
	if !r.Expire(now.Add(6*time.Minute), c) {
		t.Fatal("hard action did not expire")
	}
	if count(r.Match.Armies["winterfell"]) != before {
		t.Fatal("timeout moved army")
	}
	if r.Round != 2 || r.Phase != PhaseWesteros || r.Westeros.Current == nil {
		t.Fatalf("did not advance: %d %s", r.Round, r.Phase)
	}
}

func TestSettingsValidationAndHostOnly(t *testing.T) {
	c := LoadCatalog()
	r := NewRoom("room", "Test", "host", "Host")
	for _, s := range []Settings{{TurnMinutes: 0, Scenario: "base"}, {TurnMinutes: 121, Scenario: "base"}, {TurnMinutes: 5, Scenario: "mother"}} {
		if s.Validate() == nil {
			t.Fatal("unsupported settings accepted")
		}
	}
	settings := Settings{TurnMinutes: 12, HardTimer: true, Scenario: "base"}
	if err := r.Apply("host", Command{Type: "settings", Settings: &settings}, c); err != nil {
		t.Fatal(err)
	}
	if r.Settings != settings {
		t.Fatal("settings not persisted")
	}
	if err := r.Apply("host", Command{Type: "start"}, c); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply("host", Command{Type: "settings", Settings: &settings}, c); err == nil {
		t.Fatal("settings changed after start")
	}
}

func TestMarchChangesArmiesAndControl(t *testing.T) {
	r, c, now := fixture(t)
	if err := r.action("stark", Command{Type: "march", Region: "winterfell", Target: "white-harbor", Units: map[string]int{"footman": 1}}, c, now); err != nil {
		t.Fatal(err)
	}
	if count(r.Match.Armies["winterfell"]) != 1 || count(r.Match.Armies["white-harbor"]) != 1 || r.Match.Control["white-harbor"] != "stark" {
		t.Fatal("army/control not moved")
	}
}

func TestIllegalMarchDoesNotMutateState(t *testing.T) {
	for _, cmd := range []Command{
		{Type: "march", Region: "winterfell", Target: "sunspear", Units: map[string]int{"footman": 1}},
		{Type: "march", Region: "winterfell", Target: "white-harbor", Units: map[string]int{"footman": 3}},
		{Type: "march", Region: "winterfell", Target: "white-harbor", Units: map[string]int{"ship": 1}},
		{Type: "march", Region: "winterfell", Target: "white-harbor", Units: map[string]int{"footman": -1}},
	} {
		r, c, now := fixture(t)
		before, _ := json.Marshal(r)
		if err := r.action("stark", cmd, c, now); err == nil {
			t.Fatal("illegal march accepted")
		}
		after, _ := json.Marshal(r)
		if string(before) != string(after) {
			t.Fatal("illegal march mutated room")
		}
	}
}

func TestCombatSecretChoicesRevealOnlyAfterBoth(t *testing.T) {
	r, c, now := fixture(t)
	r.Match.Armies["white-harbor"] = []Unit{{Type: "footman", House: "greyjoy", Count: 1}}
	r.Match.Control["white-harbor"] = "greyjoy"
	if err := r.action("stark", Command{Type: "march", Region: "winterfell", Target: "white-harbor", Units: map[string]int{"footman": 2}}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase != "combat" {
		t.Fatalf("expected combat, got %s", r.Phase)
	}
	if err := r.chooseCard("stark", "eddard-stark", c, now); err != nil {
		t.Fatal(err)
	}
	// Pretend a human occupies the defender seat, to inspect its personal network view.
	r.Seats[1].Player = "guest"
	view := r.View("guest", c)
	b, _ := json.Marshal(view)
	if strings.Contains(string(b), "eddard-stark") {
		t.Fatal("attacker's secret card leaked")
	}
	if !view.Battle.Chosen["stark"] || view.Battle.YourCard != "" {
		t.Fatal("wrong private battle view")
	}
	if err := r.chooseCard("greyjoy", "euron-crows-eye", c, now); err != nil {
		t.Fatal(err)
	}
	if r.Battle.Choice == nil || r.Battle.Choice.Kind != ChoiceBlade {
		t.Fatal("expected defender's Blade decision after revealing cards")
	}
	applyAbility(t, r, c, now, "keep")
	if r.Match.LastCombat == nil || r.Match.LastCombat.AttackerCard != "eddard-stark" || r.Match.LastCombat.Winner != "stark" {
		t.Fatal("combat did not resolve")
	}
	if r.owner("white-harbor") != "stark" {
		t.Fatal("winner did not take area")
	}
}

func TestRaidRemovesOnlyAllowedEnemyOrder(t *testing.T) {
	r, c, now := fixture(t)
	r.Phase = "raids"
	r.Orders = map[string]string{"winterfell": "raid-1", "white-harbor": "march"}
	r.Match.Armies["white-harbor"] = []Unit{{Type: "footman", House: "greyjoy", Count: 1}}
	r.Match.Control["white-harbor"] = "greyjoy"
	if err := r.raid("stark", Command{Region: "winterfell", Target: "white-harbor"}, c, now); err == nil {
		t.Fatal("normal raid removed march")
	}
	r.Orders["white-harbor"] = "support-1"
	if err := r.raid("stark", Command{Region: "winterfell", Target: "white-harbor"}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Orders["white-harbor"] != "" {
		t.Fatal("support survived raid")
	}
}

func TestBotsFinishPrototypeAcrossAllDifficulties(t *testing.T) {
	for _, difficulty := range []string{"easy", "normal", "hard"} {
		t.Run(difficulty, func(t *testing.T) {
			c := LoadCatalog()
			r := NewRoom("bots", "Bots", "host", "Host")
			for i := range r.Seats {
				r.Seats[i].Bot = true
				r.Seats[i].Difficulty = difficulty
			}
			if err := r.Apply("host", Command{Type: "start"}, c); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
			for i := 0; i < 11 && r.Phase != "finished"; i++ {
				if r.Phase != "orders_revealed" {
					t.Fatalf("bots stuck in %s", r.Phase)
				}
				if err := r.ApplyAt(r.Host, Command{Type: "continue"}, c, now); err != nil {
					t.Fatal(err)
				}
				r.runBots(c, now)
				r = r.Clone()
			}
			if r.Phase != "finished" || r.Match.Winner == "" {
				t.Fatalf("prototype did not finish: %s round %d", r.Phase, r.Round)
			}
		})
	}
}
