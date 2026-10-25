package game

import (
	"encoding/json"
	"testing"
	"time"
)

func plainBladeBattle(t *testing.T, holder string) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := cardBattle(t, "stark", "greyjoy", "greatjon-umber", "dagmar-cleftjaw")
	r.Match.BladeUsed = false
	r.Match.Tracks["fiefdoms"] = []string{holder, "stark", "greyjoy"}
	r.Battle.Stage = "tides"
	return r, c, now
}

func TestBladeWithoutTidesOffersExplicitPersistedChoice(t *testing.T) {
	r, c, now := plainBladeBattle(t, "stark")
	r.advanceCardEffects(c, now)
	if r.Phase != "card_effect" || r.Battle.Choice == nil || r.Battle.Choice.Kind != ChoiceBlade || r.Match.LastCombat != nil || r.Match.BladeUsed {
		t.Fatal("missing unspent Blade decision", r.Battle)
	}
	r = r.Clone()
	before, _ := json.Marshal(r)
	if err := r.chooseAbility("greyjoy", "strength", c, now); err == nil {
		t.Fatal("foreign Blade use accepted")
	}
	if err := r.chooseAbility("stark", "redraw", c, now); err == nil {
		t.Fatal("redraw without Tides accepted")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("rejected decision mutated state")
	}
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "strength")
	if !r.Match.BladeUsed || r.Match.LastCombat.Attack != 6 || r.Match.LastCombat.Defense != 3 || r.Match.LastCombat.BladeBonus != "stark" || r.Match.LastCombat.Modifiers != nil {
		t.Fatal("Blade result incorrect", r.Match.LastCombat)
	}
	if err := r.chooseAbility("stark", "strength", c, now); err == nil {
		t.Fatal("Blade used twice")
	}
}

func TestBladeWithoutTidesDefenderAndDecline(t *testing.T) {
	for _, use := range []bool{false, true} {
		r, c, now := plainBladeBattle(t, "greyjoy")
		r.advanceCardEffects(c, now)
		choice := "keep"
		if use {
			choice = "strength"
		}
		applyAbility(t, r, c, now, choice)
		want := 3
		if use {
			want++
		}
		if r.Match.BladeUsed != use || r.Match.LastCombat.Defense != want {
			t.Fatal(r.Match.LastCombat)
		}
	}
}

func TestBladeWithoutTidesSkipsUsedOrNonParticipantHolder(t *testing.T) {
	for _, holder := range []string{"stark", "lannister"} {
		r, c, now := plainBladeBattle(t, holder)
		r.Match.BladeUsed = holder == "stark"
		r.advanceCardEffects(c, now)
		if r.Match.LastCombat == nil || r.Match.LastCombat.BladeBonus != "" {
			t.Fatal("unavailable Blade affected combat")
		}
	}
}

func TestBladeWithoutTidesSoftAndHardTimer(t *testing.T) {
	r, c, now := plainBladeBattle(t, "stark")
	r.advanceCardEffects(c, now)
	deadline := r.Clock.Deadline
	if r.Expire(deadline.Add(time.Second), c) {
		t.Fatal("soft timer decided for player")
	}
	r.Settings.HardTimer = true
	if !r.Clone().Expire(deadline.Add(time.Second), c) {
		t.Fatal("hard timer did not decline Blade")
	}
	r.Settings.HardTimer = false
	r.Match.BladeUsed = true
	if err := r.chooseAbility("stark", "strength", c, now); err == nil {
		t.Fatal("used Blade accepted after state change")
	}
}
