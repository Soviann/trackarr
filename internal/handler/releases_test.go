package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleasesHandler_List(t *testing.T) {
	db := testutil.NewTestDB(t)

	prowlarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"guid":        "guid-101",
				"title":       "Gladiator.II.2024.MULTi.1080p.WEB",
				"size":        5000000000,
				"publishDate": time.Now().Format(time.RFC3339),
				"seeders":     50,
				"leechers":    2,
				"indexer":     "C411",
				"indexerId":   1,
				"tmdbId":      float64(558449),
				"categories": []map[string]any{
					{"id": 2000, "name": "Movies"},
				},
			},
		})
	}))
	defer prowlarrServer.Close()

	cfg := &config.Config{
		ProwlarrURL:    prowlarrServer.URL,
		ProwlarrAPIKey: "key",
	}

	titleRepo := repository.NewTitleRepository(db)
	prowlarrSvc := service.NewProwlarrService(cfg, nil, titleRepo, nil)
	h := NewReleasesHandler(db, prowlarrSvc, titleRepo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/releases?type=movie", nil)
	rec := httptest.NewRecorder()

	err := h.List(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var releases []service.ProwlarrRelease
	err = json.Unmarshal(rec.Body.Bytes(), &releases)
	require.NoError(t, err)
	require.Len(t, releases, 1)
	assert.Equal(t, "Gladiator II", releases[0].CleanTitle)
	assert.Equal(t, "movie", releases[0].Type)
}

func TestReleasesHandler_Add(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	h := NewReleasesHandler(db, nil, titleRepo, taskRepo)

	payload := AddReleasePayload{
		TMDBID: 558449,
		Type:   model.TitleTypeMovie,
		Title:  "Gladiator II",
		Year:   2024,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/releases/add", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	err := h.Add(rec, req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var created model.Title
	err = json.Unmarshal(rec.Body.Bytes(), &created)
	require.NoError(t, err)
	assert.Equal(t, model.TitleTypeMovie, created.Type)
	assert.Equal(t, 2024, created.Year)
	assert.Equal(t, model.TitleStatusPlanToWatch, created.Status)
	assert.NotNil(t, created.TMDBID)
	assert.Equal(t, int64(558449), *created.TMDBID)

	// Second add should return existing title with 200 OK
	req2 := httptest.NewRequest(http.MethodPost, "/api/releases/add", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	err = h.Add(rec2, req2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec2.Code)
}
