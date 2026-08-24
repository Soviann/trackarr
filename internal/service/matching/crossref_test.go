package matching

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDBJSON = `{
  "data": [
    {
      "sources": [
        "https://anilist.co/anime/21",
        "https://myanimelist.net/anime/21",
        "https://www.themoviedb.org/tv/46298",
        "https://www.thetvdb.com/series/85004",
        "https://www.imdb.com/title/tt0388629/"
      ],
      "title": "One Piece",
      "type": "TV",
      "episodes": 1000
    },
    {
      "sources": [
        "https://anilist.co/anime/16498",
        "https://myanimelist.net/anime/16498",
        "https://www.themoviedb.org/tv/1429"
      ],
      "title": "Shingeki no Kyojin",
      "type": "TV",
      "episodes": 25
    },
    {
      "sources": [
        "https://example.com/unknown"
      ],
      "title": "Unknown Source",
      "type": "TV",
      "episodes": 1
    }
  ]
}`

func newTestCrossRefDB(t *testing.T) *CrossRefDB {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "anime-offline-database.json")
	require.NoError(t, os.WriteFile(path, []byte(testDBJSON), 0o644))

	db, err := LoadCrossRefDB(path)
	require.NoError(t, err)
	return db
}

func TestCrossRefLookupByAniList(t *testing.T) {
	db := newTestCrossRefDB(t)

	ids := db.Lookup(ExternalIDs{AniList: 21})
	require.NotNil(t, ids)
	assert.Equal(t, int64(21), ids.AniList)
	assert.Equal(t, int64(21), ids.MAL)
	assert.Equal(t, int64(46298), ids.TMDBTV)
	assert.Equal(t, int64(85004), ids.TVDB)
	assert.Equal(t, "tt0388629", ids.IMDB)
}

func TestCrossRefLookupByIMDB(t *testing.T) {
	db := newTestCrossRefDB(t)

	ids := db.Lookup(ExternalIDs{IMDB: "tt0388629"})
	require.NotNil(t, ids)
	assert.Equal(t, int64(21), ids.AniList)
}

func TestCrossRefLookupByTMDB(t *testing.T) {
	db := newTestCrossRefDB(t)

	ids := db.Lookup(ExternalIDs{TMDBTV: 1429})
	require.NotNil(t, ids)
	assert.Equal(t, int64(16498), ids.AniList)
	assert.Equal(t, int64(16498), ids.MAL)
	assert.Equal(t, "", ids.IMDB) // not in test data
}

func TestCrossRefLookupByMAL(t *testing.T) {
	db := newTestCrossRefDB(t)

	ids := db.Lookup(ExternalIDs{MAL: 16498})
	require.NotNil(t, ids)
	assert.Equal(t, int64(16498), ids.AniList)
}

func TestCrossRefLookupByTVDB(t *testing.T) {
	db := newTestCrossRefDB(t)

	ids := db.Lookup(ExternalIDs{TVDB: 85004})
	require.NotNil(t, ids)
	assert.Equal(t, int64(21), ids.AniList)
}

func TestCrossRefLookupNotFound(t *testing.T) {
	db := newTestCrossRefDB(t)

	ids := db.Lookup(ExternalIDs{AniList: 99999})
	assert.Nil(t, ids)
}

func TestCrossRefLookupEmptyIDs(t *testing.T) {
	db := newTestCrossRefDB(t)

	ids := db.Lookup(ExternalIDs{})
	assert.Nil(t, ids)
}

func TestCrossRefLoadMissingFile(t *testing.T) {
	_, err := LoadCrossRefDB("/nonexistent/file.json")
	assert.Error(t, err)
}

func TestParseSourceURLs(t *testing.T) {
	// Entry with only unknown sources returns nil
	ids := parseSourceURLs([]string{"https://example.com/unknown"})
	assert.Nil(t, ids)

	// Empty sources
	ids = parseSourceURLs([]string{})
	assert.Nil(t, ids)
}
