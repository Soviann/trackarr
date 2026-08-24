package repository_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDBWithSeason opens an in-memory test DB, runs all migrations, inserts
// one title and one season (id=1), and returns the DB. Used by the multi-part
// tests below.
func newTestDBWithSeason(t *testing.T) *sql.DB {
	t.Helper()
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Test Title", true)
	testutil.InsertSeason(t, db, titleID, 1)
	return db
}

func ptrInt(v int) *int       { return &v }
func ptrStr(v string) *string { return &v }

// --- Legacy single-mapping tests (now using Add instead of the removed Set) ---

func TestSeasonExternalIDs_AddAndGet(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Solo Leveling", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 2)

	repo := repository.NewSeasonExternalIDRepository(db)
	require.NoError(t, repo.Add(context.Background(), seasonID, "anilist", "166240"))

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

func TestSeasonExternalIDs_AddDeduplicates(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "JJK", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)

	repo := repository.NewSeasonExternalIDRepository(db)
	require.NoError(t, repo.Add(context.Background(), seasonID, "anilist", "113415"))
	require.NoError(t, repo.Add(context.Background(), seasonID, "anilist", "113415")) // dedup, no error

	parts, err := repo.ListParts(context.Background(), seasonID, "anilist")
	require.NoError(t, err)
	assert.Len(t, parts, 1)
}

// --- New multi-part tests ---

func TestAddAndListParts(t *testing.T) {
	db := newTestDBWithSeason(t) // existing helper creating a season id=1
	repo := repository.NewSeasonExternalIDRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Add(ctx, 1, repository.ProviderAniList, "100"))
	require.NoError(t, repo.Add(ctx, 1, repository.ProviderAniList, "200"))
	require.NoError(t, repo.Add(ctx, 1, repository.ProviderAniList, "100")) // dedup, no error

	parts, err := repo.ListParts(ctx, 1, repository.ProviderAniList)
	require.NoError(t, err)
	assert.Len(t, parts, 2)
}

func TestPartsOrderByStartDateThenSortOrder(t *testing.T) {
	db := newTestDBWithSeason(t)
	repo := repository.NewSeasonExternalIDRepository(db)
	ctx := context.Background()
	require.NoError(t, repo.Add(ctx, 1, repository.ProviderAniList, "B"))
	require.NoError(t, repo.Add(ctx, 1, repository.ProviderAniList, "A"))
	// no meta yet -> NULL start_date, sort_order: external_id tiebreak => A,B
	parts, _ := repo.ListParts(ctx, 1, repository.ProviderAniList)
	require.Len(t, parts, 2)
	assert.Equal(t, "A", parts[0].ExternalID)

	// give B an earlier start date -> B first
	require.NoError(t, repo.UpdatePartMeta(ctx, 1, repository.ProviderAniList, "B",
		ptrInt(80), ptrInt(12), ptrStr("2020-01-01")))
	parts, _ = repo.ListParts(ctx, 1, repository.ProviderAniList)
	assert.Equal(t, "B", parts[0].ExternalID)

	// explicit reorder wins
	require.NoError(t, repo.Reorder(ctx, 1, repository.ProviderAniList, []string{"A", "B"}))
	parts, _ = repo.ListParts(ctx, 1, repository.ProviderAniList)
	assert.Equal(t, "A", parts[0].ExternalID)
}
