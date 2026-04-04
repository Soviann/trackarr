package httputil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ParseIDParam extracts an int64 URL parameter by name.
func ParseIDParam(r *http.Request, key string) (int64, error) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return id, nil
}

// ParseQueryInt extracts an integer query parameter with a default value.
func ParseQueryInt(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return v
}

// ReadJSON decodes the request body (limited to maxBytes) into v.
func ReadJSON(r *http.Request, v interface{}, maxBytes int64) error {
	return json.NewDecoder(io.LimitReader(r.Body, maxBytes)).Decode(v)
}
