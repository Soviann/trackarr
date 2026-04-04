package service_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupImporter(t *testing.T) *service.SimklImporter {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	return service.NewSimklImporter(
		repository.NewTitleRepository(db),
		repository.NewSeasonRepository(db),
		repository.NewEpisodeRepository(db),
		repository.NewWatchEventRepository(db),
	)
}

func TestSimklImport_Movie(t *testing.T) {
	importer := setupImporter(t)

	backup := &service.SimklBackup{
		Movies: []service.SimklItem{{
			Status:        "completed",
			UserRating:    intPtr(9),
			LastWatchedAt: "2024-03-15T20:00:00Z",
			Movie:         &service.SimklMedia{Title: "Dune: Part Two", Year: 2024, IDs: service.SimklIDs{IMDB: "tt15239678", TMDB: 693134}},
		}},
	}

	result, err := importer.Import(backup, false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 0, result.Skipped)
}

func TestSimklImport_DuplicateSkip(t *testing.T) {
	importer := setupImporter(t)

	backup := &service.SimklBackup{
		Movies: []service.SimklItem{{
			Status: "completed",
			Movie:  &service.SimklMedia{Title: "Dune", Year: 2024, IDs: service.SimklIDs{IMDB: "tt1234567"}},
		}},
	}

	result1, _ := importer.Import(backup, false)
	assert.Equal(t, 1, result1.Created)

	result2, _ := importer.Import(backup, false)
	assert.Equal(t, 0, result2.Created)
	assert.Equal(t, 1, result2.Skipped)
}

func TestSimklImport_ShowWithEpisodes(t *testing.T) {
	importer := setupImporter(t)

	backup := &service.SimklBackup{
		Shows: []service.SimklItem{{
			Status: "completed",
			Show:   &service.SimklMedia{Title: "Breaking Bad", Year: 2008, IDs: service.SimklIDs{IMDB: "tt0903747", TMDB: 1396}},
			Seasons: []service.SimklSeason{{
				Number: 1,
				Episodes: []service.SimklEpisode{
					{Number: 1, WatchedAt: "2020-01-01T00:00:00Z"},
					{Number: 2, WatchedAt: "2020-01-02T00:00:00Z"},
				},
			}},
		}},
	}

	result, err := importer.Import(backup, false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
}

func TestSimklImport_AnimeMovie(t *testing.T) {
	importer := setupImporter(t)

	backup := &service.SimklBackup{
		Anime: []service.SimklItem{{
			Status:    "completed",
			AnimeType: "movie",
			Show:      &service.SimklMedia{Title: "Your Name", Year: 2016, IDs: service.SimklIDs{AniList: 21519}},
		}},
	}

	result, err := importer.Import(backup, false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
}

func TestSimklImport_StatusMapping(t *testing.T) {
	importer := setupImporter(t)

	backup := &service.SimklBackup{
		Movies: []service.SimklItem{
			{Status: "plantowatch", Movie: &service.SimklMedia{Title: "A", Year: 2024, IDs: service.SimklIDs{IMDB: "tt0000001"}}},
			{Status: "notinteresting", Movie: &service.SimklMedia{Title: "B", Year: 2024, IDs: service.SimklIDs{IMDB: "tt0000002"}}},
			{Status: "hold", Movie: &service.SimklMedia{Title: "C", Year: 2024, IDs: service.SimklIDs{IMDB: "tt0000003"}}},
		},
	}

	result, err := importer.Import(backup, false)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Created)
}

func intPtr(i int) *int { return &i }
