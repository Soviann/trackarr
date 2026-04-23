package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenreHandler_List(t *testing.T) {
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })

	// Insert a title and some genres
	id1 := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Movie 1", Language: "en", IsPrimary: true}})
	id2 := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeMovie,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}, []model.TitleName{{Name: "Movie 2", Language: "en", IsPrimary: true}})

	_, err = db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, 'Drama'), (?, 'Action')`, id1, id1)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, 'Drama')`, id2)
	require.NoError(t, err)

	h := handler.NewGenreHandler(repository.NewGenreRepository(db))

	req := httptest.NewRequest(http.MethodGet, "/api/genres", nil)
	w := httptest.NewRecorder()
	err = h.List(w, req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var genres []map[string]any
	assert.NoError(t, json.NewDecoder(w.Body).Decode(&genres))
	assert.Len(t, genres, 2)
	assert.Equal(t, "Drama", genres[0]["genre"])
	assert.InDelta(t, float64(2), genres[0]["count"], 0)
}
