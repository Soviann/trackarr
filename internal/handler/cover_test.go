package handler_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Soviann/trackarr/internal/handler"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverHandler_Serve(t *testing.T) {
	tempDir := t.TempDir()
	coversDir := filepath.Join(tempDir, "covers")
	require.NoError(t, os.MkdirAll(coversDir, 0755))

	testFile := filepath.Join(coversDir, "poster.jpg")
	require.NoError(t, os.WriteFile(testFile, []byte("fake-image-data"), 0644))

	h := handler.NewCoverHandler(tempDir)

	r := chi.NewRouter()
	r.Get("/covers/{filename}", h.Serve)

	t.Run("serves existing file with immutable cache header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/covers/poster.jpg", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "fake-image-data", rr.Body.String())
		assert.Contains(t, rr.Header().Get("Cache-Control"), "immutable")
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/covers/..%2F..%2Fetc%2Fpasswd", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("returns 404 for missing file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/covers/missing.jpg", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("rejects root directory / dot path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/covers/.", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("rejects directory instead of file", func(t *testing.T) {
		subDir := filepath.Join(coversDir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		req := httptest.NewRequest(http.MethodGet, "/covers/subdir", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}
