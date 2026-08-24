package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/stretchr/testify/require"
)

// CreateWatchEvent inserts a single watch event and returns its new ID.
func CreateWatchEvent(t *testing.T, db *sql.DB, event *model.WatchEvent) int64 {
	t.Helper()
	var id int64
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		id, e = repository.NewWatchEventWriter(tx).Create(context.Background(), event)
		return e
	}))
	return id
}
