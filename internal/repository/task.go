package repository

import (
	"fmt"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

type TaskRepository struct {
	db database.DBTX
}

func NewTaskRepository(db database.DBTX) *TaskRepository {
	return &TaskRepository{db: db}
}

// Enqueue adds a new task to the queue. If a dedup_key is provided and a non-dead task
// with the same key already exists, the insert is silently skipped.
func (r *TaskRepository) Enqueue(taskType model.TaskType, payload string, dedupKey *string) (int64, error) {
	res, err := r.db.Exec(
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
func (r *TaskRepository) EnqueueWithDelay(taskType model.TaskType, payload string, dedupKey *string, runAt time.Time) (int64, error) {
	res, err := r.db.Exec(
		`INSERT OR IGNORE INTO task_queue (task_type, payload, dedup_key, run_at) VALUES (?, ?, ?, ?)`,
		taskType, payload, dedupKey, runAt,
	)
	if err != nil {
		return 0, fmt.Errorf("enqueue task with delay: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// FetchDue returns up to `limit` tasks that are due for execution and marks them as running.
func (r *TaskRepository) FetchDue(limit int) ([]model.Task, error) {
	now := time.Now()

	// First wake up sleeping tasks whose run_at has passed
	_, err := r.db.Exec(
		`UPDATE task_queue SET status = 'pending', updated_at = ? WHERE status = 'sleeping' AND run_at <= ?`,
		now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("wake sleeping tasks: %w", err)
	}

	rows, err := r.db.Query(
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

	// Mark fetched tasks as running
	for i := range tasks {
		_, err := r.db.Exec(
			`UPDATE task_queue SET status = 'running', updated_at = ? WHERE id = ?`,
			now, tasks[i].ID,
		)
		if err != nil {
			return nil, fmt.Errorf("mark task %d running: %w", tasks[i].ID, err)
		}
		tasks[i].Status = model.TaskStatusRunning
	}

	return tasks, nil
}

func (r *TaskRepository) GetByID(id int64) (*model.Task, error) {
	var t model.Task
	err := r.db.QueryRow(`SELECT id, task_type, payload, status, attempts, max_attempts, day, last_error, run_at, created_at, updated_at, dedup_key
		FROM task_queue WHERE id = ?`, id).Scan(&t.ID, &t.TaskType, &t.Payload, &t.Status, &t.Attempts, &t.MaxAttempts, &t.Day, &t.LastError, &t.RunAt, &t.CreatedAt, &t.UpdatedAt, &t.DedupKey)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Complete removes a successfully processed task.
func (r *TaskRepository) Complete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM task_queue WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("complete task %d: %w", id, err)
	}
	return nil
}

// Fail records a task failure and schedules the next retry or marks it dead/sleeping.
func (r *TaskRepository) Fail(id int64, errMsg string, nextRunAt time.Time) error {
	now := time.Now()

	// Increment attempts, then check if we need to advance day or mark dead
	_, err := r.db.Exec(
		`UPDATE task_queue SET
			attempts = attempts + 1,
			last_error = ?,
			updated_at = ?
		 WHERE id = ?`,
		errMsg, now, id,
	)
	if err != nil {
		return fmt.Errorf("fail task %d: %w", id, err)
	}

	// Re-read the task to check state
	var attempts, maxAttempts, day int
	err = r.db.QueryRow(`SELECT attempts, max_attempts, day FROM task_queue WHERE id = ?`, id).Scan(&attempts, &maxAttempts, &day)
	if err != nil {
		return fmt.Errorf("read task %d after fail: %w", id, err)
	}

	if attempts >= maxAttempts {
		if day >= 2 {
			// Day 2 exhausted → dead
			_, err = r.db.Exec(
				`UPDATE task_queue SET status = 'dead', updated_at = ? WHERE id = ?`,
				now, id,
			)
		} else {
			// Day 1 exhausted → sleep until tomorrow, reset attempts
			_, err = r.db.Exec(
				`UPDATE task_queue SET status = 'sleeping', day = day + 1, attempts = 0, run_at = ?, updated_at = ? WHERE id = ?`,
				nextDayMorning(), now, id,
			)
		}
	} else {
		// More attempts available → reschedule
		_, err = r.db.Exec(
			`UPDATE task_queue SET status = 'pending', run_at = ?, updated_at = ? WHERE id = ?`,
			nextRunAt, now, id,
		)
	}

	if err != nil {
		return fmt.Errorf("update task %d status after fail: %w", id, err)
	}
	return nil
}

// nextDayMorning returns tomorrow at 06:00 UTC.
func nextDayMorning() time.Time {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 6, 0, 0, 0, time.UTC)
}

// ListPending returns all non-dead, non-completed tasks.
func (r *TaskRepository) ListPending() ([]model.Task, error) {
	return r.listByStatuses("pending", "running", "sleeping")
}

// ListDead returns all dead tasks.
func (r *TaskRepository) ListDead() ([]model.Task, error) {
	return r.listByStatuses("dead")
}

func (r *TaskRepository) listByStatuses(statuses ...string) ([]model.Task, error) {
	query := `SELECT id, task_type, payload, status, attempts, max_attempts, day, last_error, run_at, created_at, updated_at, dedup_key
		FROM task_queue WHERE status IN (`
	args := make([]any, len(statuses))
	for i, s := range statuses {
		if i > 0 {
			query += ", "
		}
		query += "?"
		args[i] = s
	}
	query += `) ORDER BY run_at ASC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.TaskType, &t.Payload, &t.Status, &t.Attempts, &t.MaxAttempts, &t.Day, &t.LastError, &t.RunAt, &t.CreatedAt, &t.UpdatedAt, &t.DedupKey); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// RetryDead resets a dead task to pending for a fresh retry cycle.
func (r *TaskRepository) RetryDead(id int64) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE task_queue SET status = 'pending', attempts = 0, day = 1, run_at = ?, updated_at = ? WHERE id = ? AND status = 'dead'`,
		now, now, id,
	)
	if err != nil {
		return fmt.Errorf("retry dead task %d: %w", id, err)
	}
	return nil
}

// Delete removes a task from the queue.
func (r *TaskRepository) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM task_queue WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	return nil
}

// DeleteBatch removes multiple tasks from the queue.
func (r *TaskRepository) DeleteBatch(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	query := "DELETE FROM task_queue WHERE id IN ("
	args := make([]any, len(ids))
	for i, id := range ids {
		if i > 0 {
			query += ", "
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("delete batch tasks: %w", err)
	}
	return nil
}

// CountByStatus returns the count of tasks grouped by status.
func (r *TaskRepository) CountByStatus() (map[model.TaskStatus]int, error) {
	rows, err := r.db.Query(`SELECT status, COUNT(*) FROM task_queue GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("count tasks by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[model.TaskStatus]int)
	for rows.Next() {
		var status model.TaskStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan task count: %w", err)
		}
		counts[status] = count
	}
	return counts, nil
}

// CountDead returns the number of dead tasks.
func (r *TaskRepository) CountDead() (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM task_queue WHERE status = 'dead'`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count dead tasks: %w", err)
	}
	return count, nil
}
