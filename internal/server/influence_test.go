package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"westeros-web/internal/game"
)

func influenceServerFixture(t *testing.T) (*Server, *game.Room, map[string]*http.Cookie) {
	t.Helper()
	s := testServer(t)
	r, cookies := seedInfluenceRoom(t, s)
	return s, r, cookies
}

func seedInfluenceRoom(t *testing.T, s *Server) (*game.Room, map[string]*http.Cookie) {
	t.Helper()
	view, cookie := createRoom(t, s)
	r, err := s.Store.Load(view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.ApplyAt(r.Host, game.Command{Type: "start"}, s.Catalog, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	cookies := map[string]*http.Cookie{"stark": cookie}
	for i := range r.Seats {
		seat := &r.Seats[i]
		if seat.House != "stark" {
			cookies[seat.House] = &http.Cookie{Name: cookieName, Value: randomToken()}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.AddCookie(cookies[seat.House])
			seat.Player = playerID(req)
		}
		seat.Bot = false
		r.Match.Power[seat.House] = 20
	}
	r.Phase, r.Orders = game.PhaseWesteros, map[string]string{}
	if err = r.BeginClashOfKings(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err = s.Store.Save(r, view.Revision); err != nil {
		t.Fatal(err)
	}
	return r, cookies
}

func influenceRequest(s *Server, id string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	method, path := http.MethodGet, "/api/rooms/"+id
	if body != "" {
		method, path = http.MethodPost, path+"/commands"
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Accept-Language", "en")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	return w
}

func TestInfluenceHTTPAndWebSocketKeepOtherBidsSecret(t *testing.T) {
	s, r, cookies := influenceServerFixture(t)
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	guest := cookies["greyjoy"]
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rooms/" + r.ID + "/ws?lang=en"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Cookie": {guest.Name + "=" + guest.Value}, "Origin": {httpServer.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	readSocket := func() game.View {
		t.Helper()
		var v game.View
		if err := conn.ReadJSON(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	initial := readSocket()
	if initial.Phase != game.PhaseInfluenceBidding || initial.Influence == nil || initial.Influence.Bids.YourBid != nil {
		t.Fatal("invalid initial auction snapshot")
	}
	body := fmt.Sprintf(`{"type":"bid","bid":13,"house":"greyjoy","revision":%d}`, r.Revision)
	w := influenceRequest(s, r.ID, cookies["stark"], body)
	if w.Code != http.StatusOK {
		t.Fatalf("bid HTTP %d: %s", w.Code, w.Body.String())
	}
	var own game.View
	if err := json.Unmarshal(w.Body.Bytes(), &own); err != nil {
		t.Fatal(err)
	}
	if own.Influence.Bids.YourBid == nil || *own.Influence.Bids.YourBid != 13 || !own.Influence.Bids.Submitted["stark"] || own.Influence.Bids.Submitted["greyjoy"] {
		t.Fatal("house spoofing succeeded or own bid is missing")
	}
	foreign := readSocket()
	if foreign.Influence.Bids.YourBid != nil || foreign.Influence.Bids.Values != nil || foreign.Match.Power["stark"] != 20 || !foreign.Influence.Bids.Submitted["stark"] {
		t.Fatal("bid amount leaked in WebSocket broadcast")
	}
	w = influenceRequest(s, r.ID, guest, "")
	var raw map[string]json.RawMessage
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &raw) != nil {
		t.Fatal("cannot restore guest session")
	}
	if strings.Contains(string(raw["influence"]), `"values"`) || strings.Contains(string(raw["influence"]), `"yourBid"`) {
		t.Fatal("bid amount leaked on HTTP reconnect")
	}
	// Stale commands must not overwrite the persisted bid or debit power.
	w = influenceRequest(s, r.ID, cookies["stark"], body)
	if w.Code != http.StatusConflict {
		t.Fatal("stale bid accepted")
	}
	saved, err := s.Store.Load(r.ID)
	if err != nil || saved.Revision != r.Revision+1 || saved.Influence.Bids.Values["stark"] != 13 || saved.Match.Power["stark"] != 20 {
		t.Fatal("stale command mutated auction", err)
	}
	body = fmt.Sprintf(`{"type":"bid","bid":0,"revision":%d}`, saved.Revision)
	w = influenceRequest(s, r.ID, guest, body)
	if w.Code != http.StatusOK {
		t.Fatalf("zero bid HTTP %d: %s", w.Code, w.Body.String())
	}
	foreign = readSocket()
	if foreign.Influence.Bids.YourBid == nil || *foreign.Influence.Bids.YourBid != 0 || foreign.Influence.Bids.Values != nil {
		t.Fatal("zero bid lost or other bids revealed")
	}
}

func TestInfluenceSchedulerFindsBidsAndTieChoicesWithoutBrowser(t *testing.T) {
	s, r, _ := influenceServerFixture(t)
	for i := range r.Seats {
		r.Seats[i].Bot = true
		r.Match.Power[r.Seats[i].House] = 0
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	initial := r.Revision
	for i := 0; i < 50; i++ {
		if err := s.AdvanceBots(time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		saved, err := s.Store.Load(r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.Revision != initial+int64(i)+1 {
			t.Fatal("automation failed to persist exactly one decision")
		}
		if saved.Phase == game.PhaseWesteros {
			if len(saved.Match.LastInfluence) != 3 || saved.Influence != nil {
				t.Fatal("missing final auction results")
			}
			return
		}
	}
	t.Fatal("automation stalled in zero-power auctions")
}

func TestInfluenceDeadlineIsPersistedAndDoesNotOverwriteCommittedBid(t *testing.T) {
	s, r, _ := influenceServerFixture(t)
	r.Settings.HardTimer = true
	past := time.Now().UTC().Add(-10 * time.Minute)
	// Recreate the event at a past instant so the real SQL Due query finds it.
	r.Influence = nil
	r.Phase = game.PhaseWesteros
	if err := r.BeginClashOfKings(past); err != nil {
		t.Fatal(err)
	}
	amount := 3
	if err := r.ApplyAt(r.Host, game.Command{Type: "bid", Bid: &amount}, s.Catalog, past); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	if err := s.Expire(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Store.Load(r.ID)
	if err != nil || saved.Phase != game.PhaseInfluenceTies || saved.Influence.Bids.Values["stark"] != 3 || saved.Match.Power["stark"] != 17 || saved.Revision != r.Revision+1 {
		t.Fatal("server deadline did not resume persisted auction", err)
	}
	if err := s.Expire(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	saved, _ = s.Store.Load(r.ID)
	if saved.Revision != r.Revision+1 {
		t.Fatal("fresh tie decision expired twice")
	}
}
