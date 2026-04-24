package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Backfill of an anime with a title-level AniList ID must stamp the S1 season
// mapping so the AniList push queue can resolve the entry without extra
// manual linking.
func TestBackfillPreviousEpisodes_StampsAniListOnSeasonOne(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Demon Slayer", true)
	anilistID := int64(113415)

	s1 := testutil.InsertSeason(t, db, titleID, 1)

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		// Trigger on S1E5 → backfill creates episodes 1-4 on the existing S1.
		return service.BackfillPreviousEpisodes(ctx, tx, titleID, &anilistID, nil, 1, 5, time.Now().UTC())
	}))

	got, err := testutil.GetSeasonExternalID(t, db, s1, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "113415", got)
}

// No AniList ID on the title → no mapping row.
func TestBackfillPreviousEpisodes_SkipsStampWhenTitleHasNoAniListID(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Some Show", false)
	s1 := testutil.InsertSeason(t, db, titleID, 1)

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return service.BackfillPreviousEpisodes(ctx, tx, titleID, nil, nil, 1, 5, time.Now().UTC())
	}))

	got, err := testutil.GetSeasonExternalID(t, db, s1, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

// An existing user-confirmed mapping on S1 must not be overwritten.
func TestBackfillPreviousEpisodes_PreservesExistingMapping(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Demon Slayer", true)
	anilistID := int64(999) // wrong — user had already linked the correct one below.

	s1 := testutil.InsertSeason(t, db, titleID, 1)
	testutil.InsertSeasonExternalID(t, db, s1, "anilist", "113415")

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return service.BackfillPreviousEpisodes(ctx, tx, titleID, &anilistID, nil, 1, 5, time.Now().UTC())
	}))

	got, err := testutil.GetSeasonExternalID(t, db, s1, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "113415", got, "existing mapping survives the backfill")
}

// S1 stamping also applies when the backfill walks previous TMDB seasons: the
// trigger is S2E1 with TMDB returning S1 as a previous season.
func TestBackfillPreviousEpisodes_StampsAniListOnPreviousS1(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Demon Slayer", true)
	anilistID := int64(113415)

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return service.BackfillPreviousEpisodes(
			ctx, tx, titleID, &anilistID,
			[]service.TMDBSeasonInfo{{Number: 1, EpisodeCount: 12}},
			2, 1, time.Now().UTC(),
		)
	}))

	var s1 int64
	require.NoError(t, db.QueryRow(`SELECT id FROM seasons WHERE title_id = ? AND season_number = 1`, titleID).Scan(&s1))
	got, err := testutil.GetSeasonExternalID(t, db, s1, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "113415", got)
}

// Only S1 gets the title-level mapping — later seasons need their own link.
func TestBackfillPreviousEpisodes_DoesNotStampLaterSeasons(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Demon Slayer", true)
	anilistID := int64(113415)

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return service.BackfillPreviousEpisodes(ctx, tx, titleID, &anilistID, nil, 2, 3, time.Now().UTC())
	}))

	var s2 int64
	require.NoError(t, db.QueryRow(`SELECT id FROM seasons WHERE title_id = ? AND season_number = 2`, titleID).Scan(&s2))
	got, err := testutil.GetSeasonExternalID(t, db, s2, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "", got, "title-level AniList ID describes S1 only")
}
