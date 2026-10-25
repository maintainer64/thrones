package game

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

type InfluenceTrack string

const (
	TrackIronThrone InfluenceTrack = "ironThrone"
	TrackFiefdoms   InfluenceTrack = "fiefdoms"
	TrackKingsCourt InfluenceTrack = "kingsCourt"
)

type AuctionStage string

const (
	AuctionBidding AuctionStage = "bidding"
	AuctionTies    AuctionStage = "ties"
)

const (
	PhaseWesteros         = "westeros"
	PhaseInfluenceBidding = "influence_bidding"
	PhaseInfluenceTies    = "influence_ties"
)

var influenceTrackOrder = []InfluenceTrack{TrackIronThrone, TrackFiefdoms, TrackKingsCourt}

type InfluenceResult struct {
	Track   InfluenceTrack `json:"track"`
	Bids    map[string]int `json:"bids"`
	Ranking []string       `json:"ranking"`
}

// InfluenceAuction is saved with Room, never returned directly to clients.
// TieBreakHouse is captured at the start of each track. In particular, the OLD
// throne holder resolves every tie in the throne auction, even after losing #1.
type InfluenceAuction struct {
	Track         InfluenceTrack    `json:"track"`
	Stage         AuctionStage      `json:"stage"`
	Bids          SealedBids        `json:"bids"`
	TieBreakHouse string            `json:"tieBreakHouse"`
	Ranking       []string          `json:"ranking"`
	Tied          []string          `json:"tied"`
	Results       []InfluenceResult `json:"results"`
}

type InfluenceView struct {
	Track         InfluenceTrack    `json:"track"`
	Stage         AuctionStage      `json:"stage"`
	Bids          BidsView          `json:"bids"`
	TieBreakHouse string            `json:"tieBreakHouse"`
	Ranking       []string          `json:"ranking"`
	Tied          []string          `json:"tied"`
	Results       []InfluenceResult `json:"results"`
}

// BeginClashOfKings is a rule-effect hook, deliberately NOT a host/debug command.
// The Westeros card resolver calls it; there is no extra auction each round.
// Completion returns to Westeros so the remaining revealed cards can resolve.
func (r *Room) BeginClashOfKings(now time.Time) error {
	if r.Phase != PhaseWesteros || r.Match == nil || r.Influence != nil || r.Battle != nil {
		return errors.New("Торги за влияние доступны только по событию Вестероса")
	}
	if err := r.validateInfluenceTracks(); err != nil {
		return err
	}
	r.Influence = &InfluenceAuction{}
	r.startInfluenceTrack(TrackIronThrone, now)
	return nil
}

// Validate before mutating: every track must contain exactly the seated houses.
func (r *Room) validateInfluenceTracks() error {
	for _, track := range influenceTrackOrder {
		houses := r.Match.Tracks[string(track)]
		seen := map[string]bool{}
		if len(houses) == 0 || len(houses) != len(r.Seats) {
			return errors.New("Некорректный состав треков влияния")
		}
		for _, house := range houses {
			if seen[house] || r.houseSeat(house) == nil || r.Match.Power[house] < 0 {
				return errors.New("Некорректный состав треков влияния")
			}
			seen[house] = true
		}
	}
	return nil
}

func (r *Room) startInfluenceTrack(track InfluenceTrack, now time.Time) {
	a := r.Influence
	a.Track, a.Stage = track, AuctionBidding
	a.Bids = newSealedBids(r.Match.Tracks[string(track)])
	a.TieBreakHouse = r.Match.Tracks[string(TrackIronThrone)][0]
	a.Ranking, a.Tied = []string{}, []string{}
	r.Phase, r.Match.ActiveHouse = PhaseInfluenceBidding, ""
	r.startClock("", now)
	r.event("influence", "", "", "", "Начаты тайные торги: "+influenceTrackName(track))
}

func influenceTrackName(track InfluenceTrack) string {
	switch track {
	case TrackIronThrone:
		return "Железный трон"
	case TrackFiefdoms:
		return "Вотчины"
	default:
		return "Королевский двор"
	}
}

func (r *Room) submitInfluenceBid(house string, amount *int, now time.Time) error {
	if r.Phase != PhaseInfluenceBidding || r.Influence == nil {
		return errors.New("Сейчас не время тайных ставок")
	}
	if amount == nil {
		return errors.New("Укажите ставку, включая нулевую")
	}
	a := r.Influence
	if err := a.Bids.submit(house, *amount, r.Match.Power[house]); err != nil {
		return err
	}
	r.event("bid", house, "", "", HouseNames[house]+": ставка подтверждена")
	if !a.Bids.complete() {
		return nil
	}
	// The only debit point; changing phase prevents a duplicate reveal/debit.
	for _, h := range a.Bids.Participants {
		value := a.Bids.Values[h]
		r.Match.Power[h] -= value
		r.event("bid-reveal", h, "", "", fmt.Sprintf("%s: раскрыта ставка %d", HouseNames[h], value))
	}
	a.Stage = AuctionTies
	r.Phase = PhaseInfluenceTies
	r.advanceInfluenceRanking(now)
	return nil
}

func (r *Room) advanceInfluenceRanking(now time.Time) {
	a := r.Influence
	for len(a.Ranking) < len(a.Bids.Participants) {
		best := -1
		a.Tied = []string{}
		for _, house := range a.Bids.Participants {
			if slices.Contains(a.Ranking, house) {
				continue
			}
			value := a.Bids.Values[house]
			if value > best {
				best, a.Tied = value, []string{house}
			} else if value == best {
				a.Tied = append(a.Tied, house)
			}
		}
		if len(a.Tied) > 1 {
			r.Match.ActiveHouse = a.TieBreakHouse
			r.startClock(a.TieBreakHouse, now)
			return
		}
		a.Ranking = append(a.Ranking, a.Tied[0])
	}
	r.Match.Tracks[string(a.Track)] = append([]string{}, a.Ranking...)
	a.Results = append(a.Results, InfluenceResult{Track: a.Track, Bids: copyBids(a.Bids.Values), Ranking: append([]string{}, a.Ranking...)})
	r.event("influence", "", "", "", "Завершены торги: "+influenceTrackName(a.Track))
	// Used dominance tokens stay used; ownership derives from the new tracks.
	for i, track := range influenceTrackOrder {
		if track == a.Track && i+1 < len(influenceTrackOrder) {
			r.startInfluenceTrack(influenceTrackOrder[i+1], now)
			return
		}
	}
	r.Match.LastInfluence = copyInfluenceResults(a.Results)
	r.Influence = nil
	r.Phase, r.Match.ActiveHouse, r.Clock = PhaseWesteros, "", Clock{}
}

func (r *Room) chooseInfluenceTie(house, target string, now time.Time) error {
	if r.Phase != PhaseInfluenceTies || r.Influence == nil || r.Influence.TieBreakHouse != house {
		return errors.New("Порядок при равных ставках определяет владелец Железного трона")
	}
	a := r.Influence
	if !slices.Contains(a.Tied, target) {
		return errors.New("Выберите дом из текущей группы равных ставок")
	}
	a.Ranking = append(a.Ranking, target)
	r.event("influence-tie", house, "", "", HouseNames[house]+": выбран следующий дом при равных ставках — "+HouseNames[target])
	r.advanceInfluenceRanking(now)
	return nil
}

func (r *Room) influenceView(house string) *InfluenceView {
	if r.Influence == nil {
		return nil
	}
	a := r.Influence
	return &InfluenceView{Track: a.Track, Stage: a.Stage, Bids: a.Bids.view(house), TieBreakHouse: a.TieBreakHouse,
		Ranking: append([]string{}, a.Ranking...), Tied: append([]string{}, a.Tied...), Results: copyInfluenceResults(a.Results)}
}

func copyInfluenceResults(results []InfluenceResult) []InfluenceResult {
	copy := make([]InfluenceResult, len(results))
	for i, result := range results {
		copy[i] = InfluenceResult{Track: result.Track, Bids: copyBids(result.Bids), Ranking: append([]string{}, result.Ranking...)}
	}
	return copy
}

func (r *Room) advanceInfluenceBot(now time.Time) bool {
	a := r.Influence
	if a == nil {
		return false
	}
	if r.Phase == PhaseInfluenceTies {
		seat := r.houseSeat(a.TieBreakHouse)
		if seat == nil || !seat.Bot {
			return false
		}
		target := a.Tied[0]
		if slices.Contains(a.Tied, seat.House) {
			target = seat.House
		}
		return r.chooseInfluenceTie(seat.House, target, now) == nil
	}
	for _, house := range a.Bids.Participants {
		seat := r.houseSeat(house)
		if seat == nil || !seat.Bot || a.Bids.view(house).Submitted[house] {
			continue
		}
		// Only public power/track data is used, never opponents' committed amounts.
		power := r.Match.Power[house]
		remaining := len(influenceTrackOrder) - slices.Index(influenceTrackOrder, a.Track)
		amount := power / remaining
		if seat.Difficulty == "easy" {
			amount = randomIndex(power + 1)
		} else if seat.Difficulty == "hard" && a.Track == TrackFiefdoms && power > 0 {
			amount = min(power, amount+1)
		}
		return r.submitInfluenceBid(house, &amount, now) == nil
	}
	return false
}

func (r *Room) expireInfluence(now time.Time) bool {
	if r.Phase == PhaseInfluenceTies {
		return r.chooseInfluenceTie(r.Influence.TieBreakHouse, r.Influence.Tied[0], now) == nil
	}
	// Snapshot the list: the last bid may advance to a different track. Never
	// timeout that freshly opened auction in this invocation.
	pending := []string{}
	for _, house := range r.Influence.Bids.Participants {
		if _, ok := r.Influence.Bids.Values[house]; !ok {
			pending = append(pending, house)
		}
	}
	for _, house := range pending {
		zero := 0
		if err := r.submitInfluenceBid(house, &zero, now); err != nil {
			return false
		}
	}
	return true
}
