package game

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

type WildlingCardID string

const (
	WildlingSilence     WildlingCardID = "silence-at-the-wall"
	WildlingPreemptive  WildlingCardID = "preemptive-raid"
	WildlingCrowKillers WildlingCardID = "crow-killers"
	WildlingRaiders     WildlingCardID = "rattleshirts-raiders"
	WildlingMilkwater   WildlingCardID = "massing-on-the-milkwater"
	WildlingKing        WildlingCardID = "a-king-beyond-the-wall"
	WildlingMammoths    WildlingCardID = "mammoth-riders"
	WildlingHorde       WildlingCardID = "the-horde-descends"
	WildlingSkinchanger WildlingCardID = "skinchanger-scout"
)

const (
	PhaseWildlingBidding = "wildling_bidding"
	PhaseWildlingTies    = "wildling_ties"
	PhaseWildlingEffects = "wildling_effects"
)

// Deck and committed bids are private Room state. The next card stays in Deck
// through bidding AND tie selection, and is drawn only after the key bidder is
// known. Raven access, when implemented, must use a separate personal projection.
type WildlingState struct {
	Deck   []WildlingCardID `json:"deck"`
	Attack *WildlingAttack  `json:"attack,omitempty"`
	Last   *WildlingResult  `json:"last,omitempty"`
}

type WildlingAttack struct {
	Strength      int             `json:"strength"`
	Bids          SealedBids      `json:"bids"`
	TieBreakHouse string          `json:"tieBreakHouse"`
	Tied          []string        `json:"tied"`
	Total         int             `json:"total"`
	WatchWon      bool            `json:"watchWon"`
	SelectedHouse string          `json:"selectedHouse,omitempty"`
	Card          WildlingCardID  `json:"card,omitempty"`
	Effect        *WildlingEffect `json:"effect,omitempty"`
	ExcludedHouse string          `json:"excludedHouse,omitempty"`
}

type WildlingOutcome struct {
	Total         int    `json:"total"`
	WatchWon      bool   `json:"watchWon"`
	SelectedHouse string `json:"selectedHouse,omitempty"`
}

type WildlingResult struct {
	Strength      int             `json:"strength"`
	Outcome       WildlingOutcome `json:"outcome"`
	Card          WildlingCardID  `json:"card"`
	Bids          map[string]int  `json:"bids"`
	ThreatAfter   int             `json:"threatAfter"`
	ExcludedHouse string          `json:"excludedHouse,omitempty"`
}

type WildlingAttackView struct {
	Strength           int              `json:"strength"`
	Bids               BidsView         `json:"bids"`
	TieBreakHouse      string           `json:"tieBreakHouse"`
	Tied               []string         `json:"tied"`
	Outcome            *WildlingOutcome `json:"outcome,omitempty"`
	Card               WildlingCardID   `json:"card,omitempty"`
	EffectsImplemented bool             `json:"effectsImplemented"`
	Choice             *WildlingChoice  `json:"choice,omitempty"`
	ExcludedHouse      string           `json:"excludedHouse,omitempty"`
}

type WildlingView struct {
	Remaining int                 `json:"remaining"`
	Attack    *WildlingAttackView `json:"attack,omitempty"`
	Last      *WildlingResult     `json:"last,omitempty"`
}

func baseWildlingCards() []WildlingCardID {
	return []WildlingCardID{WildlingSilence, WildlingPreemptive, WildlingCrowKillers,
		WildlingRaiders, WildlingMilkwater, WildlingKing, WildlingMammoths, WildlingHorde, WildlingSkinchanger}
}

func newWildlingState() *WildlingState {
	deck := baseWildlingCards()
	for i := len(deck) - 1; i > 0; i-- {
		j := randomIndex(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
	return &WildlingState{Deck: deck}
}

func validWildlingDeck(deck []WildlingCardID) bool {
	expected := baseWildlingCards()
	actual := slices.Clone(deck)
	slices.Sort(expected)
	slices.Sort(actual)
	return slices.Equal(expected, actual)
}

func (r *Room) pendingWildlingContext() bool {
	return r.Match != nil && r.Battle == nil && r.Influence == nil && r.Supply == nil && r.Mustering == nil &&
		r.Westeros != nil && r.Westeros.Run != nil && r.Westeros.Run.Round == r.Round &&
		r.Westeros.Run.Stage == WesterosAttackStep && r.Westeros.Run.Attack != nil
}

// Internal handoff from the real Westeros queue, never a player/host command.
func (r *Room) beginWildlingAttack(now time.Time) error {
	if r.Phase != PhaseWesteros || !r.pendingWildlingContext() || (r.Wildlings != nil && r.Wildlings.Attack != nil) {
		return errors.New("Нападение одичалых доступно только по событию Вестероса")
	}
	strength := r.Westeros.Run.Attack.Strength
	if strength < 0 || strength > 12 || strength%2 != 0 || r.Westeros.Threat < 0 || r.Westeros.Threat > 12 || r.Westeros.Threat%2 != 0 {
		return errors.New("Некорректная угроза одичалых")
	}
	if err := r.validateInfluenceTracks(); err != nil {
		return err
	}
	state := r.Wildlings
	if state == nil {
		state = newWildlingState()
	}
	if !validWildlingDeck(state.Deck) {
		return errors.New("Некорректный состав колоды одичалых")
	}
	r.Wildlings = state
	r.startWildlingBidding(strength, r.Match.Tracks[string(TrackIronThrone)], "", now)
	return nil
}

func (r *Room) startWildlingBidding(strength int, participants []string, excluded string, now time.Time) {
	r.Wildlings.Attack = &WildlingAttack{Strength: strength, Bids: newSealedBids(participants),
		TieBreakHouse: r.Match.Tracks[string(TrackIronThrone)][0], Tied: []string{}, ExcludedHouse: excluded}
	r.Phase, r.Match.ActiveHouse = PhaseWildlingBidding, ""
	r.startClock("", now)
	r.event("wildling-bidding", "", "", "", fmt.Sprintf("Ночной дозор: начат тайный сбор власти против силы %d", strength))
}

func (r *Room) submitWildlingBid(house string, amount *int, now time.Time) error {
	if r.Phase != PhaseWildlingBidding || r.Wildlings == nil || r.Wildlings.Attack == nil {
		return errors.New("Сейчас нет тайного сбора власти в Ночной дозор")
	}
	if amount == nil {
		return errors.New("Укажите ставку, включая нулевую")
	}
	a := r.Wildlings.Attack
	if err := a.Bids.submit(house, *amount, r.Match.Power[house]); err != nil {
		return err
	}
	r.event("wildling-bid", house, "", "", HouseNames[house]+": взнос в Ночной дозор подтверждён")
	if !a.Bids.complete() {
		return nil
	}
	for _, participant := range a.Bids.Participants {
		bid := a.Bids.Values[participant]
		a.Total += bid
		r.Match.Power[participant] -= bid
		r.event("wildling-bid-reveal", participant, "", "", fmt.Sprintf("%s: раскрыт взнос в Ночной дозор — %d", HouseNames[participant], bid))
	}
	a.WatchWon = a.Total >= a.Strength
	// Only the highest OR lowest tie matters, depending on the actual outcome.
	best := a.Bids.Values[a.Bids.Participants[0]]
	for _, participant := range a.Bids.Participants {
		bid := a.Bids.Values[participant]
		better := (a.WatchWon && bid > best) || (!a.WatchWon && bid < best)
		if better {
			best, a.Tied = bid, []string{}
		}
		if bid == best {
			a.Tied = append(a.Tied, participant)
		}
	}
	if len(a.Tied) == 1 {
		r.revealWildlingCard(a.Tied[0])
	} else {
		r.Phase, r.Match.ActiveHouse = PhaseWildlingTies, a.TieBreakHouse
		r.startClock(a.TieBreakHouse, now)
	}
	return nil
}

func (r *Room) chooseWildlingTie(house, target string) error {
	if r.Phase != PhaseWildlingTies || r.Wildlings == nil || r.Wildlings.Attack == nil || r.Wildlings.Attack.TieBreakHouse != house {
		return errors.New("Порядок при равных ставках определяет владелец Железного трона")
	}
	if !slices.Contains(r.Wildlings.Attack.Tied, target) {
		return errors.New("Выберите дом из текущей группы равных ставок")
	}
	r.revealWildlingCard(target)
	return nil
}

func (r *Room) revealWildlingCard(house string) {
	s, a := r.Wildlings, r.Wildlings.Attack
	a.SelectedHouse, a.Card, a.Tied = house, s.Deck[0], []string{}
	s.Deck = slices.Clone(s.Deck[1:])
	r.Phase, r.Match.ActiveHouse, r.Clock = PhaseWildlingEffects, "", Clock{}
	r.event("wildling-card", house, "", "", "Открыта карта одичалых: "+string(a.Card))
}

// Reject unknown cards even though all nine base effects are implemented.
func wildlingEffectsImplemented(card WildlingCardID) bool {
	return slices.Contains(baseWildlingCards(), card)
}

func (r *Room) validWildlingEffects() bool {
	if r.Phase != PhaseWildlingEffects || r.Wildlings == nil || r.Wildlings.Attack == nil || !r.pendingWildlingContext() {
		return false
	}
	s, a := r.Wildlings, r.Wildlings.Attack
	if !wildlingEffectsImplemented(a.Card) || !a.Bids.complete() || !slices.Contains(a.Bids.Participants, a.SelectedHouse) {
		return false
	}
	if !validWildlingDeck(append(slices.Clone(s.Deck), a.Card)) {
		return false
	}
	return true
}

// Effects call this only after every penalty and nested decision has completed.
func (r *Room) finishWildlingAttack() bool {
	s, a := r.Wildlings, r.Wildlings.Attack
	r.Phase = PhaseWesteros
	if err := r.resumeWesterosAfterWildlings(a.WatchWon); err != nil {
		r.Phase = PhaseWildlingEffects
		return false
	}
	r.recordWildlingResult()
	s.Attack = nil
	r.Match.ActiveHouse, r.Clock = "", Clock{}
	r.event("wildling-complete", "", "", "", "Нападение одичалых завершено")
	return true
}

func (r *Room) recordWildlingResult() {
	s, a := r.Wildlings, r.Wildlings.Attack
	s.Deck = append(s.Deck, a.Card) // Used card goes to bottom, without reshuffling.
	s.Last = &WildlingResult{Strength: a.Strength, Outcome: WildlingOutcome{Total: a.Total, WatchWon: a.WatchWon, SelectedHouse: a.SelectedHouse},
		Card: a.Card, Bids: copyBids(a.Bids.Values), ThreatAfter: r.Westeros.Threat, ExcludedHouse: a.ExcludedHouse}
}

func (s *WildlingState) view(house string) *WildlingView {
	if s == nil {
		return nil
	}
	v := &WildlingView{Remaining: len(s.Deck)}
	if s.Last != nil {
		last := *s.Last
		last.Bids = copyBids(last.Bids)
		v.Last = &last
	}
	if a := s.Attack; a != nil {
		v.Attack = &WildlingAttackView{Strength: a.Strength, Bids: a.Bids.view(house), TieBreakHouse: a.TieBreakHouse, Tied: slices.Clone(a.Tied), ExcludedHouse: a.ExcludedHouse}
		if a.Bids.complete() {
			v.Attack.Outcome = &WildlingOutcome{Total: a.Total, WatchWon: a.WatchWon, SelectedHouse: a.SelectedHouse}
		}
		if a.Card != "" {
			v.Attack.Card, v.Attack.EffectsImplemented = a.Card, wildlingEffectsImplemented(a.Card)
			if a.Effect != nil && a.Effect.Choice != nil {
				choice := *a.Effect.Choice
				choice.Options = slices.Clone(choice.Options)
				choice.UnitOptions = slices.Clone(choice.UnitOptions)
				v.Attack.Choice = &choice
			}
		}
	}
	return v
}

func (r *Room) advanceWildlingBot(c Catalog, now time.Time) bool {
	if r.Wildlings == nil || r.Wildlings.Attack == nil {
		return false
	}
	if r.Phase == PhaseWildlingEffects {
		if r.validWildlingEffects() && r.Wildlings.Attack.Effect != nil && r.Wildlings.Attack.Effect.Choice != nil {
			choice := r.Wildlings.Attack.Effect.Choice
			seat := r.houseSeat(choice.House)
			if seat == nil || !seat.Bot {
				return false
			}
			return r.chooseWildlingEffect(choice.House, r.wildlingBotOption(seat, c), c) == nil
		}
		return r.advanceWildlingEffects(c, now)
	}
	a := r.Wildlings.Attack
	if r.Phase == PhaseWildlingTies {
		seat := r.houseSeat(a.TieBreakHouse)
		if seat == nil || !seat.Bot {
			return false
		}
		target := a.Tied[0]
		if seat.Difficulty == "easy" {
			target = a.Tied[randomIndex(len(a.Tied))]
		} else if a.WatchWon && slices.Contains(a.Tied, seat.House) {
			target = seat.House
		} else if !a.WatchWon && target == seat.House && len(a.Tied) > 1 {
			target = a.Tied[1]
		}
		return r.chooseWildlingTie(seat.House, target) == nil
	}
	if r.Phase != PhaseWildlingBidding {
		return false
	}
	for _, house := range a.Bids.Participants {
		seat := r.houseSeat(house)
		if seat == nil || !seat.Bot || a.Bids.view(house).Submitted[house] {
			continue
		}
		power := r.Match.Power[house]
		amount := min(power, (a.Strength+len(a.Bids.Participants)-1)/len(a.Bids.Participants))
		if seat.Difficulty == "easy" {
			amount = randomIndex(power + 1)
		} else if seat.Difficulty == "hard" && a.Strength > 0 {
			amount = min(power, amount+1)
		}
		return r.submitWildlingBid(house, &amount, now) == nil
	}
	return false
}

func (r *Room) expireWildlings(c Catalog, now time.Time) bool {
	if r.Wildlings == nil || r.Wildlings.Attack == nil {
		return false
	}
	a := r.Wildlings.Attack
	if r.Phase == PhaseWildlingEffects && a.Effect != nil && a.Effect.Choice != nil && len(a.Effect.Choice.Options) > 0 {
		return r.chooseWildlingEffect(a.Effect.Choice.House, a.Effect.Choice.Options[0], c) == nil
	}
	if r.Phase == PhaseWildlingTies {
		return r.chooseWildlingTie(a.TieBreakHouse, a.Tied[0]) == nil
	}
	if r.Phase != PhaseWildlingBidding {
		return false
	}
	pending := []string{}
	for _, house := range a.Bids.Participants {
		if _, submitted := a.Bids.Values[house]; !submitted {
			pending = append(pending, house)
		}
	}
	for _, house := range pending {
		zero := 0
		if err := r.submitWildlingBid(house, &zero, now); err != nil {
			return false
		}
	}
	return len(pending) > 0
}
