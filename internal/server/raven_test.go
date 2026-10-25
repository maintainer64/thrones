package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"westeros-web/internal/game"
)

func seedRavenRoom(t *testing.T, s *Server) (*game.Room, map[string]*http.Cookie) {
	t.Helper()
	r, cookies := seedInfluenceRoom(t, s)
	r.Influence, r.Phase, r.Clock = nil, "orders_revealed", game.Clock{}
	r.Match.Tracks[string(game.TrackKingsCourt)] = slices.Clone(game.BaseHouses)
	r.Match.Armies = map[string][]game.Unit{
		"winterfell":   {{Type: "footman", House: "stark", Count: 2}},
		"white-harbor": {{Type: "footman", House: "stark", Count: 1}},
		"pyke":         {{Type: "footman", House: "greyjoy", Count: 1}},
	}
	r.Orders = map[string]string{"winterfell": "march", "white-harbor": "march-minus", "pyke": "defend-1"}
	index := slices.Index(r.Wildlings.Deck, game.WildlingPreemptive)
	r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
	if err := r.ApplyAt(r.Host, game.Command{Type: "continue"}, s.Catalog, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	return r, cookies
}

func ravenCommand(t *testing.T, s *Server, id string, cookie *http.Cookie, command game.Command) *httptest.ResponseRecorder {
	t.Helper()
	r, err := s.Store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	command.Revision = r.Revision
	body, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	return influenceRequest(s, id, cookie, string(body))
}

func TestRavenHTTPWebSocketPrivacyAndRevisionProtection(t *testing.T) {
	s := testServer(t)
	r, cookies := seedRavenRoom(t, s)
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	guest := cookies["greyjoy"]
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/api/rooms/"+r.ID+"/ws?lang=en", http.Header{"Cookie": {guest.Name + "=" + guest.Value}, "Origin": {httpServer.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	read := func() game.View {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var view game.View
		if err := conn.ReadJSON(&view); err != nil {
			t.Fatal(err)
		}
		return view
	}
	if view := read(); view.Raven == nil || view.Raven.Card != "" || view.Raven.Options != nil {
		t.Fatal("invalid initial foreign projection")
	}
	peek := ravenCommand(t, s, r.ID, cookies["stark"], game.Command{Type: "raven", Choice: string(game.RavenPeek)})
	if peek.Code != 200 || !strings.Contains(peek.Body.String(), `"card":"preemptive-raid"`) {
		t.Fatalf("own peek HTTP %d: %s", peek.Code, peek.Body.String())
	}
	saved, _ := s.Store.Load(r.ID)
	if saved.Raven == nil || saved.Raven.Card != game.WildlingPreemptive || !saved.Match.RavenUsed || saved.Phase != game.PhaseRaven {
		t.Fatal("peek was not persisted")
	}
	foreign := read()
	if foreign.Raven == nil || foreign.Raven.Stage != game.RavenPeeking || foreign.Raven.Card != "" || foreign.Raven.Options != nil {
		t.Fatal("foreign WS contains private Raven state")
	}
	for _, response := range []string{mustJSON(t, foreign), influenceRequest(s, r.ID, guest, "").Body.String()} {
		if strings.Contains(response, "preemptive-raid") || strings.Contains(response, `"deck"`) || strings.Contains(response, `"draw"`) {
			t.Fatal("secret in foreign HTTP/WS snapshot or history")
		}
	}
	for _, choice := range []game.RavenDecision{game.RavenBottom, game.RavenReplace, game.RavenSkip} {
		w := ravenCommand(t, s, r.ID, guest, game.Command{Type: "raven", House: "stark", Choice: string(choice), Region: "winterfell", Order: "support-1"})
		if w.Code != 400 || strings.Contains(w.Body.String(), "preemptive-raid") || strings.Contains(w.Body.String(), "Посыльного") {
			t.Fatalf("foreign command HTTP %d: %s", w.Code, w.Body.String())
		}
	}
	stale, _ := json.Marshal(game.Command{Type: "raven", Choice: string(game.RavenBottom), Revision: r.Revision})
	if response := influenceRequest(s, r.ID, cookies["stark"], string(stale)); response.Code != 409 {
		t.Fatal("stale command accepted")
	}
	after, _ := s.Store.Load(r.ID)
	if after.Revision != saved.Revision || after.Raven.Card != saved.Raven.Card || !after.Clock.Deadline.Equal(saved.Clock.Deadline) {
		t.Fatal("rejected commands changed pending decision")
	}
	finish := ravenCommand(t, s, r.ID, cookies["stark"], game.Command{Type: "raven", Choice: string(game.RavenBottom)})
	if finish.Code != 200 || strings.Contains(finish.Body.String(), "preemptive-raid") {
		t.Fatal("placement failed or exposed hidden card")
	}
	final := read()
	if final.Raven != nil || final.Phase == game.PhaseRaven || !final.Match.RavenUsed {
		t.Fatal("WS did not resume after placement")
	}
	stored, _ := s.Store.Load(r.ID)
	if stored.Wildlings.Deck[len(stored.Wildlings.Deck)-1] != game.WildlingPreemptive {
		t.Fatal("bottom placement not persisted")
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRavenSchedulerAndHardTimeoutWorkWithoutBrowsers(t *testing.T) {
	for _, mode := range []string{"human", "bot", "hard-before", "hard-after"} {
		t.Run(mode, func(t *testing.T) {
			s := testServer(t)
			r, cookies := seedRavenRoom(t, s)
			if mode == "hard-after" {
				if w := ravenCommand(t, s, r.ID, cookies["stark"], game.Command{Type: "raven", Choice: string(game.RavenPeek)}); w.Code != 200 {
					t.Fatal(w.Body.String())
				}
				r, _ = s.Store.Load(r.ID)
			}
			if mode == "bot" {
				r.Seat(r.Host).Bot = true
			}
			r.Settings.HardTimer = strings.HasPrefix(mode, "hard")
			r.Revision++
			if err := s.Store.Save(r, r.Revision-1); err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 3; step++ {
				if err := s.Expire(r.Clock.Deadline); err != nil {
					t.Fatal(err)
				}
				if err := s.AdvanceBots(r.Clock.Deadline); err != nil {
					t.Fatal(err)
				}
				current, _ := s.Store.Load(r.ID)
				if current.Phase != game.PhaseRaven {
					break
				}
			}
			current, _ := s.Store.Load(r.ID)
			if mode == "human" {
				if current.Revision != r.Revision || current.Phase != game.PhaseRaven {
					t.Fatal("scheduler played for a human")
				}
			} else if current.Phase == game.PhaseRaven || current.Raven != nil {
				t.Fatal("pending Raven was omitted from timer/bot queries")
			}
			if strings.HasPrefix(mode, "hard") && !slices.Equal(current.Wildlings.Deck, r.Wildlings.Deck) {
				t.Fatal("safe timeout changed the deck")
			}
		})
	}
}
