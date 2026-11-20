package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	onRefresh    func()
}

func (f *fakeMetadataSyncer) RefreshTitles(ctx context.Context) []RefreshResult {
	f.refreshCalls.Add(1)
	if f.onRefresh != nil {
		f.onRefresh()
	}
	return nil
}

type fakeJellyfinMovieSyncer struct {
	calls   atomic.Int32
	summary JellyfinMovieSyncSummary
	err     error
	onSync  func()
}

func (f *fakeJellyfinMovieSyncer) SyncUnwatchedMovies(ctx context.Context) (JellyfinMovieSyncSummary, error) {
	f.calls.Add(1)
	if f.onSync != nil {
		f.onSync()
	}
	return f.summary, f.err
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
	s.SetJellyfinMovieSyncer(nil)
	s.Start(context.Background(), time.Second)
	s.syncJellyfinMovies(context.Background())
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

func TestScheduler_JellyfinMovieSync(t *testing.T) {
	t.Run("runInitialPass and runScheduledPass invoke jellyfinSyncer before RefreshTitles", func(t *testing.T) {
		db := setupSchedulerTestDB(t)
		var order []string
		syncer := &fakeMetadataSyncer{
			onRefresh: func() { order = append(order, "refresh") },
		}
		jfSyncer := &fakeJellyfinMovieSyncer{
			summary: JellyfinMovieSyncSummary{Fetched: 2, Unwatched: 1, Created: 1},
			onSync:  func() { order = append(order, "jellyfin") },
		}

		sched := NewScheduler(db, syncer, nil, nil, nil)
		sched.SetJellyfinMovieSyncer(jfSyncer)

		sched.runInitialPass(context.Background())
		assert.Equal(t, []string{"jellyfin", "refresh"}, order)
		assert.Equal(t, int32(1), jfSyncer.calls.Load())
		assert.Equal(t, int32(1), syncer.refreshCalls.Load())

		order = nil
		sched.runScheduledPass(context.Background())
		assert.Equal(t, []string{"jellyfin", "refresh"}, order)
		assert.Equal(t, int32(2), jfSyncer.calls.Load())
		assert.Equal(t, int32(2), syncer.refreshCalls.Load())
	})

	t.Run("tolerates nil syncer and sync errors without halting scheduler", func(t *testing.T) {
		db := setupSchedulerTestDB(t)
		syncer := &fakeMetadataSyncer{}
		sched := NewScheduler(db, syncer, nil, nil, nil)

		// Nil jellyfinSyncer
		assert.NotPanics(t, func() {
			sched.runInitialPass(context.Background())
			sched.runScheduledPass(context.Background())
		})
		assert.Equal(t, int32(2), syncer.refreshCalls.Load())

		// Error from jellyfinSyncer
		jfSyncer := &fakeJellyfinMovieSyncer{
			err: fmt.Errorf("jellyfin offline"),
		}
		sched.SetJellyfinMovieSyncer(jfSyncer)
		assert.NotPanics(t, func() {
			sched.runInitialPass(context.Background())
			sched.runScheduledPass(context.Background())
		})
		assert.Equal(t, int32(2), jfSyncer.calls.Load())
		assert.Equal(t, int32(4), syncer.refreshCalls.Load())
	})

	t.Run("runs real JellyfinSyncService during initial pass using SQLite settings", func(t *testing.T) {
		db := setupSchedulerTestDB(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Items": []map[string]any{
					{
						"Id":             "jf-sched-1",
						"Name":           "Interstellar",
						"Type":           "Movie",
						"ProductionYear": 2014,
						"ProviderIds":    map[string]string{"Tmdb": "157336"},
					},
				},
			})
		}))
		defer server.Close()

		tx, err := db.Begin()
		require.NoError(t, err)
		sw := repository.NewSettingWriter(tx)
		require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinURL, server.URL))
		require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinAPIKey, "sched-key"))
		require.NoError(t, sw.Set(context.Background(), repository.SettingKeyJellyfinUserID, "sched-user"))
		require.NoError(t, tx.Commit())

		jfSvc := NewJellyfinSyncService(db, JellyfinSyncConfig{HTTPClient: server.Client()}, testutil.NopLogger())
		jfSvc.SetAPILimiter(NewAPILimiter(10, 1))

		sched := NewScheduler(db, nil, nil, nil, nil)
		sched.SetJellyfinMovieSyncer(jfSvc)
		sched.runInitialPass(context.Background())

		tmdbID := int64(157336)
		movieType := model.TitleTypeMovie
		title, err := repository.NewTitleRepository(db).FindByExternalID(nil, &tmdbID, nil, nil, nil, &movieType)
		require.NoError(t, err)
		assert.Equal(t, model.TitleStatusPlanToWatch, title.Status)
		assert.NotNil(t, title.DownloadedAt)
	})
}
