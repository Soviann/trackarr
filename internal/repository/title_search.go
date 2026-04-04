package repository

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nicolasvasse/plextracker/internal/model"
)

const searchResultLimit = 50

// searchRelevance scores how well a name matches the search term.
// Lower score = better match.
const (
	relevanceExact      = 0 // name == search (case-insensitive)
	relevanceWordExact  = 1 // a word in name == search
	relevancePrefix     = 2 // name starts with search
	relevanceWordPrefix = 3 // a word in name starts with search
	relevanceContains   = 4 // name contains search
	relevanceFTS        = 5 // FTS match (other)
	relevanceFuzzy      = 10 // fuzzy match base (+ levenshtein distance)
)

type searchResult struct {
	title     model.Title
	relevance int
}

// searchTitlesPaginated performs a relevance-ranked search with pagination.
func (r *TitleRepository) searchTitlesPaginated(searchTerm string, filter TitleFilter) (*PaginatedResult, error) {
	allResults, err := r.searchTitles(searchTerm, filter)
	if err != nil {
		return nil, err
	}

	total := len(allResults)
	limit := filter.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	offset := filter.Offset
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	page := allResults[offset:end]
	page, err = r.loadTitleRelationsLight(page)
	if err != nil {
		return nil, err
	}

	return &PaginatedResult{
		Titles:  page,
		Total:   total,
		HasMore: end < total,
	}, nil
}

// searchTitles performs a relevance-ranked search, returning up to searchResultLimit results.
func (r *TitleRepository) searchTitles(searchTerm string, filter TitleFilter) ([]model.Title, error) {
	useFTS := len(searchTerm) >= 2

	baseCols := `t.id, t.type, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.created_at, t.updated_at`

	var query string
	var conditions []string
	var args []interface{}

	if useFTS {
		ftsQuery := buildFTSQuery(searchTerm)
		query = `SELECT ` + baseCols + `, tn.name, tn.language FROM titles t JOIN title_names tn ON tn.title_id = t.id JOIN title_names_fts fts ON fts.rowid = tn.id`
		conditions = append(conditions, `title_names_fts MATCH ?`)
		args = append(args, ftsQuery)
	} else {
		query = `SELECT ` + baseCols + `, tn.name, tn.language FROM titles t JOIN title_names tn ON tn.title_id = t.id`
		conditions = append(conditions, `tn.name LIKE ?`)
		args = append(args, "%"+searchTerm+"%")
	}

	if filter.Status != nil {
		conditions = append(conditions, `t.status = ?`)
		args = append(args, *filter.Status)
	}
	if filter.Type != nil {
		conditions = append(conditions, `t.type = ?`)
		args = append(args, *filter.Type)
	}
	if filter.MatchStatus != nil {
		conditions = append(conditions, `t.match_status = ?`)
		args = append(args, *filter.MatchStatus)
	}

	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, ` AND `)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search titles: %w", err)
	}

	searchLower := strings.ToLower(searchTerm)
	bestByTitle := map[int64]*searchResult{}

	for rows.Next() {
		var t model.Title
		var matchedName, matchedLang string
		if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource, &t.CreatedAt, &t.UpdatedAt,
			&matchedName, &matchedLang); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan search title: %w", err)
		}

		rel := scoreRelevance(matchedName, searchLower)
		t.MatchedName = &matchedName
		t.MatchedLanguage = &matchedLang

		if existing, ok := bestByTitle[t.ID]; !ok || rel < existing.relevance {
			bestByTitle[t.ID] = &searchResult{title: t, relevance: rel}
		}
	}
	rows.Close()

	// Fuzzy fallback if few FTS results
	seen := make(map[int64]bool, len(bestByTitle))
	for id := range bestByTitle {
		seen[id] = true
	}

	if useFTS && len(bestByTitle) < 3 && len(searchTerm) >= 3 {
		fuzzyResults, err := r.fuzzySearch(searchTerm, seen, filter)
		if err == nil {
			for _, ft := range fuzzyResults {
				bestByTitle[ft.ID] = &searchResult{title: ft, relevance: relevanceFuzzy}
			}
		}
	}

	// Sort by relevance, then by title name alphabetically
	results := make([]searchResult, 0, len(bestByTitle))
	for _, sr := range bestByTitle {
		results = append(results, *sr)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].relevance != results[j].relevance {
			return results[i].relevance < results[j].relevance
		}
		ni := primaryName(results[i].title)
		nj := primaryName(results[j].title)
		return strings.ToLower(ni) < strings.ToLower(nj)
	})

	// Limit results
	if len(results) > searchResultLimit {
		results = results[:searchResultLimit]
	}

	titles := make([]model.Title, len(results))
	for i, sr := range results {
		titles[i] = sr.title
	}

	return titles, nil
}

// scoreRelevance computes how relevant a matched name is to the search term.
func scoreRelevance(name string, searchLower string) int {
	nameLower := strings.ToLower(name)

	if nameLower == searchLower {
		return relevanceExact
	}

	for _, word := range strings.Fields(nameLower) {
		if word == searchLower {
			return relevanceWordExact
		}
	}

	if strings.HasPrefix(nameLower, searchLower) {
		return relevancePrefix
	}

	for _, word := range strings.Fields(nameLower) {
		if strings.HasPrefix(word, searchLower) {
			return relevanceWordPrefix
		}
	}

	if strings.Contains(nameLower, searchLower) {
		return relevanceContains
	}

	return relevanceFTS
}

func primaryName(t model.Title) string {
	for _, n := range t.Names {
		if n.IsPrimary {
			return n.Name
		}
	}
	if len(t.Names) > 0 {
		return t.Names[0].Name
	}
	return ""
}

// buildFTSQuery transforms a search string into an FTS5 prefix query.
// "nar ship" becomes "nar* ship*" for prefix matching.
func buildFTSQuery(search string) string {
	words := strings.Fields(search)
	for i, w := range words {
		w = strings.NewReplacer(`"`, `""`, `*`, ``).Replace(w)
		words[i] = `"` + w + `"` + `*`
	}
	return strings.Join(words, " ")
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len([]rune(a)), len([]rune(b))
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev = curr
	}
	return prev[lb]
}

// fuzzySearch finds titles by Levenshtein distance when FTS5 returns few results.
func (r *TitleRepository) fuzzySearch(search string, seen map[int64]bool, filter TitleFilter) ([]model.Title, error) {
	rows, err := r.db.Query(`SELECT id, title_id, name, language FROM title_names`)
	if err != nil {
		return nil, fmt.Errorf("fuzzy search names: %w", err)
	}

	type candidate struct {
		titleID  int64
		name     string
		language string
		dist     int
	}
	searchLower := strings.ToLower(search)
	var candidates []candidate
	for rows.Next() {
		var id, titleID int64
		var name, lang string
		if err := rows.Scan(&id, &titleID, &name, &lang); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan fuzzy name: %w", err)
		}
		if seen[titleID] {
			continue
		}
		nameLower := strings.ToLower(name)
		bestDist := levenshtein(searchLower, nameLower)
		for _, word := range strings.Fields(nameLower) {
			if d := levenshtein(searchLower, word); d < bestDist {
				bestDist = d
			}
		}
		maxDist := 2
		if len(searchLower) > 6 {
			maxDist = 3
		}
		if bestDist <= maxDist {
			candidates = append(candidates, candidate{titleID, name, lang, bestDist})
		}
	}
	rows.Close()

	if len(candidates) == 0 {
		return nil, nil
	}

	best := map[int64]candidate{}
	for _, c := range candidates {
		if existing, ok := best[c.titleID]; !ok || c.dist < existing.dist {
			best[c.titleID] = c
		}
	}

	var ids []interface{}
	matchInfo := map[int64]candidate{}
	for _, c := range best {
		ids = append(ids, c.titleID)
		matchInfo[c.titleID] = c
		seen[c.titleID] = true
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	query := `SELECT t.id, t.type, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.created_at, t.updated_at FROM titles t WHERE t.id IN (` + placeholders + `)`
	var args []interface{}
	args = append(args, ids...)

	if filter.Status != nil {
		query += ` AND t.status = ?`
		args = append(args, *filter.Status)
	}
	if filter.Type != nil {
		query += ` AND t.type = ?`
		args = append(args, *filter.Type)
	}

	tRows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("fuzzy search titles: %w", err)
	}

	var titles []model.Title
	for tRows.Next() {
		var t model.Title
		if err := tRows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
			&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource, &t.CreatedAt, &t.UpdatedAt); err != nil {
			tRows.Close()
			return nil, fmt.Errorf("scan fuzzy title: %w", err)
		}
		if c, ok := matchInfo[t.ID]; ok {
			t.MatchedName = &c.name
			t.MatchedLanguage = &c.language
		}
		titles = append(titles, t)
	}
	tRows.Close()

	return titles, nil
}
