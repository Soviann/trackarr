package service

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

type TrackarrBackup struct {
	Version           string                              `json:"version"`
	ExportedAt        string                              `json:"exported_at"`
	TotalCount        int                                 `json:"total_count"`
	Titles            []model.Title                       `json:"titles"`
	SeasonExternalIDs []repository.BackupSeasonExternalID `json:"season_external_ids,omitempty"`
	WatchEvents       []repository.BackupWatchEvent       `json:"watch_events,omitempty"`
	TitleRelations    []model.TitleRelation               `json:"title_relations,omitempty"`
	WrappedSnapshots  []repository.BackupWrappedSnapshot  `json:"wrapped_snapshots,omitempty"`
}

type BackupSummaryResult struct {
	DryRun  bool   `json:"dry_run"`
	Created int    `json:"created"`
	Skipped int    `json:"skipped"`
	Errors  int    `json:"errors"`
	Total   int    `json:"total"`
	Message string `json:"message,omitempty"`
}

type BackupService struct {
	db        *sql.DB
	titleRepo *repository.TitleRepository
	importer  *SimklImporter
}

func NewBackupService(
	db *sql.DB,
	titleRepo *repository.TitleRepository,
	seasonRepo *repository.SeasonRepository,
	episodeRepo *repository.EpisodeRepository,
	eventRepo *repository.WatchEventRepository,
	taskRepo *repository.TaskRepository,
) *BackupService {
	importer := NewSimklImporter(
		db,
		titleRepo,
		seasonRepo,
		episodeRepo,
		eventRepo,
		WithTaskRepository(taskRepo),
	)
	return &BackupService{
		db:        db,
		titleRepo: titleRepo,
		importer:  importer,
	}
}

// ExportJSON returns a full lossless Trackarr JSON v2 backup of all titles, seasons, episodes,
// external mappings, watch events, title relations, and wrapped snapshots.
func (s *BackupService) ExportJSON(ctx context.Context) ([]byte, error) {
	titles, err := s.titleRepo.ListAll()
	if err != nil {
		return nil, fmt.Errorf("list all titles for json export: %w", err)
	}

	br := repository.NewBackupRepository(s.db)
	seasonExtIDs, err := br.ListAllSeasonExternalIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list season external ids for json export: %w", err)
	}
	watchEvents, err := br.ListAllWatchEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list watch events for json export: %w", err)
	}
	relations, err := br.ListAllTitleRelations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list title relations for json export: %w", err)
	}
	wrapped, err := br.ListAllWrappedSnapshots(ctx)
	if err != nil {
		return nil, fmt.Errorf("list wrapped snapshots for json export: %w", err)
	}

	backup := TrackarrBackup{
		Version:           "2.0",
		ExportedAt:        time.Now().UTC().Format(time.RFC3339),
		TotalCount:        len(titles),
		Titles:            titles,
		SeasonExternalIDs: seasonExtIDs,
		WatchEvents:       watchEvents,
		TitleRelations:    relations,
		WrappedSnapshots:  wrapped,
	}

	return json.MarshalIndent(backup, "", "  ")
}

// ExportSQLiteToFile writes a compacted, consistent SQLite snapshot to destPath using VACUUM INTO.
func (s *BackupService) ExportSQLiteToFile(ctx context.Context, destPath string) error {
	return repository.NewBackupRepository(s.db).VacuumInto(ctx, destPath)
}

// WriteAutomatedBackups writes daily gzipped SQLite (.db.gz) and JSON v2 (.json.gz) backups
// into backupDir and removes rotated archives older than retentionDays.
func (s *BackupService) WriteAutomatedBackups(ctx context.Context, backupDir string, retentionDays int) error {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}
	dateStamp := time.Now().Format("2006-01-02")

	// 1. SQLite snapshot (.db.gz)
	tmpDB := filepath.Join(backupDir, fmt.Sprintf(".tmp-%s-%d.db", dateStamp, time.Now().UnixNano()))
	defer func() { _ = os.Remove(tmpDB) }()
	if err := s.ExportSQLiteToFile(ctx, tmpDB); err != nil {
		return err
	}
	dbDest := filepath.Join(backupDir, fmt.Sprintf("trackarr-%s.db.gz", dateStamp))
	if err := gzipFile(tmpDB, dbDest); err != nil {
		return fmt.Errorf("compress sqlite backup: %w", err)
	}

	// 2. Lossless JSON v2 (.json.gz)
	jsonBytes, err := s.ExportJSON(ctx)
	if err != nil {
		return err
	}
	jsonDest := filepath.Join(backupDir, fmt.Sprintf("trackarr-%s.json.gz", dateStamp))
	if err := writeGzipBytes(jsonDest, jsonBytes); err != nil {
		return fmt.Errorf("compress json backup: %w", err)
	}

	// 3. Rotation
	if retentionDays > 0 {
		cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
		entries, err := os.ReadDir(backupDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				if !strings.HasPrefix(name, "trackarr-") || (!strings.HasSuffix(name, ".db.gz") && !strings.HasSuffix(name, ".json.gz")) {
					continue
				}
				info, infoErr := entry.Info()
				if infoErr == nil && info.ModTime().Before(cutoff) {
					_ = os.Remove(filepath.Join(backupDir, name))
				}
			}
		}
	}
	return nil
}

func gzipFile(srcPath, destPath string) error {
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()

	tmpDest := destPath + ".tmp"
	out, err := os.OpenFile(tmpDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		_ = out.Close()
		_ = os.Remove(tmpDest)
		return err
	}
	if err := gz.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpDest)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpDest)
		return err
	}
	return os.Rename(tmpDest, destPath)
}

func writeGzipBytes(destPath string, data []byte) error {
	tmpDest := destPath + ".tmp"
	out, err := os.OpenFile(tmpDest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(out)
	if _, err := gz.Write(data); err != nil {
		_ = gz.Close()
		_ = out.Close()
		_ = os.Remove(tmpDest)
		return err
	}
	if err := gz.Close(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpDest)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpDest)
		return err
	}
	return os.Rename(tmpDest, destPath)
}

// ExportCSV returns a CSV table containing library titles with summary statistics.
func (s *BackupService) ExportCSV(ctx context.Context) ([]byte, error) {
	titles, err := s.titleRepo.ListAll()
	if err != nil {
		return nil, fmt.Errorf("list all titles for csv export: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	headers := []string{
		"Title", "Type", "IsAnime", "Year", "Status",
		"MyRating", "TMDBRating", "WatchedEpisodes", "TotalEpisodes",
		"LastWatched", "IMDBID", "TMDBID", "TVDBID", "AniListID", "SimklID", "Genres",
	}
	if err := w.Write(headers); err != nil {
		return nil, fmt.Errorf("write csv headers: %w", err)
	}

	for _, t := range titles {
		titleName := t.PrimaryName()

		watchedEps := 0
		totalEps := 0
		for _, season := range t.Seasons {
			for _, ep := range season.Episodes {
				totalEps++
				if ep.Watched {
					watchedEps++
				}
			}
		}

		myRatingStr := ""
		if t.MyRating != nil {
			myRatingStr = strconv.Itoa(*t.MyRating)
		}

		tmdbRatingStr := ""
		if t.TMDBRating != nil {
			tmdbRatingStr = fmt.Sprintf("%.1f", *t.TMDBRating)
		}

		lastWatchedStr := ""
		if t.LastWatchedAt != nil {
			lastWatchedStr = t.LastWatchedAt.Format("2006-01-02")
		}

		imdbStr := ""
		if t.IMDBID != nil {
			imdbStr = *t.IMDBID
		}

		tmdbStr := ""
		if t.TMDBID != nil {
			tmdbStr = strconv.FormatInt(*t.TMDBID, 10)
		}

		tvdbStr := ""
		if t.TVDBID != nil {
			tvdbStr = strconv.FormatInt(*t.TVDBID, 10)
		}

		anilistStr := ""
		if t.AniListID != nil {
			anilistStr = strconv.FormatInt(*t.AniListID, 10)
		}

		simklStr := ""
		if t.SimklID != nil {
			simklStr = strconv.FormatInt(*t.SimklID, 10)
		}

		record := []string{
			titleName,
			string(t.Type),
			strconv.FormatBool(t.IsAnime),
			strconv.Itoa(t.Year),
			string(t.Status),
			myRatingStr,
			tmdbRatingStr,
			strconv.Itoa(watchedEps),
			strconv.Itoa(totalEps),
			lastWatchedStr,
			imdbStr,
			tmdbStr,
			tvdbStr,
			anilistStr,
			simklStr,
			strings.Join(t.Genres, "; "),
		}

		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("write csv record: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ExportTrakt returns a JSON payload formatted for Trakt.tv / Simkl sync.
func (s *BackupService) ExportTrakt(ctx context.Context) ([]byte, error) {
	titles, err := s.titleRepo.ListAll()
	if err != nil {
		return nil, fmt.Errorf("list all titles for trakt export: %w", err)
	}

	backup := SimklBackup{
		Movies: []SimklItem{},
		Shows:  []SimklItem{},
		Anime:  []SimklItem{},
	}

	for _, t := range titles {
		media := &SimklMedia{
			Title: t.PrimaryName(),
			Year:  t.Year,
			IDs: SimklIDs{
				IMDB:    safeDerefStr(t.IMDBID),
				TMDB:    flexInt64(safeDerefInt64(t.TMDBID)),
				AniList: flexInt64(safeDerefInt64(t.AniListID)),
				TVDB:    flexInt64(safeDerefInt64(t.TVDBID)),
				Simkl:   flexInt64(safeDerefInt64(t.SimklID)),
				Slug:    safeDerefStr(t.SimklSlug),
			},
		}

		item := SimklItem{
			Status:     mapModelStatusToSimkl(t.Status),
			UserRating: t.MyRating,
			Seasons:    []SimklSeason{},
		}

		if t.LastWatchedAt != nil {
			item.LastWatchedAt = t.LastWatchedAt.Format(time.RFC3339)
		}

		if t.Type == model.TitleTypeMovie {
			item.Movie = media
			if t.IsAnime {
				item.AnimeType = "movie"
				backup.Anime = append(backup.Anime, item)
			} else {
				backup.Movies = append(backup.Movies, item)
			}
		} else {
			item.Show = media
			for _, season := range t.Seasons {
				simklSeason := SimklSeason{
					Number:   season.SeasonNumber,
					Episodes: []SimklEpisode{},
				}
				for _, ep := range season.Episodes {
					if ep.Watched {
						simklEp := SimklEpisode{
							Number: ep.Episode,
						}
						if ep.LastWatchedAt != nil {
							simklEp.WatchedAt = ep.LastWatchedAt.Format(time.RFC3339)
						} else if ep.FirstWatchedAt != nil {
							simklEp.WatchedAt = ep.FirstWatchedAt.Format(time.RFC3339)
						}
						simklSeason.Episodes = append(simklSeason.Episodes, simklEp)
					}
				}
				if len(simklSeason.Episodes) > 0 {
					item.Seasons = append(item.Seasons, simklSeason)
				}
			}

			if t.IsAnime {
				item.AnimeType = "tv"
				backup.Anime = append(backup.Anime, item)
			} else {
				backup.Shows = append(backup.Shows, item)
			}
		}
	}

	return json.MarshalIndent(backup, "", "  ")
}

// Import processes a backup payload from reader (ZIP, GZIP, JSON, or CSV).
func (s *BackupService) Import(r io.Reader, filename string, dryRun bool) (*BackupSummaryResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read import data: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	isZip := ext == ".zip" || (len(data) >= 4 && bytes.HasPrefix(data, []byte("PK\x03\x04")))
	isGzip := ext == ".gz" || (len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b)

	if isZip {
		extractedData, extractedName, err := extractFromZipBytes(data)
		if err != nil {
			return nil, fmt.Errorf("extract zip archive: %w", err)
		}
		data = extractedData
		ext = strings.ToLower(filepath.Ext(extractedName))
	} else if isGzip {
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("open gzip backup: %w", err)
		}
		unpacked, err := io.ReadAll(gr)
		_ = gr.Close()
		if err != nil {
			return nil, fmt.Errorf("read gzip backup: %w", err)
		}
		data = unpacked
		if strings.HasSuffix(strings.ToLower(filename), ".json.gz") {
			ext = ".json"
		}
	}

	// Native lossless Trackarr JSON import (v1 or v2)
	if ext == ".json" || ext == "" {
		var trackarr TrackarrBackup
		if err := json.Unmarshal(data, &trackarr); err == nil && trackarr.Version != "" && len(trackarr.Titles) > 0 {
			return s.importTrackarrV2(context.Background(), &trackarr, dryRun)
		}
	}

	backup, err := s.parseToSimklBackup(data, ext)
	if err != nil {
		return nil, fmt.Errorf("parse backup data: %w", err)
	}

	totalItems := len(backup.Movies) + len(backup.Shows) + len(backup.Anime)
	if totalItems == 0 {
		return &BackupSummaryResult{
			DryRun:  dryRun,
			Created: 0,
			Skipped: 0,
			Errors:  0,
			Total:   0,
			Message: "No titles found in uploaded file",
		}, nil
	}

	res, err := s.importer.Import(backup, dryRun)
	if err != nil {
		return nil, fmt.Errorf("execute import: %w", err)
	}

	msg := fmt.Sprintf("%d titles ready to import, %d already in library", res.Created, res.Skipped)
	if !dryRun {
		msg = fmt.Sprintf("Successfully imported %d titles (%d skipped)", res.Created, res.Skipped)
	}

	return &BackupSummaryResult{
		DryRun:  dryRun,
		Created: res.Created,
		Skipped: res.Skipped,
		Errors:  res.Errors,
		Total:   totalItems,
		Message: msg,
	}, nil
}

func (s *BackupService) parseToSimklBackup(data []byte, ext string) (*SimklBackup, error) {
	// 1. Try CSV
	if ext == ".csv" || (bytes.Contains(data, []byte(",")) && !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) && !bytes.HasPrefix(bytes.TrimSpace(data), []byte("["))) {
		return parseCSVToBackup(data)
	}

	// 2. Try Trackarr JSON format (has "titles" array)
	var trackarr TrackarrBackup
	if err := json.Unmarshal(data, &trackarr); err == nil && len(trackarr.Titles) > 0 {
		return convertTrackarrToSimklBackup(trackarr.Titles), nil
	}

	// 3. Try Simkl/Trakt JSON format (has "movies", "shows", or "anime")
	var simkl SimklBackup
	if err := json.Unmarshal(data, &simkl); err == nil && (len(simkl.Movies) > 0 || len(simkl.Shows) > 0 || len(simkl.Anime) > 0) {
		return &simkl, nil
	}

	// 4. Try raw array of titles JSON
	var rawTitles []model.Title
	if err := json.Unmarshal(data, &rawTitles); err == nil && len(rawTitles) > 0 {
		return convertTrackarrToSimklBackup(rawTitles), nil
	}

	return nil, fmt.Errorf("unrecognized file format: expected JSON, CSV, or ZIP archive")
}

func extractFromZipBytes(data []byte) ([]byte, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, "", fmt.Errorf("open zip reader: %w", err)
	}

	// Look for SimklBackup.json first, then any .json, then any .csv
	var jsonFile *zip.File
	var csvFile *zip.File

	for _, f := range reader.File {
		name := filepath.Base(f.Name)
		if strings.HasPrefix(name, ".") || f.FileInfo().IsDir() {
			continue
		}
		if name == "SimklBackup.json" {
			return readZipFile(f)
		}
		if strings.HasSuffix(strings.ToLower(name), ".json") && jsonFile == nil {
			jsonFile = f
		}
		if strings.HasSuffix(strings.ToLower(name), ".csv") && csvFile == nil {
			csvFile = f
		}
	}

	if jsonFile != nil {
		return readZipFile(jsonFile)
	}
	if csvFile != nil {
		return readZipFile(csvFile)
	}

	return nil, "", fmt.Errorf("no valid .json or .csv backup file found inside zip archive")
}

const maxBackupEntrySize = 50 << 20 // 50 MB

func readZipFile(f *zip.File) ([]byte, string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, "", fmt.Errorf("open file %s in zip: %w", f.Name, err)
	}
	defer rc.Close()

	content, err := io.ReadAll(io.LimitReader(rc, maxBackupEntrySize+1))
	if err != nil {
		return nil, "", fmt.Errorf("read file %s in zip: %w", f.Name, err)
	}
	if len(content) > maxBackupEntrySize {
		return nil, "", fmt.Errorf("file %s in zip exceeds maximum allowed size (%d MB)", f.Name, maxBackupEntrySize>>20)
	}
	return content, f.Name, nil
}

func convertTrackarrToSimklBackup(titles []model.Title) *SimklBackup {
	backup := &SimklBackup{
		Movies: []SimklItem{},
		Shows:  []SimklItem{},
		Anime:  []SimklItem{},
	}

	for _, t := range titles {
		media := &SimklMedia{
			Title: t.PrimaryName(),
			Year:  t.Year,
			IDs: SimklIDs{
				IMDB:    safeDerefStr(t.IMDBID),
				TMDB:    flexInt64(safeDerefInt64(t.TMDBID)),
				AniList: flexInt64(safeDerefInt64(t.AniListID)),
				TVDB:    flexInt64(safeDerefInt64(t.TVDBID)),
				Simkl:   flexInt64(safeDerefInt64(t.SimklID)),
				Slug:    safeDerefStr(t.SimklSlug),
			},
		}

		item := SimklItem{
			Status:     mapModelStatusToSimkl(t.Status),
			UserRating: t.MyRating,
			Seasons:    []SimklSeason{},
		}

		if t.LastWatchedAt != nil {
			item.LastWatchedAt = t.LastWatchedAt.Format(time.RFC3339)
		}

		if t.Type == model.TitleTypeMovie {
			item.Movie = media
			if t.IsAnime {
				item.AnimeType = "movie"
				backup.Anime = append(backup.Anime, item)
			} else {
				backup.Movies = append(backup.Movies, item)
			}
		} else {
			item.Show = media
			for _, season := range t.Seasons {
				simklSeason := SimklSeason{
					Number:   season.SeasonNumber,
					Episodes: []SimklEpisode{},
				}
				for _, ep := range season.Episodes {
					if ep.Watched {
						simklEp := SimklEpisode{
							Number: ep.Episode,
						}
						if ep.LastWatchedAt != nil {
							simklEp.WatchedAt = ep.LastWatchedAt.Format(time.RFC3339)
						} else if ep.FirstWatchedAt != nil {
							simklEp.WatchedAt = ep.FirstWatchedAt.Format(time.RFC3339)
						}
						simklSeason.Episodes = append(simklSeason.Episodes, simklEp)
					}
				}
				item.Seasons = append(item.Seasons, simklSeason)
			}

			if t.IsAnime {
				item.AnimeType = "tv"
				backup.Anime = append(backup.Anime, item)
			} else {
				backup.Shows = append(backup.Shows, item)
			}
		}
	}

	return backup
}

func parseCSVToBackup(data []byte) (*SimklBackup, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.TrimLeadingSpace = true
	r.LazyQuotes = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("csv file is empty or has no data rows")
	}

	headerMap := make(map[string]int)
	for i, col := range rows[0] {
		headerMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	getCol := func(row []string, names ...string) string {
		for _, name := range names {
			if idx, ok := headerMap[strings.ToLower(name)]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
		}
		return ""
	}

	backup := &SimklBackup{
		Movies: []SimklItem{},
		Shows:  []SimklItem{},
		Anime:  []SimklItem{},
	}

	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}
		titleName := getCol(row, "title", "name", "titre")
		if titleName == "" {
			continue
		}

		typeStr := strings.ToLower(getCol(row, "type", "media_type", "type_titre"))
		isMovie := typeStr == "movie" || typeStr == "film"
		isAnime := strings.ToLower(getCol(row, "isanime", "is_anime", "anime")) == "true"

		year, _ := strconv.Atoi(getCol(row, "year", "annee", "release_year"))
		imdbID := getCol(row, "imdbid", "imdb_id", "imdb")
		tmdbID, _ := strconv.ParseInt(getCol(row, "tmdbid", "tmdb_id", "tmdb"), 10, 64)
		tvdbID, _ := strconv.ParseInt(getCol(row, "tvdbid", "tvdb_id", "tvdb"), 10, 64)
		anilistID, _ := strconv.ParseInt(getCol(row, "anilistid", "anilist_id", "anilist"), 10, 64)
		simklID, _ := strconv.ParseInt(getCol(row, "simklid", "simkl_id", "simkl"), 10, 64)

		status := getCol(row, "status", "statut")
		ratingStr := getCol(row, "myrating", "my_rating", "rating", "note")
		var rating *int
		if r, err := strconv.Atoi(ratingStr); err == nil && r > 0 {
			rating = &r
		}

		lastWatched := getCol(row, "lastwatched", "last_watched", "watched_at")

		media := &SimklMedia{
			Title: titleName,
			Year:  year,
			IDs: SimklIDs{
				IMDB:    imdbID,
				TMDB:    flexInt64(tmdbID),
				AniList: flexInt64(anilistID),
				TVDB:    flexInt64(tvdbID),
				Simkl:   flexInt64(simklID),
			},
		}

		item := SimklItem{
			Status:        status,
			UserRating:    rating,
			LastWatchedAt: lastWatched,
			Seasons:       []SimklSeason{},
		}

		if isMovie {
			item.Movie = media
			if isAnime {
				item.AnimeType = "movie"
				backup.Anime = append(backup.Anime, item)
			} else {
				backup.Movies = append(backup.Movies, item)
			}
		} else {
			item.Show = media
			if isAnime {
				item.AnimeType = "tv"
				backup.Anime = append(backup.Anime, item)
			} else {
				backup.Shows = append(backup.Shows, item)
			}
		}
	}

	return backup, nil
}

func mapModelStatusToSimkl(status model.TitleStatus) string {
	switch status {
	case model.TitleStatusCompleted:
		return "completed"
	case model.TitleStatusWatching:
		return "watching"
	case model.TitleStatusPlanToWatch:
		return "plantowatch"
	case model.TitleStatusDropped:
		return "notinteresting"
	default:
		return "watching"
	}
}

func safeDerefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func safeDerefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

type seasonKey struct {
	titleID int64
	season  int
}

type episodeKey struct {
	titleID int64
	season  int
	episode int
}

func findExistingBackupTitle(titles *repository.TitleRepository, t *model.Title) (*model.Title, error) {
	if t.IMDBID != nil || t.TMDBID != nil || t.AniListID != nil || t.TVDBID != nil || t.ExternalSourceID != nil {
		if existing, err := titles.FindByExternalID(t.IMDBID, t.TMDBID, t.ExternalSourceID, t.AniListID, t.TVDBID, &t.Type); err == nil && existing != nil {
			return existing, nil
		}
	}
	if primary := t.PrimaryName(); primary != "" {
		if existing, err := titles.FindByNameAndType(primary, t.Type, t.Year); err == nil && existing != nil {
			return existing, nil
		}
	}
	return nil, nil
}

func (s *BackupService) importTrackarrV2(ctx context.Context, backup *TrackarrBackup, dryRun bool) (*BackupSummaryResult, error) {
	total := len(backup.Titles)
	if total == 0 {
		return &BackupSummaryResult{DryRun: dryRun, Message: "No titles found in uploaded file"}, nil
	}

	if dryRun {
		created, skipped := 0, 0
		for i := range backup.Titles {
			existing, _ := findExistingBackupTitle(s.titleRepo, &backup.Titles[i])
			if existing != nil {
				skipped++
			} else {
				created++
			}
		}
		return &BackupSummaryResult{
			DryRun:  true,
			Created: created,
			Skipped: skipped,
			Total:   total,
			Message: fmt.Sprintf("%d titles ready to import, %d already in library", created, skipped),
		}, nil
	}

	var created, skipped int
	err := database.WithTxContext(ctx, s.db, func(tx *sql.Tx) error {
		tr := repository.NewTitleRepository(tx)
		tw := repository.NewTitleWriter(tx)
		gw := repository.NewGenreWriter(tx)
		sw := repository.NewSeasonWriter(tx)
		ew := repository.NewEpisodeWriter(tx)
		rw := repository.NewTitleRelationWriter(tx)
		bw := repository.NewBackupWriter(tx)

		titleIDMap := make(map[int64]int64, len(backup.Titles))
		seasonIDMap := make(map[seasonKey]int64)
		episodeIDMap := make(map[episodeKey]int64)

		for i := range backup.Titles {
			t := backup.Titles[i]
			existing, _ := findExistingBackupTitle(tr, &t)
			if existing != nil {
				titleIDMap[t.ID] = existing.ID
				skipped++
				continue
			}

			newID, err := tw.Create(ctx, &t, t.Names)
			if err != nil {
				return fmt.Errorf("create title %q: %w", t.PrimaryName(), err)
			}
			if err := bw.UpdateTitleExtraFields(ctx, newID, t); err != nil {
				return fmt.Errorf("update extra fields for title %d: %w", newID, err)
			}
			if len(t.Genres) > 0 {
				if err := gw.ReplaceForTitle(ctx, newID, t.Genres); err != nil {
					return fmt.Errorf("restore genres for title %d: %w", newID, err)
				}
			}

			for _, season := range t.Seasons {
				newSeason, err := sw.GetOrCreate(ctx, newID, season.SeasonNumber)
				if err != nil {
					return fmt.Errorf("restore season %d for title %d: %w", season.SeasonNumber, newID, err)
				}
				if season.TotalEpisodes != nil {
					if err := sw.UpdateTotalEpisodes(ctx, newSeason.ID, *season.TotalEpisodes); err != nil {
						return fmt.Errorf("restore season total episodes: %w", err)
					}
				}
				seasonIDMap[seasonKey{titleID: newID, season: season.SeasonNumber}] = newSeason.ID

				for _, ep := range season.Episodes {
					newEp, err := ew.GetOrCreate(ctx, newSeason.ID, ep.Episode)
					if err != nil {
						return fmt.Errorf("restore episode s%de%d: %w", season.SeasonNumber, ep.Episode, err)
					}
					if err := bw.UpdateEpisodeState(ctx, newEp.ID, ep); err != nil {
						return fmt.Errorf("restore episode state s%de%d: %w", season.SeasonNumber, ep.Episode, err)
					}
					episodeIDMap[episodeKey{titleID: newID, season: season.SeasonNumber, episode: ep.Episode}] = newEp.ID
				}
			}

			titleIDMap[t.ID] = newID
			created++
		}

		for _, sei := range backup.SeasonExternalIDs {
			newTitleID, ok := titleIDMap[sei.TitleID]
			if !ok {
				continue
			}
			if sid, ok := seasonIDMap[seasonKey{titleID: newTitleID, season: sei.SeasonNumber}]; ok {
				if err := bw.RestoreSeasonExternalID(ctx, sid, sei); err != nil {
					return fmt.Errorf("restore season external id: %w", err)
				}
			}
		}

		for _, we := range backup.WatchEvents {
			newTitleID, ok := titleIDMap[we.TitleID]
			if !ok {
				continue
			}
			var epID *int64
			if we.SeasonNumber != nil && we.EpisodeNumber != nil {
				if eid, ok := episodeIDMap[episodeKey{titleID: newTitleID, season: *we.SeasonNumber, episode: *we.EpisodeNumber}]; ok {
					epID = &eid
				}
			}
			if err := bw.RestoreWatchEvent(ctx, newTitleID, epID, we); err != nil {
				return fmt.Errorf("restore watch event: %w", err)
			}
		}

		relsByTitle := make(map[int64][]model.TitleRelation)
		for _, rel := range backup.TitleRelations {
			newTitleID, ok := titleIDMap[rel.TitleID]
			if !ok {
				continue
			}
			rel.TitleID = newTitleID
			if rel.SeasonNumber != nil {
				if sid, ok := seasonIDMap[seasonKey{titleID: newTitleID, season: *rel.SeasonNumber}]; ok {
					rel.SeasonID = &sid
				} else {
					rel.SeasonID = nil
				}
			} else {
				rel.SeasonID = nil
			}
			if rel.MatchedTitleID != nil {
				if mapped, ok := titleIDMap[*rel.MatchedTitleID]; ok {
					rel.MatchedTitleID = &mapped
				} else {
					rel.MatchedTitleID = nil
				}
			}
			relsByTitle[newTitleID] = append(relsByTitle[newTitleID], rel)
		}
		for tid, rels := range relsByTitle {
			if err := rw.UpsertBatch(ctx, tid, rels); err != nil {
				return fmt.Errorf("restore title relations for %d: %w", tid, err)
			}
		}

		for _, ws := range backup.WrappedSnapshots {
			if err := bw.RestoreWrappedSnapshot(ctx, ws); err != nil {
				return fmt.Errorf("restore wrapped snapshot %d: %w", ws.Year, err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("execute v2 import: %w", err)
	}

	return &BackupSummaryResult{
		DryRun:  false,
		Created: created,
		Skipped: skipped,
		Total:   total,
		Message: fmt.Sprintf("Successfully imported %d titles (%d skipped)", created, skipped),
	}, nil
}
