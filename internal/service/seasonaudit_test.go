package service_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/nicolasvasse/plextracker/internal/service"
	"github.com/nicolasvasse/plextracker/internal/service/matching"
	"github.com/nicolasvasse/plextracker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anilistMedia is a minimal AniList relations node for the stub server. It only
// carries the fields ResolveSeasonChain reads: id, format, title, and PREQUEL
// edges to other nodes.
type anilistMedia struct {
	id      int64
	format  string // TV, ONA, MOVIE, ...
	title   string
	prequel int64 // 0 = none; a TV prequel to walk
}

// newAniListChainServer returns a Pipeline whose AniList client answers the
// relations query from the given media map. Resolving an id walks PREQUEL edges
// to the root, mirroring real AniList behaviour.
func newAniListChainServer(t *testing.T, media map[int64]anilistMedia) *matching.Pipeline {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		idFloat, ok := req.Variables["id"].(float64)
		if !ok {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		m, found := media[int64(idFloat)]
		if !found {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		node := map[string]any{
			"id":        m.id,
			"format":    m.format,
			"title":     map[string]any{"english": m.title, "romaji": m.title},
			"relations": map[string]any{"edges": []any{}},
		}
		if m.prequel != 0 {
			node["relations"] = map[string]any{
				"edges": []any{
					map[string]any{
						"relationType": "PREQUEL",
						"node": map[string]any{
							"id":     m.prequel,
							"type":   "ANIME",
							"format": "TV",
							"title":  map[string]any{},
						},
					},
				},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"Media": node},
		})
	}))
	t.Cleanup(server.Close)

	anilist := matching.NewAniListClientWithURL(server.URL)
	return matching.NewPipeline(nil, anilist, nil, nil, t.TempDir())
}

func newSeasonAuditDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSeasonAuditService_Scan_ProposesSeasonForStray(t *testing.T) {
	db := newSeasonAuditDB(t)

	// AniList chain: root (id 10, IsRoot) ← season 2 (id 20, PREQUEL→10).
	pipeline := newAniListChainServer(t, map[int64]anilistMedia{
		10: {id: 10, format: "TV", title: "Root Series"},
		20: {id: 20, format: "TV", title: "Root Series Season 2", prequel: 10},
	})

	shared := "tt5550000"
	var rootAniList int64 = 10
	var seasonAniList int64 = 20
	rootID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2023,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		IMDBID:      &shared,
		AniListID:   &rootAniList,
	}, []model.TitleName{{Name: "Root Series", Language: "en", IsPrimary: true}})
	strayID := testutil.CreateTitle(t, db, &model.Title{
		Type:        model.TitleTypeSeries,
		IsAnime:     true,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
		IMDBID:      &shared,
		AniListID:   &seasonAniList,
	}, []model.TitleName{{Name: "Root Series Season 2", Language: "en", IsPrimary: true}})

	svc := newSeasonAuditService(db, pipeline)

	proposals, err := svc.Scan(t.Context())
	require.NoError(t, err)
	require.Len(t, proposals, 1)

	p := proposals[0]
	assert.Equal(t, strayID, p.SourceTitleID)
	assert.Equal(t, rootID, p.TargetTitleID)
	assert.Equal(t, 2, p.SeasonNumber)
	assert.Equal(t, "imdb:"+shared, p.SharedID)
	assert.Equal(t, "Root Series Season 2", p.SourceName)
	assert.Equal(t, "Root Series", p.TargetName)
}

func TestSeasonAuditService_Scan_ExcludesDismissed(t *testing.T) {
	db := newSeasonAuditDB(t)
	pipeline := newAniListChainServer(t, map[int64]anilistMedia{
		10: {id: 10, format: "TV", title: "Root"},
		20: {id: 20, format: "TV", title: "Root S2", prequel: 10},
	})

	shared := "tt6660000"
	var rootAniList int64 = 10
	var seasonAniList int64 = 20
	rootID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, IsAnime: true, Year: 2023,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
		IMDBID: &shared, AniListID: &rootAniList,
	}, []model.TitleName{{Name: "Root", Language: "en", IsPrimary: true}})
	strayID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, IsAnime: true, Year: 2024,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
		IMDBID: &shared, AniListID: &seasonAniList,
	}, []model.TitleName{{Name: "Root S2", Language: "en", IsPrimary: true}})

	svc := newSeasonAuditService(db, pipeline)
	require.NoError(t, svc.Dismiss(t.Context(), strayID, rootID))

	proposals, err := svc.Scan(t.Context())
	require.NoError(t, err)
	assert.Empty(t, proposals)
}

func TestSeasonAuditService_Accept_MergesAndRecordsEvent(t *testing.T) {
	db := newSeasonAuditDB(t)
	pipeline := newAniListChainServer(t, map[int64]anilistMedia{
		10: {id: 10, format: "TV", title: "Root"},
		20: {id: 20, format: "TV", title: "Root S2", prequel: 10},
	})

	shared := "tt7770000"
	var rootAniList int64 = 10
	var seasonAniList int64 = 20
	rootID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, IsAnime: true, Year: 2023,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
		IMDBID: &shared, AniListID: &rootAniList,
	}, []model.TitleName{{Name: "Root", Language: "en", IsPrimary: true}})
	rootS1 := testutil.GetOrCreateSeason(t, db, rootID, 1)
	require.NotNil(t, rootS1)

	strayID := testutil.CreateTitle(t, db, &model.Title{
		Type: model.TitleTypeSeries, IsAnime: true, Year: 2024,
		Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed,
		IMDBID: &shared, AniListID: &seasonAniList,
	}, []model.TitleName{{Name: "Root S2", Language: "en", IsPrimary: true}})
	// Stray carries its own "season 1" which becomes season 2 of the root.
	straySeason := testutil.GetOrCreateSeason(t, db, strayID, 1)
	ep := testutil.GetOrCreateEpisode(t, db, straySeason.ID, 1)
	_ = testutil.ToggleEpisodeWatched(t, db, ep.ID)

	svc := newSeasonAuditService(db, pipeline)
	require.NoError(t, svc.Accept(t.Context(), strayID, rootID, 2))

	titleRepo := repository.NewTitleRepository(db)

	// Source title is gone.
	_, err := titleRepo.GetByID(strayID)
	assert.Error(t, err)

	// Target now has season 1 (its own) + season 2 (from the stray).
	root, err := titleRepo.GetByID(rootID)
	require.NoError(t, err)
	require.Len(t, root.Seasons, 2)
	assert.Equal(t, 1, root.Seasons[0].SeasonNumber)
	assert.Equal(t, 2, root.Seasons[1].SeasonNumber)

	// Exactly one season_attached event on the target.
	events, err := repository.NewMatchEventRepository(db).ListRecent(t.Context(), 10)
	require.NoError(t, err)
	var attached int
	for _, e := range events {
		if e.Kind == model.MatchEventSeasonAttached {
			attached++
			require.NotNil(t, e.TitleID)
			assert.Equal(t, rootID, *e.TitleID)
		}
	}
	assert.Equal(t, 1, attached)
}

func newSeasonAuditService(db *sql.DB, pipeline *matching.Pipeline) *service.SeasonAuditService {
	titleRepo := repository.NewTitleRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	titleSvc := service.NewTitleService(db, titleRepo, taskRepo, pipeline)
	auditRepo := repository.NewSeasonAuditRepository(db)
	return service.NewSeasonAuditService(db, titleRepo, auditRepo, pipeline, titleSvc)
}
