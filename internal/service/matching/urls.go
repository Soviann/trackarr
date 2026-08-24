package matching

import (
	"regexp"
	"strconv"
)

var (
	reIMDB       = regexp.MustCompile(`imdb\.com/(?:[a-z]{2}/)?title/(tt\d+)`)
	reAniList    = regexp.MustCompile(`anilist\.co/anime/(\d+)`)
	reTMDB       = regexp.MustCompile(`themoviedb\.org/(movie|tv)/(\d+)`)
	reTVDBSeries = regexp.MustCompile(`thetvdb\.com/series/([a-zA-Z0-9_-]+)`)
	reTVDBMovie  = regexp.MustCompile(`thetvdb\.com/movies/([a-zA-Z0-9_-]+)`)
)

// ParsedURL holds the result of parsing a media URL.
// TVDBSeriesSlug and TVDBMovieSlug require resolution to numeric IDs via the TVDB API.
type ParsedURL struct {
	ExternalIDs
	TVDBSeriesSlug string
	TVDBMovieSlug  string
}

// ParseURL extracts external IDs from common media URLs.
// For TVDB URLs, it returns a ParsedURL with the slug filled in
// (numeric ID resolution requires a TVDB API call — see Pipeline.ResolveURL).
func ParseURL(rawURL string) *ExternalIDs {
	p := ParseURLFull(rawURL)
	if p == nil {
		return nil
	}
	// Only return non-nil when we have a numeric ID (legacy callers)
	if p.IMDB != "" || p.TMDBMovie != 0 || p.TMDBTV != 0 || p.AniList != 0 ||
		p.TVDB != 0 || p.TVDBSeriesSlug != "" || p.TVDBMovieSlug != "" {
		ids := p.ExternalIDs
		return &ids
	}
	return nil
}

// ParseURLFull extracts all URL information including TVDB slugs.
func ParseURLFull(rawURL string) *ParsedURL {
	if m := reIMDB.FindStringSubmatch(rawURL); m != nil {
		return &ParsedURL{ExternalIDs: ExternalIDs{IMDB: m[1]}}
	}
	if m := reTMDB.FindStringSubmatch(rawURL); m != nil {
		id, _ := strconv.ParseInt(m[2], 10, 64)
		if m[1] == "movie" {
			return &ParsedURL{ExternalIDs: ExternalIDs{TMDBMovie: id}}
		}
		return &ParsedURL{ExternalIDs: ExternalIDs{TMDBTV: id}}
	}
	if m := reAniList.FindStringSubmatch(rawURL); m != nil {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		return &ParsedURL{ExternalIDs: ExternalIDs{AniList: id}}
	}
	if m := reTVDBMovie.FindStringSubmatch(rawURL); m != nil {
		return &ParsedURL{TVDBMovieSlug: m[1]}
	}
	if m := reTVDBSeries.FindStringSubmatch(rawURL); m != nil {
		return &ParsedURL{TVDBSeriesSlug: m[1]}
	}
	return nil
}
