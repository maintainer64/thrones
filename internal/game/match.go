package game

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Match contains public state only. Secret simultaneous choices are held in Room.Battle.
type Match struct {
	Armies         map[string][]Unit   `json:"armies"`
	Control        map[string]string   `json:"control"`
	ControlTokens  map[string]string   `json:"controlTokens,omitempty"`
	Garrisons      map[string]int      `json:"garrisons"`
	Power          map[string]int      `json:"power"`
	Supply         map[string]int      `json:"supply"`
	Tracks         map[string][]string `json:"tracks"`
	Discard        map[string][]string `json:"discard"`
	ActiveHouse    string              `json:"activeHouse"`
	Winner         string              `json:"winner,omitempty"`
	LastCombat     *CombatResult       `json:"lastCombat,omitempty"`
	ResumePosition *int                `json:"resumePosition,omitempty"`
	BladeUsed      bool                `json:"bladeUsed"`
	RavenUsed      bool                `json:"ravenUsed"`
	BlockedRegions []string            `json:"blockedRegions,omitempty"`
	LastInfluence  []InfluenceResult   `json:"lastInfluence,omitempty"`
	CombatSequence int                 `json:"combatSequence,omitempty"`
	Combats        []*CombatRecord     `json:"combats,omitempty"`
}
type Event struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	House string `json:"house"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	Text  string `json:"text"`
}
type Battle struct {
	ID             int               `json:"id,omitempty"`
	From           string            `json:"from"`
	To             string            `json:"to"`
	Attacker       string            `json:"attacker"`
	Defender       string            `json:"defender"`
	Army           []Unit            `json:"army"`
	March          int               `json:"march"`
	Cards          map[string]string `json:"cards"`
	Loser          string            `json:"loser,omitempty"`
	Survivors      []Unit            `json:"survivors,omitempty"`
	Revealed       bool              `json:"revealed,omitempty"`
	Stage          string            `json:"stage,omitempty"`
	Done           map[string]bool   `json:"done,omitempty"`
	Choice         *CardChoice       `json:"choice,omitempty"`
	Order          string            `json:"order,omitempty"`
	TurnPosition   *int              `json:"turnPosition,omitempty"`
	RetreatChooser string            `json:"retreatChooser,omitempty"`
	Effects        []string          `json:"effects,omitempty"`
	Tides          *TidesState       `json:"tides,omitempty"`
	BladeBonus     string            `json:"bladeBonus,omitempty"`
}
type BattleView struct {
	ID                int                  `json:"id"`
	Strength          CombatStrength       `json:"strength"`
	From              string               `json:"from"`
	To                string               `json:"to"`
	Attacker          string               `json:"attacker"`
	Defender          string               `json:"defender"`
	Army              []Unit               `json:"army"`
	Chosen            map[string]bool      `json:"chosen"`
	YourCard          string               `json:"yourCard,omitempty"`
	Retreats          []string             `json:"retreats"`
	Loser             string               `json:"loser,omitempty"`
	Cards             map[string]string    `json:"cards,omitempty"`
	Choice            *CardChoice          `json:"choice,omitempty"`
	RetreatChooser    string               `json:"retreatChooser,omitempty"`
	Modifiers         map[string]TidesCard `json:"modifiers,omitempty"`
	ModifiersRevealed bool                 `json:"modifiersRevealed"`
}
type CombatResult struct {
	ID           int                  `json:"id,omitempty"`
	Strength     *CombatStrength      `json:"strength,omitempty"`
	Attacker     string               `json:"attacker"`
	Defender     string               `json:"defender"`
	AttackerCard string               `json:"attackerCard"`
	DefenderCard string               `json:"defenderCard"`
	Attack       int                  `json:"attack"`
	Defense      int                  `json:"defense"`
	Winner       string               `json:"winner"`
	Region       string               `json:"region"`
	Effects      []string             `json:"effects,omitempty"`
	Modifiers    map[string]TidesCard `json:"modifiers,omitempty"`
	BladeBonus   string               `json:"bladeBonus,omitempty"`
	BladeRedraw  string               `json:"bladeRedraw,omitempty"`
	Losses       map[string][]Unit    `json:"losses,omitempty"`
	RetreatHouse string               `json:"retreatHouse,omitempty"`
	RetreatTo    string               `json:"retreatTo,omitempty"`
	ReturnTo     string               `json:"returnTo,omitempty"`
	Final        *CombatFinal         `json:"final,omitempty"`
}

func (r *Room) event(kind, house, from, to, text string) {
	id := 1
	if len(r.Events) > 0 {
		id = r.Events[len(r.Events)-1].ID + 1
	}
	r.Events = append(r.Events, Event{id, kind, house, from, to, text})
	if len(r.Events) > 40 {
		r.Events = append([]Event{}, r.Events[len(r.Events)-40:]...)
	}
	r.Log = append(r.Log, text)
	if len(r.Log) > 100 {
		r.Log = append([]string{}, r.Log[len(r.Log)-100:]...)
	}
}
func (r *Room) initializeMatch(c Catalog) {
	r.Westeros = newWesterosState()
	r.Wildlings = newWildlingState()
	r.Match = r.startingMatch(c)
}
func (r *Room) armies(c Catalog) map[string][]Unit {
	if r.Match != nil {
		return r.Match.Armies
	}
	return c.Units
}
func (r *Room) starLimit(h string, c Catalog) int {
	if r.Match == nil {
		return StarLimit(h, c)
	}
	stars := courtStars(len(r.Seats))
	for i, house := range r.Match.Tracks["kingsCourt"] {
		if house == h && i < len(stars) {
			return stars[i]
		}
	}
	return 0
}
func (r *Room) definition(id string, c Catalog) Region {
	for _, d := range c.Boards["westeros"].Regions {
		if d.ID == id {
			return d
		}
	}
	return Region{}
}
func (r *Room) owner(id string) string {
	if r.Match == nil {
		return ""
	}
	for _, u := range r.Match.Armies[id] {
		if u.Count > 0 {
			return u.House
		}
	}
	return r.Match.Control[id]
}
func count(army []Unit) int {
	n := 0
	for _, u := range army {
		n += u.Count
	}
	return n
}
func (r *Room) regionIDs(house string) []string {
	ids := []string{}
	if r.Match != nil {
		for id, army := range r.Match.Armies {
			if count(army) > 0 && army[0].House == house {
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	return ids
}
func (r *Room) actionRegions(house string) []string {
	ids := []string{}
	kind := "raid"
	if r.Phase == "marches" {
		kind = "march"
	}
	for _, id := range r.regionIDs(house) {
		o, _ := FindOrder(r.Orders[id])
		if o.Kind == kind {
			ids = append(ids, id)
		}
	}
	return ids
}
func (r *Room) beginActions(c Catalog, now time.Time) {
	if r.Match == nil {
		r.initializeMatch(c)
	}
	r.Phase = "raids"
	r.Match.ActiveHouse = ""
	r.nextAction(c, now)
}
func (r *Room) nextAction(c Catalog, now time.Time) {
	for {
		track := r.Match.Tracks["ironThrone"]
		start := 0
		for i, h := range track {
			if h == r.Match.ActiveHouse {
				start = (i + 1) % len(track)
			}
		}
		if r.Match.ResumePosition != nil {
			start = (*r.Match.ResumePosition + 1) % len(track)
			r.Match.ResumePosition = nil
		}
		for i := 0; i < len(track); i++ {
			h := track[(start+i)%len(track)]
			if len(r.actionRegions(h)) > 0 {
				r.Match.ActiveHouse = h
				r.startClock(h, now)
				return
			}
		}
		if r.Phase == "raids" {
			r.Phase = "marches"
			r.Match.ActiveHouse = ""
			continue
		}
		r.finishRound(c, now)
		return
	}
}
func (r *Room) checkTurn(house, phase string) error {
	if r.Match == nil || r.Phase != phase || r.Match.ActiveHouse != house {
		return errors.New("Сейчас не ваш ход")
	}
	return nil
}
func (r *Room) action(house string, cmd Command, c Catalog, now time.Time) error {
	switch cmd.Type {
	case "pass":
		if r.Phase != "raids" && r.Phase != "marches" {
			return errors.New("Сейчас нельзя пропускать приказ")
		}
		if err := r.checkTurn(house, r.Phase); err != nil {
			return err
		}
		regions := r.actionRegions(house)
		if len(regions) == 0 {
			return errors.New("Нет приказов для пропуска")
		}
		id := cmd.Region
		if id == "" {
			id = regions[0]
		}
		valid := false
		for _, x := range regions {
			if x == id {
				valid = true
			}
		}
		if !valid {
			return errors.New("Нет вашего активного приказа")
		}
		delete(r.Orders, id)
		r.event("pass", house, id, "", HouseNames[house]+": приказ снят без выполнения")
		r.nextAction(c, now)
		return nil
	case "raid":
		return r.raid(house, cmd, c, now)
	case "march":
		return r.march(house, cmd, c, now)
	case "card":
		return r.chooseCard(house, cmd.Card, c, now)
	case "retreat":
		return r.retreat(house, cmd.Target, c, now)
	case "ability":
		return r.chooseAbility(house, cmd.Choice, c, now)
	}
	return errors.New("Неизвестное действие")
}
func contains(list []string, id string) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}
func (r *Room) raid(house string, cmd Command, c Catalog, now time.Time) error {
	if err := r.checkTurn(house, "raids"); err != nil {
		return err
	}
	order, ok := FindOrder(r.Orders[cmd.Region])
	if !ok || order.Kind != "raid" || r.owner(cmd.Region) != house {
		return errors.New("Нужен собственный приказ набега")
	}
	if !contains(r.definition(cmd.Region, c).Neighbors, cmd.Target) || r.owner(cmd.Target) == house {
		return errors.New("Набег возможен только в соседнюю чужую область")
	}
	from, to := r.definition(cmd.Region, c), r.definition(cmd.Target, c)
	if from.Type == "land" && to.Type != "land" {
		return errors.New("С суши нельзя совершить набег в море или порт")
	}
	target, ok := FindOrder(r.Orders[cmd.Target])
	if !ok || (target.Kind != "support" && target.Kind != "power" && target.Kind != "raid" && !(order.Star && target.Kind == "defend")) {
		return errors.New("Этот приказ нельзя снять набегом")
	}
	if target.Kind == "power" {
		r.gainPower(house, 1)
		enemy := r.owner(cmd.Target)
		if r.Match.Power[enemy] > 0 {
			r.Match.Power[enemy]--
		}
	}
	delete(r.Orders, cmd.Target)
	delete(r.Orders, cmd.Region)
	r.event("raid", house, cmd.Region, cmd.Target, HouseNames[house]+": набег в "+to.Name)
	r.nextAction(c, now)
	return nil
}
func (r *Room) reachable(from, to, house string, c Catalog) bool {
	if r.Match != nil && (contains(r.Match.BlockedRegions, from) || contains(r.Match.BlockedRegions, to)) {
		return false
	}
	a, b := r.definition(from, c), r.definition(to, c)
	if a.ID == "" || b.ID == "" || from == to {
		return false
	}
	if a.Type != "land" {
		if b.Type == "land" {
			return false
		}
		if b.Type == "port" {
			for _, n := range b.Neighbors {
				if r.definition(n, c).Type == "land" && r.Match.Control[n] != house {
					return false
				}
			}
		}
		return contains(a.Neighbors, to)
	}
	if b.Type != "land" {
		return false
	}
	if contains(a.Neighbors, to) {
		return true
	}
	// Land units may be transported along a connected chain of their own fleets.
	seen := map[string]bool{from: true}
	queue := append([]string{}, a.Neighbors...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		if id == to {
			return true
		}
		d := r.definition(id, c)
		if d.Type != "sea" || r.owner(id) != house || count(r.Match.Armies[id]) == 0 {
			continue
		}
		queue = append(queue, d.Neighbors...)
	}
	return false
}
func (r *Room) supplyOK(house string, armies map[string][]Unit, c Catalog) bool {
	sizes := []int{}
	for _, army := range armies {
		if len(army) > 0 && army[0].House == house && count(army) > 1 {
			sizes = append(sizes, count(army))
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))
	level := r.Match.Supply[house]
	if level < 0 || level >= len(c.SupplyLimits) {
		return false
	}
	limits := c.SupplyLimits[level]
	if len(sizes) > len(limits) {
		return false
	}
	for i, n := range sizes {
		if n > limits[i] {
			return false
		}
	}
	return true
}
func merge(a, b []Unit) []Unit {
	out := append([]Unit{}, a...)
	for _, u := range b {
		found := false
		for i := range out {
			if out[i].Type == u.Type && out[i].House == u.House {
				out[i].Count += u.Count
				found = true
			}
		}
		if !found {
			out = append(out, u)
		}
	}
	return out
}
func (r *Room) march(house string, cmd Command, c Catalog, now time.Time) error {
	if err := r.checkTurn(house, "marches"); err != nil {
		return err
	}
	order, ok := FindOrder(r.Orders[cmd.Region])
	if !ok || order.Kind != "march" || r.owner(cmd.Region) != house {
		return errors.New("Нужен собственный приказ похода")
	}
	if !r.reachable(cmd.Region, cmd.Target, house, c) {
		return errors.New("Область недоступна для похода: проверьте соседство и транспорт")
	}
	original := r.Match.Armies[cmd.Region]
	moved := []Unit{}
	left := []Unit{}
	for typ, n := range cmd.Units {
		if n < 0 {
			return errors.New("Отрицательное количество отрядов")
		}
		known := false
		for _, u := range original {
			if u.Type == typ {
				known = true
				if n > u.Count {
					return errors.New("Недостаточно отрядов")
				}
			}
		}
		if !known && n != 0 {
			return errors.New("Неизвестный тип отряда")
		}
	}
	for _, u := range original {
		n := cmd.Units[u.Type]
		if n > 0 {
			moved = append(moved, Unit{u.Type, house, n})
		}
		if u.Count > n {
			left = append(left, Unit{u.Type, house, u.Count - n})
		}
	}
	if count(moved) == 0 {
		return errors.New("Выберите хотя бы один отряд")
	}
	destination := r.Match.Armies[cmd.Target]
	enemy := r.owner(cmd.Target)
	hostile := enemy != "" && enemy != house && (count(destination) > 0 || r.Match.Garrisons[cmd.Target] > 0)
	candidate := map[string][]Unit{}
	for id, army := range r.Match.Armies {
		candidate[id] = army
	}
	candidate[cmd.Region] = left
	if !hostile {
		candidate[cmd.Target] = merge(destination, moved)
	} else {
		candidate[cmd.Target] = moved
	}
	if !r.supplyOK(house, candidate, c) {
		return errors.New("Поход нарушает ограничения снабжения")
	}
	if cmd.LeaveToken && count(left) == 0 && r.definition(cmd.Region, c).Type == "land" && r.definition(cmd.Region, c).Home != house && r.Match.ControlTokens[cmd.Region] != house && r.Match.Power[house] < 1 {
		return errors.New("Нет жетона власти для сохранения контроля")
	}
	if enemy == "" && r.Match.Garrisons[cmd.Target] > 0 {
		strength := armyStrength(moved, r.definition(cmd.Target, c).Castle > 0) + order.Modifier + r.support(house, cmd.Target, c)
		if strength < r.Match.Garrisons[cmd.Target] {
			return errors.New("Недостаточно силы для атаки нейтрального гарнизона")
		}
	}
	r.Match.Armies[cmd.Region] = left
	delete(r.Orders, cmd.Region)
	if count(left) == 0 && r.definition(cmd.Region, c).Type == "land" {
		if cmd.LeaveToken && r.definition(cmd.Region, c).Home != house && r.Match.ControlTokens[cmd.Region] != house {
			r.Match.Power[house]--
			if r.Match.ControlTokens == nil {
				r.Match.ControlTokens = map[string]string{}
			}
			r.Match.ControlTokens[cmd.Region] = house
		}
		r.restoreEmptyControl(cmd.Region, c)
	}
	r.event("march", house, cmd.Region, cmd.Target, HouseNames[house]+": поход в "+r.definition(cmd.Target, c).Name)
	if hostile {
		r.Battle = &Battle{From: cmd.Region, To: cmd.Target, Attacker: house, Defender: enemy, Army: moved, March: order.Modifier, Cards: map[string]string{}}
		r.Battle.Order = order.ID
		r.rememberCombat(c)
		for i, h := range r.Match.Tracks["ironThrone"] {
			if h == house {
				position := i
				r.Battle.TurnPosition = &position
				break
			}
		}
		r.Phase = "combat"
		r.Match.ActiveHouse = house
		r.startClock(house, now)
		r.event("combat", house, cmd.Region, cmd.Target, "Начался бой: "+HouseNames[house]+" против "+HouseNames[enemy])
		return nil
	}
	r.Match.Armies[cmd.Target] = merge(destination, moved)
	delete(r.Match.Garrisons, cmd.Target)
	if r.definition(cmd.Target, c).Type == "land" {
		if r.Match.ControlTokens[cmd.Target] != house {
			delete(r.Match.ControlTokens, cmd.Target)
		}
		r.Match.Control[cmd.Target] = house
	}
	if r.checkVictory(c) {
		return nil
	}
	r.nextAction(c, now)
	return nil
}
func armyStrength(army []Unit, castle bool) int {
	n := 0
	for _, u := range army {
		v := 1
		if u.Type == "knight" {
			v = 2
		}
		if u.Type == "siege-engine" {
			v = 0
			if castle {
				v = 4
			}
		}
		n += v * u.Count
	}
	return n
}
func (r *Room) support(house, target string, c Catalog) int {
	total := 0
	to := r.definition(target, c)
	for _, id := range to.Neighbors {
		o, _ := FindOrder(r.Orders[id])
		if o.Kind != "support" || r.owner(id) != house {
			continue
		}
		from := r.definition(id, c)
		if from.Type == "land" && to.Type != "land" {
			continue
		}
		total += armyStrength(r.Match.Armies[id], to.Castle > 0) + o.Modifier
	}
	return total
}
func (r *Room) availableCards(house string, c Catalog) []string {
	ids := []string{}
	if house == "" {
		return ids
	}
	for id := range c.Houses[house].Cards {
		if r.Match == nil || !contains(r.Match.Discard[house], id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
func (r *Room) battleView(house string, c Catalog) *BattleView {
	b := r.Battle
	v := &BattleView{From: b.From, To: b.To, Attacker: b.Attacker, Defender: b.Defender, Army: append([]Unit{}, b.Army...), Chosen: map[string]bool{}, Loser: b.Loser, Retreats: []string{}}
	v.ID = b.ID
	v.Strength = r.combatBreakdown(c, b.Revealed)
	if record := r.combatRecord(); record != nil && record.Result != nil && record.Result.Strength != nil {
		v.Strength = *record.Result.Strength
	}
	for h, card := range b.Cards {
		v.Chosen[h] = card != ""
		if h == house {
			v.YourCard = card
		}
	}
	if b.Loser == house {
		v.Retreats = r.retreatRegions(c)
	}
	if b.Revealed {
		v.Cards = map[string]string{}
		for h, id := range b.Cards {
			v.Cards[h] = id
		}
	}
	v.RetreatChooser = r.retreatChooser()
	if b.Tides != nil {
		v.Modifiers = map[string]TidesCard{}
		v.ModifiersRevealed = b.Tides.Revealed
		for owner, card := range b.Tides.Cards {
			if owner == house || b.Tides.Revealed {
				v.Modifiers[owner] = card
			}
		}
	}
	if r.Phase == "retreat" && v.RetreatChooser == house {
		v.Retreats = r.retreatRegions(c)
	} else if r.Phase == "retreat" {
		v.Retreats = []string{}
	}
	if b.Loser != "" {
		v.Army = append([]Unit{}, b.Survivors...)
	}
	if b.Choice != nil {
		choice := *b.Choice
		choice.Options = append([]string{}, b.Choice.Options...)
		if house != choice.House {
			choice.Options = nil
		}
		v.Choice = &choice
	}
	return v
}
func (r *Room) chooseCard(house, card string, c Catalog, now time.Time) error {
	b := r.Battle
	if r.Phase != "combat" || b == nil || (house != b.Attacker && house != b.Defender) {
		return errors.New("Вы не участвуете в этом бою")
	}
	if b.Cards[house] != "" {
		return errors.New("Карта уже выбрана")
	}
	if !contains(r.availableCards(house, c), card) {
		return errors.New("Эта карта недоступна")
	}
	r.rememberCombat(c)
	b.Cards[house] = card
	if b.Cards[b.Attacker] == "" || b.Cards[b.Defender] == "" {
		waiting := b.Attacker
		if b.Cards[waiting] != "" {
			waiting = b.Defender
		}
		r.Match.ActiveHouse = waiting
		r.startClock(waiting, now)
		return nil
	}
	b.Revealed, b.Stage, b.Done = true, "before", map[string]bool{}
	r.advanceCardEffects(c, now)
	return nil
}
func removeCasualties(army []Unit, n int) []Unit {
	return removeCombatUnits(army, n, true)
}

// Defeated siege engines die independently of icons. A skull hitting the
// winning army removes only the requested number of units, including sieges.
func removeCombatUnits(army []Unit, n int, defeated bool) []Unit {
	if !defeated && n <= 0 {
		return append([]Unit{}, army...)
	}
	out := []Unit{}
	sorted := append([]Unit{}, army...)
	sort.SliceStable(sorted, func(i, j int) bool {
		priority := map[string]int{"siege-engine": 0, "footman": 1, "ship": 1, "knight": 2}
		return priority[sorted[i].Type] < priority[sorted[j].Type]
	})
	for _, u := range sorted {
		if defeated && u.Type == "siege-engine" {
			continue
		}
		kill := min(n, u.Count)
		n -= kill
		u.Count -= kill
		if u.Count > 0 {
			out = append(out, u)
		}
	}
	return out
}
func (r *Room) resolveCombat(c Catalog, now time.Time) {
	b := r.Battle
	r.rememberCombat(c)
	cards := r.combatCards(c)
	a, d := cards[b.Attacker], cards[b.Defender]
	strength := r.combatBreakdown(c, true)
	attack, defense := strength.Attacker.Total, strength.Defender.Total
	modifiers := r.tidesCards()
	a.Swords += modifiers[b.Attacker].Swords
	a.Towers += modifiers[b.Attacker].Towers
	d.Swords += modifiers[b.Defender].Swords
	d.Towers += modifiers[b.Defender].Towers
	order, _ := FindOrder(r.Orders[b.To])
	winner := b.Attacker
	if defense > attack {
		winner = b.Defender
	}
	if defense == attack {
		for _, h := range r.Match.Tracks["fiefdoms"] {
			if h == b.Attacker || h == b.Defender {
				winner = h
				break
			}
		}
	}
	r.Match.LastCombat = &CombatResult{Attacker: b.Attacker, Defender: b.Defender, AttackerCard: b.Cards[b.Attacker], DefenderCard: b.Cards[b.Defender], Attack: attack, Defense: defense, Winner: winner, Region: b.To, Effects: append([]string{}, b.Effects...)}
	r.Match.LastCombat.ID, r.Match.LastCombat.Strength = b.ID, &strength
	r.combatRecord().Result = r.Match.LastCombat
	r.Match.LastCombat.Modifiers = modifiers
	r.Match.LastCombat.BladeBonus = r.battleBladeBonus()
	if modifiers != nil {
		r.Match.LastCombat.BladeRedraw = b.Tides.BladeRedraw
		for _, house := range []string{b.Attacker, b.Defender} {
			r.event("tides", house, "", "", HouseNames[house]+fmt.Sprintf(": карта перевеса — сила +%d, мечи %d, крепости %d, черепа %d", modifiers[house].Strength, modifiers[house].Swords, modifiers[house].Towers, modifiers[house].Skulls))
		}
	}
	for _, h := range r.battleHouses() {
		id := b.Cards[h]
		printed := c.Houses[h].Cards[id]
		effective := cards[h]
		if printed.Strength != effective.Strength || printed.Swords != effective.Swords || printed.Towers != effective.Towers {
			r.effect(h, id, fmt.Sprintf("сила карты %d → %d; мечи %d → %d; башни %d → %d", printed.Strength, effective.Strength, printed.Swords, effective.Swords, printed.Towers, effective.Towers), c)
		}
		if id == "catelyn-stark" && h == b.Defender && order.Kind == "defend" {
			r.effect(h, id, "бонус приказа обороны удвоен", c)
		}
		if id == "ser-kevan-lannister" && h == b.Attacker || id == "victarion-greyjoy" && h == b.Attacker {
			r.effect(h, id, "пересчитана сила участвующих и поддерживающих отрядов", c)
		}
		if id == "salladhor-saan" && len(r.supportingRegions(h, c)) > 0 {
			r.effect(h, id, "обнулена сила кораблей противника на этапе способности; последующие эффекты применяются по Железному трону", c)
		}
	}
	b.Loser = b.Defender
	casualties := max(0, a.Swords-d.Towers)
	if d.Ability == "the-blackfish" {
		casualties = 0
	}
	b.Survivors = removeCasualties(r.Match.Armies[b.To], casualties)
	if winner == b.Defender {
		b.Loser = b.Attacker
		casualties = max(0, d.Swords-a.Towers)
		if a.Ability == "the-blackfish" {
			casualties = 0
		}
		b.Survivors = removeCasualties(b.Army, casualties)
		if a.Ability != "the-blackfish" {
			b.Survivors = removeCombatUnits(b.Survivors, modifiers[b.Defender].Skulls, false)
		}
		if d.Ability != "the-blackfish" {
			r.Match.Armies[b.To] = removeCombatUnits(r.Match.Armies[b.To], modifiers[b.Attacker].Skulls, false)
		}
		if count(r.Match.Armies[b.To]) == 0 {
			delete(r.Orders, b.To)
		}
	} else {
		if d.Ability != "the-blackfish" {
			b.Survivors = removeCombatUnits(b.Survivors, modifiers[b.Attacker].Skulls, false)
		}
		if a.Ability != "the-blackfish" {
			b.Army = removeCombatUnits(b.Army, modifiers[b.Defender].Skulls, false)
		}
		// Winning still defeats the garrison even if a skull kills the last attacker.
		delete(r.Match.Garrisons, b.To)
		if d.Ability == "arianne-martell" {
			r.Match.LastCombat.ReturnTo = b.From
			r.Match.Armies[b.From] = merge(r.Match.Armies[b.From], b.Army)
			r.Match.Armies[b.To] = nil
			if home := r.definition(b.To, c).Home; home != "" && r.houseSeat(home) != nil {
				r.Match.Control[b.To] = home
			} else {
				delete(r.Match.Control, b.To)
			}
			r.effect(b.Defender, "arianne-martell", "победившие отряды возвращены в исходную область", c)
		} else {
			r.Match.Armies[b.To] = append([]Unit{}, b.Army...)
			if count(b.Army) > 0 {
				if r.Match.ControlTokens[b.To] != b.Attacker {
					delete(r.Match.ControlTokens, b.To)
				}
				r.Match.Control[b.To] = b.Attacker
			} else if home := r.definition(b.To, c).Home; home != "" && r.houseSeat(home) != nil {
				r.Match.Control[b.To] = home
			} else {
				delete(r.Match.Control, b.To)
			}
		}
		delete(r.Orders, b.To)
		if a.Ability == "ser-loras-tyrell" && d.Ability != "arianne-martell" && count(b.Army) > 0 && b.Order != "" {
			r.Orders[b.To] = b.Order
			r.effect(b.Attacker, "ser-loras-tyrell", "приказ похода перенесён в занятую область", c)
		}
	}
	if b.Cards[winner] == "tywin-lannister" {
		r.gainPower(winner, 2)
		r.effect(winner, "tywin-lannister", "получено до двух жетонов власти", c)
	}
	record := r.combatRecord()
	attackers, defenders := b.Army, b.Survivors
	if winner == b.Defender {
		attackers, defenders = b.Survivors, r.Match.Armies[b.To]
	}
	r.Match.LastCombat.Losses = map[string][]Unit{b.Attacker: combatLosses(record.Attackers, attackers), b.Defender: combatLosses(record.Defenders, defenders)}
	if count(b.Survivors) > 0 {
		r.Match.LastCombat.RetreatHouse = b.Loser
	}
	r.Match.LastCombat.Effects = append([]string{}, b.Effects...)
	r.event("victory", winner, b.From, b.To, fmt.Sprintf("Бой: %d : %d. Победа дома %s", attack, defense, HouseNames[winner]))
	b.Stage = "post"
	r.advanceCardEffects(c, now)
}
func (r *Room) retreatRegions(c Catalog) []string {
	b := r.Battle
	ids := []string{}
	if b == nil || b.Loser == "" {
		return ids
	}
	if b.Loser == b.Attacker && b.RetreatChooser == "" {
		if owner := r.owner(b.From); owner != "" && owner != b.Loser {
			return ids
		}
		return []string{b.From}
	}
	for _, region := range c.Boards["westeros"].Regions {
		id := region.ID
		if !r.reachable(b.To, id, b.Loser, c) || id == b.From && b.Loser == b.Defender {
			continue
		}
		d := r.definition(id, c)
		from := r.definition(b.To, c)
		if (d.Type == "land") != (from.Type == "land") {
			continue
		}
		if owner := r.owner(id); owner != "" && owner != b.Loser {
			continue
		}
		if r.Match.Garrisons[id] > 0 && d.Home != b.Loser {
			continue
		}
		armies := map[string][]Unit{}
		for k, v := range r.Match.Armies {
			armies[k] = v
		}
		armies[id] = merge(armies[id], b.Survivors)
		if r.supplyOK(b.Loser, armies, c) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
func (r *Room) retreat(house, target string, c Catalog, now time.Time) error {
	if r.Phase != "retreat" || r.Battle == nil || r.retreatChooser() != house {
		return errors.New("Сейчас не ваше отступление")
	}
	if !contains(r.retreatRegions(c), target) {
		return errors.New("Недопустимая область отступления")
	}
	b := r.Battle
	r.Match.Armies[target] = merge(r.Match.Armies[target], b.Survivors)
	if r.definition(target, c).Type == "land" {
		r.Match.Control[target] = b.Loser
	}
	r.event("retreat", b.Loser, b.To, target, HouseNames[b.Loser]+": отступление в "+r.definition(target, c).Name)
	if result := r.Match.LastCombat; result != nil && result.ID == b.ID {
		result.RetreatHouse, result.RetreatTo = b.Loser, target
		if record := r.combatRecord(); record != nil {
			record.Result = result
		}
	}
	b.Survivors = nil
	r.completeCombat(c, now)
	return nil
}
func (r *Room) completeCombat(c Catalog, now time.Time) {
	b := r.Battle
	if b.Stage != "after" {
		// Survivors still here have no legal retreat; an actual retreat clears
		// this slice before calling completeCombat. Record their destruction.
		if count(b.Survivors) > 0 {
			if result := r.Match.LastCombat; result != nil && result.ID == b.ID {
				result.Losses[b.Loser] = merge(result.Losses[b.Loser], b.Survivors)
				result.RetreatHouse = ""
				if record := r.combatRecord(); record != nil {
					record.Result = result
				}
			}
			b.Survivors = nil
		}
		b.Stage = "after"
		for _, h := range []string{b.Attacker, b.Defender} {
			r.discardCard(h, b.Cards[h], c)
		}
		if b.Cards[b.Loser] == "roose-bolton" {
			r.Match.Discard[b.Loser] = []string{}
			r.effect(b.Loser, "roose-bolton", "весь сброс, включая Русе, возвращён в руку", c)
		}
		for id := range r.Orders {
			if count(r.Match.Armies[id]) == 0 {
				delete(r.Orders, id)
			}
		}
	}
	r.advanceCardEffects(c, now)
}
func (r *Room) finishCombat(c Catalog, now time.Time) {
	r.rememberCombatFinal()
	attacker := r.Battle.Attacker
	r.Match.ResumePosition = r.Battle.TurnPosition
	r.Battle = nil
	if r.checkVictory(c) {
		return
	}
	r.Phase = "marches"
	r.Match.ActiveHouse = attacker
	r.nextAction(c, now)
}
func (r *Room) gainPower(house string, n int) {
	placed := 0
	for _, owner := range r.Match.ControlTokens {
		if owner == house {
			placed++
		}
	}
	r.Match.Power[house] = min(max(0, 20-placed), max(0, r.Match.Power[house]+n))
}
func (r *Room) castles(house string, c Catalog) int {
	n := 0
	for _, region := range c.Boards["westeros"].Regions {
		if region.Castle > 0 && r.Match.Control[region.ID] == house {
			n++
		}
	}
	return n
}
func (r *Room) checkVictory(c Catalog) bool {
	for _, s := range r.Seats {
		if r.castles(s.House, c) >= 7 {
			r.endGame(s.House)
			return true
		}
	}
	return false
}
func (r *Room) endGame(winner string) {
	r.Match.Winner = winner
	r.Match.ActiveHouse = ""
	r.Phase = "finished"
	r.Clock = Clock{}
	r.event("game-over", winner, "", "", "Победил дом "+HouseNames[winner])
}
func (r *Room) finishRound(c Catalog, now time.Time) {
	r.advancePower(c, now)
}
func (r *Room) completeRound(c Catalog, now time.Time) {
	if r.Round >= 10 {
		best := ""
		for _, h := range r.Match.Tracks["ironThrone"] {
			if best == "" || r.castles(h, c) > r.castles(best, c) {
				best = h
			}
		}
		r.endGame(best)
		return
	}
	r.Round++
	r.Phase, r.Match.ActiveHouse, r.Clock = PhaseWesteros, "", Clock{}
	if err := r.BeginWesteros(now); err != nil {
		// Corrupt snapshots must not silently skip events or reopen planning.
		r.event("rules-error", "", "", "", err.Error())
	}
}

func (r *Room) beginPlanningRound(c Catalog, now time.Time) {
	r.Orders = map[string]string{}
	r.Phase = "planning"
	r.Match.ActiveHouse = ""
	for i := range r.Seats {
		r.Seats[i].Ready = false
	}
	r.startClock("", now)
	r.event("round", "", "", "", fmt.Sprintf("Раунд %d: назначьте новые тайные приказы", r.Round))
	r.runPlanningBots(c)
}
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
