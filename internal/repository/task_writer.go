package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// TaskWriter performs write operations on the task queue within a caller-owned
// transaction. Accepting only *sql.Tx makes "write to the pool without a
// transaction" a compile-time error — the same class of bug that used to
// surface as SQLite BUSY deadlocks when two tasks raced the write connection.
type TaskWriter struct {
	tx *sql.Tx
}

func NewTaskWriter(tx *sql.Tx) *TaskWriter {
	return &TaskWriter{tx: tx}
}

// Enqueue adds a new task to the queue. If a dedup_key is provided and a non-dead
// task with the same key already exists, the insert is silently skipped.
func (w *TaskWriter) Enqueue(ctx context.Context, taskType model.TaskType, payload string, dedupKey *string) (int64, error) {
	res, err := w.tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO task_queue (task_type, payload, dedup_key) VALUES (?, ?, ?)`,
		taskType, payload, dedupKey,
	)
	if err != nil {
		return 0, fmt.Errorf("enqueue task: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// EnqueueWithDelay adds a task scheduled for a future time.
func (w *TaskWriter) EnqueueWithDelay(ctx context.Context, taskType model.TaskType, payload string, dedupKey *string, runAt time.Time) (int64, error) {
	res, err := w.tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO task_queue (task_type, payload, dedup_key, run_at) VALUES (?, ?, ?, ?)`,
		taskType, payload, dedupKey, runAt,
	)
	if err != nil {
		return 0, fmt.Errorf("enqueue task with delay: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// FetchDue returns up to `limit` tasks that are due for execution and marks
// them as running. Wake-up, select and the running-stamp share one tx so two
// workers cannot grab the same task.
func (w *TaskWriter) FetchDue(ctx context.Context, limit int) ([]model.Task, error) {
	now := time.Now()

	if _, err := w.tx.ExecContext(ctx,
		`UPDATE task_queue SET status = 'pending', updated_at = ? WHERE status = 'sleeping' AND run_at <= ?`,
		now, now,
	); err != nil {
		return nil, fmt.Errorf("wake sleeping tasks: %w", err)
	}

	rows, err := w.tx.QueryContext(ctx,
		`SELECT id, task_type, payload, status, attempts, max_attempts, day, last_error, run_at, created_at, updated_at, dedup_key
		 FROM task_queue
		 WHERE status = 'pending' AND run_at <= ?
		 ORDER BY run_at ASC
		 LIMIT ?`,
		now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch due tasks: %w", err)
	}

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.TaskType, &t.Payload, &t.Status, &t.Attempts, &t.MaxAttempts, &t.Day, &t.LastError, &t.RunAt, &t.CreatedAt, &t.UpdatedAt, &t.DedupKey); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	rows.Close()

	if len(tasks) > 0 {
		placeholders := make([]string, len(tasks))
		args := make([]interface{}, 0, len(tasks)+1)
		args = append(args, now)
		for i, t := range tasks {
			placeholders[i] = "?"
			args = append(args, t.ID)
		}
		query := fmt.Sprintf(`UPDATE task_queue SET status = 'running', updated_at = ? WHERE id IN (%s)`, strings.Join(placeholders, ","))
		if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
			return nil, fmt.Errorf("mark tasks running: %w", err)
		}
		for i := range tasks {
			tasks[i].Status = model.TaskStatusRunning
		}
	}

	return tasks, nil
}

// Complete removes a successfully processed task.
func (w *TaskWriter) Complete(ctx context.Context, id int64) error {
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM task_queue WHERE id = ?`, id); err != nil {
		return fmt.Errorf("complete task %d: %w", id, err)
	}
	return nil
}

// Fail records a task failure and schedules the next retry, puts the task to
// sleep until the next day, or marks it dead — whichever the retry policy
// dictates given attempts/max_attempts/day.
func (w *TaskWriter) Fail(ctx context.Context, id int64, errMsg string, nextRunAt time.Time) error {
	now := time.Now()

	var attempts, maxAttempts, day int
	err := w.tx.QueryRowContext(ctx,
		`UPDATE task_queue SET
			attempts = attempts + 1,
			last_error = ?,
			updated_at = ?
		 WHERE id = ?
		 RETURNING attempts, max_attempts, day`,
		errMsg, now, id,
	).Scan(&attempts, &maxAttempts, &day)
	if err != nil {
		return fmt.Errorf("fail task %d: %w", id, err)
	}

	switch {
	case attempts >= maxAttempts && day >= 7:
		_, err = w.tx.ExecContext(ctx,
			`UPDATE task_queue SET status = 'dead', updated_at = ? WHERE id = ?`,
			now, id,
		)
	case attempts >= maxAttempts:
		_, err = w.tx.ExecContext(ctx,
			`UPDATE task_queue SET status = 'sleeping', day = day + 1, attempts = 0, run_at = ?, updated_at = ? WHERE id = ?`,
			nextDayMorning(), now, id,
		)
	default:
		_, err = w.tx.ExecContext(ctx,
			`UPDATE task_queue SET status = 'pending', run_at = ?, updated_at = ? WHERE id = ?`,
			nextRunAt, now, id,
		)
	}
	if err != nil {
		return fmt.Errorf("update task %d status after fail: %w", id, err)
	}
	return nil
}

// RetryDead resets a dead task to pending for a fresh retry cycle.
func (w *TaskWriter) RetryDead(ctx context.Context, id int64) error {
	now := time.Now()
	if _, err := w.tx.ExecContext(ctx,
		`UPDATE task_queue SET status = 'pending', attempts = 0, day = 1, run_at = ?, updated_at = ? WHERE id = ? AND status = 'dead'`,
		now, now, id,
	); err != nil {
		return fmt.Errorf("retry dead task %d: %w", id, err)
	}
	return nil
}

// ResetRunning resets all tasks in 'running' status back to 'pending'. Called
// at startup to rescue tasks interrupted by a crash.
func (w *TaskWriter) ResetRunning(ctx context.Context) error {
	now := time.Now()
	if _, err := w.tx.ExecContext(ctx,
		`UPDATE task_queue SET status = 'pending', updated_at = ? WHERE status = 'running'`,
		now,
	); err != nil {
		return fmt.Errorf("reset running tasks: %w", err)
	}
	return nil
}

// Delete removes a task from the queue.
func (w *TaskWriter) Delete(ctx context.Context, id int64) error {
	if _, err := w.tx.ExecContext(ctx, `DELETE FROM task_queue WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	return nil
}

// DeleteBatch removes multiple tasks from the queue.
func (w *TaskWriter) DeleteBatch(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `DELETE FROM task_queue WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := w.tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("delete batch tasks: %w", err)
	}
	return nil
}

// nextDayMorning returns tomorrow at 06:00 UTC.
func nextDayMorning() time.Time {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 6, 0, 0, 0, time.UTC)
}
