package matching

import (
	"regexp"
	"strconv"
)

var (
	reIMDB    = regexp.MustCompile(`imdb\.com/(?:[a-z]{2}/)?title/(tt\d+)`)
	reAniList = regexp.MustCompile(`anilist\.co/anime/(\d+)`)
)

// ParseURL extracts external IDs from common media URLs.
func ParseURL(url string) *ExternalIDs {
	if m := reIMDB.FindStringSubmatch(url); m != nil {
		return &ExternalIDs{IMDB: m[1]}
	}
	if m := reAniList.FindStringSubmatch(url); m != nil {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		return &ExternalIDs{AniList: id}
	}
	return nil
}
