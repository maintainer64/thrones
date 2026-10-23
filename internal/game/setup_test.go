package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func configuredRoom(t *testing.T, count int) (*Room, Catalog) {
	t.Helper()
	c := LoadCatalog()
	r := NewRoom("setup", "Setup", "host", "Host")
	s := DefaultSettings()
	s.PlayerCount = count
	if err := r.Apply("host", Command{Type: "settings", Settings: &s}, c); err != nil {
		t.Fatal(err)
	}
	return r, c
}

func TestBaseSetupsUseOnlyParticipatingHousesAndCorrectTokens(t *testing.T) {
	for _, count := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			r, c := configuredRoom(t, count)
			setup := baseSetup(c, count)
			if len(r.Seats) != count {
				t.Fatal("wrong number of seats")
			}
			preview := r.View("host", c)
			if r.Match != nil || preview.Setup.PlayerCount != count {
				t.Fatal("preview started game")
			}
			for _, id := range BaseHouses {
				if contains(setup.Houses, id) {
					continue
				}
				if r.ChooseHouse("host", id) == nil || r.Join("guest", "Guest", id) == nil {
					t.Fatal("excluded house accepted", id)
				}
			}
			if err := r.Apply("host", Command{Type: "start"}, c); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r.Match.Armies, preview.Setup.Armies) || !reflect.DeepEqual(r.Match.Tracks, preview.Setup.Tracks) {
				t.Fatal("preview differs from actual setup")
			}
			for id, army := range r.Match.Armies {
				for _, unit := range army {
					if !contains(setup.Houses, unit.House) || contains(r.Match.BlockedRegions, id) {
						t.Fatal("excluded army", id, unit)
					}
				}
			}
			for _, house := range r.Match.Control {
				if !contains(setup.Houses, house) {
					t.Fatal("nonparticipant controls area")
				}
			}
			for _, track := range r.Match.Tracks {
				if len(track) != count {
					t.Fatal("track was not compacted")
				}
				for _, house := range track {
					if !contains(setup.Houses, house) {
						t.Fatal("nonparticipant on track")
					}
				}
			}
			if len(r.Match.Power) != count || len(r.Match.Supply) != count {
				t.Fatal("nonparticipant resources")
			}
			if unitCount(r.Match.Armies["port-of-lannisport"], "ship") != 1 {
				t.Fatal("FAQ Lannister starting ship missing")
			}
			counts := map[FortificationKind]int{}
			for _, value := range r.fortifications(r.Match, c) {
				counts[value.Kind]++
			}
			wantNeutral := map[int]int{3: 2, 4: 12, 5: 9, 6: 2}[count]
			if counts[FortificationGarrison] != count || counts[FortificationNeutral] != wantNeutral {
				t.Fatal("wrong token setup", counts)
			}
			if count == 3 && counts[FortificationBlocked] != 16 || count != 3 && counts[FortificationBlocked] != 0 {
				t.Fatal("wrong blocked areas", counts)
			}
			for position, house := range r.Match.Tracks["kingsCourt"] {
				if r.starLimit(house, c) != courtStars(count)[position] {
					t.Fatal("wrong Court overlay")
				}
			}
			if !reflect.DeepEqual(r.Match, r.Clone().Match) {
				t.Fatal("setup changed across JSON persistence")
			}
		})
	}
}

func TestSetupCannotEvictHumansOrChangeAfterStart(t *testing.T) {
	r, c := configuredRoom(t, 6)
	if err := r.Join("guest", "Guest", "martell"); err != nil {
		t.Fatal(err)
	}
	s := r.Settings
	s.PlayerCount = 3
	before, _ := json.Marshal(r)
	if r.Apply("host", Command{Type: "settings", Settings: &s}, c) == nil {
		t.Fatal("human silently evicted")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("failed setup changed room")
	}
	if r.Apply("guest", Command{Type: "settings", Settings: &s}, c) == nil {
		t.Fatal("guest changed setup")
	}
	if err := r.Apply("host", Command{Type: "start"}, c); err != nil {
		t.Fatal(err)
	}
	if r.Apply("host", Command{Type: "settings", Settings: &s}, c) == nil {
		t.Fatal("setup changed during game")
	}
	for _, n := range []int{-1, 1, 2, 7, 99} {
		s.PlayerCount = n
		if s.Validate() == nil {
			t.Fatal("invalid count", n)
		}
	}
}

func TestBlockedAreasRejectMarchPreviewTransportRetreatAndMuster(t *testing.T) {
	r, c := configuredRoom(t, 3)
	r.initializeMatch(c)
	r.Phase = "marches"
	r.Match.ActiveHouse = "stark"
	r.Match.Supply["stark"] = 6
	r.Match.Armies["the-reach"] = []Unit{{Type: "knight", House: "stark", Count: 3}}
	r.Match.Control["the-reach"] = "stark"
	r.Orders = map[string]string{"the-reach": "march-star"}
	cmd := Command{Type: "march", Region: "the-reach", Target: "highgarden", Units: map[string]int{"knight": 3}}
	before, _ := json.Marshal(r)
	if r.Apply("host", cmd, c) == nil {
		t.Fatal("blocked area conquered with high strength")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("rejected march mutated state")
	}
	preview, err := r.PreviewAction("host", cmd, c, r.Clock.StartedAt)
	if err != nil || slices.Contains(preview.Targets, "highgarden") {
		t.Fatal("blocked preview", err)
	}
	if r.reachable("redwyne-straights", "port-of-oldtown", "stark", c) {
		t.Fatal("blocked port reachable")
	}
	r.Battle = &Battle{From: "kings-landing", To: "the-reach", Attacker: "baratheon", Defender: "stark", Loser: "stark", Survivors: []Unit{{Type: "footman", House: "stark", Count: 1}}}
	if slices.Contains(r.retreatRegions(c), "highgarden") {
		t.Fatal("retreat to blocked area")
	}
	r.Phase = PhaseMustering
	r.Match.Control["oldtown"] = "stark"
	r.Mustering = &Mustering{Points: map[string]int{"oldtown": 2}}
	if _, _, err := r.musterCandidate("stark", MusterOption{Region: "oldtown", Target: "oldtown", Unit: UnitFootman}, c); err == nil {
		t.Fatal("mustering in blocked area")
	}
}

func TestNeutralForceEqualitySupportAndNoCardsOrBlade(t *testing.T) {
	for _, withSupport := range []bool{false, true} {
		r, c, now := fixture(t)
		r.Settings.TidesOfBattle = true
		r.Match.BladeUsed = false
		r.Match.Armies = map[string][]Unit{"kingswood": {{Type: "knight", House: "stark", Count: 2}, {Type: "footman", House: "stark", Count: 1}}}
		r.Match.Control = map[string]string{"kingswood": "stark"}
		r.Orders = map[string]string{"kingswood": "march"}
		r.Match.Garrisons = map[string]int{"kings-landing": 5}
		if withSupport {
			r.Match.Armies["kingswood"] = []Unit{{Type: "knight", House: "stark", Count: 1}}
			r.Match.Armies["blackwater"] = []Unit{{Type: "siege-engine", House: "stark", Count: 1}}
			r.Match.Control["blackwater"] = "stark"
			r.Orders["blackwater"] = "support-1"
		}
		cmd := Command{Region: "kingswood", Target: "kings-landing", Units: map[string]int{"knight": 2, "footman": 1}}
		if withSupport {
			cmd.Units = map[string]int{"knight": 1}
		}
		if err := r.march("stark", cmd, c, now); err != nil {
			t.Fatal(err)
		}
		if r.Battle != nil || r.Match.LastCombat != nil || r.Match.BladeUsed || r.Match.Garrisons["kings-landing"] != 0 || r.owner("kings-landing") != "stark" {
			t.Fatal("neutral conquest treated as combat")
		}
	}
	r, c, now := fixture(t)
	r.Match.Armies = map[string][]Unit{"kingswood": {{Type: "footman", House: "stark", Count: 4}}}
	r.Orders = map[string]string{"kingswood": "march"}
	r.Match.Control = map[string]string{"kingswood": "stark"}
	r.Match.Garrisons = map[string]int{"kings-landing": 5}
	if r.march("stark", Command{Region: "kingswood", Target: "kings-landing", Units: map[string]int{"footman": 4}}, c, now) == nil {
		t.Fatal("insufficient strength captured neutral force")
	}
}

func TestEmptyHomeGarrisonRequiresCombatAndNeverRespawns(t *testing.T) {
	r, c, now := fixture(t)
	r.Match.Armies = map[string][]Unit{"white-harbor": {{Type: "footman", House: "greyjoy", Count: 2}}}
	r.Match.Control = map[string]string{"white-harbor": "greyjoy", "winterfell": "stark"}
	r.Match.Garrisons = map[string]int{"winterfell": 2}
	r.Match.ActiveHouse = "greyjoy"
	r.Match.BladeUsed = true
	r.Orders = map[string]string{"white-harbor": "march"}
	r.Phase = "planning"
	if r.placeOrder(r.houseSeat("stark"), "winterfell", "defend-star", c) == nil {
		t.Fatal("order assigned to garrison alone")
	}
	r.Phase = "marches"
	if err := r.march("greyjoy", Command{Region: "white-harbor", Target: "winterfell", Units: map[string]int{"footman": 2}}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase != "combat" || r.Battle.Defender != "stark" {
		t.Fatal("empty garrison bypassed combat")
	}
	if err := r.chooseCard("greyjoy", "euron-crows-eye", c, now); err != nil {
		t.Fatal(err)
	}
	if err := r.chooseCard("stark", "catelyn-stark", c, now); err != nil {
		t.Fatal(err)
	}
	if r.Match.LastCombat.Defense != 2 || r.Match.Garrisons["winterfell"] != 0 {
		t.Fatal("garrison result incorrect")
	}
	r.Match.Armies["winterfell"] = nil
	r.restoreEmptyControl("winterfell", c)
	if r.owner("winterfell") != "stark" || r.Match.Garrisons["winterfell"] != 0 {
		t.Fatal("home restoration respawned garrison")
	}
}

func TestSmallerSetupsBotsFinish(t *testing.T) {
	for _, count := range []int{3, 4, 5} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			r, c := configuredRoom(t, count)
			for i := range r.Seats {
				r.Seats[i].Bot = true
			}
			if err := r.Apply("host", Command{Type: "start"}, c); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 11 && r.Phase != "finished"; round++ {
				if r.Phase != "orders_revealed" {
					t.Fatal("bots stuck", r.Phase, r.Round)
				}
				if err := r.Apply("host", Command{Type: "continue"}, c); err != nil {
					t.Fatal(err)
				}
				r.runBots(c, r.Clock.StartedAt)
				r = r.Clone()
			}
			if r.Phase != "finished" || r.Match.Winner == "" {
				t.Fatal("smaller setup did not finish", r.Phase, r.Round)
			}
		})
	}
}

func TestCapturedExcludedHomeDoesNotRestoreAnAbsentHouse(t *testing.T) {
	r, c := configuredRoom(t, 4)
	r.initializeMatch(c)
	r.Phase = "marches"
	r.Match.ActiveHouse = "stark"
	r.Match.Supply["stark"] = 6
	r.Match.Armies = map[string][]Unit{"yronwood": {{Type: "knight", House: "stark", Count: 3}}, "white-harbor": {{Type: "footman", House: "stark", Count: 1}}}
	r.Match.Control["yronwood"] = "stark"
	delete(r.Match.Garrisons, "yronwood")
	r.Orders = map[string]string{"yronwood": "march", "white-harbor": "march-minus"}
	if err := r.march("stark", Command{Region: "yronwood", Target: "sunspear", Units: map[string]int{"knight": 3}}, c, r.Clock.StartedAt); err != nil {
		t.Fatal(err)
	}
	r.Phase = "marches"
	r.Match.ActiveHouse = "stark"
	r.Orders = map[string]string{"sunspear": "march", "white-harbor": "march-minus"}
	if err := r.march("stark", Command{Region: "sunspear", Target: "yronwood", Units: map[string]int{"knight": 3}}, c, r.Clock.StartedAt); err != nil {
		t.Fatal(err)
	}
	if r.owner("sunspear") != "" || r.Match.Garrisons["sunspear"] != 0 {
		t.Fatal("absent Martell or defeated neutral force respawned")
	}
}
