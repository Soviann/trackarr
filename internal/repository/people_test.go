package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Soviann/trackarr/internal/model"
	"github.com/Soviann/trackarr/internal/repository"
	"github.com/Soviann/trackarr/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeopleRepository_Search(t *testing.T) {
	ctx := context.Background()

	t.Run("nominal partial, full, accent-insensitive, multi-token, roles and ranking", func(t *testing.T) {
		db := setupTestDB(t)
		repo := repository.NewPeopleRepository(db)

		// Across all watch statuses: completed, watching, plan_to_watch, dropped.
		cDune := `[{"name":"Denis Villeneuve","role":"Director"},{"name":"Denis Villeneuve","role":"Writer"},{"name":"Hans Zimmer","role":"Composer"},{"name":"Timothée Chalamet","role":"Paul Atreides"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2021,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &cDune,
		}, []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}})

		cArrival := `[{"name":"Denis Villeneuve","role":"Director"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2016,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &cArrival,
		}, []model.TitleName{{Name: "Arrival", Language: "en", IsPrimary: true}})

		cInterstellar := `[{"name":"Hans Zimmer","role":"Composer"},{"name":"Timothée Chalamet","role":"Young Tom"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2014,
			Status:      model.TitleStatusPlanToWatch,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &cInterstellar,
		}, []model.TitleName{{Name: "Interstellar", Language: "en", IsPrimary: true}})

		cOppenheimer := `[{"name":"Ludwig Göransson","role":"Composer"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2023,
			Status:      model.TitleStatusDropped,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &cOppenheimer,
		}, []model.TitleName{{Name: "Oppenheimer", Language: "en", IsPrimary: true}})

		cReservoir := `[{"name":"Quentin Tarantino","role":"Director"},{"name":"Quentin Tarantino","role":"Writer"},{"name":"Quentin Tarantino","role":"Mr. Brown"},{"name":"Quentin Tarantino","role":"Composer"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        1992,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &cReservoir,
		}, []model.TitleName{{Name: "Reservoir Dogs", Language: "en", IsPrimary: true}})

		// Exact match & Composer role across plan_to_watch + completed
		res, err := repo.Search(ctx, "Hans Zimmer", 20)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, model.PersonSearchResult{
			Name:       "Hans Zimmer",
			Roles:      []string{"composer"},
			TitleCount: 2,
		}, res[0])

		// Word prefix partial match ("Zimmer")
		res, err = repo.Search(ctx, "Zimmer", 20)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "Hans Zimmer", res[0].Name)

		// Lowercase full-name match & multi-title role union
		res, err = repo.Search(ctx, "denis villeneuve", 20)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, model.PersonSearchResult{
			Name:       "Denis Villeneuve",
			Roles:      []string{"director", "writer"},
			TitleCount: 2,
		}, res[0])

		// Accent-insensitive prefix match ("Timothee" -> "Timothée Chalamet")
		res, err = repo.Search(ctx, "Timothee", 20)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, model.PersonSearchResult{
			Name:       "Timothée Chalamet",
			Roles:      []string{"actor"},
			TitleCount: 2,
		}, res[0])

		// Accent-insensitive word-prefix match ("Goransson" -> "Ludwig Göransson") even on dropped title
		res, err = repo.Search(ctx, "Goransson", 20)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, model.PersonSearchResult{
			Name:       "Ludwig Göransson",
			Roles:      []string{"composer"},
			TitleCount: 1,
		}, res[0])

		// Multi-token out-of-order query ("zimmer hans")
		res, err = repo.Search(ctx, "  zimmer   hans ", 20)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "Hans Zimmer", res[0].Name)

		// Deterministic role order ["director", "writer", "composer", "actor"] and distinct title_count=1
		res, err = repo.Search(ctx, "Tarantino", 20)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, model.PersonSearchResult{
			Name:       "Quentin Tarantino",
			Roles:      []string{"director", "writer", "composer", "actor"},
			TitleCount: 1,
		}, res[0])
	})

	t.Run("ranking tiers and tie-breakers", func(t *testing.T) {
		db := setupTestDB(t)
		repo := repository.NewPeopleRepository(db)

		// Add people whose names match "han" at different tiers:
		// 1. Exact: "Han" (1 title)
		// 2. Prefix: "Hans Zimmer" (1 title), "Hanna Lee" (2 titles) -> Hanna Lee ranks ahead of Hans Zimmer via title_count DESC
		// 3. Word prefix: "Ben Hanlon" (3 titles)
		// 4. Substring: "Johnathan Doe" (4 titles)
		// 5. Multi-token (for query "zimmer hans"): tested separately
		seed := []struct {
			title string
			creds string
		}{
			{"T1", `[{"name":"Han","role":"Actor"}]`},
			{"T2", `[{"name":"Hans Zimmer","role":"Composer"}]`},
			{"T3", `[{"name":"Hanna Lee","role":"Director"}]`},
			{"T4", `[{"name":"Hanna Lee","role":"Director"}]`},
			{"T5", `[{"name":"Ben Hanlon","role":"Actor"}]`},
			{"T6", `[{"name":"Ben Hanlon","role":"Actor"}]`},
			{"T7", `[{"name":"Ben Hanlon","role":"Actor"}]`},
			{"T8", `[{"name":"Johnathan Doe","role":"Writer"}]`},
			{"T9", `[{"name":"Johnathan Doe","role":"Writer"}]`},
			{"T10", `[{"name":"Johnathan Doe","role":"Writer"}]`},
			{"T11", `[{"name":"Johnathan Doe","role":"Writer"}]`},
		}
		for _, s := range seed {
			c := s.creds
			testutil.CreateTitle(t, db, &model.Title{
				Type:        model.TitleTypeMovie,
				Year:        2024,
				Status:      model.TitleStatusCompleted,
				MatchStatus: model.MatchStatusConfirmed,
				Credits:     &c,
			}, []model.TitleName{{Name: s.title, Language: "en", IsPrimary: true}})
		}

		res, err := repo.Search(ctx, "han", 20)
		require.NoError(t, err)
		require.Len(t, res, 5)
		assert.Equal(t, "Han", res[0].Name)           // exact
		assert.Equal(t, "Hanna Lee", res[1].Name)     // prefix, count=2
		assert.Equal(t, "Hans Zimmer", res[2].Name)   // prefix, count=1
		assert.Equal(t, "Ben Hanlon", res[3].Name)    // word prefix
		assert.Equal(t, "Johnathan Doe", res[4].Name) // substring
	})

	t.Run("boundary: blank query, no match, empty/invalid fields, limit clamping", func(t *testing.T) {
		db := setupTestDB(t)
		repo := repository.NewPeopleRepository(db)

		// Empty/whitespace/combining-only queries return non-nil empty slice
		for _, q := range []string{"", "   ", "\t\n", "\u0301"} {
			res, err := repo.Search(ctx, q, 20)
			require.NoError(t, err)
			require.NotNil(t, res)
			assert.Empty(t, res)
		}

		// Seed 60 matching people plus empty-name / empty-role entries
		var creditsList []string
		for i := 1; i <= 60; i++ {
			creditsList = append(creditsList, fmt.Sprintf(`{"name":"Actor %02d","role":"Hero"}`, i))
		}
		creditsList = append(creditsList, `{"name":"","role":"Director"}`, `{"name":"Ghost Empty Role","role":""}`)
		c := "[" + joinComma(creditsList) + "]"
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2024,
			Status:      model.TitleStatusWatching,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &c,
		}, []model.TitleName{{Name: "Ensemble", Language: "en", IsPrimary: true}})

		// Empty role & empty name ignored
		res, err := repo.Search(ctx, "Ghost", 20)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Empty(t, res)

		// Limit <= 0 defaults to 20
		res, err = repo.Search(ctx, "Actor", 0)
		require.NoError(t, err)
		assert.Len(t, res, 20)

		// Explicit limit respected
		res, err = repo.Search(ctx, "Actor", 5)
		require.NoError(t, err)
		assert.Len(t, res, 5)
		assert.Equal(t, "Actor 01", res[0].Name)

		// Limit > 50 capped at 50
		res, err = repo.Search(ctx, "Actor", 100)
		require.NoError(t, err)
		assert.Len(t, res, 50)
	})

	t.Run("error and resilience: malformed credits JSON skipped and cancelled context fails", func(t *testing.T) {
		db := setupTestDB(t)
		repo := repository.NewPeopleRepository(db)

		for _, bad := range []string{`{malformed`, `"[1,2]"`, `"null"`, `[1,2,3]`} {
			b := bad
			testutil.CreateTitle(t, db, &model.Title{
				Type:        model.TitleTypeMovie,
				Year:        2020,
				Status:      model.TitleStatusCompleted,
				MatchStatus: model.MatchStatusConfirmed,
				Credits:     &b,
			}, []model.TitleName{{Name: "Bad", Language: "en", IsPrimary: true}})
		}

		good := `[{"name":"Christopher Nolan","role":"Director"}]`
		testutil.CreateTitle(t, db, &model.Title{
			Type:        model.TitleTypeMovie,
			Year:        2010,
			Status:      model.TitleStatusCompleted,
			MatchStatus: model.MatchStatusConfirmed,
			Credits:     &good,
		}, []model.TitleName{{Name: "Inception", Language: "en", IsPrimary: true}})

		res, err := repo.Search(ctx, "Nolan", 20)
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "Christopher Nolan", res[0].Name)

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err = repo.Search(canceledCtx, "Nolan", 20)
		assert.Error(t, err)
	})
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}
