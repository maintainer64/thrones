package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func influenceFixture(t *testing.T) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := fixture(t)
	r.Phase = PhaseWesteros
	r.Orders = map[string]string{}
	for i := range r.Seats {
		s := &r.Seats[i]
		s.Player = "player-" + s.House
		r.Match.Power[s.House] = 20
	}
	if err := r.BeginClashOfKings(now); err != nil {
		t.Fatal(err)
	}
	return r, c, now
}

func submitBid(t *testing.T, r *Room, c Catalog, house string, amount int, now time.Time) {
	t.Helper()
	if err := r.ApplyAt(r.houseSeat(house).Player, Command{Type: "bid", Bid: &amount}, c, now); err != nil {
		t.Fatal(err)
	}
}

func submitAllBids(t *testing.T, r *Room, c Catalog, bids map[string]int, now time.Time) {
	t.Helper()
	for _, house := range append([]string{}, r.Influence.Bids.Participants...) {
		submitBid(t, r, c, house, bids[house], now)
	}
}

func resolveTies(t *testing.T, r *Room, c Catalog, now time.Time) {
	t.Helper()
	for r.Phase == PhaseInfluenceTies {
		a := r.Influence
		if err := r.ApplyAt(r.houseSeat(a.TieBreakHouse).Player, Command{Type: "influence_tie", Target: a.Tied[0]}, c, now); err != nil {
			t.Fatal(err)
		}
	}
}

func assertUnchanged(t *testing.T, r *Room, action func() error) {
	t.Helper()
	before, _ := json.Marshal(r)
	if err := action(); err == nil {
		t.Fatal("invalid action accepted")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("rejected action changed state")
	}
}

func TestSealedBidsHideAmountsAndAcceptZero(t *testing.T) {
	b := newSealedBids([]string{"stark", "greyjoy", "tyrell"})
	for _, test := range []struct {
		house  string
		amount int
	}{{"outsider", 1}, {"stark", -1}, {"stark", 21}} {
		if err := b.submit(test.house, test.amount, 20); err == nil || len(b.Values) != 0 {
			t.Fatal("invalid bid accepted")
		}
	}
	if err := b.submit("stark", 13, 20); err != nil {
		t.Fatal(err)
	}
	if err := b.submit("greyjoy", 0, 0); err != nil {
		t.Fatal("zero must be a real bid:", err)
	}
	if err := b.submit("greyjoy", 0, 0); err == nil {
		t.Fatal("zero bid committed twice")
	}
	for _, house := range []string{"stark", "greyjoy", "tyrell", "outsider"} {
		v := b.view(house)
		if v.Values != nil || !v.Submitted["stark"] || !v.Submitted["greyjoy"] || v.Submitted["tyrell"] {
			t.Fatalf("wrong sealed view: %+v", v)
		}
		if house == "greyjoy" && (v.YourBid == nil || *v.YourBid != 0) {
			t.Fatal("zero own bid missing")
		}
		if (house == "tyrell" || house == "outsider") && v.YourBid != nil {
			t.Fatal("other house bid exposed")
		}
	}
	if err := b.submit("tyrell", 2, 20); err != nil || !b.complete() {
		t.Fatal("last bid did not complete auction", err)
	}
	v := b.view("stark")
	if !reflect.DeepEqual(v.Values, b.Values) {
		t.Fatal("missing simultaneous reveal")
	}
	v.Values["stark"] = 99
	*v.YourBid = 99
	if b.Values["stark"] != 13 {
		t.Fatal("view aliases secret storage")
	}
}

func TestInfluenceCommandsValidateWithoutMutation(t *testing.T) {
	r, c, now := influenceFixture(t)
	negative, over := -1, 21
	for _, command := range []Command{
		{Type: "bid"}, {Type: "bid", Bid: &negative}, {Type: "bid", Bid: &over},
		{Type: "influence_tie", Target: "stark"}, {Type: "order", Region: "winterfell", Order: "march"},
		{Type: "pass"}, {Type: "ready"}, {Type: "continue"},
	} {
		assertUnchanged(t, r, func() error { return r.ApplyAt("player-stark", command, c, now) })
	}
	assertUnchanged(t, r, func() error { return r.BeginClashOfKings(now) })
	assertUnchanged(t, r, func() error { return r.ApplyAt("outsider", Command{Type: "bid", Bid: &over}, c, now) })
	submitBid(t, r, c, "stark", 13, now)
	assertUnchanged(t, r, func() error { return r.ApplyAt("player-stark", Command{Type: "bid", Bid: &negative}, c, now) })
	for _, s := range r.Seats {
		if r.Match.Power[s.House] != 20 {
			t.Fatal("balance reveals a pending bid")
		}
		v := r.View(s.Player, c)
		if v.Influence.Bids.Values != nil || (s.House != "stark" && v.Influence.Bids.YourBid != nil) {
			t.Fatal("bid leaked through view")
		}
		if strings.Contains(strings.Join(v.Log, "\n"), "ставка 13") {
			t.Fatal("secret bid leaked through log")
		}
	}
}

func TestInfluenceOldThroneHolderResolvesWholeTrack(t *testing.T) {
	r, c, now := influenceFixture(t)
	old := r.Match.Tracks["ironThrone"][0]
	winner := "stark"
	if old == winner {
		winner = "greyjoy"
	}
	bids := map[string]int{}
	for _, house := range BaseHouses {
		bids[house] = 2
	}
	bids[winner] = 10
	submitAllBids(t, r, c, bids, now)
	if r.Influence.TieBreakHouse != old || r.Match.ActiveHouse != old || r.Match.Tracks["ironThrone"][0] != old || r.Influence.Ranking[0] != winner {
		t.Fatal("throne changed before the whole track was assigned")
	}
	assertUnchanged(t, r, func() error {
		return r.ApplyAt(r.houseSeat(winner).Player, Command{Type: "influence_tie", Target: old}, c, now)
	})
	assertUnchanged(t, r, func() error { return r.chooseInfluenceTie(old, winner, now) })
	assertUnchanged(t, r, func() error { return r.chooseInfluenceTie(old, "unknown", now) })
	if len(r.View("player-"+winner, c).Influence.Bids.Values) != 6 {
		t.Fatal("bids not revealed simultaneously")
	}
	resolveTies(t, r, c, now)
	if r.Influence.Track != TrackFiefdoms || r.Influence.TieBreakHouse != winner || r.Match.Tracks["ironThrone"][0] != winner {
		t.Fatal("new holder must decide the following auction")
	}
	for house, bid := range bids {
		if r.Match.Power[house] != 20-bid {
			t.Fatal("incorrect or repeated power debit")
		}
	}
	if len(r.Influence.Bids.Values) != 0 || r.Influence.Stage != AuctionBidding || r.Match.ActiveHouse != "" {
		t.Fatal("next track did not start with fresh sealed bids")
	}
}

func TestInfluenceAllTracksPersistAndResumeWesteros(t *testing.T) {
	r, c, now := influenceFixture(t)
	r.Match.BladeUsed = true
	winners := []string{"stark", "greyjoy", "tyrell"}
	for i, track := range influenceTrackOrder {
		if r.Influence.Track != track {
			t.Fatal("wrong auction order")
		}
		bids := map[string]int{}
		for j, house := range BaseHouses {
			bids[house] = j
		}
		bids[winners[i]] = 6
		submitAllBids(t, r, c, bids, now)
		r = r.Clone()
		resolveTies(t, r, c, now)
		if r.Match.Tracks[string(track)][0] != winners[i] {
			t.Fatal("incorrect winner")
		}
	}
	if r.Phase != PhaseWesteros || r.Influence != nil || !r.Clock.Deadline.IsZero() || r.Match.ActiveHouse != "" {
		t.Fatal("auction did not return control to Westeros")
	}
	if len(r.Match.LastInfluence) != 3 || !r.Match.BladeUsed || r.bladeHolder() != "greyjoy" || r.starLimit("tyrell", c) != 3 {
		t.Fatal("missing results, incorrect holders or a used blade refreshed")
	}
	for _, house := range BaseHouses {
		want := 20
		for _, result := range r.Match.LastInfluence {
			want -= result.Bids[house]
		}
		if r.Match.Power[house] != want || want < 0 {
			t.Fatal("power accounting broken")
		}
	}
	assertUnchanged(t, r, func() error { return r.submitInfluenceBid("stark", new(int), now) })
}

func TestInfluenceTieRankingSurvivesReload(t *testing.T) {
	r, c, now := influenceFixture(t)
	submitAllBids(t, r, c, map[string]int{}, now)
	owner := r.Influence.TieBreakHouse
	target := r.Influence.Tied[len(r.Influence.Tied)-1]
	if err := r.chooseInfluenceTie(owner, target, now); err != nil {
		t.Fatal(err)
	}
	deadline := r.Clock.Deadline
	r = r.Clone()
	if r.Influence.Ranking[0] != target || r.Influence.TieBreakHouse != owner || !r.Clock.Deadline.Equal(deadline) {
		t.Fatal("reload lost pending tie choice")
	}
	assertUnchanged(t, r, func() error { return r.chooseInfluenceTie(owner, target, now) })
	resolveTies(t, r, c, now)
	if r.Match.Tracks["ironThrone"][0] != target || r.Influence.TieBreakHouse != target {
		t.Fatal("explicit tie decision was ignored")
	}
}

func TestInfluenceSoftAndHardTimeouts(t *testing.T) {
	r, c, now := influenceFixture(t)
	submitBid(t, r, c, "stark", 3, now)
	before, _ := json.Marshal(r)
	if r.Expire(now.Add(24*time.Hour), c) {
		t.Fatal("soft timer must not decide")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("soft timer changed state")
	}
	r.Settings.HardTimer = true
	deadline := r.Clock.Deadline
	r = r.Clone()
	if r.Expire(deadline.Add(-time.Nanosecond), c) || !r.Expire(deadline, c) {
		t.Fatal("incorrect deadline")
	}
	if r.Match.Power["stark"] != 17 || r.Influence.Bids.Values["stark"] != 3 || r.Phase != PhaseInfluenceTies {
		t.Fatal("timeout overwrote a committed bid")
	}
	if r.Expire(deadline, c) {
		t.Fatal("fresh tie decision expired immediately")
	}
	for r.Phase == PhaseInfluenceTies {
		if !r.Expire(r.Clock.Deadline, c) {
			t.Fatal("hard timer did not resolve a tie")
		}
	}
	if r.Influence.Track != TrackFiefdoms || len(r.Influence.Bids.Values) != 0 {
		t.Fatal("old timeout spilled into the next track")
	}
}

func TestInfluenceTimeoutLastMissingBidDoesNotSpillIntoNextTrack(t *testing.T) {
	r, c, now := influenceFixture(t)
	r.Settings.HardTimer = true
	participants := append([]string{}, r.Influence.Bids.Participants...)
	// Missing first participant: revealing its zero bid immediately completes
	// this track without ties. No subsequent participant may receive a new bid.
	for i, house := range participants[1:] {
		submitBid(t, r, c, house, i+1, now)
	}
	if !r.Expire(r.Clock.Deadline, c) {
		t.Fatal("timeout did not reveal")
	}
	if r.Influence.Track != TrackFiefdoms || len(r.Influence.Bids.Values) != 0 {
		t.Fatal("expired previous track placed bids in the next one")
	}
}

func TestInfluenceBotsAdvanceOneDecisionAndWaitForHumans(t *testing.T) {
	r, c, now := influenceFixture(t)
	for i := range r.Seats {
		s := &r.Seats[i]
		s.Bot = s.House != "stark"
		s.Difficulty = []string{"easy", "normal", "hard"}[i%3]
	}
	for i := 1; i <= 5; i++ {
		if !r.AdvanceBot(c, now) || len(r.Influence.Bids.Values) != i {
			t.Fatal("automation must persist one bid at a time")
		}
	}
	if r.AdvanceBot(c, now) {
		t.Fatal("bot decided for human")
	}
	submitBid(t, r, c, "stark", 0, now)
	for i := 0; i < 100 && r.Influence != nil; i++ {
		if r.Phase == PhaseInfluenceBidding {
			if _, ok := r.Influence.Bids.Values["stark"]; !ok {
				submitBid(t, r, c, "stark", 0, now)
				continue
			}
		}
		if r.Phase == PhaseInfluenceTies && r.Influence.TieBreakHouse == "stark" {
			if err := r.chooseInfluenceTie("stark", r.Influence.Tied[0], now); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if !r.AdvanceBot(c, now) {
			t.Fatal("auction stalled")
		}
	}
	if r.Influence != nil || r.Phase != PhaseWesteros {
		t.Fatal("bots could not finish all tracks")
	}
}

func TestInfluenceBotIgnoresHiddenOpponentAmounts(t *testing.T) {
	r, c, now := influenceFixture(t)
	bot := r.Influence.Bids.Participants[0]
	other := r.Influence.Bids.Participants[1]
	r.houseSeat(bot).Bot = true
	r.houseSeat(bot).Difficulty = "hard"
	a, b := r.Clone(), r.Clone()
	submitBid(t, a, c, other, 0, now)
	submitBid(t, b, c, other, 20, now)
	if !a.AdvanceBot(c, now) || !b.AdvanceBot(c, now) || a.Influence.Bids.Values[bot] != b.Influence.Bids.Values[bot] {
		t.Fatal("bot read a secret bid")
	}
}

func TestClashCannotBeStartedArbitrarily(t *testing.T) {
	r, _, now := fixture(t)
	assertUnchanged(t, r, func() error { return r.BeginClashOfKings(now) })
	r.Phase = PhaseWesteros
	r.Match.Tracks["kingsCourt"][0] = "unknown"
	assertUnchanged(t, r, func() error { return r.BeginClashOfKings(now) })
}

func TestInfluenceTieChoiceCannotJumpBetweenBidGroups(t *testing.T) {
	r, c, now := influenceFixture(t)
	old := r.Influence.TieBreakHouse
	submitAllBids(t, r, c, map[string]int{"stark": 5, "greyjoy": 5, "baratheon": 3, "lannister": 3, "tyrell": 1, "martell": 0}, now)
	assertUnchanged(t, r, func() error { return r.chooseInfluenceTie(old, "baratheon", now) })
	if err := r.chooseInfluenceTie(old, "greyjoy", now); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Influence.Ranking, []string{"greyjoy", "stark"}) || r.Match.Tracks["ironThrone"][0] != old {
		t.Fatal("wrong group order or premature throne transfer")
	}
	assertUnchanged(t, r, func() error { return r.chooseInfluenceTie(old, "tyrell", now) })
	if err := r.chooseInfluenceTie(old, "lannister", now); err != nil {
		t.Fatal(err)
	}
	want := []string{"greyjoy", "stark", "lannister", "baratheon", "tyrell", "martell"}
	if !reflect.DeepEqual(r.Match.Tracks["ironThrone"], want) {
		t.Fatalf("wrong track: %v", r.Match.Tracks["ironThrone"])
	}
}

func TestInfluenceEventsAreLocalizedWithoutChangingSnapshot(t *testing.T) {
	r, c, now := influenceFixture(t)
	submitAllBids(t, r, c, map[string]int{}, now)
	resolveTies(t, r, c, now)
	before, _ := json.Marshal(r)
	v := LocalizeView(r.View("player-stark", c), "en")
	for _, event := range v.Events {
		if hasCyrillic(event.Text) {
			t.Fatalf("untranslated influence event: %s", event.Text)
		}
	}
	for _, message := range v.Log {
		if hasCyrillic(message) {
			t.Fatalf("untranslated influence log: %s", message)
		}
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("English projection changed saved auction")
	}
}
