package server

import (
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

func TestCrowKillersHTTPWebSocketOptionalRewardAndHeadlessLoss(t *testing.T) {
	for _, won := range []bool{true, false} {
		s, r, cookies, now := wildlingServerFixture(t, game.WildlingCrowKillers)
		knights, footmen := 0, 3
		if !won {
			r.Wildlings.Attack.Strength, r.Westeros.Run.Attack.Strength, r.Westeros.Threat = 12, 12, 12
			knights, footmen = 3, 9
		}
		r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "knight", House: "stark", Count: knights}, {Type: "footman", House: "stark", Count: footmen}}}
		r.Settings.HardTimer = true
		for _, seat := range r.Seats {
			bid := 1
			if seat.House == "stark" {
				bid = 0
				if won {
					bid = 7
				}
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
		if err != nil {
			t.Fatal(err)
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
		read := func() game.View {
			t.Helper()
			_, raw, err := conn.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), `"destroy":`) || strings.Contains(string(raw), `"queue":`) || strings.Contains(string(raw), `"remaining":`) && strings.Contains(string(raw), `"effect":`) {
				t.Fatal("internal unit continuation leaked over WS")
			}
			return checkWildlingWire(t, raw, true)
		}
		view := read()
		if view.Wildlings.Attack.Choice == nil || view.Wildlings.Attack.Choice.Count != 2 {
			t.Fatal("unit decision missing from shared projection")
		}
		body := fmt.Sprintf(`{"type":"wildling_choice","house":"stark","choice":"winterfell","revision":%d}`, r.Revision)
		if w := influenceRequest(s, r.ID, guest, body); w.Code != http.StatusBadRequest {
			t.Fatal("another house changed Stark's units")
		}
		bad := fmt.Sprintf(`{"type":"wildling_choice","choice":"pyke","revision":%d}`, r.Revision)
		if w := influenceRequest(s, r.ID, cookies["stark"], bad); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Choose an eligible unit") {
			t.Fatal("invalid region accepted or error untranslated", w.Body.String())
		}
		unchanged, err := s.Store.Load(r.ID)
		if err != nil || !reflect.DeepEqual(r, unchanged) {
			t.Fatal("rejected command changed units", err)
		}
		body = fmt.Sprintf(`{"type":"wildling_choice","house":"greyjoy","choice":"winterfell","revision":%d}`, r.Revision)
		if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusOK {
			t.Fatal("valid unit decision failed", w.Body.String())
		}
		view = read()
		if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusConflict {
			t.Fatal("stale request repeated a loss or upgrade")
		}
		for step := 0; step < 24 && view.Wildlings.Attack != nil; step++ {
			if view.Wildlings.Attack.Choice == nil {
				if err := s.AdvanceBots(now); err != nil {
					t.Fatal(err)
				}
			} else if won {
				body = fmt.Sprintf(`{"type":"wildling_choice","choice":"finish","revision":%d}`, view.Revision)
				if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusOK {
					t.Fatal("could not decline the second upgrade", w.Body.String())
				}
			} else {
				now = view.Clock.Deadline
				if err := s.Expire(now); err != nil {
					t.Fatal(err)
				}
			}
			view = read()
		}
		if view.Wildlings.Attack != nil || view.Wildlings.Last == nil || view.Wildlings.Last.Card != game.WildlingCrowKillers {
			t.Fatal("headless unit resolution failed to finish")
		}
		gotKnights, gotFootmen := 0, 0
		for _, unit := range view.Match.Armies["winterfell"] {
			if unit.Type == "knight" {
				gotKnights += unit.Count
			} else if unit.Type == "footman" {
				gotFootmen += unit.Count
			}
		}
		if (won && (gotKnights != 1 || gotFootmen != 2)) || (!won && (gotKnights != 0 || gotFootmen != 10)) {
			t.Fatal("HTTP/WS state has duplicated effects or missing replacements")
		}
		conn.Close()
		httpServer.Close()
	}
}
