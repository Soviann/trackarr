package repository

import (
	"fmt"
	"sort"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

// DuplicateGroup is a set of confirmed series titles that transitively share an
// external id. SharedID is a human-readable representative key for the group,
// formatted "imdb:<value>" / "tvdb:<value>" / "tmdb:<value>" — used by the
// service to label the proposal (the id that flagged the duplicate). When a
// group merges several keys (e.g. shares imdb with A and tmdb with B), SharedID
// holds the first key encountered for the component; the exact representative is
// not load-bearing, only the display.
type DuplicateGroup struct {
	SharedID string
	Titles   []model.Title
}

// SeasonAuditRepository reads duplicate-series groups and dismissal state.
type SeasonAuditRepository struct {
	db database.DBTX
}

// NewSeasonAuditRepository creates a new SeasonAuditRepository.
func NewSeasonAuditRepository(db database.DBTX) *SeasonAuditRepository {
	return &SeasonAuditRepository{db: db}
}

// DuplicateSeriesGroups returns confirmed series titles that share an external
// id (imdb, tvdb, or tmdb) with at least one other confirmed series title,
// grouped together. Each returned group has >= 2 titles.
//
// A title may share its imdb with one peer and its tmdb with another; such
// titles are merged into a single connected component (union-find) so each
// title ends up in exactly one group. Full titles are loaded via GetByID so the
// service can read names and seasons.
func (r *SeasonAuditRepository) DuplicateSeriesGroups() ([]DuplicateGroup, error) {
	// key (e.g. "imdb:tt123") -> set of title ids sharing that key.
	keyToIDs := map[string]map[int64]struct{}{}
	// id -> first key it appeared under (representative for display).
	idFirstKey := map[int64]string{}

	columns := []struct{ column, prefix string }{
		{"imdb_id", "imdb"},
		{"tvdb_id", "tvdb"},
		{"tmdb_id", "tmdb"},
	}

	for _, c := range columns {
		// Find confirmed series titles whose id value is shared by >1 confirmed
		// series, returning each member alongside the shared value.
		query := fmt.Sprintf(`
			SELECT t.id, CAST(t.%[1]s AS TEXT)
			FROM titles t
			JOIN (
				SELECT %[1]s FROM titles
				WHERE type = 'series' AND match_status = 'confirmed' AND %[1]s IS NOT NULL
				GROUP BY %[1]s HAVING COUNT(*) > 1
			) d ON t.%[1]s = d.%[1]s
			WHERE t.type = 'series' AND t.match_status = 'confirmed'`, c.column)

		rows, err := r.db.Query(query)
		if err != nil {
			return nil, fmt.Errorf("season_audit: duplicate %s query: %w", c.column, err)
		}
		for rows.Next() {
			var id int64
			var val string
			if err := rows.Scan(&id, &val); err != nil {
				rows.Close()
				return nil, fmt.Errorf("season_audit: scan duplicate %s: %w", c.column, err)
			}
			key := c.prefix + ":" + val
			if keyToIDs[key] == nil {
				keyToIDs[key] = map[int64]struct{}{}
			}
			keyToIDs[key][id] = struct{}{}
			if _, ok := idFirstKey[id]; !ok {
				idFirstKey[id] = key
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("season_audit: iterate duplicate %s: %w", c.column, err)
		}
		rows.Close()
	}

	if len(keyToIDs) == 0 {
		return nil, nil
	}

	// Union-find over title ids: every id sharing a key is unioned together so
	// transitive overlaps (imdb-with-A, tmdb-with-B) collapse to one component.
	uf := newUnionFind()
	for _, ids := range keyToIDs {
		var prev int64
		first := true
		for id := range ids {
			uf.add(id)
			if first {
				prev = id
				first = false
				continue
			}
			uf.union(prev, id)
		}
	}

	// Group ids by their component root.
	components := map[int64][]int64{}
	for id := range idFirstKey {
		root := uf.find(id)
		components[root] = append(components[root], id)
	}

	var groups []DuplicateGroup
	for _, ids := range components {
		if len(ids) < 2 {
			continue
		}
		// SharedID: the first-seen key of any member, deterministically the one
		// with the lowest member id (so output is stable across map iteration).
		var repID int64 = -1
		for _, id := range ids {
			if repID == -1 || id < repID {
				repID = id
			}
		}
		group := DuplicateGroup{SharedID: idFirstKey[repID]}
		for _, id := range ids {
			t, err := r.titleByID(id)
			if err != nil {
				return nil, err
			}
			group.Titles = append(group.Titles, *t)
		}
		sort.Slice(group.Titles, func(i, j int) bool {
			return group.Titles[i].ID < group.Titles[j].ID
		})
		groups = append(groups, group)
	}
	return groups, nil
}

// titleByID loads a full title via a fresh TitleRepository over the same handle.
func (r *SeasonAuditRepository) titleByID(id int64) (*model.Title, error) {
	t, err := NewTitleRepository(r.db).GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("season_audit: load title %d: %w", id, err)
	}
	return t, nil
}

// IsDismissed reports whether the (source, target) attachment pair has been
// dismissed by the user.
func (r *SeasonAuditRepository) IsDismissed(sourceID, targetID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM season_audit_dismissals
			WHERE source_title_id = ? AND target_title_id = ?
		)`, sourceID, targetID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("season_audit: is dismissed: %w", err)
	}
	return exists, nil
}

// unionFind is a minimal disjoint-set structure over int64 title ids.
type unionFind struct {
	parent map[int64]int64
}

func newUnionFind() *unionFind { return &unionFind{parent: map[int64]int64{}} }

func (u *unionFind) add(x int64) {
	if _, ok := u.parent[x]; !ok {
		u.parent[x] = x
	}
}

func (u *unionFind) find(x int64) int64 {
	u.add(x)
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]] // path compression
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int64) {
	ra, rb := u.find(a), u.find(b)
	if ra != rb {
		u.parent[ra] = rb
	}
}
