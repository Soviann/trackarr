package repository_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchEventRepo_CountByTitleID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewWatchEventRepository(db)
	titleRepo := repository.NewTitleRepository(db)

	runtime := 120
	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		Runtime:     &runtime,
	}
	id, err := titleRepo.Create(title, []model.TitleName{{Name: "Test Movie", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	// No events yet
	count, err := repo.CountByTitleID(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add two events
	_, err = repo.Create(&model.WatchEvent{TitleID: id, Source: model.WatchEventSourceManual})
	require.NoError(t, err)
	_, err = repo.Create(&model.WatchEvent{TitleID: id, Source: model.WatchEventSourceManual})
	require.NoError(t, err)

	count, err = repo.CountByTitleID(id)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}
