package router

import (
	"context"
	"database/sql"
	"embed"
	"sync"
	"time"

	"github.com/Soviann/trackarr/internal/config"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	mw "github.com/Soviann/trackarr/internal/middleware"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/service/matching"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func New(ctx context.Context, cfg *config.Config, writeDB, readDB *sql.DB, distFS embed.FS, bgSvc *service.BackgroundService, pipeline *matching.Pipeline, shutdownWG *sync.WaitGroup) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RealIP) //nolint:staticcheck // Chi deprecated RealIP in v5.3
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
	vapidPub, vapidPriv, vapidSub, _ := service.EnsureVAPIDKeys(ctx, writeDB, settingRepo, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject)
	var pushSvc service.PushNotifier
	var realPushSvc *service.PushService
	if vapidPub != "" && vapidPriv != "" {
		realPushSvc = service.NewPushService(writeDB, settingRepo, vapidPub, vapidPriv, vapidSub)
		pushSvc = realPushSvc
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
	plexSvc := service.NewPlexService(writeDB, pipeline, titleSvc, libSvc)

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
	var anilistClient *matching.AniListClient
	if pipeline != nil {
		anilistClient = pipeline.AniList()
	}
	if anilistClient == nil {
		anilistClient = matching.NewAniListClient()
	}
	anilistSearch := handler.NewAniListSearchHandler(anilistClient)
	episodes := handler.NewEpisodeHandler(writeDB, libSvc, titleReadRepo)
	backupSvc := service.NewBackupService(writeDB, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo)
	admin := handler.NewAdminHandler(ctx, writeDB, taskRepo, titleRepo, settingRepo, bgSvc, backupSvc)
	admin.SetShutdownWG(shutdownWG)
	arrSvc := service.NewArrService(cfg, settingRepo, titleRepo, writeDB)
	arr := handler.NewArrHandler(arrSvc, titleRepo, writeDB)
	prowlarrSvc := service.NewProwlarrService(cfg, settingRepo, titleReadRepo, tmdbClient)
	releasesHandler := handler.NewReleasesHandler(writeDB, prowlarrSvc, titleRepo, taskRepo)
	covers := handler.NewCoverHandler(cfg.DataDir)
	webhooks := handler.NewWebhookHandler(jellyfinSvc, plexSvc, cfg.JellyfinWebhookSecret, cfg.PlexWebhookSecret, cfg.WebhookSecret)
	push := handler.NewPushHandler(pushSvc)
	anilistAuth := handler.NewAniListAuthHandler(writeDB, settingRepo, cfg.AniListClientID)
	tvdbReady := pipeline != nil && pipeline.TVDB() != nil
	settings := handler.NewSettingsHandler(settingRepo, eventRepo, tvdbReady, cfg.JellyfinWebhookSecret != "" || cfg.PlexWebhookSecret != "" || cfg.WebhookSecret != "", prowlarrSvc)
	wrappedRepo := repository.NewWrappedRepository(writeDB)
	stats := handler.NewStatsHandler(writeDB, statsRepo, wrappedRepo, pipeline)
	genres := handler.NewGenreHandler(genreReadRepo)
	activity := handler.NewActivityHandler(activityRepo)
	history := handler.NewHistoryHandler(historyRepo)
	matchEvents := handler.NewMatchEventHandler(matchEventRepo)
	seasonAuditSvc := service.NewSeasonAuditService(writeDB, titleRepo, repository.NewSeasonAuditRepository(readDB), pipeline, titleSvc)
	seasonAudit := handler.NewSeasonAuditHandler(seasonAuditSvc)
	clientErrors := &handler.ClientErrorHandler{}

	calendarSvc := service.NewCalendarService(writeDB, titleReadRepo, settingRepo)
	calendarHandler := handler.NewCalendarHandler(calendarSvc)

	reloader := service.NewDynamicConfigReloader(
		cfg, writeDB, settingRepo, pipeline, bgSvc, nil, nil, realPushSvc,
		func(jellyfin, plex, fallback string) {
			webhooks.SetSecrets(jellyfin, plex, fallback)
		},
	)
	adminSettings := handler.NewAdminSettingsHandler(writeDB, settingRepo, reloader)

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handler.Health)

		// Auth (unauthenticated, rate-limited)
		authRateLimit := mw.RateLimit(ctx, 10, time.Minute)
		auth := handler.NewAuthHandler(writeDB, settingRepo, cfg.JWTSecret, cfg.GoogleAllowedEmail, cfg.GoogleClientID, cfg.CookieSecure)
		jwtSecret := auth.JWTSecret()

		r.Get("/config", httputil.WrapHandler(auth.PublicConfig))
		r.With(authRateLimit).Post("/auth/setup", httputil.WrapHandler(auth.Setup))
		r.With(authRateLimit).Post("/auth/login", httputil.WrapHandler(auth.Login))
		r.With(authRateLimit).Post("/auth/google", httputil.WrapHandler(auth.GoogleCallback))
		r.With(authRateLimit).Post("/auth/recover", httputil.WrapHandler(auth.Recover))
		r.Post("/auth/logout", httputil.WrapHandler(auth.Logout))

		// Media server webhooks (secured by secret token in URL, rate-limited)
		webhookRateLimit := mw.RateLimit(ctx, 60, time.Minute)
		r.With(webhookRateLimit).Post("/webhook/jellyfin/{secret}", httputil.WrapHandler(webhooks.HandleJellyfin))
		r.With(webhookRateLimit).Post("/webhook/plex/{secret}", httputil.WrapHandler(webhooks.HandlePlex))

		// Calendar iCal feed (unauthenticated, secured by secret token query parameter, rate-limited)
		calendarRateLimit := mw.RateLimit(ctx, 60, time.Minute)
		r.With(calendarRateLimit).Get("/calendar.ics", httputil.WrapHandler(calendarHandler.ServeICS))

		// Covers (unauthenticated for caching)
		r.Get("/covers/{filename}", covers.Serve)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(mw.JWTAuth(jwtSecret))

			r.Get("/calendar/events", httputil.WrapHandler(calendarHandler.GetEvents))
			r.Get("/calendar/token", httputil.WrapHandler(calendarHandler.GetToken))
			r.Post("/calendar/token/regenerate", httputil.WrapHandler(calendarHandler.RegenerateToken))

			r.Post("/auth/change-password", httputil.WrapHandler(auth.ChangePassword))
			r.Post("/auth/recovery-key/regenerate", httputil.WrapHandler(auth.RegenerateRecoveryKey))
			r.Get("/admin/auth-settings", httputil.WrapHandler(auth.GetAuthSettings))
			r.Put("/admin/auth-settings", httputil.WrapHandler(auth.UpdateAuthSettings))

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
			r.Get("/anilist/search", httputil.WrapHandler(anilistSearch.Search))
			r.Get("/releases", httputil.WrapHandler(releasesHandler.List))
			r.Post("/releases/add", httputil.WrapHandler(releasesHandler.Add))

			r.Patch("/titles/{titleID}/episodes/{episodeID}", httputil.WrapHandler(episodes.ToggleWatched))
			r.Post("/titles/{titleID}/episodes/batch-watch", httputil.WrapHandler(episodes.BatchMarkWatched))

			seasonExternal := handler.NewSeasonExternalHandler(writeDB)
			r.Post("/titles/{titleID}/seasons/{seasonID}/anilist", httputil.WrapHandler(seasonExternal.AddAniListID))
			r.Delete("/titles/{titleID}/seasons/{seasonID}/anilist/{externalID}", httputil.WrapHandler(seasonExternal.RemoveAniListID))
			r.Put("/titles/{titleID}/seasons/{seasonID}/anilist/order", httputil.WrapHandler(seasonExternal.ReorderAniList))

			r.Post("/push/subscribe", httputil.WrapHandler(push.Subscribe))
			r.Delete("/push/subscribe", httputil.WrapHandler(push.Unsubscribe))

			r.Get("/stats", httputil.WrapHandler(stats.Get))
			r.Get("/stats/wrapped", httputil.WrapHandler(stats.GetWrapped))
			r.Get("/stats/wrapped/archives", httputil.WrapHandler(stats.GetWrappedArchives))
			r.Post("/stats/wrapped/generate", httputil.WrapHandler(stats.RegenerateWrapped))
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
			r.Get("/admin/arr", httputil.WrapHandler(admin.GetArrSettings))
			r.Put("/admin/arr", httputil.WrapHandler(admin.UpdateArrSettings))
			r.Get("/admin/system-settings", httputil.WrapHandler(adminSettings.GetSystemSettings))
			r.Put("/admin/system-settings", httputil.WrapHandler(adminSettings.UpdateSystemSettings))
			r.Post("/admin/system-settings/test/tmdb", httputil.WrapHandler(adminSettings.TestTMDB))
			r.Post("/admin/system-settings/test/tvdb", httputil.WrapHandler(adminSettings.TestTVDB))
			r.Post("/admin/system-settings/test/gemini", httputil.WrapHandler(adminSettings.TestGemini))
			r.Post("/admin/system-settings/test/{app}", httputil.WrapHandler(adminSettings.TestArr))
			r.Post("/admin/system-settings/vapid/generate", httputil.WrapHandler(adminSettings.GenerateVAPIDKeys))
			r.Post("/admin/refresh-all", httputil.WrapHandler(admin.RefreshAll))
			r.Get("/admin/export/json", httputil.WrapHandler(admin.ExportJSON))
			r.Get("/admin/export/csv", httputil.WrapHandler(admin.ExportCSV))
			r.Get("/admin/export/trakt", httputil.WrapHandler(admin.ExportTrakt))
			r.Post("/admin/import", httputil.WrapHandler(admin.ImportBackup))
			r.Get("/admin/season-audit", httputil.WrapHandler(seasonAudit.List))
			r.Post("/admin/season-audit/accept", httputil.WrapHandler(seasonAudit.Accept))
			r.Post("/admin/season-audit/dismiss", httputil.WrapHandler(seasonAudit.Dismiss))

			// Arr API
			r.Route("/arr", func(r chi.Router) {
				r.Get("/{app}/rootfolder", httputil.WrapHandler(arr.ProxyRootFolder))
				r.Get("/{app}/qualityprofile", httputil.WrapHandler(arr.ProxyQualityProfile))
				r.Get("/title/{id}", httputil.WrapHandler(arr.GetTitleArr))
				r.Put("/title/{id}", httputil.WrapHandler(arr.UpdateTitleArr))
				r.Post("/push/{id}", httputil.WrapHandler(arr.PushToArr))
				r.Post("/queue/{id}/push", httputil.WrapHandler(arr.PushToArr))
			})

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
