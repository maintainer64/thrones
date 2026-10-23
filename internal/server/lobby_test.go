package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"westeros-web/internal/game"
)

func TestCreateChosenHouseAndPrivateInvitationAvailability(t *testing.T) {
	s := testServer(t)
	request := httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(`{"name":"Host private name","house":"martell"}`))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, request)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var room game.View
	if err := json.Unmarshal(w.Body.Bytes(), &room); err != nil {
		t.Fatal(err)
	}
	if room.YourHouse != "martell" || !room.Owner {
		t.Fatal("selected house or ownership lost")
	}
	hostCookie := w.Result().Cookies()[0]
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/lobby", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "Host private name") || strings.Contains(w.Body.String(), `"player"`) || strings.Contains(w.Body.String(), `"orders"`) {
		t.Fatalf("lobby leaked private data: %s", w.Body.String())
	}
	var lobby struct {
		Seats []struct {
			House     string `json:"house"`
			Available bool   `json:"available"`
		} `json:"seats"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &lobby); err != nil {
		t.Fatal(err)
	}
	if len(lobby.Seats) != 6 {
		t.Fatal("missing houses")
	}
	for _, seat := range lobby.Seats {
		if seat.Available != (seat.House != "martell") {
			t.Fatalf("wrong availability: %+v", seat)
		}
	}
	for _, want := range []int{200, 400} {
		w = httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/join", strings.NewReader(`{"name":"Guest","house":"stark"}`)))
		if w.Code != want {
			t.Fatalf("join: want %d, got %d: %s", want, w.Code, w.Body.String())
		}
	}
	request = httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/commands", strings.NewReader(`{"type":"start","revision":2}`))
	request.AddCookie(hostCookie)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, request)
	if w.Code != 200 {
		t.Fatalf("start: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/lobby", nil))
	if strings.Contains(w.Body.String(), `"available":true`) {
		t.Fatal("houses available after start")
	}
}

func TestCreateRejectsInvalidHouse(t *testing.T) {
	s := testServer(t)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(`{"name":"Host","house":"targaryen"}`)))
	if w.Code != 400 {
		t.Fatalf("unknown house accepted: %s", w.Body.String())
	}
}
