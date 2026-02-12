package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id          TEXT PRIMARY KEY,
			name        TEXT UNIQUE NOT NULL,
			domain      TEXT UNIQUE NOT NULL,
			status      TEXT DEFAULT 'stopped',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS services (
			id              TEXT PRIMARY KEY,
			project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			template_name   TEXT NOT NULL,
			name            TEXT NOT NULL,
			enabled         BOOLEAN DEFAULT 1,
			config          TEXT DEFAULT '{}',
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS port_allocations (
			id              TEXT PRIMARY KEY,
			service_id      TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
			internal_port   INTEGER NOT NULL,
			external_port   INTEGER NOT NULL,
			protocol        TEXT DEFAULT 'tcp',
			UNIQUE(external_port, protocol)
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			email         TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token      TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}

	// Safe column additions (SQLite doesn't support IF NOT EXISTS for ALTER TABLE)
	alterMigrations := []string{
		`ALTER TABLE projects ADD COLUMN path TEXT DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN user_id TEXT DEFAULT ''`,
		`ALTER TABLE projects ADD COLUMN ssl_enabled BOOLEAN DEFAULT 0`,
	}
	for _, m := range alterMigrations {
		db.Exec(m) // Ignore errors — column may already exist
	}

	return nil
}
