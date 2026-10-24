package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type plexTestEnv struct {
	db        *sql.DB
	svc       *service.PlexService
	titleRepo *repository.TitleRepository
	eventRepo *repository.WatchEventRepository
	hub       *service.PlaybackHubService
}

func setupPlexTest(t *testing.T) *plexTestEnv {
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

	plexSvc := service.NewPlexService(writeDB, nil, titleSvc, libSvc, playbackHub)

	return &plexTestEnv{
		db:        writeDB,
		svc:       plexSvc,
		titleRepo: titleRepo,
		eventRepo: eventRepo,
		hub:       playbackHub,
	}
}

func TestProcessPlexWebhook_Movie(t *testing.T) {
	env := setupPlexTest(t)

	p := &model.PlexPayload{
		Event: "media.scrobble",
		Metadata: model.PlexMetadata{
			Type:      "movie",
			Title:     "Inception",
			Year:      2010,
			RatingKey: "12345",
			Guid: []model.PlexGUIDItem{
				{ID: "imdb://tt1375666"},
				{ID: "tmdb://27205"},
			},
		},
	}

	err := env.svc.ProcessPlexWebhook(context.Background(), p, `{"raw":"movie"}`)
	require.NoError(t, err)

	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	movie := res.Titles[0]
	assert.Equal(t, "Inception", movie.PrimaryName())
	assert.Equal(t, model.TitleTypeMovie, movie.Type)
	assert.Equal(t, model.TitleStatusCompleted, movie.Status)
	require.NotNil(t, movie.IMDBID)
	assert.Equal(t, "tt1375666", *movie.IMDBID)
	require.NotNil(t, movie.TMDBID)
	assert.Equal(t, int64(27205), *movie.TMDBID)
	require.NotNil(t, movie.ExternalSourceID)
	assert.Equal(t, "12345", *movie.ExternalSourceID)

	events, err := env.eventRepo.ListByTitle(movie.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, model.WatchEventSourcePlex, events[0].Source)
	require.NotNil(t, events[0].RawPayload)
	assert.Equal(t, `{"raw":"movie"}`, *events[0].RawPayload)
}

func TestProcessPlexWebhook_Episode(t *testing.T) {
	env := setupPlexTest(t)

	p := &model.PlexPayload{
		Event: "media.scrobble",
		Metadata: model.PlexMetadata{
			Type:                 "episode",
			Title:                "Winter Is Coming",
			GrandparentTitle:     "Game of Thrones",
			GrandparentRatingKey: "65000",
			Year:                 2011,
			ParentIndex:          1,
			Index:                1,
			RatingKey:            "65001",
		},
	}

	err := env.svc.ProcessPlexWebhook(context.Background(), p, `{"raw":"episode"}`)
	require.NoError(t, err)

	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	show := res.Titles[0]
	assert.Equal(t, "Game of Thrones", show.PrimaryName())
	assert.Equal(t, model.TitleTypeSeries, show.Type)
	assert.Equal(t, model.TitleStatusWatching, show.Status)
	require.NotNil(t, show.ExternalSourceID)
	assert.Equal(t, "65000", *show.ExternalSourceID)

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

func TestProcessPlexWebhook_IgnoredNonScrobble(t *testing.T) {
	env := setupPlexTest(t)

	cases := []struct {
		name string
		p    *model.PlexPayload
	}{
		{"play event", &model.PlexPayload{Event: "media.play", Metadata: model.PlexMetadata{Type: "movie", Title: "Inception"}}},
		{"pause event", &model.PlexPayload{Event: "media.pause", Metadata: model.PlexMetadata{Type: "movie", Title: "Inception"}}},
		{"stop event", &model.PlexPayload{Event: "media.stop", Metadata: model.PlexMetadata{Type: "movie", Title: "Inception"}}},
		{"unsupported type", &model.PlexPayload{Event: "media.scrobble", Metadata: model.PlexMetadata{Type: "track", Title: "Song"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := env.svc.ProcessPlexWebhook(context.Background(), tc.p, "{}")
			require.NoError(t, err)

			res, err := env.titleRepo.List(repository.TitleFilter{})
			require.NoError(t, err)
			assert.Empty(t, res.Titles)
		})
	}
}

func TestProcessPlexWebhook_RealProductionPayload(t *testing.T) {
	env := setupPlexTest(t)

	// Raw JSON payload sample for Plex webhook scrobble
	const prodJSON = `{"Rating":0,"event":"media.scrobble","user":true,"owner":true,"Account":{"id":100001,"title":"TestUser"},"Server":{"title":"MediaServer","uuid":"0000000000000000000000000000000000000001"},"Player":{"local":true,"PublicAddress":"203.0.113.10","title":"Smart TV","uuid":"player-uuid-0001"},"Metadata":{"addedAt":"2026-03-03T18:56:50Z","grandparentRatingKey":"65538","grandparentTitle":"Dr. Globule","Guid":[{"Scheme":"tvdb","Host":"5228295"}],"index":23,"key":"/library/metadata/65562","parentIndex":1,"parentRatingKey":"65539","parentTitle":"Season 1","ratingKey":"65562","title":"Ronchons et dragons","type":"episode","year":1994}}`

	var p model.PlexPayload
	require.NoError(t, json.Unmarshal([]byte(prodJSON), &p))

	err := env.svc.ProcessPlexWebhook(context.Background(), &p, prodJSON)
	require.NoError(t, err)

	res, err := env.titleRepo.List(repository.TitleFilter{})
	require.NoError(t, err)
	require.Len(t, res.Titles, 1)

	show := res.Titles[0]
	assert.Equal(t, "Dr. Globule", show.PrimaryName())
	assert.Equal(t, model.TitleTypeSeries, show.Type)
	assert.Equal(t, 1994, show.Year)
	require.NotNil(t, show.ExternalSourceID)
	assert.Equal(t, "65538", *show.ExternalSourceID)

	var seasonID int64
	err = env.db.QueryRow(`SELECT id FROM seasons WHERE title_id = ? AND season_number = 1`, show.ID).Scan(&seasonID)
	require.NoError(t, err)

	var watched bool
	err = env.db.QueryRow(`SELECT watched FROM episodes WHERE season_id = ? AND episode = 23`, seasonID).Scan(&watched)
	require.NoError(t, err)
	assert.True(t, watched)
}

func TestProcessPlexWebhook_PlayAndStop(t *testing.T) {
	env := setupPlexTest(t)

	// Pre-seed title in library
	res, err := env.db.Exec(`INSERT INTO titles (type, year, status, match_status, external_source_id) VALUES ('movie', 2010, 'watching', 'confirmed', '99001')`)
	require.NoError(t, err)
	titleID, err := res.LastInsertId()
	require.NoError(t, err)
	_, err = env.db.Exec(`INSERT INTO title_names (title_id, name, language, is_primary) VALUES (?, 'Inception', 'en', 1)`, titleID)
	require.NoError(t, err)

	// 1. media.play event: resolves title, puts active session in hub, but does NOT mark watched
	playPayload := &model.PlexPayload{
		Event: "media.play",
		Player: model.PlexPlayer{
			Title: "Living Room Shield",
		},
		Account: model.PlexAccount{
			Title: "alice",
		},
		Metadata: model.PlexMetadata{
			RatingKey:  "99001",
			Type:       "movie",
			Title:      "Inception",
			Year:       2010,
			Duration:   8880000,
			ViewOffset: 120000,
		},
	}

	err = env.svc.ProcessPlexWebhook(context.Background(), playPayload, "{}")
	require.NoError(t, err)

	active := env.hub.GetActiveSession()
	require.NotNil(t, active)
	assert.Equal(t, "Inception", active.TitleName)
	assert.Equal(t, "Living Room Shield", active.Player)
	assert.Equal(t, int64(8880), active.DurationSec)
	assert.Equal(t, int64(120), active.ViewOffsetSec)

	// Ensure no watch events exist in DB
	count, err := env.eventRepo.CountByTitleID(active.TitleID)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "media.play must NOT insert watch event")

	// 2. media.stop event: clears the active session from hub
	stopPayload := &model.PlexPayload{
		Event: "media.stop",
		Player: model.PlexPlayer{
			Title: "Living Room Shield",
		},
	}
	err = env.svc.ProcessPlexWebhook(context.Background(), stopPayload, "{}")
	require.NoError(t, err)
	assert.Nil(t, env.hub.GetActiveSession())
}
