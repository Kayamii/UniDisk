package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

// Store wraps the SQLite database and exposes typed query methods.
type Store struct {
	db *sql.DB
}

// Open opens (creating parent dirs as needed) the SQLite database at path and
// applies the embedded schema. The schema uses IF NOT EXISTS, so Open is
// idempotent and safe to call on every boot.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}

	// _busy_timeout avoids "database is locked" under brief WAL contention.
	dsn := path + "?_busy_timeout=5000&_foreign_keys=on"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles one writer at a time; a single connection avoids
	// surprising "locked" errors while keeping reads fast under WAL.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for callers needing transactions.
func (s *Store) DB() *sql.DB { return s.db }
