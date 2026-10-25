package game

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func provisionFixture(t *testing.T) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := fixture(t)
	r.Phase = PhaseWesteros
	r.Orders = map[string]string{}
	for i := range r.Seats {
		r.Seats[i].Player = r.Seats[i].House
	}
	return r, c, now
}
func TestSupplyOnlyRecalculatesOnItsEventAndRequiresDisband(t *testing.T) {
	r, c, now := provisionFixture(t)
	r.Match.Armies = map[string][]Unit{
		"winterfell":   {{Type: "footman", House: "stark", Count: 4}},
		"white-harbor": {{Type: "knight", House: "stark", Count: 2}},
		"seagard":      {{Type: "footman", House: "stark", Count: 2}},
	}
	if r.Match.Supply["stark"] != 6 {
		t.Fatal("fixture")
	}
	if err := r.BeginSupply(c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase != PhaseSupply || r.Match.ActiveHouse != "stark" || r.Match.Supply["stark"] != 2 {
		t.Fatal("event did not count actual controlled barrels")
	}
	v := r.View("stark", c).Supply
	if v == nil || !reflect.DeepEqual(v.Limits, []int{3, 2, 2}) || len(v.Armies) != 3 {
		t.Fatal("missing supply projection")
	}
	for _, cmd := range []Command{{Type: "disband", Region: "winterfell", Units: map[string]int{"footman": -1}}, {Type: "disband", Region: "winterfell", Units: map[string]int{"footman": 5}}, {Type: "disband", Region: "unknown", Units: map[string]int{"footman": 1}}, {Type: "disband", Region: "winterfell"}} {
		assertUnchanged(t, r, func() error { return r.ApplyAt("stark", cmd, c, now) })
	}
	assertUnchanged(t, r, func() error {
		return r.ApplyAt("greyjoy", Command{Type: "disband", Region: "winterfell", Units: map[string]int{"footman": 1}}, c, now)
	})
	deadline := r.Clock.Deadline
	r = r.Clone()
	if !r.Clock.Deadline.Equal(deadline) {
		t.Fatal("reload reset deadline")
	}
	if err := r.ApplyAt("stark", Command{Type: "disband", Region: "winterfell", Units: map[string]int{"footman": 1}}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase != PhaseWesteros || r.Supply != nil || count(r.Match.Armies["winterfell"]) != 3 || !r.supplyOK("stark", r.Match.Armies, c) {
		t.Fatal("did not complete reconciliation")
	}
	// Losing a barrel does not continuously change the recorded supply level.
	r.Match.Armies["seagard"] = nil
	delete(r.Match.Control, "seagard")
	if r.Match.Supply["stark"] != 2 {
		t.Fatal("supply changed outside its event")
	}
}
func TestSupplyIncludesFleetAndPortArmiesButNotSingles(t *testing.T) {
	r, c, now := provisionFixture(t)
	r.Match.Armies = map[string][]Unit{"winterfell": {{Type: "footman", House: "stark", Count: 2}}, "the-shivering-sea": {{Type: "ship", House: "stark", Count: 2}}, "port-of-winterfell": {{Type: "ship", House: "stark", Count: 2}}}
	if err := r.BeginSupply(c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase != PhaseSupply || r.Match.Supply["stark"] != 1 {
		t.Fatal("third fleet army ignored")
	}
	if err := r.disband("stark", "port-of-winterfell", map[string]int{"ship": 1}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Supply != nil || r.Phase != PhaseWesteros {
		t.Fatal("single ship incorrectly counted as an army")
	}
}
func TestDisbandPreservesOnlyEstablishedControl(t *testing.T) {
	for _, token := range []bool{false, true} {
		r, c, now := provisionFixture(t)
		r.Match.Supply["stark"] = 0
		r.Match.Armies = map[string][]Unit{"white-harbor": {{Type: "footman", House: "stark", Count: 2}}, "winterfell": {{Type: "footman", House: "stark", Count: 2}}, "moat-cailin": {{Type: "footman", House: "stark", Count: 2}}}
		r.Match.Control["white-harbor"] = "stark"
		if token {
			r.Match.ControlTokens = map[string]string{"white-harbor": "stark"}
		}
		if err := r.BeginSupply(c, now); err != nil {
			t.Fatal(err)
		}
		if err := r.disband("stark", "white-harbor", map[string]int{"footman": 2}, c, now); err != nil {
			t.Fatal(err)
		}
		if (r.Match.Control["white-harbor"] == "stark") != token {
			t.Fatal("last unit manufactured or lost a control token")
		}
	}
}
func TestSupplyHardTimeoutResolvesOnlyCurrentHouseAndSoftDoesNothing(t *testing.T) {
	r, c, now := provisionFixture(t)
	r.Match.Armies = map[string][]Unit{"winterfell": {{Type: "footman", House: "stark", Count: 5}}, "pyke": {{Type: "footman", House: "greyjoy", Count: 5}}}
	if err := r.BeginSupply(c, now); err != nil {
		t.Fatal(err)
	}
	house := r.Match.ActiveHouse
	before, _ := json.Marshal(r)
	if r.Expire(now.Add(time.Hour), c) {
		t.Fatal("soft timeout")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("soft mutation")
	}
	r.Settings.HardTimer = true
	if !r.Expire(r.Clock.Deadline, c) {
		t.Fatal("hard timeout did nothing")
	}
	if !r.supplyOK(house, r.Match.Armies, c) || r.Match.ActiveHouse == house {
		t.Fatal("did not reconcile current house")
	}
	if r.Phase == PhaseSupply && r.Expire(now, c) {
		t.Fatal("expired following decision")
	}
}
func TestMusteringPointsStockUpgradesAndTurnOrder(t *testing.T) {
	r, c, now := provisionFixture(t)
	r.Match.Control = map[string]string{"winterfell": "stark", "white-harbor": "stark"}
	if err := r.BeginMustering(c, now); err != nil {
		t.Fatal(err)
	}
	if r.Match.ActiveHouse != "stark" || r.Mustering.Points["winterfell"] != 2 || r.Mustering.Points["white-harbor"] != 1 {
		t.Fatal("wrong mustering points")
	}
	before := r.reserve("stark", c)
	o := MusterOption{Region: "white-harbor", Target: "white-harbor", Unit: UnitKnight}
	assertUnchanged(t, r, func() error { return r.muster("stark", o, c, now) })
	o = MusterOption{Region: "winterfell", Target: "winterfell", Unit: UnitKnight, Upgrade: true}
	if err := r.muster("stark", o, c, now); err != nil {
		t.Fatal(err)
	}
	after := r.reserve("stark", c)
	if after["footman"] != before["footman"]+1 || after["knight"] != before["knight"]-1 || r.Mustering.Points["winterfell"] != 1 {
		t.Fatal("upgrade cost or stock is wrong")
	}
	for _, o := range r.musterOptions(c) {
		copy := r.Clone()
		if err := copy.muster("stark", o, c, now); err != nil {
			t.Fatal("advertised invalid option", o, err)
		}
	}
	assertUnchanged(t, r, func() error { return r.finishMuster("greyjoy", c, now) })
	if err := r.finishMuster("stark", c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase != PhaseWesteros || r.Mustering != nil {
		t.Fatal("did not resume event resolver")
	}
}
func TestMusteringRejectsCheatingAndLeavesStateUnchanged(t *testing.T) {
	r, c, now := provisionFixture(t)
	r.Match.Control = map[string]string{"winterfell": "stark"}
	if err := r.BeginMustering(c, now); err != nil {
		t.Fatal(err)
	}
	for _, o := range []MusterOption{
		{Region: "winterfell", Target: "winterfell", Unit: UnitType("dragon")},
		{Region: "pyke", Target: "pyke", Unit: UnitFootman},
		{Region: "winterfell", Target: "white-harbor", Unit: UnitFootman},
		{Region: "winterfell", Target: "winterfell", Unit: UnitShip},
		{Region: "winterfell", Target: "the-narrow-sea", Unit: UnitShip},
		{Region: "winterfell", Target: "winterfell", Unit: UnitFootman, Upgrade: true},
	} {
		assertUnchanged(t, r, func() error { return r.muster("stark", o, c, now) })
	}
	assertUnchanged(t, r, func() error {
		return r.muster("greyjoy", MusterOption{Region: "winterfell", Target: "winterfell", Unit: UnitFootman}, c, now)
	})
	r.Match.Armies["white-harbor"] = []Unit{{Type: "knight", House: "stark", Count: c.Houses["stark"].Limits["knight"]}}
	assertUnchanged(t, r, func() error {
		return r.muster("stark", MusterOption{Region: "winterfell", Target: "winterfell", Unit: UnitKnight, Upgrade: true}, c, now)
	})
}
func TestMusteringShipsPortsAndEnemySeas(t *testing.T) {
	r, c, now := provisionFixture(t)
	r.Match.Control = map[string]string{"white-harbor": "stark"}
	r.Match.Armies = map[string][]Unit{"the-narrow-sea": {{Type: "ship", House: "greyjoy", Count: 1}}}
	if err := r.BeginMustering(c, now); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, r, func() error {
		return r.muster("stark", MusterOption{Region: "white-harbor", Target: "the-narrow-sea", Unit: UnitShip}, c, now)
	})
	if err := r.muster("stark", MusterOption{Region: "white-harbor", Target: "port-of-white-harbor", Unit: UnitShip}, c, now); err != nil {
		t.Fatal("port recruitment blocked by enemy sea", err)
	}
	if count(r.Match.Armies["port-of-white-harbor"]) != 1 || r.Phase != PhaseWesteros {
		t.Fatal("ship missing")
	}
	r.Phase = PhaseWesteros
	r.Match.Armies["port-of-white-harbor"] = []Unit{{Type: "ship", House: "stark", Count: 3}}
	if err := r.BeginMustering(c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase == PhaseMustering {
		assertUnchanged(t, r, func() error {
			return r.muster("stark", MusterOption{Region: "white-harbor", Target: "port-of-white-harbor", Unit: UnitShip}, c, now)
		})
	}
}
func TestMusteringObeysSupplyButUpgradeDoesNotGrowArmy(t *testing.T) {
	r, c, now := provisionFixture(t)
	r.Match.Supply["stark"] = 0
	r.Match.Control = map[string]string{"white-harbor": "stark"}
	r.Match.Armies = map[string][]Unit{"white-harbor": {{Type: "footman", House: "stark", Count: 2}}, "winterfell": {{Type: "footman", House: "stark", Count: 2}}}
	if err := r.BeginMustering(c, now); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, r, func() error {
		return r.muster("stark", MusterOption{Region: "white-harbor", Target: "white-harbor", Unit: UnitFootman}, c, now)
	})
	if err := r.muster("stark", MusterOption{Region: "white-harbor", Target: "white-harbor", Unit: UnitSiege, Upgrade: true}, c, now); err != nil {
		t.Fatal(err)
	}
	if count(r.Match.Armies["white-harbor"]) != 2 {
		t.Fatal("upgrade grew army")
	}
}
func TestSpecialPowerChoiceMustersOnlyLocallyOrCollectsPower(t *testing.T) {
	for _, choice := range []string{"power", "muster"} {
		r, c, now := provisionFixture(t)
		r.Match.Control["white-harbor"] = "stark"
		r.Orders = map[string]string{"winterfell": "power-star"}
		r.finishRound(c, now)
		if r.Phase != PhasePower || r.Match.ActiveHouse != "stark" {
			t.Fatal("no special order choice")
		}
		before := r.Match.Power["stark"]
		assertUnchanged(t, r, func() error { return r.consolidate("stark", "winterfell", "invalid", c, now) })
		if err := r.consolidate("stark", "winterfell", choice, c, now); err != nil {
			t.Fatal(err)
		}
		if choice == "power" {
			if r.Match.Power["stark"] != before+2 || r.Phase != PhaseWesteros || r.Round != 2 {
				t.Fatal("power order did not end round")
			}
		} else {
			if r.Phase != PhaseMustering || len(r.Mustering.Points) != 1 || r.Mustering.Points["winterfell"] != 2 || r.Match.Power["stark"] != before {
				t.Fatal("mustering was not local")
			}
			if err := r.finishMuster("stark", c, now); err != nil {
				t.Fatal(err)
			}
			if r.Phase != PhaseWesteros || r.Round != 2 {
				t.Fatal("mustering did not resume end of round")
			}
		}
	}
}
func TestProvisionBotsAndHardTimerFinishWithoutBrowser(t *testing.T) {
	r, c, now := provisionFixture(t)
	for i := range r.Seats {
		r.Seats[i].Bot = true
		r.Seats[i].Difficulty = []string{"easy", "normal", "hard"}[i%3]
	}
	if err := r.BeginMustering(c, now); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100 && r.Phase == PhaseMustering; i++ {
		if !r.AdvanceBot(c, now) {
			t.Fatal("bot stalled")
		}
		r = r.Clone()
	}
	if r.Phase != PhaseWesteros {
		t.Fatal("bots did not finish")
	}
	r.Match.Control["winterfell"] = "stark"
	r.Orders = map[string]string{"winterfell": "power-star"}
	r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 1}}
	r.finishRound(c, now)
	r.Settings.HardTimer = true
	if !r.Expire(r.Clock.Deadline, c) || r.Phase != PhaseWesteros || r.Round != 2 || r.Westeros.Run == nil {
		t.Fatal("power timeout did not safely collect")
	}
}
func TestProvisionHooksCannotBeForcedDuringOtherPhases(t *testing.T) {
	r, c, now := fixture(t)
	assertUnchanged(t, r, func() error { return r.BeginSupply(c, now) })
	assertUnchanged(t, r, func() error { return r.BeginMustering(c, now) })
}

func TestExistingControlTokenDoesNotRequirePayingAgain(t *testing.T) {
	for _, leave := range []bool{false, true} {
		r, c, now := fixture(t)
		r.Match.ControlTokens = map[string]string{"white-harbor": "stark"}
		r.Match.Control["white-harbor"] = "stark"
		r.Match.Armies["white-harbor"] = []Unit{{Type: "footman", House: "stark", Count: 1}}
		r.Orders = map[string]string{"white-harbor": "march"}
		before := r.Match.Power["stark"]
		if err := r.march("stark", Command{Region: "white-harbor", Target: "winterfell", Units: map[string]int{"footman": 1}, LeaveToken: leave}, c, now); err != nil {
			t.Fatal(err)
		}
		if r.Match.ControlTokens["white-harbor"] != "stark" || r.Match.Control["white-harbor"] != "stark" || r.Match.Power["stark"] != before {
			t.Fatal("existing token removed or paid for twice")
		}
	}
}
func TestPowerPoolCountsEstablishedTokens(t *testing.T) {
	r, _, _ := fixture(t)
	r.Match.ControlTokens = map[string]string{"white-harbor": "stark", "seagard": "greyjoy"}
	r.gainPower("stark", 99)
	if r.Match.Power["stark"] != 19 {
		t.Fatal("exceeded total power pool")
	}
	delete(r.Match.ControlTokens, "white-harbor")
	r.gainPower("stark", 1)
	if r.Match.Power["stark"] != 20 {
		t.Fatal("returned token did not free power pool")
	}
}
