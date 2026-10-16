package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/cache"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLibraryHandler(t *testing.T) (*handler.LibraryHandler, *sql.DB, *repository.TitleRepository, *repository.SeasonRepository, *repository.EpisodeRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	h := handler.NewLibraryHandler(titleRepo)
	return h, db, titleRepo, seasonRepo, episodeRepo
}

func TestLibraryHandler_ContinueWatching(t *testing.T) {
	h, db, _, _, _ := setupLibraryHandler(t)

	// Create a Watching title with one unwatched episode
	watchingID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Serie en cours", Language: "fr", IsPrimary: true}},
	)
	season := testutil.UpsertSeason(t, db, watchingID, 1, 3)
	testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 1, Name: "Ep1", AirDate: "2024-01-01"},
		{EpisodeNumber: 2, Name: "Ep2", AirDate: "2024-01-08"},
	})

	// Create a Completed title — should not appear
	testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Terminé", Language: "fr", IsPrimary: true}},
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/titles/continue-watching", nil)
	err := h.ContinueWatching(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Len(t, result, 1)
}

func TestLibraryHandler_Upcoming(t *testing.T) {
	h, db, _, _, _ := setupLibraryHandler(t)

	// Create a Watching title with next_air_date in the future
	futureDate := "2099-12-31"
	futureEp := "S2 E1"
	testutil.CreateTitle(t, db,
		&model.Title{
			Type:           model.TitleTypeSeries,
			Year:           2024,
			Status:         model.TitleStatusWatching,
			MatchStatus:    model.MatchStatusConfirmed,
			NextAirDate:    &futureDate,
			NextAirEpisode: &futureEp,
		},
		[]model.TitleName{{Name: "À venir", Language: "fr", IsPrimary: true}},
	)

	// Create a title without next_air_date — should not appear
	testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Sans date", Language: "fr", IsPrimary: true}},
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/titles/upcoming", nil)
	err := h.Upcoming(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Len(t, result, 1)
}

func TestLibraryHandler_ContinueWatching_CacheAndInvalidation(t *testing.T) {
	h, db, _, _, _ := setupLibraryHandler(t)

	c := cache.New(10, 0)
	database.SetInvalidator(c)
	defer database.SetInvalidator(nil)
	h.SetCache(c)

	watchingID := testutil.CreateTitle(t, db,
		&model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Cache Test Series", Language: "en", IsPrimary: true}},
	)
	season := testutil.UpsertSeason(t, db, watchingID, 1, 2)
	testutil.UpsertEpisodesBatch(t, db, season.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 1, Name: "Ep1", AirDate: "2024-01-01"},
		{EpisodeNumber: 2, Name: "Ep2", AirDate: "2024-01-08"},
	})

	// 1. First fetch — cache miss, then populated
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/titles/continue-watching", nil)
	require.NoError(t, h.ContinueWatching(w1, req1))
	assert.Equal(t, http.StatusOK, w1.Code)

	var res1 []repository.ContinueWatchingItem
	require.NoError(t, json.NewDecoder(w1.Body).Decode(&res1))
	require.Len(t, res1, 1)
	assert.Equal(t, 0, res1[0].WatchedEpisodes)

	// Verify it is in cache
	cachedVal, hit := c.Get("library:continue_watching")
	assert.True(t, hit)
	assert.NotNil(t, cachedVal)

	// 2. Second fetch — served from cache
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/titles/continue-watching", nil)
	require.NoError(t, h.ContinueWatching(w2, req2))
	assert.Equal(t, http.StatusOK, w2.Code)

	// 3. Mutate episode (mark Ep1 watched) inside WithTxContext
	var epID int64
	require.NoError(t, db.QueryRow("SELECT id FROM episodes WHERE season_id = ? AND episode = 1", season.ID).Scan(&epID))

	err := database.WithTx(db, func(tx *sql.Tx) error {
		_, err := repository.NewEpisodeWriter(tx).ToggleWatched(t.Context(), epID)
		return err
	})
	require.NoError(t, err)

	// Verify cache was invalidated post-commit
	_, hitAfterWrite := c.Get("library:continue_watching")
	assert.False(t, hitAfterWrite, "cache must be invalidated after write transaction")

	// 4. Third fetch — refreshed from DB, now WatchedEpisodes is 1
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/titles/continue-watching", nil)
	require.NoError(t, h.ContinueWatching(w3, req3))
	assert.Equal(t, http.StatusOK, w3.Code)

	var res3 []repository.ContinueWatchingItem
	require.NoError(t, json.NewDecoder(w3.Body).Decode(&res3))
	require.Len(t, res3, 1)
	assert.Equal(t, 1, res3[0].WatchedEpisodes)
}

