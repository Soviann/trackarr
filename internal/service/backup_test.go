package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type backupTestDeps struct {
	svc       *service.BackupService
	titleRepo *repository.TitleRepository
	db        *sql.DB
}

func setupBackupService(t *testing.T) backupTestDeps {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	seasonRepo := repository.NewSeasonRepository(db)
	episodeRepo := repository.NewEpisodeRepository(db)
	eventRepo := repository.NewWatchEventRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	svc := service.NewBackupService(db, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo)
	return backupTestDeps{svc: svc, titleRepo: titleRepo, db: db}
}

func createTestTitle(t *testing.T, db *sql.DB, title *model.Title, names []model.TitleName) int64 {
	t.Helper()
	var id int64
	err := database.WithTxContext(context.Background(), db, func(tx *sql.Tx) error {
		var err error
		id, err = repository.NewTitleWriter(tx).Create(context.Background(), title, names)
		return err
	})
	require.NoError(t, err)
	return id
}

func TestBackupService_ExportJSON(t *testing.T) {
	deps := setupBackupService(t)
	ctx := context.Background()

	// Seed a title
	now := time.Now().UTC().Truncate(time.Second)
	rating := 9
	tmdbID := int64(12345)
	title := &model.Title{
		Type:          model.TitleTypeMovie,
		Year:          2024,
		Status:        model.TitleStatusCompleted,
		MatchStatus:   model.MatchStatusConfirmed,
		TMDBID:        &tmdbID,
		MyRating:      &rating,
		LastWatchedAt: &now,
	}
	names := []model.TitleName{{Name: "Dune: Part Two", Language: "en", IsPrimary: true}}
	createTestTitle(t, deps.db, title, names)

	data, err := deps.svc.ExportJSON(ctx)
	require.NoError(t, err)

	var backup service.TrackarrBackup
	err = json.Unmarshal(data, &backup)
	require.NoError(t, err)

	assert.Equal(t, "2.0", backup.Version)
	assert.Equal(t, 1, backup.TotalCount)
	require.Len(t, backup.Titles, 1)
	assert.Equal(t, "Dune: Part Two", backup.Titles[0].PrimaryName())
	assert.Equal(t, 2024, backup.Titles[0].Year)
	assert.Equal(t, int64(12345), *backup.Titles[0].TMDBID)
}

func TestBackupService_ExportCSV(t *testing.T) {
	deps := setupBackupService(t)
	ctx := context.Background()

	rating := 8
	title := &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2022,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		MyRating:    &rating,
	}
	names := []model.TitleName{{Name: "Severance", Language: "en", IsPrimary: true}}
	createTestTitle(t, deps.db, title, names)

	data, err := deps.svc.ExportCSV(ctx)
	require.NoError(t, err)

	csvContent := string(data)
	assert.True(t, strings.HasPrefix(csvContent, "Title,Type,IsAnime,Year,Status"))
	assert.Contains(t, csvContent, "Severance,series,false,2022,watching,8")
}

func TestBackupService_ExportTrakt(t *testing.T) {
	deps := setupBackupService(t)
	ctx := context.Background()

	rating := 10
	imdbID := "tt0903747"
	title := &model.Title{
		Type:        model.TitleTypeSeries,
		Year:        2008,
		Status:      model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		IMDBID:      &imdbID,
		MyRating:    &rating,
	}
	names := []model.TitleName{{Name: "Breaking Bad", Language: "en", IsPrimary: true}}
	createTestTitle(t, deps.db, title, names)

	data, err := deps.svc.ExportTrakt(ctx)
	require.NoError(t, err)

	var trakt service.SimklBackup
	err = json.Unmarshal(data, &trakt)
	require.NoError(t, err)

	require.Len(t, trakt.Shows, 1)
	assert.Equal(t, "Breaking Bad", trakt.Shows[0].Show.Title)
	assert.Equal(t, "tt0903747", trakt.Shows[0].Show.IDs.IMDB)
	assert.Equal(t, 10, *trakt.Shows[0].UserRating)
}

func TestBackupService_Import_JSON_DryRun_And_Live(t *testing.T) {
	deps := setupBackupService(t)

	simklJSON := `{
		"movies": [
			{
				"status": "completed",
				"user_rating": 9,
				"movie": {
					"title": "Inception",
					"year": 2010,
					"ids": { "tmdb": 27205, "imdb": "tt1375666" }
				}
			}
		],
		"shows": [
			{
				"status": "watching",
				"show": {
					"title": "Dark",
					"year": 2017,
					"ids": { "tmdb": 70523 }
				}
			}
		]
	}`

	// 1. Dry run
	res, err := deps.svc.Import(strings.NewReader(simklJSON), "backup.json", true)
	require.NoError(t, err)
	assert.True(t, res.DryRun)
	assert.Equal(t, 2, res.Total)
	assert.Equal(t, 2, res.Created)
	assert.Equal(t, 0, res.Skipped)

	// Ensure DB is still empty after dry-run
	titles, err := deps.titleRepo.ListAll()
	require.NoError(t, err)
	assert.Empty(t, titles)

	// 2. Live import
	resLive, err := deps.svc.Import(strings.NewReader(simklJSON), "backup.json", false)
	require.NoError(t, err)
	assert.False(t, resLive.DryRun)
	assert.Equal(t, 2, resLive.Total)
	assert.Equal(t, 2, resLive.Created)

	titlesAfter, err := deps.titleRepo.ListAll()
	require.NoError(t, err)
	assert.Len(t, titlesAfter, 2)

	// 3. Second import should skip existing items
	resDuplicate, err := deps.svc.Import(strings.NewReader(simklJSON), "backup.json", false)
	require.NoError(t, err)
	assert.Equal(t, 0, resDuplicate.Created)
	assert.Equal(t, 2, resDuplicate.Skipped)
}

func TestBackupService_Import_CSV(t *testing.T) {
	deps := setupBackupService(t)

	csvData := `Title,Type,Year,Status,MyRating,TMDBID
Oppenheimer,movie,2023,completed,10,872585
Succession,series,2018,completed,9,76331
`

	res, err := deps.svc.Import(strings.NewReader(csvData), "library.csv", false)
	require.NoError(t, err)
	assert.Equal(t, 2, res.Created)

	titles, err := deps.titleRepo.ListAll()
	require.NoError(t, err)
	assert.Len(t, titles, 2)
}

func TestBackupService_Import_ZIP(t *testing.T) {
	deps := setupBackupService(t)

	// Create in-memory zip archive
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	jsonContent := `{
		"movies": [
			{
				"status": "completed",
				"movie": {
					"title": "Interstellar",
					"year": 2014,
					"ids": { "tmdb": 157336 }
				}
			}
		]
	}`

	w, err := zw.Create("SimklBackup.json")
	require.NoError(t, err)
	_, err = w.Write([]byte(jsonContent))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	// Import zip
	res, err := deps.svc.Import(bytes.NewReader(zipBuf.Bytes()), "archive.zip", false)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Created)

	titles, err := deps.titleRepo.ListAll()
	require.NoError(t, err)
	require.Len(t, titles, 1)
	assert.Equal(t, "Interstellar", titles[0].PrimaryName())
}

func TestBackupService_Import_ZIP_Oversized(t *testing.T) {
	deps := setupBackupService(t)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	w, err := zw.Create("oversized.json")
	require.NoError(t, err)

	// Write 51MB of spaces (highly compressible)
	chunk := bytes.Repeat([]byte(" "), 1024*1024)
	for i := 0; i < 51; i++ {
		_, err = w.Write(chunk)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	_, err = deps.svc.Import(bytes.NewReader(zipBuf.Bytes()), "archive.zip", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed size")
}

func TestBackupService_V2LosslessRoundTrip(t *testing.T) {
	src := setupBackupService(t)
	ctx := context.Background()

	// Seed rich data: personal_notes, watch_providers, origin_country, season rating,
	// season_external_ids, watch_events, and wrapped_snapshots.
	tmdb := int64(209867)
	radarr := int64(55)
	notes := "Rewatch in winter!"
	providers := []model.WatchProvider{{ID: 8, Name: "Netflix"}}
	origin := "JP"
	var titleID, seasonID, epID int64
	require.NoError(t, database.WithTxContext(ctx, src.db, func(tx *sql.Tx) error {
		var err error
		titleID, err = repository.NewTitleWriter(tx).Create(ctx, &model.Title{
			Type: model.TitleTypeSeries, IsAnime: true, Year: 2023,
			Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
			TMDBID: &tmdb, RadarrID: &radarr, ArrIgnored: true,
		}, []model.TitleName{{Name: "Frieren", Language: "en", IsPrimary: true}})
		if err != nil {
			return err
		}
		if err := repository.NewBackupWriter(tx).UpdateTitleExtraFields(ctx, titleID, model.Title{
			PersonalNotes:  &notes,
			WatchProviders: providers,
			OriginCountry:  &origin,
		}); err != nil {
			return err
		}
		s, err := repository.NewSeasonWriter(tx).GetOrCreate(ctx, titleID, 1)
		if err != nil {
			return err
		}
		seasonID = s.ID
		if err := repository.NewSeasonWriter(tx).UpdateTotalEpisodes(ctx, seasonID, 28); err != nil {
			return err
		}
		ep, err := repository.NewEpisodeWriter(tx).GetOrCreate(ctx, seasonID, 1)
		if err != nil {
			return err
		}
		epID = ep.ID
		if err := repository.NewEpisodeWriter(tx).MarkWatched(ctx, epID, time.Now().UTC()); err != nil {
			return err
		}
		if err := repository.NewBackupWriter(tx).RestoreSeasonExternalID(ctx, seasonID, repository.BackupSeasonExternalID{
			Provider: repository.ProviderAniList, ExternalID: "154587", SortOrder: 1,
		}); err != nil {
			return err
		}
		if _, err := repository.NewWatchEventWriter(tx).Create(ctx, &model.WatchEvent{
			TitleID:   titleID,
			EpisodeID: &epID,
			Source:    model.WatchEventSourceManual,
		}); err != nil {
			return err
		}
		return repository.NewBackupWriter(tx).RestoreWrappedSnapshot(ctx, repository.BackupWrappedSnapshot{
			Year: 2025, DataJSON: `{"year":2025}`, CreatedAt: time.Now().UTC(),
		})
	}))

	exported, err := src.svc.ExportJSON(ctx)
	require.NoError(t, err)

	var parsed service.TrackarrBackup
	require.NoError(t, json.Unmarshal(exported, &parsed))
	assert.Equal(t, "2.0", parsed.Version)
	require.Len(t, parsed.Titles, 1)
	require.NotNil(t, parsed.Titles[0].PersonalNotes)
	assert.Equal(t, notes, *parsed.Titles[0].PersonalNotes)
	require.Len(t, parsed.SeasonExternalIDs, 1)
	require.Len(t, parsed.WatchEvents, 1)
	require.Len(t, parsed.WrappedSnapshots, 1)

	// Import into a completely fresh DB and verify zero data loss
	dst := setupBackupService(t)
	res, err := dst.svc.Import(bytes.NewReader(exported), "trackarr-backup.json", false)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Created)

	titles, err := dst.titleRepo.ListAll()
	require.NoError(t, err)
	require.Len(t, titles, 1)
	require.NotNil(t, titles[0].PersonalNotes)
	assert.Equal(t, notes, *titles[0].PersonalNotes)
	assert.Equal(t, providers, titles[0].WatchProviders)
	require.NotNil(t, titles[0].OriginCountry)
	assert.Equal(t, origin, *titles[0].OriginCountry)
	require.NotNil(t, titles[0].RadarrID)
	assert.Equal(t, radarr, *titles[0].RadarrID)
	require.Len(t, titles[0].Seasons, 1)
	require.NotNil(t, titles[0].Seasons[0].TotalEpisodes)
	assert.Equal(t, 28, *titles[0].Seasons[0].TotalEpisodes)
	require.Len(t, titles[0].Seasons[0].Episodes, 1)
	assert.True(t, titles[0].Seasons[0].Episodes[0].Watched)

	br := repository.NewBackupRepository(dst.db)
	extIDs, err := br.ListAllSeasonExternalIDs(ctx)
	require.NoError(t, err)
	require.Len(t, extIDs, 1)
	assert.Equal(t, "154587", extIDs[0].ExternalID)

	events, err := br.ListAllWatchEvents(ctx)
	require.NoError(t, err)
	require.Len(t, events, 1)

	wrapped, err := br.ListAllWrappedSnapshots(ctx)
	require.NoError(t, err)
	require.Len(t, wrapped, 1)
	assert.Equal(t, 2025, wrapped[0].Year)
}

func TestBackupService_WriteAutomatedBackupsAndVacuum(t *testing.T) {
	deps := setupBackupService(t)
	tmpDir := t.TempDir()

	// Create an old rotated file that should be pruned
	oldFile := filepath.Join(tmpDir, "trackarr-2020-01-01.db.gz")
	require.NoError(t, os.WriteFile(oldFile, []byte("old"), 0o644))
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(oldFile, oldTime, oldTime))

	require.NoError(t, deps.svc.WriteAutomatedBackups(context.Background(), tmpDir, 7))

	today := time.Now().Format("2006-01-02")
	_, err := os.Stat(filepath.Join(tmpDir, fmt.Sprintf("trackarr-%s.db.gz", today)))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, fmt.Sprintf("trackarr-%s.json.gz", today)))
	assert.NoError(t, err)
	_, err = os.Stat(oldFile)
	assert.True(t, os.IsNotExist(err), "old backup should be rotated out")
}
