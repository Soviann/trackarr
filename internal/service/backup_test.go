package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
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

	assert.Equal(t, "1.0", backup.Version)
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

