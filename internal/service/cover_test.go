package service_test

import (
	"context"
	"os"
	"path/filepath"
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

func TestCoverService_CleanupUnusedCovers(t *testing.T) {
	dataDir := t.TempDir()

	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	titleRepo := repository.NewTitleRepository(db)
	coverSvc := service.NewCoverService(db, titleRepo, nil, nil, dataDir)

	coversDir := filepath.Join(dataDir, "covers")
	require.NoError(t, os.MkdirAll(coversDir, 0755))

	files := []string{
		"a123.jpg", // Sunday prefix 'a'
		"b456.jpg", // Sunday prefix 'b'
		"j789.jpg", // Monday prefix 'j'
		"z012.jpg", // Tuesday prefix 'z'
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(coversDir, f), []byte("dummy"), 0644))
	}

	coverA := "a123.jpg"
	coverZ := "z012.jpg"
	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2023,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
		CoverURL:    &coverA,
	}, []model.TitleName{{Name: "Movie A", Language: "en", IsPrimary: true}})

	testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2023,
		Status:      model.TitleStatusPlanToWatch,
		MatchStatus: model.MatchStatusConfirmed,
		CoverURL:    &coverZ,
	}, []model.TitleName{{Name: "Movie Z", Language: "en", IsPrimary: true}})

	coverSvc.CleanupUnusedCovers(context.Background(), time.Sunday)

	// After Sunday cleanup, 'b456.jpg' should be deleted (starts with 'b', unused)
	// 'a123.jpg' remains (starts with 'a', used)
	// 'j789.jpg' remains (starts with 'j', not checked on Sunday)
	// 'z012.jpg' remains (starts with 'z', not checked on Sunday)

	_, err = os.Stat(filepath.Join(coversDir, "b456.jpg"))
	assert.True(t, os.IsNotExist(err), "b456.jpg should be deleted")

	_, err = os.Stat(filepath.Join(coversDir, "a123.jpg"))
	assert.NoError(t, err, "a123.jpg should remain")

	_, err = os.Stat(filepath.Join(coversDir, "j789.jpg"))
	assert.NoError(t, err, "j789.jpg should remain")

	_, err = os.Stat(filepath.Join(coversDir, "z012.jpg"))
	assert.NoError(t, err, "z012.jpg should remain")
}
