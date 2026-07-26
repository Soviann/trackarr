package service_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jellyfinTestEnv struct {
	db        *sql.DB
	svc       *service.JellyfinService
	titleRepo *repository.TitleRepository
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

	jfSvc := service.NewJellyfinService(writeDB, nil, titleSvc, libSvc)

	return &jellyfinTestEnv{
		db:        writeDB,
		svc:       jfSvc,
		titleRepo: titleRepo,
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
