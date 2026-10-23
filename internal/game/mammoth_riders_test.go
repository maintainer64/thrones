package game

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

func mammothFixture(t *testing.T, won bool) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := revealedWildlingEffect(t, WildlingMammoths, won)
	r.Match.Armies, r.Match.Control = map[string][]Unit{}, map[string]string{}
	r.Orders = map[string]string{}
	return r, c, now
}

func finishMammoths(t *testing.T, r *Room, c Catalog, now time.Time) {
	t.Helper()
	for i := range r.Seats {
		r.Seats[i].Bot, r.Seats[i].Difficulty = true, "normal"
	}
	for step := 0; step < 64 && r.Wildlings.Attack != nil; step++ {
		nextWesteros(t, r, c, now)
	}
	if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Card != WildlingMammoths || !validWildlingDeck(r.Wildlings.Deck) {
		t.Fatal("Mammoth Riders did not complete exactly once")
	}
}

func TestMammothRewardReturnsOnlyOneChosenCardOrCanBeDeclined(t *testing.T) {
	for _, option := range []string{"eddard-stark", "catelyn-stark", WildlingFinishReward} {
		r, c, now := mammothFixture(t, true)
		r.Match.Discard["stark"] = []string{"eddard-stark", "catelyn-stark", "robb-stark"}
		r.Match.Discard["greyjoy"] = []string{"euron-crows-eye"}
		before := r.Clone()
		nextWesteros(t, r, c, now)
		choice := r.Wildlings.Attack.Effect.Choice
		if choice.Kind != WildlingRetrieveCard || choice.Count != 1 || len(choice.Options) != 4 {
			t.Fatal("optional single-card reward missing")
		}
		if err := r.chooseWildlingEffect("stark", option, c); err != nil {
			t.Fatal(err)
		}
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", option, c) })
		finishMammoths(t, r, c, now)
		want := 2
		if option == WildlingFinishReward {
			want = 3
		}
		if len(r.Match.Discard["stark"]) != want || (option != WildlingFinishReward && slices.Contains(r.Match.Discard["stark"], option)) || !reflect.DeepEqual(before.Match.Discard["greyjoy"], r.Match.Discard["greyjoy"]) || !reflect.DeepEqual(before.Match.Power, r.Match.Power) || r.Westeros.Threat != 0 {
			t.Fatal("reward restored more than one card or modified another house")
		}
	}
	r, c, now := mammothFixture(t, true)
	finishMammoths(t, r, c, now)
	if len(r.Match.Discard["stark"]) != 0 {
		t.Fatal("empty discard produced a card")
	}
}

func TestMammothLossAllowsEveryUnitTypeAndUsesThreeThenTwoQuotas(t *testing.T) {
	r, c, now := mammothFixture(t, false)
	r.Match.Armies = map[string][]Unit{
		"winterfell":        {{Type: "footman", House: "stark", Count: 1}, {Type: "knight", House: "stark", Count: 1}, {Type: "siege-engine", House: "stark", Count: 1}},
		"the-shivering-sea": {{Type: "ship", House: "stark", Count: 1}},
		"pyke":              {{Type: "footman", House: "greyjoy", Count: 3}},
		"sunspear":          {{Type: "knight", House: "martell", Count: 1}},
	}
	nextWesteros(t, r, c, now)
	choice := r.Wildlings.Attack.Effect.Choice
	if choice.House != "stark" || choice.Count != 3 || len(choice.UnitOptions) != 4 {
		t.Fatal("lowest must lose three, with all four unit types selectable")
	}
	for _, id := range []string{"winterfell:footman", "the-shivering-sea:ship", "winterfell:siege-engine"} {
		if err := r.chooseWildlingEffect("stark", id, c); err != nil {
			t.Fatal(err)
		}
		nextWesteros(t, r, c, now)
	}
	if count(r.Match.Armies["winterfell"]) != 1 || r.Match.Armies["winterfell"][0].Type != "knight" || count(r.Match.Armies["the-shivering-sea"]) != 0 {
		t.Fatal("chosen casualties or surviving unit type ignored")
	}
	finishMammoths(t, r, c, now)
	if count(r.Match.Armies["pyke"]) != 1 || count(r.Match.Armies["sunspear"]) != 0 || r.Westeros.Threat != 8 {
		t.Fatal("others did not lose min(2, available units)")
	}
}

func TestMammothPortShipsStaySelectableUntilHouseFinishesItsLosses(t *testing.T) {
	for _, token := range []bool{false, true} {
		r, c, now := mammothFixture(t, false)
		r.Match.Armies = map[string][]Unit{
			"white-harbor":         {{Type: "footman", House: "stark", Count: 1}},
			"port-of-white-harbor": {{Type: "ship", House: "stark", Count: 2}},
			"winterfell":           {{Type: "knight", House: "stark", Count: 1}},
		}
		r.Match.Control["white-harbor"] = "stark"
		if token {
			r.Match.ControlTokens = map[string]string{"white-harbor": "stark"}
		}
		nextWesteros(t, r, c, now)
		if err := r.chooseWildlingEffect("stark", "white-harbor:footman", c); err != nil {
			t.Fatal(err)
		}
		if count(r.Match.Armies["port-of-white-harbor"]) != 2 {
			t.Fatal("orphan-port cleanup ran before casualties were fully selected")
		}
		r = r.Clone()
		for i := 0; i < 2; i++ {
			nextWesteros(t, r, c, now)
			if err := r.chooseWildlingEffect("stark", "port-of-white-harbor:ship", c); err != nil {
				t.Fatal(err)
			}
		}
		finishMammoths(t, r, c, now)
		if count(r.Match.Armies["winterfell"]) != 1 || count(r.Match.Armies["port-of-white-harbor"]) != 0 || (r.owner("white-harbor") == "stark") != token {
			t.Fatal("chosen port losses demanded an extra casualty or corrupted control")
		}
	}
}

func TestMammothInvalidChoicesAndStructuredOptionsAreIsolated(t *testing.T) {
	for _, won := range []bool{false, true} {
		r, c, now := mammothFixture(t, won)
		r.Match.Discard["stark"] = []string{"eddard-stark"}
		r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 4}}
		nextWesteros(t, r, c, now)
		valid := "winterfell:footman"
		if won {
			valid = "eddard-stark"
		}
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("greyjoy", valid, c) })
		for _, invalid := range []string{"unknown", "winterfell:knight", "pyke:footman", "robb-stark", ""} {
			assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", invalid, c) })
		}
		if !won {
			assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", WildlingFinishReward, c) })
			view := r.Wildlings.view("outsider")
			view.Attack.Choice.UnitOptions[0].Region, view.Attack.Choice.UnitOptions[0].Count = "changed", 99
			if r.Wildlings.Attack.Effect.Choice.UnitOptions[0].Region == "changed" || r.Wildlings.Attack.Effect.Choice.UnitOptions[0].Count == 99 {
				t.Fatal("structured choices alias authoritative units")
			}
		}
		if err := r.chooseWildlingEffect("stark", valid, c); err != nil {
			t.Fatal(err)
		}
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", valid, c) })
		finishMammoths(t, r, c, now)
		for _, event := range r.Events {
			if hasCyrillic(TranslateSystemText(event.Text, "en")) {
				t.Fatal("untranslated Mammoth Riders effect", event.Text)
			}
		}
	}
}

func TestMammothLowestWithFewerThanThreeUnitsLosesOnlyAvailablePieces(t *testing.T) {
	for _, n := range []int{0, 1, 2} {
		r, c, now := mammothFixture(t, false)
		if n > 0 {
			r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: n}}
		}
		finishMammoths(t, r, c, now)
		if count(r.Match.Armies["winterfell"]) != 0 || r.Westeros.Threat != 8 {
			t.Fatal("short army stalled or an unavailable casualty was required")
		}
	}
}

func TestMammothUnselectedOrphanPortIsCleanedAfterAllChosenCasualties(t *testing.T) {
	for _, token := range []bool{false, true} {
		r, c, now := mammothFixture(t, false)
		r.Match.Armies = map[string][]Unit{
			"white-harbor":         {{Type: "footman", House: "stark", Count: 1}},
			"port-of-white-harbor": {{Type: "ship", House: "stark", Count: 1}},
			"winterfell":           {{Type: "footman", House: "stark", Count: 2}},
		}
		if token {
			r.Match.ControlTokens = map[string]string{"white-harbor": "stark"}
		}
		r.Orders["port-of-white-harbor"] = "support-1"
		for _, id := range []string{"white-harbor:footman", "winterfell:footman", "winterfell:footman"} {
			nextWesteros(t, r, c, now)
			if err := r.chooseWildlingEffect("stark", id, c); err != nil {
				t.Fatal(err)
			}
		}
		ships := 0
		if token {
			ships = 1
		}
		if count(r.Match.Armies["port-of-white-harbor"]) != ships || (!token && r.Orders["port-of-white-harbor"] != "") {
			t.Fatal("port cleanup was omitted or ignored an established control token")
		}
		finishMammoths(t, r, c, now)
	}
}

func TestMammothBotsAndHardTimerCompleteBothOutcomes(t *testing.T) {
	for _, mode := range []string{"easy", "normal", "hard", "hard-timer"} {
		for _, won := range []bool{false, true} {
			r, c, now := mammothFixture(t, won)
			r.Match.Discard["stark"] = []string{"eddard-stark", "catelyn-stark"}
			r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 4}}
			r.Match.Armies["the-shivering-sea"] = []Unit{{Type: "ship", House: "stark", Count: 2}}
			for i := range r.Seats {
				r.Seats[i].Bot, r.Seats[i].Difficulty = mode != "hard-timer", mode
			}
			for step := 0; step < 32 && r.Wildlings.Attack != nil; step++ {
				if r.Clock.Deadline.IsZero() {
					nextWesteros(t, r, c, now)
					continue
				}
				copy := r.Clone()
				if r.Expire(r.Clock.Deadline, c) || !reflect.DeepEqual(copy, r) {
					t.Fatal("soft deadline chose for a human")
				}
				if mode == "hard-timer" {
					r.Settings.HardTimer = true
					deadline := r.Clock.Deadline
					if !deadline.After(now) || r.Expire(deadline.Add(-time.Second), c) || !r.Expire(deadline, c) {
						t.Fatal("fresh casualty deadline missing or timeout failed")
					}
					r.Settings.HardTimer, now = false, deadline
				} else {
					nextWesteros(t, r, c, now)
				}
			}
			if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Outcome.WatchWon != won || (!won && count(r.Match.Armies["winterfell"])+count(r.Match.Armies["the-shivering-sea"]) != 3) {
				t.Fatal("headless effect did not finish with the correct quota", mode, won)
			}
		}
	}
}

func TestMammothBotChoicesOnlyUsePublicOptionsAndOwnDiscard(t *testing.T) {
	for _, won := range []bool{false, true} {
		r, c, now := mammothFixture(t, won)
		r.Match.Discard["stark"] = []string{"eddard-stark", "catelyn-stark"}
		r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 4}, {Type: "knight", House: "stark", Count: 1}}
		nextWesteros(t, r, c, now)
		for _, difficulty := range []string{"normal", "hard"} {
			left, right := r.Clone(), r.Clone()
			left.houseSeat("stark").Bot, right.houseSeat("stark").Bot = true, true
			left.houseSeat("stark").Difficulty, right.houseSeat("stark").Difficulty = difficulty, difficulty
			right.Wildlings.Deck[0], right.Wildlings.Deck[1] = right.Wildlings.Deck[1], right.Wildlings.Deck[0]
			if !left.AdvanceBot(c, now) || !right.AdvanceBot(c, now) || !reflect.DeepEqual(left.Match, right.Match) {
				t.Fatal("bot inspected the future deck")
			}
			if won && slices.Contains(left.Match.Discard["stark"], "eddard-stark") {
				t.Fatal("normal/hard bot did not retrieve the strongest offered card")
			}
		}
	}
}
