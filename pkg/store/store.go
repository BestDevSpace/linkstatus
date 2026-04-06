package store

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dataDir string) (*Store, error) {
	dbPath := filepath.Join(dataDir, "linkstatus.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func initSchema(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS probe_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL CHECK(status IN ('up', 'down')),
		rating INTEGER NOT NULL CHECK(rating >= 1 AND rating <= 5),
		latency_ms REAL NOT NULL,
		ping_latency_ms REAL,
		dns_latency_ms REAL,
		error_message TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_probe_logs_timestamp ON probe_logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_probe_logs_status ON probe_logs(status);
	`
	_, err := db.Exec(query)
	return err
}
