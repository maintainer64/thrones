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

func TestMammothStructuredLossAndCardRewardHTTPWebSocket(t *testing.T) {
	for _, won := range []bool{false, true} {
		s, r, cookies, now := wildlingServerFixture(t, game.WildlingMammoths)
		if !won {
			r.Wildlings.Attack.Strength, r.Westeros.Run.Attack.Strength, r.Westeros.Threat = 12, 12, 12
		}
		r.Settings.HardTimer = true
		r.Match.Discard["stark"] = []string{"eddard-stark", "robb-stark"}
		r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "footman", House: "stark", Count: 4}, {Type: "knight", House: "stark", Count: 1}}}
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
			if strings.Contains(string(raw), `"vacated"`) || strings.Contains(string(raw), `"queue"`) || strings.Contains(string(raw), `"effect"`) {
				t.Fatal("private state leaked")
			}
			return checkWildlingWire(t, raw, true)
		}
		view := read()
		if !won && (len(view.Wildlings.Attack.Choice.UnitOptions) != 2 || view.Wildlings.Attack.Choice.Count != 3) {
			t.Fatal("structured unit options missing")
		}
		option := "winterfell:footman"
		if won {
			option = "eddard-stark"
		}
		body := fmt.Sprintf(`{"type":"wildling_choice","house":"stark","choice":%q,"revision":%d}`, option, r.Revision)
		if w := influenceRequest(s, r.ID, guest, body); w.Code != http.StatusBadRequest {
			t.Fatal("foreign choice accepted")
		}
		bad := fmt.Sprintf(`{"type":"wildling_choice","choice":"unknown","revision":%d}`, r.Revision)
		if w := influenceRequest(s, r.ID, cookies["stark"], bad); w.Code != http.StatusBadRequest {
			t.Fatal("invalid target accepted")
		}
		unchanged, err := s.Store.Load(r.ID)
		if err != nil || !reflect.DeepEqual(r, unchanged) {
			t.Fatal("rejected command changed state", err)
		}
		body = fmt.Sprintf(`{"type":"wildling_choice","house":"greyjoy","choice":%q,"revision":%d}`, option, r.Revision)
		if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusOK {
			t.Fatal("valid decision failed", w.Body.String())
		}
		view = read()
		if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusConflict {
			t.Fatal("stale decision replayed")
		}
		for step := 0; step < 24 && view.Wildlings.Attack != nil; step++ {
			if view.Wildlings.Attack.Choice == nil {
				if err := s.AdvanceBots(now); err != nil {
					t.Fatal(err)
				}
			} else {
				now = view.Clock.Deadline
				if err := s.Expire(now); err != nil {
					t.Fatal(err)
				}
			}
			view = read()
		}
		if view.Wildlings.Attack != nil || view.Wildlings.Last == nil || view.Wildlings.Last.Card != game.WildlingMammoths {
			t.Fatal("headless continuation failed")
		}
		if won {
			if !reflect.DeepEqual(view.Match.Discard["stark"], []string{"robb-stark"}) {
				t.Fatal("retrieved more than one card")
			}
		} else {
			total := 0
			for _, unit := range view.Match.Armies["winterfell"] {
				total += unit.Count
			}
			if total != 2 {
				t.Fatal("wrong casualty quota")
			}
		}
		conn.Close()
		httpServer.Close()
	}
}
