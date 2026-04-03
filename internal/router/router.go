package router

import (
	"database/sql"
	"embed"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/handler"
)

func New(cfg *config.Config, db *sql.DB, distFS embed.FS) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handler.Health)
	})

	// SPA catch-all
	r.Handle("/*", handler.SPAHandler(distFS))

	return r
}
