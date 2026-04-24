package cmd

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/router"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

func Serve(distFS embed.FS) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := cfg.DataDir + "/plextracker.db"
	writeDB, readDB, err := database.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer writeDB.Close()
	defer readDB.Close()

	if err := database.Migrate(writeDB); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Background refresh job (all writes — use writeDB)
	titleRepo := repository.NewTitleRepository(writeDB)
	settingRepo := repository.NewSettingRepository(writeDB)
	taskRepo := repository.NewTaskRepository(writeDB)

	var tmdbClient *matching.TMDBClient
	var pipeline *matching.Pipeline
	anilistClient := matching.NewAniListClient()

	if cfg.TMDBAPIKey != "" {
		tmdbClient = matching.NewTMDBClient(cfg.TMDBAPIKey)

		var geminiClient *matching.GeminiClient
		if len(cfg.GeminiAPIKeys) > 0 {
			geminiClient = matching.NewGeminiClient(cfg.GeminiAPIKeys)
		}

		var crossDB *matching.CrossRefDB
		crossrefPath := filepath.Join(cfg.DataDir, "anime-offline-database.json")
		if cdb, err := matching.LoadCrossRefDB(crossrefPath); err == nil {
			crossDB = cdb
		} else {
			log.Printf("matching: crossref DB not loaded (optional): %v", err)
		}

		pipeline = matching.NewPipeline(tmdbClient, anilistClient, geminiClient, crossDB, cfg.DataDir)
	}

	var tvdbClient *matching.TVDBClient
	if cfg.TVDBAPIKey != "" {
		tvdbClient = matching.NewTVDBClient(cfg.TVDBAPIKey)
		if err := tvdbClient.Login(context.Background()); err != nil {
			log.Printf("warning: TVDB login failed, TVDB enrichment disabled: %v", err)
			tvdbClient = nil
		} else {
			log.Printf("TVDB client ready")
			if pipeline != nil {
				pipeline.SetTVDB(tvdbClient)
			}
		}
	} else {
		log.Printf("warning: TVDB_API_KEY not set, TVDB enrichment disabled")
	}

	var pushSvc service.PushNotifier = service.NewNoopNotifier()
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc = service.NewPushService(writeDB, settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	}

	titleSvc := service.NewTitleService(writeDB, titleRepo, taskRepo, pipeline)

	// shutdownWG tracks background goroutines (ticker, task queue worker, async
	// Plex enrichment, RefreshOne) so Serve() can wait for them to exit before
	// closing the database. Without this, Shutdown returns while in-flight
	// transactions are still running → "database is closed" errors and tasks
	// left in status=running.
	var shutdownWG sync.WaitGroup

	coverSvc := service.NewCoverService(writeDB, titleRepo, tmdbClient, anilistClient, cfg.DataDir)
	bgSvc := service.NewBackgroundService(writeDB, titleRepo, settingRepo, tmdbClient, coverSvc, pushSvc)
	bgSvc.SetShutdownWG(&shutdownWG)
	if tvdbClient != nil {
		bgSvc.SetTVDB(tvdbClient)
	}
	if !cfg.DisableBackgroundTasks {
		bgSvc.StartTicker(ctx, 24*time.Hour)
	}

	r := router.New(ctx, cfg, writeDB, readDB, distFS, bgSvc, pipeline, &shutdownWG)

	// Task queue worker
	worker := service.NewTaskQueueWorker(taskRepo, titleRepo, pipeline, tmdbClient, anilistClient, pushSvc, settingRepo, cfg.DataDir, titleSvc, writeDB)
	worker.SetShutdownWG(&shutdownWG)
	if !cfg.DisableBackgroundTasks {
		worker.Start(ctx)
	}

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: r,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("PlexTracker listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Printf("shutdown signal received, stopping server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
		// Wait for background goroutines (ticker, task queue, async enrichment,
		// RefreshOne) to finish before returning — otherwise the deferred
		// writeDB/readDB.Close() races their in-flight transactions.
		done := make(chan struct{})
		go func() {
			shutdownWG.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			log.Printf("shutdown: background goroutines did not finish within 10s, closing DB anyway")
		}
		return nil
	}
}
