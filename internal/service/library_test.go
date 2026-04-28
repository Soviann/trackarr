package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupLibraryService wires a LibraryService against an in-memory DB, mirroring
// the minimal dependencies Plex+manual paths need. Backfill and TMDB are nil:
// the tests care only about enqueue side effects, never auto-complete.
func setupLibraryService(t *testing.T) (*service.LibraryService, *sql.DB) {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titles := repository.NewTitleRepository(db)
	seasons := repository.NewSeasonRepository(db)
	episodes := repository.NewEpisodeRepository(db)
	events := repository.NewWatchEventRepository(db)
	settings := repository.NewSettingRepository(db)
	// Wire a real BackfillService — ToggleEpisodeWatched no longer nests its
	// tx, so this exercises the post-commit backfill path without deadlocking.
	backfill := service.NewBackfillService(db, nil)
	libSvc := service.NewLibraryService(db, titles, seasons, episodes, events, settings, service.NewNoopNotifier(), backfill, nil)
	return libSvc, db
}

// collectPendingTasks returns every non-dead task in the queue so assertions
// can inspect both the push kind and payload without relying on insertion
// order (SQLite orders by run_at which is identical at ms granularity).
func collectPendingTasks(t *testing.T, db *sql.DB) []model.Task {
	t.Helper()
	list, err := repository.NewTaskRepository(db).ListPending()
	require.NoError(t, err)
	return list
}

func TestToggleEpisodeWatched_EnqueuesAniListPushForSeason(t *testing.T) {
	libSvc, db := setupLibraryService(t)
	titleID := testutil.InsertTitle(t, db, "JJK", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	ep := testutil.GetOrCreateEpisode(t, db, seasonID, 1)

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		_, _, _, err := libSvc.ToggleEpisodeWatched(ctx, tx, titleID, ep.ID)
		return err
	}))

	tasks := collectPendingTasks(t, db)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeAniListPushSeason, tasks[0].TaskType)

	var payload service.AniListPushSeasonPayload
	require.NoError(t, json.Unmarshal([]byte(tasks[0].Payload), &payload))
	assert.Equal(t, seasonID, payload.SeasonID)
}

// TestToggleEpisodeWatched_DoesNotDeadlockAgainstBackfill guards against the
// regression we fixed: BackfillService opens its own writeDB transaction, so
// calling it inside ToggleEpisodeWatched's tx used to deadlock forever under
// MaxOpenConns=1. If someone reintroduces the nested call, this test hangs
// past its timeout — which pytest-style `-timeout` on go test cuts short.
func TestToggleEpisodeWatched_DoesNotDeadlockAgainstBackfill(t *testing.T) {
	libSvc, db := setupLibraryService(t)
	titleID := testutil.InsertTitle(t, db, "JJK", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	// Mark episode 1 as present + toggling episode 2: backfill will try to
	// mark episode 1 as watched during the post-commit trigger.
	testutil.GetOrCreateEpisode(t, db, seasonID, 1)
	ep2 := testutil.GetOrCreateEpisode(t, db, seasonID, 2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		_, _, _, err := libSvc.ToggleEpisodeWatched(ctx, tx, titleID, ep2.ID)
		return err
	}))
	// Post-commit trigger: fires the backfill path that previously deadlocked
	// when nested inside the tx. Must return within ctx; the 10s timeout
	// aborts if the bug is reintroduced.
	libSvc.TriggerBackfillForEpisode(ctx, titleID, ep2)
}

// Unwatching still enqueues: the derived status flips from CURRENT/COMPLETED
// back to PLANNING or CURRENT, so AniList must be told about the regression.
func TestToggleEpisodeUnwatched_EnqueuesAniListPush(t *testing.T) {
	libSvc, db := setupLibraryService(t)
	titleID := testutil.InsertTitle(t, db, "JJK", true)
	seasonID := testutil.InsertSeason(t, db, titleID, 1)
	ep := testutil.GetOrCreateEpisode(t, db, seasonID, 1)

	// First toggle: watched → 1 task enqueued.
	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		_, _, _, err := libSvc.ToggleEpisodeWatched(ctx, tx, titleID, ep.ID)
		return err
	}))
	require.Len(t, collectPendingTasks(t, db), 1)

	// Second toggle: unwatched → another task enqueued.
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		_, _, _, err := libSvc.ToggleEpisodeWatched(ctx, tx, titleID, ep.ID)
		return err
	}))
	tasks := collectPendingTasks(t, db)
	require.Len(t, tasks, 2)
	for _, task := range tasks {
		assert.Equal(t, model.TaskTypeAniListPushSeason, task.TaskType)
	}
}

// Manual batch-watch can cover episodes from multiple seasons (backlog
// catch-up). One push per distinct season, no duplicates.
func TestMarkEpisodesWatched_EnqueuesDistinctSeasonPushes(t *testing.T) {
	libSvc, db := setupLibraryService(t)
	titleID := testutil.InsertTitle(t, db, "JJK", true)
	s1 := testutil.InsertSeason(t, db, titleID, 1)
	s2 := testutil.InsertSeason(t, db, titleID, 2)

	s1e1 := testutil.GetOrCreateEpisode(t, db, s1, 1)
	s1e2 := testutil.GetOrCreateEpisode(t, db, s1, 2)
	s2e1 := testutil.GetOrCreateEpisode(t, db, s2, 1)

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		_, _, err := libSvc.MarkEpisodesWatched(ctx, tx, titleID,
			[]int64{s1e1.ID, s1e2.ID, s2e1.ID}, nil,
			model.WatchEventSourceManual, nil)
		return err
	}))

	tasks := collectPendingTasks(t, db)
	require.Len(t, tasks, 2)

	gotSeasons := map[int64]bool{}
	for _, task := range tasks {
		assert.Equal(t, model.TaskTypeAniListPushSeason, task.TaskType)
		var payload service.AniListPushSeasonPayload
		require.NoError(t, json.Unmarshal([]byte(task.Payload), &payload))
		gotSeasons[payload.SeasonID] = true
	}
	assert.True(t, gotSeasons[s1], "s1 push missing")
	assert.True(t, gotSeasons[s2], "s2 push missing")
}

// Anime movies scrobbled via Plex reach MarkMovieWatched, never the PATCH
// handler. The movie push must be enqueued from the library layer.
func TestMarkMovieWatched_AnimeMovie_EnqueuesMoviePush(t *testing.T) {
	libSvc, db := setupLibraryService(t)
	titleID := testutil.InsertMovieTitle(t, db, "Your Name", 21519)

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		_, err := libSvc.MarkMovieWatched(ctx, tx, titleID, model.WatchEventSourceManual, nil)
		return err
	}))

	tasks := collectPendingTasks(t, db)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeAniListPushMovie, tasks[0].TaskType)

	var payload service.AniListPushMoviePayload
	require.NoError(t, json.Unmarshal([]byte(tasks[0].Payload), &payload))
	assert.Equal(t, titleID, payload.TitleID)
}

// Non-anime or AniList-less movies must not pollute the queue — the push
// service would skip them anyway, but creating the task is pure waste.
func TestMarkMovieWatched_NonAnimeMovie_NoPushEnqueued(t *testing.T) {
	libSvc, db := setupLibraryService(t)
	titleID := testutil.CreateTitle(t, db,
		&model.Title{
			Type:        model.TitleTypeMovie,
			IsAnime:     false,
			Year:        2024,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
		},
		[]model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}},
	)

	ctx := context.Background()
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		_, err := libSvc.MarkMovieWatched(ctx, tx, titleID, model.WatchEventSourceManual, nil)
		return err
	}))

	assert.Empty(t, collectPendingTasks(t, db))
}
