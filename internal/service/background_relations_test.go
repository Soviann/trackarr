package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackgroundService_RefreshTMDBMovieCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collection/1241" {
			jsonResp := `{
				"id": 1241,
				"name": "Harry Potter Collection",
				"parts": [
					{
						"id": 671,
						"title": "Harry Potter 1",
						"release_date": "2001-11-16",
						"poster_path": "/hp1.jpg",
						"vote_average": 7.9
					},
					{
						"id": 672,
						"title": "Harry Potter 2",
						"release_date": "2002-11-15",
						"poster_path": "/hp2.jpg",
						"vote_average": 7.7
					}
				]
			}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonResp))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Harry Potter 1", false)
	ctx := context.Background()

	tmdbClient := matching.NewTMDBClient("test-key")
	tmdbClient.SetBaseURL(server.URL)

	svc := &BackgroundService{
		writeDB: db,
		tmdb:    tmdbClient,
	}

	movieDetails := &matching.TMDBMovieDetails{
		ID:          671,
		Title:       "Harry Potter 1",
		ReleaseDate: "2001-11-16",
		BelongsToCollection: &matching.TMDBCollectionInfo{
			ID:   1241,
			Name: "Harry Potter Collection",
		},
	}

	title := &repository.TitleLite{
		ID:     titleID,
		TMDBID: int64Ptr(671),
		Type:   model.TitleTypeMovie,
	}

	var result RefreshResult
	svc.refreshTMDBMovieCollection(ctx, title, movieDetails, &result)

	assert.True(t, result.Refreshed)

	relRepo := repository.NewTitleRelationRepository(db)
	rels, err := relRepo.GetByTitleID(ctx, titleID)
	require.NoError(t, err)
	require.Len(t, rels, 1) // HP1 itself is excluded, so only HP2 remains
	assert.Equal(t, int64(672), rels[0].ExternalID)
	assert.Equal(t, "tmdb", rels[0].Provider)
	assert.Equal(t, "Harry Potter 2", rels[0].Title)
	assert.Equal(t, model.RelationSequel, rels[0].RelationType)
}

func TestBackgroundService_RefreshTVDBRelations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/series/81189/extended" {
			jsonResp := `{
				"data": {
					"id": 81189,
					"name": "Breaking Bad",
					"lists": [
						{"id": 79, "name": "Breaking Bad Franchise", "isOfficial": true}
					]
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonResp))
			return
		}
		if r.URL.Path == "/lists/79/extended" {
			jsonResp := `{
				"status": "success",
				"data": {
					"id": 79,
					"name": "Breaking Bad Franchise",
					"isOfficial": true,
					"entities": [
						{"order": 1, "seriesId": 81189, "movieId": null},
						{"order": 2, "seriesId": 273181, "movieId": null},
						{"order": 3, "seriesId": null, "movieId": 131199}
					]
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonResp))
			return
		}
		if r.URL.Path == "/series/273181/extended" {
			jsonResp := `{
				"data": {
					"id": 273181,
					"name": "Better Call Saul",
					"year": "2015",
					"score": 8.9,
					"image": "https://artworks.thetvdb.com/bcs.jpg"
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonResp))
			return
		}
		if r.URL.Path == "/movies/131199/extended" {
			jsonResp := `{
				"data": {
					"id": 131199,
					"name": "El Camino",
					"year": "2019",
					"score": 7.8,
					"image": "https://artworks.thetvdb.com/elcamino.jpg"
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(jsonResp))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	db := testutil.NewTestDB(t)
	titleID := testutil.InsertTitle(t, db, "Breaking Bad", false)
	ctx := context.Background()

	tvdbClient := matching.NewTVDBClient("test-key")
	tvdbClient.SetBaseURL(server.URL)
	tvdbClient.SetTokenForTest("mock-token")

	svc := &BackgroundService{
		writeDB: db,
		tvdb:    tvdbClient,
	}

	title := &repository.TitleLite{
		ID:     titleID,
		TVDBID: int64Ptr(81189),
		Type:   model.TitleTypeSeries,
	}

	var result RefreshResult
	svc.refreshTVDBRelations(ctx, title, &result)

	assert.True(t, result.Refreshed)

	relRepo := repository.NewTitleRelationRepository(db)
	rels, err := relRepo.GetByTitleID(ctx, titleID)
	require.NoError(t, err)
	require.Len(t, rels, 2) // BB is filtered out, so BCS and El Camino remain
	assert.Equal(t, "tvdb", rels[0].Provider)
	assert.Equal(t, int64(273181), rels[0].ExternalID)
	assert.Equal(t, "Better Call Saul", rels[0].Title)
	assert.Equal(t, "El Camino", rels[1].Title)
}

func int64Ptr(v int64) *int64 {
	return &v
}
