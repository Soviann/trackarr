package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEpisodeWriter_MarkAllWatchedForTitle(t *testing.T) {
	db := setupTestDB(t)
	episodeRepo := repository.NewEpisodeRepository(db)

	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, Year: 2024,
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "T", Language: "en", IsPrimary: true}})

	s1 := testutil.GetOrCreateSeason(t, db, titleID, 1)
	s2 := testutil.GetOrCreateSeason(t, db, titleID, 2)
	ep1 := testutil.GetOrCreateEpisode(t, db, s1.ID, 1)
	_ = testutil.GetOrCreateEpisode(t, db, s1.ID, 2)
	_ = testutil.GetOrCreateEpisode(t, db, s2.ID, 1)
	// Add future episode and TBA episode — neither should be marked watched
	_ = testutil.SeedEpisode(t, db, s2.ID, 2, "2099-01-01", false)
	testutil.UpsertEpisodesBatch(t, db, s2.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 3, Name: "TBA", AirDate: ""},
	})

	// ep1 already watched two days ago.
	past := time.Now().UTC().Add(-48 * time.Hour)
	testutil.MarkEpisodeWatched(t, db, ep1.ID, past)

	var count int64
	now := time.Now().UTC()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		count, e = repository.NewEpisodeWriter(tx).MarkAllWatchedForTitle(context.Background(), titleID, now)
		return e
	}))

	assert.Equal(t, int64(2), count, "only the 2 unwatched, aired, non-TBA episodes are newly marked")

	// S1E1, S1E2, S2E1 are watched. S2E2 (future) and S2E3 (TBA) remain unwatched.
	s1Eps, _ := episodeRepo.GetBySeasonID(s1.ID)
	for _, e := range s1Eps {
		assert.Truef(t, e.Watched, "S1 E%d must be watched", e.Episode)
	}
	s2Eps, _ := episodeRepo.GetBySeasonID(s2.ID)
	for _, e := range s2Eps {
		if e.Episode == 1 {
			assert.True(t, e.Watched, "S2 E1 must be watched")
		} else {
			assert.Falsef(t, e.Watched, "S2 E%d (future/TBA) must NOT be marked watched", e.Episode)
		}
	}

	// The already-watched episode keeps its original first_watched_at.
	for _, e := range s1Eps {
		if e.Episode == 1 {
			require.NotNil(t, e.FirstWatchedAt)
			assert.WithinDuration(t, past, *e.FirstWatchedAt, time.Second, "preserve original first_watched_at on rewatch-marked episode")
		}
	}
}

func TestTitleWriter_AddWatchMinutesForEpisodes(t *testing.T) {
	db := setupTestDB(t)
	titleRepo := repository.NewTitleRepository(db)

	runtime := 24
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, Year: 2024,
		Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed,
		Runtime: &runtime, TotalWatchMinutes: 48,
	}, []model.TitleName{{Name: "T", Language: "en", IsPrimary: true}})

	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).AddWatchMinutesForEpisodes(context.Background(), titleID, 3)
	}))

	title, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	assert.Equal(t, 48+3*24, title.TotalWatchMinutes, "watchtime grows by episodeCount * runtime")

	// A zero count is a no-op.
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).AddWatchMinutesForEpisodes(context.Background(), titleID, 0)
	}))
	title, _ = titleRepo.GetByID(titleID)
	assert.Equal(t, 48+3*24, title.TotalWatchMinutes, "zero count leaves watchtime unchanged")
}
