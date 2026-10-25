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

func wildlingServerFixture(t *testing.T, card game.WildlingCardID) (*Server, *game.Room, map[string]*http.Cookie, time.Time) {
	t.Helper()
	s, r, cookies := influenceServerFixture(t)
	r.Influence, r.Phase, r.Round, r.Westeros.Threat = nil, game.PhaseWesteros, 2, 0
	for i, event := range [3]game.WesterosCardID{game.WesterosSupply, game.WesterosGameOfThrones, game.WesterosWildlings} {
		deck := &r.Westeros.Decks[i]
		index := slices.Index(deck.Draw, event)
		deck.Draw[0], deck.Draw[index] = deck.Draw[index], deck.Draw[0]
	}
	index := slices.Index(r.Wildlings.Deck, card)
	r.Wildlings.Deck[0], r.Wildlings.Deck[index] = r.Wildlings.Deck[index], r.Wildlings.Deck[0]
	now := time.Now().UTC()
	if err := r.BeginWesteros(now); err != nil {
		t.Fatal(err)
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 10 && r.Phase != game.PhaseWildlingBidding; step++ {
		if err := s.AdvanceBots(now); err != nil {
			t.Fatal(err)
		}
		var err error
		r, err = s.Store.Load(r.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if r.Phase != game.PhaseWildlingBidding {
		t.Fatal("headless scheduler failed to start a real wildling bid round")
	}
	for _, seat := range r.Seats {
		r.Match.Power[seat.House] = 10
	}
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	return s, r, cookies, now
}

func checkWildlingWire(t *testing.T, data []byte, revealed bool) game.View {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var wildlings struct {
		Attack json.RawMessage `json:"attack"`
	}
	if err := json.Unmarshal(raw["wildlings"], &wildlings); err != nil {
		t.Fatal("invalid wildling payload", err)
	}
	// Last is a public completed attack and may contain its revealed card
	// while the NEXT attack's card must still be absent from Attack.
	if strings.Contains(string(raw["wildlings"]), `"deck"`) || (!revealed && strings.Contains(string(wildlings.Attack), `"card"`)) {
		t.Fatal("raw HTTP/WS payload leaks future wildling cards", string(raw["wildlings"]))
	}
	var view game.View
	if err := json.Unmarshal(data, &view); err != nil || view.Wildlings == nil {
		t.Fatal("invalid wildling projection", err)
	}
	return view
}

func TestWildlingHTTPAndWebSocketHideCommitmentsAndRevealOnlyAfterBidding(t *testing.T) {
	s, r, cookies, now := wildlingServerFixture(t, game.WildlingHorde)
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
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	checkWildlingWire(t, data, false)
	body := fmt.Sprintf(`{"type":"wildling_bid","house":"greyjoy","bid":7,"revision":%d}`, r.Revision)
	w := influenceRequest(s, r.ID, cookies["stark"], body)
	if w.Code != http.StatusOK {
		t.Fatal("bid failed", w.Code, w.Body.String())
	}
	own := checkWildlingWire(t, w.Body.Bytes(), false)
	if own.Wildlings.Attack.Bids.YourBid == nil || *own.Wildlings.Attack.Bids.YourBid != 7 || !own.Wildlings.Attack.Bids.Submitted["stark"] || own.Wildlings.Attack.Bids.Submitted["greyjoy"] {
		t.Fatal("request house spoofing changed the identity of the contributor")
	}
	_, data, err = conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	foreign := checkWildlingWire(t, data, false)
	if foreign.Wildlings.Attack.Bids.YourBid != nil || foreign.Wildlings.Attack.Bids.Values != nil || foreign.Wildlings.Attack.Outcome != nil || foreign.Match.Power["stark"] != 10 {
		t.Fatal("a private amount, sum or early balance change leaked over WS")
	}
	if w := influenceRequest(s, r.ID, guest, ""); w.Code != http.StatusOK {
		t.Fatal("reconnect failed")
	} else {
		checkWildlingWire(t, w.Body.Bytes(), false)
	}
	if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusConflict {
		t.Fatal("stale bid was accepted")
	}
	body = fmt.Sprintf(`{"type":"wildling_bid","bid":0,"revision":%d}`, own.Revision)
	if w := influenceRequest(s, r.ID, cookies["stark"], body); w.Code != http.StatusBadRequest {
		t.Fatal("committed bid can be changed")
	}
	revision := own.Revision
	for _, house := range game.BaseHouses {
		if house == "stark" {
			continue
		}
		body = fmt.Sprintf(`{"type":"wildling_bid","bid":0,"revision":%d}`, revision)
		w = influenceRequest(s, r.ID, cookies[house], body)
		if w.Code != http.StatusOK {
			t.Fatal("zero bid failed", w.Body.String())
		}
		var updated game.View
		if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
			t.Fatal(err)
		}
		revision = updated.Revision
		_, data, err = conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		view := checkWildlingWire(t, data, updated.Phase == game.PhaseWildlingEffects)
		if updated.Phase == game.PhaseWildlingEffects && (view.Wildlings.Attack.Card != game.WildlingHorde || view.Wildlings.Attack.Bids.Values["stark"] != 7 || view.Match.Power["stark"] != 3) {
			t.Fatal("last commitment did not reveal/debit exactly once")
		}
	}
	before, err := s.Store.Load(r.ID)
	if err != nil || before.Phase != game.PhaseWildlingEffects {
		t.Fatal("effects not reached", err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	before, err = s.Store.Load(r.ID)
	if err != nil || before.Phase != game.PhaseWildlingEffects || before.Wildlings.Attack.Effect.Choice == nil {
		t.Fatal("durable human decision not created", err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	after, err := s.Store.Load(r.ID)
	if err != nil || after.Revision != before.Revision || !reflect.DeepEqual(after.Wildlings, before.Wildlings) {
		t.Fatal("human decision was silently completed", err)
	}
}

func TestWildlingTieHTTPUsesThroneIdentityAndCompletedAttackPersists(t *testing.T) {
	s, r, cookies, now := wildlingServerFixture(t, game.WildlingSilence)
	for _, house := range game.BaseHouses {
		body := fmt.Sprintf(`{"type":"wildling_bid","bid":0,"revision":%d}`, r.Revision)
		w := influenceRequest(s, r.ID, cookies[house], body)
		if w.Code != http.StatusOK {
			t.Fatal("zero bid rejected")
		}
		checkWildlingWire(t, w.Body.Bytes(), false)
		var err error
		r, err = s.Store.Load(r.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if r.Phase != game.PhaseWildlingTies {
		t.Fatal("all-zero attack skipped its tie")
	}
	holder := r.Wildlings.Attack.TieBreakHouse
	foreign := "stark"
	if holder == foreign {
		foreign = "greyjoy"
	}
	body := fmt.Sprintf(`{"type":"wildling_tie","house":%q,"target":"stark","revision":%d}`, holder, r.Revision)
	if w := influenceRequest(s, r.ID, cookies[foreign], body); w.Code != http.StatusBadRequest {
		t.Fatal("request body impersonated the Iron Throne holder")
	}
	if w := influenceRequest(s, r.ID, cookies[holder], body); w.Code != http.StatusOK {
		t.Fatal("legal tie decision failed", w.Body.String())
	} else {
		view := checkWildlingWire(t, w.Body.Bytes(), true)
		if view.Wildlings.Attack.Card != game.WildlingSilence || !view.Wildlings.Attack.EffectsImplemented {
			t.Fatal("resolved tie failed to reveal the actual card")
		}
	}
	if w := influenceRequest(s, r.ID, cookies[holder], body); w.Code != http.StatusConflict {
		t.Fatal("repeated tie command was accepted")
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Store.Load(r.ID)
	if err != nil || saved.Phase != game.PhaseWesteros || saved.Wildlings.Attack != nil || saved.Wildlings.Last.Card != game.WildlingSilence || len(saved.Wildlings.Deck) != 9 || saved.Wildlings.Deck[8] != game.WildlingSilence || saved.Westeros.Run.NextCard != 3 {
		t.Fatal("headless effect failed to save and resume the exact cursor", err)
	}
	for _, command := range []string{"skip_wildlings", "begin_wildlings", "finish_wildlings"} {
		body := fmt.Sprintf(`{"type":%q,"revision":%d}`, command, saved.Revision)
		if w := influenceRequest(s, r.ID, cookies[holder], body); w.Code != http.StatusBadRequest {
			t.Fatal("a player can force/skip a rule effect")
		}
	}
}

func TestWildlingHeadlessHardDeadlinePreservesNewTieClock(t *testing.T) {
	s, r, _, now := wildlingServerFixture(t, game.WildlingSilence)
	r.Settings.HardTimer = true
	r.Revision++
	if err := s.Store.Save(r, r.Revision-1); err != nil {
		t.Fatal(err)
	}
	deadline := r.Clock.Deadline
	if err := s.Expire(deadline); err != nil {
		t.Fatal(err)
	}
	tied, err := s.Store.Load(r.ID)
	if err != nil || tied.Phase != game.PhaseWildlingTies || tied.Wildlings.Attack.Card != "" || !tied.Clock.Deadline.After(deadline) {
		t.Fatal("headless expiration skipped a new decision", err)
	}
	if err := s.Expire(deadline); err != nil {
		t.Fatal(err)
	}
	unchanged, err := s.Store.Load(r.ID)
	if err != nil || unchanged.Revision != tied.Revision {
		t.Fatal("old deadline also expired the new tie", err)
	}
	if err := s.Expire(tied.Clock.Deadline); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceBots(now); err != nil {
		t.Fatal(err)
	}
	completed, err := s.Store.Load(r.ID)
	if err != nil || completed.Wildlings.Attack != nil || completed.Wildlings.Last.Card != game.WildlingSilence || completed.Westeros.Threat != 0 {
		t.Fatal("hard-timer attack failed to complete without a browser", err)
	}
}
