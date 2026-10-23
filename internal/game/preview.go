package game

import (
	"errors"
	"time"
)

type ActionPreview struct {
	Revision int64    `json:"revision"`
	Region   string   `json:"region"`
	Targets  []string `json:"targets"`
}

// PreviewAction uses the command validator on disposable snapshots, never the live
// room. It includes supply, ports, fleet transport, selected troops and garrisons.
// Only public action phases are supported; no secret-order probing is possible.
func (r *Room) PreviewAction(player string, command Command, catalog Catalog, now time.Time) (ActionPreview, error) {
	result := ActionPreview{Revision: r.Revision, Region: command.Region, Targets: []string{}}
	seat := r.Seat(player)
	if seat == nil {
		return result, errors.New("Нет доступа к комнате")
	}
	phase := "marches"
	if command.Type == "raid" {
		phase = "raids"
	} else if command.Type != "march" {
		return result, errors.New("Предпросмотр доступен только для похода или набега")
	}
	if err := r.checkTurn(seat.House, phase); err != nil {
		return result, err
	}
	if !contains(r.actionRegions(seat.House), command.Region) {
		return result, errors.New("Выберите собственный активный приказ")
	}
	for _, target := range catalog.Boards["westeros"].Regions {
		if target.ID == command.Region {
			continue
		}
		candidate := command
		candidate.Target = target.ID
		if err := r.Clone().ApplyAt(player, candidate, catalog, now); err == nil {
			result.Targets = append(result.Targets, target.ID)
		}
	}
	return result, nil
}
