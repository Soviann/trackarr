package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTitleService_CreateFromPlex_DuplicateDetection(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	// 1. Pre-existing title (e.g. from Simkl)
	tmdbID := int64(119335)
	existingID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2022,
		Status:      model.TitleStatusCompleted,
		TMDBID:      &tmdbID,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "In the Land of Leadale", Language: "en", IsPrimary: true}})

	// 2. Setup Pipeline mock
	mux := http.NewServeMux()
	mux.HandleFunc("/search/tv", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"id":119335,"name":"In the Land of Leadale","first_air_date":"2022-01-05"}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tmdbClient := matching.NewTMDBClient("test")
	tmdbClient.SetBaseURL(server.URL)
	pipeline := matching.NewPipeline(tmdbClient, nil, nil, nil, t.TempDir())

	svc := service.NewTitleService(db, titleRepo, taskRepo, pipeline)

	// 3. Create from Scrobble (matches existing TMDB ID)
	ids := service.ExternalIDs{} // Empty IDs, let pipeline match
	ratingKey := "plex-123"
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback() //nolint:errcheck
	newID, err := svc.CreateFromScrobble(context.Background(), tx, "In the Land of Leadale", 2022, ids, model.TitleTypeSeries, ratingKey, nil, model.TitleStatusWatching)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// Verify it returned the existing ID
	assert.Equal(t, existingID, newID)

	// Verify the existing title was updated with Plex rating key
	title, _ := titleRepo.GetByID(existingID)
	require.NotNil(t, title.PlexRatingKey)
	assert.Equal(t, "plex-123", *title.PlexRatingKey)
}

func TestMerge_OpensOwnTx(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	svc := service.NewTitleService(db, titleRepo, taskRepo, nil)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2020,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Dest Title", Language: "en", IsPrimary: true}})

	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2021,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Source Title", Language: "en", IsPrimary: true}})

	err = svc.Merge(context.Background(), db, destID, sourceID, nil)
	require.NoError(t, err)

	// Source title must be deleted after merge.
	_, err = titleRepo.GetByID(sourceID)
	assert.True(t, errors.Is(err, sql.ErrNoRows), "expected sql.ErrNoRows, got: %v", err)
}

func TestMerge_StampsSourceAniListOnDestSeason(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	svc := service.NewTitleService(db, titleRepo, taskRepo, nil)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2020,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Jujutsu Kaisen", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, destID, 1)

	srcAniList := int64(145064)
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2023,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		AniListID:   &srcAniList,
	}, []model.TitleName{{Name: "JJK S2", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 1)

	offset := 1
	require.NoError(t, svc.Merge(context.Background(), db, destID, sourceID, &offset))

	var destS2 int64
	require.NoError(t, db.QueryRow(`SELECT id FROM seasons WHERE title_id = ? AND season_number = 2`, destID).Scan(&destS2))
	got, err := testutil.GetSeasonExternalID(t, db, destS2, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "145064", got)
}

func TestSetExternalIDs_ClearsRoutesAniListAndLocks(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	svc := service.NewTitleService(db, titleRepo, taskRepo, nil)

	tmdb, imdb, tvdb, anilist := int64(316694), "tt475306", int64(475306), int64(209219)
	id := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, IsAnime: true, Year: 2020,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
		TMDBID: &tmdb, IMDBID: &imdb, TVDBID: &tvdb, AniListID: &anilist,
	}, []model.TitleName{{Name: "Anime", Language: "en", IsPrimary: true}})
	seasonID := testutil.InsertSeason(t, db, id, 1)

	// Toggle off: clear TVDB, keep TMDB/IMDB, route AniList to the season.
	newAniList := int64(209219)
	require.NoError(t, svc.SetExternalIDs(context.Background(), db, id, service.ExternalIDEdit{
		TMDBID:          &tmdb,
		IMDBID:          &imdb,
		TVDBID:          nil, // emptied → clear
		AniListID:       &newAniList,
		AniListSeasonID: &seasonID,
		AutoFill:        false,
	}))

	got, err := titleRepo.GetByID(id)
	require.NoError(t, err)
	assert.Nil(t, got.TVDBID, "emptied TVDB cleared to NULL")
	require.NotNil(t, got.TMDBID)
	assert.Equal(t, tmdb, *got.TMDBID, "kept TMDB")
	require.NotNil(t, got.MatchSource)
	assert.Equal(t, "manual", *got.MatchSource, "marked manually matched")

	seasonAniList, err := testutil.GetSeasonExternalID(t, db, seasonID, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "209219", seasonAniList, "AniList routed to the season mapping")

	// Toggle off locks every ID so the enrichment refresh can't rewrite them.
	var payloadJSON string
	require.NoError(t, db.QueryRow(
		`SELECT payload FROM task_queue WHERE task_type = 'enrichment' ORDER BY id DESC LIMIT 1`,
	).Scan(&payloadJSON))
	var payload service.EnrichmentPayload
	require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))
	assert.True(t, payload.PreserveMatch, "manual edit preserves match state")
	assert.ElementsMatch(t,
		[]string{service.LockTMDB, service.LockIMDB, service.LockTVDB, service.LockAniList},
		payload.LockedIDs, "toggle off locks all IDs")
}

func TestSetExternalIDs_AutoFillLocksOnlyProvided(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	svc := service.NewTitleService(db, titleRepo, taskRepo, nil)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2014,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Movie", Language: "en", IsPrimary: true}})

	// Toggle on: provide only TMDB; leave the rest empty for back-fill.
	tmdb := int64(550)
	require.NoError(t, svc.SetExternalIDs(context.Background(), db, id, service.ExternalIDEdit{
		TMDBID:   &tmdb,
		AutoFill: true,
	}))

	var payloadJSON string
	require.NoError(t, db.QueryRow(
		`SELECT payload FROM task_queue WHERE task_type = 'enrichment' ORDER BY id DESC LIMIT 1`,
	).Scan(&payloadJSON))
	var payload service.EnrichmentPayload
	require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))
	assert.Equal(t, []string{service.LockTMDB}, payload.LockedIDs,
		"auto-fill locks only the provided ID, leaving blanks open for matching")
}

// TestSetExternalIDs_IMDBOnlyAnchorEnqueuesEnrichment guards the share/rematch
// path where the user pastes only an IMDb id and ticks "auto-find the other
// IDs". An IMDb id is a valid enrichment anchor — TMDB's /find/{imdb_id}
// resolves the rest and plexIDStrategy short-circuits before any fuzzy name
// search — so enrichment must run even though no TMDB id was supplied.
// Previously the service bailed whenever TMDB was nil, so auto-find silently
// did nothing.
func TestSetExternalIDs_IMDBOnlyAnchorEnqueuesEnrichment(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	svc := service.NewTitleService(db, titleRepo, taskRepo, nil)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeMovie, Year: 2025,
		Status: model.TitleStatusPlanToWatch, MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Placeholder", Language: "en", IsPrimary: true}})

	imdb := "tt31974288"
	require.NoError(t, svc.SetExternalIDs(context.Background(), db, id, service.ExternalIDEdit{
		IMDBID:   &imdb,
		AutoFill: true,
	}))

	var payloadJSON string
	require.NoError(t, db.QueryRow(
		`SELECT payload FROM task_queue WHERE task_type = 'enrichment' ORDER BY id DESC LIMIT 1`,
	).Scan(&payloadJSON))
	var payload service.EnrichmentPayload
	require.NoError(t, json.Unmarshal([]byte(payloadJSON), &payload))
	assert.Equal(t, "tt31974288", payload.IMDBID,
		"IMDb id carried into the enrichment payload as the anchor")
	assert.Equal(t, []string{service.LockIMDB}, payload.LockedIDs,
		"auto-fill locks only the user-supplied IMDb id, leaving TMDB/TVDB/AniList open for back-fill")
}

func TestMerge_ReSearchesAniListWhenSourceLacksID(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	// Fake AniList endpoint returning a canned top result.
	alServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"Page": map[string]any{
					"media": []any{
						map[string]any{
							"id": 123,
							"title": map[string]string{
								"romaji":  "Sequel",
								"english": "Sequel",
							},
							"format":     "TV",
							"seasonYear": 2023,
						},
					},
				},
			},
		})
	}))
	defer alServer.Close()

	alClient := matching.NewAniListClientWithURL(alServer.URL)
	pipeline := matching.NewPipeline(nil, alClient, nil, nil, t.TempDir())

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	svc := service.NewTitleService(db, titleRepo, taskRepo, pipeline)

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2020,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Parent", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, destID, 1)

	// Source has no AniList ID — service must re-query AniList.
	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2023,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Sequel", Language: "en", IsPrimary: true}})
	_ = testutil.InsertSeason(t, db, sourceID, 1)

	offset := 1
	require.NoError(t, svc.Merge(context.Background(), db, destID, sourceID, &offset))

	var destS2 int64
	require.NoError(t, db.QueryRow(`SELECT id FROM seasons WHERE title_id = ? AND season_number = 2`, destID).Scan(&destS2))
	got, err := testutil.GetSeasonExternalID(t, db, destS2, "anilist")
	require.NoError(t, err)
	assert.Equal(t, "123", got)

	destTitle, err := titleRepo.GetByID(destID)
	require.NoError(t, err)
	require.NotNil(t, destTitle.AniListID)
	assert.Equal(t, int64(123), *destTitle.AniListID, "anilist_id backfilled on destination title row")
}

func TestMerge_CancelledContext(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	svc := service.NewTitleService(db, titleRepo, taskRepo, nil)

	sourceID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2021,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Source Title", Language: "en", IsPrimary: true}})

	destID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2020,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Dest Title", Language: "en", IsPrimary: true}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling Merge

	err = svc.Merge(ctx, db, destID, sourceID, nil)
	assert.Error(t, err)

	// Source title must still exist — merge was aborted.
	src, getErr := titleRepo.GetByID(sourceID)
	require.NoError(t, getErr)
	assert.Equal(t, sourceID, src.ID)
}
