package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"westeros-web/internal/game"
)

func TestLocalizedCatalogAndErrors(t *testing.T) {
	s := testServer(t)
	for _, check := range []struct{ query, header, language, card string }{
		{"", "", "ru", "Эддард Старк"},
		{"", "en-US,en;q=0.8", "en", "Eddard Stark"},
		{"?lang=ru", "en", "ru", "Эддард Старк"},
		{"?lang=en", "ru", "en", "Eddard Stark"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/api/catalog"+check.query, nil)
		r.Header.Set("Accept-Language", check.header)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		var body struct {
			Houses map[string]game.House `json:"houses"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || w.Header().Get("Content-Language") != check.language || body.Houses["stark"].Cards["eddard-stark"].Name != check.card {
			t.Fatalf("bad localized catalog (%+v): %s", check, w.Body.String())
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(`{"name":""}`))
	r.Header.Set("Accept-Language", "en")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Enter a name") {
		t.Fatalf("untranslated error: %s", w.Body.String())
	}
}

func TestWebSocketLanguageIsPersonalAndDoesNotMutateRoom(t *testing.T) {
	s := testServer(t)
	room, cookie := createRoom(t, s)
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	header := http.Header{"Cookie": {cookie.Name + "=" + cookie.Value}, "Origin": {httpServer.URL}}
	connections := map[string]*websocket.Conn{}
	for _, language := range []string{"ru", "en"} {
		url := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rooms/" + room.ID + "/ws?lang=" + language
		conn, _, err := websocket.DefaultDialer.Dial(url, header)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		connections[language] = conn
		var view game.View
		if err := conn.ReadJSON(&view); err != nil {
			t.Fatal(err)
		}
		prefix := "Создана комната."
		if language == "en" {
			prefix = "Room created."
		}
		if !strings.HasPrefix(view.Log[0], prefix) || view.Seats[0].Name != "Player" {
			t.Fatalf("wrong %s projection: %+v", language, view)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/commands", strings.NewReader(`{"type":"start","revision":1}`))
	r.Header.Set("Accept-Language", "en")
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("start: %s", w.Body.String())
	}
	for language, conn := range connections {
		var view game.View
		if err := conn.ReadJSON(&view); err != nil {
			t.Fatal(err)
		}
		prefix := "Начато тайное планирование"
		if language == "en" {
			prefix = "Secret planning"
		}
		if view.Revision != 2 || view.Phase != "planning" || !strings.HasPrefix(view.Log[len(view.Log)-1], prefix) {
			t.Fatalf("wrong %s update: %+v", language, view)
		}
	}
	saved, err := s.Store.Load(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 2 || saved.Seats[1].Name != "Бот · Грейджой" || !strings.HasPrefix(saved.Log[0], "Создана комната.") {
		t.Fatal("English client changed the saved room")
	}
}
