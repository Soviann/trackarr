package matching

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTVDBGetListExtended(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/lists/79/extended", r.URL.Path)

		jsonResp := `{
			"status": "success",
			"data": {
				"id": 79,
				"name": "Breaking Bad Franchise",
				"overview": "The Breaking Bad Universe.",
				"isOfficial": true,
				"entities": [
					{
						"order": 1,
						"seriesId": 81189,
						"movieId": null
					},
					{
						"order": 2,
						"seriesId": 273181,
						"movieId": null
					},
					{
						"order": 3,
						"seriesId": null,
						"movieId": 131199
					}
				]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jsonResp))
	}))
	defer server.Close()

	client := NewTVDBClient("test-key")
	client.SetBaseURL(server.URL)
	client.SetTokenForTest("mock-token")

	details, err := client.GetListExtended(context.Background(), 79)
	require.NoError(t, err)
	require.NotNil(t, details)

	assert.Equal(t, int64(79), details.ID)
	assert.Equal(t, "Breaking Bad Franchise", details.Name)
	assert.True(t, details.IsOfficial)
	assert.Len(t, details.Entities, 3)
	assert.Equal(t, int64(81189), *details.Entities[0].SeriesID)
	assert.Equal(t, int64(273181), *details.Entities[1].SeriesID)
	assert.Equal(t, int64(131199), *details.Entities[2].MovieID)
}

func TestTVDBGetListExtended_InvalidID(t *testing.T) {
	client := NewTVDBClient("test-key")
	_, err := client.GetListExtended(context.Background(), 0)
	assert.Error(t, err)
}
