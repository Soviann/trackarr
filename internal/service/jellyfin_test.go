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
	}

	err = env.svc.ProcessJellyfinWebhook(context.Background(), startPayload, "{}")
	require.NoError(t, err)

	active := env.hub.GetActiveSession()
	require.NotNil(t, active)
	assert.Equal(t, "Stranger Things", active.TitleName)
	assert.Equal(t, "jf-item-ep1", active.Player)
	require.NotNil(t, active.SeasonNumber)
	assert.Equal(t, 1, *active.SeasonNumber)
	require.NotNil(t, active.EpisodeNumber)
	assert.Equal(t, 1, *active.EpisodeNumber)

	// 2. PlaybackStop (not played to completion): clears active session
	stopPayload := &model.JellyfinPayload{
		NotificationType:   "PlaybackStop",
		ItemID:             "jf-item-ep1",
		PlayedToCompletion: "False",
	}
	err = env.svc.ProcessJellyfinWebhook(context.Background(), stopPayload, "{}")
	require.NoError(t, err)
	assert.Nil(t, env.hub.GetActiveSession())
}
