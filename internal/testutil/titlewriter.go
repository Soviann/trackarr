// Package testutil provides small helpers that wrap the compile-time-safe
// tx-only writers in boilerplate-free forms for tests. Tests use the pool
// handle directly and do not care about tx granularity, so wrapping each
// write in a short transaction is fine.
package testutil

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/require"
)

// CreateTitle inserts a title+names and returns the new ID.
func CreateTitle(t *testing.T, db *sql.DB, title *model.Title, names []model.TitleName) int64 {
	t.Helper()
	var id int64
	err := database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		id, e = repository.NewTitleWriter(tx).Create(context.Background(), title, names)
		return e
	})
	require.NoError(t, err)
	return id
}

// CreateTitleErr is the error-returning variant for tests that assert errors.
func CreateTitleErr(db *sql.DB, title *model.Title, names []model.TitleName) (int64, error) {
	var id int64
	err := database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		id, e = repository.NewTitleWriter(tx).Create(context.Background(), title, names)
		return e
	})
	return id, err
}

// UpdateTitle applies a partial update.
func UpdateTitle(t *testing.T, db *sql.DB, id int64, u repository.TitleUpdate) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(context.Background(), id, u)
	}))
}

// UpdateTitleErr is the error-returning variant.
func UpdateTitleErr(db *sql.DB, id int64, u repository.TitleUpdate) error {
	return database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(context.Background(), id, u)
	})
}

// UpdateTitleLastWatchedAt advances last_watched_at.
func UpdateTitleLastWatchedAt(t *testing.T, db *sql.DB, id int64, at time.Time) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).UpdateLastWatchedAt(context.Background(), id, at)
	}))
}

// ReplaceTitleNames wipes and re-inserts names.
func ReplaceTitleNames(t *testing.T, db *sql.DB, id int64, names []model.TitleName) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).ReplaceNames(context.Background(), id, names)
	}))
}

// MergeTitles consolidates source into dest.
func MergeTitles(t *testing.T, db *sql.DB, destID, sourceID int64, seasonOffset int) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Merge(context.Background(), destID, sourceID, seasonOffset)
	}))
}

// MergeTitlesErr is the error-returning variant.
func MergeTitlesErr(db *sql.DB, destID, sourceID int64, seasonOffset int) error {
	return database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Merge(context.Background(), destID, sourceID, seasonOffset)
	})
}

// DeleteTitle removes a title.
func DeleteTitle(t *testing.T, db *sql.DB, id int64) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Delete(context.Background(), id)
	}))
}

// BatchDeleteTitles removes multiple titles.
func BatchDeleteTitles(t *testing.T, db *sql.DB, ids []int64) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).BatchDelete(context.Background(), ids)
	}))
}

// BatchUpdateTitleStatus updates status for multiple titles.
func BatchUpdateTitleStatus(t *testing.T, db *sql.DB, ids []int64, status string) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).BatchUpdateStatus(context.Background(), ids, status)
	}))
}
