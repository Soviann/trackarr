package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
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
