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

func TestMilkwaterHTTPWebSocketAndHeadlessFAQRecovery(t *testing.T) {
	s, r, cookies, now := wildlingServerFixture(t, game.WildlingMilkwater)
	r.Wildlings.Attack.Strength, r.Westeros.Run.Attack.Strength, r.Westeros.Threat = 12, 12, 12
	r.Settings.HardTimer = true
	for _, seat := range r.Seats {
		cards := []string{}
		for id := range s.Catalog.Houses[seat.House].Cards {
			cards = append(cards, id)
		}
		slices.Sort(cards)
		keep := []string{cards[0]}
		bid := 1
		if seat.House == "stark" {
			keep, bid = []string{"roose-bolton", "greatjon-umber"}, 0
		}
		r.Match.Discard[seat.House] = []string{}
		for _, id := range cards {
			if !slices.Contains(keep, id) {
				r.Match.Discard[seat.House] = append(r.Match.Discard[seat.House], id)
			}
		}
		if err := r.ApplyAt(seat.Player, game.Command{Type: "wildling_bid", Bid: &bid}, s.Catalog, now); err != nil {
			t.Fatal(err)
		}
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	r, err := s.Store.Load(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(s)
	defer httpServer.Close()
	guest := cookies["greyjoy"]
	url := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/rooms/" + r.ID + "/ws?lang=en"
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Cookie": {guest.Name + "=" + guest.Value}, "Origin": {httpServer.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	read := func() game.View {
		t.Helper()
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		for _, private := range []string{`"deck"`, `"remainingCards"`, `"queue"`, `"effect"`} {
			if strings.Contains(string(fields["wildlings"]), private) {
				t.Fatal("private resolver internals leaked over the wire", private)
			}
		}
		return checkWildlingWire(t, raw, true)
	}
	view := read()
	if view.Wildlings.Attack.Choice == nil || view.Wildlings.Attack.Choice.Kind != game.WildlingDiscardCard || len(view.Wildlings.Attack.Choice.Options) != 2 {
		t.Fatal("real discard decision missing from the shared projection")
	}
	body := fmt.Sprintf(`{"type":"wildling_choice","house":"stark","choice":"roose-bolton","revision":%d}`, r.Revision)
	if w := influenceRequest(s, r.ID, guest, body); w.Code != http.StatusBadRequest {
		t.Fatal("another player could discard Stark's house card")
	}
	bad := fmt.Sprintf(`{"type":"wildling_choice","choice":"eddard-stark","revision":%d}`, r.Revision)
	if w := influenceRequest(s, r.ID, cookies["stark"], bad); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Choose a house card eligible") {
		t.Fatal("already discarded card accepted or error untranslated", w.Body.String())
	}
	unchanged, err := s.Store.Load(r.ID)
	if err != nil || !reflect.DeepEqual(unchanged, r) {
		t.Fatal("rejected command changed the saved hand", err)
	}
	body = fmt.Sprintf(`{"type":"wildling_choice","house":"greyjoy","choice":"roose-bolton","revision":%d}`, r.Revision)
	if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusOK {
		t.Fatal("valid discard failed", w.Body.String())
	}
	view = read()
	if view.Wildlings.Attack.Choice != nil || len(view.Match.Discard["stark"]) != 6 {
		t.Fatal("first discard did not leave one held card")
	}
	if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusConflict {
		t.Fatal("stale discard was replayed")
	}
	if err := s.AdvanceBots(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	view = read()
	if view.Wildlings.Attack.Choice == nil || !slices.Equal(view.Wildlings.Attack.Choice.Options, []string{"greatjon-umber"}) || !view.Clock.Deadline.After(r.Clock.Deadline) {
		t.Fatal("FAQ final card lost or inherited the previous deadline")
	}
	if err := s.Expire(view.Clock.Deadline); err != nil {
		t.Fatal(err)
	}
	view = read()
	if !slices.Equal(view.Match.Discard["stark"], []string{"greatjon-umber"}) || view.Wildlings.Attack.Choice != nil {
		t.Fatal("headless deadline did not recover six cards and keep only the last discard")
	}
	for step := 0; step < 8 && view.Wildlings.Attack != nil; step++ {
		if err := s.AdvanceBots(now); err != nil {
			t.Fatal(err)
		}
		view = read()
	}
	if view.Wildlings.Attack != nil || view.Wildlings.Last == nil || view.Wildlings.Last.Card != game.WildlingMilkwater || view.Wildlings.Last.ThreatAfter != 8 || !slices.Equal(view.Match.Discard["stark"], []string{"greatjon-umber"}) {
		t.Fatal("completion rediscarded the recovered hand or lost its report")
	}
}
