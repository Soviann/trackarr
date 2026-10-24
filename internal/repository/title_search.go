package repository

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/Soviann/trackarr/internal/model"
)

const searchResultLimit = 50

// searchRelevance scores how well a name matches the search term.
// Lower score = better match.
const (
	relevanceExact      = 0  // name == search (case-insensitive)
	relevanceWordExact  = 1  // a word in name == search
	relevancePrefix     = 2  // name starts with search
	relevanceWordPrefix = 3  // a word in name starts with search
	relevanceContains   = 4  // name contains search
	relevanceFTS        = 5  // FTS match (other)
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
	offset := min(filter.Offset, total)
	end := min(offset+limit, total)

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

	var query string
	var conditions []string
	var args []any

	if useFTS {
		ftsQuery := buildFTSQuery(searchTerm)
		query = `SELECT ` + titleSelectCols + `, tn.name, tn.language FROM titles t JOIN title_names tn ON tn.title_id = t.id JOIN title_names_fts fts ON fts.rowid = tn.id`
		conditions = append(conditions, `title_names_fts MATCH ?`)
		args = append(args, ftsQuery)
	} else {
		query = `SELECT ` + titleSelectCols + `, tn.name, tn.language FROM titles t JOIN title_names tn ON tn.title_id = t.id`
		conditions = append(conditions, `tn.name LIKE ?`)
		args = append(args, "%"+searchTerm+"%")
	}

	conditions, args = appendFilterConditions(conditions, args, filter)

	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, ` AND `)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search titles: %w", err)
	}
	defer rows.Close()

	searchLower := strings.ToLower(searchTerm)
	bestByTitle := map[int64]*searchResult{}

	for rows.Next() {
		var t model.Title
		var matchedName, matchedLang string
		if err := scanTitleRow(rows, &t, &matchedName, &matchedLang); err != nil {
			return nil, fmt.Errorf("scan search title: %w", err)
		}

		rel := scoreRelevance(matchedName, searchLower)
		t.MatchedName = &matchedName
		t.MatchedLanguage = &matchedLang

		if existing, ok := bestByTitle[t.ID]; !ok || rel < existing.relevance {
			bestByTitle[t.ID] = &searchResult{title: t, relevance: rel}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search titles: %w", err)
	}

	// Fuzzy fallback if few FTS results
	seen := make(map[int64]bool, len(bestByTitle))
	for id := range bestByTitle {
		seen[id] = true
	}

	if useFTS && len(bestByTitle) < 3 && len(searchTerm) >= 3 {
		triResults, err := r.trigramSearch(searchTerm, seen, filter)
		if err == nil {
			for _, ft := range triResults {
				rel := relevanceFuzzy
				if ft.MatchedName != nil {
					if rScore := scoreRelevance(*ft.MatchedName, searchLower); rScore < relevanceFuzzy {
						rel = rScore
					}
				}
				bestByTitle[ft.ID] = &searchResult{title: ft, relevance: rel}
			}
		}
	}

	// Sort by relevance, then by title name alphabetically
	results := make([]searchResult, 0, len(bestByTitle))
	for _, sr := range bestByTitle {
		results = append(results, *sr)
	}
	slices.SortFunc(results, func(a, b searchResult) int {
		if c := cmp.Compare(a.relevance, b.relevance); c != 0 {
			return c
		}
		return cmp.Compare(strings.ToLower(primaryName(a.title)), strings.ToLower(primaryName(b.title)))
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

// buildTrigramQuery transforms a search string into an FTS5 trigram query.
// It extracts quoted 3-grams and substrings from terms and joins them with OR.
func buildTrigramQuery(search string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '"' || r == '*' || r == ':' || r == '(' || r == ')' {
			return -1
		}
		return r
	}, strings.ToLower(search))

	fields := strings.Fields(clean)
	var tokens []string
	seen := make(map[string]bool)

	// If the entire clean phrase has >= 3 runes, add it as a quoted substring candidate
	runesAll := []rune(clean)
	if len(runesAll) >= 3 {
		tokens = append(tokens, `"`+clean+`"`)
		seen[clean] = true
	}

	for _, field := range fields {
		runes := []rune(field)
		if len(runes) >= 3 && !seen[field] {
			tokens = append(tokens, `"`+field+`"`)
			seen[field] = true
		}
		for i := 0; i <= len(runes)-3; i++ {
			tri := string(runes[i : i+3])
			if !seen[tri] {
				seen[tri] = true
				tokens = append(tokens, `"`+tri+`"`)
			}
		}
	}

	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " OR ")
}

// trigramSearch finds titles using the SQLite FTS5 trigram table when standard FTS returns few results.
func (r *TitleRepository) trigramSearch(search string, seen map[int64]bool, filter TitleFilter) ([]model.Title, error) {
	triQuery := buildTrigramQuery(search)
	if triQuery == "" {
		return nil, nil
	}

	query := `SELECT ` + titleSelectCols + `, tn.name, tn.language FROM titles t JOIN title_names tn ON tn.title_id = t.id JOIN title_names_trigram_fts fts ON fts.rowid = tn.id`
	conditions := []string{`title_names_trigram_fts MATCH ?`}
	args := []any{triQuery}

	conditions, args = appendFilterConditions(conditions, args, filter)

	query += ` WHERE ` + strings.Join(conditions, ` AND `)
	query += ` ORDER BY bm25(title_names_trigram_fts) LIMIT 50`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("trigram search titles: %w", err)
	}
	defer rows.Close()

	var results []model.Title
	for rows.Next() {
		var t model.Title
		var matchedName, matchedLang string
		if err := scanTitleRow(rows, &t, &matchedName, &matchedLang); err != nil {
			return nil, fmt.Errorf("scan trigram title: %w", err)
		}
		if seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		t.MatchedName = &matchedName
		t.MatchedLanguage = &matchedLang
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trigram titles: %w", err)
	}

	return results, nil
}

func appendFilterConditions(conditions []string, args []any, filter TitleFilter) ([]string, []any) {
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
	if filter.SeriesStatus != nil {
		conditions = append(conditions, `t.series_status = ?`)
		args = append(args, *filter.SeriesStatus)
	}
	if filter.IsAnime != nil {
		conditions = append(conditions, `t.is_anime = ?`)
		if *filter.IsAnime {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if filter.Decade != nil {
		conditions = append(conditions, `t.year BETWEEN ? AND ?`)
		args = append(args, *filter.Decade, *filter.Decade+9)
	}
	if filter.ReleaseFrom != nil {
		if filter.IncludeNoRelease {
			conditions = append(conditions, `(t.release_date >= ? OR t.release_date IS NULL)`)
		} else {
			conditions = append(conditions, `t.release_date >= ?`)
		}
		args = append(args, *filter.ReleaseFrom)
	}
	if filter.ReleaseTo != nil {
		if filter.IncludeNoRelease {
			conditions = append(conditions, `(t.release_date <= ? OR t.release_date IS NULL)`)
		} else {
			conditions = append(conditions, `t.release_date <= ?`)
		}
		args = append(args, *filter.ReleaseTo)
	}
	if !filter.IncludeNoRelease && (filter.ReleaseFrom != nil || filter.ReleaseTo != nil) {
		conditions = append(conditions, `t.release_date IS NOT NULL`)
	}
	if len(filter.Genres) > 0 {
		op := filter.GenreOp
		if op != "AND" {
			op = "OR"
		}
		if op == "OR" {
			placeholders := make([]string, len(filter.Genres))
			for i, g := range filter.Genres {
				placeholders[i] = "?"
				args = append(args, g)
			}
			conditions = append(conditions, `EXISTS (SELECT 1 FROM title_genres tg WHERE tg.title_id = t.id AND tg.genre IN (`+strings.Join(placeholders, ",")+`))`)
		} else { // AND
			for _, g := range filter.Genres {
				conditions = append(conditions, `EXISTS (SELECT 1 FROM title_genres tg WHERE tg.title_id = t.id AND tg.genre = ?)`)
				args = append(args, g)
			}
		}
	}
	if len(filter.OriginCountries) > 0 {
		placeholders := make([]string, len(filter.OriginCountries))
		for i, c := range filter.OriginCountries {
			placeholders[i] = "?"
			args = append(args, c)
		}
		conditions = append(conditions, `t.origin_country IN (`+strings.Join(placeholders, ",")+`)`)
	}
	if filter.MyRatingMin != nil {
		conditions = append(conditions, `t.my_rating >= ?`)
		args = append(args, *filter.MyRatingMin)
	}
	if filter.TMDBRatingMin != nil {
		conditions = append(conditions, `t.tmdb_rating >= ?`)
		args = append(args, *filter.TMDBRatingMin)
	}
	return conditions, args
}
