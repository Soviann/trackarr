package cmd

import (
	"embed"
	"fmt"
	"log"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/router"
)

func Serve(distFS embed.FS) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := cfg.DataDir + "/plextracker.db"
	db, err := database.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	r := router.New(cfg, db, distFS)

	log.Printf("PlexTracker listening on %s", cfg.ListenAddr)
	return http.ListenAndServe(cfg.ListenAddr, r)
}
