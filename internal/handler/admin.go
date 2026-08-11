package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

const deleteTasksBatchMaxIDs = 1000

type AdminHandler struct {
	serverCtx  context.Context // lifecycle ctx — cancelled on SIGTERM so fire-and-forget goroutines stop at shutdown
	writeDB    *sql.DB
	tasks      *repository.TaskRepository
	titles     *repository.TitleRepository
	settings   *repository.SettingRepository
	bgSvc      *service.BackgroundService
	shutdownWG *sync.WaitGroup // optional — joined on shutdown so RefreshAll goroutine can finish
}

func NewAdminHandler(serverCtx context.Context, writeDB *sql.DB, tasks *repository.TaskRepository, titles *repository.TitleRepository, settings *repository.SettingRepository, bgSvc *service.BackgroundService) *AdminHandler {
	return &AdminHandler{serverCtx: serverCtx, writeDB: writeDB, tasks: tasks, titles: titles, settings: settings, bgSvc: bgSvc}
}

// SetShutdownWG registers a WaitGroup that RefreshAll goroutines increment on
// dispatch and decrement on completion. cmd/serve.go waits on the same WG
// before closing the database, so the bulk refresh can finish in-flight writes
// instead of being killed mid-transaction.
func (h *AdminHandler) SetShutdownWG(wg *sync.WaitGroup) {
	h.shutdownWG = wg
}

// Counts returns aggregate counts for the admin hub badges.
func (h *AdminHandler) Counts(w http.ResponseWriter, r *http.Request) error {
	deadCount, err := h.tasks.CountDead()
	if err != nil {
		return httputil.InternalError("count dead tasks", err)
	}

	statusCounts, err := h.titles.GetStatusCounts()
	if err != nil {
		return httputil.InternalError("count pending validations", err)
	}

	arrQueue, err := h.titles.ListArrQueue()
	if err != nil {
		return httputil.InternalError("count arr queue", err)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]int{
		"dead_tasks":          deadCount,
		"pending_validations": statusCounts.PendingReview,
		"arr_queue":           len(arrQueue),
	})
	return nil
}

// ListTasks returns all non-completed tasks (pending + dead).
func (h *AdminHandler) ListTasks(w http.ResponseWriter, r *http.Request) error {
	filter := r.URL.Query().Get("filter")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	const maxLimit = 500
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	limit = min(limit, maxLimit)
	offset := 0
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	tasks, total, err := h.tasks.ListPaginated(filter, limit, offset)
	if err != nil {
		return httputil.InternalError("list tasks paginated", err)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"tasks": tasks,
		"total": total,
	})
	return nil
}

// RetryTask resets a dead task to pending.
func (h *AdminHandler) RetryTask(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return httputil.BadRequest("Invalid task ID")
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		return repository.NewTaskWriter(tx).RetryDead(r.Context(), id)
	}); err != nil {
		return httputil.InternalError("retry task", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// DeleteTask removes a task.
func (h *AdminHandler) DeleteTask(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return httputil.BadRequest("Invalid task ID")
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		return repository.NewTaskWriter(tx).Delete(r.Context(), id)
	}); err != nil {
		return httputil.InternalError("delete task", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// DeleteTasksBatch removes multiple tasks.
func (h *AdminHandler) DeleteTasksBatch(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := httputil.ReadJSON(r, &req, 1<<20); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}
	if len(req.IDs) > deleteTasksBatchMaxIDs {
		return httputil.BadRequest(fmt.Sprintf("too many IDs (max %d)", deleteTasksBatchMaxIDs))
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		return repository.NewTaskWriter(tx).DeleteBatch(r.Context(), req.IDs)
	}); err != nil {
		return httputil.InternalError("delete batch tasks", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GetNotificationPrefs returns all notification preferences.
func (h *AdminHandler) GetNotificationPrefs(w http.ResponseWriter, r *http.Request) error {
	prefs := map[string]bool{
		service.NotifRatingPrompt: service.IsNotificationEnabled(h.settings, service.NotifRatingPrompt),
		service.NotifDeadTask:     service.IsNotificationEnabled(h.settings, service.NotifDeadTask),
		service.NotifSeriesEnded:  service.IsNotificationEnabled(h.settings, service.NotifSeriesEnded),
	}
	httputil.WriteJSON(w, http.StatusOK, prefs)
	return nil
}

// UpdateNotificationPrefs updates notification preferences.
func (h *AdminHandler) UpdateNotificationPrefs(w http.ResponseWriter, r *http.Request) error {
	var prefs map[string]bool
	if err := httputil.ReadJSON(r, &prefs, 1<<20); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		writer := repository.NewSettingWriter(tx)
		for key, enabled := range prefs {
			// Only allow known keys
			switch key {
			case service.NotifRatingPrompt, service.NotifDeadTask, service.NotifSeriesEnded:
				val := "true"
				if !enabled {
					val = "false"
				}
				if err := writer.Set(r.Context(), key, val); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return httputil.InternalError("save notification prefs", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RefreshAll triggers a background refresh on all titles (including completed/dropped).
//
// Intentional fire-and-forget: 202 Accepted. Mirrors TitleHandler.RefreshOne —
// parent ctx is the server lifecycle so SIGTERM cancels the goroutine, the
// shutdown WG ensures Serve() waits for in-flight writes before closing the
// database, and a recover() prevents a TMDB-side panic from killing the goroutine
// silently. The 30-minute cap is generous for libraries up to a few thousand
// titles given the shared 2 rps API limiter; overrunning means the admin can
// re-trigger after investigation.
func (h *AdminHandler) RefreshAll(w http.ResponseWriter, r *http.Request) error {
	if h.bgSvc == nil {
		return httputil.InternalError("refresh all", fmt.Errorf("background service not available"))
	}

	ctx, cancel := context.WithTimeout(h.serverCtx, 30*time.Minute)
	if h.shutdownWG != nil {
		h.shutdownWG.Add(1)
	}
	go func() {
		if h.shutdownWG != nil {
			defer h.shutdownWG.Done()
		}
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				log.Printf("admin: refresh all panicked: %v\n%s", rec, stack)
			}
		}()
		h.bgSvc.RefreshAllTitles(ctx)
	}()

	w.WriteHeader(http.StatusAccepted)
	return nil
}

// GetArrSettings returns Radarr/Sonarr default settings.
func (h *AdminHandler) GetArrSettings(w http.ResponseWriter, r *http.Request) error {
	keys := []string{
		"radarr_std_monitored", "radarr_std_search", "radarr_std_root_folder", "radarr_std_quality_profile",
		"radarr_anime_monitored", "radarr_anime_search", "radarr_anime_root_folder", "radarr_anime_quality_profile",
		"sonarr_std_monitored", "sonarr_std_search", "sonarr_std_root_folder", "sonarr_std_quality_profile",
		"sonarr_anime_monitored", "sonarr_anime_search", "sonarr_anime_root_folder", "sonarr_anime_quality_profile",
	}

	prefs := make(map[string]string)
	for _, k := range keys {
		if val, err := h.settings.Get(k); err == nil {
			prefs[k] = val
		} else {
			prefs[k] = ""
		}
	}
	httputil.WriteJSON(w, http.StatusOK, prefs)
	return nil
}

// UpdateArrSettings updates Radarr/Sonarr default settings.
func (h *AdminHandler) UpdateArrSettings(w http.ResponseWriter, r *http.Request) error {
	var prefs map[string]string
	if err := httputil.ReadJSON(r, &prefs, 1<<20); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}

	allowedKeys := map[string]bool{
		"radarr_std_monitored": true, "radarr_std_search": true, "radarr_std_root_folder": true, "radarr_std_quality_profile": true,
		"radarr_anime_monitored": true, "radarr_anime_search": true, "radarr_anime_root_folder": true, "radarr_anime_quality_profile": true,
		"sonarr_std_monitored": true, "sonarr_std_search": true, "sonarr_std_root_folder": true, "sonarr_std_quality_profile": true,
		"sonarr_anime_monitored": true, "sonarr_anime_search": true, "sonarr_anime_root_folder": true, "sonarr_anime_quality_profile": true,
	}

	if err := database.WithTxContext(r.Context(), h.writeDB, func(tx *sql.Tx) error {
		writer := repository.NewSettingWriter(tx)
		for key, val := range prefs {
			if allowedKeys[key] {
				if err := writer.Set(r.Context(), key, val); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return httputil.InternalError("save arr settings", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
