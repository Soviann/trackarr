package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

type AdminHandler struct {
	tasks    *repository.TaskRepository
	titles   *repository.TitleRepository
	settings *repository.SettingRepository
	bgSvc    *service.BackgroundService
}

func NewAdminHandler(tasks *repository.TaskRepository, titles *repository.TitleRepository, settings *repository.SettingRepository, bgSvc *service.BackgroundService) *AdminHandler {
	return &AdminHandler{tasks: tasks, titles: titles, settings: settings, bgSvc: bgSvc}
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

	httputil.WriteJSON(w, http.StatusOK, map[string]int{
		"dead_tasks":          deadCount,
		"pending_validations": statusCounts.PendingReview,
	})
	return nil
}

// ListTasks returns all non-completed tasks (pending + dead).
func (h *AdminHandler) ListTasks(w http.ResponseWriter, r *http.Request) error {
	filter := r.URL.Query().Get("filter")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
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

	if err := h.tasks.RetryDead(id); err != nil {
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

	if err := h.tasks.Delete(id); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}

	if err := h.tasks.DeleteBatch(req.IDs); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		return httputil.BadRequest("Invalid JSON")
	}

	for key, enabled := range prefs {
		// Only allow known keys
		switch key {
		case service.NotifRatingPrompt, service.NotifDeadTask, service.NotifSeriesEnded:
			val := "true"
			if !enabled {
				val = "false"
			}
			if err := h.settings.Set(key, val); err != nil {
				return httputil.InternalError("save notification pref", err)
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// RefreshAll triggers a background refresh on all titles (including completed/dropped).
func (h *AdminHandler) RefreshAll(w http.ResponseWriter, r *http.Request) error {
	if h.bgSvc == nil {
		return httputil.InternalError("refresh all", fmt.Errorf("background service not available"))
	}

	go h.bgSvc.RefreshAllTitles()

	w.WriteHeader(http.StatusAccepted)
	return nil
}
