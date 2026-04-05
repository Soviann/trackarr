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
	return &CoverHandler{coversDir: filepath.Join(dataDir, "covers")}
}

func (h *CoverHandler) Serve(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	// Prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(h.coversDir, filename))
}
