package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"westeros-web/internal/game"
)

var ErrConflict = errors.New("Комната обновилась: повторите действие")

type Store struct{ DB *sql.DB }

func Open(filename string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS rooms(id TEXT PRIMARY KEY, revision INTEGER NOT NULL, snapshot TEXT NOT NULL);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = os.Chmod(filename, 0o600); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Due(now time.Time) ([]*game.Room, error) {
	rows, err := s.DB.Query(`SELECT snapshot FROM rooms WHERE json_extract(snapshot,'$.settings.hardTimer')=1 AND json_extract(snapshot,'$.phase') NOT IN ('lobby','finished','orders_revealed') AND julianday(json_extract(snapshot,'$.clock.deadline'))<=julianday(?)`, now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rooms := []*game.Room{}
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var room game.Room
		if err = json.Unmarshal(data, &room); err != nil {
			return nil, err
		}
		rooms = append(rooms, &room)
	}
	return rooms, rows.Err()
}
func (s *Store) ActiveActions() ([]*game.Room, error) {
	rows, err := s.DB.Query(`SELECT snapshot FROM rooms WHERE json_extract(snapshot,'$.phase') IN ('raven','westeros','wildling_bidding','wildling_ties','wildling_effects','raids','marches','combat','retreat','card_effect','influence_bidding','influence_ties','supply','mustering','consolidate_power')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rooms := []*game.Room{}
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var room game.Room
		if err = json.Unmarshal(data, &room); err != nil {
			return nil, err
		}
		rooms = append(rooms, &room)
	}
	return rooms, rows.Err()
}
func (s *Store) Create(r *game.Room) error {
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	_, e = s.DB.Exec("INSERT INTO rooms(id,revision,snapshot) VALUES(?,?,?)", r.ID, r.Revision, b)
	return e
}
func (s *Store) Load(id string) (*game.Room, error) {
	var b []byte
	if e := s.DB.QueryRow("SELECT snapshot FROM rooms WHERE id=?", id).Scan(&b); e != nil {
		return nil, e
	}
	var r game.Room
	if e := json.Unmarshal(b, &r); e != nil {
		return nil, e
	}
	return &r, nil
}
func (s *Store) Save(r *game.Room, expected int64) error {
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	result, e := s.DB.Exec("UPDATE rooms SET revision=?,snapshot=? WHERE id=? AND revision=?", r.Revision, b, r.ID, expected)
	if e != nil {
		return e
	}
	count, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if count != 1 {
		return ErrConflict
	}
	return nil
}
