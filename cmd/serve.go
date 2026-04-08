package cmd

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/router"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

func Serve(distFS embed.FS) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := cfg.DataDir + "/plextracker.db"
	db, err := database.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Background refresh job
	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	taskRepo := repository.NewTaskRepository(db)

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

	var pushSvc service.PushNotifier = service.NewNoopNotifier()
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc = service.NewPushService(settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	}

	bgSvc := service.NewBackgroundService(titleRepo, seasonRepo, episodeRepo, taskRepo, settingRepo, tmdbClient, anilistClient, pushSvc, cfg.DataDir)
	if !cfg.DisableBackgroundTasks {
		bgSvc.StartTicker(24 * time.Hour)
	}

	r := router.New(cfg, db, distFS, bgSvc, pipeline)

	// Task queue worker
	worker := service.NewTaskQueueWorker(taskRepo, titleRepo, pipeline, tmdbClient, anilistClient, pushSvc, settingRepo, cfg.DataDir)
	if !cfg.DisableBackgroundTasks {
		worker.Start(context.Background())
	}

	log.Printf("PlexTracker listening on %s", cfg.ListenAddr)
	return http.ListenAndServe(cfg.ListenAddr, r)
}
