package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// relationStub is a map from media id to a canned relationMedia response used
// by the test server.
type relationStub map[int64]relationMedia

func newRelationsTestServer(t *testing.T, stubs relationStub) *AniListClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Extract the id variable.
		idRaw, ok := req.Variables["id"]
		if !ok {
			http.Error(w, "missing id variable", http.StatusBadRequest)
			return
		}
		// Variables are decoded as float64 by encoding/json.
		idFloat, ok := idRaw.(float64)
		if !ok {
			http.Error(w, "id is not a number", http.StatusBadRequest)
			return
		}
		id := int64(idFloat)

		media, found := stubs[id]
		if !found {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"Media": media,
			},
		})
	}))
	t.Cleanup(server.Close)
	return NewAniListClientWithURL(server.URL)
}

// prequelEdge describes a PREQUEL edge to embed in a media node.
type prequelEdge struct {
	id     int64
	format string // format of the prequel node (TV, ONA, MOVIE, MANGA, ...)
}

// makeMedia builds a relationMedia with the given format/type/title and optional
// PREQUEL edges. Each prequelEdge carries the node's format declaratively so
// pickPrequel's format-based priority is exercised without post-hoc patching.
func makeMedia(id int64, format, nodeType, title string, prequels ...prequelEdge) relationMedia {
	m := relationMedia{
		ID:     id,
		Format: format,
	}
	m.Title.English = title
	for _, p := range prequels {
		edge := struct {
			RelationType string `json:"relationType"`
			Node         struct {
				ID     int64  `json:"id"`
				Type   string `json:"type"`
				Format string `json:"format"`
				Title  struct {
					Romaji  string `json:"romaji"`
					English string `json:"english"`
				} `json:"title"`
			} `json:"node"`
		}{}
		edge.RelationType = "PREQUEL"
		edge.Node.ID = p.id
		edge.Node.Type = "ANIME"
		edge.Node.Format = p.format
		m.Relations.Edges = append(m.Relations.Edges, edge)
	}
	return m
}

// makeTV builds a TV-format relationMedia with the given PREQUEL edges.
func makeTV(id int64, title string, prequels ...prequelEdge) relationMedia {
	return makeMedia(id, "TV", "ANIME", title, prequels...)
}

// makeONA builds an ONA-format relationMedia with the given PREQUEL edges.
func makeONA(id int64, title string, prequels ...prequelEdge) relationMedia {
	return makeMedia(id, "ONA", "ANIME", title, prequels...)
}

// makeMovie builds a MOVIE-format relationMedia with the given PREQUEL edges.
func makeMovie(id int64, title string, prequels ...prequelEdge) relationMedia {
	return makeMedia(id, "MOVIE", "ANIME", title, prequels...)
}

// makeRomaji builds a TV-format relationMedia with only a romaji title (no english).
func makeRomaji(id int64, romaji string, prequels ...prequelEdge) relationMedia {
	m := makeTV(id, "", prequels...)
	m.Title.Romaji = romaji
	return m
}

// TestResolveSeasonChain_LinearChain: 30→20→10, all TV.
// Resolving 30 → SeasonNumber=3, root=10. Resolving 10 → SeasonNumber=1, IsRoot=true.
func TestResolveSeasonChain_LinearChain(t *testing.T) {
	id10 := makeTV(10, "Season One") // no prequel
	id20 := makeTV(20, "Season Two", prequelEdge{10, "TV"})
	id30 := makeTV(30, "Season Three", prequelEdge{20, "TV"})

	stubs := relationStub{10: id10, 20: id20, 30: id30}
	client := newRelationsTestServer(t, stubs)

	t.Run("resolving_30", func(t *testing.T) {
		chain, err := client.ResolveSeasonChain(context.Background(), 30)
		require.NoError(t, err)
		assert.Equal(t, int64(10), chain.RootID)
		assert.Equal(t, "Season One", chain.RootTitle)
		assert.Equal(t, 3, chain.SeasonNumber)
		assert.False(t, chain.IsRoot)
		assert.True(t, chain.RootIsSeries)
	})

	t.Run("resolving_10", func(t *testing.T) {
		chain, err := client.ResolveSeasonChain(context.Background(), 10)
		require.NoError(t, err)
		assert.Equal(t, int64(10), chain.RootID)
		assert.Equal(t, "Season One", chain.RootTitle)
		assert.Equal(t, 1, chain.SeasonNumber)
		assert.True(t, chain.IsRoot)
		assert.True(t, chain.RootIsSeries)
	})
}

// TestResolveSeasonChain_RootTitleRomajiFallback: root has no English title,
// only romaji — RootTitle must use the romaji value.
func TestResolveSeasonChain_RootTitleRomajiFallback(t *testing.T) {
	// root has romaji only
	id10 := makeRomaji(10, "Shingeki no Kyojin")
	id20 := makeTV(20, "Attack on Titan Season 2", prequelEdge{10, "TV"})

	stubs := relationStub{10: id10, 20: id20}
	client := newRelationsTestServer(t, stubs)

	chain, err := client.ResolveSeasonChain(context.Background(), 20)
	require.NoError(t, err)
	assert.Equal(t, int64(10), chain.RootID)
	assert.Equal(t, "Shingeki no Kyojin", chain.RootTitle)
	assert.Equal(t, 2, chain.SeasonNumber)
	assert.False(t, chain.IsRoot)
	assert.True(t, chain.RootIsSeries)
}

// TestResolveSeasonChain_MovieInterleaved: 40(TV)→35(MOVIE)→10(TV).
// Movie is traversed but NOT counted; season ordinal = 2.
func TestResolveSeasonChain_MovieInterleaved(t *testing.T) {
	id10 := makeTV(10, "Season One") // no prequel

	// id35 is a MOVIE with a PREQUEL edge to id10 (TV)
	id35 := makeMovie(35, "Recap Movie", prequelEdge{10, "TV"})

	// id40 is TV with PREQUEL edge to id35 (MOVIE)
	id40 := makeTV(40, "Season Two", prequelEdge{35, "MOVIE"})

	stubs := relationStub{10: id10, 35: id35, 40: id40}
	client := newRelationsTestServer(t, stubs)

	chain, err := client.ResolveSeasonChain(context.Background(), 40)
	require.NoError(t, err)
	assert.Equal(t, int64(10), chain.RootID)
	assert.Equal(t, 2, chain.SeasonNumber)
	assert.False(t, chain.IsRoot)
	assert.True(t, chain.RootIsSeries)
}

// TestResolveSeasonChain_ONACounts: 52(TV)→51(ONA)→50(TV). ONA counts as a season.
// Resolving 52 → SeasonNumber=3.
func TestResolveSeasonChain_ONACounts(t *testing.T) {
	id50 := makeTV(50, "Root Series") // no prequel
	id51 := makeONA(51, "ONA Season", prequelEdge{50, "TV"})
	id52 := makeTV(52, "Third Season", prequelEdge{51, "ONA"})

	stubs := relationStub{50: id50, 51: id51, 52: id52}
	client := newRelationsTestServer(t, stubs)

	chain, err := client.ResolveSeasonChain(context.Background(), 52)
	require.NoError(t, err)
	assert.Equal(t, int64(50), chain.RootID)
	assert.Equal(t, 3, chain.SeasonNumber)
	assert.False(t, chain.IsRoot)
	assert.True(t, chain.RootIsSeries)
}

// TestResolveSeasonChain_CycleGuard: 60→61→60. Must return an error containing
// "cycle" and must not hang.
func TestResolveSeasonChain_CycleGuard(t *testing.T) {
	id60 := makeTV(60, "Cycle A", prequelEdge{61, "TV"})
	id61 := makeTV(61, "Cycle B", prequelEdge{60, "TV"})

	stubs := relationStub{60: id60, 61: id61}
	client := newRelationsTestServer(t, stubs)

	_, err := client.ResolveSeasonChain(context.Background(), 60)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

// TestResolveSeasonChain_NonSeriesStart: resolving a MOVIE id returns
// {IsRoot:true, SeasonNumber:1, RootIsSeries:false} immediately — movies are never seasons.
func TestResolveSeasonChain_NonSeriesStart(t *testing.T) {
	id99 := makeMovie(99, "A Great Film")

	stubs := relationStub{99: id99}
	client := newRelationsTestServer(t, stubs)

	chain, err := client.ResolveSeasonChain(context.Background(), 99)
	require.NoError(t, err)
	assert.Equal(t, int64(99), chain.RootID)
	assert.Equal(t, 1, chain.SeasonNumber)
	assert.True(t, chain.IsRoot)
	assert.False(t, chain.RootIsSeries)
}

// TestPickPrequel_Preference: a node with three PREQUEL edges (TV, ONA, MOVIE) must
// walk the TV one first; then without TV only the ONA; then only the MOVIE.
func TestPickPrequel_Preference(t *testing.T) {
	// Build a media node with three PREQUEL edges in ONA/MOVIE/TV order
	// to confirm TV is preferred regardless of declaration order.
	m := makeMedia(1, "TV", "ANIME", "Start",
		prequelEdge{101, "ONA"},
		prequelEdge{102, "MOVIE"},
		prequelEdge{103, "TV"},
	)
	assert.Equal(t, int64(103), pickPrequel(&m), "TV prequel must be preferred")

	// Without TV: ONA wins over MOVIE.
	m2 := makeMedia(1, "TV", "ANIME", "Start",
		prequelEdge{101, "ONA"},
		prequelEdge{102, "MOVIE"},
	)
	assert.Equal(t, int64(101), pickPrequel(&m2), "ONA prequel must be preferred over MOVIE")

	// MANGA prequel must be entirely ignored (node type is ANIME but format MANGA
	// is not a meaningful ANIME format — pickPrequel accepts any ANIME type node
	// that is not TV/ONA as "other", so test the real case: edge with Type=MANGA
	// must be skipped because pickPrequel filters on Node.Type == "ANIME").
	mMangaOnly := makeMedia(1, "TV", "ANIME", "Start")
	mMangaOnly.Relations.Edges = append(mMangaOnly.Relations.Edges, struct {
		RelationType string `json:"relationType"`
		Node         struct {
			ID     int64  `json:"id"`
			Type   string `json:"type"`
			Format string `json:"format"`
			Title  struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
			} `json:"title"`
		} `json:"node"`
	}{
		RelationType: "PREQUEL",
		Node: struct {
			ID     int64  `json:"id"`
			Type   string `json:"type"`
			Format string `json:"format"`
			Title  struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
			} `json:"title"`
		}{ID: 200, Type: "MANGA", Format: "MANGA"},
	})
	assert.Equal(t, int64(0), pickPrequel(&mMangaOnly), "MANGA-type prequel must be ignored; node treated as root")
}

// TestResolveSeasonChain_MangaOnlyPrequel: a TV entry whose only PREQUEL edge
// is a MANGA source — pickPrequel must return 0 and the entry becomes its own root.
func TestResolveSeasonChain_MangaOnlyPrequel(t *testing.T) {
	idTV := makeMedia(70, "TV", "ANIME", "Manga Adaptation")
	idTV.Relations.Edges = append(idTV.Relations.Edges, struct {
		RelationType string `json:"relationType"`
		Node         struct {
			ID     int64  `json:"id"`
			Type   string `json:"type"`
			Format string `json:"format"`
			Title  struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
			} `json:"title"`
		} `json:"node"`
	}{
		RelationType: "PREQUEL",
		Node: struct {
			ID     int64  `json:"id"`
			Type   string `json:"type"`
			Format string `json:"format"`
			Title  struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
			} `json:"title"`
		}{ID: 71, Type: "MANGA", Format: "MANGA"},
	})

	stubs := relationStub{70: idTV}
	client := newRelationsTestServer(t, stubs)

	chain, err := client.ResolveSeasonChain(context.Background(), 70)
	require.NoError(t, err)
	assert.Equal(t, int64(70), chain.RootID)
	assert.Equal(t, 1, chain.SeasonNumber)
	assert.True(t, chain.IsRoot)
	assert.True(t, chain.RootIsSeries)
}

// TestResolveSeasonChain_DepthCap: a linear chain longer than maxChainDepth must
// return an error containing "too deep".
func TestResolveSeasonChain_DepthCap(t *testing.T) {
	const chainLen = maxChainDepth + 2 // just over the cap
	stubs := make(relationStub, chainLen)

	// Build IDs 1..chainLen. Node 1 has no prequel. Node i+1 has prequel i.
	stubs[1] = makeTV(1, fmt.Sprintf("Season %d", 1))
	for i := int64(2); i <= chainLen; i++ {
		stubs[i] = makeTV(i, fmt.Sprintf("Season %d", i), prequelEdge{i - 1, "TV"})
	}

	client := newRelationsTestServer(t, stubs)

	// Resolve from the last node in the chain — must exceed depth cap.
	_, err := client.ResolveSeasonChain(context.Background(), int64(chainLen))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too deep")
}

// TestResolveSeasonChain_TVChainWithMovieRoot: a TV season whose prequel chain
// ends at a MOVIE root. RootIsSeries must be false; the season is still located
// at SeasonNumber=1 (itself is TV but root is MOVIE, chain has no TV prequels).
func TestResolveSeasonChain_TVChainWithMovieRoot(t *testing.T) {
	// id80: MOVIE (root, no prequel)
	id80 := makeMovie(80, "The Movie")
	// id81: TV with PREQUEL edge to id80 (MOVIE)
	id81 := makeTV(81, "TV Season After Movie", prequelEdge{80, "MOVIE"})

	stubs := relationStub{80: id80, 81: id81}
	client := newRelationsTestServer(t, stubs)

	chain, err := client.ResolveSeasonChain(context.Background(), 81)
	require.NoError(t, err)
	// Root is the MOVIE node.
	assert.Equal(t, int64(80), chain.RootID)
	// id80 is MOVIE → not a series format → RootIsSeries=false.
	assert.False(t, chain.RootIsSeries)
	// id81 is TV but id80 (MOVIE) is not a series, so seasons count = 1 (id81 itself).
	assert.Equal(t, 1, chain.SeasonNumber)
	assert.False(t, chain.IsRoot)
}
