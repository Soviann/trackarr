package repository_test

import (
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRepository_ResetRunning(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	repo := repository.NewTaskRepository(db)

	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, `{"title_id": 1}`, nil)
	testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, `{"title_id": 2}`, nil)

	tasks := testutil.FetchDueTasks(t, db, 10)
	assert.Len(t, tasks, 2)
	for _, task := range tasks {
		assert.Equal(t, model.TaskStatusRunning, task.Status)
	}

	pending, err := repo.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 2)
	for _, task := range pending {
		assert.Equal(t, model.TaskStatusRunning, task.Status)
	}

	testutil.ResetRunningTasks(t, db)

	pending, err = repo.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 2)
	for _, task := range pending {
		assert.Equal(t, model.TaskStatusPending, task.Status)
	}
}

func TestTaskRepository_FetchDue_WakeSleeping(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	repo := repository.NewTaskRepository(db)

	id := testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, `{"title_id": 1}`, nil)

	// Force sleep on first fail by capping max_attempts at 1.
	_, err = db.Exec(`UPDATE task_queue SET max_attempts = 1 WHERE id = ?`, id)
	require.NoError(t, err)

	testutil.FailTask(t, db, id, "some error", time.Now().Add(time.Hour))

	task, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSleeping, task.Status)
	assert.Equal(t, 2, task.Day)

	past := time.Now().Add(-time.Hour)
	_, err = db.Exec(`UPDATE task_queue SET run_at = ? WHERE id = ?`, past, id)
	require.NoError(t, err)

	tasks := testutil.FetchDueTasks(t, db, 10)
	assert.Len(t, tasks, 1)
	assert.Equal(t, model.TaskStatusRunning, tasks[0].Status)
}

func TestTaskWriter_Enqueue_WakeSleeping(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	repo := repository.NewTaskRepository(db)

	dedupKey := "arr_push_123"
	id := testutil.EnqueueTask(t, db, model.TaskTypeRadarrPush, `{"title_id": 123}`, &dedupKey)

	// Force sleep on fail
	_, err = db.Exec(`UPDATE task_queue SET max_attempts = 1 WHERE id = ?`, id)
	require.NoError(t, err)
	testutil.FailTask(t, db, id, "API error", time.Now().Add(24*time.Hour))

	task, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSleeping, task.Status)
	assert.Equal(t, "API error", *task.LastError)

	// Re-enqueue with the same dedup_key
	testutil.EnqueueTask(t, db, model.TaskTypeRadarrPush, `{"title_id": 123, "monitored": true}`, &dedupKey)

	task, err = repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusPending, task.Status)
	assert.Equal(t, `{"title_id": 123, "monitored": true}`, task.Payload)
	assert.Nil(t, task.LastError)
	assert.Equal(t, 0, task.Attempts)
}

func TestTaskWriter_FetchDue_Priority(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))

	// Enqueue tasks with varying timestamps
	past10 := time.Now().Add(-10 * time.Minute)
	past5 := time.Now().Add(-5 * time.Minute)
	past2 := time.Now().Add(-2 * time.Minute)
	past1 := time.Now().Add(-1 * time.Minute)

	// Enrichment & refresh (priority 1)
	idEnrich := testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, `{"title_id": 1}`, nil)
	_, err = db.Exec(`UPDATE task_queue SET run_at = ? WHERE id = ?`, past10, idEnrich)
	require.NoError(t, err)

	idRefresh := testutil.EnqueueTask(t, db, model.TaskTypeRefresh, `{"title_id": 2}`, nil)
	_, err = db.Exec(`UPDATE task_queue SET run_at = ? WHERE id = ?`, past5, idRefresh)
	require.NoError(t, err)

	// Interactive tasks (priority 0)
	idAniList := testutil.EnqueueTask(t, db, model.TaskTypeAniListPushSeason, `{"season_id": 10}`, nil)
	_, err = db.Exec(`UPDATE task_queue SET run_at = ? WHERE id = ?`, past2, idAniList)
	require.NoError(t, err)

	idRadarr := testutil.EnqueueTask(t, db, model.TaskTypeRadarrPush, `{"title_id": 3}`, nil)
	_, err = db.Exec(`UPDATE task_queue SET run_at = ? WHERE id = ?`, past1, idRadarr)
	require.NoError(t, err)

	// Fetch all 4 tasks
	tasks := testutil.FetchDueTasks(t, db, 4)
	require.Len(t, tasks, 4)

	// Priority 0 must come first, ordered by run_at ASC
	assert.Equal(t, model.TaskTypeAniListPushSeason, tasks[0].TaskType)
	assert.Equal(t, idAniList, tasks[0].ID)

	assert.Equal(t, model.TaskTypeRadarrPush, tasks[1].TaskType)
	assert.Equal(t, idRadarr, tasks[1].ID)

	// Priority 1 must come after, ordered by run_at ASC
	assert.Equal(t, model.TaskTypeEnrichment, tasks[2].TaskType)
	assert.Equal(t, idEnrich, tasks[2].ID)

	assert.Equal(t, model.TaskTypeRefresh, tasks[3].TaskType)
	assert.Equal(t, idRefresh, tasks[3].ID)
}

