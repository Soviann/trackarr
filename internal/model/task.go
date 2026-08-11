package model

import "time"

type TaskType string

const (
	TaskTypeEnrichment        TaskType = "enrichment"
	TaskTypeRefresh           TaskType = "refresh"
	TaskTypeCoverFetch        TaskType = "cover_fetch"
	TaskTypeAniListPushSeason TaskType = "anilist_push_season"
	TaskTypeAniListPushMovie  TaskType = "anilist_push_movie"
	TaskTypeRadarrPush        TaskType = "radarr_push"
	TaskTypeSonarrPush        TaskType = "sonarr_push"
)

type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusSleeping TaskStatus = "sleeping"
	TaskStatusDead     TaskStatus = "dead"
)

type Task struct {
	ID          int64      `json:"id"`
	TaskType    TaskType   `json:"task_type"`
	Payload     string     `json:"payload"`
	Status      TaskStatus `json:"status"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	Day         int        `json:"day"`
	LastError   *string    `json:"last_error"`
	RunAt       time.Time  `json:"run_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DedupKey    *string    `json:"dedup_key,omitempty"`
}
