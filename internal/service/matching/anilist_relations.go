package matching

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const getRelationsQuery = `
query ($id: Int) {
  Media(id: $id, type: ANIME) {
    id
    format
    title { romaji english }
    relations {
      edges {
        relationType
        node {
          id
          type
          format
          title { romaji english }
          coverImage { large medium }
          startDate { year month day }
          averageScore
          episodes
          duration
          description
        }
      }
    }
  }
}
`

type relationNode struct {
	ID     int64  `json:"id"`
	Type   string `json:"type"`
	Format string `json:"format"`
	Title  struct {
		Romaji  string `json:"romaji"`
		English string `json:"english"`
	} `json:"title"`
	CoverImage struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
	} `json:"coverImage"`
	StartDate struct {
		Year  *int `json:"year"`
		Month *int `json:"month"`
		Day   *int `json:"day"`
	} `json:"startDate"`
	AverageScore *int    `json:"averageScore"`
	Episodes     *int    `json:"episodes"`
	Duration     *int    `json:"duration"`
	Description  *string `json:"description"`
}

type relationEdge struct {
	RelationType string       `json:"relationType"`
	Node         relationNode `json:"node"`
}

type relationMedia struct {
	ID     int64  `json:"id"`
	Format string `json:"format"`
	Title  struct {
		Romaji  string `json:"romaji"`
		English string `json:"english"`
	} `json:"title"`
	Relations struct {
		Edges []relationEdge `json:"edges"`
	} `json:"relations"`
}

func (m relationMedia) displayTitle() string {
	if m.Title.English != "" {
		return m.Title.English
	}
	return m.Title.Romaji
}

func (c *AniListClient) getRelations(ctx context.Context, id int64) (*relationMedia, error) {
	var resp struct {
		Media relationMedia `json:"Media"`
	}
	if err := c.query(ctx, getRelationsQuery, map[string]any{"id": id}, "", &resp); err != nil {
		return nil, fmt.Errorf("get relations: %w", err)
	}
	return &resp.Media, nil
}

// FranchiseRelationNode represents a related anime node from the AniList relations graph.
type FranchiseRelationNode struct {
	ID           int64
	RelationType string
	Format       string
	Title        string
	RomajiTitle  string
	CoverURL     string
	Year         *int
	Score        *int
	EpisodeCount *int
	Duration     *int
	Overview     *string
}

// GetFranchiseRelations returns all side stories, movies, spin-offs, and specials
// linked to an AniList media ID (excluding manga adaptations and main TV/ONA seasons).
func (c *AniListClient) GetFranchiseRelations(ctx context.Context, id int64) ([]FranchiseRelationNode, error) {
	current, err := c.getRelations(ctx, id)
	if err != nil {
		return nil, err
	}

	var out []FranchiseRelationNode
	for _, e := range current.Relations.Edges {
		if e.Node.Type != "ANIME" {
			continue
		}
		// Skip main TV/ONA prequel/sequel edges as they are primary seasons
		if (e.RelationType == "PREQUEL" || e.RelationType == "SEQUEL") && seriesFormat(e.Node.Format) {
			continue
		}

		title := e.Node.Title.English
		if title == "" {
			title = e.Node.Title.Romaji
		}

		coverURL := e.Node.CoverImage.Large
		if coverURL == "" {
			coverURL = e.Node.CoverImage.Medium
		}

		out = append(out, FranchiseRelationNode{
			ID:           e.Node.ID,
			RelationType: e.RelationType,
			Format:       e.Node.Format,
			Title:        title,
			RomajiTitle:  e.Node.Title.Romaji,
			CoverURL:     coverURL,
			Year:         e.Node.StartDate.Year,
			Score:        e.Node.AverageScore,
			EpisodeCount: e.Node.Episodes,
			Duration:     e.Node.Duration,
			Overview:     e.Node.Description,
		})
	}
	return out, nil
}

// SeasonChain is the outcome of walking an entry's PREQUEL chain on AniList.
type SeasonChain struct {
	RootID       int64  // AniList id of the chain root (the "main" series entry)
	RootTitle    string // English title of the root, romaji fallback
	SeasonNumber int    // 1-based ordinal of the resolved entry within the chain
	PartNumber   int    // 1-based part/cour number within the season (1 for standard/first part)
	IsRoot       bool
	RootIsSeries bool // true when the chain root's format is TV or ONA; consumers must check before merging a TV season into a non-series root
}

// seriesFormat reports whether a format counts as a season in the chain.
// TV and ONA are season carriers; movies/OVA/specials are traversed through
// without incrementing the season ordinal.
func seriesFormat(format string) bool { return format == "TV" || format == "ONA" }

const maxChainDepth = 25

// parsedPartInfo contains parsed season and part information from a title.
type parsedPartInfo struct {
	season int // 0 if not explicitly mentioned in the title
	part   int // 0 if not explicitly mentioned in the title
}

var (
	partRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:part|cour)\s*(\d+)\b`),
		regexp.MustCompile(`(?i)\b(\d+)(?:st|nd|rd|th)\s*cour\b`),
		regexp.MustCompile(`(?i)\bpart\s*([ivx]+)\b`),
	}
	seasonRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bseason\s*(\d+)\b`),
		regexp.MustCompile(`(?i)\b(\d+)(?:st|nd|rd|th)\s*season\b`),
		regexp.MustCompile(`(?i)\bseason\s*([ivx]+)\b`),
	}
	romanNumerals = map[string]int{
		"i": 1, "ii": 2, "iii": 3, "iv": 4, "v": 5,
		"vi": 6, "vii": 7, "viii": 8, "ix": 9, "x": 10,
	}
)

func parseRoman(s string) int {
	return romanNumerals[strings.ToLower(s)]
}

// parseSeasonAndPart inspects an anime title (or english + romaji) and extracts
// explicit season and part/cour numbers if present.
func parseSeasonAndPart(title string) parsedPartInfo {
	var info parsedPartInfo

	for _, re := range partRegexes {
		m := re.FindStringSubmatch(title)
		if len(m) > 1 {
			if num, err := strconv.Atoi(m[1]); err == nil {
				info.part = num
				break
			} else if rom := parseRoman(m[1]); rom > 0 {
				info.part = rom
				break
			}
		}
	}

	for _, re := range seasonRegexes {
		m := re.FindStringSubmatch(title)
		if len(m) > 1 {
			if num, err := strconv.Atoi(m[1]); err == nil {
				info.season = num
				break
			} else if rom := parseRoman(m[1]); rom > 0 {
				info.season = rom
				break
			}
		}
	}

	// Roman numeral season after title (e.g. "Mushoku Tensei II", "DanMachi V") if not already matched
	if info.season == 0 {
		cleanTitle := title
		for _, re := range partRegexes {
			cleanTitle = re.ReplaceAllString(cleanTitle, "")
		}
		reStandaloneRoman := regexp.MustCompile(`\b([IVXLCDM]+)\b`)
		matches := reStandaloneRoman.FindAllString(cleanTitle, -1)
		for _, match := range matches {
			if rom := parseRoman(match); rom > 1 { // skip "I" to avoid false positive standalone words
				info.season = rom
				break
			}
		}
	}

	return info
}

func nodePartInfo(m *relationMedia) parsedPartInfo {
	info := parseSeasonAndPart(m.displayTitle())
	if info.season == 0 || info.part == 0 {
		// Try the other title if available
		alt := m.Title.Romaji
		if m.Title.English != "" && m.Title.Romaji != "" {
			altInfo := parseSeasonAndPart(alt)
			if info.season == 0 {
				info.season = altInfo.season
			}
			if info.part == 0 {
				info.part = altInfo.part
			}
		}
	}
	return info
}

// isSameSeasonContinuation returns true if child is a continuation part/cour of parent.
func isSameSeasonContinuation(child, parent *relationMedia) bool {
	childInfo := nodePartInfo(child)
	if childInfo.part <= 1 {
		return false
	}

	parentInfo := nodePartInfo(parent)

	// If child explicitly states Season X Part Y:
	if childInfo.season > 0 {
		// If parent is also Season X
		if parentInfo.season == childInfo.season {
			return true
		}
		// If parent didn't have explicit season, but child has Season X Part Y with Part > 1
		// and parent was the immediate prequel
		if parentInfo.season == 0 && childInfo.season == 1 {
			return true
		}
	} else {
		// Child has no explicit season number, but has Part > 1
		// E.g. "Mushoku Tensei Part 2" after "Mushoku Tensei"
		return true
	}

	return false
}

// ResolveSeasonChain walks PREQUEL edges from the given AniList media to the
// chain root. The season number is 1 + the count of TV/ONA prequels on the
// path, excluding continuation parts of the same season. Movies and one-off
// formats are never seasons: they return IsRoot=true.
//
// Intermediate movies are traversed without being counted as seasons. The
// root itself may be a non-series node (e.g. a MOVIE). Consumers must check
// RootIsSeries before merging a TV season into the root, as the auto-merge
// logic must not attach a TV season to a MOVIE root.
func (c *AniListClient) ResolveSeasonChain(ctx context.Context, id int64) (*SeasonChain, error) {
	current, err := c.getRelations(ctx, id)
	if err != nil {
		return nil, err
	}
	if !seriesFormat(current.Format) {
		return &SeasonChain{RootID: current.ID, RootTitle: current.displayTitle(), SeasonNumber: 1, PartNumber: 1, IsRoot: true, RootIsSeries: false}, nil
	}

	currPartInfo := nodePartInfo(current)
	resolvedPart := currPartInfo.part
	if resolvedPart == 0 {
		resolvedPart = 1
	}

	visited := map[int64]bool{current.ID: true}
	// Walk the chain and collect all TV/ONA nodes
	var chainNodes []*relationMedia
	chainNodes = append(chainNodes, current)

	curr := current
	for depth := 0; depth < maxChainDepth; depth++ {
		next := pickPrequel(curr)
		if next == 0 {
			break
		}
		if visited[next] {
			return nil, fmt.Errorf("anilist relation cycle at media %d", next)
		}
		visited[next] = true
		prev, err := c.getRelations(ctx, next)
		if err != nil {
			return nil, err
		}
		chainNodes = append(chainNodes, prev)
		curr = prev
	}
	if len(visited) > maxChainDepth {
		return nil, fmt.Errorf("anilist relation chain too deep for media %d", id)
	}

	root := chainNodes[len(chainNodes)-1]

	// Compute season number by counting transitions from root down to current
	// Reverse iterate: root (index len-1) down to current (index 0)
	seasons := 1
	// Find the index of the first series node from the root
	rootSeriesIdx := -1
	for i := len(chainNodes) - 1; i >= 0; i-- {
		if seriesFormat(chainNodes[i].Format) {
			rootSeriesIdx = i
			break
		}
	}

	if rootSeriesIdx != -1 {
		lastSeries := chainNodes[rootSeriesIdx]
		for i := rootSeriesIdx - 1; i >= 0; i-- {
			node := chainNodes[i]
			if !seriesFormat(node.Format) {
				continue
			}
			// Check if node is continuation of lastSeries
			if !isSameSeasonContinuation(node, lastSeries) {
				seasons++
			}
			lastSeries = node
		}
	}

	return &SeasonChain{
		RootID:       root.ID,
		RootTitle:    root.displayTitle(),
		SeasonNumber: seasons,
		PartNumber:   resolvedPart,
		IsRoot:       root.ID == id,
		RootIsSeries: seriesFormat(root.Format),
	}, nil
}

// pickPrequel returns the id of the PREQUEL edge to follow: TV first, then
// ONA, then any ANIME prequel (movie recaps sit between cours in some chains).
func pickPrequel(m *relationMedia) int64 {
	var tv, ona, other int64
	for _, e := range m.Relations.Edges {
		if e.RelationType != "PREQUEL" || e.Node.Type != "ANIME" {
			continue
		}
		switch {
		case e.Node.Format == "TV" && tv == 0:
			tv = e.Node.ID
		case e.Node.Format == "ONA" && ona == 0:
			ona = e.Node.ID
		case other == 0:
			other = e.Node.ID
		}
	}
	if tv != 0 {
		return tv
	}
	if ona != 0 {
		return ona
	}
	return other
}
