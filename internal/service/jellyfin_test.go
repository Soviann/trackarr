package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jellyfinTestEnv struct {
	db        *sql.DB
	svc       *service.JellyfinService
	titleRepo *repository.TitleRepository
	eventRepo *repository.WatchEventRepository
	hub       *service.PlaybackHubService
	taskRepo  *repository.TaskRepository
}

func setupJellyfinTest(t *testing.T) *jellyfinTestEnv {
	t.Helper()
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(writeDB))

	t.Cleanup(func() {
		writeDB.Close()
	})

	titleRepo := repository.NewTitleRepository(writeDB)
	seasonRepo := repository.NewSeasonRepository(writeDB)
	episodeRepo := repository.NewEpisodeRepository(writeDB)
	eventRepo := repository.NewWatchEventRepository(writeDB)
	settingRepo := repository.NewSettingRepository(writeDB)
	taskRepo := repository.NewTaskRepository(writeDB)

	pushSvc := service.NewNoopNotifier()
	titleSvc := service.NewTitleService(writeDB, titleRepo, taskRepo, nil)
	libSvc := service.NewLibraryService(writeDB, titleRepo, seasonRepo, episodeRepo, eventRepo, settingRepo, pushSvc, nil, nil)
	playbackHub := service.NewPlaybackHubService()

	jfSvc := service.NewJellyfinService(writeDB, nil, titleSvc, libSvc, playbackHub)

	return &jellyfinTestEnv{
		db:        writeDB,
		svc:       jfSvc,
		titleRepo: titleRepo,
		eventRepo: eventRepo,
		hub:       playbackHub,
		taskRepo:  taskRepo,
	}
}

func TestProcessJellyfinWebhook_Movie(t *testing.T) {
	env := setupJellyfinTest(t)

	jf := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemType:           "Movie",
		Name:               "The Matrix",
		Year:               "1999",
		PlayedToCompletion: "True",
		ProviderIMDB:       "tt0133093",
		ProviderTMDB:       "603",
		ItemID:             "movie-item-123",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	movie := res.Titles[0]
	assert.Equal(t, "The Matrix", movie.PrimaryName())
	assert.Equal(t, model.TitleTypeMovie, movie.Type)
	assert.Equal(t, model.TitleStatusCompleted, movie.Status)
	require.NotNil(t, movie.IMDBID)
	assert.Equal(t, "tt0133093", *movie.IMDBID)
	require.NotNil(t, movie.TMDBID)
	assert.Equal(t, int64(603), *movie.TMDBID)
}

func TestProcessJellyfinWebhook_Episode(t *testing.T) {
	env := setupJellyfinTest(t)

	jf := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemType:           "Episode",
		Name:               "Winter Is Coming",
		Year:               "2011",
		PlayedToCompletion: "True",
		SeriesName:         "Game of Thrones",
		SeriesID:           "series-guid-1",
		Season:             "1",
		Episode:            "1",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	show := res.Titles[0]
	assert.Equal(t, "Game of Thrones", show.PrimaryName())
	assert.Equal(t, model.TitleTypeSeries, show.Type)
	assert.Equal(t, model.TitleStatusWatching, show.Status)

	var seasonID int64
	var seasonNum int
	err = env.db.QueryRow(`SELECT id, season_number FROM seasons WHERE title_id = ?`, show.ID).Scan(&seasonID, &seasonNum)
	require.NoError(t, err)
	assert.Equal(t, 1, seasonNum)

	var epNum int
	var watched bool
	err = env.db.QueryRow(`SELECT episode, watched FROM episodes WHERE season_id = ?`, seasonID).Scan(&epNum, &watched)
	require.NoError(t, err)
	assert.Equal(t, 1, epNum)
	assert.True(t, watched)
}

func TestProcessJellyfinWebhook_Ignored(t *testing.T) {
	env := setupJellyfinTest(t)

	cases := []struct {
		name string
		jf   *model.JellyfinPayload
	}{
		{"playback start", &model.JellyfinPayload{NotificationType: "PlaybackStart", ItemType: "Movie", PlayedToCompletion: "True"}},
		{"stopped before completion", &model.JellyfinPayload{NotificationType: "PlaybackStop", ItemType: "Movie", PlayedToCompletion: "False"}},
		{"completion flag empty", &model.JellyfinPayload{NotificationType: "PlaybackStop", ItemType: "Movie", PlayedToCompletion: ""}},
		{"unsupported item type", &model.JellyfinPayload{NotificationType: "PlaybackStop", ItemType: "Audio", PlayedToCompletion: "True"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := env.svc.ProcessJellyfinWebhook(context.Background(), tc.jf, "{}")
			require.NoError(t, err)

			res, err := env.titleRepo.List(repository.TitleFilter{})
			require.NoError(t, err)
			assert.Empty(t, res.Titles)
		})
	}
}

func TestProcessJellyfinWebhook_EpisodeProviderIDsNotSetAsSeriesIDs(t *testing.T) {
	env := setupJellyfinTest(t)

	jf := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemType:           "Episode",
		Name:               "Episode 1",
		Year:               "2026",
		PlayedToCompletion: "True",
		ProviderIMDB:       "tt41303506", // Episode IMDb ID
		ProviderTVDB:       "11580706",   // Episode TVDB ID
		SeriesName:         "The Appraiser Series",
		SeriesID:           "series-guid-appraiser",
		Season:             "1",
		Episode:            "1",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	show := res.Titles[0]
	assert.Equal(t, "The Appraiser Series", show.PrimaryName())
	assert.Nil(t, show.IMDBID, "Episode IMDb ID must not be assigned to series row")
	assert.Nil(t, show.TVDBID, "Episode TVDB ID must not be assigned to series row")
}

func TestProcessJellyfinWebhook_EpisodeDifferentYearDoesNotHijackExistingSeries(t *testing.T) {
	env := setupJellyfinTest(t)

	// Pre-existing series from 1965 in plan_to_watch
	existing1965ID := testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        1965,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{
		{Name: "Sherlock Holmes", Language: "en", IsPrimary: true},
	})

	jf := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemType:           "Episode",
		Name:               "A Scandal in Bohemia",
		Year:               "1984",
		PlayedToCompletion: "True",
		SeriesName:         "Sherlock Holmes",
		SeriesID:           "jf-series-1984-guid",
		Season:             "1",
		Episode:            "1",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	// Check 1965 series: must NOT be hijacked
	t1965, err := env.titleRepo.GetByID(existing1965ID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusPlanToWatch, t1965.Status, "1965 series must remain plan_to_watch")
	assert.Nil(t, t1965.ExternalSourceID, "1965 series must not receive 1984 series external source ID")

	// Total titles must now be 2 (the 1965 series + a separate 1984 series)
	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	assert.Len(t, res.Titles, 2, "must create a distinct series for 1984 rather than hijacking 1965")
}

func TestProcessJellyfinWebhook_EpisodeWithDisambiguationHints(t *testing.T) {
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	defer writeDB.Close()
	require.NoError(t, database.Migrate(writeDB))

	titleRepo := repository.NewTitleRepository(writeDB)
	seasonRepo := repository.NewSeasonRepository(writeDB)
	episodeRepo := repository.NewEpisodeRepository(writeDB)
	eventRepo := repository.NewWatchEventRepository(writeDB)
	settingRepo := repository.NewSettingRepository(writeDB)
	taskRepo := repository.NewTaskRepository(writeDB)

	tmdbMux := http.NewServeMux()
	tmdbMux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 100, "name": "Sherlock Holmes", "first_air_date": "1965-01-01"},
				{"id": 200, "name": "The Adventures of Sherlock Holmes", "first_air_date": "1984-04-24"},
			},
		})
	})
	tmdbMux.HandleFunc("/tv/200", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 200, "name": "The Adventures of Sherlock Holmes", "first_air_date": "1984-04-24",
			"external_ids": map[string]any{
				"imdb_id": "tt0086705",
				"tvdb_id": 77800,
			},
		})
	})
	tmdbMux.HandleFunc("/tv/200/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"translations": []map[string]any{
				{"iso_639_1": "en", "data": map[string]any{"name": "The Adventures of Sherlock Holmes"}},
			},
		})
	})
	tmdbServer := httptest.NewServer(tmdbMux)
	defer tmdbServer.Close()

	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Contents []struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		prompt := ""
		if len(req.Contents) > 0 && len(req.Contents[0].Parts) > 0 {
			prompt = req.Contents[0].Parts[0].Text
		}

		if strings.Contains(prompt, "disambiguation") {
			assert.Contains(t, prompt, "Un scandale en Bohême")
			assert.Contains(t, prompt, "Season: 1, Episode: 1")
			assert.Contains(t, prompt, "tt0506445")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"candidates": []map[string]any{
					{"content": map[string]any{
						"parts": []map[string]any{
							{"text": `{"matched_tmdb_id": 200, "confidence": "high", "reason": "S01E01 match"}`},
						},
					}},
				},
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"candidates": []map[string]any{
					{"content": map[string]any{
						"parts": []map[string]any{
							{"text": `{"confirmed": true, "confidence": "high", "reason": "match"}`},
						},
					}},
				},
			})
		}
	}))
	defer geminiServer.Close()

	tmdbClient := matching.NewTMDBClient("test-key")
	tmdbClient.SetBaseURL(tmdbServer.URL)

	geminiClient := matching.NewGeminiClient([]string{"test-key"})
	geminiClient.SetBaseURL(geminiServer.URL)

	pipeline := matching.NewPipeline(tmdbClient, nil, geminiClient, nil, t.TempDir())

	pushSvc := service.NewNoopNotifier()
	titleSvc := service.NewTitleService(writeDB, titleRepo, taskRepo, pipeline)
	libSvc := service.NewLibraryService(writeDB, titleRepo, seasonRepo, episodeRepo, eventRepo, settingRepo, pushSvc, nil, nil)
	jfSvc := service.NewJellyfinService(writeDB, nil, titleSvc, libSvc, nil)

	jf := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemType:           "Episode",
		Name:               "Un scandale en Bohême",
		Year:               "0",
		PlayedToCompletion: "True",
		ProviderIMDB:       "tt0506445",
		SeriesName:         "Sherlock Holmes",
		SeriesID:           "jf-series-disambig",
		Season:             "1",
		Episode:            "1",
	}

	err = jfSvc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	res, err := titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	show := res.Titles[0]
	require.NotNil(t, show.TMDBID)
	assert.Equal(t, int64(200), *show.TMDBID)
}

func TestProcessJellyfinWebhook_PlayAndStop(t *testing.T) {
	env := setupJellyfinTest(t)

	// Pre-seed series in library
	res, err := env.db.Exec(`INSERT INTO titles (type, year, status, match_status, external_source_id) VALUES ('series', 2016, 'watching', 'confirmed', 'jf-series-st')`)
	require.NoError(t, err)
	titleID, err := res.LastInsertId()
	require.NoError(t, err)
	_, err = env.db.Exec(`INSERT INTO title_names (title_id, name, language, is_primary) VALUES (?, 'Stranger Things', 'en', 1)`, titleID)
	require.NoError(t, err)

	// 1. PlaybackStart: registers active session without inserting watch event
	startPayload := &model.JellyfinPayload{
		NotificationType: "PlaybackStart",
		ItemType:         "Episode",
		Name:             "Chapter One",
		Year:             "2016",
		SeriesName:       "Stranger Things",
		SeriesID:         "jf-series-st",
		Season:           "1",
		Episode:          "1",
		ItemID:           "jf-item-ep1",
		DeviceName:       "Living Room Shield",
	}

	err = env.svc.ProcessJellyfinWebhook(context.Background(), startPayload, "{}")
	require.NoError(t, err)

	active := env.hub.GetActiveSession()
	require.NotNil(t, active)
	assert.Equal(t, "Stranger Things", active.TitleName)
	assert.Equal(t, "Living Room Shield", active.Player)
	require.NotNil(t, active.SeasonNumber)
	assert.Equal(t, 1, *active.SeasonNumber)
	require.NotNil(t, active.EpisodeNumber)
	assert.Equal(t, 1, *active.EpisodeNumber)

	// 2. PlaybackStop (not played to completion): clears active session
	stopPayload := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemID:             "jf-item-ep1",
		DeviceName:         "Living Room Shield",
		PlayedToCompletion: "False",
	}
	err = env.svc.ProcessJellyfinWebhook(context.Background(), stopPayload, "{}")
	require.NoError(t, err)
	assert.Nil(t, env.hub.GetActiveSession())

	// 3. Fallback resolution of Player: ClientName then "Jellyfin"
	startFallbackClient := &model.JellyfinPayload{
		NotificationType: "PlaybackStart",
		ItemType:         "Episode",
		SeriesID:         "jf-series-st",
		ClientName:       "Jellyfin Web",
	}
	require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), startFallbackClient, "{}"))
	activeClient := env.hub.GetActiveSession()
	require.NotNil(t, activeClient)
	assert.Equal(t, "Jellyfin Web", activeClient.Player)
	env.hub.ClearSession("jellyfin", "Jellyfin Web")

	startFallbackDefault := &model.JellyfinPayload{
		NotificationType: "PlaybackStart",
		ItemType:         "Episode",
		SeriesID:         "jf-series-st",
	}
	require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), startFallbackDefault, "{}"))
	activeDefault := env.hub.GetActiveSession()
	require.NotNil(t, activeDefault)
	assert.Equal(t, "Jellyfin", activeDefault.Player)
}

func TestParseJellyfinRating(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected int
		expectOK bool
	}{
		// Nominal / standard cases
		{"standard integer", "8", 8, true},
		{"standard with whitespace", "  7  ", 7, true},
		{"mid-scale integer", "5", 5, true},

		// Decimal rounding cases
		{"decimal round down", "8.4", 8, true},
		{"decimal round half up", "8.5", 9, true},
		{"decimal round up", "8.6", 9, true},
		{"decimal below 1 rounding up", "0.5", 1, true},
		{"decimal round down to 1", "1.4", 1, true},

		// Boundary cases
		{"boundary min valid", "1", 1, true},
		{"boundary max valid", "10", 10, true},
		{"boundary clamped decimal over 10", "10.4", 10, true},
		{"boundary clamped integer over 10", "11", 10, true},
		{"boundary high value clamped", "100", 10, true},
		{"boundary zero", "0", 0, false},
		{"boundary positive rounding to 0", "0.4", 0, false},
		{"boundary negative zero", "-0.0", 0, false},

		// Error / Invalid cases
		{"empty string", "", 0, false},
		{"whitespace only", "   ", 0, false},
		{"non-numeric string", "foo", 0, false},
		{"negative integer", "-2", 0, false},
		{"negative float", "-0.1", 0, false},
		{"nan string", "NaN", 0, false},
		{"infinity string", "+Inf", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := service.ParseJellyfinRating(tt.raw)
			assert.Equal(t, tt.expectOK, ok, "ParseJellyfinRating(%q) ok mismatch", tt.raw)
			assert.Equal(t, tt.expected, got, "ParseJellyfinRating(%q) value mismatch", tt.raw)
		})
	}
}

func TestJellyfinPayload_RatingJSON(t *testing.T) {
	rawJSON := `{
		"notification_type": "UserDataSaved",
		"item_type": "Movie",
		"name": "Inception",
		"year": "2010",
		"rating": "9",
		"provider_tmdb": "27205"
	}`

	var payload model.JellyfinPayload
	err := json.Unmarshal([]byte(rawJSON), &payload)
	require.NoError(t, err)
	assert.Equal(t, "UserDataSaved", payload.NotificationType)
	assert.Equal(t, "9", payload.Rating)
	assert.Equal(t, "27205", payload.ProviderTMDB)
}

func TestProcessJellyfinWebhook_UserDataSaved_MovieRating(t *testing.T) {
	env := setupJellyfinTest(t)

	tmdbID := int64(157336)
	movieID := testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2014,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbID,
	}, []model.TitleName{
		{Name: "Interstellar", Language: "en", IsPrimary: true},
	})

	jf := &model.JellyfinPayload{
		NotificationType: "UserDataSaved",
		ItemType:         "Movie",
		Name:             "Interstellar",
		Rating:           "9",
		ProviderTMDB:     "157336",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	title, err := env.titleRepo.GetByID(movieID)
	require.NoError(t, err)
	require.NotNil(t, title.MyRating)
	assert.Equal(t, 9, *title.MyRating)
}

func TestProcessJellyfinWebhook_UserDataSaved_SeriesRating(t *testing.T) {
	env := setupJellyfinTest(t)

	tvdbID := int64(81189)
	seriesID := testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		TVDBID:      &tvdbID,
	}, []model.TitleName{
		{Name: "Breaking Bad", Language: "en", IsPrimary: true},
	})

	jf := &model.JellyfinPayload{
		NotificationType: "UserDataSaved",
		ItemType:         "Series",
		Name:             "Breaking Bad",
		Rating:           "10",
		ProviderTVDB:     "81189",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	title, err := env.titleRepo.GetByID(seriesID)
	require.NoError(t, err)
	require.NotNil(t, title.MyRating)
	assert.Equal(t, 10, *title.MyRating)
}

func TestProcessJellyfinWebhook_UserDataSaved_EpisodeIgnored(t *testing.T) {
	env := setupJellyfinTest(t)

	tvdbID := int64(81189)
	seriesID := testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		TVDBID:      &tvdbID,
	}, []model.TitleName{
		{Name: "Breaking Bad", Language: "en", IsPrimary: true},
	})

	jf := &model.JellyfinPayload{
		NotificationType: "UserDataSaved",
		ItemType:         "Episode",
		Name:             "Pilot",
		Rating:           "9",
		ProviderTVDB:     "81189",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	title, err := env.titleRepo.GetByID(seriesID)
	require.NoError(t, err)
	assert.Nil(t, title.MyRating, "Episode rating must not update series rating")
}

func TestProcessJellyfinWebhook_UserDataSaved_RedundantRatingSkipped(t *testing.T) {
	env := setupJellyfinTest(t)

	tmdbID := int64(157336)
	existingRating := 8
	movieID := testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2014,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbID,
		MyRating:    &existingRating,
	}, []model.TitleName{
		{Name: "Interstellar", Language: "en", IsPrimary: true},
	})

	jf := &model.JellyfinPayload{
		NotificationType: "UserDataSaved",
		ItemType:         "Movie",
		Name:             "Interstellar",
		Rating:           "8",
		ProviderTMDB:     "157336",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	title, err := env.titleRepo.GetByID(movieID)
	require.NoError(t, err)
	require.NotNil(t, title.MyRating)
	assert.Equal(t, 8, *title.MyRating)

	// Ensure no tasks were enqueued
	tasks, err := env.taskRepo.ListPending()
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestProcessJellyfinWebhook_UserDataSaved_AnimeMovieEnqueuesAniListPush(t *testing.T) {
	env := setupJellyfinTest(t)

	anilistID := int64(199)
	tmdbID := int64(129)
	movieID := testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeMovie,
		IsAnime:     true,
		Year:        2001,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		AniListID:   &anilistID,
		TMDBID:      &tmdbID,
	}, []model.TitleName{
		{Name: "Spirited Away", Language: "en", IsPrimary: true},
	})

	jf := &model.JellyfinPayload{
		NotificationType: "UserDataSaved",
		ItemType:         "Movie",
		Name:             "Spirited Away",
		Rating:           "10",
		ProviderTMDB:     "129",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	title, err := env.titleRepo.GetByID(movieID)
	require.NoError(t, err)
	require.NotNil(t, title.MyRating)
	assert.Equal(t, 10, *title.MyRating)

	tasks, err := env.taskRepo.ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeAniListPushMovie, tasks[0].TaskType)
}

func TestProcessJellyfinWebhook_UserDataSaved_AnimeSeriesCompletedSeasonEnqueuesAniListPush(t *testing.T) {
	env := setupJellyfinTest(t)

	tmdbID := int64(209867)
	seriesID := testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2023,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbID,
	}, []model.TitleName{
		{Name: "Frieren: Beyond Journey's End", Language: "en", IsPrimary: true},
	})

	season := testutil.UpsertSeason(t, env.db, seriesID, 1, 1)
	testutil.MarkEpisodesWatched(t, env.db, season.ID, 1)
	testutil.InsertSeasonExternalID(t, env.db, season.ID, repository.ProviderAniList, "154587")

	jf := &model.JellyfinPayload{
		NotificationType: "UserDataSaved",
		ItemType:         "Series",
		Name:             "Frieren: Beyond Journey's End",
		Rating:           "10",
		ProviderTMDB:     "209867",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	title, err := env.titleRepo.GetByID(seriesID)
	require.NoError(t, err)
	require.NotNil(t, title.MyRating)
	assert.Equal(t, 10, *title.MyRating)

	tasks, err := env.taskRepo.ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeAniListPushSeason, tasks[0].TaskType)
}
