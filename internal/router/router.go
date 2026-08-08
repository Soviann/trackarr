package router

import (
	"context"
	"database/sql"
	"embed"
	"log"
	"sync"
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

func New(ctx context.Context, cfg *config.Config, writeDB, readDB *sql.DB, distFS embed.FS, bgSvc *service.BackgroundService, pipeline *matching.Pipeline, shutdownWG *sync.WaitGroup) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(mw.RedactingLogger("/api/webhook/jellyfin/"))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(mw.SecurityHeaders)

	// Repositories (writes)
	titleRepo := repository.NewTitleRepository(writeDB)
	seasonRepo := repository.NewSeasonRepository(writeDB)
	episodeRepo := repository.NewEpisodeRepository(writeDB)
	eventRepo := repository.NewWatchEventRepository(writeDB)
	settingRepo := repository.NewSettingRepository(writeDB)
	taskRepo := repository.NewTaskRepository(writeDB)

	// Repositories (reads — use readDB so list queries don't block on background writes)
	titleReadRepo := repository.NewTitleRepository(readDB)

	// Services
	var pushSvc service.PushNotifier
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		pushSvc = service.NewPushService(writeDB, settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	} else {
		pushSvc = service.NewNoopNotifier()
	}

	// Backfill service (optional — requires TMDB for full backfill)
	var tmdbClient *matching.TMDBClient
	if pipeline != nil {
		tmdbClient = pipeline.TMDB()
	}
	backfillSvc := service.NewBackfillService(writeDB, tmdbClient)

	titleSvc := service.NewTitleService(writeDB, titleRepo, taskRepo, pipeline)
	libSvc := service.NewLibraryService(writeDB, titleRepo, seasonRepo, episodeRepo, eventRepo, settingRepo, pushSvc, backfillSvc, pipeline)

	jellyfinSvc := service.NewJellyfinService(writeDB, pipeline, titleSvc, libSvc)

	// Stats repository (read-only)
	statsRepo := repository.NewStatsRepository(readDB)
	activityRepo := repository.NewActivityRepository(readDB)
	historyRepo := repository.NewHistoryRepository(readDB)
	matchEventRepo := repository.NewMatchEventRepository(readDB)

	// Genre repository (read-only)
	genreReadRepo := repository.NewGenreRepository(readDB)

	// Handlers
	titles := handler.NewTitleHandler(ctx, writeDB, titleRepo, titleReadRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, pipeline, titleSvc, bgSvc)
	titles.SetShutdownWG(shutdownWG)
	library := handler.NewLibraryHandler(titleReadRepo)

	// TMDB search handler (optional — requires TMDB key)
	tmdbSearch := handler.NewTMDBHandler(tmdbClient)
	episodes := handler.NewEpisodeHandler(writeDB, libSvc, titleReadRepo)
	admin := handler.NewAdminHandler(ctx, writeDB, taskRepo, titleRepo, settingRepo, bgSvc)
	admin.SetShutdownWG(shutdownWG)
	covers := handler.NewCoverHandler(cfg.DataDir)
	webhooks := handler.NewWebhookHandler(jellyfinSvc, cfg.JellyfinWebhookSecret)
	push := handler.NewPushHandler(pushSvc)
	anilistAuth := handler.NewAniListAuthHandler(writeDB, settingRepo, cfg.AniListClientID)
	tvdbReady := pipeline != nil && pipeline.TVDB() != nil
	settings := handler.NewSettingsHandler(settingRepo, tvdbReady)
	stats := handler.NewStatsHandler(statsRepo)
	genres := handler.NewGenreHandler(genreReadRepo)
	activity := handler.NewActivityHandler(activityRepo)
	history := handler.NewHistoryHandler(historyRepo)
	matchEvents := handler.NewMatchEventHandler(matchEventRepo)
	seasonAuditSvc := service.NewSeasonAuditService(writeDB, titleRepo, repository.NewSeasonAuditRepository(readDB), pipeline, titleSvc)
	seasonAudit := handler.NewSeasonAuditHandler(seasonAuditSvc)
	clientErrors := &handler.ClientErrorHandler{}

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handler.Health)
		r.Get("/config", handler.PublicConfig(cfg.GoogleClientID, cfg.VAPIDPublicKey, cfg.DebugLogin))

		// Auth (unauthenticated, rate-limited)
		authRateLimit := mw.RateLimit(ctx, 10, time.Minute)
		auth := handler.NewAuthHandler(cfg.JWTSecret, cfg.GoogleAllowedEmail, cfg.GoogleClientID, cfg.CookieSecure)
		if cfg.DebugLogin && cfg.DebugLoginUser != "" && cfg.DebugLoginPassword != "" {
			auth.WithDevLogin(cfg.DebugLoginUser, cfg.DebugLoginPassword)
			r.Group(func(r chi.Router) {
				r.Use(authRateLimit)
				r.Post("/auth/dev", httputil.WrapHandler(auth.DevLogin))
			})
			log.Println("⚠️  Dev login enabled — POST /api/auth/dev")
		}
		r.With(authRateLimit).Post("/auth/google", httputil.WrapHandler(auth.GoogleCallback))
		r.Post("/auth/logout", httputil.WrapHandler(auth.Logout))

		// Jellyfin webhook (secured by secret token in URL, rate-limited)
		webhookRateLimit := mw.RateLimit(ctx, 60, time.Minute)
		r.With(webhookRateLimit).Post("/webhook/jellyfin/{secret}", httputil.WrapHandler(webhooks.HandleJellyfin))

		// Covers (unauthenticated for caching)
		r.Get("/covers/{filename}", covers.Serve)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(mw.JWTAuth(cfg.JWTSecret))

			r.Get("/titles", httputil.WrapHandler(titles.List))
			r.Post("/titles", httputil.WrapHandler(titles.Create))
			// Static sub-routes must come BEFORE /{id} to avoid chi matching them as ID params
			r.Get("/titles/review-count", httputil.WrapHandler(titles.ReviewCount))
			r.Get("/titles/resolve", httputil.WrapHandler(titles.Resolve))
			r.Get("/titles/continue-watching", httputil.WrapHandler(library.ContinueWatching))
			r.Get("/titles/upcoming", httputil.WrapHandler(library.Upcoming))
			r.Post("/titles/batch-delete", httputil.WrapHandler(titles.BatchDelete))
			r.Post("/titles/batch-status", httputil.WrapHandler(titles.BatchStatus))
			// Parameterized routes after static ones
			r.Get("/titles/{id}", httputil.WrapHandler(titles.GetByID))
			r.Patch("/titles/{id}", httputil.WrapHandler(titles.Update))
			r.Delete("/titles/{id}", httputil.WrapHandler(titles.Delete))
			r.Post("/titles/{id}/rematch", httputil.WrapHandler(titles.Rematch))
			r.Put("/titles/{id}/external-ids", httputil.WrapHandler(titles.SetExternalIDs))
			r.Post("/titles/{id}/merge", httputil.WrapHandler(titles.Merge))
			r.Post("/titles/{id}/refresh", httputil.WrapHandler(titles.RefreshOne))
			r.Get("/tmdb/search", httputil.WrapHandler(tmdbSearch.Search))

			r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(episodes.ToggleWatched))
			r.Post("/titles/{titleID}/episodes/batch-watch", httputil.WrapHandler(episodes.BatchMarkWatched))

			seasonExternal := handler.NewSeasonExternalHandler(writeDB)
			r.Post("/titles/{titleID}/seasons/{seasonID}/anilist", httputil.WrapHandler(seasonExternal.AddAniListID))
			r.Delete("/titles/{titleID}/seasons/{seasonID}/anilist/{externalID}", httputil.WrapHandler(seasonExternal.RemoveAniListID))
			r.Put("/titles/{titleID}/seasons/{seasonID}/anilist/order", httputil.WrapHandler(seasonExternal.ReorderAniList))

			r.Post("/push/subscribe", httputil.WrapHandler(push.Subscribe))
			r.Delete("/push/subscribe", httputil.WrapHandler(push.Unsubscribe))

			r.Get("/stats", httputil.WrapHandler(stats.Get))
			r.Get("/stats/activity", httputil.WrapHandler(activity.List))
			r.Get("/match-events", httputil.WrapHandler(matchEvents.List))
			r.Get("/genres", httputil.WrapHandler(genres.List))
			r.Get("/countries", httputil.WrapHandler(titles.Countries))

			r.Get("/titles/{id}/history", httputil.WrapHandler(history.Get))

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
			r.Get("/admin/season-audit", httputil.WrapHandler(seasonAudit.List))
			r.Post("/admin/season-audit/accept", httputil.WrapHandler(seasonAudit.Accept))
			r.Post("/admin/season-audit/dismiss", httputil.WrapHandler(seasonAudit.Dismiss))

			clientErrorsRateLimit := mw.RateLimit(ctx, 30, time.Minute)
			r.With(clientErrorsRateLimit).Post("/client-errors", httputil.WrapHandler(clientErrors.Handle))
		})
	})

	// Top-level cover route to support legacy /covers/{filename} URLs without /api prefix
	r.Get("/covers/{filename}", covers.Serve)

	// SPA catch-all
	r.Handle("/*", handler.SPAHandler(distFS))

	return r
}
