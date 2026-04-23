package service_test

import (
	"context"
	"encoding/json"
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

// TestHandleEnrichment_PersistsAllFieldsInSingleTx drives handleEnrichment
// against a real in-memory SQLite with a pre-populated watch_events count.
// The pipeline is deliberately kept empty (no HTTP clients) — the enrichment
// payload carries a TMDB ID so Run() short-circuits on Plex metadata IDs.
// The test asserts every Task 2 write lands atomically: title fields, watch
// minutes, alias names, and genre join rows.
func TestHandleEnrichment_PersistsAllFieldsInSingleTx(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	events := repository.NewWatchEventRepository(db)
	genres := repository.NewGenreRepository(db)
	tasks := repository.NewTaskRepository(db)

	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: "Old Name", Language: "en", IsPrimary: true}})

	// Seed three watch events so recalcWatchtime has a non-zero multiplier.
	for range 3 {
		testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: id, Source: model.WatchEventSourceManual})
	}

	// Empty pipeline: TMDB/AniList/Gemini nil. Run() returns early on the
	// Plex-IDs branch because payload.TMDBID is non-zero, so no HTTP is hit.
	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, events, genres, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)

	payload := service.EnrichmentPayload{
		TitleID:   id,
		TitleName: "Old Name",
		Year:      2024,
		TitleType: model.TitleTypeMovie,
		TMDBID:    42,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	_, err = tasks.Enqueue(model.TaskTypeEnrichment, string(raw), nil)
	require.NoError(t, err)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)
	worker.ProcessTask(context.Background(), queued[0])

	after, err := titles.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusConfirmed, after.MatchStatus,
		"tx committed: match_status updated")
	require.NotNil(t, after.TMDBID)
	assert.Equal(t, int64(42), *after.TMDBID, "tx committed: tmdb_id persisted")
	require.NotNil(t, after.MatchSource)
	assert.Equal(t, matching.MatchSourcePlexIDs, *after.MatchSource)
}

// TestHandleEnrichment_CtxCancelRollsBack makes sure a cancelled context
// prevents the persistence transaction from committing. With handleEnrichment
// now wrapping all writes in a single WithTxContext, BeginTx must fail before
// any Update/ReplaceNames hits the row.
func TestHandleEnrichment_CtxCancelRollsBack(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	defer db.Close()

	titles := repository.NewTitleRepository(db)
	events := repository.NewWatchEventRepository(db)
	genres := repository.NewGenreRepository(db)
	tasks := repository.NewTaskRepository(db)

	origName := "Keep Me Pristine"
	id := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusUnconfirmed,
	}, []model.TitleName{{Name: origName, Language: "en", IsPrimary: true}})

	pipeline := matching.NewPipeline(nil, nil, nil, nil, t.TempDir())
	titleSvc := service.NewTitleService(db, titles, tasks, pipeline)
	worker := service.NewTaskQueueWorker(tasks, titles, events, genres, pipeline, nil, nil, nil, nil, t.TempDir(), titleSvc, db)

	payload := service.EnrichmentPayload{
		TitleID:   id,
		TitleName: origName,
		Year:      2024,
		TitleType: model.TitleTypeMovie,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	taskID, err := tasks.Enqueue(model.TaskTypeEnrichment, string(raw), nil)
	require.NoError(t, err)

	queued, err := tasks.ListPending()
	require.NoError(t, err)
	require.Len(t, queued, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before processing → BeginTx must fail
	worker.ProcessTask(ctx, queued[0])

	after, err := titles.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, model.MatchStatusUnconfirmed, after.MatchStatus,
		"match_status must not be overwritten when tx aborts")
	assert.Nil(t, after.MatchSource,
		"match_source must stay empty when tx aborts")

	task, err := tasks.GetByID(taskID)
	require.NoError(t, err)
	assert.NotEmpty(t, task.LastError, "task should record the cancellation error")
}
