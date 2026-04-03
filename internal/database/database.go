package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open creates a new SQLite connection with WAL mode and foreign keys enabled.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite single-writer

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

// Migrate runs all pending migrations.
func Migrate(db *sql.DB) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
