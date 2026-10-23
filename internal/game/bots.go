package game

import (
	"errors"
	"sort"
	"time"
)

func (r *Room) houseSeat(house string) *Seat {
	for i := range r.Seats {
		if r.Seats[i].House == house {
			return &r.Seats[i]
		}
	}
	return nil
}
func (r *Room) fillOrders(seat *Seat, c Catalog) {
	for _, id := range r.regionIDs(seat.House) {
		if r.Orders[id] != "" {
			continue
		}
		for _, o := range Orders {
			if r.placeOrder(seat, id, o.ID, c) == nil {
				break
			}
		}
	}
}
func (r *Room) safeAction(house string, c Catalog, now time.Time) error {
	if r.Phase == "card_effect" {
		choice := r.Battle.Choice
		value := ""
		if !choice.Optional && len(choice.Options) > 0 {
			value = choice.Options[0]
		}
		return r.chooseAbility(house, value, c, now)
	}
	if r.Phase == "combat" {
		cards := r.availableCards(house, c)
		if len(cards) == 0 {
			return errors.New("Нет карты для автоматического выбора")
		}
		sort.SliceStable(cards, func(i, j int) bool {
			return c.Houses[house].Cards[cards[i]].Strength < c.Houses[house].Cards[cards[j]].Strength
		})
		return r.chooseCard(house, cards[0], c, now)
	}
	if r.Phase == "retreat" {
		ids := r.retreatRegions(c)
		if len(ids) == 0 {
			r.completeCombat(c, now)
			return nil
		}
		return r.retreat(house, ids[0], c, now)
	}
	return r.action(house, Command{Type: "pass"}, c, now)
}
func (r *Room) runBots(c Catalog, now time.Time) {
	// Bounded execution prevents a malformed state from monopolising an HTTP request.
	for step := 0; step < 300; step++ {
		if !r.AdvanceBot(c, now) {
			return
		}
	}
}

// AdvanceBot executes exactly one real, persisted decision. The server calls it
// on its automation tick so clients see individual turns instead of a teleporting batch.
func (r *Room) AdvanceBot(c Catalog, now time.Time) bool {
	if r.Phase == PhaseRaven {
		return r.advanceRaven(c, now, false)
	}
	if r.Phase == PhaseWesteros {
		if r.pendingWildlingContext() && (r.Wildlings == nil || r.Wildlings.Attack == nil) {
			return r.beginWildlingAttack(now) == nil
		}
		if r.advanceWesterosChoice(c, now, false) {
			return true
		}
		return r.advanceWesteros(c, now)
	}
	if r.Phase == PhaseWildlingBidding || r.Phase == PhaseWildlingTies || r.Phase == PhaseWildlingEffects {
		return r.advanceWildlingBot(c, now)
	}
	if r.Phase == PhaseSupply || r.Phase == PhaseMustering || r.Phase == PhasePower {
		return r.advanceProvisionBot(c, now, false)
	}
	if r.Phase == PhaseInfluenceBidding || r.Phase == PhaseInfluenceTies {
		return r.advanceInfluenceBot(now)
	}
	if r.Match == nil || r.Phase == "planning" || r.Phase == "orders_revealed" || r.Phase == "finished" {
		return false
	}
	house := r.Match.ActiveHouse
	if r.Phase == "combat" {
		house = ""
		for _, h := range []string{r.Battle.Attacker, r.Battle.Defender} {
			s := r.houseSeat(h)
			if s != nil && s.Bot && r.Battle.Cards[h] == "" {
				house = h
				break
			}
		}
	}
	seat := r.houseSeat(house)
	if seat == nil || !seat.Bot {
		return false
	}
	if !r.botAction(seat, c, now) {
		if r.safeAction(house, c, now) != nil {
			return false
		}
	}
	return true
}
func (r *Room) botAction(s *Seat, c Catalog, now time.Time) bool {
	house := s.House
	if r.Phase == "card_effect" {
		choice := r.Battle.Choice
		if choice == nil || choice.House != house {
			return false
		}
		value := ""
		if len(choice.Options) > 0 {
			value = choice.Options[0]
		}
		if choice.Kind == "card" || choice.Kind == "replace" {
			owner := house
			if choice.Card == "patchface" {
				owner = r.enemy(house)
			}
			best := -1
			for _, id := range choice.Options {
				if strength := c.Houses[owner].Cards[id].Strength; strength > best {
					best = strength
					value = id
				}
			}
		}
		if choice.Kind == ChoiceTidesRedraw {
			value = string(TidesKeep)
			if r.Battle.Tides.Cards[house].ID == TideZero && s.Difficulty != "easy" {
				value = string(TidesRedraw)
			}
		}
		if choice.Kind == ChoiceTidesBlade || choice.Kind == ChoiceBlade {
			value = string(TidesKeep)
			a, d := r.combatStrengths(c)
			own, other := a, d
			if house == r.Battle.Defender {
				own, other = d, a
			}
			if own <= other && own+1 >= other {
				value = string(TidesUseBlade)
			}
		}
		return r.chooseAbility(house, value, c, now) == nil
	}
	if r.Phase == "combat" {
		cards := r.availableCards(house, c)
		if len(cards) == 0 {
			return false
		}
		// Selection uses our hand and public army strengths, never the opponent's hidden card.
		sort.Slice(cards, func(i, j int) bool { return cards[i] < cards[j] })
		chosen := cards[randomIndex(len(cards))]
		if s.Difficulty != "easy" {
			sort.SliceStable(cards, func(i, j int) bool {
				return c.Houses[house].Cards[cards[i]].Strength > c.Houses[house].Cards[cards[j]].Strength
			})
			chosen = cards[0]
		}
		return r.chooseCard(house, chosen, c, now) == nil
	}
	if r.Phase == "retreat" {
		ids := r.retreatRegions(c)
		sort.SliceStable(ids, func(i, j int) bool { return r.definition(ids[i], c).Castle > r.definition(ids[j], c).Castle })
		if len(ids) > 0 {
			return r.retreat(house, ids[0], c, now) == nil
		}
		return false
	}
	for _, from := range r.actionRegions(house) {
		targets := []string{}
		for _, d := range c.Boards["westeros"].Regions {
			targets = append(targets, d.ID)
		}
		if s.Difficulty == "easy" {
			for i := range targets {
				j := randomIndex(len(targets))
				targets[i], targets[j] = targets[j], targets[i]
			}
		} else {
			sort.SliceStable(targets, func(i, j int) bool {
				return r.destinationScore(targets[i], house, c, s.Difficulty) > r.destinationScore(targets[j], house, c, s.Difficulty)
			})
		}
		for _, to := range targets {
			cmd := Command{Type: "raid", Region: from, Target: to}
			if r.Phase == "marches" {
				if r.owner(to) == house {
					continue
				}
				cmd.Type = "march"
				cmd.Units = map[string]int{}
				for _, u := range r.Match.Armies[from] {
					cmd.Units[u.Type] += u.Count
				}
				if s.Difficulty == "hard" && count(r.Match.Armies[from]) > 1 && r.definition(from, c).Castle > 0 {
					for _, u := range r.Match.Armies[from] {
						if u.Type == "footman" && u.Count > 0 {
							cmd.Units[u.Type]--
							break
						}
					}
				}
			}
			// Validate speculative moves on an isolated copy: rejected candidates cannot mutate the real match.
			copy := r.Clone()
			if copy.action(house, cmd, c, now) == nil {
				*r = *copy
				return true
			}
		}
	}
	return false
}
func (r *Room) destinationScore(id, house string, c Catalog, difficulty string) int {
	d := r.definition(id, c)
	score := d.Castle*20 + d.Crowns*3 + d.Supply*2
	if r.owner(id) == house {
		score -= 30
	}
	if r.owner(id) == "" {
		score += 10
	}
	if difficulty == "hard" && r.owner(id) != "" && r.owner(id) != house {
		score -= armyStrength(r.Match.Armies[id], false)*3 + r.support(r.owner(id), id, c)
	}
	return score
}
