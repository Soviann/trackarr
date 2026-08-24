package matching

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ExternalIDs holds all known external IDs for a title.
type ExternalIDs struct {
	IMDB      string
	TMDBMovie int64
	TMDBTV    int64
	TVDB      int64
	AniList   int64
	MAL       int64
}

// CrossRefDB provides ID cross-referencing using anime-offline-database JSON.
type CrossRefDB struct {
	// Indexes for fast lookup by any single ID.
	byAniList   map[int64]*ExternalIDs
	byMAL       map[int64]*ExternalIDs
	byTMDBMovie map[int64]*ExternalIDs
	byTMDBTV    map[int64]*ExternalIDs
	byTVDB      map[int64]*ExternalIDs
	byIMDB      map[string]*ExternalIDs
}

type offlineDBFile struct {
	Data []offlineDBEntry `json:"data"`
}

type offlineDBEntry struct {
	Sources  []string `json:"sources"`
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Episodes int      `json:"episodes"`
}

// LoadCrossRefDB loads the anime-offline-database JSON file and builds lookup indexes.
func LoadCrossRefDB(path string) (*CrossRefDB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open crossref db: %w", err)
	}
	defer f.Close()

	var db offlineDBFile
	if err := json.NewDecoder(f).Decode(&db); err != nil {
		return nil, fmt.Errorf("decode crossref db: %w", err)
	}

	cdb := &CrossRefDB{
		byAniList:   make(map[int64]*ExternalIDs),
		byMAL:       make(map[int64]*ExternalIDs),
		byTMDBMovie: make(map[int64]*ExternalIDs),
		byTMDBTV:    make(map[int64]*ExternalIDs),
		byTVDB:      make(map[int64]*ExternalIDs),
		byIMDB:      make(map[string]*ExternalIDs),
	}

	for _, entry := range db.Data {
		ids := parseSourceURLs(entry.Sources)
		if ids == nil {
			continue
		}
		if ids.AniList != 0 {
			cdb.byAniList[ids.AniList] = ids
		}
		if ids.MAL != 0 {
			cdb.byMAL[ids.MAL] = ids
		}
		if ids.TMDBMovie != 0 {
			cdb.byTMDBMovie[ids.TMDBMovie] = ids
		}
		if ids.TMDBTV != 0 {
			cdb.byTMDBTV[ids.TMDBTV] = ids
		}
		if ids.TVDB != 0 {
			cdb.byTVDB[ids.TVDB] = ids
		}
		if ids.IMDB != "" {
			cdb.byIMDB[ids.IMDB] = ids
		}
	}

	return cdb, nil
}

// Lookup takes any known subset of IDs and returns all cross-referenced IDs.
// Returns nil if no match is found.
func (cdb *CrossRefDB) Lookup(ids ExternalIDs) *ExternalIDs {
	if ids.TMDBMovie != 0 {
		if found, ok := cdb.byTMDBMovie[ids.TMDBMovie]; ok {
			return found
		}
	}
	if ids.TMDBTV != 0 {
		if found, ok := cdb.byTMDBTV[ids.TMDBTV]; ok {
			return found
		}
	}
	if ids.AniList != 0 {
		if found, ok := cdb.byAniList[ids.AniList]; ok {
			return found
		}
	}
	if ids.MAL != 0 {
		if found, ok := cdb.byMAL[ids.MAL]; ok {
			return found
		}
	}
	if ids.IMDB != "" {
		if found, ok := cdb.byIMDB[ids.IMDB]; ok {
			return found
		}
	}
	if ids.TVDB != 0 {
		if found, ok := cdb.byTVDB[ids.TVDB]; ok {
			return found
		}
	}
	return nil
}

func parseSourceURLs(sources []string) *ExternalIDs {
	ids := &ExternalIDs{}
	found := false

	for _, src := range sources {
		switch {
		case strings.Contains(src, "anilist.co"):
			if id := extractTrailingInt(src); id != 0 {
				ids.AniList = id
				found = true
			}
		case strings.Contains(src, "myanimelist.net"):
			if id := extractTrailingInt(src); id != 0 {
				ids.MAL = id
				found = true
			}
		case strings.Contains(src, "themoviedb.org"):
			if id := extractTrailingInt(src); id != 0 {
				if strings.Contains(src, "/movie/") {
					ids.TMDBMovie = id
				} else if strings.Contains(src, "/tv/") {
					ids.TMDBTV = id
				}
				found = true
			}
		case strings.Contains(src, "thetvdb.com"):
			if id := extractTrailingInt(src); id != 0 {
				ids.TVDB = id
				found = true
			}
		case strings.Contains(src, "imdb.com"):
			// Extract tt1234567 from URL
			parts := strings.Split(strings.TrimRight(src, "/"), "/")
			for _, p := range parts {
				if strings.HasPrefix(p, "tt") {
					ids.IMDB = p
					found = true
					break
				}
			}
		}
	}

	if !found {
		return nil
	}
	return ids
}

func extractTrailingInt(url string) int64 {
	url = strings.TrimRight(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return 0
	}
	id, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	return id
}
