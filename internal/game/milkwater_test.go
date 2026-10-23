package game

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

func milkwaterHand(t *testing.T, r *Room, c Catalog, house string, hand ...string) {
	t.Helper()
	for _, id := range hand {
		if _, ok := c.Houses[house].Cards[id]; !ok {
			t.Fatal("unknown fixture card", house, id)
		}
	}
	r.Match.Discard[house] = []string{}
	for id := range c.Houses[house].Cards {
		if !slices.Contains(hand, id) {
			r.Match.Discard[house] = append(r.Match.Discard[house], id)
		}
	}
	slices.Sort(r.Match.Discard[house])
}

func finishMilkwater(t *testing.T, r *Room, c Catalog, now time.Time) {
	t.Helper()
	for i := range r.Seats {
		r.Seats[i].Bot = true
	}
	for step := 0; step < 32 && r.Wildlings.Attack != nil; step++ {
		nextWesteros(t, r, c, now)
	}
	if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Card != WildlingMilkwater || !validWildlingDeck(r.Wildlings.Deck) {
		t.Fatal("Milkwater failed to finish and return its card to the deck")
	}
}

func TestMilkwaterVictoryRestoresOnlyHighestBidderAndCannotRepeat(t *testing.T) {
	for _, empty := range []bool{true, false} {
		r, c, now := revealedWildlingEffect(t, WildlingMilkwater, true)
		if !empty {
			milkwaterHand(t, r, c, "stark", "catelyn-stark")
		}
		r.Match.Discard["greyjoy"] = []string{"euron-crows-eye"}
		before := r.Clone()
		nextWesteros(t, r, c, now)
		if len(r.Match.Discard["stark"]) != 0 || len(r.availableCards("stark", c)) != 7 || !reflect.DeepEqual(before.Match.Discard["greyjoy"], r.Match.Discard["greyjoy"]) || r.Wildlings.Last.Card != WildlingMilkwater || r.Westeros.Threat != 0 {
			t.Fatal("winner did not recover the entire hand, or another house was rewarded")
		}
		if !reflect.DeepEqual(before.Match.Power, r.Match.Power) || !reflect.DeepEqual(before.Match.Tracks, r.Match.Tracks) {
			t.Fatal("hand recovery changed unrelated resources")
		}
		copy := r.Clone()
		if r.advanceWildlingEffects(c, now) || !reflect.DeepEqual(copy, r) {
			t.Fatal("completed reward replayed")
		}
		for _, event := range r.Events {
			if hasCyrillic(TranslateSystemText(event.Text, "en")) {
				t.Fatal("untranslated hand recovery", event.Text)
			}
		}
	}
}

func TestMilkwaterLowestDiscardsAllOriginalHighestCardsButNotLowerOnes(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingMilkwater, false)
	milkwaterHand(t, r, c, "stark", "roose-bolton", "greatjon-umber", "catelyn-stark")
	nextWesteros(t, r, c, now)
	choice := r.Wildlings.Attack.Effect.Choice
	if choice.House != "stark" || choice.Kind != WildlingDiscardCard || !slices.Equal(choice.Options, []string{"greatjon-umber", "roose-bolton"}) {
		t.Fatal("lowest bidder was not offered exactly the highest-strength cards")
	}
	for _, id := range []string{"roose-bolton", "greatjon-umber"} {
		if err := r.chooseWildlingEffect("stark", id, c); err != nil {
			t.Fatal(err)
		}
		nextWesteros(t, r, c, now)
	}
	if !slices.Equal(r.availableCards("stark", c), []string{"catelyn-stark"}) || r.Wildlings.Attack.Effect.Choice.House == "stark" {
		t.Fatal("penalty re-evaluated the highest strength after each discard")
	}
	finishMilkwater(t, r, c, now)
}

func TestMilkwaterFAQEqualLastTwoRecoverSixAndLeaveTheLastDiscard(t *testing.T) {
	for _, first := range []string{"roose-bolton", "greatjon-umber"} {
		r, c, now := revealedWildlingEffect(t, WildlingMilkwater, false)
		milkwaterHand(t, r, c, "stark", "roose-bolton", "greatjon-umber")
		nextWesteros(t, r, c, now)
		if err := r.chooseWildlingEffect("stark", first, c); err != nil {
			t.Fatal(err)
		}
		last := "roose-bolton"
		if first == last {
			last = "greatjon-umber"
		}
		if len(r.availableCards("stark", c)) != 1 || !slices.Equal(r.Wildlings.Attack.Effect.RemainingCards, []string{last}) {
			t.Fatal("first discard did not preserve the original final target")
		}
		r = r.Clone()
		nextWesteros(t, r, c, now)
		if !slices.Equal(r.Wildlings.Attack.Effect.Choice.Options, []string{last}) {
			t.Fatal("the final target was skipped because only one card remains")
		}
		if err := r.chooseWildlingEffect("stark", last, c); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(r.Match.Discard["stark"], []string{last}) || len(r.availableCards("stark", c)) != 6 || !slices.Contains(r.availableCards("stark", c), first) {
			t.Fatal("FAQ: last discarded card must remain down, other six return")
		}
		finishMilkwater(t, r, c, now)
		discarded := 0
		for _, event := range r.Events {
			if event.Type == "wildling-discard" && event.House == "stark" {
				discarded++
			}
			if hasCyrillic(TranslateSystemText(event.Text, "en")) {
				t.Fatal("untranslated card discard", event.Text)
			}
		}
		if discarded != 2 || !slices.Equal(r.Match.Discard["stark"], []string{last}) || r.Westeros.Threat != 8 {
			t.Fatal("recovered cards were penalized again or effect resumed incorrectly")
		}
	}
}

func TestMilkwaterOtherHousesDiscardExactlyOneAndSingleCardHandsAreExempt(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingMilkwater, false)
	for _, house := range BaseHouses {
		cards := r.availableCards(house, c)
		milkwaterHand(t, r, c, house, cards[0])
	}
	milkwaterHand(t, r, c, "greyjoy", "euron-crows-eye", "balon-greyjoy")
	original := r.Clone()
	for step := 0; step < 8 && (r.Wildlings.Attack.Effect == nil || r.Wildlings.Attack.Effect.Choice == nil); step++ {
		nextWesteros(t, r, c, now)
	}
	choice := r.Wildlings.Attack.Effect.Choice
	if choice == nil || choice.House != "greyjoy" || len(choice.Options) != 2 {
		t.Fatal("exempt houses must be skipped and another house may choose any held card")
	}
	if err := r.chooseWildlingEffect("greyjoy", "balon-greyjoy", c); err != nil {
		t.Fatal(err)
	}
	finishMilkwater(t, r, c, now)
	for _, house := range BaseHouses {
		if house != "greyjoy" && !reflect.DeepEqual(original.Match.Discard[house], r.Match.Discard[house]) {
			t.Fatal("a house that started with only one card was penalized", house)
		}
	}
	if !slices.Equal(r.availableCards("greyjoy", c), []string{"euron-crows-eye"}) {
		t.Fatal("another house discarded more than its single required card")
	}
}

func TestMilkwaterInvalidForeignAndRepeatedChoicesLeaveStateUntouched(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingMilkwater, false)
	milkwaterHand(t, r, c, "stark", "roose-bolton", "greatjon-umber", "catelyn-stark")
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", "roose-bolton", c) })
	nextWesteros(t, r, c, now)
	for _, invalid := range []struct{ house, id string }{{"greyjoy", "roose-bolton"}, {"outsider", "roose-bolton"}, {"stark", "catelyn-stark"}, {"stark", "eddard-stark"}, {"stark", "euron-crows-eye"}, {"stark", "fiefdoms"}, {"stark", ""}} {
		assertUnchanged(t, r, func() error { return r.chooseWildlingEffect(invalid.house, invalid.id, c) })
	}
	view := r.Wildlings.view("outsider")
	view.Attack.Choice.Options[0], view.Attack.Choice.House = "changed", "changed"
	if r.Wildlings.Attack.Effect.Choice.House != "stark" || slices.Contains(r.Wildlings.Attack.Effect.RemainingCards, "changed") || slices.Contains(r.Wildlings.Attack.Effect.Choice.Options, "changed") {
		t.Fatal("public options alias the pending original penalty set")
	}
	if err := r.chooseWildlingEffect("stark", "roose-bolton", c); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", "roose-bolton", c) })
	nextWesteros(t, r, c, now)
	assertUnchanged(t, r, func() error { return r.chooseWildlingEffect("stark", "roose-bolton", c) })
}

func TestMilkwaterBotsAndHardTimerFinishBothOutcomes(t *testing.T) {
	for _, mode := range []string{"easy", "normal", "hard", "hard-timer"} {
		for _, won := range []bool{false, true} {
			r, c, now := revealedWildlingEffect(t, WildlingMilkwater, won)
			milkwaterHand(t, r, c, "stark", "roose-bolton", "greatjon-umber")
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
					t.Fatal("soft timer made a card choice")
				}
				if mode == "hard-timer" {
					r.Settings.HardTimer = true
					deadline := r.Clock.Deadline
					if r.Expire(deadline.Add(-time.Second), c) || !r.Expire(deadline, c) {
						t.Fatal("hard deadline acted early or failed to discard a legal card")
					}
					r.Settings.HardTimer, now = false, deadline
				} else {
					nextWesteros(t, r, c, now)
				}
			}
			if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Outcome.WatchWon != won || !validWildlingDeck(r.Wildlings.Deck) {
				t.Fatal("bot/deadline effect did not complete", mode, won)
			}
		}
	}
}

func TestMilkwaterBotDiscardsWeakestLegalCardWithoutReadingFutureDeck(t *testing.T) {
	r, c, now := revealedWildlingEffect(t, WildlingMilkwater, false)
	for step := 0; step < 2; step++ {
		nextWesteros(t, r, c, now)
		choice := r.Wildlings.Attack.Effect.Choice
		if step == 0 {
			if err := r.chooseWildlingEffect(choice.House, choice.Options[0], c); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, difficulty := range []string{"normal", "hard"} {
		left, right := r.Clone(), r.Clone()
		house := left.Wildlings.Attack.Effect.Choice.House
		left.houseSeat(house).Bot, right.houseSeat(house).Bot = true, true
		left.houseSeat(house).Difficulty, right.houseSeat(house).Difficulty = difficulty, difficulty
		right.Wildlings.Deck[0], right.Wildlings.Deck[1] = right.Wildlings.Deck[1], right.Wildlings.Deck[0]
		if !left.AdvanceBot(c, now) || !right.AdvanceBot(c, now) || !reflect.DeepEqual(left.Match.Discard, right.Match.Discard) {
			t.Fatal("bot card choice depends on the secret next wildling card")
		}
		last := left.Match.Discard[house][0]
		if c.Houses[house].Cards[last].Strength != 0 {
			t.Fatal("ordinary/hard bot did not discard its weakest eligible card")
		}
	}
}
