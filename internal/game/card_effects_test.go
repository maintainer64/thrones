package game

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode"
)

func cardBattle(t *testing.T, attacker, defender, a, d string) (*Room, Catalog, time.Time) {
	t.Helper()
	r, c, now := fixture(t)
	r.Phase = "combat"
	r.Match.ActiveHouse = attacker
	r.Match.Armies = map[string][]Unit{"white-harbor": {{Type: "footman", House: defender, Count: 2}}}
	r.Match.Control = map[string]string{"winterfell": attacker, "white-harbor": defender}
	r.Match.Garrisons = map[string]int{}
	// Generic card-effect fixtures opt out of the optional Blade decision.
	// Blade-specific fixtures explicitly enable it.
	r.Match.BladeUsed = true
	r.Orders = map[string]string{}
	for i := range r.Seats {
		r.Seats[i].Player = r.Seats[i].House
	}
	r.Battle = &Battle{From: "winterfell", To: "white-harbor", Attacker: attacker, Defender: defender, Army: []Unit{{Type: "footman", House: attacker, Count: 3}}, Cards: map[string]string{attacker: a, defender: d}, Revealed: true, Stage: "before", Done: map[string]bool{}, Order: "march"}
	return r, c, now
}
func applyAbility(t *testing.T, r *Room, c Catalog, now time.Time, value string) {
	t.Helper()
	if r.Battle == nil || r.Battle.Choice == nil {
		t.Fatalf("expected choice, phase %s", r.Phase)
	}
	if err := r.chooseAbility(r.Battle.Choice.House, value, c, now); err != nil {
		t.Fatal(err)
	}
}
func finishRetreat(t *testing.T, r *Room, c Catalog, now time.Time) {
	t.Helper()
	if r.Phase == "retreat" {
		options := r.retreatRegions(c)
		if len(options) == 0 {
			t.Fatal("retreat without options")
		}
		if err := r.retreat(r.retreatChooser(), options[0], c, now); err != nil {
			t.Fatal(err)
		}
	}
}
func unitCount(army []Unit, typ string) int {
	n := 0
	for _, u := range army {
		if u.Type == typ {
			n += u.Count
		}
	}
	return n
}

func TestRussianBaseCardsHaveNamesDescriptionsAndUniqueIDs(t *testing.T) {
	c := LoadCatalog()
	n := 0
	for _, h := range BaseHouses {
		for id, card := range c.Houses[h].Cards {
			n++
			if !card.Implemented || card.Description == "" || !strings.ContainsFunc(card.Name, func(r rune) bool { return unicode.Is(unicode.Cyrillic, r) }) {
				t.Fatalf("missing Russian card: %s %+v", id, card)
			}
		}
	}
	if n != 42 {
		t.Fatal(n)
	}
}
func TestConditionalCardStrengthAndSymbols(t *testing.T) {
	for _, tc := range []struct {
		id, house                string
		attack                   bool
		strength, swords, towers int
		discard, support         bool
	}{
		{"stannis-baratheon", "baratheon", true, 5, 0, 0, false, false},
		{"ser-davos-seaworth", "baratheon", true, 3, 1, 0, true, false},
		{"ser-davos-seaworth", "baratheon", true, 2, 0, 0, false, false},
		{"theon-greyjoy", "greyjoy", false, 3, 1, 0, false, false},
		{"theon-greyjoy", "greyjoy", true, 2, 0, 0, false, false},
		{"nymeria-sand", "martell", true, 1, 1, 0, false, false},
		{"nymeria-sand", "martell", false, 1, 0, 1, false, false},
		{"asha-greyjoy", "greyjoy", true, 1, 2, 1, false, false},
		{"asha-greyjoy", "greyjoy", true, 1, 0, 0, false, true},
	} {
		t.Run(tc.id+map[bool]string{true: "-attack", false: "-defend"}[tc.attack]+map[bool]string{true: "-condition", false: ""}[tc.discard || tc.support], func(t *testing.T) {
			r, c, _ := cardBattle(t, tc.house, "stark", tc.id, "eddard-stark")
			if !tc.attack {
				r.Battle.Attacker, r.Battle.Defender = "stark", tc.house
				r.Battle.Cards = map[string]string{"stark": "eddard-stark", tc.house: tc.id}
			}
			if tc.id == "stannis-baratheon" {
				r.Match.Tracks["ironThrone"] = []string{"stark", tc.house}
			}
			if tc.discard {
				r.Match.Discard[tc.house] = []string{"stannis-baratheon"}
			}
			if tc.support {
				r.Match.Armies["moat-cailin"] = []Unit{{Type: "footman", House: tc.house, Count: 1}}
				r.Orders["moat-cailin"] = "support-1"
			}
			got := r.combatCards(c)[tc.house]
			if got.Strength != tc.strength || got.Swords != tc.swords || got.Towers != tc.towers {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
func TestBalonOnlyCancelsPrintedStrengthNotStannisBonus(t *testing.T) {
	r, c, _ := cardBattle(t, "greyjoy", "baratheon", "balon-greyjoy", "stannis-baratheon")
	r.Match.Tracks["ironThrone"] = []string{"greyjoy", "baratheon"}
	if got := r.combatCards(c)["baratheon"].Strength; got != 1 {
		t.Fatal(got)
	}
}
func TestCatelynDoublesDefenseOrder(t *testing.T) {
	r, c, now := cardBattle(t, "lannister", "stark", "the-hound", "catelyn-stark")
	r.Orders["white-harbor"] = "defend-star"
	r.advanceCardEffects(c, now)
	if r.Match.LastCombat.Defense != 6 {
		t.Fatal(r.Match.LastCombat)
	}
}
func TestParticipatingAndSupportingUnitsReceiveCardBonuses(t *testing.T) {
	for _, tc := range []struct {
		house, card, typ string
		want             int
	}{{"greyjoy", "victarion-greyjoy", "ship", 2}, {"lannister", "ser-kevan-lannister", "footman", 2}} {
		t.Run(tc.card, func(t *testing.T) {
			r, c, _ := cardBattle(t, tc.house, "stark", tc.card, "eddard-stark")
			u := []Unit{{Type: tc.typ, House: tc.house, Count: 2}}
			if got := r.combatArmyStrength(u, tc.house, true, c); got != tc.want*2 {
				t.Fatal(got)
			}
			r.Match.Armies["moat-cailin"] = u
			r.Orders["moat-cailin"] = "support-star"
			if got := r.combatSupport(tc.house, true, c); got != tc.want*2+1 {
				t.Fatal(got)
			}
		})
	}
}
func TestSalladhorAndVictarionResolveInIronThroneOrder(t *testing.T) {
	for _, greyjoyFirst := range []bool{false, true} {
		t.Run(map[bool]string{true: "Salladhor last", false: "Victarion last"}[greyjoyFirst], func(t *testing.T) {
			r, c, _ := cardBattle(t, "greyjoy", "baratheon", "victarion-greyjoy", "salladhor-saan")
			r.Match.Armies["moat-cailin"] = []Unit{{Type: "ship", House: "baratheon", Count: 1}}
			r.Orders["moat-cailin"] = "support-star"
			r.Match.Tracks["ironThrone"] = []string{"baratheon", "greyjoy"}
			want := 2
			if greyjoyFirst {
				r.Match.Tracks["ironThrone"] = []string{"greyjoy", "baratheon"}
				want = 0
			}
			if got := r.combatArmyStrength([]Unit{{Type: "ship", House: "greyjoy", Count: 2}}, "greyjoy", true, c); got != want {
				t.Fatal(got)
			}
			if got := r.combatSupport("baratheon", false, c); got != 2 {
				t.Fatal("support order incorrectly cancelled", got)
			}
		})
	}
}
func TestMaceDestroysFootmanBeforeStrengthButBlackfishPreventsIt(t *testing.T) {
	for _, card := range []string{"catelyn-stark", "the-blackfish"} {
		t.Run(card, func(t *testing.T) {
			r, c, now := cardBattle(t, "tyrell", "stark", "mace-tyrell", card)
			r.advanceCardEffects(c, now)
			want := 1
			if card == "the-blackfish" {
				want = 2
			}
			if r.Match.LastCombat.Defense != want+c.Houses["stark"].Cards[card].Strength {
				t.Fatal(r.Match.LastCombat)
			}
			if r.Battle == nil || unitCount(r.Battle.Survivors, "footman") != want {
				t.Fatalf("wrong survivors %+v", r.Battle)
			}
		})
	}
}
func TestBlackfishPreventsSwordCasualties(t *testing.T) {
	r, c, now := cardBattle(t, "lannister", "stark", "ser-gregor-clegane", "the-blackfish")
	r.advanceCardEffects(c, now)
	if r.Battle == nil || count(r.Battle.Survivors) != 2 {
		t.Fatal("blackfish did not protect army")
	}
}
func TestTyrionCancelReplacementAndAeronCanReplayCancelledCard(t *testing.T) {
	r, c, now := cardBattle(t, "lannister", "greyjoy", "tyrion-lannister", "balon-greyjoy")
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "cancel")
	if contains(r.Battle.Choice.Options, "balon-greyjoy") || r.Battle.Choice.House != "greyjoy" {
		t.Fatal("Tyrion offered cancelled card")
	}
	applyAbility(t, r, c, now, "aeron-damphair")
	if !contains(r.Battle.Choice.Options, "balon-greyjoy") {
		t.Fatal("Aeron must be able to replay returned card")
	}
	power := r.Match.Power["greyjoy"]
	applyAbility(t, r, c, now, "balon-greyjoy")
	if r.Match.Power["greyjoy"] != power-2 || !contains(r.Match.Discard["greyjoy"], "aeron-damphair") || r.Match.LastCombat.DefenderCard != "balon-greyjoy" {
		t.Fatal("wrong Aeron replacement")
	}
}
func TestTyrionCancelsLastCardLeavingNoCardForCombat(t *testing.T) {
	r, c, now := cardBattle(t, "lannister", "greyjoy", "tyrion-lannister", "balon-greyjoy")
	for id := range c.Houses["greyjoy"].Cards {
		if id != "balon-greyjoy" {
			r.Match.Discard["greyjoy"] = append(r.Match.Discard["greyjoy"], id)
		}
	}
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "cancel")
	if r.Match.LastCombat.DefenderCard != "" || r.Match.LastCombat.Defense != 2 {
		t.Fatal("last card not cancelled", r.Match.LastCombat)
	}
}
func TestCardChoiceIsPrivatePersistentAndInvalidCommandsAreReadOnly(t *testing.T) {
	r, c, now := cardBattle(t, "lannister", "greyjoy", "tyrion-lannister", "balon-greyjoy")
	r.advanceCardEffects(c, now)
	r = r.Clone()
	before, _ := json.Marshal(r)
	if r.View("greyjoy", c).Battle.Choice.Options != nil {
		t.Fatal("choice options leaked")
	}
	for _, cmd := range []struct{ h, value string }{{"greyjoy", "cancel"}, {"lannister", "run-command"}} {
		if err := r.chooseAbility(cmd.h, cmd.value, c, now); err == nil {
			t.Fatal("invalid choice accepted")
		}
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("rejected command mutated state")
	}
	applyAbility(t, r, c, now, "")
	if r.Match.LastCombat == nil {
		t.Fatal("declining Tyrion failed")
	}
}
func TestQueenRemovesSupportBeforeCombatAndDoranChangesTrack(t *testing.T) {
	r, c, now := cardBattle(t, "tyrell", "martell", "queen-of-thorns", "doran-martell")
	r.Match.Armies["moat-cailin"] = []Unit{{Type: "footman", House: "martell", Count: 2}}
	r.Orders["moat-cailin"] = "support-star"
	r.Match.Tracks["ironThrone"] = []string{"tyrell", "martell"}
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "moat-cailin")
	if r.Orders["moat-cailin"] != "" {
		t.Fatal("support not removed")
	}
	applyAbility(t, r, c, now, "fiefdoms")
	track := r.Match.Tracks["fiefdoms"]
	if track[len(track)-1] != "tyrell" || r.Match.LastCombat.Defense != 2 {
		t.Fatal("Doran/Queen effects failed", track, r.Match.LastCombat)
	}
}
func TestArianneReturnsVictoriousArmyAndPreventsLorasOrder(t *testing.T) {
	r, c, now := cardBattle(t, "tyrell", "martell", "ser-loras-tyrell", "arianne-martell")
	r.advanceCardEffects(c, now)
	if count(r.Match.Armies["winterfell"]) != 3 || count(r.Match.Armies["white-harbor"]) != 0 || r.Orders["white-harbor"] != "" || r.Match.Control["white-harbor"] == "tyrell" {
		t.Fatal("Arianne allowed occupation")
	}
}
func TestLorasKeepsMarchAfterWinningAndRetreat(t *testing.T) {
	r, c, now := cardBattle(t, "tyrell", "baratheon", "ser-loras-tyrell", "brienne-of-tarth")
	r.advanceCardEffects(c, now)
	finishRetreat(t, r, c, now)
	if r.Orders["white-harbor"] != "march" || r.Match.ActiveHouse != "tyrell" {
		t.Fatal("Loras order unavailable", r.Phase, r.Orders)
	}
}
func TestTywinGainsPowerAndCapsAtTwenty(t *testing.T) {
	r, c, now := cardBattle(t, "lannister", "stark", "tywin-lannister", "catelyn-stark")
	r.Match.Power["lannister"] = 19
	r.advanceCardEffects(c, now)
	if r.Match.Power["lannister"] != 20 {
		t.Fatal("wrong power reward")
	}
}
func TestRooseReturnsHisOwnCardBeforePatchfaceChooses(t *testing.T) {
	r, c, now := cardBattle(t, "baratheon", "stark", "patchface", "roose-bolton")
	r.Battle.Army = []Unit{{Type: "knight", House: "baratheon", Count: 4}}
	r.Match.Discard["stark"] = []string{"eddard-stark", "catelyn-stark"}
	r.advanceCardEffects(c, now)
	finishRetreat(t, r, c, now)
	if r.Battle.Choice.Card != "patchface" || len(r.Battle.Choice.Options) != 7 || len(r.Match.Discard["stark"]) != 0 {
		t.Fatal("Roose did not return complete hand before Patchface")
	}
	applyAbility(t, r, c, now, "roose-bolton")
	if len(r.Match.Discard["stark"]) != 1 || r.Match.Discard["stark"][0] != "roose-bolton" {
		t.Fatal("Patchface did not discard chosen card")
	}
}
func TestRenlyUpgradesOneFootmanAndRespectsReserve(t *testing.T) {
	r, c, now := cardBattle(t, "baratheon", "stark", "renly-baratheon", "catelyn-stark")
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "white-harbor")
	if unitCount(r.Match.Armies["white-harbor"], "knight") != 1 || unitCount(r.Match.Armies["white-harbor"], "footman") != 2 {
		t.Fatal("Renly did not upgrade one unit")
	}
	r, c, now = cardBattle(t, "baratheon", "stark", "renly-baratheon", "catelyn-stark")
	r.Match.Armies["kingswood"] = []Unit{{Type: "knight", House: "baratheon", Count: 5}}
	r.advanceCardEffects(c, now)
	if r.Phase == "card_effect" {
		t.Fatal("Renly offered unavailable knight")
	}
}
func TestCerseiCanRemoveDistantLosingOrder(t *testing.T) {
	r, c, now := cardBattle(t, "lannister", "stark", "cersei-lannister", "catelyn-stark")
	r.Battle.Army = []Unit{{Type: "knight", House: "lannister", Count: 3}}
	r.Match.Armies["castle-black"] = []Unit{{Type: "footman", House: "stark", Count: 1}}
	r.Orders["castle-black"] = "march-star"
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "castle-black")
	if r.Orders["castle-black"] != "" {
		t.Fatal("Cersei did not remove distant order")
	}
}
func TestRobbWinnerChoosesRetreatButUnitsRemainWithLoser(t *testing.T) {
	r, c, now := cardBattle(t, "stark", "baratheon", "robb-stark", "brienne-of-tarth")
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "choose")
	if r.retreatChooser() != "stark" || r.View("stark", c).Battle.Retreats == nil || len(r.View("baratheon", c).Battle.Retreats) != 0 {
		t.Fatal("wrong retreat chooser")
	}
	target := r.retreatRegions(c)[0]
	if err := r.retreat("stark", target, c, now); err != nil {
		t.Fatal(err)
	}
	if r.Match.Armies[target][0].House != "baratheon" {
		t.Fatal("Robb captured the retreating units")
	}
}
func TestCardChoicesHaveBotAndHardTimerFallbacks(t *testing.T) {
	for _, bot := range []bool{false, true} {
		t.Run(map[bool]string{false: "hard timer", true: "bot"}[bot], func(t *testing.T) {
			r, c, now := cardBattle(t, "lannister", "greyjoy", "tyrion-lannister", "balon-greyjoy")
			r.advanceCardEffects(c, now)
			if bot {
				r.houseSeat("lannister").Bot = true
				if !r.AdvanceBot(c, now) {
					t.Fatal("bot stuck")
				}
			} else {
				r.Settings.HardTimer = true
				if !r.Expire(now.Add(6*time.Minute), c) {
					t.Fatal("timer stuck")
				}
			}
			if r.Battle != nil && r.Battle.Choice != nil && r.Battle.Choice.House == "lannister" {
				t.Fatal("decision not advanced")
			}
		})
	}
}

func TestRobbDefendingWinnerCanChooseBeyondAttackerOrigin(t *testing.T) {
	r, c, now := cardBattle(t, "baratheon", "stark", "melisandre", "robb-stark")
	r.Match.Armies["white-harbor"] = []Unit{{Type: "knight", House: "stark", Count: 3}}
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "choose")
	if !contains(r.retreatRegions(c), "moat-cailin") {
		t.Fatal("Robb in defense restricted retreat to attack origin")
	}
	if err := r.retreat("stark", "moat-cailin", c, now); err != nil {
		t.Fatal(err)
	}
	if count(r.Match.Armies["moat-cailin"]) != 3 || r.Match.Armies["moat-cailin"][0].House != "baratheon" {
		t.Fatal("wrong retreat army")
	}
}

func TestForcedDiscardOfLastCardRecoversOtherSix(t *testing.T) {
	r, c, _ := cardBattle(t, "baratheon", "stark", "patchface", "eddard-stark")
	for id := range c.Houses["stark"].Cards {
		if id != "eddard-stark" {
			r.Match.Discard["stark"] = append(r.Match.Discard["stark"], id)
		}
	}
	r.discardCard("stark", "eddard-stark", c)
	if len(r.Match.Discard["stark"]) != 1 || r.Match.Discard["stark"][0] != "eddard-stark" || len(r.availableCards("stark", c)) != 6 {
		t.Fatal("wrong last-card recovery")
	}
}
