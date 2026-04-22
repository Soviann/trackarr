package service_test

import (
	"context"
	"encoding/json"
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

func TestAnimeFusion_SoloLeveling(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	require.NotNil(t, titleRepo)
	seasonRepo := repository.NewSeasonRepository(db)
	require.NotNil(t, seasonRepo)
	episodeRepo := repository.NewEpisodeRepository(db)
	require.NotNil(t, episodeRepo)
	taskRepo := repository.NewTaskRepository(db)
	require.NotNil(t, taskRepo)

	// 1. Create Master Title (Solo Leveling S1)
	imdbID := "tt21209876"
	masterID, err := titleRepo.Create(&model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2024,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		IMDBID:      &imdbID,
	}, []model.TitleName{{Name: "Ore dake Level Up na Ken", Language: "ja", IsPrimary: true}})
	require.NoError(t, err)
	require.NotEqual(t, int64(0), masterID)

	s1, err := seasonRepo.GetOrCreate(masterID, 1)
	require.NoError(t, err)
	require.NotNil(t, s1)
	for i := 1; i <= 12; i++ {
		ep, _ := episodeRepo.GetOrCreate(s1.ID, i)
		_, _ = episodeRepo.ToggleWatched(ep.ID)
	}

	// 2. Create Duplicate Title (Solo Leveling S2 - Arise from the Shadow)
	// Initially no IMDB ID (simulating Simkl import)
	dupID, err := titleRepo.Create(&model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2025,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Ore dake Level Up na Ken: Arise from the Shadow", Language: "ja", IsPrimary: true}})
	require.NoError(t, err)

	s2Split, _ := seasonRepo.GetOrCreate(dupID, 1) // In Simkl, S2 is often its own S1
	epS2, _ := episodeRepo.GetOrCreate(s2Split.ID, 1)
	_, _ = episodeRepo.ToggleWatched(epS2.ID)

	// 3. Setup Mock Pipeline & Worker
	tmdbMux := http.NewServeMux()
	tmdbMux.HandleFunc("/tv/301796", func(w http.ResponseWriter, r *http.Request) {
		// Mock TMDB response for S2 that provides the SHARED IMDB ID
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   301796,
			"name": "Solo Leveling: Arise from the Shadow",
			"external_ids": map[string]interface{}{
				"imdb_id": "tt21209876", // Matches Master
			},
		})
	})
	tmdbServer := httptest.NewServer(tmdbMux)
	defer tmdbServer.Close()

	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock Gemini identifying it as Season 2
		resp := `{"is_season": true, "parent_series_name": "Solo Leveling", "season_number": 2, "confidence": "high"}`
		_, _ = w.Write([]byte(`{"candidates": [{"content": {"parts": [{"text": ` + string(mustJSON(resp)) + `}]}}]}`))
	}))
	defer geminiServer.Close()

	tmdbClient := matching.NewTMDBClient("test")
	tmdbClient.SetBaseURL(tmdbServer.URL)

	geminiClient := matching.NewGeminiClient([]string{"test"})
	geminiClient.SetBaseURL(geminiServer.URL)

	pipeline := matching.NewPipeline(tmdbClient, nil, geminiClient, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titleRepo, taskRepo, pipeline)
	worker := service.NewTaskQueueWorker(taskRepo, titleRepo, nil, nil, pipeline, tmdbClient, nil, nil, nil, t.TempDir(), titleSvc, db)

	// 4. Enqueue Enrichment for Duplicate
	payload := service.EnrichmentPayload{
		TitleID:   dupID,
		TitleName: "Ore dake Level Up na Ken: Arise from the Shadow",
		Year:      2025,
		TitleType: model.TitleTypeSeries,
		IsAnime:   true,
		TMDBID:    301796,
	}
	payloadJSON, _ := json.Marshal(payload)
	taskID, _ := taskRepo.Enqueue(model.TaskTypeEnrichment, string(payloadJSON), nil)

	// 5. Run Worker
	tasks, _ := taskRepo.ListPending()
	require.Len(t, tasks, 1)
	worker.ProcessTask(context.Background(), tasks[0])

	// Check task status after processing
	tAfter, err := taskRepo.GetByID(taskID)
	if err == nil {
		t.Logf("Task still exists! Status: %s, Attempts: %d, LastError: %v", tAfter.Status, tAfter.Attempts, tAfter.LastError)
	}

	// 6. VERIFY FUSION
	// Duplicate should be deleted
	_, err = titleRepo.GetByID(dupID)
	assert.Error(t, err, "Duplicate title should be deleted")

	// Master should now have Season 2 (from duplicate)
	master, err := titleRepo.GetByID(masterID)
	require.NoError(t, err)
	require.Len(t, master.Seasons, 2)
	assert.Equal(t, 1, master.Seasons[0].SeasonNumber)
	assert.Equal(t, 2, master.Seasons[1].SeasonNumber)

	// Check episodes in Season 2
	epsS2, _ := episodeRepo.GetBySeasonID(master.Seasons[1].ID)
	assert.Len(t, epsS2, 1)
	assert.True(t, epsS2[0].Watched)

	// Check that the task was completed (deleted)
	_, err = taskRepo.GetByID(taskID)
	assert.Error(t, err, "Task should be deleted upon completion")
}

func mustJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
