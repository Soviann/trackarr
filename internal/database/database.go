package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DBTX is the common interface between *sql.DB and *sql.Tx.
// Repositories accept this so they can operate inside a transaction.
// The Context variants let callers propagate request cancellation down
// to the driver, which is important for read-heavy endpoints (stats)
// where a client disconnect should release the read-pool slot.
type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var (
	_ DBTX = (*sql.DB)(nil)
	_ DBTX = (*sql.Tx)(nil)
)

// WriteDBTX is the contract for transactional write access. Only *sql.Tx
// satisfies it in practice, which makes "pass a pool where a tx was expected"
// a compile-time error instead of a runtime deadlock or lost write.
type WriteDBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var _ WriteDBTX = (*sql.Tx)(nil)

// Open creates two SQLite connections: one for writes (MaxOpenConns=1) and one
// for reads (MaxOpenConns=4, read-only). SQLite WAL mode supports concurrent
// readers alongside a single writer, so separating connections prevents the
// background refresh from blocking HTTP list queries.
func Open(dsn string) (writeDB, readDB *sql.DB, err error) {
	base := dsn + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=-32000&_temp_store=MEMORY"

	writeDB, err = sql.Open("sqlite3", base)
	if err != nil {
		return nil, nil, fmt.Errorf("open write database: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	if _, err = writeDB.Exec("PRAGMA temp_store = 2"); err != nil {
		writeDB.Close()
		return nil, nil, fmt.Errorf("set temp_store pragma: %w", err)
	}
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

const (
	TagTitles   = "titles"
	TagStats    = "stats"
	TagCalendar = "calendar"
)

// Invalidator defines the contract for invalidating domain tags.
type Invalidator interface {
	Invalidate(tags ...string)
}

var (
	txMu        sync.Mutex
	txDirtyTags = make(map[*sql.Tx]map[string]bool)
	invalidator Invalidator
)

// SetInvalidator registers the global invalidator to be notified on transaction commit.
func SetInvalidator(inv Invalidator) {
	txMu.Lock()
	defer txMu.Unlock()
	invalidator = inv
}

// GetInvalidator returns the currently registered Invalidator.
func GetInvalidator() Invalidator {
	txMu.Lock()
	defer txMu.Unlock()
	return invalidator
}

// MarkTxDirty associates one or more domain tags with an active transaction.
// When the transaction commits, the registered Invalidator is notified.
func MarkTxDirty(tx *sql.Tx, tags ...string) {
	if tx == nil || len(tags) == 0 {
		return
	}
	txMu.Lock()
	defer txMu.Unlock()
	m, ok := txDirtyTags[tx]
	if !ok {
		m = make(map[string]bool)
		txDirtyTags[tx] = m
	}
	for _, t := range tags {
		m[t] = true
	}
}

// CommitTx commits tx and invokes the registered Invalidator with all dirty tags.
func CommitTx(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		txMu.Lock()
		delete(txDirtyTags, tx)
		txMu.Unlock()
		return err
	}

	txMu.Lock()
	dirty := txDirtyTags[tx]
	inv := invalidator
	delete(txDirtyTags, tx)
	txMu.Unlock()

	if inv != nil && len(dirty) > 0 {
		tags := make([]string, 0, len(dirty))
		for tag := range dirty {
			tags = append(tags, tag)
		}
		inv.Invalidate(tags...)
	}
	return nil
}

// RollbackTx rolls back tx and clears any pending dirty tags.
func RollbackTx(tx *sql.Tx) error {
	txMu.Lock()
	delete(txDirtyTags, tx)
	txMu.Unlock()
	return tx.Rollback()
}

// WithTx executes fn within a transaction. If fn returns an error, the
// transaction is rolled back; otherwise it is committed. Prefer
// WithTxContext from new code paths so the transaction can be aborted when
// the caller's context is cancelled.
func WithTx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	return WithTxContext(context.Background(), db, fn)
}

// WithTxContext executes fn within a transaction tied to ctx. If ctx is
// cancelled (e.g. client disconnect, request timeout) the driver aborts the
// in-flight statement and the transaction is rolled back, freeing the write
// connection. If fn returns an error, the transaction is rolled back;
// otherwise it is committed.
func WithTxContext(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		txMu.Lock()
		delete(txDirtyTags, tx)
		txMu.Unlock()
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			log.Printf("database: rollback tx: %v", rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	txMu.Lock()
	dirty := txDirtyTags[tx]
	inv := invalidator
	delete(txDirtyTags, tx)
	txMu.Unlock()

	if inv != nil && len(dirty) > 0 {
		tags := make([]string, 0, len(dirty))
		for tag := range dirty {
			tags = append(tags, tag)
		}
		inv.Invalidate(tags...)
	}

	return nil
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

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		var dirtyVersion int
		if _, scanErr := fmt.Sscanf(err.Error(), "Dirty database version %d. Fix and force version.", &dirtyVersion); scanErr == nil && dirtyVersion > 0 {
			prevVersion := dirtyVersion - 1
			log.Printf("database: dirty state detected at version %d, forcing previous version (%d) and retrying...", dirtyVersion, prevVersion)
			if err := m.Force(prevVersion); err != nil {
				return fmt.Errorf("force migration: %w", err)
			}
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				return fmt.Errorf("retry run migrations: %w", err)
			}
			return nil
		}
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// MigrateTo runs migrations up to the specified target version.
func MigrateTo(db *sql.DB, target uint) error {
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

	if err := m.Migrate(target); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate to version %d: %w", target, err)
	}
	return nil
}

// ResolveDBPath returns the path to the SQLite database.
// To ensure 100% transparent backwards compatibility with existing installations,
// it checks if plextracker.db already exists. If so, it continues using it.
// Otherwise, it defaults to trackarr.db.
func ResolveDBPath(dataDir string) string {
	legacyPath := filepath.Join(dataDir, "plextracker.db")
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath
	}
	return filepath.Join(dataDir, "trackarr.db")
}
