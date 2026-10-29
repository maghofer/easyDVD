package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"easydvd/internal/model"
	"easydvd/internal/platform"
	_ "modernc.org/sqlite"
)

type Store struct {
	db        *sql.DB
	unlock    func()
	once      sync.Once
	closeErr  error
	directory string
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	unlock, err := platform.LockFile(path + ".lock")
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		unlock()
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS jobs (id TEXT PRIMARY KEY, position INTEGER NOT NULL, payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS preferences (key TEXT PRIMARY KEY, payload BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS menu_presets (name TEXT PRIMARY KEY COLLATE NOCASE, payload BLOB NOT NULL);
 PRAGMA user_version=1;`)
	if err != nil {
		db.Close()
		unlock()
		return nil, err
	}
	return &Store{db: db, unlock: unlock, directory: filepath.Dir(path)}, nil
}
func (s *Store) Directory() string { return s.directory }
func (s *Store) Close() error {
	s.once.Do(func() { s.closeErr = s.db.Close(); s.unlock() })
	return s.closeErr
}
func (s *Store) Load() ([]model.Job, error) {
	rows, err := s.db.Query(`SELECT payload FROM jobs ORDER BY position,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []model.Job
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var j model.Job
		if err = json.Unmarshal(b, &j); err != nil {
			return nil, fmt.Errorf("invalid saved job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
func (s *Store) Save(jobs []model.Job) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM jobs`); err != nil {
		return err
	}
	for i, j := range jobs {
		b, e := json.Marshal(j)
		if e != nil {
			return e
		}
		if _, err = tx.Exec(`INSERT INTO jobs(id,position,payload) VALUES(?,?,?)`, j.ID, i, b); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Settings(fallback model.Settings) (model.Settings, error) {
	var b []byte
	err := s.db.QueryRow(`SELECT payload FROM preferences WHERE key='settings'`).Scan(&b)
	if err == sql.ErrNoRows {
		return fallback, nil
	}
	if err != nil {
		return fallback, err
	}
	// New pending jobs adopt current defaults; saved started jobs retain theirs.
	v := fallback
	err = json.Unmarshal(b, &v)
	return v, err
}
func (s *Store) SaveSettings(v model.Settings) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO preferences(key,payload) VALUES('settings',?) ON CONFLICT(key) DO UPDATE SET payload=excluded.payload`, b)
	return err
}
