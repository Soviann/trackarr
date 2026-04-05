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
	pending, err := h.tasks.ListPending()
	if err != nil {
		return httputil.InternalError("list pending tasks", err)
	}

	dead, err := h.tasks.ListDead()
	if err != nil {
		return httputil.InternalError("list dead tasks", err)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"pending": pending,
		"dead":    dead,
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

// GetNotificationPrefs returns all notification preferences.
func (h *AdminHandler) GetNotificationPrefs(w http.ResponseWriter, r *http.Request) error {
	prefs := map[string]bool{
		service.NotifRatingPrompt: service.IsNotificationEnabled(h.settings, service.NotifRatingPrompt),
		service.NotifDeadTask:     service.IsNotificationEnabled(h.settings, service.NotifDeadTask),
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
		case service.NotifRatingPrompt, service.NotifDeadTask:
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
