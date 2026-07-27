package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service/colorextract"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
)

// CoverService owns cover image lifecycle: fetching from external sources,
// bulk download for titles missing a cover, and disk cleanup of orphan files.
// BackgroundService.refresh delegates to this service so metadata refresh and
// cover concerns stay decoupled.
type CoverService struct {
	writeDB *sql.DB
	titles  *repository.TitleRepository
	tmdb    *matching.TMDBClient
	anilist *matching.AniListClient
	dataDir string
	limiter *APILimiter
}

func NewCoverService(
	writeDB *sql.DB,
	titles *repository.TitleRepository,
	tmdb *matching.TMDBClient,
	anilist *matching.AniListClient,
	dataDir string,
) *CoverService {
	return &CoverService{
		writeDB: writeDB,
		titles:  titles,
		tmdb:    tmdb,
		anilist: anilist,
		dataDir: dataDir,
		limiter: NewAPILimiter(2, 1),
	}
}

// Dir returns the absolute path to the covers directory.
func (c *CoverService) Dir() string {
	return filepath.Join(c.dataDir, "covers")
}

// HasCoverFile returns true if coverURL points to an existing file on disk in the covers directory.
func (c *CoverService) HasCoverFile(coverURL string) bool {
	if c == nil || coverURL == "" {
		return false
	}
	filename := filepath.Base(coverURL)
	if filename == "" || filename == "." || filename == "/" {
		return false
	}
	path := filepath.Join(c.Dir(), filename)
	_, err := os.Stat(path)
	return err == nil
}

// SetAPILimiter replaces the default limiter with a shared one so cover fetch
// shares the 2rps budget with BackgroundService and TaskQueueWorker.
func (c *CoverService) SetAPILimiter(limiter *APILimiter) {
	if c == nil || limiter == nil {
		return
	}
	c.limiter = limiter
}

func (c *CoverService) updateTitle(ctx context.Context, id int64, update repository.TitleUpdate) error {
	return database.WithTxContext(ctx, c.writeDB, func(tx *sql.Tx) error {
		return repository.NewTitleWriter(tx).Update(ctx, id, update)
	})
}

// ExtractAndStoreAccent reads the freshly-saved cover from disk, runs the
// histogram extractor, and persists the result on the title row. Best-effort
// by design — any error is logged but never aborts the cover save.
func (c *CoverService) ExtractAndStoreAccent(ctx context.Context, titleID int64, coverFilename string) {
	if c == nil || coverFilename == "" {
		return
	}
	path := filepath.Join(c.Dir(), coverFilename)
	imgBytes, err := os.ReadFile(path)
	if err != nil {
		log.Printf("accent: read cover %s: %v", coverFilename, err)
		return
	}
	hex, err := colorextract.ExtractAccent(imgBytes)
	if err != nil {
		log.Printf("accent: extract %s: %v", coverFilename, err)
		return
	}
	if hex == "" {
		return
	}
	if err := c.updateTitle(ctx, titleID, repository.TitleUpdate{AccentHex: &hex}); err != nil {
		log.Printf("accent: persist %d: %v", titleID, err)
	}
}

// FetchMissingCovers downloads covers for all titles without a cover.
// Tries TMDB first (if TMDB ID available), then falls back to AniList.
func (c *CoverService) FetchMissingCovers(ctx context.Context) int {
	if c == nil {
		return 0
	}

	titles, err := c.titles.ListAllForRefresh(ctx)
	if err != nil {
		log.Printf("covers: list titles: %v", err)
		return 0
	}

	fetched := 0
	for i := range titles {
		if err := ctx.Err(); err != nil {
			log.Printf("covers: fetch cancelled: %v", err)
			return fetched
		}

		title := &titles[i]
		if title.CoverURL != nil && c.HasCoverFile(*title.CoverURL) {
			continue
		}

		// Try TMDB
		if title.TMDBID != nil {
			var posterPath *string
			if title.Type == model.TitleTypeMovie {
				details, err := c.tmdb.GetMovieDetails(ctx, *title.TMDBID)
				if err != nil {
					c.enqueueCoverOnRetryable(ctx, title.ID, *title.TMDBID, title.AniListID, title.Type, err)
				} else {
					posterPath = details.PosterPath
				}
			} else {
				details, err := c.tmdb.GetTVDetails(ctx, *title.TMDBID)
				if err != nil {
					c.enqueueCoverOnRetryable(ctx, title.ID, *title.TMDBID, title.AniListID, title.Type, err)
				} else {
					posterPath = details.PosterPath
				}
			}

			if posterPath != nil && *posterPath != "" {
				coverPath, err := c.tmdb.DownloadCover(ctx, *posterPath, c.Dir())
				if err == nil {
					logTitleUpdate(title.ID, "missing cover", c.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
					c.ExtractAndStoreAccent(ctx, title.ID, coverPath)
					fetched++
					_ = c.limiter.Wait(ctx)
					continue
				}
			}
		}

		// Fallback: AniList
		if title.AniListID != nil && c.DownloadAniListCover(ctx, title) {
			fetched++
		}

		_ = c.limiter.Wait(ctx)
	}

	return fetched
}

// DownloadAniListCover fetches and saves the cover from AniList for a title.
// Returns true if the cover was successfully downloaded and saved.
func (c *CoverService) DownloadAniListCover(ctx context.Context, title *repository.TitleLite) bool {
	if c == nil || c.anilist == nil || title.AniListID == nil {
		return false
	}

	details, err := c.anilist.GetAnimeDetails(ctx, *title.AniListID)
	if err != nil || details.CoverURL == "" {
		return false
	}

	coverPath, err := c.anilist.DownloadCover(ctx, details.CoverURL, c.Dir())
	if err != nil {
		return false
	}

	logTitleUpdate(title.ID, "anilist cover", c.updateTitle(ctx, title.ID, repository.TitleUpdate{CoverURL: &coverPath}))
	c.ExtractAndStoreAccent(ctx, title.ID, coverPath)
	return true
}

func coverDailyPrefixes(day time.Weekday) []rune {
	switch day {
	case time.Sunday:
		return []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i'}
	case time.Monday:
		return []rune{'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r'}
	case time.Tuesday:
		return []rune{'s', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A'}
	case time.Wednesday:
		return []rune{'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J'}
	case time.Thursday:
		return []rune{'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S'}
	case time.Friday:
		return []rune{'T', 'U', 'V', 'W', 'X', 'Y', 'Z', '0', '1'}
	case time.Saturday:
		return []rune{'2', '3', '4', '5', '6', '7', '8', '9', '_', '-'}
	default:
		return nil
	}
}

// CleanupUnusedCovers deletes orphaned cover files sharded by the starting character of the filename.
func (c *CoverService) CleanupUnusedCovers(ctx context.Context, day time.Weekday) {
	if c == nil || c.titles == nil {
		return
	}

	prefixes := coverDailyPrefixes(day)
	if len(prefixes) == 0 {
		return
	}

	prefixMap := make(map[rune]bool)
	for _, p := range prefixes {
		prefixMap[p] = true
	}

	coversDir := c.Dir()
	entries, err := os.ReadDir(coversDir)
	if err != nil {
		log.Printf("covers: read covers dir: %v", err)
		return
	}

	var batch []string
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) == 0 {
			continue
		}

		firstChar := rune(name[0])
		if !prefixMap[firstChar] {
			continue
		}

		batch = append(batch, name)

		if len(batch) >= 100 {
			deleted += c.processCoverBatch(coversDir, batch)
			batch = batch[:0]
			_ = c.limiter.Wait(ctx)
		}
	}

	if len(batch) > 0 {
		deleted += c.processCoverBatch(coversDir, batch)
	}

	if deleted > 0 {
		log.Printf("covers: deleted %d unused covers for %s", deleted, day.String())
	}
}

func (c *CoverService) processCoverBatch(coversDir string, batch []string) int {
	used, err := c.titles.GetUsedCoversInBatch(batch)
	if err != nil {
		log.Printf("covers: get used covers batch: %v", err)
		return 0
	}

	deleted := 0
	for _, name := range batch {
		if !used[name] {
			path := filepath.Join(coversDir, name)
			if err := os.Remove(path); err != nil {
				log.Printf("covers: delete unused cover %s: %v", name, err)
			} else {
				deleted++
			}
		}
	}
	return deleted
}

func (c *CoverService) enqueueCoverOnRetryable(ctx context.Context, titleID, tmdbID int64, anilistID *int64, titleType model.TitleType, err error) {
	if !matching.IsRetryableError(err) {
		return
	}
	p := CoverFetchPayload{TitleID: titleID, TMDBID: tmdbID, TitleType: titleType}
	if anilistID != nil {
		p.AniListID = *anilistID
	}
	payload, marshalErr := json.Marshal(p)
	if marshalErr != nil {
		log.Printf("enqueue cover fetch for title %d: marshal payload: %v", titleID, marshalErr)
		return
	}
	dedupKey := fmt.Sprintf("cover_fetch:%d", titleID)
	if enqErr := database.WithTxContext(ctx, c.writeDB, func(tx *sql.Tx) error {
		_, e := repository.NewTaskWriter(tx).Enqueue(ctx, model.TaskTypeCoverFetch, string(payload), &dedupKey)
		return e
	}); enqErr != nil {
		log.Printf("enqueue cover fetch for title %d: %v", titleID, enqErr)
	}
}
