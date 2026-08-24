package matching

import (
	"context"
	"fmt"
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
        node { id type format title { romaji english } }
      }
    }
  }
}
`

type relationMedia struct {
	ID     int64  `json:"id"`
	Format string `json:"format"`
	Title  struct {
		Romaji  string `json:"romaji"`
		English string `json:"english"`
	} `json:"title"`
	Relations struct {
		Edges []struct {
			RelationType string `json:"relationType"`
			Node         struct {
				ID     int64  `json:"id"`
				Type   string `json:"type"`
				Format string `json:"format"`
				Title  struct {
					Romaji  string `json:"romaji"`
					English string `json:"english"`
				} `json:"title"`
			} `json:"node"`
		} `json:"edges"`
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

// SeasonChain is the outcome of walking an entry's PREQUEL chain on AniList.
type SeasonChain struct {
	RootID       int64  // AniList id of the chain root (the "main" series entry)
	RootTitle    string // English title of the root, romaji fallback
	SeasonNumber int    // 1-based ordinal of the resolved entry within the chain
	IsRoot       bool
	RootIsSeries bool // true when the chain root's format is TV or ONA; consumers must check before merging a TV season into a non-series root
}

// seriesFormat reports whether a format counts as a season in the chain.
// TV and ONA are season carriers; movies/OVA/specials are traversed through
// without incrementing the season ordinal.
func seriesFormat(format string) bool { return format == "TV" || format == "ONA" }

const maxChainDepth = 25

// ResolveSeasonChain walks PREQUEL edges from the given AniList media to the
// chain root. The season number is 1 + the count of TV/ONA prequels on the
// path. Movies and one-off formats are never seasons: they return IsRoot=true.
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
		return &SeasonChain{RootID: current.ID, RootTitle: current.displayTitle(), SeasonNumber: 1, IsRoot: true, RootIsSeries: false}, nil
	}

	visited := map[int64]bool{current.ID: true}
	root := current
	seasons := 1

	for depth := 0; depth < maxChainDepth; depth++ {
		next := pickPrequel(root)
		if next == 0 {
			return &SeasonChain{
				RootID:       root.ID,
				RootTitle:    root.displayTitle(),
				SeasonNumber: seasons,
				IsRoot:       root.ID == id,
				RootIsSeries: seriesFormat(root.Format),
			}, nil
		}
		if visited[next] {
			return nil, fmt.Errorf("anilist relation cycle at media %d", next)
		}
		visited[next] = true
		prev, err := c.getRelations(ctx, next)
		if err != nil {
			return nil, err
		}
		if seriesFormat(prev.Format) {
			seasons++
		}
		root = prev
	}
	return nil, fmt.Errorf("anilist relation chain too deep for media %d", id)
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
