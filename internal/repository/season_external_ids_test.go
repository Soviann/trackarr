package repository_test

import (
	"context"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeasonExternalIDs_SetAndGet(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Solo Leveling", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 2)

	repo := repository.NewSeasonExternalIDRepository(db)
	require.NoError(t, repo.Set(context.Background(), seasonID, "anilist", "166240"))

	got, err := repo.Get(context.Background(), seasonID, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "166240", got)
}

func TestSeasonExternalIDs_GetMissingReturnsEmpty(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := repository.NewSeasonExternalIDRepository(db)
	got, err := repo.Get(context.Background(), 9999, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestSeasonExternalIDs_SetUpsertsOnConflict(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "JJK", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	repo := repository.NewSeasonExternalIDRepository(db)
	require.NoError(t, repo.Set(context.Background(), seasonID, "anilist", "113415"))
	require.NoError(t, repo.Set(context.Background(), seasonID, "anilist", "999999"))

	got, _ := repo.Get(context.Background(), seasonID, "anilist")
	assert.Equal(t, "999999", got)
}

func TestSeasonExternalIDs_ListForTitle(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "JJK", true)
	s1 := testutil.InsertSeason(t, db, titleID, 1)
	s2 := testutil.InsertSeason(t, db, titleID, 2)

	repo := repository.NewSeasonExternalIDRepository(db)
	require.NoError(t, repo.Set(context.Background(), s1, "anilist", "113415"))
	require.NoError(t, repo.Set(context.Background(), s2, "anilist", "145064"))

	got, err := repo.ListForTitle(context.Background(), titleID, "anilist")
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{s1: "113415", s2: "145064"}, got)
}
