package game

import "time"

type TidesCardID string

const (
	TideZero          TidesCardID = "zero"
	TideSword         TidesCardID = "sword"
	TideFortification TidesCardID = "fortification"
	TideTwo           TidesCardID = "two"
	TideThree         TidesCardID = "three"
	TideSkull         TidesCardID = "skull"
)

type TidesDecision string

const (
	TidesKeep     TidesDecision = "keep"
	TidesRedraw   TidesDecision = "redraw"
	TidesUseBlade TidesDecision = "strength"
)

// Six printed faces, 24 physical cards. Copies must retain their real weights;
// draw two cards without replacement from a freshly shuffled deck each combat.
type TidesCard struct {
	ID       TidesCardID `json:"id"`
	Strength int         `json:"strength"`
	Swords   int         `json:"swords"`
	Towers   int         `json:"towers"`
	Skulls   int         `json:"skulls"`
}

var tidesFaces = []struct {
	Card   TidesCard
	Copies int
}{
	{TidesCard{ID: TideZero}, 8},
	{TidesCard{ID: TideSword, Strength: 1, Swords: 1}, 4},
	{TidesCard{ID: TideFortification, Strength: 1, Towers: 1}, 4},
	{TidesCard{ID: TideTwo, Strength: 2}, 4},
	{TidesCard{ID: TideThree, Strength: 3}, 2},
	{TidesCard{ID: TideSkull, Skulls: 1}, 2},
}

// This private state belongs in Room.Battle, never in the public Match.
// Persist the remaining deck as well: a reload cannot reroll a pending choice.
type TidesState struct {
	Cards       map[string]TidesCard `json:"cards"`
	Deck        []TidesCard          `json:"deck"`
	Revealed    bool                 `json:"revealed"`
	RedrawDone  bool                 `json:"redrawDone"`
	BonusDone   bool                 `json:"bonusDone"`
	BladeBonus  string               `json:"bladeBonus,omitempty"`
	BladeRedraw string               `json:"bladeRedraw,omitempty"`
}

func tidesDeck() []TidesCard {
	deck := []TidesCard{}
	for _, face := range tidesFaces {
		for i := 0; i < face.Copies; i++ {
			deck = append(deck, face.Card)
		}
	}
	for i := len(deck) - 1; i > 0; i-- {
		j := randomIndex(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
	return deck
}

func (s *TidesState) draw(house string) {
	s.Cards[house] = s.Deck[0]
	s.Deck = append([]TidesCard{}, s.Deck[1:]...)
}

func (r *Room) bladeHolder() string {
	if r.Match == nil || len(r.Match.Tracks["fiefdoms"]) == 0 {
		return ""
	}
	return r.Match.Tracks["fiefdoms"][0]
}

func (r *Room) advanceTides(c Catalog, now time.Time) {
	b := r.Battle
	if !r.Settings.TidesOfBattle {
		if b.Done == nil {
			b.Done = map[string]bool{}
		}
		if !b.Done["blade"] {
			b.Done["blade"] = true
			holder := r.bladeHolder()
			if holder != "" && (holder == b.Attacker || holder == b.Defender) && !r.Match.BladeUsed {
				r.offerChoice(holder, "", ChoiceBlade, []string{string(TidesKeep), string(TidesUseBlade)}, false, now)
				return
			}
		}
		b.Stage = "strength"
		r.resolveCombat(c, now)
		return
	}
	if b.Tides == nil {
		b.Tides = &TidesState{Cards: map[string]TidesCard{}, Deck: tidesDeck()}
		b.Tides.draw(b.Attacker)
		b.Tides.draw(b.Defender)
	}
	s := b.Tides
	holder := r.bladeHolder()
	canUse := holder != "" && (holder == b.Attacker || holder == b.Defender) && !r.Match.BladeUsed
	if !s.RedrawDone {
		s.RedrawDone = true
		if canUse && r.offerChoice(holder, "", ChoiceTidesRedraw, []string{string(TidesKeep), string(TidesRedraw)}, false, now) {
			return
		}
	}
	s.Revealed = true
	if !s.BonusDone {
		s.BonusDone = true
		if canUse && r.offerChoice(holder, "", ChoiceTidesBlade, []string{string(TidesKeep), string(TidesUseBlade)}, false, now) {
			return
		}
	}
	b.Stage = "strength"
	r.resolveCombat(c, now)
}

func (r *Room) chooseTides(house string, kind CardChoiceKind, value TidesDecision, c Catalog, now time.Time) {
	s := r.Battle.Tides
	if value == TidesRedraw && kind == ChoiceTidesRedraw {
		s.draw(house)
		s.BladeRedraw = house
		r.Match.BladeUsed = true
		r.event("tides-blade", house, "", "", HouseNames[house]+": Валирийский меч — замена карты перевеса")
	}
	if value == TidesUseBlade && kind == ChoiceTidesBlade {
		s.BladeBonus = house
		r.Match.BladeUsed = true
		r.event("tides-blade", house, "", "", HouseNames[house]+": Валирийский меч — бонус +1")
	}
	r.advanceTides(c, now)
}

func (r *Room) combatStrengths(c Catalog) (int, int) {
	strength := r.combatBreakdown(c, true)
	return strength.Attacker.Total, strength.Defender.Total
}

func (r *Room) battleBladeBonus() string {
	if r.Battle.BladeBonus != "" {
		return r.Battle.BladeBonus
	}
	if r.Settings.TidesOfBattle && r.Battle.Tides != nil {
		return r.Battle.Tides.BladeBonus
	}
	return ""
}

func (r *Room) tidesCards() map[string]TidesCard {
	if !r.Settings.TidesOfBattle || r.Battle.Tides == nil || !r.Battle.Tides.Revealed {
		return nil
	}
	result := map[string]TidesCard{}
	for house, card := range r.Battle.Tides.Cards {
		result[house] = card
	}
	return result
}
