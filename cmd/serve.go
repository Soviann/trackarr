package cmd

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/router"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/Soviann/trackarr/internal/version"
)

func Serve(distFS embed.FS) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Structured logging defaults: TextHandler at Info, stdout. Single-user
	// project, console-readable output beats JSON. Set before any work so
	// every migrated caller (taskqueue, webhook…) shares the same sink.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := database.ResolveDBPath(cfg.DataDir)
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

	var pushSvc service.PushNotifier
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc = service.NewPushService(writeDB, settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	} else {
		pushSvc = service.NewNoopNotifier()
	}

	titleSvc := service.NewTitleService(writeDB, titleRepo, taskRepo, pipeline)

	// shutdownWG tracks background goroutines (ticker, task queue worker, async
	// media enrichment, RefreshOne) so Serve() can wait for them to exit before
	// closing the database. Without this, Shutdown returns while in-flight
	// transactions are still running → "database is closed" errors and tasks
	// left in status=running.
	var shutdownWG sync.WaitGroup

	// Shared external-API limiter: 2rps / burst 1 against TMDB+AniList, shared
	// across background refresh, cover fetch and task queue worker so three
	// independent loops don't burn 6rps in parallel.
	externalAPILimiter := service.NewAPILimiter(2, 1)

	coverSvc := service.NewCoverService(writeDB, titleRepo, tmdbClient, anilistClient, cfg.DataDir)
	coverSvc.SetAPILimiter(externalAPILimiter)
	bgSvc := service.NewMetadataSyncService(writeDB, titleRepo, settingRepo, tmdbClient, coverSvc, pushSvc)
	bgSvc.SetShutdownWG(&shutdownWG)
	bgSvc.SetAPILimiter(externalAPILimiter)
	if tvdbClient != nil {
		bgSvc.SetTVDB(tvdbClient)
	}
	if anilistClient != nil {
		bgSvc.SetAniList(anilistClient)
	}

	scheduler := service.NewScheduler(writeDB, bgSvc, coverSvc, nil, nil)
	scheduler.SetShutdownWG(&shutdownWG)
	if !cfg.DisableBackgroundTasks {
		scheduler.Start(ctx, 24*time.Hour)
	}

	r := router.New(ctx, cfg, writeDB, readDB, distFS, bgSvc, pipeline, &shutdownWG)

	// Task queue worker
	worker := service.NewTaskQueueWorker(taskRepo, titleRepo, pipeline, tmdbClient, anilistClient, pushSvc, settingRepo, cfg.DataDir, titleSvc, writeDB)
	worker.SetShutdownWG(&shutdownWG)
	worker.SetAPILimiter(externalAPILimiter)
	worker.SetCovers(coverSvc)
	// AniList push service: drives anilist_push_season / anilist_push_movie tasks.
	// The same matching.AniListClient used for enrichment satisfies the narrow
	// aniListPushClient interface — no adapter needed.
	anilistPushSvc := service.NewAniListPushService(writeDB, anilistClient, slog.Default())
	worker.SetAniListPush(anilistPushSvc)

	arrSvc := service.NewArrService(cfg, settingRepo, titleRepo, writeDB)
	worker.SetArrService(arrSvc)

	if !cfg.DisableBackgroundTasks {
		worker.Start(ctx)
	}

	geminiDesc := "disabled"
	if len(cfg.GeminiAPIKeys) > 0 {
		geminiDesc = fmt.Sprintf("enabled (%d keys)", len(cfg.GeminiAPIKeys))
	}
	tvdbDesc := "disabled"
	if tvdbClient != nil {
		tvdbDesc = "enabled"
	}
	tmdbDesc := "disabled"
	if tmdbClient != nil {
		tmdbDesc = "enabled"
	}
	vapidDesc := "disabled"
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		vapidDesc = "enabled"
	}
	radarrDesc := "disabled"
	if cfg.RadarrURL != "" && cfg.RadarrAPIKey != "" {
		radarrDesc = "enabled"
	}
	sonarrDesc := "disabled"
	if cfg.SonarrURL != "" && cfg.SonarrAPIKey != "" {
		sonarrDesc = "enabled"
	}
	prowlarrDesc := "disabled"
	if cfg.ProwlarrURL != "" && cfg.ProwlarrAPIKey != "" {
		prowlarrDesc = "enabled"
	}

	log.Printf("[Trackarr %s] Starting server on %s (data: %s)", version.Info(), cfg.ListenAddr, cfg.DataDir)
	log.Printf("[Trackarr] Database: %s (SQLite WAL mode)", dbPath)
	log.Printf("[Trackarr] Metadata: TMDB=%s TVDB=%s Gemini=%s AniList=enabled", tmdbDesc, tvdbDesc, geminiDesc)
	log.Printf("[Trackarr] Integrations: Radarr=%s Sonarr=%s Prowlarr=%s WebPush=%s", radarrDesc, sonarrDesc, prowlarrDesc, vapidDesc)

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: r,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[Trackarr] Ready and listening on http://%s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
