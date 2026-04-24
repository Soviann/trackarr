// Package testutil provides small helpers that wrap the compile-time-safe
// tx-only writers in boilerplate-free forms for tests. Tests use the pool
// handle directly and do not care about tx granularity, so wrapping each
// write in a short transaction is fine.
package testutil

import (
	"database/sql"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/stretchr/testify/require"
)

// NewTestDB opens an in-memory SQLite write-DB with all migrations applied.
// The underlying connection is closed when t ends.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })
	return db
}
