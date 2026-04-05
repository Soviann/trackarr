package matching

import "fmt"

type AniListSearchResult struct {
	ID           int64  `json:"id"`
	RomajiTitle  string `json:"romajiTitle"`
	EnglishTitle string `json:"englishTitle"`
	Episodes     *int   `json:"episodes"`
	Format       string `json:"format"` // TV, MOVIE, OVA, ONA, SPECIAL, MUSIC
	SeasonYear   *int   `json:"seasonYear"`
	MALID        *int64 `json:"idMal"`
}

func (r *AniListSearchResult) DisplayTitle() string {
	if r.EnglishTitle != "" {
		return r.EnglishTitle
	}
	return r.RomajiTitle
}

type AniListDetails struct {
	ID           int64  `json:"id"`
	MALID        *int64 `json:"idMal"`
	RomajiTitle  string `json:"romajiTitle"`
	EnglishTitle string `json:"englishTitle"`
	Episodes     *int   `json:"episodes"`
	Format       string `json:"format"`
	SeasonYear   *int   `json:"seasonYear"`
	CoverURL     string `json:"coverURL"` // extraLarge or large
}

type AniListNames struct {
	Romaji  string
	English string
}

const searchAnimeQuery = `
query ($search: String) {
  Page(perPage: 5) {
    media(search: $search, type: ANIME) {
      id
      idMal
      title { romaji english }
      episodes
      format
      seasonYear
    }
  }
}
`

const getAnimeDetailsQuery = `
query ($id: Int) {
  Media(id: $id, type: ANIME) {
    id
    idMal
    title { romaji english }
    episodes
    format
    seasonYear
    coverImage { extraLarge large }
  }
}
`

func (c *AniListClient) SearchAnime(title string) ([]AniListSearchResult, error) {
	var resp struct {
		Page struct {
			Media []struct {
				ID    int64  `json:"id"`
				MALID *int64 `json:"idMal"`
				Title struct {
					Romaji  string `json:"romaji"`
					English string `json:"english"`
				} `json:"title"`
				Episodes   *int   `json:"episodes"`
				Format     string `json:"format"`
				SeasonYear *int   `json:"seasonYear"`
			} `json:"media"`
		} `json:"Page"`
	}

	err := c.query(searchAnimeQuery, map[string]interface{}{"search": title}, "", &resp)
	if err != nil {
		return nil, fmt.Errorf("search anime: %w", err)
	}

	results := make([]AniListSearchResult, len(resp.Page.Media))
	for i, m := range resp.Page.Media {
		results[i] = AniListSearchResult{
			ID:           m.ID,
			MALID:        m.MALID,
			RomajiTitle:  m.Title.Romaji,
			EnglishTitle: m.Title.English,
			Episodes:     m.Episodes,
			Format:       m.Format,
			SeasonYear:   m.SeasonYear,
		}
	}
	return results, nil
}

func (c *AniListClient) GetAnimeDetails(anilistID int64) (*AniListDetails, error) {
	var resp struct {
		Media struct {
			ID    int64  `json:"id"`
			MALID *int64 `json:"idMal"`
			Title struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
			} `json:"title"`
			Episodes   *int   `json:"episodes"`
			Format     string `json:"format"`
			SeasonYear *int   `json:"seasonYear"`
			CoverImage struct {
				ExtraLarge string `json:"extraLarge"`
				Large      string `json:"large"`
			} `json:"coverImage"`
		} `json:"Media"`
	}

	err := c.query(getAnimeDetailsQuery, map[string]interface{}{"id": anilistID}, "", &resp)
	if err != nil {
		return nil, fmt.Errorf("get anime details: %w", err)
	}

	coverURL := resp.Media.CoverImage.ExtraLarge
	if coverURL == "" {
		coverURL = resp.Media.CoverImage.Large
	}

	return &AniListDetails{
		ID:           resp.Media.ID,
		MALID:        resp.Media.MALID,
		RomajiTitle:  resp.Media.Title.Romaji,
		EnglishTitle: resp.Media.Title.English,
		Episodes:     resp.Media.Episodes,
		Format:       resp.Media.Format,
		SeasonYear:   resp.Media.SeasonYear,
		CoverURL:     coverURL,
	}, nil
}

// GetNames returns romaji and English names for an anime.
func (c *AniListClient) GetNames(anilistID int64) (*AniListNames, error) {
	details, err := c.GetAnimeDetails(anilistID)
	if err != nil {
		return nil, err
	}
	return &AniListNames{
		Romaji:  details.RomajiTitle,
		English: details.EnglishTitle,
	}, nil
}
