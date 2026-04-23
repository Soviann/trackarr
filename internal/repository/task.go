package repository

import (
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

// TaskRepository reads the task queue. Writes live on TaskWriter, which
// requires a *sql.Tx.
type TaskRepository struct {
	db database.DBTX
}

func NewTaskRepository(db database.DBTX) *TaskRepository {
	return &TaskRepository{db: db}
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

// ListPaginated returns paginated tasks based on filter.
func (r *TaskRepository) ListPaginated(filter string, limit, offset int) ([]model.Task, int, error) {
	var whereClause string
	switch filter {
	case "pending":
		whereClause = "WHERE status != 'dead' AND last_error IS NULL"
	case "errored":
		whereClause = "WHERE status = 'dead' OR last_error IS NOT NULL"
	default:
		whereClause = "WHERE status != 'completed'"
	}

	countQuery := "SELECT COUNT(*) FROM task_queue " + whereClause
	var total int
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	query := "SELECT id, task_type, payload, status, attempts, max_attempts, day, last_error, run_at, created_at, updated_at, dedup_key FROM task_queue " + whereClause + " ORDER BY run_at ASC LIMIT ? OFFSET ?"

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list paginated tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.TaskType, &t.Payload, &t.Status, &t.Attempts, &t.MaxAttempts, &t.Day, &t.LastError, &t.RunAt, &t.CreatedAt, &t.UpdatedAt, &t.DedupKey); err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	return tasks, total, nil
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
