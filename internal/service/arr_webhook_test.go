package service_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArrWebhookService_RadarrDownload_Nominal(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	push := &fakePushNotifier{}
	svc := service.NewArrWebhookService(db, titleRepo, settingRepo, push)

	ctx := context.Background()

	// Seed movie
	var titleID int64
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		tw := repository.NewTitleWriter(tx)
		tmdbID := int64(27205)
		id, err := tw.Create(ctx, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2010,
			TMDBID:      &tmdbID,
			Status:      model.TitleStatusPlanToWatch,
			MatchStatus: model.MatchStatusConfirmed,
		}, []model.TitleName{
			{Name: "Inception", Language: "en", IsPrimary: true},
		})
		if err != nil {
			return err
		}
		titleID = id
		return nil
	}))

	payload := &model.ArrWebhookPayload{
		EventType: "Download",
		Movie: &model.ArrMoviePayload{
			ID:     101,
			Title:  "Inception",
			Year:   2010,
			TMDBID: 27205,
		},
	}

	err := svc.ProcessArrWebhook(ctx, payload)
	require.NoError(t, err)

	updated, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	require.NotNil(t, updated.DownloadedAt)
	require.NotNil(t, updated.RadarrID)
	assert.Equal(t, int64(101), *updated.RadarrID)

	require.Len(t, push.calls, 1)
	assert.Contains(t, push.calls[0].Body, "Inception")
	assert.Contains(t, push.calls[0].Body, "est disponible au visionnage")
}

func TestArrWebhookService_SonarrDownload_Nominal(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	push := &fakePushNotifier{}
	svc := service.NewArrWebhookService(db, titleRepo, settingRepo, push)

	ctx := context.Background()

	// Seed series + season + episode
	var titleID int64
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		tw := repository.NewTitleWriter(tx)
		sw := repository.NewSeasonWriter(tx)
		ew := repository.NewEpisodeWriter(tx)

		tvdbID := int64(376045)
		id, err := tw.Create(ctx, &model.Title{
			Type:        model.TitleTypeSeries,
			Year:        2021,
			TVDBID:      &tvdbID,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
		}, []model.TitleName{
			{Name: "Arcane", Language: "en", IsPrimary: true},
		})
		if err != nil {
			return err
		}
		titleID = id

		season, err := sw.GetOrCreate(ctx, titleID, 1)
		if err != nil {
			return err
		}
		_, err = ew.GetOrCreate(ctx, season.ID, 1)
		return err
	}))

	payload := &model.ArrWebhookPayload{
		EventType: "Download",
		Series: &model.ArrSeriesPayload{
			ID:     45,
			Title:  "Arcane",
			TVDBID: 376045,
		},
		Episodes: []model.ArrEpisodePayload{
			{
				ID:            1,
				EpisodeNumber: 1,
				SeasonNumber:  1,
				Title:         "Welcome to the Playground",
			},
		},
	}

	err := svc.ProcessArrWebhook(ctx, payload)
	require.NoError(t, err)

	updated, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	require.NotNil(t, updated.SonarrID)
	assert.Equal(t, int64(45), *updated.SonarrID)

	episodes, err := repository.NewEpisodeRepository(db).GetBySeasonID(updated.Seasons[0].ID)
	require.NoError(t, err)
	require.Len(t, episodes, 1)
	require.NotNil(t, episodes[0].DownloadedAt)

	require.Len(t, push.calls, 1)
	assert.Contains(t, push.calls[0].Body, "Arcane")
	assert.Contains(t, push.calls[0].Body, "S01E01")
}

func TestArrWebhookService_Boundary_TestEvent(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	push := &fakePushNotifier{}
	svc := service.NewArrWebhookService(db, titleRepo, settingRepo, push)

	ctx := context.Background()
	payload := &model.ArrWebhookPayload{
		EventType:    "Test",
		InstanceName: "Radarr-Test",
	}

	err := svc.ProcessArrWebhook(ctx, payload)
	require.NoError(t, err)
	assert.Empty(t, push.calls)
}

func TestArrWebhookService_Boundary_DisabledNotification(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	push := &fakePushNotifier{}
	svc := service.NewArrWebhookService(db, titleRepo, settingRepo, push)

	ctx := context.Background()

	// Disable notif_media_downloaded in settings
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		sw := repository.NewSettingWriter(tx)
		return sw.Set(ctx, service.NotifMediaDownloaded, "false")
	}))

	// Seed movie
	var titleID int64
	require.NoError(t, database.WithTxContext(ctx, db, func(tx *sql.Tx) error {
		tw := repository.NewTitleWriter(tx)
		tmdbID := int64(100)
		id, err := tw.Create(ctx, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2024,
			TMDBID:      &tmdbID,
			Status:      model.TitleStatusPlanToWatch,
			MatchStatus: model.MatchStatusConfirmed,
		}, []model.TitleName{
			{Name: "Silent Movie", Language: "en", IsPrimary: true},
		})
		if err != nil {
			return err
		}
		titleID = id
		return nil
	}))

	payload := &model.ArrWebhookPayload{
		EventType: "Download",
		Movie: &model.ArrMoviePayload{
			ID:     5,
			Title:  "Silent Movie",
			TMDBID: 100,
		},
	}

	err := svc.ProcessArrWebhook(ctx, payload)
	require.NoError(t, err)

	updated, err := titleRepo.GetByID(titleID)
	require.NoError(t, err)
	require.NotNil(t, updated.DownloadedAt)
	// Notification was disabled:
	assert.Empty(t, push.calls)
}

func TestArrWebhookService_Error_MediaNotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	push := &fakePushNotifier{}
	svc := service.NewArrWebhookService(db, titleRepo, settingRepo, push)

	ctx := context.Background()
	payload := &model.ArrWebhookPayload{
		EventType: "Download",
		Movie: &model.ArrMoviePayload{
			ID:     9999,
			Title:  "Nonexistent Movie",
			TMDBID: 999999,
		},
	}

	// Must return nil without error (200 OK to Arr) and not send push
	err := svc.ProcessArrWebhook(ctx, payload)
	require.NoError(t, err)
	assert.Empty(t, push.calls)
}
