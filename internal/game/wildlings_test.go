package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"
)

func preparedWildlings(t *testing.T, card WildlingCardID, strength int) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := preparedWesteros(t, [3]WesterosCardID{WesterosSupply, WesterosGameOfThrones, WesterosWildlings}, strength)
	for step := 0; step < 10 && !r.pendingWildlingContext(); step++ {
		nextWesteros(t, r, c, now)
	}
	if !r.pendingWildlingContext() {
		t.Fatal("real Westeros events did not trigger an attack")
	}
	index := slices.Index(r.Wildlings.Deck, card)
	if index < 0 {
		t.Fatal("unknown test card")
	}
	r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
	for _, seat := range r.Seats {
		r.Match.Power[seat.House] = 10
	}
	nextWesteros(t, r, c, now)
	if r.Phase != PhaseWildlingBidding || r.Wildlings.Attack.Strength != strength {
		t.Fatal("attack handoff did not start sealed contributions")
	}
	return r, c, now
}

func commitWildlingBids(t *testing.T, r *Room, amounts map[string]int, now time.Time) {
	t.Helper()
	participants := slices.Clone(r.Wildlings.Attack.Bids.Participants)
	for _, house := range participants {
		amount := amounts[house]
		if err := r.submitWildlingBid(house, &amount, now); err != nil {
			t.Fatal(err)
		}
	}
}

func resolveWildlingTie(t *testing.T, r *Room, house string) {
	t.Helper()
	if r.Phase == PhaseWildlingTies {
		if err := r.chooseWildlingTie(r.Wildlings.Attack.TieBreakHouse, house); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWildlingDeckNineUniqueCopiesAndIndependentRooms(t *testing.T) {
	for repeat := 0; repeat < 30; repeat++ {
		a, b := newWildlingState(), newWildlingState()
		if len(a.Deck) != 9 || !validWildlingDeck(a.Deck) || a.Attack != nil || a.Last != nil {
			t.Fatal("invalid fresh wildling deck")
		}
		a.Deck[0] = "changed"
		if !validWildlingDeck(b.Deck) || !validWildlingDeck(baseWildlingCards()) {
			t.Fatal("rooms or base catalogue share a mutable deck")
		}
	}
}

func TestWildlingBidsStayPrivateAndPowerIsNotDebitedEarly(t *testing.T) {
	r, c, now := preparedWildlings(t, WildlingMammoths, 6)
	before := slices.Clone(r.Wildlings.Deck)
	amount := 7
	if err := r.submitWildlingBid("stark", &amount, now); err != nil {
		t.Fatal(err)
	}
	for _, house := range BaseHouses {
		view := r.Wildlings.view(house)
		if r.Match.Power[house] != 10 || view.Attack.Bids.Values != nil || view.Attack.Outcome != nil || view.Attack.Card != "" || view.Remaining != 9 {
			t.Fatal("early debit, sum, outcome or future card leaked", view)
		}
		own := view.Attack.Bids.YourBid
		if (house == "stark") != (own != nil) || (own != nil && *own != 7) || !view.Attack.Bids.Submitted["stark"] {
			t.Fatal("incorrect personal projection")
		}
	}
	if r.AdvanceBot(c, now) || !slices.Equal(before, r.Wildlings.Deck) {
		t.Fatal("automation chose for a human or consumed the hidden card early")
	}
	for _, event := range r.Events {
		if event.Type == "wildling-bid" && event.Text != HouseNames["stark"]+": взнос в Ночной дозор подтверждён" {
			t.Fatal("committed amount leaked in event text")
		}
	}
}

func TestWildlingInvalidMissingAndDuplicateBidsDoNotMutate(t *testing.T) {
	r, _, now := preparedWildlings(t, WildlingSilence, 6)
	negative, tooMuch, zero := -1, 11, 0
	for _, invalid := range []struct {
		house string
		bid   *int
	}{{"stark", nil}, {"stark", &negative}, {"stark", &tooMuch}, {"outsider", &zero}} {
		before, _ := json.Marshal(r)
		if err := r.submitWildlingBid(invalid.house, invalid.bid, now); err == nil {
			t.Fatal("invalid contribution accepted")
		}
		after, _ := json.Marshal(r)
		if string(before) != string(after) {
			t.Fatal("invalid contribution mutated authoritative state")
		}
	}
	if err := r.submitWildlingBid("stark", &zero, now); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(r)
	if err := r.submitWildlingBid("stark", &zero, now); err == nil {
		t.Fatal("a zero bid can be replaced")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("duplicate contribution mutated state")
	}
}

func TestWildlingVictoryAndDefeatChooseOnlyTheRelevantTiedBidders(t *testing.T) {
	for _, test := range []struct {
		strength int
		bids     map[string]int
		won      bool
		tied     []string
	}{
		{6, map[string]int{"stark": 4, "greyjoy": 4, "lannister": 1}, true, []string{"stark", "greyjoy"}},
		{12, map[string]int{"lannister": 1, "baratheon": 1, "tyrell": 1, "martell": 1}, false, []string{"stark", "greyjoy"}},
	} {
		r, _, now := preparedWildlings(t, WildlingSilence, test.strength)
		commitWildlingBids(t, r, test.bids, now)
		a := r.Wildlings.Attack
		actual := slices.Clone(a.Tied)
		slices.Sort(actual)
		want := slices.Clone(test.tied)
		slices.Sort(want)
		if r.Phase != PhaseWildlingTies || a.WatchWon != test.won || !slices.Equal(actual, want) || a.Card != "" || len(r.Wildlings.Deck) != 9 {
			t.Fatal("wrong tied group or card revealed before tie selection", a)
		}
		total := 0
		for _, house := range BaseHouses {
			total += test.bids[house]
			if r.Match.Power[house] != 10-test.bids[house] || r.Wildlings.view(house).Attack.Bids.Values[house] != test.bids[house] {
				t.Fatal("contributions were not simultaneously revealed and debited")
			}
		}
		if a.Total != total {
			t.Fatal("wrong Night's Watch sum")
		}
		wrong := "stark"
		if a.TieBreakHouse == wrong {
			wrong = "greyjoy"
		}
		before, _ := json.Marshal(r)
		if r.chooseWildlingTie(wrong, "stark") == nil || r.chooseWildlingTie(a.TieBreakHouse, "martell") == nil {
			t.Fatal("invalid tie selection accepted")
		}
		after, _ := json.Marshal(r)
		if string(before) != string(after) {
			t.Fatal("rejected tie choice consumed the hidden card")
		}
		if err := r.chooseWildlingTie(a.TieBreakHouse, "stark"); err != nil {
			t.Fatal(err)
		}
		if r.Phase != PhaseWildlingEffects || a.SelectedHouse != "stark" || a.Card != WildlingSilence || len(r.Wildlings.Deck) != 8 {
			t.Fatal("selected bidder or revealed card lost")
		}
		if r.chooseWildlingTie(a.TieBreakHouse, "greyjoy") == nil {
			t.Fatal("resolved tie can be changed")
		}
	}
}

func TestWildlingUniqueBidderSkipsOnlyTheTieDecision(t *testing.T) {
	for _, test := range []struct {
		strength int
		bids     map[string]int
		house    string
	}{
		{6, map[string]int{"stark": 6}, "stark"},
		{12, map[string]int{"greyjoy": 1, "lannister": 1, "baratheon": 1, "tyrell": 1, "martell": 1}, "stark"},
	} {
		r, _, now := preparedWildlings(t, WildlingHorde, test.strength)
		commitWildlingBids(t, r, test.bids, now)
		if r.Phase != PhaseWildlingEffects || r.Wildlings.Attack.SelectedHouse != test.house || r.Wildlings.Attack.Card != WildlingHorde {
			t.Fatal("single highest/lowest bidder should not require a tie choice")
		}
		if r.Wildlings.Attack.WatchWon != (test.strength == 6) {
			t.Fatal("equality must count as a Night's Watch victory")
		}
	}
}

func TestWildlingZeroStrengthStillBidsAndReturnsCardToBottom(t *testing.T) {
	r, c, now := preparedWildlings(t, WildlingSilence, 0)
	order := slices.Clone(r.Wildlings.Deck)
	commitWildlingBids(t, r, map[string]int{}, now)
	if r.Phase != PhaseWildlingTies || !r.Wildlings.Attack.WatchWon || len(r.Wildlings.Attack.Tied) != 6 {
		t.Fatal("zero strength must still allow all six zero contributions")
	}
	resolveWildlingTie(t, r, "stark")
	nextWesteros(t, r, c, now)
	if r.Phase != PhaseWesteros || r.Wildlings.Attack != nil || r.Wildlings.Last.Card != WildlingSilence || r.Westeros.Threat != 0 || r.Westeros.Run.Stage != WesterosEventsStep || r.Westeros.Run.NextCard != 3 {
		t.Fatal("real resolved attack did not return to the exact saved cursor")
	}
	want := append(slices.Clone(order[1:]), order[0])
	if !slices.Equal(r.Wildlings.Deck, want) || !validWildlingDeck(r.Wildlings.Deck) {
		t.Fatal("used wildling card was not buried at the bottom, without shuffling")
	}
	nextWesteros(t, r, c, now)
	if r.Phase != "planning" || r.Round != 2 {
		t.Fatal("completion skipped or added a round")
	}
}

func TestWildlingSkinchangerRewardsAndPenaltiesApplyExactlyOnce(t *testing.T) {
	for _, won := range []bool{true, false} {
		r, c, now := preparedWildlings(t, WildlingSkinchanger, 12)
		bids := map[string]int{}
		for _, house := range BaseHouses {
			bids[house] = 1
		}
		if won {
			bids["stark"] = 7
		} else {
			bids["stark"] = 0
		}
		commitWildlingBids(t, r, bids, now)
		if r.Phase != PhaseWildlingEffects || r.Wildlings.Attack.SelectedHouse != "stark" {
			t.Fatal("wrong highest/lowest bidder")
		}
		nextWesteros(t, r, c, now)
		if r.Wildlings.Last.Outcome.WatchWon != won || r.Wildlings.Attack != nil {
			t.Fatal("skinchanger effect did not complete")
		}
		for _, house := range BaseHouses {
			want := 9
			if won && house == "stark" {
				want = 10
			} else if !won && house == "stark" {
				want = 0
			} else if !won {
				want = 7
			}
			if r.Match.Power[house] != want {
				t.Fatal("wrong skinchanger reward/penalty", house, r.Match.Power[house], want)
			}
		}
		threat := 8
		if won {
			threat = 0
		}
		if r.Westeros.Threat != threat || r.Wildlings.Last.ThreatAfter != threat {
			t.Fatal("loss must reduce threat by two spaces / four strength")
		}
		before := r.Clone()
		if r.advanceWildlingEffects(c, now) || !reflect.DeepEqual(before, r) {
			t.Fatal("completed effect can run twice")
		}
	}
}

func TestWildlingUnknownConsequencesNeverResumeOrRotate(t *testing.T) {
	for _, card := range []WildlingCardID{"unknown", "", "preemptive"} {
		r, c, now := preparedWildlings(t, WildlingSilence, 6)
		commitWildlingBids(t, r, map[string]int{"stark": 6}, now)
		r.Wildlings.Attack.Card = card
		before := r.Clone()
		r.Settings.HardTimer = true
		before.Settings.HardTimer = true
		if r.AdvanceBot(c, now) || r.Expire(now.Add(time.Hour), c) || !reflect.DeepEqual(before, r) || len(r.Wildlings.Deck) != 8 || r.Wildlings.view("stark").Attack.EffectsImplemented {
			t.Fatal("a pending consequence was falsely treated as completed", card)
		}
	}
}

func TestWildlingHardDeadlineFillsOnlyMissingBidsAndDoesNotTimeoutTheTie(t *testing.T) {
	r, c, now := preparedWildlings(t, WildlingSilence, 6)
	one := 1
	if err := r.submitWildlingBid("stark", &one, now); err != nil {
		t.Fatal(err)
	}
	before := r.Clone()
	if r.Expire(r.Clock.Deadline, c) || !reflect.DeepEqual(before, r) {
		t.Fatal("soft deadline decided contributions")
	}
	r.Settings.HardTimer = true
	deadline := r.Clock.Deadline
	if r.Expire(deadline.Add(-time.Second), c) || !r.Expire(deadline, c) || r.Phase != PhaseWildlingTies || r.Wildlings.Attack.Bids.Values["stark"] != 1 || r.Wildlings.Attack.Card != "" {
		t.Fatal("hard deadline changed a committed bid or skipped the tie")
	}
	if !r.Clock.Deadline.After(deadline) || r.Expire(deadline, c) {
		t.Fatal("the new tie decision inherited an expired deadline")
	}
	if !r.Expire(r.Clock.Deadline, c) || r.Phase != PhaseWildlingEffects {
		t.Fatal("hard deadline did not settle the tie")
	}
}

func TestWildlingBotsDoNotInspectHiddenBidsOrFutureCard(t *testing.T) {
	for _, difficulty := range []string{"normal", "hard"} {
		r, c, now := preparedWildlings(t, WildlingSilence, 6)
		r.houseSeat("stark").Bot, r.houseSeat("stark").Difficulty = true, difficulty
		left, right := r.Clone(), r.Clone()
		left.Wildlings.Attack.Bids.Values["greyjoy"] = 0
		right.Wildlings.Attack.Bids.Values["greyjoy"] = 9
		right.Wildlings.Deck[0], right.Wildlings.Deck[1] = right.Wildlings.Deck[1], right.Wildlings.Deck[0]
		if !left.AdvanceBot(c, now) || !right.AdvanceBot(c, now) || left.Wildlings.Attack.Bids.Values["stark"] != right.Wildlings.Attack.Bids.Values["stark"] {
			t.Fatal("bot policy depends on hidden commitments or future wildling card")
		}
	}
	for _, difficulty := range []string{"easy", "normal", "hard"} {
		r, c, now := preparedWildlings(t, WildlingSilence, 6)
		for i := range r.Seats {
			r.Seats[i].Bot, r.Seats[i].Difficulty = true, difficulty
		}
		for step := 0; step < 15 && r.Wildlings.Attack != nil; step++ {
			nextWesteros(t, r, c, now)
		}
		if r.Wildlings.Attack != nil || r.Wildlings.Last == nil {
			t.Fatal("bot-only attack could not complete", difficulty)
		}
	}
}

func TestWildlingCorruptDeckAndInvalidBoundaryDoNotConsumeAnything(t *testing.T) {
	r, c, now := preparedWildlings(t, WildlingSilence, 6)
	r.Phase, r.Wildlings.Attack = PhaseWesteros, nil
	for _, corrupt := range []func(*Room){
		func(copy *Room) { copy.Wildlings.Deck = copy.Wildlings.Deck[:8] },
		func(copy *Room) { copy.Wildlings.Deck[0] = copy.Wildlings.Deck[1] },
		func(copy *Room) { copy.Westeros.Run.Attack.Strength = 5 },
		func(copy *Room) { copy.Westeros.Threat = 14 },
		func(copy *Room) { copy.Round++ },
		func(copy *Room) { copy.Phase = "planning" },
	} {
		copy := r.Clone()
		corrupt(copy)
		before, _ := json.Marshal(copy)
		if copy.beginWildlingAttack(now) == nil {
			t.Fatal("invalid deck or boundary started an attack")
		}
		after, _ := json.Marshal(copy)
		if string(before) != string(after) {
			t.Fatal("invalid boundary mutated authoritative state")
		}
	}
	if r.Wildlings.view("stark").Attack != nil || !r.AdvanceBot(c, now) {
		t.Fatal("valid pending boundary should still start")
	}
}

func TestWildlingSkinchangerCannotTakePlacedControlOrMakePowerNegative(t *testing.T) {
	r, c, now := preparedWildlings(t, WildlingSkinchanger, 12)
	r.Match.ControlTokens = map[string]string{"placed-control": "stark"}
	bids := map[string]int{}
	for _, house := range BaseHouses {
		r.Match.Power[house] = 1
		if house != "stark" {
			bids[house] = 1
		}
	}
	commitWildlingBids(t, r, bids, now)
	nextWesteros(t, r, c, now)
	for _, house := range BaseHouses {
		if r.Match.Power[house] != 0 {
			t.Fatal("penalty should take only the available tokens", house)
		}
	}
	if r.Match.ControlTokens["placed-control"] != "stark" {
		t.Fatal("discarding available power must not retrieve established control")
	}
}

func TestWildlingPublicViewsDeepCopyCurrentAndPastBids(t *testing.T) {
	r, c, now := preparedWildlings(t, WildlingSilence, 0)
	commitWildlingBids(t, r, map[string]int{}, now)
	v := r.Wildlings.view("stark")
	v.Attack.Tied[0] = "changed"
	v.Attack.Bids.Values["stark"] = 100
	v.Attack.Outcome.Total = 100
	if r.Wildlings.Attack.Tied[0] == "changed" || r.Wildlings.Attack.Bids.Values["stark"] != 0 || r.Wildlings.Attack.Total != 0 {
		t.Fatal("public attack projection aliases private state")
	}
	resolveWildlingTie(t, r, "stark")
	nextWesteros(t, r, c, now)
	v = r.Wildlings.view("outsider")
	v.Last.Bids["stark"] = 100
	v.Last.Outcome.SelectedHouse = "changed"
	if r.Wildlings.Last.Bids["stark"] != 0 || r.Wildlings.Last.Outcome.SelectedHouse == "changed" {
		t.Fatal("public past report aliases persisted state")
	}
	for _, event := range r.Events {
		if hasCyrillic(TranslateSystemText(event.Text, "en")) {
			t.Fatal("untranslated wildling event", event.Text)
		}
	}
}
