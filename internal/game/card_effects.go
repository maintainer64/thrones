package game

import (
	"errors"
	"sort"
	"time"
)

// CardChoice is a persisted decision, not a callback. Options are calculated by
// the server; only the deciding player's view receives the list.
type CardChoiceKind string

const (
	ChoiceCard        CardChoiceKind = "card"
	ChoiceReplace     CardChoiceKind = "replace"
	ChoiceRegion      CardChoiceKind = "region"
	ChoiceTrack       CardChoiceKind = "track"
	ChoiceConfirm     CardChoiceKind = "confirm"
	ChoiceTidesRedraw CardChoiceKind = "tides_redraw"
	ChoiceTidesBlade  CardChoiceKind = "tides_blade"
	ChoiceBlade       CardChoiceKind = "blade"
)

type CardChoice struct {
	House    string         `json:"house"`
	Card     string         `json:"card"`
	Kind     CardChoiceKind `json:"kind"`
	Options  []string       `json:"options"`
	Optional bool           `json:"optional"`
}

func implementedCardEffect(id string) bool {
	switch id {
	case "stannis-baratheon", "renly-baratheon", "ser-davos-seaworth", "salladhor-saan", "patchface",
		"tywin-lannister", "ser-kevan-lannister", "tyrion-lannister", "cersei-lannister",
		"robb-stark", "roose-bolton", "the-blackfish", "catelyn-stark",
		"arianne-martell", "nymeria-sand", "doran-martell",
		"victarion-greyjoy", "balon-greyjoy", "theon-greyjoy", "asha-greyjoy", "aeron-damphair",
		"mace-tyrell", "ser-loras-tyrell", "queen-of-thorns":
		return true
	}
	return false
}

func (r *Room) enemy(house string) string {
	if house == r.Battle.Attacker {
		return r.Battle.Defender
	}
	return r.Battle.Attacker
}
func (r *Room) battleHouses() []string {
	out := []string{}
	for _, h := range r.Match.Tracks["ironThrone"] {
		if h == r.Battle.Attacker || h == r.Battle.Defender {
			out = append(out, h)
		}
	}
	return out
}
func (r *Room) effect(house, id, message string, c Catalog) {
	text := c.Houses[house].Cards[id].Name + ": " + message
	r.event("card-effect", house, "", "", text)
	r.Battle.Effects = append(r.Battle.Effects, text)
	if r.Battle.Loser != "" && r.Match.LastCombat != nil {
		r.Match.LastCombat.Effects = append([]string{}, r.Battle.Effects...)
		if record := r.combatRecord(); record != nil {
			record.Result = r.Match.LastCombat
		}
	}
}
func (r *Room) offerChoice(house, card string, kind CardChoiceKind, options []string, optional bool, now time.Time) bool {
	if len(options) == 0 {
		return false
	}
	r.Battle.Choice = &CardChoice{house, card, kind, append([]string{}, options...), optional}
	r.Phase, r.Match.ActiveHouse = "card_effect", house
	r.startClock(house, now)
	return true
}
func (r *Room) otherCards(house string, c Catalog) []string {
	ids := []string{}
	for _, id := range r.availableCards(house, c) {
		if id != r.Battle.Cards[house] {
			ids = append(ids, id)
		}
	}
	return ids
}

// advanceCardEffects follows cancel -> immediate -> strength -> winner effects
// -> retreat -> cleanup -> after-combat. A resolved key is written before
// yielding to a human, so reloads and automation ticks cannot repeat an effect.
func (r *Room) advanceCardEffects(c Catalog, now time.Time) {
	b := r.Battle
	if b.Done == nil {
		b.Done = map[string]bool{}
	}
	if b.Choice != nil {
		return
	}
	if b.Stage == "tides" {
		r.advanceTides(c, now)
		return
	}
	if b.Stage == "before" {
		for _, h := range r.battleHouses() {
			if b.Cards[h] == "tyrion-lannister" && !b.Done["tyrion"] {
				b.Done["tyrion"] = true
				if b.Cards[r.enemy(h)] != "" && r.offerChoice(h, "tyrion-lannister", "confirm", []string{"cancel"}, true, now) {
					return
				}
			}
		}
		for _, h := range r.battleHouses() {
			id := b.Cards[h]
			key := "before:" + h + ":" + id
			if b.Done[key] {
				continue
			}
			b.Done[key] = true
			switch id {
			case "aeron-damphair":
				if r.Match.Power[h] >= 2 && r.offerChoice(h, id, "card", r.otherCards(h, c), true, now) {
					return
				}
			case "doran-martell":
				if r.offerChoice(h, id, "track", []string{"ironThrone", "fiefdoms", "kingsCourt"}, false, now) {
					return
				}
			case "queen-of-thorns":
				ids := []string{}
				for _, region := range r.definition(b.To, c).Neighbors {
					if r.Orders[region] != "" && r.owner(region) == r.enemy(h) {
						ids = append(ids, region)
					}
				}
				sort.Strings(ids)
				if r.offerChoice(h, id, "region", ids, false, now) {
					return
				}
			case "mace-tyrell":
				enemy := r.enemy(h)
				if b.Cards[enemy] == "the-blackfish" {
					r.effect(enemy, "the-blackfish", "потеря пехотинца от Мейса предотвращена", c)
					continue
				}
				army := &b.Army
				if enemy == b.Defender {
					army = nil
				}
				units := b.Army
				if army == nil {
					units = r.Match.Armies[b.To]
				}
				for i, u := range units {
					if u.Type == "footman" && u.Count > 0 {
						copy := append([]Unit{}, units...)
						copy[i].Count--
						if copy[i].Count == 0 {
							copy = append(copy[:i], copy[i+1:]...)
						}
						if army == nil {
							r.Match.Armies[b.To] = copy
						} else {
							b.Army = copy
						}
						r.effect(h, id, "уничтожен один пехотинец противника до подсчёта силы", c)
						break
					}
				}
			}
		}
		b.Stage = "tides"
		r.advanceTides(c, now)
		return
	}
	if b.Stage == "post" {
		winner := r.Match.LastCombat.Winner
		for _, h := range r.battleHouses() {
			id := b.Cards[h]
			key := "post:" + h
			if b.Done[key] {
				continue
			}
			b.Done[key] = true
			if h != winner {
				continue
			}
			switch id {
			case "renly-baratheon":
				if r.offerChoice(h, id, "region", r.upgradeRegions(h, c), true, now) {
					return
				}
			case "cersei-lannister":
				ids := []string{}
				for region := range r.Orders {
					if r.owner(region) == r.enemy(h) {
						ids = append(ids, region)
					}
				}
				sort.Strings(ids)
				if r.offerChoice(h, id, "region", ids, true, now) {
					return
				}
			case "robb-stark":
				previous := b.RetreatChooser
				b.RetreatChooser = h
				options := r.retreatRegions(c)
				b.RetreatChooser = previous
				if count(b.Survivors) > 0 && len(options) > 0 && r.offerChoice(h, id, "confirm", []string{"choose"}, true, now) {
					return
				}
			}
		}
		b.Stage = "retreat"
		if count(b.Survivors) > 0 && len(r.retreatRegions(c)) > 0 {
			r.Phase, r.Match.ActiveHouse = "retreat", r.retreatChooser()
			r.startClock(r.Match.ActiveHouse, now)
			return
		}
		r.completeCombat(c, now)
		return
	}
	if b.Stage == "after" {
		for _, h := range r.battleHouses() {
			id := b.Cards[h]
			if id != "patchface" || b.Done["patchface"] {
				continue
			}
			b.Done["patchface"] = true
			if r.offerChoice(h, id, "card", r.availableCards(r.enemy(h), c), true, now) {
				return
			}
		}
		r.finishCombat(c, now)
	}
}

func (r *Room) chooseAbility(house, value string, c Catalog, now time.Time) error {
	if r.Phase != "card_effect" || r.Battle == nil || r.Battle.Choice == nil || r.Battle.Choice.House != house {
		return errors.New("Сейчас не ваш выбор способности")
	}
	b := r.Battle
	choice := *b.Choice
	if value == "" && !choice.Optional || value != "" && !contains(choice.Options, value) {
		return errors.New("Недопустимый выбор способности")
	}
	if choice.Card == "aeron-damphair" && value != "" && r.Match.Power[house] < 2 {
		return errors.New("Недостаточно доступной власти")
	}
	if choice.Kind == ChoiceBlade && (r.bladeHolder() != house || r.Match.BladeUsed) {
		return errors.New("Валирийский меч сейчас недоступен")
	}
	b.Choice = nil
	if choice.Kind == ChoiceBlade {
		if value == string(TidesUseBlade) {
			b.BladeBonus = house
			r.Match.BladeUsed = true
			r.event("tides-blade", house, "", "", HouseNames[house]+": Валирийский меч — бонус +1")
		}
		r.advanceTides(c, now)
		return nil
	}
	if choice.Kind == ChoiceTidesRedraw || choice.Kind == ChoiceTidesBlade {
		r.chooseTides(house, choice.Kind, TidesDecision(value), c, now)
		return nil
	}
	if value == "" {
		r.effect(house, choice.Card, "игрок отказался от необязательного эффекта", c)
		r.advanceCardEffects(c, now)
		return nil
	}
	enemy := r.enemy(house)
	switch {
	case choice.Kind == "replace":
		b.Cards[house] = value
		r.effect(house, value, "выбрана взамен отменённой карты", c)
	case choice.Card == "tyrion-lannister":
		options := r.otherCards(enemy, c)
		b.Cards[enemy] = ""
		r.effect(house, choice.Card, "карта противника отменена и возвращена в руку", c)
		if r.offerChoice(enemy, choice.Card, "replace", options, false, now) {
			return nil
		}
	case choice.Card == "aeron-damphair":
		r.Match.Power[house] -= 2
		r.discardCard(house, "aeron-damphair", c)
		b.Cards[house] = value
		r.effect(house, choice.Card, "потрачено 2 власти; новая карта — "+c.Houses[house].Cards[value].Name, c)
	case choice.Card == "queen-of-thorns" || choice.Card == "cersei-lannister":
		delete(r.Orders, value)
		r.effect(house, choice.Card, "снят приказ в области "+r.definition(value, c).Name, c)
	case choice.Card == "doran-martell":
		track := []string{}
		for _, h := range r.Match.Tracks[value] {
			if h != enemy {
				track = append(track, h)
			}
		}
		r.Match.Tracks[value] = append(track, enemy)
		r.effect(house, choice.Card, "противник перемещён в конец трека "+value, c)
	case choice.Card == "robb-stark":
		b.RetreatChooser = house
		r.effect(house, choice.Card, "отступление противника выбирает победитель", c)
	case choice.Card == "renly-baratheon":
		units := append([]Unit{}, r.Match.Armies[value]...)
		for i := range units {
			if units[i].House == house && units[i].Type == "footman" && units[i].Count > 0 {
				units[i].Count--
				break
			}
		}
		clean := []Unit{}
		for _, u := range units {
			if u.Count > 0 {
				clean = append(clean, u)
			}
		}
		r.Match.Armies[value] = merge(clean, []Unit{{Type: "knight", House: house, Count: 1}})
		r.effect(house, choice.Card, "пехотинец заменён рыцарем: "+r.definition(value, c).Name, c)
	case choice.Card == "patchface":
		r.discardCard(enemy, value, c)
		r.effect(house, choice.Card, "из руки противника сброшена карта "+c.Houses[enemy].Cards[value].Name, c)
	}
	r.advanceCardEffects(c, now)
	return nil
}

func (r *Room) supportingRegions(house string, c Catalog) []string {
	ids := []string{}
	to := r.definition(r.Battle.To, c)
	for _, id := range to.Neighbors {
		o, _ := FindOrder(r.Orders[id])
		from := r.definition(id, c)
		if o.Kind == "support" && r.owner(id) == house && count(r.Match.Armies[id]) > 0 && !(from.Type == "land" && to.Type != "land") {
			ids = append(ids, id)
		}
	}
	return ids
}
func (r *Room) upgradeRegions(house string, c Catalog) []string {
	knights := 0
	for _, army := range r.Match.Armies {
		for _, u := range army {
			if u.House == house && u.Type == "knight" {
				knights += u.Count
			}
		}
	}
	if knights >= c.Houses[house].Limits["knight"] {
		return nil
	}
	ids := append([]string{r.Battle.To}, r.supportingRegions(house, c)...)
	if house == r.Battle.Attacker && r.Battle.Cards[r.Battle.Defender] == "arianne-martell" {
		ids = append(ids, r.Battle.From)
	}
	out := []string{}
	for _, id := range ids {
		for _, u := range r.Match.Armies[id] {
			if u.House == house && u.Type == "footman" && u.Count > 0 {
				out = append(out, id)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
func (r *Room) discardCard(house, id string, c Catalog) {
	if id == "" || contains(r.Match.Discard[house], id) {
		return
	}
	r.Match.Discard[house] = append(r.Match.Discard[house], id)
	if len(r.Match.Discard[house]) >= len(c.Houses[house].Cards) {
		r.Match.Discard[house] = []string{id}
	}
}
func (r *Room) retreatChooser() string {
	if r.Battle.RetreatChooser != "" {
		return r.Battle.RetreatChooser
	}
	return r.Battle.Loser
}

func (r *Room) combatCards(c Catalog) map[string]Card {
	b := r.Battle
	cards := map[string]Card{}
	for _, h := range r.battleHouses() {
		card := c.Houses[h].Cards[b.Cards[h]]
		switch card.Ability {
		case "stannis-baratheon":
			for _, ahead := range r.Match.Tracks["ironThrone"] {
				if ahead == r.enemy(h) {
					card.Strength++
					break
				}
				if ahead == h {
					break
				}
			}
		case "ser-davos-seaworth":
			if contains(r.Match.Discard[h], "stannis-baratheon") {
				card.Strength++
				card.Swords++
			}
		case "theon-greyjoy":
			if h == b.Defender && r.definition(b.To, c).Castle > 0 {
				card.Strength++
				card.Swords++
			}
		case "nymeria-sand":
			if h == b.Attacker {
				card.Swords++
			} else {
				card.Towers++
			}
		case "asha-greyjoy":
			if len(r.supportingRegions(h, c)) == 0 {
				card.Swords += 2
				card.Towers++
			}
		}
		cards[h] = card
	}
	for _, h := range r.battleHouses() {
		if b.Cards[r.enemy(h)] == "balon-greyjoy" {
			card := cards[h]
			card.Strength -= c.Houses[h].Cards[b.Cards[h]].Strength
			cards[h] = card
		}
	}
	return cards
}
func (r *Room) combatArmyStrength(army []Unit, side string, attacking bool, c Catalog) int {
	total := 0
	b := r.Battle
	for _, u := range army {
		value := armyStrength([]Unit{{Type: u.Type, Count: 1}}, attacking && r.definition(b.To, c).Castle > 0)
		total += value * u.Count
		for _, line := range r.combatUnitAdjustments(u, value, true, c) {
			total += line.Value
		}
	}
	return total
}
func (r *Room) combatSupport(house string, attacking bool, c Catalog) int {
	total := 0
	for _, id := range r.supportingRegions(house, c) {
		order, _ := FindOrder(r.Orders[id])
		total += r.combatArmyStrength(r.Match.Armies[id], house, attacking, c) + order.Modifier
	}
	return total
}
