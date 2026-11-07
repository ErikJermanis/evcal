// Package store keeps events and paired devices in SQLite.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure Go, so Linux builds need no cgo
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

// migrations run in order; PRAGMA user_version records how many have run.
// Only ever append to this list.
var migrations = []string{
	`CREATE TABLE events (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT '',
		start_date TEXT NOT NULL,
		start_time TEXT NOT NULL DEFAULT '',
		end_date TEXT NOT NULL DEFAULT '',
		end_time TEXT NOT NULL DEFAULT '',
		city TEXT NOT NULL DEFAULT '',
		venue TEXT NOT NULL DEFAULT '',
		organizer TEXT NOT NULL DEFAULT '',
		price TEXT NOT NULL DEFAULT '',
		audience TEXT NOT NULL DEFAULT '',
		why TEXT NOT NULL DEFAULT '',
		url TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE INDEX events_start ON events(start_date, start_time);
	CREATE TABLE devices (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		token_hash TEXT NOT NULL UNIQUE,
		created_at TEXT NOT NULL,
		last_seen_at TEXT NOT NULL,
		revoked_at TEXT
	);
	CREATE TABLE pairing_tokens (
		token_hash TEXT PRIMARY KEY,
		device_name TEXT NOT NULL,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		used_at TEXT
	);`,
}

// Open opens (creating if needed) the database at path and migrates it.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection serialises writes; traffic is a handful of devices.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// Ping checks that the database answers, for the health endpoint.
func (s *Store) Ping() error { return s.db.Ping() }

// Backup writes a consistent copy of the live database to dest, which must
// not exist yet. It is safe while the server is running.
func (s *Store) Backup(dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	_, err := s.db.Exec("VACUUM INTO ?", dest)
	return err
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }
