package game

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

func hordeFixture(t *testing.T, won bool) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := revealedWildlingEffect(t, WildlingHorde, won)
	r.Match.Armies, r.Match.Control = map[string][]Unit{}, map[string]string{}
	r.Orders = map[string]string{}
	return r, c, now
}

func finishHorde(t *testing.T, r *Room, c Catalog, now time.Time) {
	t.Helper()
	for i := range r.Seats {
		r.Seats[i].Bot, r.Seats[i].Difficulty = true, "normal"
	}
	for step := 0; step < 64 && r.Wildlings.Attack != nil; step++ {
		nextWesteros(t, r, c, now)
	}
	if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Card != WildlingHorde || !validWildlingDeck(r.Wildlings.Deck) {
		t.Fatal("Horde did not finish exactly once")
	}
	before := r.Clone()
	if r.advanceWildlingEffects(c, now) || !reflect.DeepEqual(before, r) {
		t.Fatal("completed effect was applied twice")
	}
	for _, e := range r.Events {
		if hasCyrillic(TranslateSystemText(e.Text, "en")) {
			t.Fatal("untranslated Horde event", e.Text)
		}
	}
}

func TestHordeLowestMustDestroyTwoInOneCastleAcrossReload(t *testing.T) {
	r, c, now := hordeFixture(t, false)
	r.Match.Armies = map[string][]Unit{
		"winterfell":        {{Type: "footman", House: "stark", Count: 1}, {Type: "knight", House: "stark", Count: 1}},
		"white-harbor":      {{Type: "siege-engine", House: "stark", Count: 2}},
		"the-shivering-sea": {{Type: "ship", House: "stark", Count: 2}},
		"pyke":              {{Type: "footman", House: "greyjoy", Count: 2}},
	}
	nextWesteros(t, r, c, now)
	choice := r.Wildlings.Attack.Effect.Choice
	if choice.Count != 2 || len(choice.UnitOptions) != 3 || slices.Contains(choice.Options, "the-shivering-sea:ship") {
		t.Fatal("lowest bidder was allowed to avoid eligible castles")
	}
	for _, target := range []string{"the-shivering-sea:ship", "pyke:footman", WildlingFinishReward, "unknown"} {
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", target, c) })
	}
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("greyjoy", "winterfell:footman", c) })
	if err := r.chooseWildlingEffect("stark", "winterfell:footman", c); err != nil {
		t.Fatal(err)
	}
	r = r.Clone()
	nextWesteros(t, r, c, now)
	choice = r.Wildlings.Attack.Effect.Choice
	if choice.Count != 1 || !slices.Equal(choice.Options, []string{"winterfell:knight"}) || r.Wildlings.Attack.Effect.Units.Region != "winterfell" {
		t.Fatal("reload lost selected castle or recomputed the original >=2 condition")
	}
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", "white-harbor:siege-engine", c) })
	// Public options must not share mutable slices with the persisted state.
	view := r.View("greyjoy", c)
	view.Wildlings.Attack.Choice.Options[0] = "unknown"
	view.Wildlings.Attack.Choice.UnitOptions[0].Region = "unknown"
	if err := r.chooseWildlingEffect("stark", "winterfell:knight", c); err != nil {
		t.Fatal(err)
	}
	finishHorde(t, r, c, now)
	if count(r.Match.Armies["winterfell"]) != 0 || count(r.Match.Armies["white-harbor"]) != 2 || count(r.Match.Armies["the-shivering-sea"]) != 2 || count(r.Match.Armies["pyke"]) != 1 || r.Westeros.Threat != 8 {
		t.Fatal("wrong 2/1 quota, region or threat")
	}
}

func TestHordeLowestFallbackCanLoseUnitsInDifferentAreasAndPorts(t *testing.T) {
	r, c, now := hordeFixture(t, false)
	r.Match.Armies = map[string][]Unit{
		"white-harbor":         {{Type: "footman", House: "stark", Count: 1}},
		"port-of-white-harbor": {{Type: "ship", House: "stark", Count: 1}},
		"winterfell":           {{Type: "knight", House: "stark", Count: 1}},
	}
	nextWesteros(t, r, c, now)
	if r.Wildlings.Attack.Effect.Units.CastleOnly {
		t.Fatal("two different castles with one unit each are not a two-unit castle")
	}
	for _, id := range []string{"white-harbor:footman", "port-of-white-harbor:ship"} {
		if err := r.chooseWildlingEffect("stark", id, c); err != nil {
			t.Fatal(err)
		}
		if id == "white-harbor:footman" {
			r = r.Clone()
			nextWesteros(t, r, c, now)
		}
	}
	finishHorde(t, r, c, now)
	if count(r.Match.Armies["winterfell"]) != 1 || count(r.Match.Armies["port-of-white-harbor"]) != 0 {
		t.Fatal("fallback or deferred port cleanup destroyed an extra unit")
	}
}

func TestHordeLossSkipsEmptyHousesAndCapsAtAvailableUnits(t *testing.T) {
	for _, n := range []int{0, 1} {
		r, c, now := hordeFixture(t, false)
		r.Match.Armies["the-shivering-sea"] = []Unit{{Type: "ship", House: "stark", Count: n}}
		finishHorde(t, r, c, now)
		if count(r.Match.Armies["the-shivering-sea"]) != 0 {
			t.Fatal("not all available units were lost")
		}
	}
}

func TestHordeRewardUsesOnlyOneChosenCastleAndSharedMusterRules(t *testing.T) {
	r, c, now := hordeFixture(t, true)
	r.Match.Control = map[string]string{"winterfell": "stark", "white-harbor": "stark", "moat-cailin": "stark", "pyke": "greyjoy"}
	r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 1}}
	r.Match.Supply["stark"] = 6
	nextWesteros(t, r, c, now)
	if r.Wildlings.Attack.Effect.Choice.Kind != WildlingChooseCastle {
		t.Fatal("castle choice missing")
	}
	for _, target := range []string{"pyke", "the-shivering-sea", "unknown"} {
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", target, c) })
	}
	if err := r.chooseWildlingEffect("stark", "winterfell", c); err != nil {
		t.Fatal(err)
	}
	r = r.Clone() // Selection is saved before entering the nested resolver.
	nextWesteros(t, r, c, now)
	if r.Phase != PhaseMustering || r.Mustering.Source != MusterFromWildlings || r.Mustering.Region != "winterfell" || len(r.Mustering.Points) != 1 || r.Mustering.Points["winterfell"] != 2 {
		t.Fatal("reward offered more than one castle or lost source")
	}
	assertUnchanged(t, r, func() error {
		return r.muster("stark", MusterOption{Region: "white-harbor", Target: "white-harbor", Unit: UnitFootman}, c, now)
	})
	assertUnchanged(t, r, func() error { return r.finishMuster("greyjoy", c, now) })
	if err := r.muster("stark", MusterOption{Region: "winterfell", Target: "winterfell", Unit: UnitKnight, Upgrade: true}, c, now); err != nil {
		t.Fatal(err)
	}
	r = r.Clone()
	if r.Mustering.Points["winterfell"] != 1 {
		t.Fatal("upgrade did not spend exactly one point")
	}
	if err := r.muster("stark", MusterOption{Region: "winterfell", Target: "winterfell", Unit: UnitFootman}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase != PhaseWildlingEffects || r.Mustering != nil {
		t.Fatal("nested resolver returned to the wrong phase")
	}
	finishHorde(t, r, c, now)
	if count(r.Match.Armies["winterfell"]) != 2 || r.Westeros.Threat != 0 {
		t.Fatal("reward was repeated or threat was not reset")
	}
}

func TestHordeRewardShipsSupplyStockAndDecline(t *testing.T) {
	r, c, now := hordeFixture(t, true)
	r.Match.Control["white-harbor"] = "stark"
	nextWesteros(t, r, c, now)
	if err := r.chooseWildlingEffect("stark", "white-harbor", c); err != nil {
		t.Fatal(err)
	}
	nextWesteros(t, r, c, now)
	if err := r.muster("stark", MusterOption{Region: "white-harbor", Target: "port-of-white-harbor", Unit: UnitShip}, c, now); err != nil {
		t.Fatal(err)
	}
	finishHorde(t, r, c, now)
	if count(r.Match.Armies["port-of-white-harbor"]) != 1 {
		t.Fatal("port ship not mustered")
	}
	for _, mode := range []string{"no-castle", "decline", "stock", "supply"} {
		r, c, now := hordeFixture(t, true)
		if mode != "no-castle" {
			r.Match.Control["winterfell"] = "stark"
		}
		if mode == "stock" {
			h := c.Houses["stark"]
			h.Limits = map[string]int{"footman": 0, "knight": 0, "ship": 0, "siege-engine": 0}
			c.Houses["stark"] = h
		}
		if mode == "supply" {
			r.Match.Supply["stark"] = 0
			r.Match.Armies = map[string][]Unit{
				"winterfell":   {{Type: "knight", House: "stark", Count: 2}},
				"white-harbor": {{Type: "knight", House: "stark", Count: 2}},
			}
			// Stock also blocks naval recruitment: this case isolates the
			// supply prohibition on adding a third land unit to either army.
			h := c.Houses["stark"]
			h.Limits = copyBids(h.Limits)
			h.Limits["ship"] = 0
			c.Houses["stark"] = h
		}
		before := r.Clone()
		if mode == "decline" {
			nextWesteros(t, r, c, now)
			if err := r.chooseWildlingEffect("stark", WildlingFinishReward, c); err != nil {
				t.Fatal(err)
			}
		}
		finishHorde(t, r, c, now)
		if !reflect.DeepEqual(before.Match.Armies, r.Match.Armies) {
			t.Fatal("reward ignored refusal, stock or supply", mode)
		}
	}
}

func TestHordeBotsAndTimersCompleteBothOutcomes(t *testing.T) {
	for _, mode := range []string{"easy", "normal", "hard", "hard-timer"} {
		for _, won := range []bool{false, true} {
			r, c, now := hordeFixture(t, won)
			r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 2}}
			r.Match.Control["white-harbor"] = "stark"
			for i := range r.Seats {
				r.Seats[i].Bot, r.Seats[i].Difficulty = mode != "hard-timer", mode
			}
			for step := 0; step < 64 && r.Wildlings.Attack != nil; step++ {
				if r.Clock.Deadline.IsZero() {
					nextWesteros(t, r, c, now)
					continue
				}
				before := r.Clone()
				if r.Expire(r.Clock.Deadline, c) || !reflect.DeepEqual(before, r) {
					t.Fatal("soft timer made a decision")
				}
				if mode == "hard-timer" {
					r.Settings.HardTimer = true
					if !r.Expire(r.Clock.Deadline, c) {
						t.Fatal("hard timer stalled", r.Phase)
					}
					r.Settings.HardTimer = false
				} else {
					nextWesteros(t, r, c, now)
				}
			}
			if r.Wildlings.Attack != nil {
				t.Fatal("headless effect stalled", mode, won)
			}
		}
	}
}

func TestHordeBotDecisionsDoNotReadHiddenDeckOrOtherHands(t *testing.T) {
	for _, won := range []bool{false, true} {
		for _, difficulty := range []string{"normal", "hard"} {
			r, c, now := hordeFixture(t, won)
			r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 2}, {Type: "knight", House: "stark", Count: 1}}
			r.Match.Armies["white-harbor"] = []Unit{{Type: "footman", House: "stark", Count: 2}}
			nextWesteros(t, r, c, now)
			r.houseSeat("stark").Difficulty = difficulty
			other := r.Clone()
			other.Wildlings.Deck[0], other.Wildlings.Deck[1] = other.Wildlings.Deck[1], other.Wildlings.Deck[0]
			other.Match.Discard["greyjoy"] = []string{"euron-crows-eye"}
			left := r.wildlingBotOption(r.houseSeat("stark"), c)
			right := other.wildlingBotOption(other.houseSeat("stark"), c)
			if left != right || !slices.Contains(r.Wildlings.Attack.Effect.Choice.Options, left) {
				t.Fatal("bot inspected hidden state or chose an unadvertised option")
			}
		}
	}
}
