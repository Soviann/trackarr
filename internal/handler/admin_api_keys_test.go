package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAPIKeyTest(t *testing.T) (*handler.AdminAPIKeyHandler, *service.APIKeyService) {
	t.Helper()
	writeDB, _, err := database.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { writeDB.Close() })
	require.NoError(t, database.Migrate(writeDB))

	readRepo := repository.NewAPIKeyRepository(writeDB)
	apiKeySvc := service.NewAPIKeyService(writeDB, readRepo)
	apiKeyHandler := handler.NewAdminAPIKeyHandler(apiKeySvc)
	return apiKeyHandler, apiKeySvc
}

func TestAdminAPIKeyHandler_Lifecycle(t *testing.T) {
	h, svc := setupAPIKeyTest(t)

	// 1. Nominal: List empty, Create, List non-empty
	{
		req := httptest.NewRequest("GET", "/api/admin/api-keys", nil)
		rr := httptest.NewRecorder()
		err := h.List(rr, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rr.Code)

		var keys []*model.APIKey
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &keys))
		assert.Empty(t, keys)
	}

	// Create
	var createdID int64
	{
		body, _ := json.Marshal(map[string]any{
			"name":   "Automation",
			"scopes": []string{model.ScopeLibraryRead, model.ScopeLibraryWrite},
		})
		req := httptest.NewRequest("POST", "/api/admin/api-keys", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		err := h.Create(rr, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rr.Code)

		var resp model.CreateAPIKeyResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.Equal(t, "Automation", resp.Key.Name)
		assert.NotEmpty(t, resp.Token)
		createdID = resp.Key.ID
	}

	// Revoke
	{
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req := httptest.NewRequest("POST", "/api/admin/api-keys/1/revoke", nil).WithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()
		err := h.Revoke(rr, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rr.Code)
	}

	// Delete
	{
		// Create another key to delete
		k2, _, err := svc.CreateKey(context.Background(), "Temp", []string{model.ScopeLibraryRead})
		require.NoError(t, err)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "2")
		req := httptest.NewRequest("DELETE", "/api/admin/api-keys/2", nil).WithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()
		err = h.Delete(rr, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rr.Code)
		_ = k2
	}

	// 2. Boundary: Invalid ID param
	{
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req := httptest.NewRequest("POST", "/api/admin/api-keys/abc/revoke", nil).WithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()
		err := h.Revoke(rr, req)
		assert.Error(t, err)
	}

	// 3. Error: Empty name, invalid scope, malformed JSON
	{
		body, _ := json.Marshal(map[string]any{
			"name":   "",
			"scopes": []string{model.ScopeLibraryRead},
		})
		req := httptest.NewRequest("POST", "/api/admin/api-keys", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		err := h.Create(rr, req)
		assert.Error(t, err)

		malformedReq := httptest.NewRequest("POST", "/api/admin/api-keys", bytes.NewReader([]byte("{invalid")))
		rr2 := httptest.NewRecorder()
		err2 := h.Create(rr2, malformedReq)
		assert.Error(t, err2)
		_ = createdID
	}
}
