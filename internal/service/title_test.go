package service_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTitleService_CreateFromPlex_DuplicateDetection(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	// 1. Pre-existing title (e.g. from Simkl)
	tmdbID := int64(119335)
	existingID, err := titleRepo.Create(&model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2022,
		Status:      model.TitleStatusCompleted,
		TMDBID:      &tmdbID,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "In the Land of Leadale", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	// 2. Setup Pipeline mock
	mux := http.NewServeMux()
	mux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":119335,"name":"In the Land of Leadale","first_air_date":"2022-01-05"}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tmdbClient := matching.NewTMDBClient("test")
	tmdbClient.SetBaseURL(server.URL)
	pipeline := matching.NewPipeline(tmdbClient, nil, nil, nil, t.TempDir())

	svc := service.NewTitleService(db, titleRepo, taskRepo, pipeline)

	// 3. Create from Plex (matches existing TMDB ID)
	ids := service.PlexExternalIDs{} // Empty IDs, let pipeline match
	ratingKey := "plex-123"
	newID, err := svc.CreateFromPlex(db, "In the Land of Leadale", 2022, ids, model.TitleTypeSeries, ratingKey, nil, model.TitleStatusWatching)
	require.NoError(t, err)

	// Verify it returned the existing ID
	assert.Equal(t, existingID, newID)

	// Verify the existing title was updated with Plex rating key
	title, _ := titleRepo.GetByID(existingID)
	require.NotNil(t, title.PlexRatingKey)
	assert.Equal(t, "plex-123", *title.PlexRatingKey)
}
