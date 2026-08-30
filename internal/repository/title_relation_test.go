package repository_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTitleRelations_UpsertBatchAndGet(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "My Hero Academia", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 2)

	// Insert a movie that matches anilist_id 101347 in local library
	movieID := testutil.InsertTitle(t, db, "Two Heroes", true)
	_, err := db.Exec(`UPDATE titles SET anilist_id = 101347, type = 'movie', status = 'completed', my_rating = 8 WHERE id = ?`, movieID)
	require.NoError(t, err)

	repo := repository.NewTitleRelationRepository(db)
	ctx := context.Background()

	sid := seasonID
	year := 2018
	score := 82
	duration := 96
	overview := "All Might and Deku visit I-Island."

	relations := []model.TitleRelation{
		{
			TitleID:      titleID,
			SeasonID:     &sid,
			Provider:     "anilist",
			ExternalID:   101347,
			RelationType: model.RelationSideStory,
			Format:       "MOVIE",
			Title:        "My Hero Academia: Two Heroes",
			Year:         &year,
			Score:        &score,
			Duration:     &duration,
			Overview:     &overview,
			SortOrder:    1,
		},
		{
			TitleID:      titleID,
			SeasonID:     &sid,
			Provider:     "anilist",
			ExternalID:   98565,
			RelationType: model.RelationSideStory,
			Format:       "OVA",
			Title:        "Training of the Dead",
			SortOrder:    2,
		},
	}

	err = database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleRelationWriter(tx).UpsertBatch(ctx, titleID, relations)
	})
	require.NoError(t, err)

	// Query by title ID
	got, err := repo.GetByTitleID(ctx, titleID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// First relation should have resolved local matched movie
	assert.Equal(t, int64(101347), got[0].ExternalID)
	assert.Equal(t, "My Hero Academia: Two Heroes", got[0].Title)
	assert.Equal(t, model.RelationSideStory, got[0].RelationType)
	assert.Equal(t, "MOVIE", got[0].Format)
	require.NotNil(t, got[0].MatchedTitleID)
	assert.Equal(t, movieID, *got[0].MatchedTitleID)
	require.NotNil(t, got[0].MatchedStatus)
	assert.Equal(t, model.TitleStatusCompleted, *got[0].MatchedStatus)
	require.NotNil(t, got[0].MatchedRating)
	assert.Equal(t, 8, *got[0].MatchedRating)
	require.NotNil(t, got[0].SeasonNumber)
	assert.Equal(t, 2, *got[0].SeasonNumber)

	// Second relation (OVA) is not in local library
	assert.Equal(t, int64(98565), got[1].ExternalID)
	assert.Nil(t, got[1].MatchedTitleID)

	// Query by season ID
	seasonRels, err := repo.GetBySeasonID(ctx, seasonID)
	require.NoError(t, err)
	require.Len(t, seasonRels, 2)
}

func TestTitleRelations_UpsertBatchCleansUpStale(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "My Hero Academia", true)
	ctx := context.Background()
	repo := repository.NewTitleRelationRepository(db)

	initial := []model.TitleRelation{
		{TitleID: titleID, ExternalID: 101, RelationType: model.RelationSideStory, Format: "MOVIE", Title: "Movie 1"},
		{TitleID: titleID, ExternalID: 102, RelationType: model.RelationSideStory, Format: "MOVIE", Title: "Movie 2"},
	}
	err := database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleRelationWriter(tx).UpsertBatch(ctx, titleID, initial)
	})
	require.NoError(t, err)

	got, err := repo.GetByTitleID(ctx, titleID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Update with only Movie 2
	updated := []model.TitleRelation{
		{TitleID: titleID, ExternalID: 102, RelationType: model.RelationSideStory, Format: "MOVIE", Title: "Movie 2 Updated"},
	}
	err = database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleRelationWriter(tx).UpsertBatch(ctx, titleID, updated)
	})
	require.NoError(t, err)

	got, err = repo.GetByTitleID(ctx, titleID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(102), got[0].ExternalID)
	assert.Equal(t, "Movie 2 Updated", got[0].Title)
}

func TestTitleRelations_DeleteForTitle(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "My Hero Academia", true)
	ctx := context.Background()
	repo := repository.NewTitleRelationRepository(db)

	initial := []model.TitleRelation{
		{TitleID: titleID, ExternalID: 101, RelationType: model.RelationSideStory, Format: "MOVIE", Title: "Movie 1"},
	}
	err := database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleRelationWriter(tx).UpsertBatch(ctx, titleID, initial)
	})
	require.NoError(t, err)

	require.NoError(t, repo.DeleteForTitle(ctx, titleID))
	got, err := repo.GetByTitleID(ctx, titleID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestTitleRelations_TMDB_And_TVDB_Matching(t *testing.T) {
	db := testutil.NewTestDB(t)
	parentID := testutil.InsertTitle(t, db, "Harry Potter 1", false)
	localMovieID := testutil.InsertTitle(t, db, "Harry Potter 2", false)
	localSeriesID := testutil.InsertTitle(t, db, "Better Call Saul", false)
	ctx := context.Background()

	// Assign TMDB and TVDB IDs
	_, err := db.ExecContext(ctx, "UPDATE titles SET tmdb_id = 672, status = 'completed', my_rating = 9 WHERE id = ?", localMovieID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "UPDATE titles SET tvdb_id = 273181, status = 'plan_to_watch' WHERE id = ?", localSeriesID)
	require.NoError(t, err)

	repo := repository.NewTitleRelationRepository(db)
	y2002, y2015, y2004 := 2002, 2015, 2004
	relations := []model.TitleRelation{
		{
			TitleID:      parentID,
			Provider:     "tmdb",
			ExternalID:   672,
			RelationType: model.RelationCollection,
			Format:       "MOVIE",
			Title:        "Harry Potter and the Chamber of Secrets",
			Year:         &y2002,
		},
		{
			TitleID:      parentID,
			Provider:     "tvdb",
			ExternalID:   273181,
			RelationType: model.RelationSpinOff,
			Format:       "TV",
			Title:        "Better Call Saul",
			Year:         &y2015,
		},
		{
			TitleID:      parentID,
			Provider:     "tmdb",
			ExternalID:   999999, // Unmatched
			RelationType: model.RelationCollection,
			Format:       "MOVIE",
			Title:        "Harry Potter 3",
			Year:         &y2004,
		},
	}

	err = database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleRelationWriter(tx).UpsertBatch(ctx, parentID, relations)
	})
	require.NoError(t, err)

	got, err := repo.GetByTitleID(ctx, parentID)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// TMDB matched
	assert.Equal(t, "tmdb", got[0].Provider)
	assert.Equal(t, &localMovieID, got[0].MatchedTitleID)
	require.NotNil(t, got[0].MatchedStatus)
	assert.Equal(t, model.TitleStatusCompleted, *got[0].MatchedStatus)
	require.NotNil(t, got[0].MatchedRating)
	assert.Equal(t, 9, *got[0].MatchedRating)

	// TVDB matched
	assert.Equal(t, "tvdb", got[1].Provider)
	assert.Equal(t, &localSeriesID, got[1].MatchedTitleID)
	require.NotNil(t, got[1].MatchedStatus)
	assert.Equal(t, model.TitleStatusPlanToWatch, *got[1].MatchedStatus)

	// Unmatched
	assert.Equal(t, int64(999999), got[2].ExternalID)
	assert.Nil(t, got[2].MatchedTitleID)
}

