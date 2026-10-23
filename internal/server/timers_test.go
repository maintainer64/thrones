package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"westeros-web/internal/game"
)

func TestHardTimerRunsWithoutConnectedBrowsers(t *testing.T) {
	s := testServer(t)
	view, _ := createRoom(t, s)
	room, err := s.Store.Load(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	room.Settings.HardTimer = true
	past := time.Now().UTC().Add(-10 * time.Minute)
	if err = room.ApplyAt(room.Host, game.Command{Type: "start"}, s.Catalog, past); err != nil {
		t.Fatal(err)
	}
	room.Revision++
	if err = s.Store.Save(room, 1); err != nil {
		t.Fatal(err)
	}
	if err = s.Expire(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Store.Load(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Phase != "orders_revealed" || saved.Revision != 3 {
		t.Fatalf("timer stalled without browser: %s rev %d", saved.Phase, saved.Revision)
	}
	if err = s.Expire(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	saved, _ = s.Store.Load(view.ID)
	if saved.Revision != 3 {
		t.Fatal("timer fired twice")
	}
}

func TestLateCommandCannotRaceHardDeadline(t *testing.T) {
	s := testServer(t)
	view, cookie := createRoom(t, s)
	room, _ := s.Store.Load(view.ID)
	room.Settings.HardTimer = true
	if err := room.ApplyAt(room.Host, game.Command{Type: "start"}, s.Catalog, time.Now().UTC().Add(-10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	room.Revision++
	if err := s.Store.Save(room, 1); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+view.ID+"/commands", strings.NewReader(`{"type":"order","revision":2,"region":"winterfell","order":"march"}`))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("late command HTTP %d: %s", w.Code, w.Body.String())
	}
	saved, _ := s.Store.Load(view.ID)
	if saved.Phase != "orders_revealed" {
		t.Fatal("expired command continued planning")
	}
}
