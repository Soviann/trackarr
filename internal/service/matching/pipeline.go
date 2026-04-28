package matching

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// Match source constants track which pipeline step produced the match.
const (
	MatchSourcePlexIDs       = "plex_ids"
	MatchSourceCrossRef      = "crossref"
	MatchSourceTMDBSearch    = "tmdb_search"
	MatchSourceAniListSearch = "anilist_search"
	MatchSourceGeminiFuzzy   = "gemini_fuzzy"
	MatchSourceManual        = "manual"
	MatchSourceNone          = "none"
)

// Confidence levels returned by Gemini verification/resolution.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// Pipeline orchestrates the media matching process through an ordered chain of
// MatchStrategy steps. The first strategy to return matched=true wins.
type Pipeline struct {
	tmdb       *TMDBClient
	tvdb       *TVDBClient
	anilist    *AniListClient
	gemini     *GeminiClient
	crossDB    *CrossRefDB // may be nil if not loaded
	dataDir    string
	strategies []MatchStrategy
}

func NewPipeline(tmdb *TMDBClient, anilist *AniListClient, gemini *GeminiClient, crossDB *CrossRefDB, dataDir string) *Pipeline {
	p := &Pipeline{
		tmdb:    tmdb,
		anilist: anilist,
		gemini:  gemini,
		crossDB: crossDB,
		dataDir: dataDir,
	}
	p.strategies = []MatchStrategy{
		&plexIDStrategy{p: p},
		&crossRefStrategy{p: p},
		&tmdbSearchStrategy{p: p},
		&aniListSearchStrategy{p: p},
		&geminiFuzzyStrategy{p: p},
	}
	return p
}

// SetTVDB injects the TVDB client into the pipeline.
func (p *Pipeline) SetTVDB(tvdb *TVDBClient) { p.tvdb = tvdb }

// TMDB returns the underlying TMDB client.
func (p *Pipeline) TMDB() *TMDBClient { return p.tmdb }

// TVDB returns the underlying TVDB client.
func (p *Pipeline) TVDB() *TVDBClient { return p.tvdb }

// MatchResult holds the outcome of running the matching pipeline.
type MatchResult struct {
	IMDBID      string            `json:"imdb_id"`
	TMDBID      int64             `json:"tmdb_id"`
	TVDBID      int64             `json:"tvdb_id"`
	AniListID   int64             `json:"anilist_id"`
	MatchStatus model.MatchStatus `json:"match_status"`
	MatchSource string            `json:"match_source"` // which pipeline step produced the match
	Names       []model.TitleName `json:"names"`        // multilingual names
	CoverFile   string            `json:"cover_file"`   // local filename in covers dir
	TitleType   model.TitleType   `json:"type"`         // resolved type (movie or series)
	IsAnime     bool              `json:"is_anime"`
	// TMDB/TVDB metadata
	Overview      string   `json:"overview"`
	Genres        []string `json:"genres"`
	Runtime       *int     `json:"runtime"`
	TMDBRating    *float64 `json:"tmdb_rating"`
	Credits       string   `json:"credits"` // JSON array
	AniListRating *int     `json:"anilist_rating"`
	ReleaseDate   string   `json:"release_date"`
}

// MatchInput holds the info needed to start the matching pipeline.
type MatchInput struct {
	Title string
	Year  int
	Type  model.TitleType
	// IDs already known from Plex metadata
	IMDBID    string
	TMDBID    int64
	TVDBID    int64
	AniListID int64
	// Force anime detection (e.g. from Simkl section)
	IsAnime bool
}

// Run executes the matching pipeline by iterating over p.strategies in order.
// Each strategy is self-contained: nil clients are handled inside Try (skipped
// silently). The first strategy that returns matched=true wins; if every
// strategy passes, the fallback is an unconfirmed result keyed on the input
// title.
func (p *Pipeline) Run(ctx context.Context, input MatchInput) (*MatchResult, error) {
	for _, s := range p.strategies {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result, matched, err := s.Try(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Name(), err)
		}
		if matched {
			return result, nil
		}
	}
	return unmatchedResult(input), nil
}

// ResolveURL attempts to identify a title directly from an external URL.
func (p *Pipeline) ResolveURL(ctx context.Context, rawURL string) (*MatchResult, error) {
	parsed := ParseURLFull(rawURL)
	if parsed == nil {
		return nil, fmt.Errorf("could not parse URL: %s", rawURL)
	}

	// Resolve TVDB slugs to numeric IDs
	if parsed.TVDBSeriesSlug != "" {
		if p.tvdb == nil {
			return nil, fmt.Errorf("TVDB URL not resolvable: TVDB client not configured")
		}
		details, err := p.tvdb.GetSeriesBySlug(ctx, parsed.TVDBSeriesSlug)
		if err != nil {
			return nil, fmt.Errorf("could not resolve TVDB series slug %q: %w", parsed.TVDBSeriesSlug, err)
		}
		return p.Run(ctx, MatchInput{
			TVDBID: details.ID,
			Type:   model.TitleTypeSeries,
		})
	}

	if parsed.TVDBMovieSlug != "" {
		if p.tvdb == nil {
			return nil, fmt.Errorf("TVDB URL not resolvable: TVDB client not configured")
		}
		details, err := p.tvdb.GetMovieBySlug(ctx, parsed.TVDBMovieSlug)
		if err != nil {
			return nil, fmt.Errorf("could not resolve TVDB movie slug %q: %w", parsed.TVDBMovieSlug, err)
		}
		return p.Run(ctx, MatchInput{
			TVDBID: details.ID,
			Type:   model.TitleTypeMovie,
		})
	}

	input := MatchInput{
		IMDBID:    parsed.IMDB,
		AniListID: parsed.AniList,
	}

	if parsed.TMDBMovie != 0 {
		input.TMDBID = parsed.TMDBMovie
		input.Type = model.TitleTypeMovie
	} else if parsed.TMDBTV != 0 {
		input.TMDBID = parsed.TMDBTV
		input.Type = model.TitleTypeSeries
	}

	return p.Run(ctx, input)
}

func (p *Pipeline) searchTMDB(ctx context.Context, input MatchInput, result *MatchResult) bool {
	var searchResults []TMDBSearchResult
	var err error

	if input.Type == model.TitleTypeMovie {
		searchResults, err = p.tmdb.SearchMovie(ctx, input.Title, input.Year)
	} else {
		searchResults, err = p.tmdb.SearchTV(ctx, input.Title, input.Year)
	}

	if err != nil {
		log.Printf("TMDB search failed: %v", err)
		return false
	}

	if len(searchResults) == 0 {
		return false
	}

	// Take the first result
	best := searchResults[0]
	result.TMDBID = best.ID
	return true
}

func (p *Pipeline) searchAniList(ctx context.Context, input MatchInput, result *MatchResult) bool {
	results, err := p.anilist.SearchAnime(ctx, input.Title)
	if err != nil {
		log.Printf("AniList search failed: %v", err)
		return false
	}

	if len(results) == 0 {
		return false
	}

	best := results[0]
	result.AniListID = best.ID
	return true
}

func (p *Pipeline) IdentifyAnimeSeason(ctx context.Context, title string, year int) (*AnimeSeasonIdentification, error) {
	if p.gemini == nil {
		return nil, fmt.Errorf("gemini not configured")
	}
	return p.gemini.IdentifyAnimeSeason(ctx, title, year)
}

// SearchAniListByName looks up the top AniList match for a name and returns
// its ID (or 0 when the client is not configured or no result is returned).
// Used by the anime-merge flow to recover a per-season AniList ID when the
// source title lacks one. Errors propagate to the caller so a network blip
// doesn't silently mask a missing mapping.
func (p *Pipeline) SearchAniListByName(ctx context.Context, title string) (int64, error) {
	if p.anilist == nil {
		return 0, nil
	}
	results, err := p.anilist.SearchAnime(ctx, title)
	if err != nil {
		return 0, fmt.Errorf("search anilist by name: %w", err)
	}
	if len(results) == 0 {
		return 0, nil
	}
	return results[0].ID, nil
}

func (p *Pipeline) verifyAndEnrich(ctx context.Context, input MatchInput, result *MatchResult) (*MatchResult, error) {
	if p.gemini != nil {
		// Build candidate info from TMDB
		candidateTitle := input.Title
		candidateYear := input.Year

		if p.tmdb != nil && result.TMDBID != 0 {
			if input.Type == model.TitleTypeMovie {
				if details, err := p.tmdb.GetMovieDetails(ctx, result.TMDBID); err == nil {
					candidateTitle = details.Title
					if sr := details.ExternalIDs; sr != nil {
						if sr.IMDBID != "" {
							result.IMDBID = sr.IMDBID
						}
					}
					if details.IMDBID != "" {
						result.IMDBID = details.IMDBID
					}
					candidateYear = TMDBSearchResult{ReleaseDate: details.ReleaseDate}.Year()
				}
			} else {
				if details, err := p.tmdb.GetTVDetails(ctx, result.TMDBID); err == nil {
					candidateTitle = details.Name
					if sr := details.ExternalIDs; sr != nil {
						if sr.IMDBID != "" {
							result.IMDBID = sr.IMDBID
						}
						if sr.TVDBID != 0 {
							result.TVDBID = sr.TVDBID
						}
					}
					candidateYear = TMDBSearchResult{FirstAirDate: details.FirstAirDate}.Year()
				}
			}
		}

		verification, err := p.gemini.VerifyMatch(
			ctx,
			PlexInfo{Title: input.Title, Year: input.Year, Type: string(input.Type)},
			MatchCandidate{Title: candidateTitle, Year: candidateYear, TMDBID: result.TMDBID, IMDBID: result.IMDBID},
		)
		switch {
		case err != nil:
			log.Printf("gemini verification failed: %v", err)
			result.MatchStatus = model.MatchStatusUnconfirmed
		case verification.Confirmed && verification.Confidence == ConfidenceHigh:
			result.MatchStatus = model.MatchStatusPendingReview
		default:
			result.MatchStatus = model.MatchStatusUnconfirmed
		}
	} else {
		result.MatchStatus = model.MatchStatusPendingReview
	}

	p.enrichFromIDs(ctx, result, input)
	return result, nil
}

// enrichFromIDs fetches multilingual names, covers, and cross-references.
func (p *Pipeline) enrichFromIDs(ctx context.Context, result *MatchResult, input MatchInput) {
	p.resolveIDsFromSources(ctx, result, input)
	p.appendAniListRomaji(ctx, result)

	// Fallback: use input title if no names found
	if len(result.Names) == 0 {
		result.Names = []model.TitleName{{Name: input.Title, Language: "en", IsPrimary: true}}
	}

	tmdbRes, tvdbRes := p.fetchTMDBAndTVDBParallel(ctx, result)
	mergeMetadata(result, tmdbRes, tvdbRes)
	reconcileIDs(result, tmdbRes, tvdbRes)

	// AniList cover as last resort
	if result.CoverFile == "" && p.anilist != nil && result.AniListID != 0 {
		p.downloadAniListCover(ctx, result)
	}
}

// mergeMetadata fuses TMDB and TVDB fetch results into MatchResult metadata
// fields, applying these precedence rules:
//
//   - Overview: longest wins (TMDB on tie).
//   - Genres: case-insensitive union (TMDB ordering first, TVDB appended).
//   - Runtime / ReleaseDate / Cover: TMDB preferred, TVDB fallback.
//   - TMDBRating, Credits: TMDB only (TVDB does not provide these).
//   - IsAnime: stays true if already set; promoted to true when TVDB tags it.
//   - Names: union of TMDB+TVDB language maps (TMDB wins on duplicate language);
//     "en" entries are flagged IsPrimary.
func mergeMetadata(result *MatchResult, tmdbRes tmdbFetchResult, tvdbRes tvdbFetchResult) {
	if tmdbRes.overview != "" || tvdbRes.overview != "" {
		if len(tvdbRes.overview) > len(tmdbRes.overview) {
			result.Overview = tvdbRes.overview
		} else if tmdbRes.overview != "" {
			result.Overview = tmdbRes.overview
		}
	}

	result.Genres = mergeGenres(tmdbRes.genres, tvdbRes.genres)

	if tmdbRes.runtime != nil {
		result.Runtime = tmdbRes.runtime
	} else if tvdbRes.runtime != nil {
		result.Runtime = tvdbRes.runtime
	}

	if tmdbRes.tmdbRating != nil {
		result.TMDBRating = tmdbRes.tmdbRating
	}

	if tmdbRes.credits != "" {
		result.Credits = tmdbRes.credits
	}

	if tmdbRes.releaseDate != "" {
		result.ReleaseDate = tmdbRes.releaseDate
	} else if tvdbRes.releaseDate != "" {
		result.ReleaseDate = tvdbRes.releaseDate
	}

	if !result.IsAnime && tvdbRes.isAnime {
		result.IsAnime = true
	}

	if len(tmdbRes.names) > 0 || len(tvdbRes.names) > 0 {
		mergedNames := mergeNames(tmdbRes.names, tvdbRes.names)
		for lang, name := range mergedNames {
			result.Names = append(result.Names, model.TitleName{
				Name:      name,
				Language:  lang,
				IsPrimary: lang == "en",
			})
		}
	}

	if tmdbRes.coverFile != "" {
		result.CoverFile = tmdbRes.coverFile
	} else if tvdbRes.coverFile != "" {
		result.CoverFile = tvdbRes.coverFile
	}
}

// reconcileIDs fills missing external IDs from TMDB/TVDB fetch results and
// detects cross-source conflicts. Precedence:
//
//   - Empty IDs are filled (IMDB from TMDB then TVDB; TVDBID and TMDBID back-fill from the other source).
//   - When TMDB and TVDB both return an IMDBID and they differ, TMDB wins
//     (canonical primary matcher) and MatchStatus is downgraded to pending_review.
//   - When the existing TMDBID (Plex/TMDB-sourced) differs from TVDB's reported
//     TMDB counterpart, the existing ID is kept and MatchStatus is downgraded.
//
// Existing populated IDs are never overwritten except by the IMDB conflict rule
// (which re-assigns to make TMDB-canonical invariant explicit).
func reconcileIDs(result *MatchResult, tmdbRes tmdbFetchResult, tvdbRes tvdbFetchResult) {
	if result.IMDBID == "" && tmdbRes.imdbID != "" {
		result.IMDBID = tmdbRes.imdbID
	}
	if result.IMDBID == "" && tvdbRes.imdbID != "" {
		result.IMDBID = tvdbRes.imdbID
	}

	if result.TVDBID == 0 && tmdbRes.tvdbID != 0 {
		result.TVDBID = tmdbRes.tvdbID
	}

	if result.TMDBID == 0 && tvdbRes.tmdbID != 0 {
		result.TMDBID = tvdbRes.tmdbID
	}

	if tmdbRes.imdbID != "" && tvdbRes.imdbID != "" && tmdbRes.imdbID != tvdbRes.imdbID {
		log.Printf("cross-ref conflict: IMDB mismatch (tvdb=%d): TMDB says %q, TVDB says %q — keeping TMDB, downgrading to pending_review", result.TVDBID, tmdbRes.imdbID, tvdbRes.imdbID)
		result.IMDBID = tmdbRes.imdbID // TMDB canonical; discard TVDB's conflicting IMDB ID
		result.MatchStatus = model.MatchStatusPendingReview
	}

	if result.TMDBID != 0 && tvdbRes.tmdbID != 0 && result.TMDBID != tvdbRes.tmdbID {
		log.Printf("cross-ref conflict: TMDB ID mismatch (tvdb=%d): have=%d, TVDB says=%d — keeping existing, downgrading to pending_review", result.TVDBID, result.TMDBID, tvdbRes.tmdbID)
		result.MatchStatus = model.MatchStatusPendingReview
	}
}

// fetchTMDBAndTVDBParallel fetches TMDB and TVDB details concurrently when
// the corresponding IDs are populated. A 20s timeout caps the combined fetch.
// Returns zero-valued structs for any source that is nil, has no ID, or fails.
func (p *Pipeline) fetchTMDBAndTVDBParallel(ctx context.Context, result *MatchResult) (tmdbFetchResult, tvdbFetchResult) {
	var (
		tmdbRes tmdbFetchResult
		tvdbRes tvdbFetchResult
		wg      sync.WaitGroup
	)

	fetchCtx, fetchCancel := context.WithTimeout(ctx, 20*time.Second)
	defer fetchCancel()

	if p.tmdb != nil && result.TMDBID != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.fetchTMDBData(fetchCtx, result, &tmdbRes)
		}()
	}

	if p.tvdb != nil && result.TVDBID != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.fetchTVDBData(fetchCtx, result, &tvdbRes)
		}()
	}

	wg.Wait()
	return tmdbRes, tvdbRes
}

// appendAniListRomaji appends an "x-romaji" name from AniList GetNames when
// an AniList ID is known. Best-effort: errors and empty romaji are silently skipped.
func (p *Pipeline) appendAniListRomaji(ctx context.Context, result *MatchResult) {
	if p.anilist == nil || result.AniListID == 0 {
		return
	}
	alNames, err := p.anilist.GetNames(ctx, result.AniListID)
	if err != nil || alNames.Romaji == "" {
		return
	}
	result.Names = append(result.Names, model.TitleName{
		Name:     alNames.Romaji,
		Language: "x-romaji",
	})
}

// resolveIDsFromSources fills missing external IDs by chaining cross-ref lookup,
// TMDB FindByID (using IMDBID), and AniList search. Sets IsAnime=true when an
// AniList ID is present after resolution.
func (p *Pipeline) resolveIDsFromSources(ctx context.Context, result *MatchResult, input MatchInput) {
	// Cross-reference to fill missing IDs
	if p.crossDB != nil {
		crossIDs := p.crossDB.Lookup(ExternalIDs{
			IMDB:      result.IMDBID,
			TMDBMovie: result.TMDBID, // Try as movie first if we don't know
			TMDBTV:    result.TMDBID,
			TVDB:      result.TVDBID,
			AniList:   result.AniListID,
		})
		if crossIDs != nil {
			mergeIDs(result, crossIDs)
		}
	}

	// Try TMDB lookup by external ID if TMDBID still unknown (not in cross-ref DB)
	if result.TMDBID == 0 && p.tmdb != nil && result.IMDBID != "" {
		tmdbResult, mediaType, err := p.tmdb.FindByID(ctx, result.IMDBID, "imdb_id")
		if err == nil && tmdbResult != nil {
			result.TMDBID = tmdbResult.ID
			if result.TitleType == "" {
				if mediaType == "movie" {
					result.TitleType = model.TitleTypeMovie
				} else {
					result.TitleType = model.TitleTypeSeries
				}
			}
		}
	}

	// Try AniList search only for anime or series (movies don't appear on AniList)
	if result.AniListID == 0 && p.anilist != nil && (input.IsAnime || result.TitleType == model.TitleTypeSeries) {
		searchResults, err := p.anilist.SearchAnime(ctx, input.Title)
		if err != nil {
			log.Printf("anilist enrichment search failed: %v", err)
		} else if len(searchResults) > 0 {
			result.AniListID = searchResults[0].ID
		}
	}

	// Detect anime from AniList ID
	if result.AniListID != 0 {
		result.IsAnime = true
	}
}

// tmdbFetchResult holds data fetched from TMDB in a goroutine.
type tmdbFetchResult struct {
	overview    string
	genres      []string
	credits     string
	runtime     *int
	tmdbRating  *float64
	releaseDate string
	coverFile   string
	imdbID      string
	tvdbID      int64
	names       map[string]string
}

// tvdbFetchResult holds data fetched from TVDB in a goroutine.
type tvdbFetchResult struct {
	overview    string
	genres      []string
	runtime     *int
	imdbID      string
	tmdbID      int64
	names       map[string]string
	coverFile   string
	isAnime     bool
	releaseDate string // year only (e.g. "2008"), used as fallback when TMDB has no date
}

// mergeGenres unions TMDB and TVDB genre slices, deduplicating case-insensitively.
// TMDB genres take priority on case conflicts.
func mergeGenres(tmdbGenres []string, tvdbGenres []string) []string {
	seen := make(map[string]bool, len(tmdbGenres)+len(tvdbGenres))
	merged := make([]string, 0, len(tmdbGenres)+len(tvdbGenres))
	for _, g := range tmdbGenres {
		lower := strings.ToLower(g)
		if !seen[lower] {
			seen[lower] = true
			merged = append(merged, g)
		}
	}
	for _, g := range tvdbGenres {
		lower := strings.ToLower(g)
		if !seen[lower] {
			seen[lower] = true
			merged = append(merged, g)
		}
	}
	return merged
}

// mergeNames unions name maps from two sources; primary wins on duplicate key.
func mergeNames(primary, secondary map[string]string) map[string]string {
	result := make(map[string]string, len(primary)+len(secondary))
	for k, v := range secondary {
		result[k] = v
	}
	// Primary overwrites secondary
	for k, v := range primary {
		result[k] = v
	}
	return result
}

// fetchTMDBData fetches TMDB details and cover into a local struct (goroutine-safe).
func (p *Pipeline) fetchTMDBData(ctx context.Context, result *MatchResult, out *tmdbFetchResult) {
	coversDir := filepath.Join(p.dataDir, "covers")
	if result.TitleType == model.TitleTypeMovie {
		details, err := p.tmdb.GetMovieDetails(ctx, result.TMDBID)
		if err != nil {
			log.Printf("fetch movie details failed: %v", err)
			return
		}
		if details.IMDBID != "" {
			out.imdbID = details.IMDBID
		} else if details.ExternalIDs != nil && details.ExternalIDs.IMDBID != "" {
			out.imdbID = details.ExternalIDs.IMDBID
		}
		if details.ExternalIDs != nil {
			out.tvdbID = details.ExternalIDs.TVDBID
		}
		out.overview = details.Overview
		_, credits, runtime, rating := ExtractMovieMetadata(details)
		out.genres = extractGenreNames(details.Genres)
		out.credits = credits
		out.runtime = runtime
		out.tmdbRating = rating
		out.releaseDate = details.ReleaseDate
		if details.PosterPath != nil && *details.PosterPath != "" {
			filename, err := p.tmdb.DownloadCover(*details.PosterPath, coversDir)
			if err != nil {
				log.Printf("download tmdb movie cover failed: %v", err)
			} else {
				out.coverFile = filename
			}
		}
		// Translations (en/fr names)
		names, err := p.tmdb.GetTitleNames(ctx, result.TMDBID, "movie")
		if err == nil {
			out.names = names
		}
	} else {
		details, err := p.tmdb.GetTVDetails(ctx, result.TMDBID)
		if err != nil {
			log.Printf("fetch tv details failed: %v", err)
			return
		}
		if details.ExternalIDs != nil {
			out.imdbID = details.ExternalIDs.IMDBID
			out.tvdbID = details.ExternalIDs.TVDBID
		}
		out.overview = details.Overview
		_, credits, runtime, rating := ExtractTVMetadata(details)
		out.genres = extractGenreNames(details.Genres)
		out.credits = credits
		out.runtime = runtime
		out.tmdbRating = rating
		out.releaseDate = details.FirstAirDate
		if details.PosterPath != nil && *details.PosterPath != "" {
			filename, err := p.tmdb.DownloadCover(*details.PosterPath, coversDir)
			if err != nil {
				log.Printf("download tmdb tv cover failed: %v", err)
			} else {
				out.coverFile = filename
			}
		}
		names, err := p.tmdb.GetTitleNames(ctx, result.TMDBID, "tv")
		if err == nil {
			out.names = names
		}
	}
}

// fetchTVDBData fetches TVDB details and cover into a local struct (goroutine-safe).
func (p *Pipeline) fetchTVDBData(ctx context.Context, result *MatchResult, out *tvdbFetchResult) {
	coversDir := filepath.Join(p.dataDir, "covers")
	if result.TitleType == model.TitleTypeMovie {
		details, err := p.tvdb.GetMovieDetails(ctx, result.TVDBID)
		if err != nil {
			log.Printf("fetch tvdb movie details failed: %v", err)
			return
		}
		out.overview = extractMovieOverview(details)
		out.genres = extractMovieGenres(details)
		out.imdbID = extractMovieIMDB(details)
		out.tmdbID = extractMovieTMDB(details)
		out.names = extractMovieNames(details)
		if details.Runtime != nil {
			out.runtime = details.Runtime
		}
		for _, g := range out.genres {
			lower := strings.ToLower(g)
			if lower == "anime" || lower == "animation" {
				out.isAnime = true
				break
			}
		}
		if details.Year != "" {
			out.releaseDate = details.Year
		}
		if details.Image != "" {
			filename, err := p.tvdb.DownloadCover(details.Image, result.TVDBID, coversDir)
			if err != nil {
				log.Printf("download tvdb movie cover failed: %v", err)
			} else {
				out.coverFile = filename
			}
		}
	} else {
		details, err := p.tvdb.GetSeriesDetails(ctx, result.TVDBID)
		if err != nil {
			log.Printf("fetch tvdb series details failed: %v", err)
			return
		}
		out.overview = extractSeriesOverview(details)
		out.genres = extractSeriesGenres(details)
		out.imdbID = extractSeriesIMDB(details)
		out.tmdbID = extractSeriesTMDB(details)
		out.names = extractSeriesNames(details)
		if details.Runtime != nil {
			out.runtime = details.Runtime
		}
		for _, g := range out.genres {
			lower := strings.ToLower(g)
			if lower == "anime" || lower == "animation" {
				out.isAnime = true
				break
			}
		}
		if details.Year != "" {
			out.releaseDate = details.Year
		}
		if details.Image != "" {
			filename, err := p.tvdb.DownloadCover(details.Image, result.TVDBID, coversDir)
			if err != nil {
				log.Printf("download tvdb series cover failed: %v", err)
			} else {
				out.coverFile = filename
			}
		}
	}
}

func (p *Pipeline) downloadAniListCover(ctx context.Context, result *MatchResult) {
	details, err := p.anilist.GetAnimeDetails(ctx, result.AniListID)
	if err != nil {
		return
	}

	if details.AverageScore != nil {
		result.AniListRating = details.AverageScore
	}

	if details.CoverURL == "" {
		return
	}

	coversDir := filepath.Join(p.dataDir, "covers")
	filename, err := p.anilist.DownloadCover(details.CoverURL, coversDir)
	if err != nil {
		log.Printf("download anilist cover failed: %v", err)
		return
	}
	result.CoverFile = filename
}

// mergeIDs fills empty ID slots from a cross-reference lookup.
// All fills are gated on empty-slot conditions: existing runtime IDs (from Plex or a prior lookup)
// are never overwritten. If a slot is already populated, the cross-ref value is silently ignored.
func mergeIDs(result *MatchResult, ids *ExternalIDs) {
	if result.IMDBID == "" && ids.IMDB != "" {
		result.IMDBID = ids.IMDB
	}
	if result.TMDBID == 0 {
		if ids.TMDBMovie != 0 {
			result.TMDBID = ids.TMDBMovie
			if result.TitleType == "" {
				result.TitleType = model.TitleTypeMovie
			}
		} else if ids.TMDBTV != 0 {
			result.TMDBID = ids.TMDBTV
			if result.TitleType == "" {
				result.TitleType = model.TitleTypeSeries
			}
		}
	}
	if result.TVDBID == 0 && ids.TVDB != 0 {
		result.TVDBID = ids.TVDB
	}
	if result.AniListID == 0 && ids.AniList != 0 {
		result.AniListID = ids.AniList
	}
}
