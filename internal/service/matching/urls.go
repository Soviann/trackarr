package matching

import (
	"regexp"
	"strconv"
)

var (
	reIMDB    = regexp.MustCompile(`imdb\.com/title/(tt\d+)`)
	reAniList = regexp.MustCompile(`anilist\.co/anime/(\d+)`)
	reTVDB    = regexp.MustCompile(`thetvdb\.com/(series|movies)/([a-z0-9-]+)`)
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
	// TVDB slugs are not IDs, we'd need another lookup, but let's at least detect them
	if m := reTVDB.FindStringSubmatch(url); m != nil {
		// For now we don't return anything as we can't easily get the ID from slug here
		// but we might want to return the slug in a different field if needed.
	}
	return nil
}
