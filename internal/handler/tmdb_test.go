package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTMDBHandler_Search(t *testing.T) {
	t.Run("fails when TMDB not configured", func(t *testing.T) {
		h := handler.NewTMDBHandler(nil)
		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search?query=test", nil)
		rr := httptest.NewRecorder()

		err := h.Search(rr, req)
		require.Error(t, err)
	})

	t.Run("requires query parameter", func(t *testing.T) {
		tmdb := matching.NewTMDBClient("fake-key")
		h := handler.NewTMDBHandler(tmdb)
		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search", nil)
		rr := httptest.NewRecorder()

		err := h.Search(rr, req)
		require.Error(t, err)
	})

	t.Run("rejects invalid media type", func(t *testing.T) {
		tmdb := matching.NewTMDBClient("fake-key")
		h := handler.NewTMDBHandler(tmdb)
		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search?query=test&type=book", nil)
		rr := httptest.NewRecorder()

		err := h.Search(rr, req)
		require.Error(t, err)
	})

	t.Run("searches movies successfully", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/search/movie") {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"results": [
						{"id": 123, "title": "Inception", "release_date": "2010-07-16", "poster_path": "/poster.jpg"}
					]
				}`))
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		tmdb := matching.NewTMDBClient("fake-key")
		tmdb.SetBaseURL(srv.URL)
		h := handler.NewTMDBHandler(tmdb)

		req := httptest.NewRequest(http.MethodGet, "/api/tmdb/search?query=Inception&type=movie", nil)
		rr := httptest.NewRecorder()

		require.NoError(t, h.Search(rr, req))
		assert.Equal(t, http.StatusOK, rr.Code)

		var results []struct {
			ID        int64   `json:"id"`
			Title     string  `json:"title"`
			Year      int     `json:"year"`
			PosterURL *string `json:"poster_url"`
		}
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&results))
		require.Len(t, results, 1)
		assert.Equal(t, int64(123), results[0].ID)
		assert.Equal(t, "Inception", results[0].Title)
		assert.Equal(t, 2010, results[0].Year)
		require.NotNil(t, results[0].PosterURL)
		assert.Equal(t, "https://image.tmdb.org/t/p/w342/poster.jpg", *results[0].PosterURL)
	})
}
