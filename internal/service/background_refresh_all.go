package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/repository"
)

type RefreshJobStatus string

const (
	RefreshStatusIdle      RefreshJobStatus = "idle"
	RefreshStatusRunning   RefreshJobStatus = "running"
	RefreshStatusPaused    RefreshJobStatus = "paused"
	RefreshStatusCompleted RefreshJobStatus = "completed"
	RefreshStatusFailed    RefreshJobStatus = "failed"
)

// RefreshJobProgress holds the state and progress of the library-wide metadata refresh.
type RefreshJobProgress struct {
	Status          RefreshJobStatus `json:"status"`
	TotalTitles     int              `json:"total_titles"`
	ProcessedTitles int              `json:"processed_titles"`
	CurrentTitle    string           `json:"current_title,omitempty"`
	CursorID        int64            `json:"cursor_id,omitempty"`
	StartedAt       *time.Time       `json:"started_at,omitempty"`
	UpdatedAt       *time.Time       `json:"updated_at,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
	LastError       string           `json:"last_error,omitempty"`
}

// StartRefreshAllJob initiates or resumes the library-wide metadata refresh job.
// If forceRestart is true or previous job is completed, it starts fresh from 0.
func (s *BackgroundService) StartRefreshAllJob(serverCtx context.Context, forceRestart bool) (RefreshJobProgress, error) {
	if s == nil {
		return RefreshJobProgress{}, errors.New("background service not available")
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if s.refreshProgress.Status == RefreshStatusRunning {
		return s.refreshProgress, errors.New("refresh already in progress")
	}

	// Load persisted state if empty or idle
	if s.refreshProgress.Status == "" || s.refreshProgress.Status == RefreshStatusIdle {
		s.refreshProgress = s.loadRefreshJobProgress()
	}

	now := time.Now().UTC()
	if forceRestart || s.refreshProgress.Status == RefreshStatusCompleted {
		s.refreshProgress = RefreshJobProgress{
			Status:    RefreshStatusRunning,
			StartedAt: &now,
			UpdatedAt: &now,
		}
	} else {
		// Resuming from paused or failed state
		s.refreshProgress.Status = RefreshStatusRunning
		if s.refreshProgress.StartedAt == nil {
			s.refreshProgress.StartedAt = &now
		}
		s.refreshProgress.UpdatedAt = &now
		s.refreshProgress.LastError = ""
	}

	jobCtx, cancel := context.WithCancel(serverCtx)
	s.refreshCancel = cancel

	if s.shutdownWG != nil {
		s.shutdownWG.Add(1)
	}

	go func() {
		if s.shutdownWG != nil {
			defer s.shutdownWG.Done()
		}
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("background: refresh all job panicked: %v", rec)
			}
		}()
		s.runRefreshAllLoop(jobCtx)
	}()

	s.saveRefreshJobProgress(serverCtx, s.refreshProgress)
	return s.refreshProgress, nil
}

// CancelRefreshAllJob interrupts the currently running metadata refresh job.
func (s *BackgroundService) CancelRefreshAllJob() (RefreshJobProgress, error) {
	if s == nil {
		return RefreshJobProgress{}, errors.New("background service not available")
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if s.refreshProgress.Status == "" {
		s.refreshProgress = s.loadRefreshJobProgress()
	}

	if s.refreshProgress.Status != RefreshStatusRunning {
		if s.refreshProgress.Status == "" {
			s.refreshProgress.Status = RefreshStatusIdle
		}
		return s.refreshProgress, nil
	}

	if s.refreshCancel != nil {
		s.refreshCancel()
	}

	s.refreshProgress.Status = RefreshStatusPaused
	now := time.Now().UTC()
	s.refreshProgress.UpdatedAt = &now
	s.saveRefreshJobProgress(context.Background(), s.refreshProgress)
	return s.refreshProgress, nil
}

// GetRefreshAllProgress returns the current progress of the metadata refresh job.
func (s *BackgroundService) GetRefreshAllProgress() RefreshJobProgress {
	if s == nil {
		return RefreshJobProgress{Status: RefreshStatusIdle}
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if s.refreshProgress.Status == "" {
		s.refreshProgress = s.loadRefreshJobProgress()
		if s.refreshProgress.Status == "" {
			s.refreshProgress.Status = RefreshStatusIdle
		}
	}
	return s.refreshProgress
}

func (s *BackgroundService) loadRefreshJobProgress() RefreshJobProgress {
	if s.settings == nil {
		return RefreshJobProgress{Status: RefreshStatusIdle}
	}
	val, err := s.settings.Get(repository.SettingKeyRefreshJobProgress)
	if err != nil || val == "" {
		return RefreshJobProgress{Status: RefreshStatusIdle}
	}
	var p RefreshJobProgress
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return RefreshJobProgress{Status: RefreshStatusIdle}
	}
	// If the server crashed or was restarted while running, mark as paused
	if p.Status == RefreshStatusRunning {
		p.Status = RefreshStatusPaused
	}
	return p
}

func (s *BackgroundService) saveRefreshJobProgress(ctx context.Context, p RefreshJobProgress) {
	if s.settings == nil || s.writeDB == nil {
		return
	}
	data, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		return repository.NewSettingWriter(tx).Set(ctx, repository.SettingKeyRefreshJobProgress, string(data))
	})
}

func (s *BackgroundService) runRefreshAllLoop(jobCtx context.Context) {
	titles, err := s.titles.ListAllForRefresh(jobCtx)
	if err != nil {
		log.Printf("background: list titles for refresh all: %v", err)
		s.refreshMu.Lock()
		s.refreshProgress.Status = RefreshStatusFailed
		s.refreshProgress.LastError = fmt.Sprintf("failed to list titles: %v", err)
		now := time.Now().UTC()
		s.refreshProgress.UpdatedAt = &now
		s.saveRefreshJobProgress(context.Background(), s.refreshProgress)
		s.refreshMu.Unlock()
		return
	}

	total := len(titles)
	s.refreshMu.Lock()
	s.refreshProgress.TotalTitles = total
	startIndex := 0
	if s.refreshProgress.CursorID > 0 {
		// Attempt to locate cursor
		for i, t := range titles {
			if t.ID == s.refreshProgress.CursorID {
				startIndex = i + 1
				break
			}
		}
		if startIndex == 0 && s.refreshProgress.ProcessedTitles > 0 && s.refreshProgress.ProcessedTitles < total {
			startIndex = s.refreshProgress.ProcessedTitles
		}
	}
	s.refreshProgress.ProcessedTitles = startIndex
	s.refreshMu.Unlock()

	for i := startIndex; i < total; i++ {
		if err := jobCtx.Err(); err != nil {
			s.refreshMu.Lock()
			s.refreshProgress.Status = RefreshStatusPaused
			now := time.Now().UTC()
			s.refreshProgress.UpdatedAt = &now
			s.saveRefreshJobProgress(context.Background(), s.refreshProgress)
			s.refreshMu.Unlock()
			return
		}

		title := &titles[i]

		s.refreshMu.Lock()
		s.refreshProgress.CurrentTitle = title.PrimaryName
		s.refreshProgress.CursorID = title.ID
		s.refreshProgress.ProcessedTitles = i
		s.refreshMu.Unlock()

		// Refresh title with bounded per-title timeout
		titleCtx, titleCancel := context.WithTimeout(jobCtx, 45*time.Second)
		res := s.refreshTitle(titleCtx, title)
		titleCancel()

		// Always stamp last_refreshed_at on processing, so title moves to the back of the queue
		if !res.Refreshed {
			_ = database.WithTxContext(context.Background(), s.writeDB, func(tx *sql.Tx) error {
				return repository.NewTitleWriter(tx).MarkRefreshed(context.Background(), title.ID, time.Now().UTC())
			})
		}

		_ = s.limiter.Wait(jobCtx)

		// Every 10 titles or last title, persist progress
		if (i+1)%10 == 0 || i == total-1 {
			s.refreshMu.Lock()
			s.refreshProgress.ProcessedTitles = i + 1
			now := time.Now().UTC()
			s.refreshProgress.UpdatedAt = &now
			s.saveRefreshJobProgress(context.Background(), s.refreshProgress)
			s.refreshMu.Unlock()
		}
	}

	// Completed
	s.refreshMu.Lock()
	s.refreshProgress.Status = RefreshStatusCompleted
	s.refreshProgress.ProcessedTitles = total
	s.refreshProgress.CurrentTitle = ""
	now := time.Now().UTC()
	s.refreshProgress.CompletedAt = &now
	s.refreshProgress.UpdatedAt = &now
	s.saveRefreshJobProgress(context.Background(), s.refreshProgress)
	s.refreshMu.Unlock()
}
