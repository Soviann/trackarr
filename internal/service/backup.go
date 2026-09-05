package service

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
)

type TrackarrBackup struct {
	Version    string        `json:"version"`
	ExportedAt string        `json:"exported_at"`
	TotalCount int           `json:"total_count"`
	Titles     []model.Title `json:"titles"`
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

// ExportJSON returns a full Trackarr JSON backup of all titles and relations.
func (s *BackupService) ExportJSON(ctx context.Context) ([]byte, error) {
	titles, err := s.titleRepo.ListAll()
	if err != nil {
		return nil, fmt.Errorf("list all titles for json export: %w", err)
	}

	backup := TrackarrBackup{
		Version:    "1.0",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		TotalCount: len(titles),
		Titles:     titles,
	}

	return json.MarshalIndent(backup, "", "  ")
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

// Import processes a backup payload from reader (ZIP, JSON, or CSV).
func (s *BackupService) Import(r io.Reader, filename string, dryRun bool) (*BackupSummaryResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read import data: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	isZip := ext == ".zip" || (len(data) >= 4 && bytes.HasPrefix(data, []byte("PK\x03\x04")))

	if isZip {
		extractedData, extractedName, err := extractFromZipBytes(data)
		if err != nil {
			return nil, fmt.Errorf("extract zip archive: %w", err)
		}
		data = extractedData
		ext = strings.ToLower(filepath.Ext(extractedName))
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

func readZipFile(f *zip.File) ([]byte, string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, "", fmt.Errorf("open file %s in zip: %w", f.Name, err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", fmt.Errorf("read file %s in zip: %w", f.Name, err)
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
