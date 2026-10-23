package game

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

func revealedWildlingEffect(t *testing.T, card WildlingCardID, won bool) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := preparedWildlings(t, card, 12)
	bids := map[string]int{}
	for _, house := range BaseHouses {
		bids[house] = 1
	}
	bids["stark"] = 0
	if won {
		bids["stark"] = 7
	}
	commitWildlingBids(t, r, bids, now)
	if r.Phase != PhaseWildlingEffects || r.Wildlings.Attack.SelectedHouse != "stark" || r.Wildlings.Attack.WatchWon != won {
		t.Fatal("fixture did not select the intended highest/lowest bidder")
	}
	return r, c, now
}

func TestWildlingRaidersRewardIsTrackDeltaAndCappedAtSix(t *testing.T) {
	for _, supply := range []int{0, 4, 5, 6} {
		r, c, now := revealedWildlingEffect(t, WildlingRaiders, true)
		r.Match.Supply["stark"] = supply
		before := copyBids(r.Match.Supply)
		nextWesteros(t, r, c, now)
		before["stark"] = min(6, supply+1)
		if !reflect.DeepEqual(before, r.Match.Supply) || r.Supply != nil || r.Wildlings.Attack != nil || r.Wildlings.Last.Card != WildlingRaiders {
			t.Fatal("reward recomputed map icons, affected another house or exceeded track bounds")
		}
		copy := r.Clone()
		if r.advanceWildlingEffects(c, now) || !reflect.DeepEqual(copy, r) {
			t.Fatal("reward could be applied twice")
		}
	}
}

func TestWildlingRaidersPenaltyDisbandsLowestFirstAndResumesExactlyOnce(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingRaiders, false)
	order := append([]string{"stark"}, r.Wildlings.Attack.otherHouses()...)
	cursor := r.Westeros.Run.NextCard
	for _, house := range BaseHouses {
		r.Match.Supply[house] = 1
	}
	r.Match.Supply["stark"] = 2
	r.Match.Armies = map[string][]Unit{
		"winterfell":   {{Type: "footman", House: "stark", Count: 3}},
		"white-harbor": {{Type: "footman", House: "stark", Count: 2}},
		"pyke":         {{Type: "footman", House: "greyjoy", Count: 3}},
	}
	deck := slices.Clone(r.Wildlings.Deck)
	for i, house := range order {
		nextWesteros(t, r, c, now)
		wantLevel := 0
		if r.Match.Supply[house] != wantLevel || r.Westeros.Run.NextCard != cursor || r.Westeros.Threat != 12 || !slices.Equal(deck, r.Wildlings.Deck) || r.Wildlings.Last != nil {
			t.Fatal("penalty recomputed icons, advanced the cursor or finalized too early", house)
		}
		for _, waiting := range order[i+1:] {
			if r.Match.Supply[waiting] != 1 {
				t.Fatal("another house was penalized before the previous house resolved")
			}
		}
		if house == "stark" || house == "greyjoy" {
			if r.Phase != PhaseSupply || r.Supply.Source != SupplyFromWildlings || r.Match.ActiveHouse != house || r.View(house, c).Supply.Level != wantLevel {
				t.Fatal("nested supply decision did not select the penalized house")
			}
			deadline := r.Clock.Deadline
			r = r.Clone() // Full persisted continuation, not a local callback.
			copy := r.Clone()
			if r.AdvanceBot(c, now) || r.Expire(deadline, c) || !reflect.DeepEqual(copy, r) {
				t.Fatal("reload or soft timer bypassed the human disband")
			}
			region := "winterfell"
			if house == "greyjoy" {
				region = "pyke"
			}
			if err := r.disband(house, region, map[string]int{"footman": 1}, c, now); err != nil {
				t.Fatal(err)
			}
			if r.Phase != PhaseWildlingEffects || r.Supply != nil || !r.Clock.Deadline.IsZero() || r.Match.Supply[house] != wantLevel {
				t.Fatal("disband failed to return to the saved effect without recounting supply")
			}
		}
	}
	nextWesteros(t, r, c, now)
	if r.Phase != PhaseWesteros || r.Wildlings.Attack != nil || r.Wildlings.Last.Card != WildlingRaiders || r.Westeros.Threat != 8 || r.Westeros.Run.NextCard != cursor || !validWildlingDeck(r.Wildlings.Deck) {
		t.Fatal("completed penalties did not return to the original event boundary")
	}
	for _, event := range r.Events {
		if hasCyrillic(TranslateSystemText(event.Text, "en")) {
			t.Fatal("untranslated supply effect", event.Text)
		}
	}
}

func TestWildlingRaidersSupplyFloorAndHardTimerBotDisbands(t *testing.T) {
	for _, bot := range []bool{false, true} {
		r, c, now := revealedWildlingEffect(t, WildlingRaiders, false)
		for _, house := range BaseHouses {
			r.Match.Supply[house] = 0
		}
		r.Match.Armies = map[string][]Unit{"winterfell": {{Type: "footman", House: "stark", Count: 4}}}
		nextWesteros(t, r, c, now)
		if r.Phase != PhaseSupply || r.Match.Supply["stark"] != 0 {
			t.Fatal("supply floor ignored")
		}
		if bot {
			r.houseSeat("stark").Bot = true
			for i := 0; i < 4 && r.Phase == PhaseSupply; i++ {
				nextWesteros(t, r, c, now)
			}
		} else {
			r.Settings.HardTimer = true
			if !r.Expire(r.Clock.Deadline, c) {
				t.Fatal("hard deadline failed to use the shared supply resolver")
			}
		}
		for i := 0; i < 8 && r.Wildlings.Attack != nil; i++ {
			nextWesteros(t, r, c, now)
		}
		if r.Wildlings.Attack != nil || !r.supplyOK("stark", r.Match.Armies, c) || count(r.Match.Armies["winterfell"]) != 2 {
			t.Fatal("headless reconciliation failed or destroyed more units than needed")
		}
		for _, house := range BaseHouses {
			if r.Match.Supply[house] != 0 {
				t.Fatal("penalty went below zero or recounted icons")
			}
		}
	}
}

func TestWildlingKingRewardChoosesAnyTrackAndPreservesUsedBlade(t *testing.T) {
	for _, track := range influenceTrackOrder {
		r, c, now := revealedWildlingEffect(t, WildlingKing, true)
		r.Match.BladeUsed = true
		original := r.Clone()
		nextWesteros(t, r, c, now)
		choice := r.Wildlings.view("stark").Attack.Choice
		if choice == nil || choice.House != "stark" || len(choice.Options) != 3 || choice.Kind != WildlingChooseTrack || r.Clock.House != "stark" {
			t.Fatal("winner was not offered all three influence tracks")
		}
		deadline := r.Clock.Deadline
		r = r.Clone()
		copy := r.Clone()
		if r.AdvanceBot(c, now) || r.Expire(deadline, c) || !reflect.DeepEqual(copy, r) {
			t.Fatal("human choice bypassed or restarted on reload/soft deadline")
		}
		for _, invalid := range []struct{ house, option string }{{"greyjoy", string(track)}, {"stark", "unknown"}, {"stark", ""}} {
			assertUnchanged(t, r, func() error { return r.chooseWildlingEffect(invalid.house, invalid.option, c) })
		}
		if err := r.chooseWildlingEffect("stark", string(track), c); err != nil {
			t.Fatal(err)
		}
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", string(track), c) })
		for _, current := range influenceTrackOrder {
			want := original.Match.Tracks[string(current)]
			if current == track {
				want = append([]string{"stark"}, slices.Delete(slices.Clone(want), slices.Index(want, "stark"), slices.Index(want, "stark")+1)...)
			}
			if !slices.Equal(want, r.Match.Tracks[string(current)]) {
				t.Fatal("track choice changed relative order or moved a different track", current)
			}
		}
		nextWesteros(t, r, c, now)
		if !r.Match.BladeUsed || r.Wildlings.Attack != nil || r.Wildlings.Last.Card != WildlingKing {
			t.Fatal("dominance transfer refreshed the spent blade or failed to finish")
		}
	}
}

func TestWildlingKingPenaltyMovesLowestFirstThenEachOtherHouse(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingKing, false)
	r.Match.BladeUsed = true
	others := r.Wildlings.Attack.otherHouses()
	original := r.Clone()
	nextWesteros(t, r, c, now)
	for _, track := range influenceTrackOrder {
		ranking := r.Match.Tracks[string(track)]
		if ranking[len(ranking)-1] != "stark" {
			t.Fatal("lowest bidder must resolve all three demotions first")
		}
	}
	for i, house := range others {
		choice := r.Wildlings.Attack.Effect.Choice
		if choice == nil || choice.House != house || !slices.Equal(choice.Options, []string{string(TrackFiefdoms), string(TrackKingsCourt)}) {
			t.Fatal("others not offered only fiefdoms/court in turn order", house)
		}
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect(house, string(TrackIronThrone), c) })
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", string(TrackFiefdoms), c) })
		option := TrackFiefdoms
		if i%2 != 0 {
			option = TrackKingsCourt
		}
		if err := r.chooseWildlingEffect(house, string(option), c); err != nil {
			t.Fatal(err)
		}
		ranking := r.Match.Tracks[string(option)]
		if ranking[len(ranking)-1] != house || r.Wildlings.Last != nil || r.Westeros.Threat != 12 {
			t.Fatal("wrong demotion or prematurely finalized attack")
		}
		r = r.Clone()
		nextWesteros(t, r, c, now.Add(time.Duration(i+1)*time.Minute))
	}
	wantIron := append(slices.Clone(others), "stark")
	if !slices.Equal(wantIron, r.Match.Tracks[string(TrackIronThrone)]) || reflect.DeepEqual(original.Match.Tracks, r.Match.Tracks) || !r.Match.BladeUsed || r.Wildlings.Attack != nil || r.Westeros.Threat != 8 {
		t.Fatal("wrong final tracks, refreshed blade or attack did not finish")
	}
	for _, event := range r.Events {
		if hasCyrillic(TranslateSystemText(event.Text, "en")) {
			t.Fatal("untranslated influence effect", event.Text)
		}
	}
}

func TestWildlingKingChoiceProjectionDoesNotAlias(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingKing, false)
	nextWesteros(t, r, c, now)
	choice := r.Wildlings.view("outsider").Attack.Choice
	choice.House, choice.Options[0] = "changed", "changed"
	if r.Wildlings.Attack.Effect.Choice.House == "changed" || r.Wildlings.Attack.Effect.Choice.Options[0] == "changed" {
		t.Fatal("public effect choices alias the authoritative continuation")
	}
}

func TestWildlingKingHardTimerGetsFreshDeadlinePerHouseAndCompletes(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingKing, false)
	r.Settings.HardTimer = true
	for i := 0; i < 5; i++ {
		nextWesteros(t, r, c, now)
		deadline := r.Clock.Deadline
		if !deadline.After(now) || r.Expire(deadline.Add(-time.Second), c) || !r.Expire(deadline, c) || !r.Clock.Deadline.IsZero() {
			t.Fatal("hard timer skipped a decision, reused a deadline or acted too early")
		}
		now = deadline
	}
	nextWesteros(t, r, c, now)
	if r.Wildlings.Attack != nil || r.Wildlings.Last == nil {
		t.Fatal("hard-timer-only attack did not complete")
	}
}

func TestWildlingNewEffectsBotsFinishBothOutcomes(t *testing.T) {
	for _, card := range []WildlingCardID{WildlingRaiders, WildlingKing} {
		for _, won := range []bool{true, false} {
			for _, difficulty := range []string{"easy", "normal", "hard"} {
				r, c, now := revealedWildlingEffect(t, card, won)
				for i := range r.Seats {
					r.Seats[i].Bot, r.Seats[i].Difficulty = true, difficulty
				}
				for step := 0; step < 30 && r.Wildlings.Attack != nil; step++ {
					nextWesteros(t, r, c, now)
				}
				if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Outcome.WatchWon != won || !validWildlingDeck(r.Wildlings.Deck) {
					t.Fatal("bot-only effect could not complete", card, won, difficulty)
				}
			}
		}
	}
}

func TestWildlingKingBotTrackChoiceDoesNotDependOnFutureCards(t *testing.T) {
	for _, won := range []bool{false, true} {
		for _, difficulty := range []string{"normal", "hard"} {
			r, c, now := revealedWildlingEffect(t, WildlingKing, won)
			nextWesteros(t, r, c, now)
			house := r.Wildlings.Attack.Effect.Choice.House
			r.houseSeat(house).Bot, r.houseSeat(house).Difficulty = true, difficulty
			left, right := r.Clone(), r.Clone()
			right.Wildlings.Deck[0], right.Wildlings.Deck[1] = right.Wildlings.Deck[1], right.Wildlings.Deck[0]
			if !left.AdvanceBot(c, now) || !right.AdvanceBot(c, now) || !reflect.DeepEqual(left.Match.Tracks, right.Match.Tracks) {
				t.Fatal("bot's track decision depends on a future wildling card")
			}
		}
	}
}

func TestWildlingEffectsInvalidBoundaryDoesNotApplyResourceChanges(t *testing.T) {
	for _, card := range baseWildlingCards() {
		for _, corrupt := range []func(*Room){
			func(r *Room) { r.Round++ },
			func(r *Room) { r.Wildlings.Deck[0] = r.Wildlings.Deck[1] },
			func(r *Room) { r.Westeros.Run.Stage = WesterosEventsStep },
		} {
			r, c, now := revealedWildlingEffect(t, card, false)
			corrupt(r)
			copy := r.Clone()
			if r.AdvanceBot(c, now) || !reflect.DeepEqual(copy, r) {
				t.Fatal("invalid continuation mutated resources", card)
			}
		}
	}
}
