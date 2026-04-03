package cmd

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
)

func Import(args []string) error {
	dryRun := false
	var path string

	for _, arg := range args {
		if arg == "--dry-run" {
			dryRun = true
		} else {
			path = arg
		}
	}

	if path == "" {
		return fmt.Errorf("usage: plextracker import [--dry-run] <backup-file>")
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	dbPath := dataDir + "/plextracker.db"
	db, err := database.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Read backup
	data, err := readBackup(path)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	var backup service.SimklBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return fmt.Errorf("parse backup: %w", err)
	}

	importer := service.NewSimklImporter(
		repository.NewTitleRepository(db),
		repository.NewSeasonRepository(db),
		repository.NewEpisodeRepository(db),
		repository.NewWatchEventRepository(db),
	)

	if dryRun {
		fmt.Println("=== DRY RUN ===")
	}

	result, err := importer.Import(&backup, dryRun)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	fmt.Printf("Import complete: %d created, %d skipped, %d errors\n", result.Created, result.Skipped, result.Errors)
	return nil
}

func readBackup(path string) ([]byte, error) {
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		return readFromZip(path)
	}
	return os.ReadFile(path)
}

func readFromZip(path string) ([]byte, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == "SimklBackup.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open file in zip: %w", err)
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}

	return nil, fmt.Errorf("SimklBackup.json not found in zip")
}
