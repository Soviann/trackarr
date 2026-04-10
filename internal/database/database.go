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

// DBTX is the common interface between *sql.DB and *sql.Tx.
// Repositories accept this so they can operate inside a transaction.
type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Open creates two SQLite connections: one for writes (MaxOpenConns=1) and one
// for reads (MaxOpenConns=4, read-only). SQLite WAL mode supports concurrent
// readers alongside a single writer, so separating connections prevents the
// background refresh from blocking HTTP list queries.
func Open(dsn string) (writeDB, readDB *sql.DB, err error) {
	base := dsn + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000"

	writeDB, err = sql.Open("sqlite3", base)
	if err != nil {
		return nil, nil, fmt.Errorf("open write database: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	if err = writeDB.Ping(); err != nil {
		writeDB.Close()
		return nil, nil, fmt.Errorf("ping write database: %w", err)
	}

	readDB, err = sql.Open("sqlite3", base+"&mode=ro")
	if err != nil {
		writeDB.Close()
		return nil, nil, fmt.Errorf("open read database: %w", err)
	}
	readDB.SetMaxOpenConns(4)
	if err = readDB.Ping(); err != nil {
		writeDB.Close()
		readDB.Close()
		return nil, nil, fmt.Errorf("ping read database: %w", err)
	}

	return writeDB, readDB, nil
}

// WithTx executes fn within a transaction. If fn returns an error, the
// transaction is rolled back; otherwise it is committed.
func WithTx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
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
