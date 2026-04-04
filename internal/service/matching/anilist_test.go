package matching

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAniListServer(t *testing.T) (*httptest.Server, *AniListClient) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch {
		case contains(req.Query, "Page(perPage"):
			// Search query
			malID := int64(21)
			eps := 148
			year := 2011
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"Page": map[string]interface{}{
						"media": []interface{}{
							map[string]interface{}{
								"id":    21,
								"idMal": malID,
								"title": map[string]string{
									"romaji":  "One Punch Man",
									"english": "One Punch Man",
								},
								"episodes":   eps,
								"format":     "TV",
								"seasonYear": year,
							},
						},
					},
				},
			})

		case contains(req.Query, "Media(id"):
			// Details query
			malID := int64(21)
			eps := 12
			year := 2015
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"Media": map[string]interface{}{
						"id":    21,
						"idMal": malID,
						"title": map[string]string{
							"romaji":  "One Punch Man",
							"english": "One Punch Man",
						},
						"episodes":   eps,
						"format":     "TV",
						"seasonYear": year,
					},
				},
			})

		case contains(req.Query, "SaveMediaListEntry"):
			// Sync rating mutation
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"SaveMediaListEntry": map[string]interface{}{
						"id":    1,
						"score": 85,
					},
				},
			})
		}
	}))
	t.Cleanup(server.Close)

	client := NewAniListClient()
	client.apiURL = server.URL

	return server, client
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAniListSearchAnime(t *testing.T) {
	_, client := newTestAniListServer(t)

	results, err := client.SearchAnime("One Punch Man")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, int64(21), results[0].ID)
	assert.Equal(t, "One Punch Man", results[0].RomajiTitle)
	assert.Equal(t, "One Punch Man", results[0].DisplayTitle())
	assert.NotNil(t, results[0].MALID)
	assert.Equal(t, int64(21), *results[0].MALID)
}

func TestAniListGetAnimeDetails(t *testing.T) {
	_, client := newTestAniListServer(t)

	details, err := client.GetAnimeDetails(21)
	require.NoError(t, err)
	assert.Equal(t, int64(21), details.ID)
	assert.Equal(t, "TV", details.Format)
	assert.NotNil(t, details.Episodes)
	assert.Equal(t, 12, *details.Episodes)
}

func TestAniListSyncRating(t *testing.T) {
	_, client := newTestAniListServer(t)

	err := client.SyncRating(21, 85, "test-token")
	require.NoError(t, err)
}

func TestAniListGetNames(t *testing.T) {
	_, client := newTestAniListServer(t)

	names, err := client.GetNames(21)
	require.NoError(t, err)
	assert.Equal(t, "One Punch Man", names.Romaji)
	assert.Equal(t, "One Punch Man", names.English)
}

func TestAniListSearchResultDisplayTitle(t *testing.T) {
	r := AniListSearchResult{RomajiTitle: "Shingeki no Kyojin", EnglishTitle: ""}
	assert.Equal(t, "Shingeki no Kyojin", r.DisplayTitle())

	r.EnglishTitle = "Attack on Titan"
	assert.Equal(t, "Attack on Titan", r.DisplayTitle())
}
