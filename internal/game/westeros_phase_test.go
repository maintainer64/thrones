package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"
)

func preparedWesteros(t *testing.T, cards [3]WesterosCardID, threat int) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c := started(t)
	for i := range r.Seats {
		r.Seats[i].Bot = false
	}
	for i, card := range cards {
		deck := &r.Westeros.Decks[i]
		index := slices.Index(deck.Draw, card)
		if index < 0 {
			t.Fatal("test selected card from the wrong deck", i, card)
		}
		deck.Draw[0], deck.Draw[index] = deck.Draw[index], deck.Draw[0]
	}
	r.Phase, r.Round, r.Westeros.Threat = PhaseWesteros, 2, threat
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if err := r.BeginWesteros(now); err != nil {
		t.Fatal(err)
	}
	return r, c, now
}

func nextWesteros(t *testing.T, r *Room, c Catalog, now time.Time) {
	t.Helper()
	if !r.AdvanceBot(c, now) {
		t.Fatal("expected one durable automatic step", r.Phase, r.Westeros.Run)
	}
}

func TestWesterosPhaseStartsWithoutRoundOneOrDuplicateEffects(t *testing.T) {
	r, c, now := preparedWesteros(t, [3]WesterosCardID{WesterosLastDays, WesterosLastDays, WesterosSeaOfStorms}, 0)
	if r.Westeros.Run.Stage != WesterosThreatStep || r.Westeros.Threat != 0 || len(r.Orders) != 0 || !r.Clock.Deadline.IsZero() {
		t.Fatal("boundary did not clear old planning state or applied threat too early")
	}
	before := r.Clone()
	if err := r.BeginWesteros(now.Add(time.Minute)); err != nil || !reflect.DeepEqual(r, before) {
		t.Fatal("same-boundary retry rerolled events or reset progress", err)
	}
	nextWesteros(t, r, c, now)
	if r.Westeros.Threat != 6 || r.Westeros.Run.NextCard != 0 {
		t.Fatal("three mammoth icons must advance three spaces / six strength")
	}
	for cursor := 1; cursor <= 3; cursor++ {
		nextWesteros(t, r, c, now)
		if r.Westeros.Run.NextCard != cursor || r.Westeros.Threat != 6 {
			t.Fatal("wrong event order or repeated threat growth")
		}
	}
	nextWesteros(t, r, c, now)
	if r.Phase != "planning" || r.Round != 2 || r.Westeros.Run.Stage != WesterosFinished || r.Westeros.Run.Restriction != WesterosNoRaid {
		t.Fatal("did not return to planning in the same round")
	}
	if r.AdvanceBot(c, now) {
		t.Fatal("completed events were applied twice")
	}
	// Normal matches enter the same real event flow from the second round.
	ordinary, _ := started(t)
	ordinary.completeRound(c, now)
	if ordinary.Phase != PhaseWesteros || ordinary.Round != 2 || ordinary.Westeros.Current == nil || ordinary.Westeros.Run == nil || ordinary.Westeros.Run.Stage != WesterosThreatStep {
		t.Fatal("normal round boundary did not enter Westeros")
	}
}

func TestWesterosResumesAfterSupplyAndAllThreeAuctions(t *testing.T) {
	r, c, now := preparedWesteros(t, [3]WesterosCardID{WesterosSupply, WesterosClashOfKings, WesterosRainsOfAutumn}, 0)
	r.Match.Armies = map[string][]Unit{"winterfell": {{Type: "footman", House: "stark", Count: 5}}}
	r.Match.Control = map[string]string{"winterfell": "stark"}
	nextWesteros(t, r, c, now) // threat
	nextWesteros(t, r, c, now) // supply
	if r.Phase != PhaseSupply || r.Westeros.Run.NextCard != 1 || r.Influence != nil || r.AdvanceBot(c, now) {
		t.Fatal("supply did not pause before the second event")
	}
	if err := r.disband("stark", "winterfell", map[string]int{"footman": 4}, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Phase != PhaseWesteros || r.Supply != nil {
		t.Fatal("supply did not return to the continuation")
	}
	nextWesteros(t, r, c, now)
	if r.Phase != PhaseInfluenceBidding || r.Westeros.Run.NextCard != 2 || r.Influence.Track != TrackIronThrone {
		t.Fatal("auction did not follow supply")
	}
	for i := range r.Seats {
		r.Seats[i].Bot = true
	}
	for step := 0; step < 200 && r.Influence != nil; step++ {
		nextWesteros(t, r, c, now)
	}
	if r.Influence != nil || r.Phase != PhaseWesteros || len(r.Match.LastInfluence) != 3 {
		t.Fatal("all three auctions failed to return to the saved cursor")
	}
	nextWesteros(t, r, c, now)
	if r.Westeros.Run.Restriction != WesterosNoMarchPlus || r.Westeros.Run.NextCard != 3 {
		t.Fatal("third event did not follow the auction exactly once")
	}
	nextWesteros(t, r, c, now)
	if r.Westeros.Run.Stage != WesterosFinished || r.Round != 2 {
		t.Fatal("auction completion started an extra round")
	}
}

func TestWesterosMusteringPausesAndResumesTheSecondCard(t *testing.T) {
	r, c, now := preparedWesteros(t, [3]WesterosCardID{WesterosMustering, WesterosLastDays, WesterosWebOfLies}, 0)
	r.Match.Armies = map[string][]Unit{}
	r.Match.Control = map[string]string{"winterfell": "stark"}
	nextWesteros(t, r, c, now)
	nextWesteros(t, r, c, now)
	if r.Phase != PhaseMustering || r.Westeros.Run.NextCard != 1 || r.Westeros.Run.Restriction != WesterosNoRestriction {
		t.Fatal("mustering did not hold the remaining events")
	}
	for step := 0; step < 6 && r.Mustering != nil; step++ {
		if err := r.finishMuster(r.Match.ActiveHouse, c, now); err != nil {
			t.Fatal(err)
		}
	}
	if r.Phase != PhaseWesteros {
		t.Fatal("mustering completed outside Westeros")
	}
	nextWesteros(t, r, c, now)
	nextWesteros(t, r, c, now)
	if r.Westeros.Run.Restriction != WesterosNoSupport {
		t.Fatal("remaining events were lost after mustering")
	}
}

func TestWesterosDominanceChoicesValidateHolderOptionsAndReplay(t *testing.T) {
	for _, event := range []struct {
		card  WesterosCardID
		deck  int
		track InfluenceTrack
	}{
		{WesterosThroneOfBlades, 0, TrackIronThrone},
		{WesterosDarkWings, 1, TrackKingsCourt},
		{WesterosPutToSword, 2, TrackFiefdoms},
	} {
		t.Run(string(event.card), func(t *testing.T) {
			cards := [3]WesterosCardID{WesterosLastDays, WesterosLastDays, WesterosPutToSword}
			cards[event.deck] = event.card
			r, c, now := preparedWesteros(t, cards, 0)
			for step := 0; step <= event.deck+1; step++ {
				nextWesteros(t, r, c, now)
			}
			pending := r.Westeros.Run.Choice
			if pending == nil || pending.Card != event.card || pending.House != r.Match.Tracks[string(event.track)][0] || r.Westeros.Run.NextCard != event.deck+1 {
				t.Fatal("wrong dominance holder, event or cursor", pending)
			}
			for _, option := range pending.Options {
				copy := r.Clone()
				wrong := "stark"
				if wrong == pending.House {
					wrong = "greyjoy"
				}
				before, _ := json.Marshal(copy)
				if copy.chooseWesteros(wrong, option, c, now) == nil || copy.chooseWesteros(pending.House, "invalid", c, now) == nil {
					t.Fatal("accepted wrong house or invalid effect")
				}
				after, _ := json.Marshal(copy)
				if string(before) != string(after) {
					t.Fatal("rejected choice mutated phase or clock")
				}
				if err := copy.chooseWesteros(pending.House, option, c, now); err != nil {
					t.Fatal(err)
				}
				if copy.Westeros.Run.Choice != nil || copy.Westeros.Run.Stage != WesterosEventsStep || copy.Westeros.Run.NextCard != event.deck+1 || copy.chooseWesteros(pending.House, option, c, now) == nil {
					t.Fatal("choice not consumed exactly once")
				}
				if option == WesterosChooseMuster && copy.Phase != PhaseMustering || option == WesterosChooseClash && copy.Phase != PhaseInfluenceBidding {
					t.Fatal("nested effect did not pause")
				}
				if option == WesterosChooseNoDefend && copy.Westeros.Run.Restriction != WesterosNoDefense || option == WesterosChooseNoMarch && copy.Westeros.Run.Restriction != WesterosNoMarchPlus {
					t.Fatal("selected order restriction was not applied")
				}
			}
		})
	}
}

func TestWesterosOrderRestrictionsShareValidatorAndExpireNextRound(t *testing.T) {
	for _, test := range []struct {
		restriction WesterosRestriction
		kind        string
	}{
		{WesterosNoRaid, "raid"}, {WesterosNoSupport, "support"},
		{WesterosNoPower, "power"}, {WesterosNoDefense, "defend"},
		{WesterosNoMarchPlus, "march-star"},
	} {
		t.Run(string(test.restriction), func(t *testing.T) {
			r, c, _ := preparedWesteros(t, [3]WesterosCardID{WesterosLastDays, WesterosLastDays, WesterosWebOfLies}, 0)
			r.Westeros.Run.Stage, r.Westeros.Run.Restriction = WesterosFinished, test.restriction
			r.Phase = "planning"
			seat := r.Seat("host-secret")
			available := r.availableOrders(seat, c)["winterfell"]
			for _, order := range Orders {
				banned := order.Kind == test.kind || order.ID == test.kind
				if banned && (slices.Contains(available, order.ID) || r.validateOrder(seat, "winterfell", order.ID, c) == nil) {
					t.Fatal("API and command validators allow a banned order", order.ID)
				}
			}
			if len(available) == 0 {
				t.Fatal("restriction banned all order families")
			}
			r.fillOrders(seat, c)
			for _, id := range r.Orders {
				order, _ := FindOrder(id)
				if r.westerosOrderRestricted(order) {
					t.Fatal("hard-timer planning assigned a forbidden order")
				}
			}
			if err := r.placeOrder(seat, "winterfell", "", c); err != nil {
				t.Fatal("restriction prevented removing an order")
			}
			r.Round++
			r.Orders = map[string]string{}
			for _, order := range Orders {
				if r.westerosOrderRestricted(order) {
					t.Fatal("restriction leaked into the next round")
				}
			}
		})
	}
}

func TestWesterosEveryPrintedRestrictionEvent(t *testing.T) {
	for card, want := range map[WesterosCardID]WesterosRestriction{
		WesterosSeaOfStorms: WesterosNoRaid, WesterosWebOfLies: WesterosNoSupport,
		WesterosFeastForCrows: WesterosNoPower, WesterosStormOfSwords: WesterosNoDefense,
		WesterosRainsOfAutumn: WesterosNoMarchPlus,
	} {
		r, c, now := preparedWesteros(t, [3]WesterosCardID{WesterosLastDays, WesterosLastDays, card}, 0)
		for step := 0; step < 4; step++ {
			nextWesteros(t, r, c, now)
		}
		if r.Westeros.Run.Restriction != want {
			t.Fatal("wrong restriction for printed card", card, r.Westeros.Run.Restriction)
		}
		for _, event := range r.Events {
			if hasCyrillic(TranslateSystemText(event.Text, "en")) {
				t.Fatal("untranslated event", event.Text)
			}
		}
	}
}

func TestWesterosBoundaryRejectsInvalidTracksThreatAndPendingPreviousRound(t *testing.T) {
	for _, corrupt := range []func(*Room){
		func(r *Room) { r.Westeros.Threat = -2 },
		func(r *Room) { r.Westeros.Threat = 14 },
		func(r *Room) { r.Westeros.Threat = 3 },
		func(r *Room) { r.Match.Tracks["fiefdoms"] = []string{"stark", "stark"} },
		func(r *Room) { r.Westeros.Run = &WesterosRun{Round: 1, Stage: WesterosChoiceStep} },
	} {
		r, _ := started(t)
		r.Phase, r.Round = PhaseWesteros, 2
		corrupt(r)
		before, _ := json.Marshal(r)
		if err := r.BeginWesteros(time.Now().UTC()); err == nil {
			t.Fatal("invalid phase boundary accepted")
		}
		after, _ := json.Marshal(r)
		if string(before) != string(after) {
			t.Fatal("invalid boundary consumed cards or reset state")
		}
	}
}

func TestWesterosPowerIncludesCrownsAndUnblockedOccupiedPorts(t *testing.T) {
	r, c, _ := preparedWesteros(t, [3]WesterosCardID{WesterosLastDays, WesterosGameOfThrones, WesterosPutToSword}, 0)
	c.Boards["westeros"] = Board{Regions: []Region{
		{ID: "crowns", Type: "land", Crowns: 2}, {ID: "home", Type: "land", Home: "stark"},
		{ID: "enemy-occupied", Type: "land", Crowns: 5},
		{ID: "empty-sea", Type: "sea"}, {ID: "friendly-sea", Type: "sea"}, {ID: "enemy-sea", Type: "sea"},
		{ID: "free-port", Type: "port", Neighbors: []string{"empty-sea"}},
		{ID: "friendly-port", Type: "port", Neighbors: []string{"friendly-sea"}},
		{ID: "blocked-port", Type: "port", Neighbors: []string{"enemy-sea"}},
		{ID: "empty-port", Type: "port", Neighbors: []string{"empty-sea"}},
	}}
	r.Match.Control = map[string]string{"crowns": "stark", "home": "stark", "enemy-occupied": "stark", "empty-port": "stark"}
	ship := func(house string) []Unit { return []Unit{{Type: "ship", House: house, Count: 1}} }
	r.Match.Armies = map[string][]Unit{
		"free-port": ship("stark"), "friendly-port": ship("stark"), "blocked-port": ship("stark"),
		"friendly-sea": ship("stark"), "enemy-sea": ship("greyjoy"),
		"enemy-occupied": {{Type: "footman", House: "greyjoy", Count: 1}},
	}
	r.Match.Power["stark"] = 0
	r.westerosGainPower(c)
	if r.Match.Power["stark"] != 4 {
		t.Fatal("wrong crown/port power; home alone grants no extra crown", r.Match.Power)
	}
	r.Match.Power["stark"] = 18
	r.Match.ControlTokens = map[string]string{"placed-1": "stark", "placed-2": "stark"}
	r.westerosGainPower(c)
	if r.Match.Power["stark"] != 18 {
		t.Fatal("power exceeded the physical 20-token pool")
	}
}

func TestWesterosThresholdAttackPrecedesEventsAndCardCanAttackAgain(t *testing.T) {
	for _, watchWon := range []bool{false, true} {
		r, c, now := preparedWesteros(t, [3]WesterosCardID{WesterosLastDays, WesterosLastDays, WesterosWildlings}, 10)
		nextWesteros(t, r, c, now)
		if r.Westeros.Threat != 12 || r.Westeros.Run.NextCard != 0 || r.Westeros.Run.Attack.Trigger != WildlingsFromThreat || r.Westeros.Run.Attack.Strength != 12 {
			t.Fatal("threshold attack failed to interrupt before event I")
		}
		before := r.Clone()
		if r.advanceWesteros(c, now) || r.Expire(now.Add(time.Hour), c) || !reflect.DeepEqual(r, before) {
			t.Fatal("Westeros queue bypassed the pending Night's Watch contributions")
		}
		// This is the internal handoff contract, not a fake implementation of D.
		if err := r.resumeWesterosAfterWildlings(watchWon); err != nil {
			t.Fatal(err)
		}
		want := 8
		if watchWon {
			want = 0
		}
		if r.Westeros.Threat != want {
			t.Fatal("wrong threat after attack: loss moves TWO spaces / FOUR strength")
		}
		for step := 0; step < 3; step++ {
			nextWesteros(t, r, c, now)
		}
		if r.Westeros.Run.NextCard != 3 || r.Westeros.Run.Stage != WesterosAttackStep || r.Westeros.Run.Attack.Trigger != WildlingsFromCard || r.Westeros.Run.Attack.Strength != want {
			t.Fatal("Wildlings Attack was skipped after the threshold attack")
		}
		if err := r.resumeWesterosAfterWildlings(true); err != nil {
			t.Fatal(err)
		}
		nextWesteros(t, r, c, now)
		if r.Westeros.Threat != 0 || r.Phase != "planning" {
			t.Fatal("second handoff did not finish the three-card sequence")
		}
		if err := r.resumeWesterosAfterWildlings(true); err == nil {
			t.Fatal("completed attack can be resolved a second time")
		}
	}
}

func TestWesterosChoiceBotsAndHardSoftTimers(t *testing.T) {
	for _, difficulty := range []string{"easy", "normal", "hard"} {
		r, c, now := preparedWesteros(t, [3]WesterosCardID{WesterosThroneOfBlades, WesterosLastDays, WesterosSeaOfStorms}, 0)
		nextWesteros(t, r, c, now)
		nextWesteros(t, r, c, now)
		choice := r.Westeros.Run.Choice
		if r.AdvanceBot(c, now) {
			t.Fatal("automation took a human's decision before the deadline")
		}
		before := r.Clone()
		if r.Expire(r.Clock.Deadline.Add(time.Second), c) || !reflect.DeepEqual(before, r) {
			t.Fatal("soft timer decided for the player")
		}
		hard := r.Clone()
		hard.Settings.HardTimer = true
		if hard.Expire(hard.Clock.Deadline.Add(-time.Second), c) || !hard.Expire(hard.Clock.Deadline, c) || hard.Westeros.Run.Choice != nil || hard.Phase != PhaseWesteros {
			t.Fatal("hard timer failed to make the safe no-effect choice")
		}
		seat := r.houseSeat(choice.House)
		seat.Bot, seat.Difficulty = true, difficulty
		if !r.AdvanceBot(c, now) || r.Westeros.Run.Choice != nil {
			t.Fatal("bot could not resolve a legal dominance decision")
		}
		if difficulty != "easy" && r.Phase != PhaseMustering {
			t.Fatal("normal/hard policy did not choose mustering")
		}
	}
}

func TestWesterosContinuationProjectionDoesNotAliasChoiceOrAttack(t *testing.T) {
	r, c, now := preparedWesteros(t, [3]WesterosCardID{WesterosThroneOfBlades, WesterosLastDays, WesterosSeaOfStorms}, 0)
	nextWesteros(t, r, c, now)
	nextWesteros(t, r, c, now)
	view := r.View("host-secret", c).Westeros
	view.Run.Choice.House = "changed"
	view.Run.Choice.Options[0] = "changed"
	if r.Westeros.Run.Choice.House == "changed" || r.Westeros.Run.Choice.Options[0] == "changed" {
		t.Fatal("public choice aliases saved continuation")
	}
	r.Westeros.Run.Stage, r.Westeros.Run.Choice = WesterosEventsStep, nil
	r.pauseForWildlings(WildlingsFromCard)
	view = r.View("outsider", c).Westeros
	view.Run.Attack.Strength = -1
	if r.Westeros.Run.Attack.Strength == -1 {
		t.Fatal("public attack aliases saved continuation")
	}
}

func TestWesterosAllBotEventSequencesReachPlanning(t *testing.T) {
	for _, cards := range [][3]WesterosCardID{
		{WesterosSupply, WesterosClashOfKings, WesterosSeaOfStorms},
		{WesterosMustering, WesterosGameOfThrones, WesterosRainsOfAutumn},
		{WesterosThroneOfBlades, WesterosDarkWings, WesterosPutToSword},
	} {
		r, c, now := preparedWesteros(t, cards, 0)
		for i := range r.Seats {
			r.Seats[i].Bot = true
		}
		for step := 0; step < 500 && r.Westeros.Run.Stage != WesterosFinished; step++ {
			nextWesteros(t, r, c, now)
		}
		if r.Westeros.Run.Stage != WesterosFinished || r.Phase != "orders_revealed" || r.Round != 2 || r.Westeros.Run.NextCard != 3 {
			t.Fatal("bot-only event sequence did not complete", cards, r.Phase, r.Westeros.Run)
		}
		for _, id := range r.Orders {
			order, _ := FindOrder(id)
			if r.westerosOrderRestricted(order) {
				t.Fatal("planning bots ignored this round's event restriction", id)
			}
		}
	}
}
