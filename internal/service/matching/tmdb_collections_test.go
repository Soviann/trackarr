package matching

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTMDBGetMovieCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/collection/1241", r.URL.Path)
		assert.Equal(t, "test-key", r.URL.Query().Get("api_key"))
		assert.Equal(t, "fr-FR", r.URL.Query().Get("language"))

		jsonResp := `{
			"id": 1241,
			"name": "Harry Potter Collection",
			"overview": "The complete saga of Harry Potter films.",
			"poster_path": "/collection_poster.jpg",
			"backdrop_path": "/collection_backdrop.jpg",
			"parts": [
				{
					"id": 671,
					"title": "Harry Potter and the Philosopher's Stone",
					"overview": "First year at Hogwarts.",
					"release_date": "2001-11-16",
					"poster_path": "/hp1.jpg",
					"vote_average": 7.9,
					"vote_count": 25000
				},
				{
					"id": 672,
					"title": "Harry Potter and the Chamber of Secrets",
					"overview": "Second year at Hogwarts.",
					"release_date": "2002-11-15",
					"poster_path": "/hp2.jpg",
					"vote_average": 7.7,
					"vote_count": 22000
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jsonResp))
	}))
	defer server.Close()

	client := NewTMDBClient("test-key")
	client.SetBaseURL(server.URL)

	details, err := client.GetMovieCollection(context.Background(), 1241, "fr-FR")
	require.NoError(t, err)
	require.NotNil(t, details)

	assert.Equal(t, int64(1241), details.ID)
	assert.Equal(t, "Harry Potter Collection", details.Name)
	assert.Len(t, details.Parts, 2)
	assert.Equal(t, int64(671), details.Parts[0].ID)
	assert.Equal(t, "Harry Potter and the Philosopher's Stone", details.Parts[0].Title)
	assert.Equal(t, 7.9, details.Parts[0].VoteAverage)
}

func TestTMDBGetMovieCollection_InvalidID(t *testing.T) {
	client := NewTMDBClient("test-key")
	_, err := client.GetMovieCollection(context.Background(), 0, "fr-FR")
	assert.Error(t, err)
}
