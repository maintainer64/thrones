package game

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

func crowKillersFixture(t *testing.T, won bool) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := revealedWildlingEffect(t, WildlingCrowKillers, won)
	r.Match.Armies, r.Match.Control = map[string][]Unit{}, map[string]string{}
	r.Orders = map[string]string{}
	return r, c, now
}

func finishCrowKillers(t *testing.T, r *Room, c Catalog, now time.Time) {
	t.Helper()
	for i := range r.Seats {
		r.Seats[i].Bot, r.Seats[i].Difficulty = true, "normal"
	}
	for step := 0; step < 64 && r.Wildlings.Attack != nil; step++ {
		nextWesteros(t, r, c, now)
	}
	if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Card != WildlingCrowKillers || !validWildlingDeck(r.Wildlings.Deck) {
		t.Fatal("Crow Killers failed to finish or return to the Westeros cursor")
	}
}

func TestCrowKillersRewardIsOptionalAndNeverMoreThanTwo(t *testing.T) {
	for _, upgrades := range []int{0, 1, 2} {
		r, c, now := crowKillersFixture(t, true)
		r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 3}}
		r.Match.Armies["pyke"] = []Unit{{Type: "footman", House: "greyjoy", Count: 2}}
		original := r.Clone()
		nextWesteros(t, r, c, now)
		if r.Wildlings.Attack.Effect.Choice.Count != 2 || !slices.Equal(r.Wildlings.Attack.Effect.Choice.Options, []string{"winterfell", WildlingFinishReward}) {
			t.Fatal("up-to-two reward missing or cannot be declined")
		}
		for i := 0; i < upgrades; i++ {
			if err := r.chooseWildlingEffect("stark", "winterfell", c); err != nil {
				t.Fatal(err)
			}
			assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", "winterfell", c) })
			nextWesteros(t, r, c, now)
		}
		if upgrades < 2 {
			if err := r.chooseWildlingEffect("stark", WildlingFinishReward, c); err != nil {
				t.Fatal(err)
			}
			finishCrowKillers(t, r, c, now)
		}
		if r.Wildlings.Attack != nil || r.reserve("stark", c)["knight"] != 5-upgrades || r.reserve("stark", c)["footman"] != 7+upgrades || count(r.Match.Armies["winterfell"]) != 3 || !reflect.DeepEqual(original.Match.Armies["pyke"], r.Match.Armies["pyke"]) {
			t.Fatal("reward lost units, exceeded two upgrades or affected another house")
		}
		if !reflect.DeepEqual(original.Match.Supply, r.Match.Supply) || !reflect.DeepEqual(original.Match.Power, r.Match.Power) || !reflect.DeepEqual(original.Match.Tracks, r.Match.Tracks) {
			t.Fatal("1:1 replacement changed unrelated resources")
		}
	}
}

func TestCrowKillersRewardRespectsKnightsInReserveAndAvailableFootmen(t *testing.T) {
	for _, test := range []struct{ knights, footmen, upgrades int }{{5, 3, 0}, {4, 3, 1}, {0, 1, 1}, {0, 0, 0}} {
		r, c, now := crowKillersFixture(t, true)
		r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: test.footmen}, {Type: "knight", House: "stark", Count: test.knights}}
		finishCrowKillers(t, r, c, now)
		_, knights := r.wildlingUnitRegions("stark", UnitKnight, c)
		_, footmen := r.wildlingUnitRegions("stark", UnitFootman, c)
		if knights != test.knights+test.upgrades || footmen != test.footmen-test.upgrades || r.reserve("stark", c)["knight"] < 0 {
			t.Fatal("reward ignored physical pieces", test, knights, footmen)
		}
	}
}

func TestCrowKillersLossLowestAllAndOthersExactlyTwoInTurnOrder(t *testing.T) {
	r, c, now := crowKillersFixture(t, false)
	r.Match.Armies = map[string][]Unit{
		"winterfell": {{Type: "knight", House: "stark", Count: 3}},
		"pyke":       {{Type: "knight", House: "greyjoy", Count: 3}},
		"sunspear":   {{Type: "knight", House: "martell", Count: 1}},
	}
	order := append([]string{"stark"}, r.Wildlings.Attack.otherHouses()...)
	visited := []string{}
	for step := 0; step < 32 && r.Wildlings.Attack != nil; step++ {
		if r.Wildlings.Attack.Effect != nil && r.Wildlings.Attack.Effect.Choice != nil {
			choice := r.Wildlings.Attack.Effect.Choice
			if choice.Kind != WildlingDowngradeKnight {
				t.Fatal("adequate reserve should replace knights, not destroy them")
			}
			if len(visited) == 0 || visited[len(visited)-1] != choice.House {
				visited = append(visited, choice.House)
			}
			if err := r.chooseWildlingEffect(choice.House, choice.Options[0], c); err != nil {
				t.Fatal(err)
			}
		} else {
			nextWesteros(t, r, c, now)
		}
	}
	wantVisited := []string{}
	for _, house := range order {
		if house == "stark" || house == "greyjoy" || house == "martell" {
			wantVisited = append(wantVisited, house)
		}
	}
	if !slices.Equal(visited, wantVisited) || r.Wildlings.Attack != nil || r.Westeros.Threat != 8 {
		t.Fatal("lowest must resolve first, then other houses in turn order", visited)
	}
	for _, test := range []struct {
		house            string
		knights, footmen int
	}{{"stark", 0, 3}, {"greyjoy", 1, 2}, {"martell", 0, 1}} {
		_, knights := r.wildlingUnitRegions(test.house, UnitKnight, c)
		_, footmen := r.wildlingUnitRegions(test.house, UnitFootman, c)
		if knights != test.knights || footmen != test.footmen {
			t.Fatal("wrong penalty quota", test.house, knights, footmen)
		}
	}
}

func TestCrowKillersFootmanShortageChoosesDestructionBeforeReplacement(t *testing.T) {
	for _, lowest := range []bool{true, false} {
		r, c, now := crowKillersFixture(t, false)
		house, first, second, footRegion := "stark", "white-harbor", "winterfell", "moat-cailin"
		if !lowest {
			house, first, second, footRegion = "greyjoy", "seagard", "pyke", "flints-finger"
		}
		r.Match.Armies[first] = []Unit{{Type: "knight", House: house, Count: 1}}
		r.Match.Armies[second] = []Unit{{Type: "knight", House: house, Count: 2}}
		r.Match.Armies[footRegion] = []Unit{{Type: "footman", House: house, Count: 9}}
		for step := 0; step < 8 && (r.Wildlings.Attack.Effect == nil || r.Wildlings.Attack.Effect.Choice == nil); step++ {
			nextWesteros(t, r, c, now)
		}
		choice := r.Wildlings.Attack.Effect.Choice
		destroy := 1
		if lowest {
			destroy = 2
		}
		if choice.House != house || choice.Kind != WildlingDestroyKnight || choice.Count != destroy {
			t.Fatal("shortage must offer the exact destruction quota first")
		}
		for i := 0; i < destroy; i++ {
			if err := r.chooseWildlingEffect(house, second, c); err != nil {
				t.Fatal(err)
			}
			r = r.Clone()
			nextWesteros(t, r, c, now)
		}
		if r.Wildlings.Attack.Effect.Choice.Kind != WildlingDowngradeKnight || r.Wildlings.Attack.Effect.Choice.Count != 1 {
			t.Fatal("destruction did not continue to the saved replacement quota")
		}
		if err := r.chooseWildlingEffect(house, first, c); err != nil {
			t.Fatal(err)
		}
		finishCrowKillers(t, r, c, now)
		_, footmen := r.wildlingUnitRegions(house, UnitFootman, c)
		_, knights := r.wildlingUnitRegions(house, UnitKnight, c)
		wantKnights := 0
		if !lowest {
			wantKnights = 1
		}
		if footmen != 10 || knights != wantKnights || r.Match.Armies[first][0].Type != "footman" {
			t.Fatal("player's chosen surviving area or physical reserve was ignored", footmen, knights)
		}
	}
}

func TestCrowKillersLastUnitDestructionPreservesOnlyEstablishedControl(t *testing.T) {
	for _, token := range []bool{false, true} {
		r, c, now := crowKillersFixture(t, false)
		r.Match.Armies = map[string][]Unit{
			"white-harbor": {{Type: "knight", House: "stark", Count: 1}},
			"winterfell":   {{Type: "footman", House: "stark", Count: 10}},
		}
		r.Match.Control["white-harbor"] = "stark"
		r.Orders["white-harbor"] = "defend-1"
		if token {
			r.Match.ControlTokens = map[string]string{"white-harbor": "stark"}
		}
		nextWesteros(t, r, c, now)
		if err := r.chooseWildlingEffect("stark", "white-harbor", c); err != nil {
			t.Fatal(err)
		}
		if count(r.Match.Armies["white-harbor"]) != 0 || r.Orders["white-harbor"] != "" || (r.owner("white-harbor") == "stark") != token {
			t.Fatal("last-unit loss manufactured control or removed an established token")
		}
		finishCrowKillers(t, r, c, now)
	}
}

func TestCrowKillersInvalidChoicesAndPublicProjectionDoNotMutateState(t *testing.T) {
	for _, won := range []bool{true, false} {
		r, c, now := crowKillersFixture(t, won)
		r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 1}, {Type: "knight", House: "stark", Count: 1}}
		r.Match.Armies["pyke"] = []Unit{{Type: "footman", House: "greyjoy", Count: 1}, {Type: "knight", House: "greyjoy", Count: 1}}
		nextWesteros(t, r, c, now)
		for _, invalid := range []struct{ house, option string }{{"greyjoy", "winterfell"}, {"stark", "pyke"}, {"stark", "unknown"}, {"stark", "the-shivering-sea"}, {"stark", ""}, {"stark", "eddard-stark"}} {
			assertUnchanged(t, r, func() error { return r.chooseWildlingEffect(invalid.house, invalid.option, c) })
		}
		if !won {
			assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", WildlingFinishReward, c) })
		}
		view := r.Wildlings.view("outsider")
		view.Attack.Choice.Count, view.Attack.Choice.Options[0] = 100, "changed"
		if r.Wildlings.Attack.Effect.Units.Remaining == 100 || slices.Contains(r.Wildlings.Attack.Effect.Choice.Options, "changed") {
			t.Fatal("public choice aliases the private unit continuation")
		}
		if err := r.chooseWildlingEffect("stark", "winterfell", c); err != nil {
			t.Fatal(err)
		}
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", "winterfell", c) })
		finishCrowKillers(t, r, c, now)
		for _, event := range r.Events {
			if hasCyrillic(TranslateSystemText(event.Text, "en")) {
				t.Fatal("untranslated Crow Killers effect", event.Text)
			}
		}
	}
}

func TestCrowKillersBotsAndHardDeadlinesFinishBothOutcomes(t *testing.T) {
	for _, mode := range []string{"easy", "normal", "hard", "hard-timer"} {
		for _, won := range []bool{true, false} {
			r, c, now := crowKillersFixture(t, won)
			r.Match.Armies["winterfell"] = []Unit{{Type: "knight", House: "stark", Count: 3}, {Type: "footman", House: "stark", Count: 9}}
			r.Match.Armies["pyke"] = []Unit{{Type: "knight", House: "greyjoy", Count: 3}, {Type: "footman", House: "greyjoy", Count: 10}}
			for i := range r.Seats {
				r.Seats[i].Bot, r.Seats[i].Difficulty = mode != "hard-timer", mode
			}
			for step := 0; step < 64 && r.Wildlings.Attack != nil; step++ {
				if r.Clock.Deadline.IsZero() {
					nextWesteros(t, r, c, now)
					continue
				}
				copy := r.Clone()
				if r.Expire(r.Clock.Deadline, c) || !reflect.DeepEqual(copy, r) {
					t.Fatal("soft deadline chose units for a human")
				}
				if mode == "hard-timer" {
					r.Settings.HardTimer = true
					deadline := r.Clock.Deadline
					if !deadline.After(now) || r.Expire(deadline.Add(-time.Second), c) || !r.Expire(deadline, c) {
						t.Fatal("unit choice reused an expired clock or failed to expire")
					}
					r.Settings.HardTimer, now = false, deadline
				} else {
					nextWesteros(t, r, c, now)
				}
			}
			if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Outcome.WatchWon != won {
				t.Fatal("headless units effect failed", mode, won)
			}
			for _, house := range BaseHouses {
				if r.reserve(house, c)["footman"] < 0 || r.reserve(house, c)["knight"] < 0 {
					t.Fatal("bot/deadline created pieces beyond their physical limits")
				}
			}
		}
	}
}

func TestCrowKillersBotChoicesDoNotInspectFutureWildlingCards(t *testing.T) {
	for _, won := range []bool{false, true} {
		r, c, now := crowKillersFixture(t, won)
		r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 9}, {Type: "knight", House: "stark", Count: 1}}
		r.Match.Armies["white-harbor"] = []Unit{{Type: "knight", House: "stark", Count: 2}}
		nextWesteros(t, r, c, now)
		for _, difficulty := range []string{"normal", "hard"} {
			left, right := r.Clone(), r.Clone()
			left.houseSeat("stark").Bot, right.houseSeat("stark").Bot = true, true
			left.houseSeat("stark").Difficulty, right.houseSeat("stark").Difficulty = difficulty, difficulty
			right.Wildlings.Deck[0], right.Wildlings.Deck[1] = right.Wildlings.Deck[1], right.Wildlings.Deck[0]
			if !left.AdvanceBot(c, now) || !right.AdvanceBot(c, now) || !reflect.DeepEqual(left.Match.Armies, right.Match.Armies) {
				t.Fatal("unit choice depends on the next hidden card")
			}
		}
	}
}
