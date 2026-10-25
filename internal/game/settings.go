package game

import (
	"errors"
	"time"
)

type Settings struct {
	TurnMinutes   int    `json:"turnMinutes"`
	HardTimer     bool   `json:"hardTimer"`
	Scenario      string `json:"scenario"`
	TidesOfBattle bool   `json:"tidesOfBattle"`
	PlayerCount   int    `json:"playerCount"`
}

func DefaultSettings() Settings { return Settings{TurnMinutes: 5, Scenario: "base", PlayerCount: 6} }
func (s Settings) Validate() error {
	if s.houseCount() < 3 || s.houseCount() > 6 {
		return errors.New("Число домов должно быть от 3 до 6 (люди и боты вместе)")
	}
	if s.TurnMinutes < 1 || s.TurnMinutes > 120 {
		return errors.New("Лимит хода должен быть от 1 до 120 минут")
	}
	if s.Scenario != "base" {
		return errors.New("Дополнение ещё не реализовано: доступен только базовый тактический прототип")
	}
	return nil
}

type Clock struct {
	House     string    `json:"house"`
	StartedAt time.Time `json:"startedAt"`
	Deadline  time.Time `json:"deadline"`
}

func (r *Room) startClock(house string, now time.Time) {
	minutes := r.Settings.TurnMinutes
	if minutes < 1 {
		minutes = 5
	}
	r.Clock = Clock{House: house, StartedAt: now, Deadline: now.Add(time.Duration(minutes) * time.Minute)}
}

// Expire is called by the server, including when no browsers are connected.
// Soft limits never mutate the game or replace a player's decision.
func (r *Room) Expire(now time.Time, c Catalog) bool {
	if !r.Settings.HardTimer || r.Clock.Deadline.IsZero() || now.Before(r.Clock.Deadline) {
		return false
	}
	if r.Phase == PhaseRaven {
		if !r.validRaven() {
			return false
		}
		house := r.Raven.House
		if !r.advanceRaven(c, now, true) {
			return false
		}
		r.event("timeout", house, "", "", HouseNames[house]+": ход завершён по жёсткому таймеру")
		return true
	}
	if r.Phase == PhaseWesteros {
		return r.advanceWesterosChoice(c, now, true)
	}
	if r.Phase == PhaseWildlingBidding || r.Phase == PhaseWildlingTies || r.Phase == PhaseWildlingEffects {
		return r.expireWildlings(c, now)
	}
	if r.Influence != nil && (r.Phase == PhaseInfluenceBidding || r.Phase == PhaseInfluenceTies) {
		return r.expireInfluence(now)
	}
	if r.Phase == PhaseSupply || r.Phase == PhaseMustering || r.Phase == PhasePower {
		house := r.Match.ActiveHouse
		changed := r.advanceProvisionBot(c, now, true)
		for r.Phase == PhaseSupply && r.Match.ActiveHouse == house && changed {
			if !r.advanceProvisionBot(c, now, true) {
				break
			}
		}
		if changed {
			r.event("timeout", house, "", "", HouseNames[house]+": ход завершён по жёсткому таймеру")
		}
		return changed
	}
	if r.Phase == "planning" {
		for i := range r.Seats {
			s := &r.Seats[i]
			if s.Ready {
				continue
			}
			r.fillOrders(s, c)
			s.Ready = true
			r.event("timeout", s.House, "", "", HouseNames[s.House]+": время планирования истекло; назначены недостающие приказы")
		}
		r.revealIfReady()
		return true
	}
	house := r.Clock.House
	if house == "" {
		return false
	}
	if err := r.safeAction(house, c, now); err != nil {
		return false
	}
	r.event("timeout", house, "", "", HouseNames[house]+": ход завершён по жёсткому таймеру")
	return true
}
