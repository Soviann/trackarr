package testutil

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/stretchr/testify/require"
)

// ReplaceGenres replaces the genres of a title inside a fresh transaction.
func ReplaceGenres(t *testing.T, db *sql.DB, titleID int64, genres []string) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewGenreWriter(tx).ReplaceForTitle(context.Background(), titleID, genres)
	}))
}
