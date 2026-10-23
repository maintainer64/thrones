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

func TestHordeLossAndNestedMusterHTTPWebSocket(t *testing.T) {
	for _, won := range []bool{false, true} {
		t.Run(fmt.Sprint(won), func(t *testing.T) {
			s, r, cookies, now := wildlingServerFixture(t, game.WildlingHorde)
			if !won {
				r.Wildlings.Attack.Strength, r.Westeros.Run.Attack.Strength, r.Westeros.Threat = 12, 12, 12
			}
			r.Match.Armies = map[string][]game.Unit{"winterfell": {{Type: "footman", House: "stark", Count: 2}}}
			r.Match.Control = map[string]string{"winterfell": "stark"}
			r.Match.Supply["stark"] = 6
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
			hs := httptest.NewServer(s)
			defer hs.Close()
			guest := cookies["greyjoy"]
			url := "ws" + strings.TrimPrefix(hs.URL, "http") + "/api/rooms/" + r.ID + "/ws?lang=en"
			conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Cookie": {guest.Name + "=" + guest.Value}, "Origin": {hs.URL}})
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
				if strings.Contains(string(raw), `"effect"`) || strings.Contains(string(raw), `"castleOnly"`) || strings.Contains(string(raw), `"queue"`) {
					t.Fatal("internal continuation leaked")
				}
				return checkWildlingWire(t, raw, true)
			}
			view := read()
			option := "winterfell:footman"
			if won {
				option = "winterfell"
			}
			body := fmt.Sprintf(`{"type":"wildling_choice","house":"stark","choice":%q,"revision":%d}`, option, view.Revision)
			if w := influenceRequest(s, r.ID, guest, body); w.Code != http.StatusBadRequest {
				t.Fatal("foreign decision accepted")
			}
			bad := fmt.Sprintf(`{"type":"wildling_choice","choice":"pyke","revision":%d}`, view.Revision)
			if w := influenceRequest(s, r.ID, cookies["stark"], bad); w.Code != http.StatusBadRequest {
				t.Fatal("invalid target accepted")
			}
			unchanged, err := s.Store.Load(r.ID)
			if err != nil || !reflect.DeepEqual(r, unchanged) {
				t.Fatal("rejected request mutated state", err)
			}
			if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusOK {
				t.Fatal("valid choice rejected", w.Body.String())
			}
			view = read()
			if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusConflict {
				t.Fatal("stale decision replayed")
			}
			if err := s.AdvanceBots(now); err != nil {
				t.Fatal(err)
			}
			view = read()
			if won {
				if view.Phase != game.PhaseMustering || view.Mustering == nil || len(view.Mustering.Points) != 1 || view.Mustering.Points["winterfell"] != 2 {
					t.Fatal("nested muster not delivered over WebSocket")
				}
				body = fmt.Sprintf(`{"type":"muster","region":"winterfell","target":"winterfell","unit":"knight","upgrade":true,"revision":%d}`, view.Revision)
			} else {
				if view.Wildlings.Attack.Choice.Count != 1 || view.Wildlings.Attack.Choice.Options[0] != "winterfell:footman" {
					t.Fatal("second casualty lost castle scope")
				}
				body = fmt.Sprintf(`{"type":"wildling_choice","choice":"winterfell:footman","revision":%d}`, view.Revision)
			}
			if w := influenceRequest(s, r.ID, guest, body); w.Code != http.StatusBadRequest {
				t.Fatal("foreign nested decision accepted")
			}
			if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusOK {
				t.Fatal("nested decision failed", w.Body.String())
			}
			view = read()
			if won {
				if view.Mustering.Points["winterfell"] != 1 {
					t.Fatal("upgrade point cost not propagated")
				}
				body = fmt.Sprintf(`{"type":"finish_muster","revision":%d}`, view.Revision)
				if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusOK {
					t.Fatal("could not decline remaining muster points")
				}
				view = read()
			}
			for step := 0; step < 16 && view.Wildlings.Attack != nil; step++ {
				if err := s.AdvanceBots(now); err != nil {
					t.Fatal(err)
				}
				view = read()
			}
			if view.Wildlings.Attack != nil || view.Wildlings.Last.Card != game.WildlingHorde || view.Mustering != nil {
				t.Fatal("headless completion failed")
			}
		})
	}
}
