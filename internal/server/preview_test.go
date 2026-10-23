package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"westeros-web/internal/game"
)

func TestPreviewAuthorizationRevisionAndReadOnlyStorage(t *testing.T) {
	s := testServer(t)
	view, cookie := createRoom(t, s)
	r, err := s.Store.Load(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	identity := httptest.NewRequest(http.MethodGet, "/", nil)
	identity.AddCookie(cookie)
	player := playerID(identity)
	if err = r.Apply(player, game.Command{Type: "start"}, s.Catalog); err != nil {
		t.Fatal(err)
	}
	r.Phase = "marches"
	r.Match.ActiveHouse = "stark"
	r.Orders = map[string]string{"winterfell": "march"}
	old := r.Revision
	r.Revision++
	if err = s.Store.Save(r, old); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(r)
	for _, tc := range []struct {
		name   string
		member bool
		stale  bool
		kind   string
		origin string
		status int
	}{
		{"member", true, false, "march", "", 200},
		{"outsider", false, false, "march", "", 403},
		{"stale", true, true, "march", "", 409},
		{"wrong-kind", true, false, "settings", "", 400},
		{"foreign-origin", true, false, "march", "https://evil.example", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			revision := r.Revision
			if tc.stale {
				revision--
			}
			data, _ := json.Marshal(game.Command{Type: tc.kind, Revision: revision, Region: "winterfell", Units: map[string]int{"footman": 1}})
			request := httptest.NewRequest(http.MethodPost, "/api/rooms/"+r.ID+"/preview", strings.NewReader(string(data)))
			if tc.member {
				request.AddCookie(cookie)
			}
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			response := httptest.NewRecorder()
			s.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("HTTP %d: %s", response.Code, response.Body.String())
			}
			if tc.status == 200 {
				var preview game.ActionPreview
				if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
					t.Fatal(err)
				}
				if preview.Revision != r.Revision || len(preview.Targets) == 0 {
					t.Fatal("invalid preview", preview)
				}
			}
		})
	}
	saved, _ := s.Store.Load(r.ID)
	after, _ := json.Marshal(saved)
	if string(before) != string(after) {
		t.Fatal("preview changed persisted state")
	}
}
