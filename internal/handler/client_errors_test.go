package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/stretchr/testify/assert"
)

func TestClientErrorHandler_Handle(t *testing.T) {
	h := &handler.ClientErrorHandler{}

	t.Run("valid payload returns 204", func(t *testing.T) {
		body := `{"message":"TypeError: cannot read property","stack":"at App.tsx:42"}`
		r := httptest.NewRequest(http.MethodPost, "/api/client-errors", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		err := h.Handle(w, r)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("empty message returns 400", func(t *testing.T) {
		body := `{"message":""}`
		r := httptest.NewRequest(http.MethodPost, "/api/client-errors", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		err := h.Handle(w, r)
		var apiErr *httputil.APIError
		assert.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Status)
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/client-errors", strings.NewReader("{invalid"))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		err := h.Handle(w, r)
		var apiErr *httputil.APIError
		assert.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Status)
	})

	t.Run("stack is optional", func(t *testing.T) {
		body := `{"message":"render error"}`
		r := httptest.NewRequest(http.MethodPost, "/api/client-errors", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		err := h.Handle(w, r)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
