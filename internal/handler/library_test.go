package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLibraryHandler(t *testing.T) (*handler.LibraryHandler, *repository.TitleRepository, *repository.SeasonRepository, *repository.EpisodeRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	h := handler.NewLibraryHandler(titleRepo)
	return h, titleRepo, seasonRepo, episodeRepo
}

func TestLibraryHandler_ContinueWatching(t *testing.T) {
	h, titleRepo, seasonRepo, episodeRepo := setupLibraryHandler(t)

	// Create a Watching title with one unwatched episode
	watchingID, err := titleRepo.Create(
		&model.Title{Type: model.TitleTypeSeries, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Serie en cours", Language: "fr", IsPrimary: true}},
	)
	require.NoError(t, err)
	season, err := seasonRepo.Upsert(watchingID, 1, 3)
	require.NoError(t, err)
	require.NoError(t, episodeRepo.UpsertBatch(season.ID, []repository.EpisodeUpsert{
		{EpisodeNumber: 1, Name: "Ep1"},
		{EpisodeNumber: 2, Name: "Ep2"},
	}))

	// Create a Completed title — should not appear
	_, err = titleRepo.Create(
		&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Terminé", Language: "fr", IsPrimary: true}},
	)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/titles/continue-watching", nil)
	err = h.ContinueWatching(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Len(t, result, 1)
}

func TestLibraryHandler_Upcoming(t *testing.T) {
	h, titleRepo, _, _ := setupLibraryHandler(t)

	// Create a Watching title with next_air_date in the future
	futureDate := "2099-12-31"
	futureEp := "S2 E1"
	_, err := titleRepo.Create(
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
	require.NoError(t, err)

	// Create a title without next_air_date — should not appear
	_, err = titleRepo.Create(
		&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed},
		[]model.TitleName{{Name: "Sans date", Language: "fr", IsPrimary: true}},
	)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/titles/upcoming", nil)
	err = h.Upcoming(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Len(t, result, 1)
}
