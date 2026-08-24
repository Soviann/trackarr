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

// InsertSeason creates a season for (titleID, seasonNumber) and returns its
// ID — convenience wrapper when only the ID is needed.
func InsertSeason(t *testing.T, db *sql.DB, titleID int64, seasonNumber int) int64 {
	t.Helper()
	return GetOrCreateSeason(t, db, titleID, seasonNumber).ID
}

// GetOrCreateSeason returns (creating if needed) the season for a title+number.
func GetOrCreateSeason(t *testing.T, db *sql.DB, titleID int64, seasonNumber int) *model.Season {
	t.Helper()
	var s *model.Season
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		s, e = repository.NewSeasonWriter(tx).GetOrCreate(context.Background(), titleID, seasonNumber)
		return e
	}))
	return s
}

// UpsertSeason creates or updates a season's total_episodes.
func UpsertSeason(t *testing.T, db *sql.DB, titleID int64, seasonNumber, totalEpisodes int) *model.Season {
	t.Helper()
	var s *model.Season
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		s, e = repository.NewSeasonWriter(tx).Upsert(context.Background(), titleID, seasonNumber, totalEpisodes)
		return e
	}))
	return s
}

// UpdateSeasonTotalEpisodes writes total_episodes for a season.
func UpdateSeasonTotalEpisodes(t *testing.T, db *sql.DB, id int64, total int) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewSeasonWriter(tx).UpdateTotalEpisodes(context.Background(), id, total)
	}))
}

// SetSeasonEpisodeCount is a shorter alias for UpdateSeasonTotalEpisodes used
// by tests that set up season progress fixtures.
func SetSeasonEpisodeCount(t *testing.T, db *sql.DB, id int64, total int) {
	t.Helper()
	UpdateSeasonTotalEpisodes(t, db, id, total)
}
