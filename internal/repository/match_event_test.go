package repository_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchEventRepo_ListRecent_OrderAndJoin(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewMatchEventRepository(db)

	coverURL := "https://example.com/cover.jpg"
	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		CoverURL:    &coverURL,
	}
	titleID := testutil.CreateTitle(t, db, title, []model.TitleName{{Name: "Test Movie", Language: "en", IsPrimary: true}})

	// Insert two events via writer in a transaction
	tx, err := db.Begin()
	require.NoError(t, err)
	w := repository.NewMatchEventWriter(tx)

	require.NoError(t, w.Create(t.Context(), titleID, model.MatchEventAutoConfirmed, "first event"))
	require.NoError(t, w.Create(t.Context(), titleID, model.MatchEventSeasonAttached, "second event"))
	require.NoError(t, tx.Commit())

	events, err := repo.ListRecent(t.Context(), 10)
	require.NoError(t, err)
	assert.Len(t, events, 2)

	// Newest-first order — second inserted event should be first (same second, tie-broken by id DESC)
	assert.Equal(t, model.MatchEventSeasonAttached, events[0].Kind)
	assert.Equal(t, "second event", events[0].Detail)
	assert.NotNil(t, events[0].TitleID)
	assert.Equal(t, titleID, *events[0].TitleID)
	assert.NotNil(t, events[0].CoverURL)
	assert.Equal(t, coverURL, *events[0].CoverURL)

	assert.Equal(t, model.MatchEventAutoConfirmed, events[1].Kind)
	assert.Equal(t, "first event", events[1].Detail)
}

func TestMatchEventRepo_ListRecent_LimitRespected(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewMatchEventRepository(db)

	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}
	titleID := testutil.CreateTitle(t, db, title, []model.TitleName{{Name: "Limit Test", Language: "en", IsPrimary: true}})

	tx, err := db.Begin()
	require.NoError(t, err)
	w := repository.NewMatchEventWriter(tx)
	for i := 0; i < 5; i++ {
		require.NoError(t, w.Create(t.Context(), titleID, model.MatchEventAutoConfirmed, "event"))
	}
	require.NoError(t, tx.Commit())

	events, err := repo.ListRecent(t.Context(), 3)
	require.NoError(t, err)
	assert.Len(t, events, 3)
}

func TestMatchEventRepo_CascadeDeleteOnTitle(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewMatchEventRepository(db)

	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}
	titleID := testutil.CreateTitle(t, db, title, []model.TitleName{{Name: "Cascade Test", Language: "en", IsPrimary: true}})

	tx, err := db.Begin()
	require.NoError(t, err)
	w := repository.NewMatchEventWriter(tx)
	require.NoError(t, w.Create(t.Context(), titleID, model.MatchEventAutoConfirmed, "will cascade"))
	require.NoError(t, tx.Commit())

	// Verify event exists before deletion
	events, err := repo.ListRecent(t.Context(), 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// Delete the title — should cascade-delete its match events
	_, err = db.Exec(`DELETE FROM titles WHERE id = ?`, titleID)
	require.NoError(t, err)

	events, err = repo.ListRecent(t.Context(), 10)
	require.NoError(t, err)
	assert.Empty(t, events)
}
