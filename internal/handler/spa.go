package handler

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
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
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func PublicConfig(googleClientID, vapidPublicKey string, devLogin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, map[string]interface{}{
			"google_client_id": googleClientID,
			"vapid_public_key": vapidPublicKey,
			"dev_login":        devLogin,
		})
	}
}
