package database_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_CreatesDatabase(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()
	assert.NotNil(t, db)
}

func TestOpen_AppliesPerformancePragmas(t *testing.T) {
	writeDB, readDB, err := database.Open(":memory:")
	require.NoError(t, err)
	defer writeDB.Close()
	defer readDB.Close()

	var syncMode int
	require.NoError(t, writeDB.QueryRow("PRAGMA synchronous").Scan(&syncMode))
	assert.Equal(t, 1, syncMode, "synchronous must be 1 (NORMAL)")

	var cacheSize int
	require.NoError(t, writeDB.QueryRow("PRAGMA cache_size").Scan(&cacheSize))
	assert.Equal(t, -32000, cacheSize, "cache_size must be -32000 (32MB)")

	var tempStore int
	require.NoError(t, writeDB.QueryRow("PRAGMA temp_store").Scan(&tempStore))
	assert.Equal(t, 2, tempStore, "temp_store must be 2 (MEMORY)")
}

func TestMigrate_CreatesAllTables(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = database.Migrate(db)
	require.NoError(t, err)

	// Verify all tables exist
	tables := []string{"titles", "title_names", "seasons", "episodes", "watch_events", "settings", "title_names_trigram_fts"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		assert.NoError(t, err, "table %s should exist", table)
	}
}

func TestWithTx_Commit(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.Migrate(db))

	err = database.WithTx(db, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "test_key", "test_value")
		return err
	})
	require.NoError(t, err)

	var val string
	err = db.QueryRow("SELECT value FROM settings WHERE key = ?", "test_key").Scan(&val)
	require.NoError(t, err)
	assert.Equal(t, "test_value", val)
}

func TestWithTx_Rollback(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.Migrate(db))

	err = database.WithTx(db, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "test_key", "test_value")
		if err != nil {
			return err
		}
		return fmt.Errorf("simulated error")
	})
	require.Error(t, err)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", "test_key").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestWithTxContext_CancelReleasesWriteConn reproduces the prod failure mode
// observed on 2026-04-17 where a Plex webhook transaction blocked on an
// unbounded downstream call (push/TMDB) and, because writeDB has
// MaxOpenConns=1, every subsequent write queued forever.
//
// Scenario: a transaction is started, the transaction function blocks on a
// network-like wait, the caller's context expires. WithTxContext must abort
// the transaction so the sole writeDB connection is released and a second
// concurrent write can complete instead of queuing indefinitely.
func TestWithTxContext_CancelReleasesWriteConn(t *testing.T) {
	// File-backed DB: when ctx-cancellation closes the underlying driver
	// connection, SQLite would otherwise spin up a fresh in-memory DB with no
	// schema. A temp file keeps the schema visible to reopened connections.
	dir := t.TempDir()
	db, _, err := database.Open(dir + "/test.db")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.Migrate(db))

	blockedTx := make(chan struct{})
	txDone := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		txDone <- database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
			if _, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "slow", "x"); err != nil {
				return err
			}
			close(blockedTx)
			// Simulate a hung HTTP call inside the tx (e.g. TMDB, push).
			// We wait on ctx so the cancellation is the only thing that unblocks us.
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	<-blockedTx
	cancel()

	select {
	case err := <-txDone:
		require.Error(t, err, "tx must surface the cancellation")
	case <-time.After(2 * time.Second):
		t.Fatal("tx did not return after ctx cancel — write conn would leak")
	}

	// Second writer must be able to acquire the write connection now that
	// the first tx was aborted. Prior to the fix, this call would queue on
	// the single writeDB connection forever.
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
			_, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "after_cancel", "y")
			return err
		})
	}()

	select {
	case err := <-writeDone:
		require.NoError(t, err, "writeDB conn was never released")
	case <-time.After(2 * time.Second):
		t.Fatal("second write starved — writeDB connection was not released after ctx cancel")
	}

	var val string
	err = db.QueryRow("SELECT value FROM settings WHERE key = ?", "after_cancel").Scan(&val)
	require.NoError(t, err)
	assert.Equal(t, "y", val)

	// And the cancelled tx must have rolled back.
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", "slow").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "cancelled tx must not have persisted data")
}

// TestWithTxContext_DeadlineAbortsWithoutStarvingOthers is the time-based
// equivalent of the test above: it confirms that a request with a short
// deadline (matching the 30s webhook timeout in production) cannot pin the
// write connection past the deadline.
func TestWithTxContext_DeadlineAbortsWithoutStarvingOthers(t *testing.T) {
	dir := t.TempDir()
	db, _, err := database.Open(dir + "/test.db")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.Migrate(db))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
			_, _ = tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "blocked", "x")
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	wg.Wait()

	require.True(t, errors.Is(ctx.Err(), context.DeadlineExceeded))

	err = database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "post_deadline", "y")
		return err
	})
	require.NoError(t, err)
}

func TestMigrate_AutoRecoversFromDirtyState(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/dirty.db"
	db, _, err := database.Open(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Initial migration to version 40 (an idempotent migration)
	require.NoError(t, database.MigrateTo(db, 40))

	// Manually set database to dirty state at version 40
	_, err = db.Exec("UPDATE schema_migrations SET dirty = 1")
	require.NoError(t, err)

	// Running Migrate again should auto-recover and succeed to latest version
	err = database.Migrate(db)
	require.NoError(t, err)

	var dirty bool
	err = db.QueryRow("SELECT dirty FROM schema_migrations").Scan(&dirty)
	require.NoError(t, err)
	assert.False(t, dirty)
}

type testInvalidator struct {
	mu            sync.Mutex
	invalidations [][]string
}

func (i *testInvalidator) Invalidate(tags ...string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.invalidations = append(i.invalidations, tags)
}

func TestWithTxContext_Invalidator(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.Migrate(db))

	inv := &testInvalidator{}
	database.SetInvalidator(inv)
	defer database.SetInvalidator(nil)

	t.Run("nominal: commit invalidates marked tags", func(t *testing.T) {
		inv.invalidations = nil
		err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
			database.MarkTxDirty(tx, database.TagTitles, database.TagStats)
			_, execErr := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "k1", "v1")
			return execErr
		})
		require.NoError(t, err)

		require.Len(t, inv.invalidations, 1)
		tags := inv.invalidations[0]
		assert.ElementsMatch(t, []string{database.TagTitles, database.TagStats}, tags)
	})

	t.Run("boundary: rollback does not invalidate", func(t *testing.T) {
		inv.invalidations = nil
		err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
			database.MarkTxDirty(tx, database.TagTitles)
			return errors.New("rollback intended")
		})
		require.Error(t, err)
		assert.Empty(t, inv.invalidations, "invalidator must not be called on rollback")
	})

	t.Run("boundary: commit without dirty tags does not invalidate", func(t *testing.T) {
		inv.invalidations = nil
		err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
			_, execErr := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "k2", "v2")
			return execErr
		})
		require.NoError(t, err)
		assert.Empty(t, inv.invalidations)
	})

	t.Run("manual CommitTx and RollbackTx", func(t *testing.T) {
		inv.invalidations = nil
		tx, err := db.Begin()
		require.NoError(t, err)
		database.MarkTxDirty(tx, database.TagCalendar)
		require.NoError(t, database.CommitTx(tx))
		require.Len(t, inv.invalidations, 1)
		assert.Equal(t, []string{database.TagCalendar}, inv.invalidations[0])

		inv.invalidations = nil
		txRollback, err := db.Begin()
		require.NoError(t, err)
		database.MarkTxDirty(txRollback, database.TagCalendar)
		require.NoError(t, database.RollbackTx(txRollback))
		assert.Empty(t, inv.invalidations)
	})
}
