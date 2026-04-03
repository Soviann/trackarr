package matching

import (
	"fmt"
	"log"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// Pipeline orchestrates the media matching process through Steps 1-5.
type Pipeline struct {
	tmdb    *TMDBClient
	anilist *AniListClient
	gemini  *GeminiClient
	crossDB *CrossRefDB // may be nil if not loaded
	dataDir string
}

func NewPipeline(tmdb *TMDBClient, anilist *AniListClient, gemini *GeminiClient, crossDB *CrossRefDB, dataDir string) *Pipeline {
	return &Pipeline{
		tmdb:    tmdb,
		anilist: anilist,
		gemini:  gemini,
		crossDB: crossDB,
		dataDir: dataDir,
	}
}

// MatchResult holds the outcome of running the matching pipeline.
type MatchResult struct {
	IMDBID      string
	TMDBID      int64
	TVDBID      int64
	AniListID   int64
	MatchStatus model.MatchStatus
	Names       []model.TitleName // multilingual names
	CoverFile   string            // local filename in covers dir
	TitleType   model.TitleType   // resolved type (may differ from input if anime detected)
}

// MatchInput holds the info needed to start the matching pipeline.
type MatchInput struct {
	Title string
	Year  int
	Type  model.TitleType
	// IDs already known from Plex metadata
	IMDBID string
	TMDBID int64
	TVDBID int64
}

// Run executes the full matching pipeline.
func (p *Pipeline) Run(input MatchInput) (*MatchResult, error) {
	result := &MatchResult{
		IMDBID:    input.IMDBID,
		TMDBID:    input.TMDBID,
		TVDBID:    input.TVDBID,
		TitleType: input.Type,
	}

	// Step 1: Check Plex metadata IDs — if we have TMDB or IMDB, we're confirmed
	if result.TMDBID != 0 || result.IMDBID != "" {
		result.MatchStatus = model.MatchStatusConfirmed
		p.enrichFromIDs(result, input)
		return result, nil
	}

	// Step 2: Cross-reference database lookup
	if p.crossDB != nil {
		crossIDs := p.crossDB.Lookup(ExternalIDs{
			IMDB: result.IMDBID,
			TMDB: result.TMDBID,
			TVDB: result.TVDBID,
		})
		if crossIDs != nil {
			mergeIDs(result, crossIDs)
			if result.TMDBID != 0 || result.IMDBID != "" {
				result.MatchStatus = model.MatchStatusConfirmed
				p.enrichFromIDs(result, input)
				return result, nil
			}
		}
	}

	// Step 3: TMDB API search
	if p.tmdb != nil {
		found := p.searchTMDB(input, result)
		if found {
			// Continue to Step 5 for verification
			return p.verifyAndEnrich(input, result)
		}
	}

	// Step 4: AniList search (anime only)
	if p.anilist != nil && input.Type == model.TitleTypeAnime {
		found := p.searchAniList(input, result)
		if found {
			return p.verifyAndEnrich(input, result)
		}
	}

	// Step 5 fallback: Gemini fuzzy resolution
	if p.gemini != nil {
		resolution, err := p.gemini.FuzzyResolve(PlexInfo{
			Title: input.Title,
			Year:  input.Year,
			Type:  string(input.Type),
		})
		if err != nil {
			log.Printf("gemini fuzzy resolve failed: %v", err)
		} else if resolution.CandidateTitle != "" {
			// Try TMDB search with the resolved title
			if p.tmdb != nil {
				resolvedInput := MatchInput{
					Title: resolution.CandidateTitle,
					Year:  resolution.CandidateYear,
					Type:  input.Type,
				}
				if p.searchTMDB(resolvedInput, result) {
					result.MatchStatus = model.MatchStatusUnconfirmed
					p.enrichFromIDs(result, input)
					return result, nil
				}
			}
		}
	}

	// No match found
	result.MatchStatus = model.MatchStatusUnconfirmed
	result.Names = []model.TitleName{{Name: input.Title, Language: "en", IsPrimary: true}}
	return result, nil
}

func (p *Pipeline) searchTMDB(input MatchInput, result *MatchResult) bool {
	var searchResults []TMDBSearchResult
	var err error

	if input.Type == model.TitleTypeMovie {
		searchResults, err = p.tmdb.SearchMovie(input.Title, input.Year)
	} else {
		searchResults, err = p.tmdb.SearchTV(input.Title, input.Year)
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

func (p *Pipeline) searchAniList(input MatchInput, result *MatchResult) bool {
	results, err := p.anilist.SearchAnime(input.Title)
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

func (p *Pipeline) verifyAndEnrich(input MatchInput, result *MatchResult) (*MatchResult, error) {
	if p.gemini != nil {
		// Build candidate info from TMDB
		candidateTitle := input.Title
		candidateYear := input.Year

		if p.tmdb != nil && result.TMDBID != 0 {
			if input.Type == model.TitleTypeMovie {
				if details, err := p.tmdb.GetMovieDetails(result.TMDBID); err == nil {
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
				if details, err := p.tmdb.GetTVDetails(result.TMDBID); err == nil {
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
			PlexInfo{Title: input.Title, Year: input.Year, Type: string(input.Type)},
			MatchCandidate{Title: candidateTitle, Year: candidateYear, TMDBID: result.TMDBID, IMDBID: result.IMDBID},
		)
		if err != nil {
			log.Printf("gemini verification failed: %v", err)
			result.MatchStatus = model.MatchStatusUnconfirmed
		} else if verification.Confirmed && verification.Confidence == "high" {
			result.MatchStatus = model.MatchStatusPendingReview
		} else {
			result.MatchStatus = model.MatchStatusUnconfirmed
		}
	} else {
		result.MatchStatus = model.MatchStatusPendingReview
	}

	p.enrichFromIDs(result, input)
	return result, nil
}

// enrichFromIDs fetches multilingual names, covers, and cross-references.
func (p *Pipeline) enrichFromIDs(result *MatchResult, input MatchInput) {
	// Cross-reference to fill missing IDs
	if p.crossDB != nil {
		crossIDs := p.crossDB.Lookup(ExternalIDs{
			IMDB:    result.IMDBID,
			TMDB:    result.TMDBID,
			TVDB:    result.TVDBID,
			AniList: result.AniListID,
		})
		if crossIDs != nil {
			mergeIDs(result, crossIDs)
		}
	}

	// Detect anime from AniList ID
	if result.AniListID != 0 && result.TitleType == model.TitleTypeSeries {
		result.TitleType = model.TitleTypeAnime
	}

	// Fetch multilingual names from TMDB
	if p.tmdb != nil && result.TMDBID != 0 {
		mediaType := "movie"
		if result.TitleType != model.TitleTypeMovie {
			mediaType = "tv"
		}

		names, err := p.tmdb.GetTitleNames(result.TMDBID, mediaType)
		if err == nil {
			for lang, name := range names {
				result.Names = append(result.Names, model.TitleName{
					Name:      name,
					Language:  lang,
					IsPrimary: lang == "en",
				})
			}
		}
	}

	// Add AniList names (romaji)
	if p.anilist != nil && result.AniListID != 0 {
		alNames, err := p.anilist.GetNames(result.AniListID)
		if err == nil {
			if alNames.Romaji != "" {
				result.Names = append(result.Names, model.TitleName{
					Name:     alNames.Romaji,
					Language: "x-romaji",
				})
			}
		}
	}

	// Fallback: use input title if no names found
	if len(result.Names) == 0 {
		result.Names = []model.TitleName{{Name: input.Title, Language: "en", IsPrimary: true}}
	}

	// Download cover from TMDB
	if p.tmdb != nil && result.TMDBID != 0 {
		p.downloadCover(result)
	}
}

func (p *Pipeline) downloadCover(result *MatchResult) {
	var posterPath *string

	if result.TitleType == model.TitleTypeMovie {
		details, err := p.tmdb.GetMovieDetails(result.TMDBID)
		if err == nil {
			posterPath = details.PosterPath
		}
	} else {
		details, err := p.tmdb.GetTVDetails(result.TMDBID)
		if err == nil {
			posterPath = details.PosterPath
		}
	}

	if posterPath != nil && *posterPath != "" {
		coversDir := fmt.Sprintf("%s/covers", p.dataDir)
		filename, err := p.tmdb.DownloadCover(*posterPath, coversDir)
		if err != nil {
			log.Printf("download cover failed: %v", err)
			return
		}
		result.CoverFile = filename
	}
}

func mergeIDs(result *MatchResult, ids *ExternalIDs) {
	if result.IMDBID == "" && ids.IMDB != "" {
		result.IMDBID = ids.IMDB
	}
	if result.TMDBID == 0 && ids.TMDB != 0 {
		result.TMDBID = ids.TMDB
	}
	if result.TVDBID == 0 && ids.TVDB != 0 {
		result.TVDBID = ids.TVDB
	}
	if result.AniListID == 0 && ids.AniList != 0 {
		result.AniListID = ids.AniList
	}
}
