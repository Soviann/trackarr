package router

import (
	"database/sql"
	"embed"
	"log"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/handler"
	mw "github.com/nicolasvasse/plextracker/internal/middleware"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

func New(cfg *config.Config, db *sql.DB, distFS embed.FS) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Repositories
	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)

	// Matching pipeline (optional — degrades gracefully if APIs not configured)
	var pipeline *matching.Pipeline
	if cfg.TMDBAPIKey != "" {
		tmdbClient := matching.NewTMDBClient(cfg.TMDBAPIKey)
		anilistClient := matching.NewAniListClient()

		var geminiClient *matching.GeminiClient
		if len(cfg.GeminiAPIKeys) > 0 {
			geminiClient = matching.NewGeminiClient(cfg.GeminiAPIKeys)
		}

		var crossDB *matching.CrossRefDB
		crossrefPath := filepath.Join(cfg.DataDir, "anime-offline-database.json")
		if cdb, err := matching.LoadCrossRefDB(crossrefPath); err == nil {
			crossDB = cdb
		} else {
			log.Printf("crossref DB not loaded (optional): %v", err)
		}

		pipeline = matching.NewPipeline(tmdbClient, anilistClient, geminiClient, crossDB, cfg.DataDir)
	}

	// Repositories (settings)
	settingRepo := repository.NewSettingRepository(db)

	// Services
	plexSvc := service.NewPlexService(titleRepo, seasonRepo, episodeRepo, eventRepo, pipeline)

	var pushSvc *service.PushService
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc = service.NewPushService(settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	}

	// Handlers
	titles := handler.NewTitleHandler(titleRepo, seasonRepo, episodeRepo, eventRepo)
	episodes := handler.NewEpisodeHandler(titleRepo, episodeRepo, eventRepo)
	seasons := handler.NewSeasonHandler(seasonRepo)
	covers := handler.NewCoverHandler(cfg.DataDir)
	webhooks := handler.NewWebhookHandler(plexSvc)
	push := handler.NewPushHandler(pushSvc)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handler.Health)
		r.Get("/config", handler.PublicConfig(cfg.GoogleClientID, cfg.VAPIDPublicKey))

		// Auth (unauthenticated)
		auth := handler.NewAuthHandler(cfg.JWTSecret, cfg.GoogleAllowedEmail, cfg.GoogleClientID)
		r.Post("/auth/google", auth.GoogleCallback)
		r.Post("/auth/logout", auth.Logout)

		// Plex webhook (unauthenticated)
		r.Post("/webhook/plex", webhooks.HandlePlex)

		// Covers (unauthenticated for caching)
		r.Get("/covers/{filename}", covers.Serve)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(mw.JWTAuth(cfg.JWTSecret))

			r.Get("/titles", titles.List)
			r.Get("/titles/{id}", titles.GetByID)
			r.Post("/titles", titles.Create)
			r.Patch("/titles/{id}", titles.Update)

			r.Patch("/titles/{titleID}/episodes/{episodeID}", episodes.ToggleWatched)
			r.Post("/titles/{titleID}/episodes/batch-watch", episodes.BatchMarkWatched)

			r.Patch("/titles/{titleID}/seasons/{seasonID}", seasons.UpdateRating)

			r.Post("/push/subscribe", push.Subscribe)
			r.Delete("/push/subscribe", push.Unsubscribe)
		})
	})

	// SPA catch-all
	r.Handle("/*", handler.SPAHandler(distFS))

	return r
}
