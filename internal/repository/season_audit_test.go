package repository_test

import (
	"database/sql"
	"sort"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// confirmedSeries inserts a confirmed series title with the given external IDs
// (any may be nil) and returns its ID.
func confirmedSeries(t *testing.T, db *sql.DB, name string, imdb *string, tmdb, tvdb *int64) int64 {
	t.Helper()
	return testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		IMDBID:      imdb,
		TMDBID:      tmdb,
		TVDBID:      tvdb,
	}, []model.TitleName{{Name: name, Language: "en", IsPrimary: true}})
}

func TestSeasonAudit_DuplicateSeriesGroups_SingleSharedIMDB(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewSeasonAuditRepository(db)

	shared := "tt1586814"
	other := "tt9999999"
	idA := confirmedSeries(t, db, "Series A", &shared, nil, nil)
	idB := confirmedSeries(t, db, "Series B", &shared, nil, nil)
	idLone := confirmedSeries(t, db, "Lone Series", &other, nil, nil)

	groups, err := repo.DuplicateSeriesGroups()
	require.NoError(t, err)
	require.Len(t, groups, 1)

	got := titleIDs(groups[0].Titles)
	assert.ElementsMatch(t, []int64{idA, idB}, got)
	assert.NotContains(t, got, idLone)
	assert.Equal(t, "imdb:"+shared, groups[0].SharedID)
}

func TestSeasonAudit_DuplicateSeriesGroups_ConnectedComponent(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewSeasonAuditRepository(db)

	// A shares imdb with B; B shares tmdb with C → all three in one component.
	sharedIMDB := "tt0000001"
	var sharedTMDB int64 = 555
	idA := confirmedSeries(t, db, "A", &sharedIMDB, nil, nil)
	idB := confirmedSeries(t, db, "B", &sharedIMDB, &sharedTMDB, nil)
	idC := confirmedSeries(t, db, "C", nil, &sharedTMDB, nil)

	groups, err := repo.DuplicateSeriesGroups()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.ElementsMatch(t, []int64{idA, idB, idC}, titleIDs(groups[0].Titles))
}

func TestSeasonAudit_DuplicateSeriesGroups_IgnoresMoviesAndUnconfirmed(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewSeasonAuditRepository(db)

	shared := "tt1234567"
	// One confirmed series with the shared id, plus a movie and an unconfirmed
	// series carrying the same id — neither should pull the series into a group.
	_ = confirmedSeries(t, db, "Real Series", &shared, nil, nil)

	pending := model.MatchStatusPendingReview
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		IMDBID:      &shared,
	}, []model.TitleName{{Name: "Movie", Language: "en", IsPrimary: true}})
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: pending,
		IMDBID:      &shared,
	}, []model.TitleName{{Name: "Unconfirmed Series", Language: "en", IsPrimary: true}})

	groups, err := repo.DuplicateSeriesGroups()
	require.NoError(t, err)
	assert.Empty(t, groups, "only one confirmed series carries the id → no duplicate group")
}

func TestSeasonAudit_DismissRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewSeasonAuditRepository(db)

	shared := "tt7777777"
	source := confirmedSeries(t, db, "Source", &shared, nil, nil)
	target := confirmedSeries(t, db, "Target", &shared, nil, nil)

	dismissed, err := repo.IsDismissed(source, target)
	require.NoError(t, err)
	assert.False(t, dismissed)

	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewSeasonAuditWriter(tx).Dismiss(t.Context(), source, target)
	}))

	dismissed, err = repo.IsDismissed(source, target)
	require.NoError(t, err)
	assert.True(t, dismissed)

	// Reversed pair is a distinct key — still not dismissed.
	dismissed, err = repo.IsDismissed(target, source)
	require.NoError(t, err)
	assert.False(t, dismissed)

	// Idempotent: a second Dismiss must not error.
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewSeasonAuditWriter(tx).Dismiss(t.Context(), source, target)
	}))
}

func titleIDs(titles []model.Title) []int64 {
	ids := make([]int64, len(titles))
	for i, t := range titles {
		ids[i] = t.ID
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	return ids
}
