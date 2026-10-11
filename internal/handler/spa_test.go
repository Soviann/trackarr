package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/Soviann/trackarr/internal/handler"
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

func TestSPAHandler_CacheControl(t *testing.T) {
	mockFS := fstest.MapFS{
		"frontend/dist/index.html": &fstest.MapFile{
			Data: []byte("<!DOCTYPE html><html><body>Root</body></html>"),
		},
		"frontend/dist/assets/app.12345.js": &fstest.MapFile{
			Data: []byte("console.log('test');"),
		},
		"frontend/dist/favicon.ico": &fstest.MapFile{
			Data: []byte("favicon"),
		},
	}
	h := handler.SPAHandler(mockFS)

	tests := []struct {
		name                string
		path                string
		expectedCacheHeader string
	}{
		{
			name:                "hashed asset gets immutable cache header",
			path:                "/assets/app.12345.js",
			expectedCacheHeader: "public, max-age=31536000, immutable",
		},
		{
			name:                "root index.html gets no-cache header",
			path:                "/",
			expectedCacheHeader: "no-cache, no-store, must-revalidate",
		},
		{
			name:                "explicit index.html gets no-cache header",
			path:                "/index.html",
			expectedCacheHeader: "no-cache, no-store, must-revalidate",
		},
		{
			name:                "non-asset static file gets no-cache header",
			path:                "/favicon.ico",
			expectedCacheHeader: "no-cache, no-store, must-revalidate",
		},
		{
			name:                "spa route fallback gets no-cache header",
			path:                "/titles/42",
			expectedCacheHeader: "no-cache, no-store, must-revalidate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.expectedCacheHeader, rr.Header().Get("Cache-Control"))
		})
	}
}
