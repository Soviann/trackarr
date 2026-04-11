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
	seasonRepo := repository.NewSeasonRepository(writeDB)
	episodeRepo := repository.NewEpisodeRepository(writeDB)
	settingRepo := repository.NewSettingRepository(writeDB)
	taskRepo := repository.NewTaskRepository(writeDB)
	watchEventRepo := repository.NewWatchEventRepository(writeDB)

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
		pushSvc = service.NewPushService(settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	}

	titleSvc := service.NewTitleService(writeDB, titleRepo, taskRepo, pipeline)

	bgGenreRepo := repository.NewGenreRepository(writeDB)
	bgSvc := service.NewBackgroundService(titleRepo, bgGenreRepo, seasonRepo, episodeRepo, taskRepo, settingRepo, tmdbClient, anilistClient, pushSvc, cfg.DataDir)
	if tvdbClient != nil {
		bgSvc.SetTVDB(tvdbClient)
	}
	if !cfg.DisableBackgroundTasks {
		bgSvc.StartTicker(ctx, 24*time.Hour)
	}

	r := router.New(ctx, cfg, writeDB, readDB, distFS, bgSvc, pipeline)

	// Task queue worker
	worker := service.NewTaskQueueWorker(taskRepo, titleRepo, watchEventRepo, bgGenreRepo, pipeline, tmdbClient, anilistClient, pushSvc, settingRepo, cfg.DataDir, titleSvc)
	if !cfg.DisableBackgroundTasks {
		worker.Start(ctx)
	}

	log.Printf("PlexTracker listening on %s", cfg.ListenAddr)
	return http.ListenAndServe(cfg.ListenAddr, r)
}
