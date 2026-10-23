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

func checkWesterosWireView(t *testing.T, data []byte, current bool) {
	t.Helper()
	var wire struct {
		Westeros map[string]json.RawMessage `json:"westeros"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Westeros) == 0 {
		t.Fatal("missing public deck counters")
	}
	for key := range wire.Westeros {
		if key != "decks" && key != "current" && key != "threat" && key != "run" {
			t.Fatal("unexpected private Westeros field", key)
		}
	}
	var decks []map[string]json.RawMessage
	if err := json.Unmarshal(wire.Westeros["decks"], &decks); err != nil || len(decks) != 3 {
		t.Fatal("invalid public deck projection", err)
	}
	for _, deck := range decks {
		if len(deck) != 2 || deck["remaining"] == nil || deck["discard"] == nil {
			t.Fatal("future deck order leaked or counters missing", deck)
		}
	}
	if (wire.Westeros["current"] != nil) != current {
		t.Fatal("wrong reveal state")
	}
	// Check the entire raw payload rather than a struct that silently drops fields.
	if strings.Contains(string(data), `"draw"`) {
		t.Fatal("raw HTTP/WS response exposes the hidden draw order")
	}
}

func TestWesterosHTTPWebSocketReconnectAndBroadcastKeepDecksPrivate(t *testing.T) {
	s := testServer(t)
	view, cookie := createRoom(t, s)
	request := httptest.NewRequest(http.MethodPost, "/api/rooms/"+view.ID+"/commands", strings.NewReader(`{"type":"start","revision":1}`))
	request.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, request)
	if w.Code != http.StatusOK {
		t.Fatal("cannot start", w.Body.String())
	}
	checkWesterosWireView(t, w.Body.Bytes(), false)
	r, err := s.Store.Load(view.ID)
	if err != nil || r.Westeros == nil || len(r.Westeros.Decks[0].Draw) != 10 {
		t.Fatal("start did not persist server-only decks", err)
	}
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rooms/" + r.ID + "/ws"
	for _, lang := range []string{"ru", "en"} {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?lang="+lang, http.Header{"Cookie": {cookie.Name + "=" + cookie.Value}, "Origin": {httpServer.URL}})
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, initial, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		checkWesterosWireView(t, initial, r.Westeros.Current != nil)
		// Simulate the future internal phase runner under the same server lock;
		// no debug endpoint or public draw command is added to production.
		s.mu.Lock()
		r.Phase, r.Round = game.PhaseWesteros, 2
		err = r.RevealWesterosCards()
		if err == nil {
			old := r.Revision
			r.Revision++
			err = s.Store.Save(r, old)
		}
		if err == nil {
			s.broadcast(r)
		}
		s.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		_, broadcast, err := conn.ReadMessage()
		_ = conn.Close()
		if err != nil {
			t.Fatal(err)
		}
		checkWesterosWireView(t, broadcast, true)
		request = httptest.NewRequest(http.MethodGet, "/api/rooms/"+r.ID, nil)
		request.AddCookie(cookie)
		request.Header.Set("Accept-Language", lang)
		w = httptest.NewRecorder()
		s.ServeHTTP(w, request)
		if w.Code != http.StatusOK {
			t.Fatal("reconnect failed")
		}
		checkWesterosWireView(t, w.Body.Bytes(), true)
	}
	before := r.Clone()
	for _, command := range []string{"reveal_westeros", "draw_westeros"} {
		request = httptest.NewRequest(http.MethodPost, "/api/rooms/"+r.ID+"/commands", strings.NewReader(fmt.Sprintf(`{"type":%q,"revision":%d}`, command, r.Revision)))
		request.AddCookie(cookie)
		w = httptest.NewRecorder()
		s.ServeHTTP(w, request)
		if w.Code != http.StatusBadRequest {
			t.Fatal("host can force/reroll event draws", w.Code)
		}
	}
	saved, err := s.Store.Load(r.ID)
	if err != nil || saved.Revision != before.Revision || !reflect.DeepEqual(saved.Westeros, before.Westeros) {
		t.Fatal("rejected command changed persisted cards", err)
	}
}
