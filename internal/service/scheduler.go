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

// CheckAnnualWrapped scans all calendar years with watch activity and enqueues
// background generation tasks for any year lacking a stored snapshot.
func (s *Scheduler) CheckAnnualWrapped(ctx context.Context) {
	if s == nil || s.wrappedRepo == nil || s.statsRepo == nil || s.writeDB == nil {
		return
	}

	years, err := s.statsRepo.AvailableYears(ctx)
	if err != nil {
		log.Printf("scheduler: check available years: %v", err)
		return
	}

	for _, y := range years {
		if y < 2000 {
			continue
		}

		has, err := s.wrappedRepo.HasSnapshot(ctx, y)
		if err != nil {
			log.Printf("scheduler: check wrapped snapshot for %d: %v", y, err)
			continue
		}
		if has {
			continue
		}

		payload, err := json.Marshal(GenerateWrappedPayload{Year: y})
		if err != nil {
			log.Printf("scheduler: marshal generate_wrapped payload for %d: %v", y, err)
			continue
		}

		dedupKey := fmt.Sprintf("generate_wrapped:%d", y)
		if enqErr := database.WithTxContext(ctx, s.writeDB, func(tx *sql.Tx) error {
			_, e := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeGenerateWrapped, string(payload), &dedupKey)
			return e
		}); enqErr != nil {
			log.Printf("scheduler: enqueue generate_wrapped for %d: %v", y, enqErr)
		} else {
			log.Printf("scheduler: enqueued generate_wrapped for year %d", y)
		}
	}
}
