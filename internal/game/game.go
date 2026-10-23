package game

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"sort"
	"time"
)

const RulesStage = "tactical-prototype"

type Seat struct {
	House      string `json:"house"`
	Player     string `json:"player,omitempty"`
	Name       string `json:"name"`
	Bot        bool   `json:"bot"`
	Difficulty string `json:"difficulty"`
	Ready      bool   `json:"ready"`
}
type Room struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Host      string            `json:"host"`
	Revision  int64             `json:"revision"`
	Phase     string            `json:"phase"`
	Round     int               `json:"round"`
	Seats     []Seat            `json:"seats"`
	Orders    map[string]string `json:"orders"`
	Log       []string          `json:"log"`
	Settings  Settings          `json:"settings"`
	Clock     Clock             `json:"clock"`
	Match     *Match            `json:"match,omitempty"`
	Battle    *Battle           `json:"battle,omitempty"`
	Events    []Event           `json:"events"`
	Influence *InfluenceAuction `json:"influence,omitempty"`
	Supply    *SupplyResolution `json:"supply,omitempty"`
	Mustering *Mustering        `json:"mustering,omitempty"`
	Westeros  *WesterosState    `json:"westeros,omitempty"`
	Wildlings *WildlingState    `json:"wildlings,omitempty"`
	Raven     *RavenState       `json:"raven,omitempty"`
}
type Command struct {
	Type       string         `json:"type"`
	Revision   int64          `json:"revision"`
	House      string         `json:"house,omitempty"`
	Region     string         `json:"region,omitempty"`
	Order      string         `json:"order,omitempty"`
	Bot        bool           `json:"bot,omitempty"`
	Difficulty string         `json:"difficulty,omitempty"`
	Settings   *Settings      `json:"settings,omitempty"`
	Target     string         `json:"target,omitempty"`
	Units      map[string]int `json:"units,omitempty"`
	Card       string         `json:"card,omitempty"`
	LeaveToken bool           `json:"leaveToken,omitempty"`
	Choice     string         `json:"choice,omitempty"`
	Bid        *int           `json:"bid,omitempty"`
	Unit       UnitType       `json:"unit,omitempty"`
	Upgrade    bool           `json:"upgrade,omitempty"`
}
type SeatView struct {
	House      string `json:"house"`
	Name       string `json:"name"`
	Bot        bool   `json:"bot"`
	Difficulty string `json:"difficulty"`
	Ready      bool   `json:"ready"`
	You        bool   `json:"you"`
	Occupied   bool   `json:"occupied"`
}
type View struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	Revision        int64                    `json:"revision"`
	Phase           string                   `json:"phase"`
	Round           int                      `json:"round"`
	Owner           bool                     `json:"owner"`
	YourHouse       string                   `json:"yourHouse"`
	Seats           []SeatView               `json:"seats"`
	Orders          map[string]string        `json:"orders"`
	Log             []string                 `json:"log"`
	RulesStage      string                   `json:"rulesStage"`
	Settings        Settings                 `json:"settings"`
	Clock           Clock                    `json:"clock"`
	ServerTime      time.Time                `json:"serverTime"`
	Match           *Match                   `json:"match,omitempty"`
	Battle          *BattleView              `json:"battle,omitempty"`
	Events          []Event                  `json:"events"`
	AvailableCards  []string                 `json:"availableCards"`
	LegalRegions    []string                 `json:"legalRegions"`
	AvailableOrders map[string][]string      `json:"availableOrders"`
	OrderBudget     *OrderBudget             `json:"orderBudget,omitempty"`
	Influence       *InfluenceView           `json:"influence,omitempty"`
	Supply          *SupplyView              `json:"supply,omitempty"`
	Mustering       *MusterView              `json:"mustering,omitempty"`
	Westeros        *WesterosView            `json:"westeros,omitempty"`
	Wildlings       *WildlingView            `json:"wildlings,omitempty"`
	Raven           *RavenView               `json:"raven,omitempty"`
	Setup           *SetupView               `json:"setup,omitempty"`
	Fortifications  map[string]Fortification `json:"fortifications,omitempty"`
	CourtStars      []int                    `json:"courtStars"`
}

type OrderBudget struct {
	Power        int `json:"power"`
	StarsAllowed int `json:"starsAllowed"`
	StarsUsed    int `json:"starsUsed"`
}

func NewRoom(id, name, host, playerName string) *Room {
	r := &Room{ID: id, Name: name, Host: host, Revision: 1, Phase: "lobby", Round: 1, Orders: map[string]string{}, Settings: DefaultSettings(), Log: []string{"Создана комната. Ссылка приглашает в игру, но не открывает чужое место."}}
	for i, h := range BaseHouses {
		s := Seat{House: h, Bot: i != 0, Difficulty: "normal"}
		if i == 0 {
			s.Player = host
			s.Name = playerName
		} else {
			s.Name = "Бот · " + HouseNames[h]
		}
		r.Seats = append(r.Seats, s)
	}
	return r
}
func (r *Room) Clone() *Room {
	b, _ := json.Marshal(r)
	var c Room
	if err := json.Unmarshal(b, &c); err != nil {
		panic(err)
	}
	return &c
}
func (r *Room) Seat(player string) *Seat {
	for i := range r.Seats {
		if r.Seats[i].Player == player && player != "" {
			return &r.Seats[i]
		}
	}
	return nil
}
func (r *Room) Join(player, name, house string) error {
	if r.Seat(player) != nil {
		return nil
	}
	if r.Phase != "lobby" {
		return errors.New("Игра уже началась")
	}
	for i := range r.Seats {
		s := &r.Seats[i]
		if !s.Bot && s.Player == "" && (house == "" || s.House == house) {
			s.Player = player
			s.Name = name
			return nil
		}
	}
	for i := range r.Seats {
		s := &r.Seats[i]
		if s.Player == "" && (house == "" || s.House == house) {
			s.Player, s.Name, s.Bot, s.Ready = player, name, false, false
			return nil
		}
	}
	return errors.New("Дом занят другим игроком или не существует")
}

// ChooseHouse moves only the caller. A displaced bot fills the previous seat;
// an open human seat stays open, so the host's seat configuration is preserved.
func (r *Room) ChooseHouse(player, house string) error {
	if r.Phase != "lobby" {
		return errors.New("Выбрать дом можно только до начала игры")
	}
	current := r.Seat(player)
	if current == nil {
		return errors.New("Вы не участник этой комнаты")
	}
	if current.House == house {
		return nil
	}
	for i := range r.Seats {
		target := &r.Seats[i]
		if target.House != house {
			continue
		}
		if target.Player != "" {
			return errors.New("Дом занят другим игроком")
		}
		target.Player, target.Name, current.Bot = current.Player, current.Name, target.Bot
		target.Bot, target.Ready = false, false
		current.Player, current.Name, current.Ready = "", "", false
		if current.Bot {
			current.Name = "Бот · " + HouseNames[current.House]
		}
		return nil
	}
	return errors.New("Неизвестный дом")
}
func (r *Room) Apply(player string, c Command, catalog Catalog) error {
	return r.ApplyAt(player, c, catalog, time.Now().UTC())
}
func (r *Room) ApplyAt(player string, c Command, catalog Catalog, now time.Time) error {
	seat := r.Seat(player)
	if seat == nil {
		return errors.New("Вы не участник этой комнаты")
	}
	switch c.Type {
	case "choose_house":
		return r.ChooseHouse(player, c.House)
	case "settings":
		if player != r.Host || r.Phase != "lobby" {
			return errors.New("Настройки доступны ведущему до начала игры")
		}
		if c.Settings == nil {
			return errors.New("Не указаны настройки")
		}
		if err := c.Settings.Validate(); err != nil {
			return err
		}
		return r.configureSettings(*c.Settings, catalog)
	case "configure":
		if player != r.Host || r.Phase != "lobby" {
			return errors.New("Настройки доступны только ведущему до старта")
		}
		if c.Difficulty != "easy" && c.Difficulty != "normal" && c.Difficulty != "hard" {
			return errors.New("Неизвестная сложность")
		}
		for i := range r.Seats {
			s := &r.Seats[i]
			if s.House == c.House {
				if s.Player != "" {
					return errors.New("Нельзя заменить занятого игрока")
				}
				s.Bot = c.Bot
				s.Difficulty = c.Difficulty
				s.Name = ""
				if s.Bot {
					s.Name = "Бот · " + HouseNames[s.House]
				}
				return nil
			}
		}
		return errors.New("Неизвестный дом")
	case "start":
		if player != r.Host || r.Phase != "lobby" {
			return errors.New("Запуск доступен ведущему в лобби")
		}
		if err := r.configureSettings(r.Settings, catalog); err != nil {
			return err
		}
		for _, s := range r.Seats {
			if !s.Bot && s.Player == "" {
				return errors.New("Остались свободные места")
			}
		}
		r.Phase = "planning"
		r.initializeMatch(catalog)
		r.startClock("", now)
		r.Log = append(r.Log, "Начато тайное планирование первого раунда.")
		r.runPlanningBots(catalog)
		return nil
	case "order":
		return r.placeOrder(seat, c.Region, c.Order, catalog)
	case "bid":
		return r.submitInfluenceBid(seat.House, c.Bid, now)
	case "disband":
		return r.disband(seat.House, c.Region, c.Units, catalog, now)
	case "muster":
		return r.muster(seat.House, MusterOption{Region: c.Region, Target: c.Target, Unit: c.Unit, Upgrade: c.Upgrade}, catalog, now)
	case "finish_muster":
		return r.finishMuster(seat.House, catalog, now)
	case "consolidate":
		return r.consolidate(seat.House, c.Region, c.Choice, catalog, now)
	case "influence_tie":
		return r.chooseInfluenceTie(seat.House, c.Target, now)
	case "westeros_choice":
		return r.chooseWesteros(seat.House, WesterosDecision(c.Choice), catalog, now)
	case "wildling_bid":
		return r.submitWildlingBid(seat.House, c.Bid, now)
	case "wildling_tie":
		return r.chooseWildlingTie(seat.House, c.Target)
	case "wildling_choice":
		return r.chooseWildlingEffect(seat.House, c.Choice, catalog)
	case "raven":
		return r.chooseRaven(seat.House, RavenDecision(c.Choice), c.Region, c.Order, catalog, now)
	case "ready":
		if r.Phase != "planning" {
			return errors.New("Сейчас не фаза планирования")
		}
		// Orders are optional; a player may intentionally leave areas without a token.
		seat.Ready = true
		r.revealIfReady()
		return nil
	case "continue":
		if r.Phase != "orders_revealed" {
			return errors.New("Приказы ещё не раскрыты")
		}
		return r.beginRaven(now)
	case "raid", "march", "pass", "card", "retreat", "ability":
		if err := r.action(seat.House, c, catalog, now); err != nil {
			return err
		}
		return nil
	case "unready":
		if r.Phase != "planning" {
			return errors.New("Приказы уже раскрыты")
		}
		seat.Ready = false
		return nil
	default:
		return errors.New("Это действие ещё не реализовано в Go-движке")
	}
}
func (r *Room) placeOrder(seat *Seat, region, orderID string, catalog Catalog) error {
	if err := r.validateOrder(seat, region, orderID, catalog); err != nil {
		return err
	}
	if orderID == "" {
		delete(r.Orders, region)
	} else {
		r.Orders[region] = orderID
	}
	return nil
}

// The personal UI and command path use exactly the same non-mutating validator.
func (r *Room) validateOrder(seat *Seat, region, orderID string, catalog Catalog) error {
	if r.Phase != "planning" || seat.Ready {
		return errors.New("Приказы можно менять до подтверждения готовности")
	}
	return r.validateOrderToken(seat, region, orderID, catalog)
}

// Token rules are shared by secret planning and the Raven's one public swap.
// Phase/actor restrictions belong to the calling operation, not a fake planning
// snapshot with Ready temporarily cleared.
func (r *Room) validateOrderToken(seat *Seat, region, orderID string, catalog Catalog) error {
	owned := false
	for _, u := range r.armies(catalog)[region] {
		if u.House == seat.House && u.Count > 0 {
			owned = true
		}
	}
	if !owned {
		return errors.New("В области нет ваших отрядов")
	}
	if orderID == "" {
		return nil
	}
	order, ok := FindOrder(orderID)
	if !ok {
		return errors.New("Неизвестный приказ")
	}
	if r.westerosOrderRestricted(order) {
		return errors.New("Этот приказ запрещён событием Вестероса текущего раунда")
	}
	if order.Kind == "power" && r.definition(region, catalog).Type != "land" {
		return errors.New("Сбор власти в текущем прототипе доступен только на суше")
	}
	stars := 0
	for other, id := range r.Orders {
		if other == region {
			continue
		}
		ours := false
		for _, u := range r.armies(catalog)[other] {
			if u.House == seat.House && u.Count > 0 {
				ours = true
			}
		}
		if !ours {
			continue
		}
		if id == orderID {
			return errors.New("Этот жетон уже используется")
		}
		o, _ := FindOrder(id)
		if o.Star {
			stars++
		}
	}
	if order.Star && stars >= r.starLimit(seat.House, catalog) {
		return errors.New("Превышен лимит особых приказов на Королевском дворе")
	}
	return nil
}

func (r *Room) availableOrders(seat *Seat, catalog Catalog) map[string][]string {
	result := map[string][]string{}
	if seat == nil || seat.Ready || r.Phase != "planning" {
		return result
	}
	for region, units := range r.armies(catalog) {
		owned := false
		for _, unit := range units {
			if unit.House == seat.House && unit.Count > 0 {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}
		result[region] = []string{}
		for _, order := range Orders {
			if r.validateOrder(seat, region, order.ID, catalog) == nil {
				result[region] = append(result[region], order.ID)
			}
		}
	}
	return result
}
func StarLimit(house string, c Catalog) int {
	limits := []int{3, 3, 2, 1, 0, 0}
	for rank, h := range c.Tracks["kingsCourt"] {
		if h == house && rank < len(limits) {
			return limits[rank]
		}
	}
	return 0
}
func (r *Room) revealIfReady() {
	for _, s := range r.Seats {
		if !s.Ready {
			return
		}
	}
	r.Phase = "orders_revealed"
	r.Clock = Clock{}
	r.Log = append(r.Log, "Все игроки подтвердили приказы. Приказы раскрыты одновременно.")
}
func (r *Room) View(player string, catalog Catalog) View {
	v := View{ID: r.ID, Name: r.Name, Revision: r.Revision, Phase: r.Phase, Round: r.Round, Owner: player == r.Host, Orders: map[string]string{}, Log: append([]string{}, r.Log...), RulesStage: RulesStage}
	if s := r.Seat(player); s != nil {
		v.YourHouse = s.House
	}
	v.AvailableOrders = r.availableOrders(r.Seat(player), catalog)
	v.CourtStars = courtStars(len(r.Seats))
	if r.Match != nil && v.YourHouse != "" {
		budget := &OrderBudget{Power: r.Match.Power[v.YourHouse], StarsAllowed: r.starLimit(v.YourHouse, catalog)}
		for region, id := range r.Orders {
			order, ok := FindOrder(id)
			if ok && order.Star && r.owner(region) == v.YourHouse {
				budget.StarsUsed++
			}
		}
		v.OrderBudget = budget
	}
	v.Settings = r.Settings
	v.Clock = r.Clock
	v.ServerTime = time.Now().UTC()
	v.Match = r.Match
	if r.Match == nil {
		preview := r.startingMatch(catalog)
		v.Setup = &SetupView{PlayerCount: r.Settings.houseCount(), Armies: preview.Armies, Control: preview.Control, Tracks: preview.Tracks}
		v.Fortifications = r.fortifications(preview, catalog)
	} else {
		v.Fortifications = r.fortifications(r.Match, catalog)
	}
	if r.Match != nil {
		// Empty armies are legal after movement/combat. Never serialize them as
		// null: clients render these collections with map/some/iteration.
		match := *r.Match
		match.Armies = make(map[string][]Unit, len(r.Match.Armies))
		for region, units := range r.Match.Armies {
			match.Armies[region] = append([]Unit{}, units...)
		}
		v.Match = &match
	}
	v.Influence = r.influenceView(v.YourHouse)
	v.Supply = r.supplyView(catalog)
	v.Mustering = r.musterView(catalog)
	v.Westeros = r.Westeros.view()
	v.Wildlings = r.Wildlings.view(v.YourHouse)
	v.Raven = r.ravenView(v.YourHouse, catalog)
	v.Events = append([]Event{}, r.Events...)
	if r.Match != nil {
		v.AvailableCards = r.availableCards(v.YourHouse, catalog)
	}
	if r.Battle != nil {
		v.Battle = r.battleView(v.YourHouse, catalog)
	}
	if r.Phase == "raids" || r.Phase == "marches" {
		v.LegalRegions = r.actionRegions(v.YourHouse)
	}
	for _, s := range r.Seats {
		v.Seats = append(v.Seats, SeatView{s.House, s.Name, s.Bot, s.Difficulty, s.Ready, s.Player == player, s.Bot || s.Player != ""})
	}
	for region, id := range r.Orders {
		owner := ""
		for _, u := range r.armies(catalog)[region] {
			if u.Count > 0 {
				owner = u.House
				break
			}
		}
		if r.Phase != "planning" || owner == v.YourHouse {
			v.Orders[region] = id
		}
	}
	return v
}
func randomIndex(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}
func (r *Room) runPlanningBots(c Catalog) {
	for i := range r.Seats {
		s := &r.Seats[i]
		if !s.Bot {
			continue
		}
		var regions []string
		for id, units := range r.armies(c) {
			for _, u := range units {
				if u.House == s.House && u.Count > 0 {
					regions = append(regions, id)
					break
				}
			}
		}
		sort.Strings(regions)
		for _, region := range regions {
			var legal []Order
			for _, o := range Orders {
				copy := r.Clone()
				cs := &copy.Seats[i]
				if copy.placeOrder(cs, region, o.ID, c) == nil {
					legal = append(legal, o)
				}
			}
			if len(legal) == 0 {
				continue
			}
			choice := legal[randomIndex(len(legal))]
			if s.Difficulty != "easy" {
				context := c
				context.Units = r.armies(c)
				sort.SliceStable(legal, func(a, b int) bool {
					return planningScore(legal[a], region, s.House, context, s.Difficulty) > planningScore(legal[b], region, s.House, context, s.Difficulty)
				})
				choice = legal[0]
			}
			r.Orders[region] = choice.ID
		}
		s.Ready = true
	}
	r.revealIfReady()
}
func planningScore(o Order, region, house string, c Catalog, difficulty string) int {
	score := 0
	var def Region
	for _, r := range c.Boards["westeros"].Regions {
		if r.ID == region {
			def = r
			break
		}
	}
	if o.Kind == "power" && def.Type == "land" {
		score = 5 + def.Crowns
	}
	if o.Kind == "support" {
		score = 3
	}
	if o.Kind == "march" {
		score = 7 + o.Modifier
	}
	if difficulty == "hard" {
		for _, n := range def.Neighbors {
			for _, u := range c.Units[n] {
				if u.Count > 0 && u.House != house {
					if o.Kind == "defend" {
						score += 10 + o.Modifier
					}
					if o.Kind == "support" {
						score += 2
					}
				}
			}
		}
	}
	return score
}
