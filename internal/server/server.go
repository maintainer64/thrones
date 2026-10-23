package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"westeros-web/internal/game"
	"westeros-web/internal/store"
)

const cookieName = "westeros_player"

type subscriber struct {
	player  string
	updates chan game.View
}
type Server struct {
	Store       *store.Store
	Catalog     game.Catalog
	Secure      bool
	mu          sync.Mutex
	subscribers map[string]map[*subscriber]bool
	mux         *http.ServeMux
}

func New(db *store.Store, assets fs.FS) *Server {
	s := &Server{Store: db, Catalog: game.LoadCatalog(), subscribers: map[string]map[*subscriber]bool{}, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "server": "go", "rulesStage": game.RulesStage})
	})
	s.mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, r *http.Request) {
		language := requestLanguage(r)
		catalog := s.Catalog
		if language == "en" {
			catalog = game.LoadCatalogForLanguage(language)
		}
		writeJSON(w, 200, map[string]any{"board": catalog.Boards["westeros"], "houses": catalog.Houses, "houseNames": game.HouseNamesForLanguage(language), "units": catalog.Units, "orders": game.OrdersForLanguage(language), "rulesStage": game.RulesStage, "baseSetups": catalog.Setups["base-game"].PlayerSetups})
	})
	s.mux.HandleFunc("POST /api/rooms", s.create)
	s.mux.HandleFunc("GET /api/rooms/{room}", s.get)
	s.mux.HandleFunc("GET /api/rooms/{room}/lobby", s.lobby)
	s.mux.HandleFunc("POST /api/rooms/{room}/join", s.join)
	s.mux.HandleFunc("POST /api/rooms/{room}/commands", s.command)
	s.mux.HandleFunc("POST /api/rooms/{room}/preview", s.preview)
	s.mux.HandleFunc("GET /api/rooms/{room}/ws", s.socket)
	if assets != nil {
		s.mux.Handle("GET /", http.FileServer(http.FS(assets)))
	}
	return s
}
func (s *Server) LocalAssets(directory string) {
	if directory == "" {
		return
	}
	s.mux.Handle("GET /local-assets/", http.StripPrefix("/local-assets/", http.FileServer(http.Dir(directory))))
	s.mux.HandleFunc("GET /api/local-assets", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(directory + "/manifest.json")
		if err != nil {
			writeJSON(w, 200, map[string]any{"localOnly": true, "textures": map[string]string{}})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(data)
	})
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Language", requestLanguage(r))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Cache-Control", "no-store")
		if !sameOrigin(r) {
			writeJSON(w, 403, map[string]string{"error": "Запрос с другого сайта запрещён"})
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}
func sameOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host == r.Host
	}
	return true
}
func randomToken() string {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func playerID(r *http.Request) string {
	cookies := r.CookiesNamed(cookieName)
	if len(cookies) != 1 {
		return ""
	}
	token := cookies[0].Value
	if len(token) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(token); err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (s *Server) identity(w http.ResponseWriter, r *http.Request) string {
	if id := playerID(r); id != "" {
		return id
	}
	token := randomToken()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode, Path: "/", MaxAge: 30 * 86400})
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	language := w.Header().Get("Content-Language")
	if view, ok := v.(game.View); ok {
		v = game.LocalizeView(view, language)
	}
	if data, ok := v.(map[string]string); ok && data["error"] != "" {
		copy := map[string]string{}
		for key, value := range data {
			copy[key] = value
		}
		copy["error"] = game.TranslateSystemText(data["error"], language)
		v = copy
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func requestLanguage(r *http.Request) string {
	value := r.URL.Query().Get("lang")
	if value == "" {
		value = r.Header.Get("Accept-Language")
	}
	return game.NormalizeLanguage(value)
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("expected exactly one JSON object")
	}
	return nil
}
func validName(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 40 {
		return ""
	}
	return s
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var dto struct {
		Name     string         `json:"name"`
		Title    string         `json:"title"`
		House    string         `json:"house"`
		Settings *game.Settings `json:"settings"`
	}
	if e := decode(w, r, &dto); e != nil || validName(dto.Name) == "" {
		writeJSON(w, 400, map[string]string{"error": "Укажите имя до 40 символов"})
		return
	}
	player := s.identity(w, r)
	id := randomToken()[:16]
	title := validName(dto.Title)
	if title == "" {
		title = "Совет Вестероса"
	}
	room := game.NewRoom(id, title, player, validName(dto.Name))
	if dto.House != "" {
		if err := room.ChooseHouse(player, dto.House); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
	}
	if dto.Settings != nil {
		if err := dto.Settings.Validate(); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := room.Apply(player, game.Command{Type: "settings", Settings: dto.Settings}, s.Catalog); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
	}
	if e := s.Store.Create(room); e != nil {
		writeJSON(w, 500, map[string]string{"error": "Не удалось сохранить комнату"})
		return
	}
	writeJSON(w, 201, room.View(player, s.Catalog))
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	room, e := s.Store.Load(r.PathValue("room"))
	if e != nil {
		s.loadError(w, e)
		return
	}
	id := playerID(r)
	if room.Seat(id) == nil {
		writeJSON(w, 403, map[string]string{"error": "Войдите в комнату по приглашению"})
		return
	}
	writeJSON(w, 200, room.View(id, s.Catalog))
}

// Invitations expose only seat availability, never identities, names, orders,
// or any other private room state. Joining still validates under s.mu.
func (s *Server) lobby(w http.ResponseWriter, r *http.Request) {
	room, err := s.Store.Load(r.PathValue("room"))
	if err != nil {
		s.loadError(w, err)
		return
	}
	type seat struct {
		House     string `json:"house"`
		Available bool   `json:"available"`
	}
	seats := make([]seat, 0, len(room.Seats))
	for _, s := range room.Seats {
		seats = append(seats, seat{House: s.House, Available: room.Phase == "lobby" && s.Player == ""})
	}
	writeJSON(w, 200, struct {
		Phase string `json:"phase"`
		Seats []seat `json:"seats"`
	}{Phase: room.Phase, Seats: seats})
}
func (s *Server) loadError(w http.ResponseWriter, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Комната не найдена"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "Ошибка хранилища"})
	}
}
func (s *Server) join(w http.ResponseWriter, r *http.Request) {
	var dto struct {
		Name  string `json:"name"`
		House string `json:"house"`
	}
	if e := decode(w, r, &dto); e != nil || validName(dto.Name) == "" {
		writeJSON(w, 400, map[string]string{"error": "Укажите имя до 40 символов"})
		return
	}
	id := s.identity(w, r)
	s.mu.Lock()
	defer s.mu.Unlock()
	room, e := s.Store.Load(r.PathValue("room"))
	if e != nil {
		s.loadError(w, e)
		return
	}
	if e = room.Join(id, validName(dto.Name), dto.House); e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	old := room.Revision
	room.Revision++
	if e = s.Store.Save(room, old); e != nil {
		writeJSON(w, 409, map[string]string{"error": store.ErrConflict.Error()})
		return
	}
	s.broadcast(room)
	writeJSON(w, 200, room.View(id, s.Catalog))
}
func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	var command game.Command
	if err := decode(w, r, &command); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Некорректный запрос предпросмотра"})
		return
	}
	room, err := s.Store.Load(r.PathValue("room"))
	if err != nil {
		s.loadError(w, err)
		return
	}
	if room.Seat(playerID(r)) == nil {
		writeJSON(w, 403, map[string]string{"error": "Нет доступа к комнате"})
		return
	}
	if command.Revision != room.Revision {
		writeJSON(w, 409, map[string]string{"error": store.ErrConflict.Error()})
		return
	}
	now := time.Now().UTC()
	if room.Settings.HardTimer && !room.Clock.Deadline.IsZero() && !now.Before(room.Clock.Deadline) {
		writeJSON(w, 409, map[string]string{"error": "Время хода истекло"})
		return
	}
	result, err := room.PreviewAction(playerID(r), command, s.Catalog, now)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) command(w http.ResponseWriter, r *http.Request) {
	var c game.Command
	if e := decode(w, r, &c); e != nil {
		writeJSON(w, 400, map[string]string{"error": "Некорректная команда"})
		return
	}
	id := playerID(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	room, e := s.Store.Load(r.PathValue("room"))
	if e != nil {
		s.loadError(w, e)
		return
	}
	if room.Seat(id) == nil {
		writeJSON(w, 403, map[string]string{"error": "Нет доступа к комнате"})
		return
	}
	if room.Expire(time.Now().UTC(), s.Catalog) {
		revision := room.Revision
		room.Revision++
		if err := s.Store.Save(room, revision); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Не удалось сохранить истечение таймера"})
			return
		}
		s.broadcast(room)
		writeJSON(w, 409, map[string]string{"error": "Время хода истекло. Состояние комнаты обновилось"})
		return
	}
	if c.Revision != room.Revision {
		writeJSON(w, 409, map[string]string{"error": store.ErrConflict.Error()})
		return
	}
	copy := room.Clone()
	if e = copy.Apply(id, c, s.Catalog); e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	copy.Revision++
	if e = s.Store.Save(copy, room.Revision); e != nil {
		writeJSON(w, 409, map[string]string{"error": store.ErrConflict.Error()})
		return
	}
	s.broadcast(copy)
	writeJSON(w, 200, copy.View(id, s.Catalog))
}

// RunTimers owns hard deadlines independently from browser presence and requests.
func (s *Server) RunTimers(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := s.Expire(now.UTC()); err != nil {
				log.Printf("room timer: %v", err)
			}
			if err := s.AdvanceBots(now.UTC()); err != nil {
				log.Printf("room bot: %v", err)
			}
		}
	}
}
func (s *Server) AdvanceBots(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rooms, err := s.Store.ActiveActions()
	if err != nil {
		return err
	}
	for _, room := range rooms {
		revision := room.Revision
		if room.AdvanceBot(s.Catalog, now) {
			room.Revision++
			if err = s.Store.Save(room, revision); err != nil {
				return err
			}
			s.broadcast(room)
		}
	}
	return nil
}
func (s *Server) Expire(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rooms, err := s.Store.Due(now)
	if err != nil {
		return err
	}
	for _, room := range rooms {
		revision := room.Revision
		if room.Expire(now, s.Catalog) {
			room.Revision++
			if err = s.Store.Save(room, revision); err != nil {
				return err
			}
			s.broadcast(room)
		}
	}
	return nil
}
func (s *Server) broadcast(room *game.Room) {
	for sub := range s.subscribers[room.ID] {
		view := room.View(sub.player, s.Catalog)
		select {
		case sub.updates <- view:
		default:
			select {
			case <-sub.updates:
			default:
			}
			select {
			case sub.updates <- view:
			default:
			}
		}
	}
}
func (s *Server) socket(w http.ResponseWriter, r *http.Request) {
	id := playerID(r)
	s.mu.Lock()
	room, e := s.Store.Load(r.PathValue("room"))
	if e != nil || room.Seat(id) == nil {
		s.mu.Unlock()
		writeJSON(w, 403, map[string]string{"error": "Нет доступа к комнате"})
		return
	}
	upgrader := websocket.Upgrader{CheckOrigin: sameOrigin, HandshakeTimeout: 5 * time.Second}
	conn, e := upgrader.Upgrade(w, r, nil)
	if e != nil {
		s.mu.Unlock()
		return
	}
	sub := &subscriber{id, make(chan game.View, 1)}
	if s.subscribers[room.ID] == nil {
		s.subscribers[room.ID] = map[*subscriber]bool{}
	}
	s.subscribers[room.ID][sub] = true
	sub.updates <- room.View(id, s.Catalog)
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer conn.Close()
		conn.SetReadLimit(1024)
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(60 * time.Second)) })
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		delete(s.subscribers[room.ID], sub)
		if len(s.subscribers[room.ID]) == 0 {
			delete(s.subscribers, room.ID)
		}
		s.mu.Unlock()
	}()
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case v := <-sub.updates:
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if conn.WriteJSON(game.LocalizeView(v, requestLanguage(r))) != nil {
				return
			}
		case <-ticker.C:
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)) != nil {
				return
			}
		}
	}
}
