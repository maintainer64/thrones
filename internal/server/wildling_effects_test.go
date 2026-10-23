package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"westeros-web/internal/game"
)

func TestWildlingTrackChoiceHTTPAuthorizationRevisionAndWebSocket(t *testing.T) {
	s, r, cookies, now := wildlingServerFixture(t, game.WildlingKing)
	r.Match.BladeUsed = true
	for _, seat := range r.Seats {
		bid := 0
		if seat.House == "stark" {
			bid = 1
		}
		if err := r.ApplyAt(seat.Player, game.Command{Type: "wildling_bid", Bid: &bid}, s.Catalog, now); err != nil {
			t.Fatal(err)
		}
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	r, err := s.Store.Load(r.ID)
	if err != nil || r.Wildlings.Attack.Effect.Choice == nil {
		t.Fatal("scheduler did not persist the new decision", err)
	}
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	guest := cookies["greyjoy"]
	url := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rooms/" + r.ID + "/ws?lang=en"
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Cookie": {guest.Name + "=" + guest.Value}, "Origin": {httpServer.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	view := checkWildlingWire(t, raw, true)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if view.Wildlings.Attack.Choice == nil || view.Wildlings.Attack.Choice.House != "stark" || strings.Contains(string(fields["wildlings"]), `"queue"`) || strings.Contains(string(fields["wildlings"]), `"effect"`) {
		t.Fatal("pending choice missing or private resolver state leaked")
	}
	body := fmt.Sprintf(`{"type":"wildling_choice","house":"stark","choice":"fiefdoms","revision":%d}`, r.Revision)
	if w := influenceRequest(s, r.ID, guest, body); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "There is no wildling card decision") {
		t.Fatal("foreign choice accepted or error not localized", w.Code, w.Body.String())
	}
	if w := influenceRequest(s, r.ID, cookies["stark"], fmt.Sprintf(`{"type":"wildling_choice","choice":"unknown","revision":%d}`, r.Revision)); w.Code != http.StatusBadRequest {
		t.Fatal("arbitrary track accepted")
	}
	unchanged, err := s.Store.Load(r.ID)
	if err != nil || !reflect.DeepEqual(r, unchanged) {
		t.Fatal("rejected command changed the saved decision", err)
	}
	// The body cannot nominate a different house: the player's seat is authoritative.
	body = fmt.Sprintf(`{"type":"wildling_choice","house":"greyjoy","choice":"fiefdoms","revision":%d}`, r.Revision)
	w := influenceRequest(s, r.ID, cookies["stark"], body)
	if w.Code != http.StatusOK {
		t.Fatal("valid choice failed", w.Code, w.Body.String())
	}
	_, raw, err = conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	view = checkWildlingWire(t, raw, true)
	if view.Match.Tracks["fiefdoms"][0] != "stark" || !view.Match.BladeUsed || view.Wildlings.Attack.Choice != nil || view.Revision != r.Revision+1 {
		t.Fatal("track move/used token/choice was not broadcast correctly")
	}
	if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusConflict {
		t.Fatal("stale choice accepted")
	}
	body = fmt.Sprintf(`{"type":"wildling_choice","choice":"kingsCourt","revision":%d}`, view.Revision)
	if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusBadRequest {
		t.Fatal("completed choice could be changed with a fresh revision")
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	_, raw, err = conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	view = checkWildlingWire(t, raw, true)
	if view.Wildlings.Attack != nil || view.Wildlings.Last == nil || view.Wildlings.Last.Card != game.WildlingKing || view.Phase != game.PhaseWesteros {
		t.Fatal("completed attack was not persisted and broadcast")
	}
}

func TestWildlingRaidersNestedSupplyHTTPAndHeadlessContinuation(t *testing.T) {
	s, r, cookies, now := wildlingServerFixture(t, game.WildlingRaiders)
	// Keep the fixture's actual attack boundary, but exercise a strength-12 loss.
	r.Wildlings.Attack.Strength, r.Westeros.Run.Attack.Strength, r.Westeros.Threat = 12, 12, 12
	r.Settings.HardTimer = true
	for _, seat := range r.Seats {
		r.Match.Supply[seat.House] = 1
		bid := 1
		if seat.House == "stark" {
			bid = 0
		}
		if err := r.ApplyAt(seat.Player, game.Command{Type: "wildling_bid", Bid: &bid}, s.Catalog, now); err != nil {
			t.Fatal(err)
		}
	}
	r.Match.Supply["stark"] = 2
	r.Match.Armies = map[string][]game.Unit{
		"winterfell": {{Type: "footman", House: "stark", Count: 3}},
		"pyke":       {{Type: "footman", House: "greyjoy", Count: 3}},
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	r, err := s.Store.Load(r.ID)
	if err != nil || r.Phase != game.PhaseSupply || r.Supply.Source != game.SupplyFromWildlings || r.Match.Supply["stark"] != 0 {
		t.Fatal("nested disband was not persisted", err)
	}
	body := fmt.Sprintf(`{"type":"disband","house":"stark","region":"winterfell","units":{"footman":1},"revision":%d}`, r.Revision)
	if w := influenceRequest(s, r.ID, cookies["greyjoy"], body); w.Code != http.StatusBadRequest {
		t.Fatal("another house could disband the lowest bidder's troops")
	}
	w := influenceRequest(s, r.ID, cookies["stark"], body)
	if w.Code != http.StatusOK {
		t.Fatal("lowest bidder could not resolve the existing supply API", w.Body.String())
	}
	view := checkWildlingWire(t, w.Body.Bytes(), true)
	if view.Phase != game.PhaseWildlingEffects || view.Supply != nil || view.Match.Supply["stark"] != 0 || view.Wildlings.Attack == nil {
		t.Fatal("disband recounted icons, skipped remaining penalties or lost the attack")
	}
	if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusConflict {
		t.Fatal("stale disband replayed")
	}
	for step := 0; step < 10; step++ {
		r, err = s.Store.Load(r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Wildlings.Attack == nil {
			break
		}
		if r.Phase == game.PhaseSupply {
			if err := s.Expire(r.Clock.Deadline); err != nil {
				t.Fatal(err)
			}
		} else if err := s.AdvanceBots(now); err != nil {
			t.Fatal(err)
		}
	}
	if r.Wildlings.Attack != nil || r.Wildlings.Last == nil || r.Wildlings.Last.Card != game.WildlingRaiders || r.Westeros.Threat != 8 || r.Match.Armies["pyke"][0].Count != 2 {
		t.Fatal("headless timer/scheduler failed to finish the nested penalty")
	}
}
