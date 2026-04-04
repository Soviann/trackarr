package database_test

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_CreatesDatabase(t *testing.T) {
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()
	assert.NotNil(t, db)
}

func TestMigrate_CreatesAllTables(t *testing.T) {
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = database.Migrate(db)
	require.NoError(t, err)

	// Verify all tables exist
	tables := []string{"titles", "title_names", "seasons", "episodes", "watch_events", "settings"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		assert.NoError(t, err, "table %s should exist", table)
	}
}

func TestWithTx_Commit(t *testing.T) {
	db, err := database.Open(":memory:")
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
	db, err := database.Open(":memory:")
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
