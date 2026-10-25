package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
)

func preemptivePenalty(t *testing.T, decision PreemptiveDecision) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := revealedWildlingEffect(t, WildlingPreemptive, false)
	nextWesteros(t, r, c, now)
	choice := r.Wildlings.Attack.Effect.Choice
	if choice.Kind != WildlingChoosePenalty || choice.House != "stark" || len(choice.Options) != 2 {
		t.Fatal("penalty choice missing")
	}
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("greyjoy", string(decision), c) })
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", WildlingFinishReward, c) })
	if err := r.chooseWildlingEffect("stark", string(decision), c); err != nil {
		t.Fatal(err)
	}
	return r, c, now
}

func finishPreemptive(t *testing.T, r *Room, c Catalog, now time.Time) {
	t.Helper()
	for i := range r.Seats {
		r.Seats[i].Bot, r.Seats[i].Difficulty = true, "normal"
	}
	for step := 0; step < 128 && r.Wildlings.Attack != nil; step++ {
		nextWesteros(t, r, c, now)
	}
	if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || !validWildlingDeck(r.Wildlings.Deck) {
		t.Fatal("preemptive resolver stalled or corrupted the deck")
	}
	for _, event := range r.Events {
		if hasCyrillic(TranslateSystemText(event.Text, "en")) {
			t.Fatal("untranslated Preemptive Raid event", event.Text)
		}
	}
}

func TestPreemptiveUnitPenaltyOnlyAffectsLowestAndCanBePartiallyFulfilled(t *testing.T) {
	for _, n := range []int{0, 1, 2, 4} {
		r, c, now := revealedWildlingEffect(t, WildlingPreemptive, false)
		r.Match.Armies = map[string][]Unit{
			"the-shivering-sea": {{Type: "ship", House: "stark", Count: n}},
			"pyke":              {{Type: "knight", House: "greyjoy", Count: 2}},
		}
		nextWesteros(t, r, c, now)
		if err := r.chooseWildlingEffect("stark", string(PreemptiveDestroyUnits), c); err != nil {
			t.Fatal(err)
		}
		before := r.Clone()
		finishPreemptive(t, r, c, now)
		if count(r.Match.Armies["the-shivering-sea"]) != max(0, n-2) || !reflect.DeepEqual(before.Match.Armies["pyke"], r.Match.Armies["pyke"]) || !reflect.DeepEqual(before.Match.Tracks, r.Match.Tracks) || r.Westeros.Threat != 8 {
			t.Fatal("wrong quota, incomplete choice rejected or another house penalized", n)
		}
	}
}

func TestPreemptiveUnitPenaltyRetainsChoicesAndDeferredPortCleanup(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingPreemptive, false)
	r.Match.Armies = map[string][]Unit{
		"white-harbor":         {{Type: "footman", House: "stark", Count: 1}},
		"port-of-white-harbor": {{Type: "ship", House: "stark", Count: 1}},
		"winterfell":           {{Type: "siege-engine", House: "stark", Count: 1}, {Type: "knight", House: "stark", Count: 1}},
	}
	nextWesteros(t, r, c, now)
	if err := r.chooseWildlingEffect("stark", string(PreemptiveDestroyUnits), c); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"white-harbor:footman", "port-of-white-harbor:ship"} {
		nextWesteros(t, r, c, now)
		choice := r.Wildlings.Attack.Effect.Choice
		if !slices.Contains(choice.Options, id) || choice.Kind != WildlingDestroyUnit {
			t.Fatal("casualty option disappeared")
		}
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", "pyke:knight", c) })
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", WildlingFinishReward, c) })
		if err := r.chooseWildlingEffect("stark", id, c); err != nil {
			t.Fatal(err)
		}
		r = r.Clone()
	}
	finishPreemptive(t, r, c, now)
	if count(r.Match.Armies["winterfell"]) != 2 || count(r.Match.Armies["port-of-white-harbor"]) != 0 {
		t.Fatal("port cleanup caused extra casualties")
	}
}

func TestPreemptiveHighestTrackFallsTwoPlacesNotToBottom(t *testing.T) {
	r, c, now := preemptivePenalty(t, PreemptiveLoseInfluence)
	r.Match.Tracks["ironThrone"] = []string{"greyjoy", "lannister", "stark", "baratheon", "tyrell", "martell"}
	r.Match.Tracks["fiefdoms"] = []string{"stark", "greyjoy", "lannister", "baratheon", "tyrell", "martell"}
	r.Match.Tracks["kingsCourt"] = []string{"greyjoy", "lannister", "baratheon", "stark", "tyrell", "martell"}
	r.Match.BladeUsed = true
	before := r.Clone()
	nextWesteros(t, r, c, now)
	if slices.Index(r.Match.Tracks["fiefdoms"], "stark") != 2 || r.bladeHolder() != "greyjoy" || !r.Match.BladeUsed || !slices.Equal(r.Match.Tracks["ironThrone"], before.Match.Tracks["ironThrone"]) || !slices.Equal(r.Match.Tracks["kingsCourt"], before.Match.Tracks["kingsCourt"]) {
		t.Fatal("wrong highest track, offset or dominance transfer")
	}
	finishPreemptive(t, r, c, now)
	if !reflect.DeepEqual(before.Match.Armies, r.Match.Armies) {
		t.Fatal("track option also destroyed units")
	}
}

func TestPreemptiveTiedHighestTrackIsChosenByThroneNotLowest(t *testing.T) {
	r, c, now := preemptivePenalty(t, PreemptiveLoseInfluence)
	r.Match.Tracks["ironThrone"] = []string{"greyjoy", "stark", "lannister", "baratheon", "tyrell", "martell"}
	r.Match.Tracks["fiefdoms"] = slices.Clone(r.Match.Tracks["ironThrone"])
	r.Match.Tracks["kingsCourt"] = []string{"greyjoy", "lannister", "stark", "baratheon", "tyrell", "martell"}
	nextWesteros(t, r, c, now)
	choice := r.Wildlings.Attack.Effect.Choice
	if choice.House != "greyjoy" || choice.Kind != WildlingChooseTrack || !slices.Equal(choice.Options, []string{"ironThrone", "fiefdoms"}) || r.Match.ActiveHouse != "greyjoy" {
		t.Fatal("highest-track tie not delegated to throne")
	}
	r = r.Clone()
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", "ironThrone", c) })
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("greyjoy", "kingsCourt", c) })
	view := r.View("stark", c)
	view.Wildlings.Attack.Choice.Options[0] = "kingsCourt"
	if err := r.chooseWildlingEffect("greyjoy", "ironThrone", c); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("greyjoy", "fiefdoms", c) })
	finishPreemptive(t, r, c, now)
	if slices.Index(r.Match.Tracks["ironThrone"], "stark") != 3 || slices.Index(r.Match.Tracks["fiefdoms"], "stark") != 1 {
		t.Fatal("throne's chosen track not applied exactly once")
	}
}

func TestPreemptiveTrackPenaltyAtBottomIsLegalAndDoesNotWrap(t *testing.T) {
	for _, position := range []int{4, 5} {
		r, c, now := preemptivePenalty(t, PreemptiveLoseInfluence)
		for _, track := range influenceTrackOrder {
			other := slices.DeleteFunc(slices.Clone(BaseHouses), func(h string) bool { return h == "stark" })
			r.Match.Tracks[string(track)] = slices.Insert(other, position, "stark")
		}
		before := r.Clone()
		finishPreemptive(t, r, c, now)
		if slices.Index(r.Match.Tracks["ironThrone"], "stark") != 5 || !slices.Equal(r.Match.Tracks["fiefdoms"], before.Match.Tracks["fiefdoms"]) || !slices.Equal(r.Match.Tracks["kingsCourt"], before.Match.Tracks["kingsCourt"]) {
			t.Fatal("bottom choice rejected or penalty wrapped around")
		}
	}
}

func TestPreemptiveVictoryStartsFreshSixStrengthAttackWithoutAdvancingCursor(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingPreemptive, true)
	cursor, round, power, deck := r.Westeros.Run.NextCard, r.Round, copyBids(r.Match.Power), slices.Clone(r.Wildlings.Deck)
	nextWesteros(t, r, c, now)
	a := r.Wildlings.Attack
	if r.Phase != PhaseWildlingBidding || a.Card != "" || a.Strength != 6 || a.ExcludedHouse != "stark" || len(a.Bids.Participants) != 5 || len(a.Bids.Values) != 0 || r.Westeros.Threat != 0 || r.Westeros.Run.Stage != WesterosAttackStep || r.Westeros.Run.NextCard != cursor || r.Round != round || r.Westeros.Run.Attack.Trigger != WildlingsFromPreemptive || !reflect.DeepEqual(power, r.Match.Power) {
		t.Fatal("repeat attack advanced the round, reused bids or failed to exclude highest")
	}
	if !slices.Equal(r.Wildlings.Deck, append(deck, WildlingPreemptive)) || r.Wildlings.Last.Card != WildlingPreemptive || r.Wildlings.Last.ThreatAfter != 0 {
		t.Fatal("parent card/result not archived before fresh bidding")
	}
	zero := 0
	assertUnchanged(t, r, func() error { return r.submitWildlingBid("stark", &zero, now) })
	for _, house := range BaseHouses {
		view := r.View(house, c).Wildlings.Attack
		if view.ExcludedHouse != "stark" || len(view.Bids.Submitted) != 5 || view.Card != "" || view.Bids.YourBid != nil || view.Bids.Values != nil {
			t.Fatal("repeat attack leaked new card/old contributions or omitted exemption")
		}
		if _, present := view.Bids.Submitted["stark"]; present {
			t.Fatal("excluded house marked as participating")
		}
	}
	before := r.Clone()
	if r.AdvanceBot(c, now) || !reflect.DeepEqual(before, r) {
		t.Fatal("second tick repeated parent or bypassed human bids")
	}
}

func TestPreemptiveExcludedHouseCanResolveBidTiesAndExemptionIsTemporary(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingPreemptive, true)
	r.Match.Tracks["ironThrone"] = slices.Clone(BaseHouses)
	nextWesteros(t, r, c, now)
	// The excluded Stark still holds the throne and breaks ties among five.
	for _, h := range slices.Clone(r.Wildlings.Attack.Bids.Participants) {
		zero := 0
		if err := r.submitWildlingBid(h, &zero, now); err != nil {
			t.Fatal(err)
		}
	}
	if r.Phase != PhaseWildlingTies || r.Wildlings.Attack.TieBreakHouse != "stark" || slices.Contains(r.Wildlings.Attack.Tied, "stark") {
		t.Fatal("excluded throne holder lost tie authority or entered bidder tie")
	}
	assertUnchanged(t, r, func() error { return r.chooseWildlingTie("stark", "stark") })
	if err := r.chooseWildlingTie("stark", "greyjoy"); err != nil {
		t.Fatal(err)
	}
	finishPreemptive(t, r, c, now)
	r.pauseForWildlings(WildlingsFromCard)
	if err := r.beginWildlingAttack(now); err != nil {
		t.Fatal(err)
	}
	if len(r.Wildlings.Attack.Bids.Participants) != 6 || r.Wildlings.Attack.ExcludedHouse != "" {
		t.Fatal("exemption survived beyond the repeat attack")
	}
}

func TestPreemptiveExemptionSkipsRewardsAndPenaltiesOfEveryFollowingCard(t *testing.T) {
	for _, card := range baseWildlingCards() {
		if card == WildlingPreemptive {
			continue
		}
		for _, won := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", card, won), func(t *testing.T) {
				r, c, now := revealedWildlingEffect(t, WildlingPreemptive, true)
				index := slices.Index(r.Wildlings.Deck, card)
				r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
				nextWesteros(t, r, c, now)
				before := r.Clone()
				for _, h := range slices.Clone(r.Wildlings.Attack.Bids.Participants) {
					bid := 1
					if won {
						bid = 0
					}
					if h == "greyjoy" {
						bid = 0
						if won {
							bid = 6
						}
					}
					if err := r.submitWildlingBid(h, &bid, now); err != nil {
						t.Fatal(err)
					}
				}
				finishPreemptive(t, r, c, now)
				if r.Wildlings.Last.Card != card || r.Wildlings.Last.ExcludedHouse != "stark" || r.Wildlings.Last.Outcome.WatchWon != won || r.Match.Power["stark"] != before.Match.Power["stark"] || r.Match.Supply["stark"] != before.Match.Supply["stark"] || !reflect.DeepEqual(r.Match.Discard["stark"], before.Match.Discard["stark"]) || r.Westeros.Threat != 0 {
					t.Fatal("excluded house received a direct effect or repeat threat became 6")
				}
				for _, region := range before.regionIDs("stark") {
					if !reflect.DeepEqual(before.Match.Armies[region], r.Match.Armies[region]) {
						t.Fatal("excluded house lost/gained units", region)
					}
				}
			})
		}
	}
}

func TestPreemptiveBotsAndTimersFinishBothOutcomesWithoutReadingFutureDeck(t *testing.T) {
	for _, mode := range []string{"easy", "normal", "hard", "hard-timer"} {
		for _, won := range []bool{false, true} {
			r, c, now := revealedWildlingEffect(t, WildlingPreemptive, won)
			for i := range r.Seats {
				r.Seats[i].Bot, r.Seats[i].Difficulty = mode != "hard-timer", mode
			}
			for step := 0; step < 128 && r.Wildlings.Attack != nil; step++ {
				if r.Clock.Deadline.IsZero() {
					nextWesteros(t, r, c, now)
					continue
				}
				before := r.Clone()
				if r.Expire(r.Clock.Deadline, c) || !reflect.DeepEqual(before, r) {
					t.Fatal("soft timer made a human decision")
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
				t.Fatal("headless preemptive resolution stalled", mode, won)
			}
		}
	}
	for _, mode := range []string{"normal", "hard"} {
		r, c, now := revealedWildlingEffect(t, WildlingPreemptive, false)
		nextWesteros(t, r, c, now)
		r.houseSeat("stark").Difficulty = mode
		other := r.Clone()
		other.Wildlings.Deck[0], other.Wildlings.Deck[1] = other.Wildlings.Deck[1], other.Wildlings.Deck[0]
		if r.wildlingBotOption(r.houseSeat("stark"), c) != other.wildlingBotOption(other.houseSeat("stark"), c) {
			t.Fatal("penalty decision reads the hidden deck")
		}
	}
}
