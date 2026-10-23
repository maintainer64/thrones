package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"westeros-web/internal/game"
)

func TestHTTPSetupValidatesCreateJoinAndStart(t *testing.T) {
	s := testServer(t)
	for _, house := range []string{"stark", "martell"} {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("POST", "/api/rooms", strings.NewReader(fmt.Sprintf(`{"name":"Host","house":%q,"settings":{"turnMinutes":5,"scenario":"base","playerCount":3}}`, house))))
		if house == "martell" {
			if w.Code != 400 {
				t.Fatal("excluded house created", w.Code)
			}
			continue
		}
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var room game.View
		if err := json.Unmarshal(w.Body.Bytes(), &room); err != nil {
			t.Fatal(err)
		}
		if len(room.Seats) != 3 || room.Setup == nil || room.Setup.PlayerCount != 3 {
			t.Fatal("wrong public setup")
		}
		cookie := w.Result().Cookies()[0]
		denied := httptest.NewRecorder()
		s.ServeHTTP(denied, httptest.NewRequest("POST", "/api/rooms/"+room.ID+"/join", strings.NewReader(`{"name":"Guest","house":"greyjoy"}`)))
		if denied.Code != 400 {
			t.Fatal("excluded guest joined")
		}
		lobby := httptest.NewRecorder()
		s.ServeHTTP(lobby, httptest.NewRequest("GET", "/api/rooms/"+room.ID+"/lobby", nil))
		if strings.Contains(lobby.Body.String(), "martell") || strings.Contains(lobby.Body.String(), "greyjoy") {
			t.Fatal("excluded house in invitation")
		}
		start := httptest.NewRequest("POST", "/api/rooms/"+room.ID+"/commands", strings.NewReader(fmt.Sprintf(`{"type":"start","revision":%d}`, room.Revision)))
		start.AddCookie(cookie)
		w = httptest.NewRecorder()
		s.ServeHTTP(w, start)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
}
