package cmd

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
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

	r := router.New(cfg, db, distFS)

	// Background refresh job
	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	var tmdbClient *matching.TMDBClient
	var pipeline *matching.Pipeline
	if cfg.TMDBAPIKey != "" {
		tmdbClient = matching.NewTMDBClient(cfg.TMDBAPIKey)
	}
	anilistClient := matching.NewAniListClient()

	var pushSvc service.PushNotifier = service.NewNoopNotifier()
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc = service.NewPushService(settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	}

	bgSvc := service.NewBackgroundService(titleRepo, seasonRepo, episodeRepo, taskRepo, settingRepo, tmdbClient, anilistClient, pushSvc, cfg.DataDir)
	bgSvc.StartTicker(24 * time.Hour)

	// Task queue worker
	worker := service.NewTaskQueueWorker(taskRepo, titleRepo, pipeline, tmdbClient, anilistClient, pushSvc, settingRepo, cfg.DataDir)
	worker.Start(context.Background())

	log.Printf("PlexTracker listening on %s", cfg.ListenAddr)
	return http.ListenAndServe(cfg.ListenAddr, r)
}
