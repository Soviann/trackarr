package repository_test

import (
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
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
