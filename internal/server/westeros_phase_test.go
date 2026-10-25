package server

import (
	"encoding/json"
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

func westerosChoiceFixture(t *testing.T) (*Server, *game.Room, map[string]*http.Cookie, time.Time) {
	t.Helper()
	s, r, cookies := influenceServerFixture(t)
	r.Influence, r.Phase, r.Round = nil, game.PhaseWesteros, 2
	for i, card := range [3]game.WesterosCardID{game.WesterosThroneOfBlades, game.WesterosLastDays, game.WesterosSeaOfStorms} {
		deck := &r.Westeros.Decks[i]
		index := slices.Index(deck.Draw, card)
		deck.Draw[0], deck.Draw[index] = deck.Draw[index], deck.Draw[0]
	}
	now := time.Now().UTC()
	if err := r.BeginWesteros(now); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 2; step++ {
		if err := s.AdvanceBots(now); err != nil {
			t.Fatal(err)
		}
	}
	r, err := s.Store.Load(r.ID)
	if err != nil || r.Westeros.Run.Stage != game.WesterosChoiceStep {
		t.Fatal("scheduler did not persist the human token choice", err)
	}
	return s, r, cookies, now
}

func TestWesterosChoiceHTTPAndWebSocketValidateIdentityAndRevision(t *testing.T) {
	s, r, cookies, _ := westerosChoiceFixture(t)
	holder := r.Westeros.Run.Choice.House
	foreign := "stark"
	if foreign == holder {
		foreign = "greyjoy"
	}
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	guest := cookies[foreign]
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rooms/" + r.ID + "/ws?lang=en"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Cookie": {guest.Name + "=" + guest.Value}, "Origin": {httpServer.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	checkWesterosWireView(t, data, true)
	body := fmt.Sprintf(`{"type":"westeros_choice","house":%q,"choice":"nothing","revision":%d}`, holder, r.Revision)
	if w := influenceRequest(s, r.ID, guest, body); w.Code != http.StatusBadRequest {
		t.Fatal("request body impersonated the dominance holder")
	}
	before, err := s.Store.Load(r.ID)
	if err != nil || before.Revision != r.Revision || !reflect.DeepEqual(before.Westeros, r.Westeros) {
		t.Fatal("foreign choice mutated the continuation", err)
	}
	if w := influenceRequest(s, r.ID, cookies[holder], body); w.Code != http.StatusOK {
		t.Fatal("holder's valid choice rejected", w.Code, w.Body.String())
	} else {
		checkWesterosWireView(t, w.Body.Bytes(), true)
		var result game.View
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Westeros.Run.Choice != nil || result.Westeros.Run.NextCard != 1 {
			t.Fatal("choice did not return to the same saved cursor")
		}
	}
	_, data, err = conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	checkWesterosWireView(t, data, true)
	if w := influenceRequest(s, r.ID, cookies[holder], body); w.Code != http.StatusConflict {
		t.Fatal("stale choice was accepted")
	}
	saved, err := s.Store.Load(r.ID)
	if err != nil || saved.Revision != r.Revision+1 || !reflect.DeepEqual(saved.Westeros.Current, r.Westeros.Current) {
		t.Fatal("replayed choice rerolled cards", err)
	}
	for _, command := range []string{"begin_westeros", "skip_wildlings", "resume_westeros"} {
		body = fmt.Sprintf(`{"type":%q,"revision":%d}`, command, saved.Revision)
		if w := influenceRequest(s, r.ID, cookies[holder], body); w.Code != http.StatusBadRequest {
			t.Fatal("public command can bypass an internal rule boundary", command)
		}
	}
}

func TestWesterosHardDeadlineAndAutomaticResumePersistWithoutBrowsers(t *testing.T) {
	s, r, _, now := westerosChoiceFixture(t)
	r.Settings.HardTimer = true
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	if err := s.Expire(r.Clock.Deadline); err != nil {
		t.Fatal(err)
	}
	expired, err := s.Store.Load(r.ID)
	if err != nil || expired.Revision != r.Revision+1 || expired.Westeros.Run.Choice != nil || expired.Westeros.Run.Stage != game.WesterosEventsStep {
		t.Fatal("headless hard timer did not persist the safe decision", err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	continued, err := s.Store.Load(r.ID)
	if err != nil || continued.Revision != expired.Revision+1 || continued.Westeros.Run.NextCard != 2 || !reflect.DeepEqual(continued.Westeros.Current, r.Westeros.Current) {
		t.Fatal("headless scheduler failed to resume the second event", err)
	}
}
