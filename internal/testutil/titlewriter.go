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

// InsertTitle inserts a minimal series title with one primary English name and
// returns its ID. Convenience wrapper around CreateTitle for tests that only
// care about having *a* title+season to attach child rows to.
func InsertTitle(t *testing.T, db *sql.DB, name string, isAnime bool) int64 {
	t.Helper()
	return CreateTitle(t, db,
		&model.Title{
			Type:        model.TitleTypeSeries,
			IsAnime:     isAnime,
			Year:        2024,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
		},
		[]model.TitleName{{Name: name, Language: "en", IsPrimary: true}},
	)
}

// InsertMovieTitle inserts a minimal anime movie title with an AniList ID
// and returns its ID.
func InsertMovieTitle(t *testing.T, db *sql.DB, name string, anilistID int64) int64 {
	t.Helper()
	return CreateTitle(t, db,
		&model.Title{
			Type:        model.TitleTypeMovie,
			IsAnime:     true,
			Year:        2024,
			AniListID:   &anilistID,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
		},
		[]model.TitleName{{Name: name, Language: "en", IsPrimary: true}},
	)
}

// SetTitleStatus updates titles.status for the given id.
func SetTitleStatus(t *testing.T, db *sql.DB, id int64, status string) {
	t.Helper()
	s := model.TitleStatus(status)
	UpdateTitle(t, db, id, repository.TitleUpdate{Status: &s})
}

// SetTitleRating updates titles.my_rating for the given id.
func SetTitleRating(t *testing.T, db *sql.DB, id int64, rating int) {
	t.Helper()
	UpdateTitle(t, db, id, repository.TitleUpdate{MyRating: &rating})
}

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
