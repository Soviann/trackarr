package handler

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/version"
)

// SPAHandler serves the embedded Preact SPA.
// For any path not starting with /api, it serves static files or falls back to index.html.
func SPAHandler(distFS embed.FS) http.Handler {
	sub, _ := fs.Sub(distFS, "frontend/dist")
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try serving the file directly
		if f, err := sub.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"version": version.Version,
	})
}

func PublicConfig(googleClientID, vapidPublicKey string, devLogin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// dev_login is only emitted when enabled — prod responses must not even
		// hint that the dev login flow exists. The frontend treats a missing
		// key as falsy (Login.tsx:33), so omitting is safe.
		body := map[string]any{
			"google_client_id": googleClientID,
			"vapid_public_key": vapidPublicKey,
		}
		if devLogin {
			body["dev_login"] = true
		}
		httputil.WriteJSON(w, http.StatusOK, body)
	}
}
