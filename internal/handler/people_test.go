package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Soviann/trackarr/internal/cache"
	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/handler"
	"github.com/Soviann/trackarr/internal/handler/httputil"
	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPeopleHandler(t *testing.T) (*handler.PeopleHandler, *sql.DB) {
	t.Helper()
	db := testutil.NewTestDB(t)
	repo := repository.NewPeopleRepository(db)
	return handler.NewPeopleHandler(repo), db
}

func TestPeopleHandler_Search(t *testing.T) {
	t.Run("nominal queries, accent tolerance, and limit", func(t *testing.T) {
		h, db := setupPeopleHandler(t)

		c1 := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Timothée Chalamet","role":"Paul Atreides"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2021,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &c1,
		}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})

		c2 := `[{"name":"Hans Zimmer","role":"Composer"},{"name":"Hans Zimmer Jr","role":"Composer"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2014,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &c2,
		}, []model.TitleName{{Name: "Interstellar", Language: "en", IsPrimary: true}})

		for _, tc := range []struct {
			url          string
			expectedName string
			expectedCnt  int
			expectedRole []string
		}{
			{"/api/people?search=Hans%20Zimmer", "Hans Zimmer", 2, []string{"composer"}},
			{"/api/people?search=denis%20villeneuve", "Denis Villeneuve", 1, []string{"director"}},
			{"/api/people?search=timothee", "Timothée Chalamet", 1, []string{"actor"}},
		} {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			rr := httptest.NewRecorder()
			require.NoError(t, h.Search(rr, req))
			assert.Equal(t, http.StatusOK, rr.Code)

			var got []model.PersonSearchResult
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
			require.NotEmpty(t, got)
			assert.Equal(t, tc.expectedName, got[0].Name)
			assert.Equal(t, tc.expectedCnt, got[0].TitleCount)
			assert.Equal(t, tc.expectedRole, got[0].Roles)
		}

		// Limit param respected
		req := httptest.NewRequest(http.MethodGet, "/api/people?search=Hans&limit=1", nil)
		rr := httptest.NewRecorder()
		require.NoError(t, h.Search(rr, req))
		assert.Equal(t, http.StatusOK, rr.Code)
		var limited []model.PersonSearchResult
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&limited))
		assert.Len(t, limited, 1)
	})

	t.Run("boundary: blank query and no match return 200 empty JSON array", func(t *testing.T) {
		h, _ := setupPeopleHandler(t)

		for _, url := range []string{"/api/people", "/api/people?search=", "/api/people?search=%20%20", "/api/people?search=nobody"} {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rr := httptest.NewRecorder()
			require.NoError(t, h.Search(rr, req))
			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, "[]\n", rr.Body.String())
		}
	})

	t.Run("cache hit and post-commit invalidation via TitleWriter", func(t *testing.T) {
		h, db := setupPeopleHandler(t)

		c := cache.New(10, 0)
		database.SetInvalidator(c)
		defer database.SetInvalidator(nil)
		h.SetCache(c)

		c1 := `[{"name":"Hans Zimmer","role":"Composer"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2021,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &c1,
		}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})

		// 1. Populate cache
		req1 := httptest.NewRequest(http.MethodGet, "/api/people?search=Zimmer", nil)
		rr1 := httptest.NewRecorder()
		require.NoError(t, h.Search(rr1, req1))
		assert.Equal(t, http.StatusOK, rr1.Code)

		var res1 []model.PersonSearchResult
		require.NoError(t, json.NewDecoder(rr1.Body).Decode(&res1))
		require.Len(t, res1, 1)
		assert.Equal(t, 1, res1[0].TitleCount)

		_, cached := c.Get("people:search:search=Zimmer")
		assert.True(t, cached)

		// 2. Write via TitleWriter inside WithTx -> invalidates cache post-commit
		c2 := `[{"name":"Hans Zimmer","role":"Composer"}]`
		require.NoError(t, database.WithTx(db, func(tx *sql.Tx) error {
			_, err := repository.NewTitleWriter(tx).Create(context.Background(), &model.Title{
				Type:        model.TitleTypeMovie,
				Year:        2010,
				Status:      model.TitleStatusCompleted,
				MatchStatus: model.MatchStatusConfirmed,
				Credits:     &c2,
			}, []model.TitleName{{Name: "Inception", Language: "en", IsPrimary: true}})
			return err
		}))

		_, cachedAfter := c.Get("people:search:search=Zimmer")
		assert.False(t, cachedAfter, "TitleWriter commit must invalidate people search cache")

		// 3. Re-query reflects new title_count=2
		req2 := httptest.NewRequest(http.MethodGet, "/api/people?search=Zimmer", nil)
		rr2 := httptest.NewRecorder()
		require.NoError(t, h.Search(rr2, req2))
		var res2 []model.PersonSearchResult
		require.NoError(t, json.NewDecoder(rr2.Body).Decode(&res2))
		require.Len(t, res2, 1)
		assert.Equal(t, 2, res2[0].TitleCount)
	})

	t.Run("error: closed database returns 500 APIError", func(t *testing.T) {
		db, _, err := database.Open(":memory:")
		require.NoError(t, err)
		require.NoError(t, database.Migrate(db))
		h := handler.NewPeopleHandler(repository.NewPeopleRepository(db))
		require.NoError(t, db.Close())

		req := httptest.NewRequest(http.MethodGet, "/api/people?search=Zimmer", nil)
		rr := httptest.NewRecorder()
		err = h.Search(rr, req)
		require.Error(t, err)
		var apiErr *httputil.APIError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Status)
		assert.Equal(t, "Failed to search people", apiErr.Message)
	})
}
