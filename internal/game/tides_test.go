package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func tide(id TidesCardID) TidesCard {
	for _, face := range tidesFaces {
		if face.Card.ID == id {
			return face.Card
		}
	}
	panic("invalid test card")
}

func preparedTides(t *testing.T, a, d TidesCardID) (*Room, Catalog, time.Time, *Battle) {
	t.Helper()
	r, c, now := cardBattle(t, "stark", "greyjoy", "greatjon-umber", "dagmar-cleftjaw")
	r.Settings.TidesOfBattle = true
	r.Match.BladeUsed = true
	r.Battle.Stage = "tides"
	r.Battle.Tides = &TidesState{Cards: map[string]TidesCard{"stark": tide(a), "greyjoy": tide(d)}, Deck: []TidesCard{tide(TideSword), tide(TideZero)}}
	return r, c, now, r.Battle
}

func TestTidesDeckContains24WeightedCopies(t *testing.T) {
	want := map[TidesCardID]int{TideZero: 8, TideSword: 4, TideFortification: 4, TideTwo: 4, TideThree: 2, TideSkull: 2}
	for repeat := 0; repeat < 25; repeat++ {
		deck := tidesDeck()
		if len(deck) != 24 {
			t.Fatal("deck must have 24 cards")
		}
		counts := map[TidesCardID]int{}
		for _, card := range deck {
			counts[card.ID]++
			if card != tide(card.ID) {
				t.Fatal("corrupt card face")
			}
		}
		for id, n := range want {
			if counts[id] != n {
				t.Fatalf("%s: %d copies, want %d", id, counts[id], n)
			}
		}
	}
}

func TestTidesDisabledDoesNotApplyPreparedCards(t *testing.T) {
	r, c, now, b := preparedTides(t, TideThree, TideSkull)
	r.Settings.TidesOfBattle = false
	r.advanceTides(c, now)
	result := r.Match.LastCombat
	if result.Attack != 5 || result.Defense != 3 || result.Modifiers != nil || count(b.Army) != 3 {
		t.Fatalf("disabled module affected combat: %+v", result)
	}
}

func TestTidesEachFaceAffectsStrengthAndResults(t *testing.T) {
	for _, face := range tidesFaces {
		t.Run(string(face.Card.ID), func(t *testing.T) {
			r, c, now, _ := preparedTides(t, face.Card.ID, TideZero)
			r.advanceTides(c, now)
			result := r.Match.LastCombat
			if result.Attack != 5+face.Card.Strength || result.Defense != 3 || result.Modifiers["stark"] != face.Card {
				t.Fatalf("bad result: %+v", result)
			}
			if hasCyrillic(TranslateSystemText(r.Log[len(r.Log)-2], "en")) {
				t.Fatal("untranslated tides event")
			}
		})
	}
}

func TestTidesSwordAndFortificationAffectCasualties(t *testing.T) {
	for _, check := range []struct {
		a, d      TidesCardID
		survivors int
	}{{TideSword, TideZero, 1}, {TideSword, TideFortification, 2}, {TideZero, TideFortification, 2}} {
		r, c, now, b := preparedTides(t, check.a, check.d)
		r.advanceTides(c, now)
		if count(b.Survivors) != check.survivors {
			t.Fatalf("%s/%s: %d survivors", check.a, check.d, count(b.Survivors))
		}
	}
}

func TestSkullsAffectBothSidesAndIgnoreFortifications(t *testing.T) {
	r, c, now, b := preparedTides(t, TideSkull, TideSkull)
	r.advanceTides(c, now)
	if count(b.Army) != 2 || count(b.Survivors) != 1 {
		t.Fatalf("skulls did not affect both armies: %+v", b)
	}
	// Blackfish prevents both swords and skulls, not merely fortification icons.
	r, c, now = cardBattle(t, "greyjoy", "stark", "euron-crows-eye", "the-blackfish")
	b = r.Battle
	r.Settings.TidesOfBattle = true
	r.Match.BladeUsed = true
	b.Stage = "tides"
	b.Tides = &TidesState{Cards: map[string]TidesCard{"greyjoy": tide(TideSkull), "stark": tide(TideZero)}}
	r.advanceTides(c, now)
	if count(b.Survivors) != 2 {
		t.Fatal("Blackfish did not prevent skull casualty")
	}
}

func TestWinningSkullCasualtyDoesNotDestroyAllSieges(t *testing.T) {
	units := []Unit{{Type: "siege-engine", House: "stark", Count: 2}, {Type: "footman", House: "stark", Count: 1}}
	left := removeCombatUnits(units, 1, false)
	if unitCount(left, "siege-engine") != 1 || unitCount(left, "footman") != 1 {
		t.Fatalf("skull destroyed too many units: %+v", left)
	}
	if unitCount(removeCombatUnits(units, 0, false), "siege-engine") != 2 {
		t.Fatal("zero casualties destroyed sieges")
	}
	if unitCount(removeCasualties(units, 0), "siege-engine") != 0 {
		t.Fatal("defeated sieges survived")
	}
}

func TestSkullsCanKillWinnerWithoutChangingWhoWon(t *testing.T) {
	r, c, now, b := preparedTides(t, TideSkull, TideSkull)
	b.To = "moat-cailin"
	b.Army = []Unit{{Type: "footman", House: "stark", Count: 1}}
	r.Match.Armies = map[string][]Unit{b.To: {{Type: "footman", House: "greyjoy", Count: 1}}}
	r.Match.Control = map[string]string{b.To: "greyjoy"}
	r.advanceTides(c, now)
	if r.Match.LastCombat.Winner != "stark" || count(r.Match.Armies[b.To]) != 0 || r.Match.Control[b.To] == "stark" {
		t.Fatalf("empty victorious army occupied area: %+v", r.Match)
	}
}

func TestTidesPrivateChoiceAndDeckSurviveReload(t *testing.T) {
	r, c, now, _ := preparedTides(t, TideZero, TideThree)
	r.Match.BladeUsed = false
	r.Match.Tracks["fiefdoms"] = []string{"stark", "greyjoy"}
	r.advanceTides(c, now)
	if r.Phase != "card_effect" || r.Battle.Choice.Kind != ChoiceTidesRedraw {
		t.Fatal("missing private redraw decision")
	}
	for _, house := range []string{"stark", "greyjoy", "spectator"} {
		view := r.View(house, c)
		if view.Battle.ModifiersRevealed {
			t.Fatal("cards revealed before redraw")
		}
		if house == "spectator" && len(view.Battle.Modifiers) != 0 {
			t.Fatal("spectator received a secret draw")
		}
		for owner := range view.Battle.Modifiers {
			if owner != house {
				t.Fatal("opponent received a secret draw")
			}
		}
		encoded, _ := json.Marshal(view)
		if strings.Contains(string(encoded), `"deck"`) || strings.Contains(string(encoded), `"redrawDone"`) {
			t.Fatal("private deck leaked to client")
		}
	}
	r = r.Clone()
	before, _ := json.Marshal(r.Battle.Tides)
	r.advanceCardEffects(c, now)
	after, _ := json.Marshal(r.Battle.Tides)
	if string(before) != string(after) {
		t.Fatal("reload rerolled a card")
	}
	b := r.Battle
	if err := r.chooseAbility("greyjoy", string(TidesRedraw), c, now); err == nil {
		t.Fatal("opponent used the Blade")
	}
	if err := r.chooseAbility("stark", string(TidesRedraw), c, now); err != nil {
		t.Fatal(err)
	}
	if !r.Match.BladeUsed || r.Match.LastCombat.Modifiers["stark"].ID != TideSword || r.Match.LastCombat.BladeRedraw != "stark" || len(b.Tides.Deck) != 1 {
		t.Fatal("redraw was not persisted or consumed the wrong card")
	}
}

func TestTidesBladeBonusAfterRevealAndOncePerRound(t *testing.T) {
	r, c, now, _ := preparedTides(t, TideZero, TideTwo)
	r.Match.BladeUsed = false
	r.Match.Tracks["fiefdoms"] = []string{"stark", "greyjoy"}
	r.advanceTides(c, now)
	if err := r.chooseAbility("stark", string(TidesKeep), c, now); err != nil {
		t.Fatal(err)
	}
	if r.Battle.Choice.Kind != ChoiceTidesBlade || !r.Battle.Tides.Revealed || len(r.View("greyjoy", c).Battle.Modifiers) != 2 {
		t.Fatal("bonus choice must follow public reveal")
	}
	if err := r.chooseAbility("stark", string(TidesRedraw), c, now); err == nil {
		t.Fatal("redraw allowed after reveal")
	}
	if err := r.chooseAbility("stark", string(TidesUseBlade), c, now); err != nil {
		t.Fatal(err)
	}
	if !r.Match.BladeUsed || r.Match.LastCombat.Attack != 6 || r.Match.LastCombat.BladeBonus != "stark" {
		t.Fatal("Blade bonus missing")
	}
	// The isolated combat uses a shortened track and may leave a retreat.
	// Restore a legal completed-action boundary before entering real Westeros.
	r.Match.Tracks["fiefdoms"] = slices.Clone(BaseHouses)
	r.Battle = nil
	r.finishRound(c, now)
	if r.Match.BladeUsed {
		t.Fatal("Blade did not refresh for new round")
	}
}

func TestTidesBotAndHardTimeoutDoNotStall(t *testing.T) {
	r, c, now, _ := preparedTides(t, TideZero, TideTwo)
	r.Match.BladeUsed = false
	r.Match.Tracks["fiefdoms"] = []string{"stark", "greyjoy"}
	r.houseSeat("stark").Bot = true
	r.advanceTides(c, now)
	if !r.AdvanceBot(c, now) || r.Match.LastCombat == nil || !r.Match.BladeUsed {
		t.Fatal("bot failed to resolve its redraw")
	}
	r, c, now, _ = preparedTides(t, TideZero, TideTwo)
	r.Match.BladeUsed = false
	r.Match.Tracks["fiefdoms"] = []string{"stark", "greyjoy"}
	r.Settings.HardTimer = true
	r.advanceTides(c, now)
	if !r.Expire(now.Add(6*time.Minute), c) || !r.Expire(now.Add(12*time.Minute), c) || r.Match.LastCombat == nil {
		t.Fatal("hard timeout stalled Blade choice")
	}
	if r.Match.BladeUsed {
		t.Fatal("timeout spent Blade without player consent")
	}
}

func TestTidesSettingIsHostOnlyAndLockedAfterStart(t *testing.T) {
	c := LoadCatalog()
	r := NewRoom("room", "Test", "host", "Host")
	settings := DefaultSettings()
	settings.TidesOfBattle = true
	if err := r.Apply("host", Command{Type: "settings", Settings: &settings}, c); err != nil {
		t.Fatal(err)
	}
	if !r.Clone().Settings.TidesOfBattle {
		t.Fatal("setting lost in saved room")
	}
	if err := r.Apply("host", Command{Type: "start"}, c); err != nil {
		t.Fatal(err)
	}
	settings.TidesOfBattle = false
	if err := r.Apply("host", Command{Type: "settings", Settings: &settings}, c); err == nil || !r.Settings.TidesOfBattle {
		t.Fatal("module toggled during game")
	}
}
