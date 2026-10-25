package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"westeros-web/internal/game"
	"westeros-web/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "game.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db, nil)
}

func createRoom(t *testing.T, s *Server) (game.View, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(`{"name":"Player"}`))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("create HTTP %d: %s", w.Code, w.Body.String())
	}
	var view game.View
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("insecure identity cookie")
	}
	return view, cookies[0]
}

func TestRoomMembershipAndRestore(t *testing.T) {
	s := testServer(t)
	room, cookie := createRoom(t, s)
	for _, authenticated := range []bool{false, true} {
		r := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID, nil)
		if authenticated {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		want := 403
		if authenticated {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("authenticated=%v: HTTP %d", authenticated, w.Code)
		}
	}
}

func TestRejectsCrossOriginAndTrailingJSON(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(`{"name":"Guest"}`))
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign origin accepted")
	}
	r = httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(`{"name":"Guest"} {"name":"Other"}`))
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("trailing JSON accepted")
	}
}

func TestStaleCommandCannotMutateRoom(t *testing.T) {
	s := testServer(t)
	room, cookie := createRoom(t, s)
	for _, want := range []int{200, 409} {
		r := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/commands", strings.NewReader(`{"type":"start","revision":1}`))
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("want %d got %d: %s", want, w.Code, w.Body.String())
		}
	}
	saved, err := s.Store.Load(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 2 || saved.Phase != "planning" {
		t.Fatal("stale command changed room")
	}
}

func TestWebSocketMembershipAndPrivateUpdates(t *testing.T) {
	s := testServer(t)
	room, cookie := createRoom(t, s)
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rooms/" + room.ID + "/ws"
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		conn.Close()
		t.Fatal("anonymous websocket accepted")
	}
	if response == nil || response.StatusCode != 403 {
		t.Fatal("expected websocket forbidden")
	}
	if response.Body != nil {
		response.Body.Close()
	}
	header := http.Header{"Cookie": []string{cookie.Name + "=" + cookie.Value}, "Origin": []string{httpServer.URL}}
	conn, _, err = websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var view game.View
	if err = conn.ReadJSON(&view); err != nil {
		t.Fatal(err)
	}
	if view.Phase != "lobby" || !view.Owner {
		t.Fatal("wrong initial websocket snapshot")
	}
	r := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/commands", strings.NewReader(`{"type":"start","revision":1}`))
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("start: %s", w.Body.String())
	}
	if err = conn.ReadJSON(&view); err != nil {
		t.Fatal(err)
	}
	if view.Phase != "planning" || view.Revision != 2 || len(view.Orders) != 0 {
		t.Fatalf("bad or leaked update: %+v", view)
	}
}

func TestFailedCommandDoesNotChangePersistedRoom(t *testing.T) {
	s := testServer(t)
	room, cookie := createRoom(t, s)
	r := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/commands", strings.NewReader(`{"type":"order","revision":1,"region":"pyke","order":"march"}`))
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("invalid command accepted")
	}
	saved, err := s.Store.Load(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 || len(saved.Orders) != 0 {
		t.Fatal("failed command changed persistent state")
	}
}
