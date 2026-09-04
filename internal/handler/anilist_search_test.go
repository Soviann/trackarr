package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAniListSearchHandler_Search(t *testing.T) {
	t.Run("fails when AniList not configured", func(t *testing.T) {
		h := handler.NewAniListSearchHandler(nil)
		req := httptest.NewRequest(http.MethodGet, "/api/anilist/search?query=frieren", nil)
		rr := httptest.NewRecorder()

		err := h.Search(rr, req)
		require.Error(t, err)
	})

	t.Run("requires query parameter", func(t *testing.T) {
		anilist := matching.NewAniListClient()
		h := handler.NewAniListSearchHandler(anilist)
		req := httptest.NewRequest(http.MethodGet, "/api/anilist/search", nil)
		rr := httptest.NewRecorder()

		err := h.Search(rr, req)
		require.Error(t, err)
	})

	t.Run("searches anime successfully", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"Page": {
						"media": [
							{
								"id": 154587,
								"idMal": 52991,
								"title": {
									"romaji": "Sousou no Frieren",
									"english": "Frieren: Beyond Journey's End"
								},
								"episodes": 28,
								"format": "TV",
								"seasonYear": 2023,
								"coverImage": {
									"extraLarge": "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx154587-n1fmjooSnvBh.jpg",
									"large": "https://s4.anilist.co/file/anilistcdn/media/anime/cover/medium/bx154587-n1fmjooSnvBh.jpg"
								}
							}
						]
					}
				}
			}`))
		}))
		defer srv.Close()

		anilist := matching.NewAniListClientWithURL(srv.URL)
		h := handler.NewAniListSearchHandler(anilist)

		req := httptest.NewRequest(http.MethodGet, "/api/anilist/search?query=Frieren", nil)
		rr := httptest.NewRecorder()

		require.NoError(t, h.Search(rr, req))
		assert.Equal(t, http.StatusOK, rr.Code)

		var results []struct {
			ID           int64   `json:"id"`
			RomajiTitle  string  `json:"romaji_title"`
			EnglishTitle string  `json:"english_title"`
			Title        string  `json:"title"`
			Year         *int    `json:"year"`
			Format       string  `json:"format"`
			Episodes     *int    `json:"episodes"`
			PosterURL    *string `json:"poster_url"`
		}
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&results))
		require.Len(t, results, 1)
		assert.Equal(t, int64(154587), results[0].ID)
		assert.Equal(t, "Sousou no Frieren", results[0].RomajiTitle)
		assert.Equal(t, "Frieren: Beyond Journey's End", results[0].EnglishTitle)
		assert.Equal(t, "Frieren: Beyond Journey's End", results[0].Title)
		require.NotNil(t, results[0].Year)
		assert.Equal(t, 2023, *results[0].Year)
		assert.Equal(t, "TV", results[0].Format)
		require.NotNil(t, results[0].Episodes)
		assert.Equal(t, 28, *results[0].Episodes)
		require.NotNil(t, results[0].PosterURL)
		assert.Equal(t, "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx154587-n1fmjooSnvBh.jpg", *results[0].PosterURL)
	})
}
