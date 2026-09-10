package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/service"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshAllJob_Lifecycle(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	bgSvc := service.NewBackgroundService(db, titleRepo, settingRepo, nil, nil, nil)

	// Insert 2 test titles
	ctx := context.Background()
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2020,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Movie 1", Language: "en", IsPrimary: true}})

	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2021,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Movie 2", Language: "en", IsPrimary: true}})

	// 1. Initial progress is idle
	prog := bgSvc.GetRefreshAllProgress()
	assert.Equal(t, service.RefreshStatusIdle, prog.Status)

	// 2. Start job
	startedProg, err := bgSvc.StartRefreshAllJob(ctx, false)
	require.NoError(t, err)
	assert.Equal(t, service.RefreshStatusRunning, startedProg.Status)

	// 3. Second start while running returns conflict
	_, err = bgSvc.StartRefreshAllJob(ctx, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")

	// Wait for job to finish 2 titles
	require.Eventually(t, func() bool {
		p := bgSvc.GetRefreshAllProgress()
		return p.Status == service.RefreshStatusCompleted
	}, 5*time.Second, 100*time.Millisecond)

	// Verify status after completion
	finalProg := bgSvc.GetRefreshAllProgress()
	assert.Equal(t, service.RefreshStatusCompleted, finalProg.Status)
	assert.Equal(t, 2, finalProg.TotalTitles)
	assert.Equal(t, 2, finalProg.ProcessedTitles)
}

func TestRefreshAllJob_Cancel(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	bgSvc := service.NewBackgroundService(db, titleRepo, settingRepo, nil, nil, nil)

	// 1. Cancel on idle is a no-op
	prog, err := bgSvc.CancelRefreshAllJob()
	require.NoError(t, err)
	assert.Equal(t, service.RefreshStatusIdle, prog.Status)

	// 2. Start job then immediately cancel
	_, err = bgSvc.StartRefreshAllJob(context.Background(), false)
	require.NoError(t, err)

	cancelledProg, err := bgSvc.CancelRefreshAllJob()
	require.NoError(t, err)
	assert.Equal(t, service.RefreshStatusPaused, cancelledProg.Status)
}
