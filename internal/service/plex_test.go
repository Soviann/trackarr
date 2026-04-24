package service_test

import (
	"context"
	"database/sql"
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
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPlexService(t *testing.T) (*service.PlexService, *sql.DB, *repository.TitleRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
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
	svc := service.NewPlexService(context.Background(), db, nil, titleSvc, libSvc)
	return svc, db, titleRepo
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
	svc, _, titleRepo := setupPlexService(t)

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

	err := svc.ProcessWebhook(context.Background(), payload, `{"event":"media.scrobble"}`)
	require.NoError(t, err)

	result, _ := titleRepo.List(repository.TitleFilter{})
	assert.Len(t, result.Titles, 1)
	assert.Equal(t, "Dune: Part Two", result.Titles[0].PrimaryName())
}

func TestPlexService_EpisodeScrobble(t *testing.T) {
	svc, _, titleRepo := setupPlexService(t)

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

	err := svc.ProcessWebhook(context.Background(), payload, `{}`)
	require.NoError(t, err)

	result, _ := titleRepo.List(repository.TitleFilter{})
	assert.Len(t, result.Titles, 1)

	title, _ := titleRepo.GetByID(result.Titles[0].ID)
	assert.Equal(t, "Breaking Bad", title.PrimaryName())
	assert.Len(t, title.Seasons, 1)
	assert.Len(t, title.Seasons[0].Episodes, 1)
	assert.True(t, title.Seasons[0].Episodes[0].Watched)
}

func TestPlexService_EpisodeScrobble_NoIDLeak(t *testing.T) {
	svc, _, titleRepo := setupPlexService(t)

	// Episode scrobble with episode-level GUIDs
	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			Title:                "Episode 7",
			GrandparentTitle:     "In the Land of Leadale",
			Year:                 2022,
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                7,
			RatingKey:            "ep7",
			GrandparentRatingKey: "series1",
			GUIDExternal: []*url.URL{
				mustParseURL("tmdb://3415481"), // Episode ID
			},
		},
	}

	err := svc.ProcessWebhook(context.Background(), payload, `{}`)
	require.NoError(t, err)

	result, _ := titleRepo.List(repository.TitleFilter{})
	require.Len(t, result.Titles, 1)

	title := result.Titles[0]
	// The series should NOT have the episode's TMDB ID
	if title.TMDBID != nil {
		assert.NotEqual(t, int64(3415481), *title.TMDBID)
	}
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

func setupPlexServiceWithTMDB(t *testing.T, tmdbClient *matching.TMDBClient) (*service.PlexService, *sql.DB, *repository.TitleRepository) {
	t.Helper()
	db, _, err := database.Open(":memory:")
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
	svc := service.NewPlexService(context.Background(), db, pipeline, titleSvc, libSvc)
	return svc, db, titleRepo
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
	svc, db, titleRepo := setupPlexServiceWithTMDB(t, tmdbClient)

	// Pre-create the series with a TMDB ID and plex rating key
	tmdbID := int64(1399)
	plexKey := "series1"
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2008,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		TMDBID:        &tmdbID,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Test Series", Language: "en", IsPrimary: true}})

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

	err := svc.ProcessWebhook(context.Background(), payload, `{}`)
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
	svc, db, titleRepo := setupPlexServiceWithTMDB(t, tmdbClient)

	tmdbID := int64(1399)
	plexKey := "series2"
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2008,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		TMDBID:        &tmdbID,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Test Series", Language: "en", IsPrimary: true}})

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

	err := svc.ProcessWebhook(context.Background(), payload, `{}`)
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
	svc, db, titleRepo := setupPlexServiceWithTMDB(t, tmdbClient)

	tmdbID := int64(1399)
	plexKey := "series3"
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2020,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		TMDBID:        &tmdbID,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Ongoing Show", Language: "en", IsPrimary: true}})

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

	err := svc.ProcessWebhook(context.Background(), payload, `{}`)
	require.NoError(t, err)

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
}

func TestPlexService_NoAutoCompleteWithoutTMDB(t *testing.T) {
	svc, db, titleRepo := setupPlexService(t)

	plexKey := "series4"
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2008,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "No TMDB", Language: "en", IsPrimary: true}})

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

	err := svc.ProcessWebhook(context.Background(), payload, `{}`)
	require.NoError(t, err)

	title, _ := titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusWatching, title.Status)
}

func TestPlexService_IgnoresNonScrobble(t *testing.T) {
	svc, _, titleRepo := setupPlexService(t)

	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypePlay,
		Metadata: plexwebhooks.Metadata{
			Title: "Test",
			Type:  plexwebhooks.MediaTypeMovie,
		},
	}

	err := svc.ProcessWebhook(context.Background(), payload, `{}`)
	require.NoError(t, err)

	result, _ := titleRepo.List(repository.TitleFilter{})
	assert.Len(t, result.Titles, 0)
}

func TestHandleEpisodePlay_UnwatchedEpisode_MarksWatched(t *testing.T) {
	svc, db, titleRepo := setupPlexService(t)

	// Pre-create a tracked series with a plex rating key
	plexKey := "series1"
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          2024,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Poirot", Language: "en", IsPrimary: true}})

	// Send a media.play for an episode that has never been watched (catch-up).
	payload := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypePlay,
		Metadata: plexwebhooks.Metadata{
			Title:                "S01E01",
			GrandparentTitle:     "Poirot",
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                1,
			RatingKey:            "ep1",
			GrandparentRatingKey: "series1",
		},
	}

	require.NoError(t, svc.ProcessWebhook(context.Background(), payload, `{}`))

	title, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	require.Len(t, title.Seasons, 1)
	require.Len(t, title.Seasons[0].Episodes, 1)
	ep := title.Seasons[0].Episodes[0]
	assert.True(t, ep.Watched, "episode must be marked watched on catch-up")
	require.NotNil(t, ep.FirstWatchedAt, "first_watched_at must be set")
	require.NotNil(t, ep.LastWatchedAt, "last_watched_at must be set")
}

func TestPlexService_PlayCreatesRewatchEvent(t *testing.T) {
	svc, db, titleRepo := setupPlexService(t)

	// Pre-create a tracked series
	plexKey := "series-poirot"
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          1989,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Poirot", Language: "en", IsPrimary: true}})

	// First scrobble → marks the episode watched
	scrobble := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			Title:                "The Adventure of the Clapham Cook",
			GrandparentTitle:     "Poirot",
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                1,
			RatingKey:            "ep1",
			GrandparentRatingKey: "series-poirot",
		},
	}
	require.NoError(t, svc.ProcessWebhook(context.Background(), scrobble, `{}`))

	title, _ := titleRepo.GetByID(titleID)
	require.Len(t, title.Seasons[0].Episodes, 1)
	ep := title.Seasons[0].Episodes[0]
	require.True(t, ep.Watched)
	require.NotNil(t, ep.FirstWatchedAt)
	firstWatchedAt := ep.FirstWatchedAt

	// Rewatch via media.play
	play := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypePlay,
		Metadata: plexwebhooks.Metadata{
			Title:                "The Adventure of the Clapham Cook",
			GrandparentTitle:     "Poirot",
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                1,
			RatingKey:            "ep1",
			GrandparentRatingKey: "series-poirot",
		},
	}
	require.NoError(t, svc.ProcessWebhook(context.Background(), play, `{}`))

	title, _ = titleRepo.GetByID(titleID)
	ep = title.Seasons[0].Episodes[0]
	assert.True(t, ep.Watched, "episode must stay watched")
	assert.Equal(t, firstWatchedAt, ep.FirstWatchedAt, "first_watched_at must be preserved")
	assert.NotNil(t, ep.LastWatchedAt)
	assert.True(t, ep.LastWatchedAt.After(*ep.FirstWatchedAt) || ep.LastWatchedAt.Equal(*ep.FirstWatchedAt),
		"last_watched_at must be >= first_watched_at after rewatch")
}

func TestPlexService_PlayNoAutoComplete(t *testing.T) {
	tmdbSeasons := []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	}{
		{SeasonNumber: 1, EpisodeCount: 1},
	}
	tmdbClient := newTMDBMock(t, "Ended", tmdbSeasons)
	svc, db, titleRepo := setupPlexServiceWithTMDB(t, tmdbClient)

	tmdbID := int64(1399)
	plexKey := "series1"
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:          model.TitleTypeSeries,
		Year:          1989,
		Status:        model.TitleStatusWatching,
		MatchStatus:   model.MatchStatusConfirmed,
		TMDBID:        &tmdbID,
		PlexRatingKey: &plexKey,
	}, []model.TitleName{{Name: "Poirot", Language: "en", IsPrimary: true}})

	// First scrobble to mark the episode watched
	scrobble := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypeScrobble,
		Metadata: plexwebhooks.Metadata{
			GrandparentTitle:     "Poirot",
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                1,
			RatingKey:            "ep1",
			GrandparentRatingKey: "series1",
		},
	}
	require.NoError(t, svc.ProcessWebhook(context.Background(), scrobble, `{}`))

	title, _ := titleRepo.GetByID(titleID)
	require.Equal(t, model.TitleStatusCompleted, title.Status, "series should auto-complete after scrobble")

	// media.play rewatch — must NOT reset status
	play := &plexwebhooks.Payload{
		Event: plexwebhooks.EventTypePlay,
		Metadata: plexwebhooks.Metadata{
			GrandparentTitle:     "Poirot",
			Type:                 plexwebhooks.MediaTypeEpisode,
			ParentIndex:          1,
			Index:                1,
			RatingKey:            "ep1",
			GrandparentRatingKey: "series1",
		},
	}
	require.NoError(t, svc.ProcessWebhook(context.Background(), play, `{}`))

	title, _ = titleRepo.GetByID(titleID)
	assert.Equal(t, model.TitleStatusCompleted, title.Status, "status must stay completed after rewatch play")
}
