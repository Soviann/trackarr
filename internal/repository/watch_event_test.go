package repository_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestWatchEventRepo_CountByTitleID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewWatchEventRepository(db)

	runtime := 120
	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		Runtime:     &runtime,
	}
	id := testutil.CreateTitle(t, db, title, []model.TitleName{{Name: "Test Movie", Language: "en", IsPrimary: true}})

	// No events yet
	count, err := repo.CountByTitleID(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add two events
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: id, Source: model.WatchEventSourceManual})
	testutil.CreateWatchEvent(t, db, &model.WatchEvent{TitleID: id, Source: model.WatchEventSourceManual})

	count, err = repo.CountByTitleID(id)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}
