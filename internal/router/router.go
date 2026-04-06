package router

import (
	"database/sql"
	"embed"
	"log"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	mw "github.com/nicolasvasse/plextracker/internal/middleware"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

func New(cfg *config.Config, db *sql.DB, distFS embed.FS, bgSvc *service.BackgroundService) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(mw.SecurityHeaders)

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
	var pushSvc service.PushNotifier = service.NewNoopNotifier()
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc = service.NewPushService(settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	}

	taskRepo := repository.NewTaskRepository(db)
	plexSvc := service.NewPlexService(db, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, settingRepo, pipeline, pushSvc)

	// Backfill service (optional — requires TMDB for full backfill)
	var tmdbClient *matching.TMDBClient
	if pipeline != nil {
		tmdbClient = pipeline.TMDB()
	}
	backfillSvc := service.NewBackfillService(db, tmdbClient)

	// Stats repository
	statsRepo := repository.NewStatsRepository(db)

	// Handlers
	titles := handler.NewTitleHandler(titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo)

	// TMDB search handler (optional — requires TMDB key)
	tmdbSearch := handler.NewTMDBHandler(tmdbClient)
	episodes := handler.NewEpisodeHandler(titleRepo, episodeRepo, eventRepo, settingRepo, pushSvc, backfillSvc)
	admin := handler.NewAdminHandler(taskRepo, titleRepo, settingRepo, bgSvc)
	seasons := handler.NewSeasonHandler(seasonRepo)
	covers := handler.NewCoverHandler(cfg.DataDir)
	webhooks := handler.NewWebhookHandler(plexSvc, cfg.PlexWebhookSecret)
	push := handler.NewPushHandler(pushSvc)
	anilistAuth := handler.NewAniListAuthHandler(settingRepo, cfg.AniListClientID)
	settings := handler.NewSettingsHandler(settingRepo)
	stats := handler.NewStatsHandler(statsRepo)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handler.Health)
		r.Get("/config", handler.PublicConfig(cfg.GoogleClientID, cfg.VAPIDPublicKey, cfg.DebugLogin))

		// Auth (unauthenticated, rate-limited)
		authRateLimit := mw.RateLimit(10, time.Minute)
		auth := handler.NewAuthHandler(cfg.JWTSecret, cfg.GoogleAllowedEmail, cfg.GoogleClientID, cfg.CookieSecure)
		if cfg.DebugLogin && cfg.DebugLoginUser != "" && cfg.DebugLoginPassword != "" {
			auth.WithDevLogin(cfg.DebugLoginUser, cfg.DebugLoginPassword)
			r.With(authRateLimit).Post("/auth/dev", httputil.WrapHandler(auth.DevLogin))
			log.Println("⚠️  Dev login enabled — POST /api/auth/dev")
		}
		r.With(authRateLimit).Post("/auth/google", httputil.WrapHandler(auth.GoogleCallback))
		r.Post("/auth/logout", httputil.WrapHandler(auth.Logout))

		// Plex webhook (secured by secret token in URL)
		r.Post("/webhook/plex/{secret}", httputil.WrapHandler(webhooks.HandlePlex))

		// Covers (unauthenticated for caching)
		r.Get("/covers/{filename}", covers.Serve)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(mw.JWTAuth(cfg.JWTSecret))

			r.Get("/titles", httputil.WrapHandler(titles.List))
			r.Get("/titles/{id}", httputil.WrapHandler(titles.GetByID))
			r.Post("/titles", httputil.WrapHandler(titles.Create))
			r.Patch("/titles/{id}", httputil.WrapHandler(titles.Update))
			r.Post("/titles/{id}/rematch", httputil.WrapHandler(titles.Rematch))

			r.Get("/tmdb/search", httputil.WrapHandler(tmdbSearch.Search))

			r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(episodes.ToggleWatched))
			r.Post("/titles/{titleID}/episodes/batch-watch", httputil.WrapHandler(episodes.BatchMarkWatched))

			r.Patch("/titles/{titleID}/seasons/{seasonID}", httputil.WrapHandler(seasons.UpdateRating))

			r.Post("/push/subscribe", httputil.WrapHandler(push.Subscribe))
			r.Delete("/push/subscribe", httputil.WrapHandler(push.Unsubscribe))

			r.Get("/stats", httputil.WrapHandler(stats.Get))

			r.Get("/settings", httputil.WrapHandler(settings.Get))

			r.Get("/anilist/auth", httputil.WrapHandler(anilistAuth.Authorize))
			r.Post("/anilist/token", httputil.WrapHandler(anilistAuth.SaveToken))
			r.Delete("/anilist/token", httputil.WrapHandler(anilistAuth.Disconnect))

			r.Get("/admin/counts", httputil.WrapHandler(admin.Counts))
			r.Get("/admin/tasks", httputil.WrapHandler(admin.ListTasks))
			r.Post("/admin/tasks/{id}/retry", httputil.WrapHandler(admin.RetryTask))
			r.Delete("/admin/tasks/{id}", httputil.WrapHandler(admin.DeleteTask))
			r.Post("/admin/tasks/batch-delete", httputil.WrapHandler(admin.DeleteTasksBatch))
			r.Get("/admin/notifications", httputil.WrapHandler(admin.GetNotificationPrefs))
			r.Put("/admin/notifications", httputil.WrapHandler(admin.UpdateNotificationPrefs))
			r.Post("/admin/refresh-all", httputil.WrapHandler(admin.RefreshAll))
		})
	})

	// SPA catch-all
	r.Handle("/*", handler.SPAHandler(distFS))

	return r
}
