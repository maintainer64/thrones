package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func ravenFixture(t *testing.T) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := fixture(t)
	r.Host = "stark"
	for i := range r.Seats {
		r.Seats[i].Player, r.Seats[i].Ready = r.Seats[i].House, true
	}
	r.Match.Tracks[string(TrackKingsCourt)] = slices.Clone(BaseHouses)
	r.Match.Armies["white-harbor"] = []Unit{{Type: "footman", House: "stark", Count: 1}}
	r.Match.Armies["the-shivering-sea"] = []Unit{{Type: "ship", House: "stark", Count: 1}}
	r.Match.Armies["pyke"] = []Unit{{Type: "footman", House: "greyjoy", Count: 1}}
	r.Orders = map[string]string{"winterfell": "march", "white-harbor": "march-minus", "pyke": "defend-1"}
	r.Phase, r.Match.ActiveHouse = "orders_revealed", ""
	if err := r.ApplyAt("greyjoy", Command{Type: "continue"}, c, now); err != nil {
		t.Fatal(err)
	}
	return r, c, now
}

func TestRavenMandatoryStepOnlyCourtHolderMayDecide(t *testing.T) {
	r, c, now := ravenFixture(t)
	if r.Phase != PhaseRaven || r.Match.ActiveHouse != "stark" || r.Raven.House != "stark" || r.Clock.House != "stark" || r.Match.RavenUsed {
		t.Fatal("Continue bypassed the Raven")
	}
	if len(r.View("greyjoy", c).Orders) != len(r.Orders) {
		t.Fatal("orders must remain public during Raven")
	}
	for _, player := range []string{"greyjoy", "lannister", "baratheon", "tyrell", "martell", "outsider"} {
		for _, choice := range []RavenDecision{RavenSkip, RavenPeek, RavenReplace, RavenKeep, RavenBottom} {
			assertUnchanged(t, r, func() error {
				return r.ApplyAt(player, Command{Type: "raven", Choice: string(choice), Region: "winterfell", Order: "support-1"}, c, now)
			})
		}
	}
	for _, command := range []Command{{Type: "continue"}, {Type: "ready"}, {Type: "order", Region: "winterfell", Order: "support-1"}, {Type: "pass"}} {
		assertUnchanged(t, r, func() error { return r.ApplyAt("stark", command, c, now) })
	}
	if err := r.ApplyAt("stark", Command{Type: "raven", Choice: string(RavenReplace), Region: "winterfell", Order: "support-1"}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Raven != nil || !r.Match.RavenUsed || r.Orders["winterfell"] != "support-1" || r.Phase != "marches" || r.Round != 1 {
		t.Fatal("replacement did not enter real actions")
	}
	assertUnchanged(t, r, func() error { return r.ApplyAt("stark", Command{Type: "raven", Choice: string(RavenSkip)}, c, now) })
}

func TestRavenReplacementUsesExactlyThePlanningTokenRules(t *testing.T) {
	for _, test := range []struct {
		name, region, order string
		prepare             func(*Room)
	}{
		{"no original order", "the-shivering-sea", "support-1", nil},
		{"cannot remove", "winterfell", "", nil},
		{"cannot reuse current token", "winterfell", "march", nil},
		{"unknown order", "winterfell", "invented", nil},
		{"enemy order", "pyke", "support-1", nil},
		{"unknown region", "invented", "support-1", nil},
		{"token already placed", "winterfell", "march-minus", nil},
		{"power at sea", "the-shivering-sea", "power-1", func(r *Room) { r.Orders["the-shivering-sea"] = "raid-1" }},
		{"star limit", "winterfell", "march-star", func(r *Room) {
			for i, id := range []string{"white-harbor", "the-shivering-sea", "moat-cailin"} {
				r.Match.Armies[id] = []Unit{{Type: "footman", House: "stark", Count: 1}}
				r.Orders[id] = []string{"defend-star", "support-star", "raid-star"}[i]
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, c, now := ravenFixture(t)
			if test.prepare != nil {
				test.prepare(r)
			}
			if slices.Contains(r.ravenReplacements("stark", c)[test.region], test.order) {
				t.Fatal("UI advertised an illegal replacement")
			}
			assertUnchanged(t, r, func() error {
				return r.ApplyAt("stark", Command{Type: "raven", Choice: string(RavenReplace), Region: test.region, Order: test.order}, c, now)
			})
		})
	}
	for restriction, order := range map[WesterosRestriction]string{
		WesterosNoRaid: "raid-1", WesterosNoSupport: "support-1", WesterosNoPower: "power-1",
		WesterosNoDefense: "defend-2", WesterosNoMarchPlus: "march-star",
	} {
		r, c, now := ravenFixture(t)
		r.Westeros.Run = &WesterosRun{Round: r.Round, Stage: WesterosFinished, Restriction: restriction}
		if slices.Contains(r.ravenReplacements("stark", c)["winterfell"], order) {
			t.Fatal("UI ignored Westeros restriction", restriction)
		}
		assertUnchanged(t, r, func() error {
			return r.ApplyAt("stark", Command{Type: "raven", Choice: string(RavenReplace), Region: "winterfell", Order: order}, c, now)
		})
	}
}

func TestRavenStarForStarSwapDoesNotCountTheOldToken(t *testing.T) {
	r, c, now := ravenFixture(t)
	r.Orders["winterfell"], r.Orders["white-harbor"], r.Orders["the-shivering-sea"] = "defend-star", "support-star", "raid-star"
	if err := r.ApplyAt("stark", Command{Type: "raven", Choice: string(RavenReplace), Region: "winterfell", Order: "march-star"}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Orders["winterfell"] != "march-star" || !r.Match.RavenUsed {
		t.Fatal("legal star-for-star swap failed")
	}
}

func TestRavenPeekIsPrivateIrrevocableAndPersistsUntilPlacement(t *testing.T) {
	for _, destination := range []RavenDecision{RavenKeep, RavenBottom} {
		t.Run(string(destination), func(t *testing.T) {
			r, c, now := ravenFixture(t)
			deck := slices.Clone(r.Wildlings.Deck)
			assertUnchanged(t, r, func() error { return r.chooseRaven("stark", RavenKeep, "", "", c, now) })
			if err := r.chooseRaven("stark", RavenPeek, "", "", c, now); err != nil {
				t.Fatal(err)
			}
			deadline := r.Clock.Deadline
			r = r.Clone()
			if !r.Match.RavenUsed || r.Raven.Stage != RavenPeeking || r.Raven.Card != deck[0] || !slices.Equal(deck, r.Wildlings.Deck) || !r.Clock.Deadline.Equal(deadline) {
				t.Fatal("peek lost its secret or changed the deck/deadline")
			}
			own := r.View("stark", c)
			foreign := r.View("greyjoy", c)
			if own.Raven.Card != deck[0] || foreign.Raven.Card != "" || foreign.Raven.Options != nil || own.Raven.Options != nil {
				t.Fatal("incorrect personal Raven projection")
			}
			public, _ := json.Marshal(foreign)
			for _, id := range baseWildlingCards() {
				if strings.Contains(string(public), string(id)) {
					t.Fatal("secret leaked in public state/history", id)
				}
			}
			for _, choice := range []RavenDecision{RavenSkip, RavenPeek, RavenReplace, "unknown"} {
				assertUnchanged(t, r, func() error { return r.chooseRaven("stark", choice, "winterfell", "support-1", c, now) })
			}
			if err := r.chooseRaven("stark", destination, "", "", c, now); err != nil {
				t.Fatal(err)
			}
			want := deck
			if destination == RavenBottom {
				want = append(slices.Clone(deck[1:]), deck[0])
			}
			if !slices.Equal(r.Wildlings.Deck, want) || r.Raven != nil || !r.Match.RavenUsed || r.Phase == PhaseRaven {
				t.Fatal("placement lost/reordered cards or left a replayable decision")
			}
			assertUnchanged(t, r, func() error { return r.chooseRaven("stark", destination, "", "", c, now) })
		})
	}
}

func TestRavenNoSecretInPublicHistoryForEitherDestination(t *testing.T) {
	a, c, now := ravenFixture(t)
	b := a.Clone()
	for _, r := range []*Room{a, b} {
		if err := r.chooseRaven("stark", RavenPeek, "", "", c, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.chooseRaven("stark", RavenKeep, "", "", c, now); err != nil {
		t.Fatal(err)
	}
	if err := b.chooseRaven("stark", RavenBottom, "", "", c, now); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Events, b.Events) || !reflect.DeepEqual(a.Log, b.Log) {
		t.Fatal("public history reveals the card destination")
	}
}

func TestRavenRejectsCorruptDeckWithoutMutation(t *testing.T) {
	for _, peeked := range []bool{false, true} {
		r, c, now := ravenFixture(t)
		choice := RavenPeek
		if peeked {
			if err := r.chooseRaven("stark", RavenPeek, "", "", c, now); err != nil {
				t.Fatal(err)
			}
			r.Wildlings.Deck[0], r.Wildlings.Deck[1] = r.Wildlings.Deck[1], r.Wildlings.Deck[0]
			choice = RavenBottom
		} else {
			r.Wildlings.Deck = r.Wildlings.Deck[:8]
		}
		assertUnchanged(t, r, func() error { return r.chooseRaven("stark", choice, "", "", c, now) })
	}
}

func TestRavenSoftAndHardTimeoutsSurviveReloadWithoutSecretLeaks(t *testing.T) {
	for _, hard := range []bool{false, true} {
		for _, peeked := range []bool{false, true} {
			r, c, now := ravenFixture(t)
			r.Settings.HardTimer = hard
			if peeked {
				if err := r.chooseRaven("stark", RavenPeek, "", "", c, now); err != nil {
					t.Fatal(err)
				}
			}
			deadline := r.Clock.Deadline
			r = r.Clone()
			before := r.Clone()
			if r.Expire(deadline.Add(-time.Nanosecond), c) {
				t.Fatal("expired early")
			}
			if r.Expire(deadline, c) != hard {
				t.Fatal("wrong timeout policy", hard, peeked)
			}
			if hard {
				if r.Phase == PhaseRaven || r.Raven != nil || r.Match.RavenUsed != peeked || !slices.Equal(r.Wildlings.Deck, before.Wildlings.Deck) {
					t.Fatal("hard timeout peeked/discarded a card or failed to resume")
				}
			} else if !reflect.DeepEqual(before, r) {
				t.Fatal("soft timeout changed the pending decision")
			}
		}
	}
	r, c, now := ravenFixture(t)
	r.Settings.HardTimer, r.Raven = true, nil
	if r.Expire(now.Add(time.Hour), c) {
		t.Fatal("corrupt state must not be skipped")
	}
}

func TestRavenBotsFinishAndDoNotPlayForHumans(t *testing.T) {
	for _, difficulty := range []string{"easy", "normal", "hard"} {
		r, c, now := ravenFixture(t)
		before := r.Clone()
		if r.AdvanceBot(c, now) || !reflect.DeepEqual(before, r) {
			t.Fatal("bot acted for a human")
		}
		r.houseSeat("stark").Bot, r.houseSeat("stark").Difficulty = true, difficulty
		for step := 0; step < 3 && r.Phase == PhaseRaven; step++ {
			if !r.AdvanceBot(c, now) {
				t.Fatal("Raven bot stalled", difficulty)
			}
			r = r.Clone()
		}
		if r.Phase == PhaseRaven || r.Raven != nil || !validWildlingDeck(r.Wildlings.Deck) {
			t.Fatal("Raven bot did not finish", difficulty)
		}
	}
}

func TestRavenUsedDoesNotResetOnTransferButResetsNextRound(t *testing.T) {
	r, c, now := ravenFixture(t)
	if err := r.chooseRaven("stark", RavenPeek, "", "", c, now); err != nil {
		t.Fatal(err)
	}
	if err := r.chooseRaven("stark", RavenKeep, "", "", c, now); err != nil {
		t.Fatal(err)
	}
	r.moveWildlingInfluence("greyjoy", TrackKingsCourt, true)
	if !r.Match.RavenUsed {
		t.Fatal("token transfer restored an already used Raven")
	}
	r.completeRound(c, now)
	if r.Round != 2 || r.Match.RavenUsed || r.Phase != PhaseWesteros {
		t.Fatal("new round failed to reset Raven")
	}
}

func TestRavenPersonalReplacementOptionsAreIndependentCopies(t *testing.T) {
	r, c, _ := ravenFixture(t)
	view := r.View("stark", c)
	if len(view.Raven.Options) == 0 || r.View("greyjoy", c).Raven.Options != nil {
		t.Fatal("wrong actor options")
	}
	view.Raven.Options["winterfell"][0] = "invalid"
	if slices.Contains(r.View("stark", c).Raven.Options["winterfell"], "invalid") {
		t.Fatal("view aliases authoritative options")
	}
}
