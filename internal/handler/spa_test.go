package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicConfig_OmitsDevLoginWhenDisabled(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	handler.PublicConfig("client-id", "vapid-key", false).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	assert.Equal(t, "client-id", body["google_client_id"])
	assert.Equal(t, "vapid-key", body["vapid_public_key"])
	_, hasDevLogin := body["dev_login"]
	assert.False(t, hasDevLogin, "dev_login must not appear in prod responses")
}

func TestPublicConfig_IncludesDevLoginWhenEnabled(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	handler.PublicConfig("client-id", "vapid-key", true).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	assert.Equal(t, true, body["dev_login"])
}
