package httputil_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapHandler(t *testing.T) {
	// Nominal: API route returning APIError -> JSON response
	{
		h := httputil.WrapHandler(func(w http.ResponseWriter, r *http.Request) error {
			return httputil.BadRequest("invalid field")
		})

		req := httptest.NewRequest("GET", "/api/test", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))

		var body map[string]string
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.Equal(t, "invalid field", body["error"])
	}

	// Boundary: Non-API route returning APIError -> text/plain response
	{
		h := httputil.WrapHandler(func(w http.ResponseWriter, r *http.Request) error {
			return httputil.NotFound("resource missing")
		})

		req := httptest.NewRequest("GET", "/other/path", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "resource missing")
	}

	// Error: Unhandled internal error -> 500 JSON response on /api/
	{
		h := httputil.WrapHandler(func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("unexpected crash")
		})

		req := httptest.NewRequest("GET", "/api/crash", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		var body map[string]string
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.Equal(t, "Internal error", body["error"])
	}
}
