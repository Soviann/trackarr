package model

import (
	"encoding/json"
	"strconv"
	"strings"
)

type PlexPayload struct {
	Event    string       `json:"event"`
	User     bool         `json:"user"`
	Owner    bool         `json:"owner"`
	Account  PlexAccount  `json:"Account"`
	Server   PlexServer   `json:"Server"`
	Player   PlexPlayer   `json:"Player"`
	Metadata PlexMetadata `json:"Metadata"`
}

type PlexAccount struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type PlexServer struct {
	Title string `json:"title"`
	UUID  string `json:"uuid"`
}

type PlexPlayer struct {
	Local         bool   `json:"local"`
	PublicAddress string `json:"publicAddress"`
	Title         string `json:"title"`
	UUID          string `json:"uuid"`
}

type PlexMetadata struct {
	RatingKey            string          `json:"ratingKey"`
	Key                  string          `json:"key"`
	ParentRatingKey      string          `json:"parentRatingKey"`
	GrandparentRatingKey string          `json:"grandparentRatingKey"`
	Type                 string          `json:"type"`
	Title                string          `json:"title"`
	ParentTitle          string          `json:"parentTitle"`
	GrandparentTitle     string          `json:"grandparentTitle"`
	OriginalTitle        string          `json:"originalTitle"`
	Year                 int             `json:"year"`
	Index                int             `json:"index"`       // episode number
	ParentIndex          int             `json:"parentIndex"` // season number
	Duration             int64           `json:"duration"`
	ViewOffset           int64           `json:"viewOffset"`
	Summary              string          `json:"summary"`
	Guid                 []PlexGUIDItem  `json:"Guid"`
	RawGUID              json.RawMessage `json:"guid"`
}

type PlexGUIDItem struct {
	ID     string `json:"id"`
	Scheme string `json:"Scheme"`
	Host   string `json:"Host"`
	Path   string `json:"Path"`
}

// ExtractExternalIDs extracts IMDb, TMDb, and TVDb IDs from Plex metadata.
func (m *PlexMetadata) ExtractExternalIDs() (imdb string, tmdb int64, tvdb int64) {
	for _, item := range m.Guid {
		if item.ID != "" {
			parseGUIDString(item.ID, &imdb, &tmdb, &tvdb)
		}
		if item.Scheme != "" {
			val := item.Host
			if val == "" {
				val = strings.TrimPrefix(item.Path, "/")
			}
			switch strings.ToLower(item.Scheme) {
			case "imdb":
				if imdb == "" {
					imdb = val
				}
			case "tmdb":
				if tmdb == 0 {
					if n, err := strconv.ParseInt(val, 10, 64); err == nil {
						tmdb = n
					}
				}
			case "tvdb":
				if tvdb == 0 {
					if n, err := strconv.ParseInt(val, 10, 64); err == nil {
						tvdb = n
					}
				}
			}
		}
	}

	// Also check raw string guid if it's a JSON string
	if len(m.RawGUID) > 0 {
		var str string
		if err := json.Unmarshal(m.RawGUID, &str); err == nil && str != "" {
			parseGUIDString(str, &imdb, &tmdb, &tvdb)
		}
	}

	return imdb, tmdb, tvdb
}

func parseGUIDString(guid string, imdb *string, tmdb *int64, tvdb *int64) {
	guid = strings.TrimSpace(guid)
	switch {
	case strings.HasPrefix(guid, "imdb://"):
		val := strings.TrimPrefix(guid, "imdb://")
		val = strings.Split(val, "?")[0]
		if *imdb == "" {
			*imdb = val
		}
	case strings.HasPrefix(guid, "tmdb://"):
		val := strings.TrimPrefix(guid, "tmdb://")
		val = strings.Split(val, "?")[0]
		if *tmdb == 0 {
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				*tmdb = n
			}
		}
	case strings.HasPrefix(guid, "tvdb://"):
		val := strings.TrimPrefix(guid, "tvdb://")
		val = strings.Split(val, "?")[0]
		if *tvdb == 0 {
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				*tvdb = n
			}
		}
	case strings.HasPrefix(guid, "com.plexapp.agents.imdb://"):
		val := strings.TrimPrefix(guid, "com.plexapp.agents.imdb://")
		val = strings.Split(val, "?")[0]
		if *imdb == "" {
			*imdb = val
		}
	case strings.HasPrefix(guid, "com.plexapp.agents.themoviedb://"):
		val := strings.TrimPrefix(guid, "com.plexapp.agents.themoviedb://")
		val = strings.Split(val, "?")[0]
		if *tmdb == 0 {
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				*tmdb = n
			}
		}
	case strings.HasPrefix(guid, "com.plexapp.agents.thetvdb://"):
		val := strings.TrimPrefix(guid, "com.plexapp.agents.thetvdb://")
		val = strings.Split(val, "?")[0]
		if *tvdb == 0 {
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				*tvdb = n
			}
		}
	}
}
