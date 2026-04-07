package repository_test

import (
	"testing"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRepository_ResetRunning(t *testing.T) {
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	repo := repository.NewTaskRepository(db)

	// Enqueue tasks
	_, err = repo.Enqueue(model.TaskTypeEnrichment, `{"title_id": 1}`, nil)
	require.NoError(t, err)
	_, err = repo.Enqueue(model.TaskTypeEnrichment, `{"title_id": 2}`, nil)
	require.NoError(t, err)

	// Fetch tasks to mark them as running
	tasks, err := repo.FetchDue(10)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
	for _, task := range tasks {
		assert.Equal(t, model.TaskStatusRunning, task.Status)
	}

	// Verify they are running in DB
	pending, err := repo.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 2)
	for _, task := range pending {
		assert.Equal(t, model.TaskStatusRunning, task.Status)
	}

	// Reset running tasks
	err = repo.ResetRunning()
	require.NoError(t, err)

	// Verify they are back to pending
	pending, err = repo.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 2)
	for _, task := range pending {
		assert.Equal(t, model.TaskStatusPending, task.Status)
	}
}

func TestTaskRepository_FetchDue_WakeSleeping(t *testing.T) {
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	repo := repository.NewTaskRepository(db)

	// Enqueue task and fail it to make it sleep
	id, err := repo.Enqueue(model.TaskTypeEnrichment, `{"title_id": 1}`, nil)
	require.NoError(t, err)

	// Set attempts to max to force sleep on fail
	_, err = db.Exec(`UPDATE task_queue SET max_attempts = 1 WHERE id = ?`, id)
	require.NoError(t, err)

	err = repo.Fail(id, "some error", time.Now().Add(time.Hour))
	require.NoError(t, err)

	// Verify it's sleeping
	task, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSleeping, task.Status)
	assert.Equal(t, 2, task.Day)

	// Set run_at to past
	past := time.Now().Add(-time.Hour)
	_, err = db.Exec(`UPDATE task_queue SET run_at = ? WHERE id = ?`, past, id)
	require.NoError(t, err)

	// Fetch due should wake it up
	tasks, err := repo.FetchDue(10)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, model.TaskStatusRunning, tasks[0].Status)
}
