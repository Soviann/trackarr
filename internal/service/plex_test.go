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

	plexSvc := service.NewPlexService(writeDB, nil, titleSvc, libSvc)

	return &plexTestEnv{
		db:        writeDB,
		svc:       plexSvc,
		titleRepo: titleRepo,
		eventRepo: eventRepo,
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

	// Real raw JSON extracted from production database
	const prodJSON = `{"Rating":0,"event":"media.scrobble","user":true,"owner":true,"Account":{"id":2765005,"title":"Sovian"},"Server":{"title":"GITS","uuid":"214b579771792f56ebd4af54b9c3eb4d8000b68e"},"Player":{"local":true,"PublicAddress":"109.9.244.233","title":"LG OLED55C25LB","uuid":"0u61ssozchvtm5z6btqevq11"},"Metadata":{"addedAt":"2026-03-03T18:56:50Z","grandparentRatingKey":"65538","grandparentTitle":"Dr. Globule","Guid":[{"Scheme":"tvdb","Host":"5228295"}],"index":23,"key":"/library/metadata/65562","parentIndex":1,"parentRatingKey":"65539","parentTitle":"Season 1","ratingKey":"65562","title":"Ronchons et dragons","type":"episode","year":1994}}`

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
