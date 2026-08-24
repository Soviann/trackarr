package testutil

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/stretchr/testify/require"
)

// EnqueueTask inserts a task in a fresh transaction and returns its ID.
func EnqueueTask(t *testing.T, db *sql.DB, taskType model.TaskType, payload string, dedupKey *string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		id, e = repository.NewTaskWriter(tx).Enqueue(context.Background(), taskType, payload, dedupKey)
		return e
	}))
	return id
}

// FetchDueTasks grabs up to limit due tasks and marks them as running.
func FetchDueTasks(t *testing.T, db *sql.DB, limit int) []model.Task {
	t.Helper()
	var tasks []model.Task
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		var e error
		tasks, e = repository.NewTaskWriter(tx).FetchDue(context.Background(), limit)
		return e
	}))
	return tasks
}

// ResetRunningTasks moves every 'running' task back to 'pending'.
func ResetRunningTasks(t *testing.T, db *sql.DB) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTaskWriter(tx).ResetRunning(context.Background())
	}))
}

// FailTask records a task failure with the given error message and next retry time.
func FailTask(t *testing.T, db *sql.DB, id int64, errMsg string, nextRunAt time.Time) {
	t.Helper()
	require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
		return repository.NewTaskWriter(tx).Fail(context.Background(), id, errMsg, nextRunAt)
	}))
}
