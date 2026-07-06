package repository_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListOriginCountries(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	kr := "KR"
	jp := "JP"

	titleA := &model.Title{Type: model.TitleTypeMovie, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}
	idA := testutil.CreateTitle(t, db, titleA, []model.TitleName{{Name: "A", Language: "en", IsPrimary: true}})
	testutil.UpdateTitle(t, db, idA, repository.TitleUpdate{OriginCountry: &kr})

	titleB := &model.Title{Type: model.TitleTypeMovie, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}
	idB := testutil.CreateTitle(t, db, titleB, []model.TitleName{{Name: "B", Language: "en", IsPrimary: true}})
	testutil.UpdateTitle(t, db, idB, repository.TitleUpdate{OriginCountry: &kr})

	titleC := &model.Title{Type: model.TitleTypeMovie, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}
	idC := testutil.CreateTitle(t, db, titleC, []model.TitleName{{Name: "C", Language: "en", IsPrimary: true}})
	testutil.UpdateTitle(t, db, idC, repository.TitleUpdate{OriginCountry: &jp})

	// D: no origin_country set (NULL) — must be excluded.
	titleD := &model.Title{Type: model.TitleTypeMovie, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}
	testutil.CreateTitle(t, db, titleD, []model.TitleName{{Name: "D", Language: "en", IsPrimary: true}})

	got, err := repo.ListOriginCountries()
	require.NoError(t, err)
	assert.Equal(t, []repository.CountryCount{
		{Country: "KR", Count: 2},
		{Country: "JP", Count: 1},
	}, got)
}
