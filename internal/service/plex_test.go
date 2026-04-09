package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	plexwebhooks "github.com/hekmon/plexwebhooks"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPlexService(t *testing.T) (*service.PlexService, *repository.TitleRepository) {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)

	settingRepo := repository.NewSettingRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	titleSvc := service.NewTitleService(db, titleRepo, taskRepo, nil)
	backfillSvc := service.NewBackfillService(db, nil)
	libSvc := service.NewLibraryService(db, titleRepo, seasonRepo, episodeRepo, eventRepo, settingRepo, service.NewNoopNotifier(), backfillSvc, nil)
	svc := service.NewPlexService(context.Background(), db, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, settingRepo, nil, service.NewNoopNotifier(), titleSvc, libSvc)
	return svc, titleRepo
}

func mustParseURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

func TestParseGUIDs(t *testing.T) {
	guids := []*url.URL{
		mustParseURL("imdb://tt1234567"),
		mustParseURL("tmdb://12345"),
		mustParseURL("tvdb://67890"),
	}

	ids := service.ParseGUIDs(guids)
	assert.Equal(t, "tt1234567", ids.IMDB)
	assert.Equal(t, int64(12345), ids.TMDB)
	assert.Equal(t, int64(67890), ids.TVDB)
}

func TestPlexService_MovieScrobble(t *testing.T) {
	svc, titleRepo := setupPlexService(t)

	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			Title:     "Dune: Part Two",
			Year:      2024,
			Type:      plexwebhooks.MediaTypeMovie,
			RatingKey: "12345",
			GUIDExternal: []*url.URL{
				mustParseURL("imdb://tt15239678"),
				mustParseURL("tmdb://693134"),
			},
		},
	}

	err := svc.ProcessScrobble(payload, `{"event":"media.scrobble"}`)
	require.NoError(t, err)

	result, _ := titleRepo.List(repository.TitleFilter{})
	assert.Len(t, result.Titles, 1)
	assert.Equal(t, "Dune: Part Two", result.Titles[0].PrimaryName())
}

func TestPlexService_EpisodeScrobble(t *testing.T) {
	svc, titleRepo := setupPlexService(t)

	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			Title:                "Pilot",
			GrandparentTitle:     "Breaking Bad",
			Year:                 2008,
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                1,
			RatingKey:            "ep1",
			GrandparentRatingKey: "series1",
			GUIDExternal: []*url.URL{
				mustParseURL("imdb://tt0903747"),
			},
		},
	}

	err := svc.ProcessScrobble(payload, `{}`)
	require.NoError(t, err)

	result, _ := titleRepo.List(repository.TitleFilter{})
	assert.Len(t, result.Titles, 1)

	title, _ := titleRepo.GetByID(result.Titles[0].ID)
	assert.Equal(t, "Breaking Bad", title.PrimaryName())
	assert.Len(t, title.Seasons, 1)
	assert.Len(t, title.Seasons[0].Episodes, 1)
	assert.True(t, title.Seasons[0].Episodes[0].Watched)
}

func newTMDBMock(t *testing.T, status string, seasons []struct {
	SeasonNumber int `json:"season_number"`
	EpisodeCount int `json:"episode_count"`
}) *matching.TMDBClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/tv/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(matching.TMDBTVDetails{
			ID:      1399,
			Name:    "Test Series",
			Status:  status,
			Seasons: seasons,
		})
	})
	// Return 404 for any search/movie calls to avoid pipeline errors
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := matching.NewTMDBClient("test-key")
	client.SetBaseURL(server.URL)
	return client
}

func setupPlexServiceWithTMDB(t *testing.T, tmdbClient *matching.TMDBClient) (*service.PlexService, *repository.TitleRepository) {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)

	settingRepo := repository.NewSettingRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	pipeline := matching.NewPipeline(tmdbClient, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titleRepo, taskRepo, pipeline)
	backfillSvc := service.NewBackfillService(db, tmdbClient)
	libSvc := service.NewLibraryService(db, titleRepo, seasonRepo, episodeRepo, eventRepo, settingRepo, service.NewNoopNotifier(), backfillSvc, pipeline)
	svc := service.NewPlexService(context.Background(), db, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, settingRepo, pipeline, service.NewNoopNotifier(), titleSvc, libSvc)
	return svc, titleRepo
}

func TestPlexService_AutoCompleteEndedSeries(t *testing.T) {
	tmdbSeasons := []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	}{
		{SeasonNumber: 1, EpisodeCount: 7},
		{SeasonNumber: 2, EpisodeCount: 10},
	}
	tmdbClient := newTMDBMock(t, "Ended", tmdbSeasons)
	svc, titleRepo := setupPlexServiceWithTMDB(t, tmdbClient)

	// Pre-create the series with a TMDB ID and plex rating key
	tmdbID := int64(1399)
	plexKey := "series1"
	titleID, err := titleRepo.Create(&model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2008,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		TMDBID:        &tmdbID,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Test Series", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	// Scrobble the last episode of the last season (S02E10)
	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			Title:                "Finale",
			GrandparentTitle:     "Test Series",
			Year:                 2008,
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          2,
			Index:                10,
			RatingKey:            "ep-last",
			GrandparentRatingKey: "series1",
		},
	}

	err = svc.ProcessScrobble(payload, `{}`)
	require.NoError(t, err)

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusCompleted, title.Status)
}

func TestPlexService_NoAutoCompleteForNonFinalEpisode(t *testing.T) {
	tmdbSeasons := []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	}{
		{SeasonNumber: 1, EpisodeCount: 7},
		{SeasonNumber: 2, EpisodeCount: 10},
	}
	tmdbClient := newTMDBMock(t, "Ended", tmdbSeasons)
	svc, titleRepo := setupPlexServiceWithTMDB(t, tmdbClient)

	tmdbID := int64(1399)
	plexKey := "series2"
	titleID, err := titleRepo.Create(&model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2008,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		TMDBID:        &tmdbID,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Test Series", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	// Scrobble S02E05 — not the last episode
	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			Title:                "Mid Season",
			GrandparentTitle:     "Test Series",
			Year:                 2008,
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          2,
			Index:                5,
			RatingKey:            "ep-mid",
			GrandparentRatingKey: "series2",
		},
	}

	err = svc.ProcessScrobble(payload, `{}`)
	require.NoError(t, err)

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
}

func TestPlexService_NoAutoCompleteForReturningSeries(t *testing.T) {
	tmdbSeasons := []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	}{
		{SeasonNumber: 1, EpisodeCount: 10},
	}
	tmdbClient := newTMDBMock(t, "Returning Series", tmdbSeasons)
	svc, titleRepo := setupPlexServiceWithTMDB(t, tmdbClient)

	tmdbID := int64(1399)
	plexKey := "series3"
	titleID, err := titleRepo.Create(&model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2020,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		TMDBID:        &tmdbID,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Ongoing Show", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	// Scrobble the last episode of the only season (S01E10) — but series is "Returning"
	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			Title:                "Last Ep",
			GrandparentTitle:     "Ongoing Show",
			Year:                 2020,
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                10,
			RatingKey:            "ep-ret",
			GrandparentRatingKey: "series3",
		},
	}

	err = svc.ProcessScrobble(payload, `{}`)
	require.NoError(t, err)

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
}

func TestPlexService_NoAutoCompleteWithoutTMDB(t *testing.T) {
	svc, titleRepo := setupPlexService(t)

	plexKey := "series4"
	titleID, err := titleRepo.Create(&model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2008,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "No TMDB", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			Title:                "Finale",
			GrandparentTitle:     "No TMDB",
			Year:                 2008,
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                1,
			RatingKey:            "ep-no-tmdb",
			GrandparentRatingKey: "series4",
		},
	}

	err = svc.ProcessScrobble(payload, `{}`)
	require.NoError(t, err)

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
}

func TestPlexService_IgnoresNonScrobble(t *testing.T) {
	svc, titleRepo := setupPlexService(t)

	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypePlay,
		Metadata: plexwebhooks.Metadata{
			Title: "Test",
			Type:  plexwebhooks.MediaTypeMovie,
		},
	}

	err := svc.ProcessScrobble(payload, `{}`)
	require.NoError(t, err)

	result, _ := titleRepo.List(repository.TitleFilter{})
	assert.Len(t, result.Titles, 0)
}
