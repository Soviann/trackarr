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

// SeedEpisode creates (or updates) one episode with an explicit air_date and
// watched state, returning it with its assigned ID. airDate is "YYYY-MM-DD"
// (or "" for unknown). Used by caught-up tests that need air-date control.
func SeedEpisode(t *testing.T, db *sql.DB, seasonID int64, episodeNumber int, airDate string, watched bool) *model.Episode {
	t.Helper()
	UpsertEpisodesBatch(t, db, seasonID, []repository.EpisodeUpsert{
		{EpisodeNumber: episodeNumber, Name: "", AirDate: airDate},
	})
	ep := GetOrCreateEpisode(t, db, seasonID, episodeNumber)
	if watched {
		MarkEpisodeWatched(t, db, ep.ID, time.Now())
		ep = GetOrCreateEpisode(t, db, seasonID, episodeNumber)
	}
	return ep
}

// MarkEpisodesWatched creates count episodes (numbered 1..count) in the season
// if missing, and flips watched=1 on all of them. Used by tests that fixture
// season progress without caring about per-episode timestamps.
func MarkEpisodesWatched(t *testing.T, db *sql.DB, seasonID int64, count int) {
	t.Helper()
	ids := make([]int64, 0, count)
	for i := 1; i <= count; i++ {
		ep := GetOrCreateEpisode(t, db, seasonID, i)
		ids = append(ids, ep.ID)
	}
	BatchMarkEpisodesWatched(t, db, ids, time.Now())
}
