package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCombatFinalRecordsActualControllerAndSurvivesLaterMoves(t *testing.T) {
	r, c, now, b := preparedTides(t, TideZero, TideSkull)
	b.To = "moat-cailin"
	b.Army = []Unit{{Type: "footman", House: "stark", Count: 1}}
	r.Match.Armies = map[string][]Unit{b.To: {{Type: "footman", House: "greyjoy", Count: 1}}}
	r.Match.Control = map[string]string{b.To: "greyjoy"}
	r.advanceTides(c, now)
	finishRetreat(t, r, c, now)
	final := r.Match.Combats[0].Result.Final
	if final == nil || r.Match.LastCombat.Winner != "stark" || len(final.Armies[b.To]) != 0 || final.Control[b.To] == "stark" {
		t.Fatalf("winner incorrectly occupies target: %+v", final)
	}
	encoded, _ := json.Marshal(final)
	r.Match.Armies[b.To] = []Unit{{Type: "knight", House: "lannister", Count: 3}}
	r.Match.Control[b.To] = "lannister"
	clone := r.Clone()
	actual, _ := json.Marshal(clone.Match.Combats[0].Result.Final)
	if string(encoded) != string(actual) {
		t.Fatal("final changed after live moves / JSON restart")
	}
}

func TestCombatFinalRecordsRenlyUpgradeNotAnInventedCasualty(t *testing.T) {
	r, c, now := cardBattle(t, "baratheon", "stark", "renly-baratheon", "catelyn-stark")
	r.advanceCardEffects(c, now)
	applyAbility(t, r, c, now, "white-harbor")
	finishRetreat(t, r, c, now)
	final := r.Match.Combats[0].Result.Final
	if final == nil || unitCount(final.Armies["white-harbor"], "knight") != 1 || unitCount(final.Armies["white-harbor"], "footman") != 2 {
		t.Fatalf("upgrade missing from final: %+v", final)
	}
	if count(r.Match.Combats[0].Result.Losses["baratheon"]) != 0 {
		t.Fatal("upgrade counted as a casualty")
	}
}

func TestCombatFinalCountsSurvivorsDestroyedWithoutLegalRetreat(t *testing.T) {
	r, c, now := cardBattle(t, "stark", "lannister", "roose-bolton", "ser-kevan-lannister")
	for _, region := range c.Boards["westeros"].Regions {
		if region.ID != "white-harbor" && region.ID != "winterfell" {
			r.Match.BlockedRegions = append(r.Match.BlockedRegions, region.ID)
		}
	}
	r.advanceCardEffects(c, now)
	result := r.Match.Combats[0].Result
	if result.Final == nil || count(result.Losses["lannister"]) != 2 || result.RetreatHouse != "" {
		t.Fatalf("trapped troops reported as survivors: %+v", result)
	}
	if !reflect.DeepEqual(result.Final.Armies["white-harbor"], []Unit{{Type: "footman", House: "stark", Count: 3}}) {
		t.Fatal("wrong final position")
	}
}

func TestCombatInitialSnapshotAndNoSecretStrengthLeak(t *testing.T) {
	r, c, now := fixture(t)
	r.Match.Armies["winterfell"] = []Unit{{Type: "footman", House: "stark", Count: 3}, {Type: "knight", House: "stark", Count: 1}}
	r.Match.Armies["white-harbor"] = []Unit{{Type: "footman", House: "lannister", Count: 2}}
	r.Match.Control["white-harbor"] = "lannister"
	if err := r.march("stark", Command{Region: "winterfell", Target: "white-harbor", Units: map[string]int{"footman": 1, "knight": 1}}, c, now); err != nil {
		t.Fatal(err)
	}
	record := r.combatRecord()
	if record == nil || record.ID != 1 || count(record.Attackers) != 2 || count(record.Remaining) != 2 || count(record.Defenders) != 2 {
		t.Fatalf("wrong partial march: %+v", record)
	}
	before, _ := json.Marshal(r.battleView("lannister", c).Strength)
	r.Battle.Cards["stark"] = "eddard-stark"
	r.Battle.Tides = &TidesState{Cards: map[string]TidesCard{"stark": {ID: TideThree, Strength: 3}}}
	r.Settings.TidesOfBattle = true
	after, _ := json.Marshal(r.battleView("lannister", c).Strength)
	if string(before) != string(after) {
		t.Fatalf("hidden choice leaked through strength: %s -> %s", before, after)
	}
	public, _ := json.Marshal(r.Match.Combats)
	if strings.Contains(string(public), "eddard-stark") || strings.Contains(string(public), "three") {
		t.Fatal("private draw/card leaked into history")
	}
	r.Match.Armies["winterfell"] = nil
	clone := r.Clone()
	if count(clone.Match.Combats[0].Remaining) != 2 || clone.Match.Combats[0].Initial.Attacker.Total != 3 {
		t.Fatal("initial report reconstructed from changed board")
	}
}

func TestCombatBreakdownExactContributions(t *testing.T) {
	r, c, _ := cardBattle(t, "lannister", "stark", "ser-kevan-lannister", "catelyn-stark")
	r.Battle.Army = []Unit{{Type: "footman", House: "lannister", Count: 2}, {Type: "knight", House: "lannister", Count: 1}, {Type: "siege-engine", House: "lannister", Count: 1}}
	r.Match.Armies[r.Battle.To] = []Unit{{Type: "footman", House: "stark", Count: 1}, {Type: "siege-engine", House: "stark", Count: 1}}
	r.Match.Garrisons[r.Battle.To] = 2
	r.Battle.March = -1
	r.Orders[r.Battle.To] = "defend-star"
	r.Battle.BladeBonus = "stark"
	r.Settings.TidesOfBattle = true
	r.Battle.Tides = &TidesState{Revealed: true, Cards: map[string]TidesCard{"lannister": {Strength: 2}, "stark": {Strength: 3}}}
	value := r.combatBreakdown(c, true)
	// 2 infantry + 2 knight + 4 siege + 2 Kevan - 1 march + 1 card + 2 Tides.
	if value.Attacker.Total != 12 {
		t.Fatalf("attack %+v", value.Attacker)
	}
	// 1 infantry + 0 defending siege + 2 defense + 2 Catelyn + 2 garrison + 0 card + 3 Tides + 1 Blade.
	if value.Defender.Total != 11 {
		t.Fatalf("defense %+v", value.Defender)
	}
	for _, side := range []CombatSideStrength{value.Attacker, value.Defender} {
		sum := 0
		for _, line := range side.Lines {
			sum += line.Value
		}
		if sum != side.Total {
			t.Fatal("line sum differs from total")
		}
	}
	initial := r.combatBreakdown(c, false)
	if initial.Attacker.Total != 7 || initial.Defender.Total != 5 {
		t.Fatalf("hidden effects applied: %+v", initial)
	}
	r.Battle.To = "karhold"
	plain := r.combatBreakdown(c, false)
	if plain.Attacker.Total != 3 {
		t.Fatalf("siege counted outside castle: %+v", plain.Attacker)
	}
}

func TestCombatHistorySurvivesRoundTransitionAndIsBounded(t *testing.T) {
	r, c, now := cardBattle(t, "stark", "lannister", "eddard-stark", "tyrion-lannister")
	r.Battle.Cards["lannister"] = "ser-kevan-lannister" // No discretionary effect or surviving defender.
	r.Match.Armies[r.Battle.To] = []Unit{{Type: "footman", House: "lannister", Count: 1}}
	r.advanceCardEffects(c, now)
	if r.Match.LastCombat == nil || len(r.Match.Combats) != 1 || r.Match.Combats[0].Result == nil {
		t.Fatal("combat result lost on round transition")
	}
	if r.Round != 2 {
		t.Fatalf("expected completed round, got %d %s", r.Round, r.Phase)
	}
	clone := r.Clone()
	before, _ := json.Marshal(r.Match.Combats)
	after, _ := json.Marshal(clone.Match.Combats)
	if string(before) != string(after) {
		t.Fatal("history did not survive JSON restart")
	}
	for i := 1; i < 45; i++ {
		r.Battle = &Battle{From: "winterfell", To: "white-harbor", Attacker: "stark", Defender: "lannister", Army: []Unit{}, Cards: map[string]string{}}
		r.rememberCombat(c)
	}
	if len(r.Match.Combats) != combatHistoryLimit || r.Match.Combats[0].ID != 6 || r.Match.CombatSequence != 45 {
		t.Fatal("history cap or monotonic identity failed")
	}
}
