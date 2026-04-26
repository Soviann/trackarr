package cmd

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/colorextract"
)

// BackfillAccents walks every title that has a cover on disk, runs the
// histogram extractor and persists the resulting accent_hex. Idempotent:
// titles whose accent_hex is already set are skipped unless --force is passed.
func BackfillAccents(args []string) error {
	fs := flag.NewFlagSet("backfill-accents", flag.ContinueOnError)
	force := fs.Bool("force", false, "re-extract even if accent_hex already set")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}
	dbPath := filepath.Join(dataDir, "plextracker.db")
	writeDB, _, err := database.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer writeDB.Close()

	if err := database.Migrate(writeDB); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	ctx := context.Background()
	titles := repository.NewTitleRepository(writeDB)
	all, err := titles.ListAll()
	if err != nil {
		return fmt.Errorf("list titles: %w", err)
	}

	coversDir := filepath.Join(dataDir, "covers")
	processed, updated := 0, 0
	for _, t := range all {
		if t.CoverURL == nil || *t.CoverURL == "" {
			continue
		}
		if !*force && t.AccentHex != nil && *t.AccentHex != "" {
			continue
		}
		processed++
		path := filepath.Join(coversDir, *t.CoverURL)
		imgBytes, err := os.ReadFile(path)
		if err != nil {
			log.Printf("read %s: %v", path, err)
			continue
		}
		hex, err := colorextract.ExtractAccent(imgBytes)
		if err != nil {
			log.Printf("extract %s: %v", path, err)
			continue
		}
		if hex == "" {
			continue
		}
		if err := writeAccentTx(ctx, writeDB, t.ID, &hex); err != nil {
			log.Printf("persist %d: %v", t.ID, err)
			continue
		}
		updated++
	}
	log.Printf("backfill-accents: processed %d, updated %d", processed, updated)
	return nil
}

// writeAccentTx mirrors CoverService.updateTitle: a single short transaction
// per title row so a partial backfill leaves a consistent DB.
func writeAccentTx(ctx context.Context, db *sql.DB, titleID int64, hex *string) error {
	return database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, titleID, repository.TitleUpdate{AccentHex: hex})
	})
}
