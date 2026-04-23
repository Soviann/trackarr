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

// GetOrCreateEpisode returns (creating if needed) the episode for a season+number.
func GetOrCreateEpisode(t *testing.T, db *sql.DB, seasonID int64, episodeNumber int) *model.Episode {
	t.Helper()
	var ep *model.Episode
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		ep, e = repository.NewEpisodeWriter(tx).GetOrCreate(context.Background(), seasonID, episodeNumber)
		return e
	}))
	return ep
}

// GetOrCreateEpisodeErr is the error-returning variant for tests that assert errors.
func GetOrCreateEpisodeErr(db *sql.DB, seasonID int64, episodeNumber int) (*model.Episode, error) {
	var ep *model.Episode
	err := database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		ep, e = repository.NewEpisodeWriter(tx).GetOrCreate(context.Background(), seasonID, episodeNumber)
		return e
	})
	return ep, err
}

// ToggleEpisodeWatched flips the watched state on an episode.
func ToggleEpisodeWatched(t *testing.T, db *sql.DB, id int64) *model.Episode {
	t.Helper()
	var ep *model.Episode
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		ep, e = repository.NewEpisodeWriter(tx).ToggleWatched(context.Background(), id)
		return e
	}))
	return ep
}

// BatchMarkEpisodesWatched marks multiple episodes as watched at `at`.
func BatchMarkEpisodesWatched(t *testing.T, db *sql.DB, ids []int64, at time.Time) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewEpisodeWriter(tx).BatchMarkWatched(context.Background(), ids, at)
	}))
}

// MarkEpisodeWatched marks a single episode as watched at `at`.
func MarkEpisodeWatched(t *testing.T, db *sql.DB, id int64, at time.Time) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewEpisodeWriter(tx).MarkWatched(context.Background(), id, at)
	}))
}

// UpsertEpisodesBatch inserts/updates a batch of episodes in one round-trip.
func UpsertEpisodesBatch(t *testing.T, db *sql.DB, seasonID int64, entries []repository.EpisodeUpsert) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewEpisodeWriter(tx).UpsertBatch(context.Background(), seasonID, entries)
	}))
}
