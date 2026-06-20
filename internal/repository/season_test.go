package repository_test

import (
	"context"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchedEpisodeNumbers(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Test Title", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	// Seed episodes 1..5; mark 1, 2 and 4 watched (out of order to prove sort).
	testutil.SeedEpisode(t, db, seasonID, 3, "", false)
	testutil.SeedEpisode(t, db, seasonID, 1, "", true)
	testutil.SeedEpisode(t, db, seasonID, 5, "", false)
	testutil.SeedEpisode(t, db, seasonID, 4, "", true)
	testutil.SeedEpisode(t, db, seasonID, 2, "", true)

	repo := repository.NewSeasonRepository(db)
	nums, err := repo.WatchedEpisodeNumbers(context.Background(), seasonID)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 4}, nums)
}

func TestWatchedEpisodeNumbers_NoneWatched(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Test Title", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	testutil.SeedEpisode(t, db, seasonID, 1, "", false)

	repo := repository.NewSeasonRepository(db)
	nums, err := repo.WatchedEpisodeNumbers(context.Background(), seasonID)
	require.NoError(t, err)
	assert.Empty(t, nums)
}
