package service_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestProcessJellyfinWebhook_PlaybackStart_MetadataAndFallback(t *testing.T) {
	env := setupJellyfinTest(t)

	overview := "A thief who steals corporate secrets through dream-sharing."
	tmdbID := int64(27205)
	testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2010,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &tmdbID,
		Overview:    &overview,
	}, []model.TitleName{
		{Name: "Inception", Language: "en", IsPrimary: true},
	})

	// 1. Movie with empty overview in payload falls back to title.Overview, ticks converted to seconds
	err := env.svc.ProcessJellyfinWebhook(context.Background(), &model.JellyfinPayload{
		NotificationType:      "PlaybackStart",
		ItemType:              "Movie",
		Name:                  "Inception",
		ProviderTMDB:          "27205",
		DeviceName:            "Living Room TV",
		RunTimeTicks:          "88800000000", // 8880s
		PlaybackPositionTicks: "1200000000",  // 120s
	}, "{}")
	require.NoError(t, err)

	active := env.hub.GetActiveSession()
	require.NotNil(t, active)
	assert.Equal(t, overview, active.Overview)
	assert.Equal(t, int64(8880), active.DurationSec)
	assert.Equal(t, int64(120), active.ViewOffsetSec)

	// 2. Payload overview takes precedence over title.Overview when provided
	seriesTMDB := int64(1396)
	seriesOverview := "Series overview"
	testutil.CreateTitle(t, env.db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		TMDBID:      &seriesTMDB,
		Overview:    &seriesOverview,
	}, []model.TitleName{
		{Name: "Breaking Bad", Language: "en", IsPrimary: true},
	})
	// Attach series_id as external_source_id for episode matching
	_, err = env.db.Exec(`UPDATE titles SET external_source_id = 'jf-series-1' WHERE tmdb_id = 1396`)
	require.NoError(t, err)

	err = env.svc.ProcessJellyfinWebhook(context.Background(), &model.JellyfinPayload{
		NotificationType:      "PlaybackStart",
		ItemType:              "Episode",
		Name:                  "Pilot",
		SeriesName:            "Breaking Bad",
		SeriesID:              "jf-series-1",
		Season:                "1",
		Episode:               "1",
		Overview:              "Episode-specific overview",
		RunTimeTicks:          "34800000000", // 3480s
		PlaybackPositionTicks: "0",
	}, "{}")
	require.NoError(t, err)

	active = env.hub.GetActiveSession()
	require.NotNil(t, active)
	assert.Equal(t, "Episode-specific overview", active.Overview)
	assert.Equal(t, int64(3480), active.DurationSec)
	assert.Equal(t, int64(0), active.ViewOffsetSec)
}

func TestProcessJellyfinWebhook_PlaybackStart_AutoCreatesUntrackedMovieAndSeries(t *testing.T) {
	env := setupJellyfinTest(t)

	// 1. Untracked Movie
	moviePayload := &model.JellyfinPayload{
		NotificationType: "PlaybackStart",
		ItemType:         "Movie",
		Name:             "Noryang: Deadly Sea",
		Year:             "2023",
		ProviderTMDB:     "660360",
		ProviderIMDB:     "tt22813112",
		ItemID:           "jf-noryang",
		DeviceName:       "LG Smart TV",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), moviePayload, "{}")
	require.NoError(t, err)

	active := env.hub.GetActiveSession()
	require.NotNil(t, active)
	assert.Equal(t, "Noryang: Deadly Sea", active.TitleName)
	assert.Equal(t, model.TitleTypeMovie, active.TitleType)
	assert.Equal(t, "LG Smart TV", active.Player)
	assert.True(t, active.TitleID > 0)

	createdMovie, err := env.titleRepo.GetByID(active.TitleID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusWatching, createdMovie.Status)
	assert.Equal(t, 2023, createdMovie.Year)

	// 2. Untracked Series and Episode
	seriesPayload := &model.JellyfinPayload{
		NotificationType: "PlaybackStart",
		ItemType:         "Episode",
		Name:             "Good News About Hell",
		SeriesName:       "Severance",
		Year:             "2022",
		SeriesID:         "jf-series-sev",
		Season:           "1",
		Episode:          "1",
		DeviceName:       "LG Smart TV",
	}

	err = env.svc.ProcessJellyfinWebhook(context.Background(), seriesPayload, "{}")
	require.NoError(t, err)

	activeSeries := env.hub.GetActiveSession()
	require.NotNil(t, activeSeries)
	assert.Equal(t, "Severance", activeSeries.TitleName)
	assert.Equal(t, model.TitleTypeSeries, activeSeries.TitleType)
	require.NotNil(t, activeSeries.SeasonNumber)
	assert.Equal(t, 1, *activeSeries.SeasonNumber)
	require.NotNil(t, activeSeries.EpisodeNumber)
	assert.Equal(t, 1, *activeSeries.EpisodeNumber)
	assert.True(t, activeSeries.TitleID > 0)

	createdSeries, err := env.titleRepo.GetByID(activeSeries.TitleID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusWatching, createdSeries.Status)

	// Verify season and episode exist
	require.Len(t, createdSeries.Seasons, 1)
	assert.Equal(t, 1, createdSeries.Seasons[0].SeasonNumber)
	require.Len(t, createdSeries.Seasons[0].Episodes, 1)
	assert.Equal(t, 1, createdSeries.Seasons[0].Episodes[0].Episode)
}

func TestProcessJellyfinWebhook_ItemAdded_Movie(t *testing.T) {
	env := setupJellyfinTest(t)

	jf := &model.JellyfinPayload{
		NotificationType: "ItemAdded",
		ItemType:         "Movie",
		Name:             "  Dune: Part Two  ",
		Year:             "2024",
		ItemID:           "  jf-movie-dune2  ",
		ProviderIMDB:     "  TT15239678  ",
		ProviderTMDB:     "693134",
		ProviderTVDB:     "12345",
	}

	err := env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}")
	require.NoError(t, err)

	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	movie, err := env.titleRepo.GetByID(res.Titles[0].ID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleTypeMovie, movie.Type)
	assert.Equal(t, "Dune: Part Two", movie.PrimaryName())
	require.NotNil(t, movie.OriginalTitle)
	assert.Equal(t, "Dune: Part Two", *movie.OriginalTitle)
	assert.Equal(t, 2024, movie.Year)
	assert.Equal(t, model.TitleStatusPlanToWatch, movie.Status)
	require.NotNil(t, movie.ExternalSourceID)
	assert.Equal(t, "jf-movie-dune2", *movie.ExternalSourceID)
	require.NotNil(t, movie.IMDBID)
	assert.Equal(t, "tt15239678", *movie.IMDBID)
	require.NotNil(t, movie.TMDBID)
	assert.Equal(t, int64(693134), *movie.TMDBID)
	require.NotNil(t, movie.TVDBID)
	assert.Equal(t, int64(12345), *movie.TVDBID)
	assert.Equal(t, model.MatchStatusConfirmed, movie.MatchStatus)
	require.NotNil(t, movie.MatchSource)
	assert.Equal(t, "jellyfin_webhook", *movie.MatchSource)
	assert.True(t, movie.ArrIgnored)
	assert.NotNil(t, movie.DownloadedAt, "movie added via ItemAdded must have downloaded_at set")

	watchCount, err := env.eventRepo.CountByTitleID(movie.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, watchCount)
	assert.Nil(t, env.hub.GetActiveSession())

	tasks, err := env.taskRepo.ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeEnrichment, tasks[0].TaskType)
	require.NotNil(t, tasks[0].DedupKey)
	assert.Equal(t, "enrichment:"+strconv.FormatInt(movie.ID, 10), *tasks[0].DedupKey)

	var enrichPayload service.EnrichmentPayload
	require.NoError(t, json.Unmarshal([]byte(tasks[0].Payload), &enrichPayload))
	assert.Equal(t, movie.ID, enrichPayload.TitleID)
	assert.Equal(t, "Dune: Part Two", enrichPayload.TitleName)
	assert.Equal(t, 2024, enrichPayload.Year)
	assert.Equal(t, model.TitleTypeMovie, enrichPayload.TitleType)
	assert.Equal(t, "tt15239678", enrichPayload.IMDBID)
	assert.Equal(t, int64(693134), enrichPayload.TMDBID)
	assert.Equal(t, int64(12345), enrichPayload.TVDBID)
	assert.False(t, enrichPayload.PreserveMatch)

	// Re-sending identical ItemAdded webhook must be an idempotent no-op
	require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}"))
	resAfter, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	assert.Len(t, resAfter.Titles, 1)
	tasksAfter, err := env.taskRepo.ListPending()
	require.NoError(t, err)
	assert.Len(t, tasksAfter, 1)
}

func TestProcessJellyfinWebhook_ItemAdded_SeriesAndTaskWorkerBackfill(t *testing.T) {
	env := setupJellyfinTest(t)
	dataDir := t.TempDir()

	var pngBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 30, G: 90, B: 180, A: 255})
	require.NoError(t, png.Encode(&pngBuf, img))

	tmdbMux := http.NewServeMux()
	tmdbMux.HandleFunc("/tv/95396", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":               95396,
			"name":             "Severance",
			"overview":         "Mark leads a team of office workers whose memories have been surgically divided.",
			"first_air_date":   "2022-02-18",
			"poster_path":      "/sev.png",
			"episode_run_time": []int{50},
			"vote_average":     8.4,
			"genres":           []map[string]any{{"id": 18, "name": "Drama"}},
			"external_ids": map[string]any{
				"imdb_id": "tt11280740",
				"tvdb_id": 371980,
			},
		})
	})
	tmdbMux.HandleFunc("/tv/95396/translations", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"translations": []map[string]any{
				{"iso_639_1": "en", "data": map[string]string{"name": "Severance"}},
			},
		})
	})
	tmdbMux.HandleFunc("/image/sev.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBuf.Bytes())
	})
	tmdbServer := httptest.NewServer(tmdbMux)
	defer tmdbServer.Close()

	jf := &model.JellyfinPayload{
		NotificationType: "  itemadded  ",
		ItemType:         "Series",
		Name:             "Severance",
		Year:             "2022",
		ItemID:           "jf-series-sev",
		ProviderIMDB:     "tt11280740",
		ProviderTMDB:     "95396",
		ProviderTVDB:     "371980",
	}

	require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), jf, "{}"))

	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	series, err := env.titleRepo.GetByID(res.Titles[0].ID)
	require.NoError(t, err)
	assert.Equal(t, model.TitleTypeSeries, series.Type)
	assert.Equal(t, "Severance", series.PrimaryName())
	assert.Equal(t, 2022, series.Year)
	assert.Equal(t, model.TitleStatusPlanToWatch, series.Status)
	require.NotNil(t, series.ExternalSourceID)
	assert.Equal(t, "jf-series-sev", *series.ExternalSourceID)
	assert.Nil(t, series.DownloadedAt, "series added via ItemAdded must leave titles.downloaded_at nil")
	assert.True(t, series.ArrIgnored)
	assert.Equal(t, model.MatchStatusConfirmed, series.MatchStatus)
	require.NotNil(t, series.MatchSource)
	assert.Equal(t, "jellyfin_webhook", *series.MatchSource)

	watchCount, err := env.eventRepo.CountByTitleID(series.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, watchCount)
	assert.Nil(t, env.hub.GetActiveSession())

	// Execute enqueued enrichment task via TaskQueueWorker and verify refresh:<id> backfill is queued
	tmdbClient := matching.NewTMDBClient("test-key")
	tmdbClient.SetBaseURL(tmdbServer.URL)
	pipeline := matching.NewPipeline(tmdbClient, nil, nil, nil, dataDir)
	settingRepo := repository.NewSettingRepository(env.db)
	titleSvc := service.NewTitleService(env.db, env.titleRepo, env.taskRepo, pipeline)
	coverSvc := service.NewCoverService(env.db, env.titleRepo, tmdbClient, nil, dataDir)
	limiter := service.NewAPILimiter(20, 5)
	coverSvc.SetAPILimiter(limiter)

	worker := service.NewTaskQueueWorker(env.taskRepo, env.titleRepo, pipeline, tmdbClient, nil, nil, settingRepo, dataDir, titleSvc, env.db)
	worker.SetCovers(coverSvc)
	worker.SetAPILimiter(limiter)

	var due []model.Task
	require.NoError(t, database.WithTxContext(context.Background(), env.db, func(tx *sql.Tx) error {
		var fetchErr error
		due, fetchErr = repository.NewTaskWriter(tx).FetchDue(context.Background(), 10)
		return fetchErr
	}))
	require.Len(t, due, 1)
	assert.Equal(t, model.TaskTypeEnrichment, due[0].TaskType)

	require.NoError(t, limiter.Wait(context.Background()))
	worker.ProcessTask(context.Background(), due[0])

	enriched, err := env.titleRepo.GetByID(series.ID)
	require.NoError(t, err)
	require.NotNil(t, enriched.Overview)
	assert.Contains(t, *enriched.Overview, "surgically divided")
	require.NotNil(t, enriched.CoverURL)
	assert.Equal(t, "sev.png", *enriched.CoverURL)
	assert.NotNil(t, enriched.AccentHex)

	pendingAfter, err := env.taskRepo.ListPending()
	require.NoError(t, err)
	require.Len(t, pendingAfter, 1)
	assert.Equal(t, model.TaskTypeRefresh, pendingAfter[0].TaskType)
	require.NotNil(t, pendingAfter[0].DedupKey)
	assert.Equal(t, "refresh:"+strconv.FormatInt(series.ID, 10), *pendingAfter[0].DedupKey)
}

func TestProcessJellyfinWebhook_ItemAdded_EpisodeAndUnsupportedIgnored(t *testing.T) {
	cases := []struct {
		name string
		jf   *model.JellyfinPayload
	}{
		{
			name: "episode item ignored",
			jf: &model.JellyfinPayload{
				NotificationType: "ItemAdded",
				ItemType:         "Episode",
				Name:             "Pilot",
				SeriesName:       "Severance",
				SeriesID:         "jf-series-sev",
				Season:           "1",
				Episode:          "1",
				ItemID:           "jf-ep-1",
				ProviderTMDB:     "99999",
			},
		},
		{
			name: "season item ignored",
			jf: &model.JellyfinPayload{
				NotificationType: "ItemAdded",
				ItemType:         "Season",
				Name:             "Season 1",
				ItemID:           "jf-season-1",
			},
		},
		{
			name: "audio item ignored",
			jf: &model.JellyfinPayload{
				NotificationType: "ItemAdded",
				ItemType:         "Audio",
				Name:             "Soundtrack",
				ItemID:           "jf-audio-1",
			},
		},
		{
			name: "empty item type ignored",
			jf: &model.JellyfinPayload{
				NotificationType: "ItemAdded",
				ItemType:         "",
				Name:             "Mystery",
			},
		},
		{
			name: "unidentifiable movie with empty name and no provider IDs ignored",
			jf: &model.JellyfinPayload{
				NotificationType: "ItemAdded",
				ItemType:         "Movie",
				Name:             "   ",
				ItemID:           "jf-guid-only",
			},
		},
		{
			name: "nil payload safe",
			jf:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupJellyfinTest(t)
			require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), tc.jf, "{}"))

			res, err := env.titleRepo.List(repository.TitleFilter{})
			require.NoError(t, err)
			assert.Empty(t, res.Titles)

			tasks, err := env.taskRepo.ListPending()
			require.NoError(t, err)
			assert.Empty(t, tasks)

			var watchCount int
			require.NoError(t, env.db.QueryRow(`SELECT COUNT(*) FROM watch_events`).Scan(&watchCount))
			assert.Equal(t, 0, watchCount)
			assert.Nil(t, env.hub.GetActiveSession())
		})
	}
}

func TestProcessJellyfinWebhook_ItemAdded_NameFallbacksAndBoundaries(t *testing.T) {
	t.Run("series falls back to SeriesName when Name is empty", func(t *testing.T) {
		env := setupJellyfinTest(t)
		require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), &model.JellyfinPayload{
			NotificationType: "ItemAdded",
			ItemType:         "Series",
			Name:             "   ",
			SeriesName:       "Andor",
			Year:             "2022",
			ItemID:           "jf-andor",
		}, "{}"))

		found, err := env.titleRepo.FindByNameAndType("Andor", model.TitleTypeSeries, 2022)
		require.NoError(t, err)
		assert.Equal(t, "Andor", found.PrimaryName())
		require.NotNil(t, found.OriginalTitle)
		assert.Equal(t, "Andor", *found.OriginalTitle)
		assert.Equal(t, model.MatchStatusPendingReview, found.MatchStatus)
		assert.Nil(t, found.DownloadedAt)
	})

	t.Run("synthetic fallbacks for empty name with external ID and bounds clamping", func(t *testing.T) {
		env := setupJellyfinTest(t)

		// TMDB fallback + out-of-bounds negative year
		require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), &model.JellyfinPayload{
			NotificationType: "ItemAdded",
			ItemType:         "Movie",
			Year:             "-5",
			ProviderTMDB:     "603",
			ItemID:           "jf-tmdb-only",
		}, "{}"))

		// TVDB fallback + out-of-bounds future year + non-positive TMDB ignored
		require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), &model.JellyfinPayload{
			NotificationType: "ItemAdded",
			ItemType:         "Series",
			Year:             "2500",
			ProviderTMDB:     "-10",
			ProviderTVDB:     "81189",
			ItemID:           "jf-tvdb-only",
		}, "{}"))

		// IMDb fallback + non-numeric year
		require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), &model.JellyfinPayload{
			NotificationType: "ItemAdded",
			ItemType:         "Movie",
			Year:             "not-a-year",
			ProviderIMDB:     "TT0133093",
		}, "{}"))

		movieType := model.TitleTypeMovie
		seriesType := model.TitleTypeSeries

		tmdbMovie, err := env.titleRepo.FindByExternalID(nil, ptr(int64(603)), nil, nil, nil, &movieType)
		require.NoError(t, err)
		assert.Equal(t, "TMDB 603", tmdbMovie.PrimaryName())
		assert.Nil(t, tmdbMovie.OriginalTitle)
		assert.Equal(t, 0, tmdbMovie.Year)
		assert.Equal(t, model.MatchStatusConfirmed, tmdbMovie.MatchStatus)
		assert.NotNil(t, tmdbMovie.DownloadedAt)

		tvdbSeries, err := env.titleRepo.FindByExternalID(nil, nil, nil, nil, ptr(int64(81189)), &seriesType)
		require.NoError(t, err)
		assert.Equal(t, "TVDB 81189", tvdbSeries.PrimaryName())
		assert.Nil(t, tvdbSeries.OriginalTitle)
		assert.Equal(t, 0, tvdbSeries.Year)
		assert.Nil(t, tvdbSeries.TMDBID)
		assert.Equal(t, model.MatchStatusPendingReview, tvdbSeries.MatchStatus)
		assert.Nil(t, tvdbSeries.DownloadedAt)

		imdbMovie, err := env.titleRepo.FindByExternalID(ptr("tt0133093"), nil, nil, nil, nil, &movieType)
		require.NoError(t, err)
		assert.Equal(t, "tt0133093", imdbMovie.PrimaryName())
		assert.Nil(t, imdbMovie.OriginalTitle)
		assert.Nil(t, imdbMovie.ExternalSourceID)
		assert.Equal(t, 0, imdbMovie.Year)
		assert.Equal(t, model.MatchStatusPendingReview, imdbMovie.MatchStatus)
	})
}

func TestProcessJellyfinWebhook_ItemAdded_ExistingTitlesUntouchedAndSameYearGuard(t *testing.T) {
	env := setupJellyfinTest(t)

	rating9 := 9
	rating8 := 8
	rating3 := 3
	rating10 := 10

	idByExt := testutil.CreateTitle(t, env.db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2014, Status: model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed, ExternalSourceID: ptr("jf-existing-ext"), MyRating: &rating9,
	}, []model.TitleName{{Name: "Interstellar", Language: "en", IsPrimary: true}})
	testutil.CreateWatchEvent(t, env.db, &model.WatchEvent{TitleID: idByExt, Source: model.WatchEventSourceManual})

	idByTMDB := testutil.CreateTitle(t, env.db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2010, Status: model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed, TMDBID: ptr(int64(27205)), MyRating: &rating8,
	}, []model.TitleName{{Name: "Inception", Language: "en", IsPrimary: true}})

	idByIMDB := testutil.CreateTitle(t, env.db, &model.Title{
		Type: model.TitleTypeMovie, Year: 1999, Status: model.TitleStatusDropped,
		MatchStatus: model.MatchStatusConfirmed, IMDBID: ptr("tt0137523"), MyRating: &rating3,
	}, []model.TitleName{{Name: "Fight Club", Language: "en", IsPrimary: true}})

	idByTVDB := testutil.CreateTitle(t, env.db, &model.Title{
		Type: model.TitleTypeSeries, Year: 2008, Status: model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed, TVDBID: ptr(int64(81189)), MyRating: &rating10,
	}, []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}})
	season := testutil.UpsertSeason(t, env.db, idByTVDB, 1, 7)
	testutil.MarkEpisodesWatched(t, env.db, season.ID, 1)

	idByNameYear := testutil.CreateTitle(t, env.db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2021, Status: model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})

	// All 5 payloads match existing titles and must leave them completely untouched
	existingPayloads := []*model.JellyfinPayload{
		{NotificationType: "ItemAdded", ItemType: "Movie", Name: "Interstellar", Year: "2014", ItemID: "jf-existing-ext"},
		{NotificationType: "ItemAdded", ItemType: "Movie", Name: "Inception Renamed", Year: "2010", ProviderTMDB: "27205", ItemID: "jf-new-1"},
		{NotificationType: "ItemAdded", ItemType: "Movie", Name: "Fight Club", Year: "1999", ProviderIMDB: "TT0137523", ItemID: "jf-new-2"},
		{NotificationType: "ItemAdded", ItemType: "Series", Name: "Breaking Bad", Year: "2008", ProviderTVDB: "81189", ItemID: "jf-new-3"},
		{NotificationType: "ItemAdded", ItemType: "Movie", Name: "Dune", Year: "2021", ItemID: "jf-dune-2021"},
	}
	for _, p := range existingPayloads {
		require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), p, "{}"))
	}

	tasks, err := env.taskRepo.ListPending()
	require.NoError(t, err)
	assert.Empty(t, tasks, "matching existing titles must not enqueue enrichment tasks")

	gotExt, err := env.titleRepo.GetByID(idByExt)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusCompleted, gotExt.Status)
	require.NotNil(t, gotExt.MyRating)
	assert.Equal(t, 9, *gotExt.MyRating)
	assert.Nil(t, gotExt.DownloadedAt)
	weCount, err := env.eventRepo.CountByTitleID(idByExt)
	require.NoError(t, err)
	assert.Equal(t, 1, weCount)

	gotTMDB, err := env.titleRepo.GetByID(idByTMDB)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusWatching, gotTMDB.Status)
	assert.Nil(t, gotTMDB.ExternalSourceID)

	gotIMDB, err := env.titleRepo.GetByID(idByIMDB)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusDropped, gotIMDB.Status)

	gotTVDB, err := env.titleRepo.GetByID(idByTVDB)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusCompleted, gotTVDB.Status)
	require.NotNil(t, gotTVDB.MyRating)
	assert.Equal(t, 10, *gotTVDB.MyRating)
	require.Len(t, gotTVDB.Seasons, 1)
	require.Len(t, gotTVDB.Seasons[0].Episodes, 1)
	assert.True(t, gotTVDB.Seasons[0].Episodes[0].Watched)

	gotNameYear, err := env.titleRepo.GetByID(idByNameYear)
	require.NoError(t, err)
	assert.Equal(t, model.TitleStatusCompleted, gotNameYear.Status)

	// Different year (Dune 1984) must NOT collide with Dune 2021
	require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), &model.JellyfinPayload{
		NotificationType: "ItemAdded",
		ItemType:         "Movie",
		Name:             "Dune",
		Year:             "1984",
		ItemID:           "jf-dune-1984",
	}, "{}"))

	dune1984, err := env.titleRepo.FindByNameAndType("Dune", model.TitleTypeMovie, 1984)
	require.NoError(t, err)
	assert.NotEqual(t, idByNameYear, dune1984.ID)
	assert.Equal(t, model.TitleStatusPlanToWatch, dune1984.Status)

	// Same TMDB ID across different media types (Movie vs Series) must NOT collide
	require.NoError(t, env.svc.ProcessJellyfinWebhook(context.Background(), &model.JellyfinPayload{
		NotificationType: "ItemAdded",
		ItemType:         "Series",
		Name:             "Inception: The Series",
		Year:             "2026",
		ProviderTMDB:     "27205",
		ItemID:           "jf-series-same-tmdb",
	}, "{}"))

	seriesType := model.TitleTypeSeries
	createdSeries, err := env.titleRepo.FindByExternalID(nil, ptr(int64(27205)), nil, nil, nil, &seriesType)
	require.NoError(t, err)
	assert.NotEqual(t, idByTMDB, createdSeries.ID)
	assert.Equal(t, model.TitleStatusPlanToWatch, createdSeries.Status)
	assert.Nil(t, createdSeries.DownloadedAt)
}
