package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"westeros-web/internal/game"
)

func TestPreemptiveTrackDelegationAndRepeatBidsHTTPWebSocket(t *testing.T) {
	for _, won := range []bool{false, true} {
		t.Run(fmt.Sprint(won), func(t *testing.T) {
			s, r, cookies, now := wildlingServerFixture(t, game.WildlingPreemptive)
			r.Wildlings.Attack.Strength, r.Westeros.Run.Attack.Strength, r.Westeros.Threat = 12, 12, 12
			r.Match.Tracks["ironThrone"] = []string{"greyjoy", "stark", "lannister", "baratheon", "tyrell", "martell"}
			r.Match.Tracks["fiefdoms"] = slices.Clone(r.Match.Tracks["ironThrone"])
			r.Match.Tracks["kingsCourt"] = []string{"greyjoy", "lannister", "stark", "baratheon", "tyrell", "martell"}
			r.Match.BladeUsed = true
			index := slices.Index(r.Wildlings.Deck, game.WildlingSilence)
			r.Wildlings.Deck[1], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[1]
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
			hs := httptest.NewServer(s)
			defer hs.Close()
			observer := cookies["stark"]
			url := "ws" + strings.TrimPrefix(hs.URL, "http") + "/api/rooms/" + r.ID + "/ws?lang=en"
			conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Cookie": {observer.Name + "=" + observer.Value}, "Origin": {hs.URL}})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			read := func(revealed bool) game.View {
				t.Helper()
				_, raw, err := conn.ReadMessage()
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(raw), `"effect"`) || strings.Contains(string(raw), `"queue"`) {
					t.Fatal("private resolver state leaked")
				}
				return checkWildlingWire(t, raw, revealed)
			}
			view := read(!won)
			if won {
				if view.Phase != game.PhaseWildlingBidding || view.Wildlings.Attack.Strength != 6 || view.Wildlings.Attack.ExcludedHouse != "stark" || len(view.Wildlings.Attack.Bids.Submitted) != 5 {
					t.Fatal("repeat attack/exemption not broadcast")
				}
				body := fmt.Sprintf(`{"type":"wildling_bid","house":"greyjoy","bid":0,"revision":%d}`, view.Revision)
				before, _ := s.Store.Load(r.ID)
				if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusBadRequest {
					t.Fatal("excluded house could bid by spoofing another house")
				}
				after, err := s.Store.Load(r.ID)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("rejected excluded bid mutated state", err)
				}
				for _, house := range []string{"greyjoy", "lannister", "baratheon", "tyrell", "martell"} {
					bid := 0
					if house == "greyjoy" {
						bid = 6
					}
					body = fmt.Sprintf(`{"type":"wildling_bid","bid":%d,"revision":%d}`, bid, view.Revision)
					if w := influenceRequest(s, r.ID, cookies[house], body); w.Code != http.StatusOK {
						t.Fatal("repeat contribution failed", w.Body.String())
					}
					view = read(house == "martell")
					if house == "greyjoy" && (view.Wildlings.Attack.Bids.YourBid != nil || view.Wildlings.Attack.Bids.Values != nil || view.Match.Power["greyjoy"] != 9) {
						t.Fatal("repeat secret contribution leaked to excluded observer")
					}
					if w := influenceRequest(s, r.ID, cookies[house], body); w.Code != http.StatusConflict {
						t.Fatal("stale repeat contribution replayed")
					}
				}
				if view.Wildlings.Attack.Card != game.WildlingSilence || view.Wildlings.Attack.Outcome.SelectedHouse != "greyjoy" || view.Match.Power["stark"] != 3 {
					t.Fatal("repeat attack reused old winner, card or bids")
				}
			} else {
				if view.Wildlings.Attack.Choice.Kind != game.WildlingChoosePenalty || view.Wildlings.Attack.Choice.House != "stark" {
					t.Fatal("lowest bidder not offered penalty choice")
				}
				body := fmt.Sprintf(`{"type":"wildling_choice","choice":"lose-influence","revision":%d}`, view.Revision)
				if w := influenceRequest(s, r.ID, cookies["greyjoy"], body); w.Code != http.StatusBadRequest {
					t.Fatal("throne chose penalty instead of lowest bidder")
				}
				if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusOK {
					t.Fatal("penalty choice failed", w.Body.String())
				}
				view = read(true)
				if err := s.AdvanceBots(now); err != nil {
					t.Fatal(err)
				}
				view = read(true)
				if view.Wildlings.Attack.Choice.House != "greyjoy" || view.Clock.House != "greyjoy" || len(view.Wildlings.Attack.Choice.Options) != 2 {
					t.Fatal("tied track selection not delegated")
				}
				body = fmt.Sprintf(`{"type":"wildling_choice","house":"greyjoy","choice":"fiefdoms","revision":%d}`, view.Revision)
				if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusBadRequest {
					t.Fatal("lowest bidder stole the throne's tie decision")
				}
				if w := influenceRequest(s, r.ID, cookies["greyjoy"], body); w.Code != http.StatusOK {
					t.Fatal("throne's selected track failed", w.Body.String())
				}
				view = read(true)
				if slices.Index(view.Match.Tracks["fiefdoms"], "stark") != 3 || !view.Match.BladeUsed {
					t.Fatal("penalty or used sword state not propagated")
				}
				if w := influenceRequest(s, r.ID, cookies["greyjoy"], body); w.Code != http.StatusConflict {
					t.Fatal("track penalty replayed")
				}
			}
			if err := s.AdvanceBots(now); err != nil {
				t.Fatal(err)
			}
			view = read(true)
			if view.Wildlings.Attack != nil || view.Wildlings.Last == nil || view.Wildlings.Last.Outcome.WatchWon != won {
				t.Fatal("headless completion failed")
			}
		})
	}
}
