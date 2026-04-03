package database_test

import (
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
