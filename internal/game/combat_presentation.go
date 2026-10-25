package game

// Combat reports are public, structured snapshots. No chosen card or private
// Tides draw may enter them before the appropriate reveal.
type CombatLineKind string

const (
	CombatUnit         CombatLineKind = "unit"
	CombatUnitEffect   CombatLineKind = "unit_effect"
	CombatSupportUnit  CombatLineKind = "support_unit"
	CombatSupportOrder CombatLineKind = "support_order"
	CombatMarch        CombatLineKind = "march"
	CombatDefense      CombatLineKind = "defense"
	CombatGarrison     CombatLineKind = "garrison"
	CombatHouseCard    CombatLineKind = "house_card"
	CombatCardEffect   CombatLineKind = "card_effect"
	CombatBlade        CombatLineKind = "blade"
	CombatTides        CombatLineKind = "tides"
	CombatMinimum      CombatLineKind = "minimum"
)

type CombatStrengthLine struct {
	Kind   CombatLineKind `json:"kind"`
	Region string         `json:"region,omitempty"`
	Unit   string         `json:"unit,omitempty"`
	Count  int            `json:"count,omitempty"`
	Each   int            `json:"each,omitempty"`
	Value  int            `json:"value"`
	Card   string         `json:"card,omitempty"`
}

type CombatSideStrength struct {
	House string               `json:"house"`
	Lines []CombatStrengthLine `json:"lines"`
	Total int                  `json:"total"`
}

type CombatStrength struct {
	Attacker CombatSideStrength `json:"attacker"`
	Defender CombatSideStrength `json:"defender"`
	Revealed bool               `json:"revealed"`
	Castle   bool               `json:"castle"`
}

type CombatRecord struct {
	ID        int            `json:"id"`
	Round     int            `json:"round"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Attacker  string         `json:"attacker"`
	Defender  string         `json:"defender"`
	Attackers []Unit         `json:"attackers"`
	Defenders []Unit         `json:"defenders"`
	Remaining []Unit         `json:"remaining"`
	Initial   CombatStrength `json:"initial"`
	Result    *CombatResult  `json:"result,omitempty"`
}

// CombatFinal is captured before advancing the turn/round. A winner is not
// necessarily an occupier (Arianne, Tides skulls, upgrades and failed retreats).
type CombatFinal struct {
	Armies    map[string][]Unit `json:"armies"`
	Control   map[string]string `json:"control"`
	Garrisons map[string]int    `json:"garrisons"`
}

func (r *Room) rememberCombatFinal() {
	record := r.combatRecord()
	if record == nil || r.Match.LastCombat == nil || r.Match.LastCombat.ID != record.ID {
		return
	}
	result := r.Match.LastCombat
	final := &CombatFinal{Armies: map[string][]Unit{}, Control: map[string]string{}, Garrisons: map[string]int{}}
	regions := map[string]bool{record.From: true, record.To: true}
	if result.RetreatTo != "" {
		regions[result.RetreatTo] = true
	}
	for _, side := range []CombatSideStrength{record.Initial.Attacker, record.Initial.Defender} {
		for _, line := range side.Lines {
			if line.Kind == CombatSupportUnit && line.Region != "" {
				regions[line.Region] = true
			}
		}
	}
	for region := range regions {
		final.Armies[region] = append([]Unit{}, r.Match.Armies[region]...)
		final.Control[region] = r.Match.Control[region]
		if value := r.Match.Garrisons[region]; value != 0 {
			final.Garrisons[region] = value
		}
	}
	result.Final = final
	record.Result = result
}

const combatHistoryLimit = 40

func combatLosses(before, after []Unit) []Unit {
	losses := []Unit{}
	for _, unit := range before {
		remaining := 0
		for _, alive := range after {
			if alive.Type == unit.Type && alive.House == unit.House {
				remaining += alive.Count
			}
		}
		if unit.Count > remaining {
			unit.Count -= remaining
			losses = append(losses, unit)
		}
	}
	return losses
}

func (r *Room) combatRecord() *CombatRecord {
	if r.Battle == nil || r.Match == nil || r.Battle.ID == 0 {
		return nil
	}
	for _, record := range r.Match.Combats {
		if record.ID == r.Battle.ID {
			return record
		}
	}
	return nil
}

func (r *Room) rememberCombat(c Catalog) {
	if r.combatRecord() != nil {
		return
	}
	b := r.Battle
	r.Match.CombatSequence++
	b.ID = r.Match.CombatSequence
	record := &CombatRecord{
		ID: b.ID, Round: r.Round, From: b.From, To: b.To,
		Attacker: b.Attacker, Defender: b.Defender,
		Attackers: append([]Unit{}, b.Army...), Defenders: append([]Unit{}, r.Match.Armies[b.To]...),
		Remaining: append([]Unit{}, r.Match.Armies[b.From]...), Initial: r.combatBreakdown(c, false),
	}
	r.Match.Combats = append(r.Match.Combats, record)
	if len(r.Match.Combats) > combatHistoryLimit {
		r.Match.Combats = append([]*CombatRecord{}, r.Match.Combats[len(r.Match.Combats)-combatHistoryLimit:]...)
	}
}

// Adjustments are ordered exactly like card resolution, including Salladhor's
// zero followed by a later unit bonus. Both the engine and report consume them.
func (r *Room) combatUnitAdjustments(unit Unit, base int, revealed bool, c Catalog) []CombatStrengthLine {
	lines := []CombatStrengthLine{}
	if !revealed {
		return lines
	}
	value := base
	for _, house := range r.battleHouses() {
		id := r.Battle.Cards[house]
		delta := 0
		switch id {
		case "ser-kevan-lannister":
			if house == r.Battle.Attacker && unit.House == house && unit.Type == "footman" {
				delta = 1
			}
		case "victarion-greyjoy":
			if house == r.Battle.Attacker && unit.House == house && unit.Type == "ship" {
				delta = 1
			}
		case "salladhor-saan":
			if unit.Type == "ship" && unit.House != house && len(r.supportingRegions(house, c)) > 0 {
				delta = -value
			}
		}
		if delta != 0 {
			lines = append(lines, CombatStrengthLine{Kind: CombatUnitEffect, Unit: unit.Type, Count: unit.Count, Each: delta, Value: delta * unit.Count, Card: id})
			value += delta
		}
	}
	return lines
}

func (r *Room) combatBreakdown(c Catalog, revealed bool) CombatStrength {
	b := r.Battle
	castle := r.definition(b.To, c).Castle > 0
	cards := map[string]Card{}
	if revealed {
		cards = r.combatCards(c)
	}
	makeSide := func(house string, attacking bool, army []Unit) CombatSideStrength {
		side := CombatSideStrength{House: house, Lines: []CombatStrengthLine{}}
		add := func(line CombatStrengthLine) {
			side.Lines = append(side.Lines, line)
			side.Total += line.Value
		}
		addArmy := func(units []Unit, region string, kind CombatLineKind) {
			for _, unit := range units {
				base := armyStrength([]Unit{{Type: unit.Type, Count: 1}}, attacking && castle)
				add(CombatStrengthLine{Kind: kind, Region: region, Unit: unit.Type, Count: unit.Count, Each: base, Value: base * unit.Count})
				for _, line := range r.combatUnitAdjustments(unit, base, revealed, c) {
					line.Region = region
					add(line)
				}
			}
		}
		region := b.To
		if attacking {
			region = b.From
		}
		addArmy(army, region, CombatUnit)
		if attacking {
			add(CombatStrengthLine{Kind: CombatMarch, Region: b.From, Value: b.March})
		} else {
			if order, _ := FindOrder(r.Orders[b.To]); order.Kind == "defend" {
				add(CombatStrengthLine{Kind: CombatDefense, Region: b.To, Value: order.Modifier})
				if cards[house].Ability == "catelyn-stark" {
					add(CombatStrengthLine{Kind: CombatCardEffect, Card: "catelyn-stark", Value: order.Modifier})
				}
			}
			if garrison := r.Match.Garrisons[b.To]; garrison != 0 {
				add(CombatStrengthLine{Kind: CombatGarrison, Region: b.To, Value: garrison})
			}
		}
		for _, id := range r.supportingRegions(house, c) {
			addArmy(r.Match.Armies[id], id, CombatSupportUnit)
			order, _ := FindOrder(r.Orders[id])
			add(CombatStrengthLine{Kind: CombatSupportOrder, Region: id, Value: order.Modifier})
		}
		if revealed {
			id := b.Cards[house]
			if id != "" {
				printed := c.Houses[house].Cards[id].Strength
				add(CombatStrengthLine{Kind: CombatHouseCard, Card: id, Value: printed})
				if change := cards[house].Strength - printed; change != 0 {
					add(CombatStrengthLine{Kind: CombatCardEffect, Card: id, Value: change})
				}
			}
			if r.Settings.TidesOfBattle && b.Tides != nil && b.Tides.Revealed {
				add(CombatStrengthLine{Kind: CombatTides, Value: b.Tides.Cards[house].Strength})
			}
			if r.battleBladeBonus() == house {
				add(CombatStrengthLine{Kind: CombatBlade, Value: 1})
			}
		}
		if side.Total < 0 {
			add(CombatStrengthLine{Kind: CombatMinimum, Value: -side.Total})
		}
		return side
	}
	return CombatStrength{Attacker: makeSide(b.Attacker, true, b.Army), Defender: makeSide(b.Defender, false, r.Match.Armies[b.To]), Revealed: revealed, Castle: castle}
}
