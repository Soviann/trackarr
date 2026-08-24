package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

type CoverHandler struct {
	coversDir string
}

func NewCoverHandler(dataDir string) *CoverHandler {
	abs, err := filepath.Abs(filepath.Join(dataDir, "covers"))
	if err != nil {
		// dataDir misconfigured at boot is a fatal config error; fall back to
		// the relative path so Serve's containment check still rejects all
		// requests rather than silently serving from cwd.
		abs = filepath.Join(dataDir, "covers")
	}
	return &CoverHandler{coversDir: abs}
}

func (h *CoverHandler) Serve(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	// Resolve against coversDir then verify containment. Cleaner than greping
	// for ".." / "/" / "\" — handles unicode normalization tricks, encoded
	// separators, and OS-specific path quirks the substring check missed.
	resolved, err := filepath.Abs(filepath.Join(h.coversDir, filename))
	if err != nil || (resolved != h.coversDir && !strings.HasPrefix(resolved, h.coversDir+string(filepath.Separator))) {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, resolved)
}
