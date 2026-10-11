package service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMetadataSyncer struct {
	refreshCalls atomic.Int32
}

func (f *fakeMetadataSyncer) RefreshTitles(ctx context.Context) []RefreshResult {
	f.refreshCalls.Add(1)
	return nil
}

type fakeCoverSyncer struct {
	fetchCalls   atomic.Int32
	cleanupCalls atomic.Int32
}

func (f *fakeCoverSyncer) FetchMissingCovers(ctx context.Context) int {
	f.fetchCalls.Add(1)
	return 0
}

func (f *fakeCoverSyncer) CleanupUnusedCovers(ctx context.Context, day time.Weekday) {
	f.cleanupCalls.Add(1)
}

func setupSchedulerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestScheduler_NilSafe(t *testing.T) {
	var s *Scheduler
	s.SetShutdownWG(&sync.WaitGroup{})
	s.Start(context.Background(), time.Second)
	s.CheckAnnualWrapped(context.Background())
	s.RunMaintenance(context.Background())
}

func TestScheduler_StartContextCancellation(t *testing.T) {
	db := setupSchedulerTestDB(t)
	syncer := &fakeMetadataSyncer{}
	covers := &fakeCoverSyncer{}
	statsRepo := repository.NewStatsRepository(db)
	wrappedRepo := repository.NewWrappedRepository(db)

	sched := NewScheduler(db, syncer, covers, statsRepo, wrappedRepo)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	sched.SetShutdownWG(&wg)

	sched.Start(ctx, 10*time.Millisecond)

	// Cancel immediately while in startup delay
	cancel()
	wg.Wait()

	assert.Equal(t, int32(0), syncer.refreshCalls.Load())
	assert.Equal(t, int32(0), covers.fetchCalls.Load())
}

func TestScheduler_CheckAnnualWrapped(t *testing.T) {
	db := setupSchedulerTestDB(t)
	syncer := &fakeMetadataSyncer{}
	covers := &fakeCoverSyncer{}
	statsRepo := repository.NewStatsRepository(db)
	wrappedRepo := repository.NewWrappedRepository(db)

	sched := NewScheduler(db, syncer, covers, statsRepo, wrappedRepo)

	previousYear := time.Now().Year() - 1

	// When previous year has no watch activity, no tasks are enqueued
	sched.CheckAnnualWrapped(context.Background())
	taskRepo := repository.NewTaskRepository(db)
	tasks, err := taskRepo.ListPending()
	require.NoError(t, err)
	assert.Empty(t, tasks)

	// Insert watch events for previousYear to give it watch activity
	relDate := fmt.Sprintf("%d-05-01", previousYear)
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		Year:        previousYear,
		ReleaseDate: &relDate,
	}, nil)

	watchedAt := time.Date(previousYear, 6, 15, 20, 0, 0, 0, time.UTC)
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{
		TitleID:   titleID,
		Source:    model.WatchEventSourceManual,
		CreatedAt: watchedAt,
	})

	// Run CheckAnnualWrapped again
	sched.CheckAnnualWrapped(context.Background())

	// Verify task was enqueued for previousYear
	tasks, err = taskRepo.ListPending()
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, model.TaskTypeGenerateWrapped, tasks[0].TaskType)

	// Running again should be a no-op due to deduplication or existing task
	sched.CheckAnnualWrapped(context.Background())
	tasks2, err := taskRepo.ListPending()
	require.NoError(t, err)
	assert.Len(t, tasks2, 1)
}

func TestScheduler_RunMaintenance(t *testing.T) {
	db := setupSchedulerTestDB(t)
	sched := NewScheduler(db, nil, nil, nil, nil)

	// Insert an old dead task (> 14 days) and a recent dead task (< 14 days)
	idOld := testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, `{"title_id": 1}`, nil)
	idRecent := testutil.EnqueueTask(t, db, model.TaskTypeEnrichment, `{"title_id": 2}`, nil)

	twentyDaysAgo := time.Now().Add(-20 * 24 * time.Hour)
	fiveDaysAgo := time.Now().Add(-5 * 24 * time.Hour)

	_, err := db.Exec(`UPDATE task_queue SET status = 'dead', updated_at = ? WHERE id = ?`, twentyDaysAgo, idOld)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE task_queue SET status = 'dead', updated_at = ? WHERE id = ?`, fiveDaysAgo, idRecent)
	require.NoError(t, err)

	sched.RunMaintenance(context.Background())

	taskRepo := repository.NewTaskRepository(db)
	_, err = taskRepo.GetByID(idOld)
	assert.ErrorIs(t, err, sql.ErrNoRows, "old dead task must be purged by RunMaintenance")

	alive, err := taskRepo.GetByID(idRecent)
	require.NoError(t, err)
	assert.Equal(t, idRecent, alive.ID)
}

func TestScheduler_NextDailyMaintenance(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		expected time.Duration
	}{
		{
			name:     "before 04:30 same day",
			now:      time.Date(2026, 10, 11, 2, 30, 0, 0, time.UTC),
			expected: 2 * time.Hour,
		},
		{
			name:     "exactly 04:30 schedules tomorrow",
			now:      time.Date(2026, 10, 11, 4, 30, 0, 0, time.UTC),
			expected: 24 * time.Hour,
		},
		{
			name:     "after 04:30 schedules tomorrow",
			now:      time.Date(2026, 10, 11, 6, 30, 0, 0, time.UTC),
			expected: 22 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextDailyMaintenance(tt.now)
			assert.Equal(t, tt.expected, got)
		})
	}
}
