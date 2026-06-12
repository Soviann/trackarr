package service_test

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupImporter(t *testing.T, opts ...service.SimklImporterOption) *service.SimklImporter {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	return service.NewSimklImporter(
		db,
		repository.NewTitleRepository(db),
		repository.NewSeasonRepository(db),
		repository.NewEpisodeRepository(db),
		repository.NewWatchEventRepository(db),
		opts...,
	)
}

type testDeps struct {
	importer *service.SimklImporter
	tasks    *repository.TaskRepository
	episodes *repository.EpisodeRepository
	db       *sql.DB
}

func setupImporterWithDB(t *testing.T) testDeps {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	taskRepo := repository.NewTaskRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	importer := service.NewSimklImporter(
		db,
		repository.NewTitleRepository(db),
		repository.NewSeasonRepository(db),
		episodeRepo,
		repository.NewWatchEventRepository(db),
		service.WithTaskRepository(taskRepo),
	)
	return testDeps{importer: importer, tasks: taskRepo, episodes: episodeRepo, db: db}
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

func TestSimklImport_EnqueuesEnrichment(t *testing.T) {
	deps := setupImporterWithDB(t)

	backup := &service.SimklBackup{
		Movies: []service.SimklItem{{
			Status: "completed",
			Movie:  &service.SimklMedia{Title: "Dune", Year: 2024, IDs: service.SimklIDs{IMDB: "tt1234567", TMDB: 693134}},
		}},
	}

	_, err := deps.importer.Import(backup, false)
	require.NoError(t, err)

	tasks, err := deps.tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, "enrichment", string(tasks[0].TaskType))
	assert.Contains(t, tasks[0].Payload, `"title_name":"Dune"`)
	assert.Contains(t, tasks[0].Payload, `"tmdb_id":693134`)
}

func TestSimklImport_BackfillsPreviousEpisodes(t *testing.T) {
	deps := setupImporterWithDB(t)

	backup := &service.SimklBackup{
		Shows: []service.SimklItem{{
			Status: "watching",
			Show:   &service.SimklMedia{Title: "Test Show", Year: 2020, IDs: service.SimklIDs{IMDB: "tt9999999"}},
			Seasons: []service.SimklSeason{{
				Number: 1,
				Episodes: []service.SimklEpisode{
					{Number: 3, WatchedAt: "2020-03-01T00:00:00Z"},
				},
			}},
		}},
	}

	_, err := deps.importer.Import(backup, false)
	require.NoError(t, err)

	// Season 1 should exist with episodes 1, 2, 3
	season := testutil.GetOrCreateSeason(t, deps.db, 1, 1) // titleID=1, seasonNum=1

	episodes, err := deps.episodes.GetBySeasonID(season.ID)
	require.NoError(t, err)
	require.Len(t, episodes, 3) // ep 3 from import + ep 1, 2 from backfill

	// All three should be watched
	for _, ep := range episodes {
		assert.True(t, ep.Watched, "episode %d should be watched", ep.Episode)
	}
}

func TestSimklImport_SimklIDsAndAniListEnrichment(t *testing.T) {
	deps := setupImporterWithDB(t)

	const backupJSON = `{
		"anime": [{
			"status": "completed",
			"show": {
				"title": "Dance in the Vampire Bund",
				"year": 2010,
				"ids": {
					"simkl": 36048,
					"slug": "dance-in-the-vampire-bund",
					"anilist": "6747"
				}
			}
		}]
	}`

	var backup service.SimklBackup
	require.NoError(t, json.Unmarshal([]byte(backupJSON), &backup))

	result, err := deps.importer.Import(&backup, false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)

	// Check the title has SimklID and SimklSlug set
	titles, err := repository.NewTitleRepository(deps.db).ListAll()
	require.NoError(t, err)
	require.Len(t, titles, 1)
	title := titles[0]
	require.NotNil(t, title.SimklID, "SimklID should be set")
	assert.Equal(t, int64(36048), *title.SimklID)
	require.NotNil(t, title.SimklSlug, "SimklSlug should be set")
	assert.Equal(t, "dance-in-the-vampire-bund", *title.SimklSlug)

	// Check the enqueued enrichment task carries the AniList id
	tasks, err := deps.tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, "enrichment", string(tasks[0].TaskType))
	var p service.EnrichmentPayload
	require.NoError(t, json.Unmarshal([]byte(tasks[0].Payload), &p))
	assert.Equal(t, int64(6747), p.AniListID)
}

func intPtr(i int) *int { return &i }
