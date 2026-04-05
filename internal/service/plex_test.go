package service_test

import (
	"net/url"
	"testing"

	plexwebhooks "github.com/hekmon/plexwebhooks"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
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

	svc := service.NewPlexService(db, titleRepo, seasonRepo, episodeRepo, eventRepo, nil, service.NewNoopNotifier())
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
