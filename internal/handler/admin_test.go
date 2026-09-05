package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminHandler(t *testing.T) *handler.AdminHandler {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })
	taskRepo := repository.NewTaskRepository(db)
	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)
	backupSvc := service.NewBackupService(db, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo)
	return handler.NewAdminHandler(context.Background(), db, taskRepo, titleRepo, settingRepo, nil, backupSvc) // bgSvc=nil
}

func TestAdminHandler_Counts(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/counts", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.Counts(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]int
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, 0, body["dead_tasks"])
	assert.Equal(t, 0, body["pending_validations"])
}

func TestAdminHandler_ListTasks(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/tasks?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.ListTasks(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, float64(0), body["total"])
}

func TestAdminHandler_ListTasks_Filter(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/tasks?filter=dead", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.ListTasks(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAdminHandler_RetryTask_InvalidID(t *testing.T) {
	h := setupAdminHandler(t)

	r := chi.NewRouter()
	r.Post("/tasks/{id}/retry", httputil.WrapHandler(h.RetryTask))

	req := httptest.NewRequest(http.MethodPost, "/tasks/abc/retry", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminHandler_RetryTask(t *testing.T) {
	h := setupAdminHandler(t)

	r := chi.NewRouter()
	r.Post("/tasks/{id}/retry", httputil.WrapHandler(h.RetryTask))

	req := httptest.NewRequest(http.MethodPost, "/tasks/99999/retry", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdminHandler_DeleteTask_InvalidID(t *testing.T) {
	h := setupAdminHandler(t)

	r := chi.NewRouter()
	r.Delete("/tasks/{id}", httputil.WrapHandler(h.DeleteTask))

	req := httptest.NewRequest(http.MethodDelete, "/tasks/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminHandler_DeleteTask(t *testing.T) {
	h := setupAdminHandler(t)

	r := chi.NewRouter()
	r.Delete("/tasks/{id}", httputil.WrapHandler(h.DeleteTask))

	req := httptest.NewRequest(http.MethodDelete, "/tasks/99999", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdminHandler_DeleteTasksBatch_InvalidJSON(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/tasks/batch", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	err := h.DeleteTasksBatch(rr, req)

	require.Error(t, err)
	apiErr, ok := err.(*httputil.APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestAdminHandler_DeleteTasksBatch(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/tasks/batch", strings.NewReader(`{"ids":[]}`))
	rr := httptest.NewRecorder()
	require.NoError(t, h.DeleteTasksBatch(rr, req))

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdminHandler_GetNotificationPrefs(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/notification-prefs", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.GetNotificationPrefs(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]bool
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	// Default when key is absent = enabled (true)
	assert.True(t, body[service.NotifRatingPrompt])
	assert.True(t, body[service.NotifDeadTask])
	assert.True(t, body[service.NotifSeriesEnded])
}

func TestAdminHandler_UpdateNotificationPrefs_InvalidJSON(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodPut, "/api/admin/notification-prefs", strings.NewReader("{bad}"))
	rr := httptest.NewRecorder()
	err := h.UpdateNotificationPrefs(rr, req)

	require.Error(t, err)
	apiErr, ok := err.(*httputil.APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestAdminHandler_UpdateNotificationPrefs(t *testing.T) {
	h := setupAdminHandler(t)

	body := `{"notif_dead_task":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/notification-prefs", strings.NewReader(body))
	rr := httptest.NewRecorder()
	require.NoError(t, h.UpdateNotificationPrefs(rr, req))

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestAdminHandler_RefreshAll_NilBgSvc(t *testing.T) {
	h := setupAdminHandler(t) // bgSvc=nil

	req := httptest.NewRequest(http.MethodPost, "/api/admin/refresh-all", nil)
	rr := httptest.NewRecorder()
	err := h.RefreshAll(rr, req)

	require.Error(t, err)
	apiErr, ok := err.(*httputil.APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Status)
}

func TestAdminHandler_ExportJSON(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/export/json", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.ExportJSON(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "attachment; filename=\"trackarr-backup-")
}

func TestAdminHandler_ExportCSV(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/export/csv", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.ExportCSV(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/csv; charset=utf-8", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "attachment; filename=\"trackarr-library-")
}

func TestAdminHandler_ExportTrakt(t *testing.T) {
	h := setupAdminHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/export/trakt", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.ExportTrakt(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "attachment; filename=\"trackarr-trakt-sync-")
}

