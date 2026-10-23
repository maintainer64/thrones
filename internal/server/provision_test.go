package server

import (
	"encoding/json"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestSupplyPersistsAndSchedulerDisbandsWithoutBrowser(t *testing.T) {
	s, r, _ := influenceServerFixture(t)
	r.Influence = nil
	r.Phase = game.PhaseWesteros
	r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "footman", House: "stark", Count: 5}}}
	r.Match.Control = map[string]string{"winterfell": "stark"}
	r.Seat(r.Host).Bot = true
	if err := r.BeginSupply(s.Catalog, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.AdvanceBots(time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := s.Store.Load(r.ID)
	if err != nil || saved.Phase != game.PhaseWesteros || saved.Match.Armies["winterfell"][0].Count != 3 || saved.Revision != r.Revision+2 {
		t.Fatal("disband scheduler did not persist decisions", err)
	}
}
func TestMusterHTTPRejectsForeignAndStaleCommands(t *testing.T) {
	s, r, cookies := influenceServerFixture(t)
	r.Influence = nil
	r.Phase = game.PhaseWesteros
	r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "footman", House: "stark", Count: 2}}}
	r.Match.Control = map[string]string{"winterfell": "stark"}
	r.Match.Supply["stark"] = 6
	if err := r.BeginMustering(s.Catalog, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(game.Command{Type: "muster", Revision: r.Revision, Region: "winterfell", Target: "winterfell", Unit: game.UnitKnight, Upgrade: true})
	if w := influenceRequest(s, r.ID, cookies["greyjoy"], string(body)); w.Code != 400 {
		t.Fatal("foreign muster accepted")
	}
	if w := influenceRequest(s, r.ID, cookies["stark"], string(body)); w.Code != 200 {
		t.Fatal("muster failed", w.Code, w.Body.String())
	}
	if w := influenceRequest(s, r.ID, cookies["stark"], string(body)); w.Code != 409 {
		t.Fatal("stale muster accepted")
	}
	saved, err := s.Store.Load(r.ID)
	if err != nil || saved.Mustering.Points["winterfell"] != 1 || saved.Revision != r.Revision+1 {
		t.Fatal("mustering failed persistence", err)
	}
}
