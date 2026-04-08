package matching

import (
	"regexp"
	"strconv"
)

var (
	reIMDB    = regexp.MustCompile(`imdb\.com/(?:[a-z]{2}/)?title/(tt\d+)`)
	reAniList = regexp.MustCompile(`anilist\.co/anime/(\d+)`)
	reTMDB    = regexp.MustCompile(`themoviedb\.org/(movie|tv)/(\d+)`)
)

// ParseURL extracts external IDs from common media URLs.
func ParseURL(url string) *ExternalIDs {
	if m := reIMDB.FindStringSubmatch(url); m != nil {
		return &ExternalIDs{IMDB: m[1]}
	}
	if m := reTMDB.FindStringSubmatch(url); m != nil {
		id, _ := strconv.ParseInt(m[2], 10, 64)
		if m[1] == "movie" {
			return &ExternalIDs{TMDBMovie: id}
		}
		return &ExternalIDs{TMDBTV: id}
	}
	if m := reAniList.FindStringSubmatch(url); m != nil {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		return &ExternalIDs{AniList: id}
	}
	return nil
}
