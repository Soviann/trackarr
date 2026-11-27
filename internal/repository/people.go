package repository

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
	"golang.org/x/text/unicode/norm"
)

const (
	DefaultPeopleSearchLimit = 20
	MaxPeopleSearchLimit     = 50
)

var canonicalPersonRoles = []string{"director", "writer", "composer", "actor"}

// PeopleRepository provides read-only queries over cast and crew credits.
type PeopleRepository struct {
	db database.DBTX
}

// NewPeopleRepository creates a PeopleRepository backed by db.
func NewPeopleRepository(db database.DBTX) *PeopleRepository {
	return &PeopleRepository{db: db}
}

type creditEntryJSON struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type personAggregate struct {
	nameVariants map[string]int
	roles        map[string]bool
	titleIDs     map[int64]bool
	rank         int
}

type rankedPerson struct {
	result model.PersonSearchResult
	rank   int
}

// Search finds matching people across titles.credits case-insensitively and
// accent-tolerantly, returning aggregated roles and distinct title counts.
func (r *PeopleRepository) Search(ctx context.Context, query string, limit int) ([]model.PersonSearchResult, error) {
	normQuery := normalizePersonText(query)
	if normQuery == "" {
		return []model.PersonSearchResult{}, nil
	}

	if limit <= 0 {
		limit = DefaultPeopleSearchLimit
	}
	if limit > MaxPeopleSearchLimit {
		limit = MaxPeopleSearchLimit
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, credits
		FROM titles
		WHERE credits IS NOT NULL
		  AND credits != ''
		  AND credits != '[]'
	`)
	if err != nil {
		return nil, fmt.Errorf("people: search credits: %w", err)
	}
	defer rows.Close()

	queryTokens := strings.Fields(normQuery)
	byPerson := make(map[string]*personAggregate)

	for rows.Next() {
		var titleID int64
		var rawCredits string
		if err := rows.Scan(&titleID, &rawCredits); err != nil {
			return nil, fmt.Errorf("people: scan title credits: %w", err)
		}

		var entries []creditEntryJSON
		if err := json.Unmarshal([]byte(rawCredits), &entries); err != nil {
			continue
		}

		for _, entry := range entries {
			trimmedName := strings.TrimSpace(entry.Name)
			if trimmedName == "" {
				continue
			}
			role := classifyCreditRole(entry.Role)
			if role == "" {
				continue
			}
			normName := normalizePersonText(trimmedName)
			rank, matched := personMatchRank(normName, normQuery, queryTokens)
			if !matched {
				continue
			}

			agg, exists := byPerson[normName]
			if !exists {
				agg = &personAggregate{
					nameVariants: make(map[string]int),
					roles:        make(map[string]bool),
					titleIDs:     make(map[int64]bool),
					rank:         rank,
				}
				byPerson[normName] = agg
			} else if rank < agg.rank {
				agg.rank = rank
			}

			agg.nameVariants[trimmedName]++
			agg.roles[role] = true
			agg.titleIDs[titleID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("people: iterate title credits: %w", err)
	}

	if len(byPerson) == 0 {
		return []model.PersonSearchResult{}, nil
	}

	ranked := make([]rankedPerson, 0, len(byPerson))
	for _, agg := range byPerson {
		roles := make([]string, 0, len(canonicalPersonRoles))
		for _, cr := range canonicalPersonRoles {
			if agg.roles[cr] {
				roles = append(roles, cr)
			}
		}
		ranked = append(ranked, rankedPerson{
			result: model.PersonSearchResult{
				Name:       pickPreferredPersonName(agg.nameVariants),
				Roles:      roles,
				TitleCount: len(agg.titleIDs),
			},
			rank: agg.rank,
		})
	}

	slices.SortFunc(ranked, func(a, b rankedPerson) int {
		if c := cmp.Compare(a.rank, b.rank); c != 0 {
			return c
		}
		if c := cmp.Compare(b.result.TitleCount, a.result.TitleCount); c != 0 {
			return c
		}
		if c := cmp.Compare(strings.ToLower(a.result.Name), strings.ToLower(b.result.Name)); c != 0 {
			return c
		}
		return cmp.Compare(a.result.Name, b.result.Name)
	})

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	out := make([]model.PersonSearchResult, len(ranked))
	for i, item := range ranked {
		out[i] = item.result
	}
	return out, nil
}

func classifyCreditRole(rawRole string) string {
	switch strings.TrimSpace(rawRole) {
	case "Director":
		return "director"
	case "Writer":
		return "writer"
	case "Composer":
		return "composer"
	case "":
		return ""
	default:
		return "actor"
	}
}

// normalizePersonText applies Unicode NFD decomposition, strips combining
// diacritic marks (unicode.Mn), lowercases, and collapses whitespace runs.
func normalizePersonText(s string) string {
	decomposed := norm.NFD.String(s)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(strings.ToLower(b.String())), " ")
}

func personMatchRank(normName, normQuery string, queryTokens []string) (int, bool) {
	if normName == "" || normQuery == "" {
		return 0, false
	}
	if normName == normQuery {
		return 0, true
	}
	if strings.HasPrefix(normName, normQuery) {
		return 1, true
	}
	for _, word := range strings.Fields(normName) {
		if strings.HasPrefix(word, normQuery) {
			return 2, true
		}
	}
	if strings.Contains(normName, normQuery) {
		return 3, true
	}
	if len(queryTokens) > 1 {
		for _, tok := range queryTokens {
			if !strings.Contains(normName, tok) {
				return 0, false
			}
		}
		return 4, true
	}
	return 0, false
}

func pickPreferredPersonName(variants map[string]int) string {
	best := ""
	bestCount := -1
	bestNonASCII := -1
	for name, count := range variants {
		nonASCII := 0
		for _, r := range name {
			if r > unicode.MaxASCII {
				nonASCII++
			}
		}
		if count > bestCount ||
			(count == bestCount && nonASCII > bestNonASCII) ||
			(count == bestCount && nonASCII == bestNonASCII && (best == "" || name < best)) {
			best = name
			bestCount = count
			bestNonASCII = nonASCII
		}
	}
	return best
}
