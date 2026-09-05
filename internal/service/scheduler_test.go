package service

import (
	"context"
	"database/sql"
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

	// Insert watch events for 2024 to make it an available year
	relDate := "2024-05-01"
	titleID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		Year:        2024,
		ReleaseDate: &relDate,
	}, nil)

	watchedAt := time.Date(2024, 6, 15, 20, 0, 0, 0, time.UTC)
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{
		TitleID:   titleID,
		Source:    model.WatchEventSourceManual,
		CreatedAt: watchedAt,
	})

	// Run CheckAnnualWrapped
	sched.CheckAnnualWrapped(context.Background())

	// Verify tasks were enqueued for available years
	taskRepo := repository.NewTaskRepository(db)
	tasks, err := taskRepo.ListPending()
	require.NoError(t, err)
	assert.NotEmpty(t, tasks)
	for _, task := range tasks {
		assert.Equal(t, model.TaskTypeGenerateWrapped, task.TaskType)
	}
	initialCount := len(tasks)

	// Running again should be a no-op due to deduplication or existing task
	sched.CheckAnnualWrapped(context.Background())
	tasks2, err := taskRepo.ListPending()
	require.NoError(t, err)
	assert.Len(t, tasks2, initialCount)
}
