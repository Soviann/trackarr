package cmd

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/service"
)

// SyncJellyfinRatings executes the outbound Jellyfin ratings synchronization CLI command.
func SyncJellyfinRatings(args []string) error {
	return runSyncJellyfinRatings(args, os.Stdout, nil)
}

func runSyncJellyfinRatings(args []string, out io.Writer, overrideDB *sql.DB) error {
	fs := flag.NewFlagSet("sync-jellyfin-ratings", flag.ContinueOnError)
	fs.SetOutput(out)

	defaultURL := os.Getenv("JELLYFIN_URL")
	defaultAPIKey := os.Getenv("JELLYFIN_API_KEY")
	defaultUserID := os.Getenv("JELLYFIN_USER_ID")

	jellyfinURL := fs.String("jellyfin-url", defaultURL, "Jellyfin server base URL (fallback: $JELLYFIN_URL)")
	apiKey := fs.String("api-key", defaultAPIKey, "Jellyfin API key (fallback: $JELLYFIN_API_KEY)")
	userID := fs.String("user-id", defaultUserID, "Jellyfin user ID (fallback: $JELLYFIN_USER_ID)")
	dryRun := fs.Bool("dry-run", false, "Simulate rating synchronization without sending updates to Jellyfin")

	fs.Usage = func() {
		fmt.Fprintf(out, "Usage: trackarr sync-jellyfin-ratings [flags]\n\n")
		fmt.Fprintf(out, "Syncs ratings from Trackarr into Jellyfin library items.\n\n")
		fmt.Fprintf(out, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	urlVal := strings.TrimSpace(*jellyfinURL)
	keyVal := strings.TrimSpace(*apiKey)
	userVal := strings.TrimSpace(*userID)

	if urlVal == "" || keyVal == "" || userVal == "" {
		var missing []string
		if urlVal == "" {
			missing = append(missing, "-jellyfin-url (or $JELLYFIN_URL)")
		}
		if keyVal == "" {
			missing = append(missing, "-api-key (or $JELLYFIN_API_KEY)")
		}
		if userVal == "" {
			missing = append(missing, "-user-id (or $JELLYFIN_USER_ID)")
		}
		fmt.Fprintf(out, "Error: missing required parameter(s): %s\n\n", strings.Join(missing, ", "))
		fs.Usage()
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	var writeDB *sql.DB
	if overrideDB != nil {
		writeDB = overrideDB
	} else {
		dataDir := os.Getenv("DATA_DIR")
		var dbPath string
		if dataDir != "" {
			dbPath = database.ResolveDBPath(dataDir)
		} else {
			for _, dir := range []string{"data", "./data", "/data"} {
				candidate := database.ResolveDBPath(dir)
				if isRegularFile(candidate) {
					dbPath = candidate
					break
				}
			}
			if dbPath == "" {
				dbPath = filepath.Join("data", "trackarr.db")
			}
		}

		if !isRegularFile(dbPath) {
			return fmt.Errorf("database file not found at %s: ensure Trackarr has been initialized", dbPath)
		}

		db, readDB, err := database.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()
		defer readDB.Close()

		if err := database.Migrate(db); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		writeDB = db
	}

	cfg := service.JellyfinSyncConfig{
		BaseURL: urlVal,
		APIKey:  keyVal,
		UserID:  userVal,
		DryRun:  *dryRun,
	}

	logger := slog.Default()
	syncSvc := service.NewJellyfinSyncService(writeDB, cfg, logger)

	fmt.Fprintf(out, "Starting Jellyfin rating synchronization...\n")
	if *dryRun {
		fmt.Fprintf(out, "Mode: DRY RUN (no modifications will be sent to Jellyfin)\n")
	}
	fmt.Fprintf(out, "Jellyfin URL: %s\n", urlVal)
	fmt.Fprintf(out, "User ID:      %s\n\n", userVal)

	ctx := context.Background()
	summary, err := syncSvc.SyncRatings(ctx)
	if err != nil {
		return fmt.Errorf("sync ratings: %w", err)
	}

	printSummaryReport(out, summary, *dryRun)
	return nil
}

func printSummaryReport(w io.Writer, s service.JellyfinSyncSummary, dryRun bool) {
	fmt.Fprintf(w, "\n========================================\n")
	fmt.Fprintf(w, "      Jellyfin Rating Sync Summary      \n")
	fmt.Fprintf(w, "========================================\n")
	if dryRun {
		fmt.Fprintf(w, "  Mode:    DRY RUN (no changes pushed)\n")
	} else {
		fmt.Fprintf(w, "  Mode:    LIVE\n")
	}
	fmt.Fprintf(w, "  Matched: %d\n", s.Matched)
	fmt.Fprintf(w, "  Updated: %d\n", s.Updated)
	fmt.Fprintf(w, "  Skipped: %d\n", s.Skipped)
	fmt.Fprintf(w, "  Missing: %d\n", s.Missing)
	fmt.Fprintf(w, "  Errors:  %d\n", s.Errors)
	fmt.Fprintf(w, "========================================\n")
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
