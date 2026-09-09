package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

// MetadataSyncer defines the metadata sync methods invoked by the Scheduler.
type MetadataSyncer interface {
	RefreshTitles(ctx context.Context) []RefreshResult
}

// CoverSyncer defines the cover maintenance methods invoked by the Scheduler.
type CoverSyncer interface {
	FetchMissingCovers(ctx context.Context) int
	CleanupUnusedCovers(ctx context.Context, day time.Weekday)
}

// Scheduler orchestrates periodic background jobs (daily metadata refresh,
// periodic missing cover fetch, weekly unused cover cleanup, and annual wrapped checks).
type Scheduler struct {
	writeDB     *sql.DB
	syncSvc     MetadataSyncer
	covers      CoverSyncer
	statsRepo   *repository.StatsRepository
	wrappedRepo *repository.WrappedRepository
	shutdownWG  *sync.WaitGroup
}

// NewScheduler creates a new background job scheduler.
func NewScheduler(
	writeDB *sql.DB,
	syncSvc MetadataSyncer,
	covers CoverSyncer,
	statsRepo *repository.StatsRepository,
	wrappedRepo *repository.WrappedRepository,
) *Scheduler {
	if statsRepo == nil && writeDB != nil {
		statsRepo = repository.NewStatsRepository(writeDB)
	}
	if wrappedRepo == nil && writeDB != nil {
		wrappedRepo = repository.NewWrappedRepository(writeDB)
	}
	return &Scheduler{
		writeDB:     writeDB,
		syncSvc:     syncSvc,
		covers:      covers,
		statsRepo:   statsRepo,
		wrappedRepo: wrappedRepo,
	}
}

// SetShutdownWG registers a WaitGroup the scheduler goroutine increments on start
// and decrements on exit.
func (s *Scheduler) SetShutdownWG(wg *sync.WaitGroup) {
	if s == nil {
		return
	}
	s.shutdownWG = wg
}

// Start launches the background recurring jobs on the given interval.
func (s *Scheduler) Start(ctx context.Context, interval time.Duration) {
	if s == nil {
		return
	}

	if s.shutdownWG != nil {
		s.shutdownWG.Add(1)
	}
	go func() {
		if s.shutdownWG != nil {
			defer s.shutdownWG.Done()
		}
		// Outer loop restarts the ticker after a panic so a single bad iteration
		// cannot silently kill the schedule.
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("scheduler: panic in ticker loop: %v", r)
						time.Sleep(30 * time.Second)
					}
				}()

				select {
				case <-ctx.Done():
					return
				case <-time.After(30 * time.Second):
				}

				s.runInitialPass(ctx)

				ticker := time.NewTicker(interval)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						s.runScheduledPass(ctx)
					}
				}
			}()

			if ctx.Err() != nil {
				return
			}
		}
	}()
}

func (s *Scheduler) runInitialPass(ctx context.Context) {
	if s.covers != nil {
		log.Println("scheduler: fetching missing covers")
		if n := s.covers.FetchMissingCovers(ctx); n > 0 {
			log.Printf("scheduler: fetched %d missing covers", n)
		}
	}
	if s.syncSvc != nil {
		log.Println("scheduler: starting initial title refresh")
		s.syncSvc.RefreshTitles(ctx)
	}
	s.CheckAnnualWrapped(ctx)
}

func (s *Scheduler) runScheduledPass(ctx context.Context) {
	if s.syncSvc != nil {
		log.Println("scheduler: starting scheduled title refresh")
		s.syncSvc.RefreshTitles(ctx)
	}
	s.CheckAnnualWrapped(ctx)

	if s.covers != nil {
		day := time.Now().Weekday()
		log.Printf("scheduler: starting unused covers cleanup for %s", day.String())
		s.covers.CleanupUnusedCovers(ctx, day)
	}
}

// CheckAnnualWrapped checks if the previous calendar year has concluded and needs
// an annual Wrapped snapshot generated. Snapshots are only generated at date (for the
// immediately elapsed year), never retroactively backfilled for older historical years.
func (s *Scheduler) CheckAnnualWrapped(ctx context.Context) {
	if s == nil || s.wrappedRepo == nil || s.statsRepo == nil || s.writeDB == nil {
		return
	}

	previousYear := time.Now().Year() - 1
	if previousYear < 2025 {
		return
	}

	has, err := s.wrappedRepo.HasSnapshot(ctx, previousYear)
	if err != nil {
		log.Printf("scheduler: check wrapped snapshot for %d: %v", previousYear, err)
		return
	}
	if has {
		return
	}

	// Only enqueue if the elapsed year actually has watch activity
	rawStats, _, err := s.statsRepo.GetWrappedData(ctx, previousYear)
	if err != nil {
		log.Printf("scheduler: check wrapped data for %d: %v", previousYear, err)
		return
	}
	if rawStats.TotalTitles == 0 {
		return
	}

	payload, err := json.Marshal(GenerateWrappedPayload{Year: previousYear})
	if err != nil {
		log.Printf("scheduler: marshal generate_wrapped payload for %d: %v", previousYear, err)
		return
	}

	dedupKey := fmt.Sprintf("generate_wrapped:%d", previousYear)
	if enqErr := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
		_, e := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeGenerateWrapped, string(payload), &dedupKey)
		return e
	}); enqErr != nil {
		log.Printf("scheduler: enqueue generate_wrapped for %d: %v", previousYear, enqErr)
	} else {
		log.Printf("scheduler: enqueued generate_wrapped for year %d", previousYear)
	}
}
