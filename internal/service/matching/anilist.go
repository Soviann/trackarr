package matching

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const anilistAPIURL = "https://graphql.anilist.co"

type AniListClient struct {
	httpClient *http.Client
	apiURL     string // overridable for tests
}

func NewAniListClient() *AniListClient {
	return &AniListClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiURL:     anilistAPIURL,
	}
}

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
}

type AniListNames struct {
	Romaji  string
	English string
}

type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
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
  }
}
`

const syncRatingMutation = `
mutation ($mediaId: Int, $scoreRaw: Int) {
  SaveMediaListEntry(mediaId: $mediaId, scoreRaw: $scoreRaw) {
    id
    score(format: POINT_100)
  }
}
`

func (c *AniListClient) SearchAnime(title string) ([]AniListSearchResult, error) {
	var resp struct {
		Page struct {
			Media []struct {
				ID         int64  `json:"id"`
				MALID      *int64 `json:"idMal"`
				Title      struct {
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
			ID         int64  `json:"id"`
			MALID      *int64 `json:"idMal"`
			Title      struct {
				Romaji  string `json:"romaji"`
				English string `json:"english"`
			} `json:"title"`
			Episodes   *int   `json:"episodes"`
			Format     string `json:"format"`
			SeasonYear *int   `json:"seasonYear"`
		} `json:"Media"`
	}

	err := c.query(getAnimeDetailsQuery, map[string]interface{}{"id": anilistID}, "", &resp)
	if err != nil {
		return nil, fmt.Errorf("get anime details: %w", err)
	}

	return &AniListDetails{
		ID:           resp.Media.ID,
		MALID:        resp.Media.MALID,
		RomajiTitle:  resp.Media.Title.Romaji,
		EnglishTitle: resp.Media.Title.English,
		Episodes:     resp.Media.Episodes,
		Format:       resp.Media.Format,
		SeasonYear:   resp.Media.SeasonYear,
	}, nil
}

// SyncRating sends a rating (1-100 scale) to AniList for the given media.
// Requires a valid OAuth access token.
func (c *AniListClient) SyncRating(anilistID int64, rating int, accessToken string) error {
	var resp struct {
		SaveMediaListEntry struct {
			ID    int64 `json:"id"`
			Score int   `json:"score"`
		} `json:"SaveMediaListEntry"`
	}

	err := c.query(syncRatingMutation, map[string]interface{}{
		"mediaId":  anilistID,
		"scoreRaw": rating,
	}, accessToken, &resp)
	if err != nil {
		return fmt.Errorf("sync rating: %w", err)
	}
	return nil
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

func (c *AniListClient) query(gql string, variables map[string]interface{}, accessToken string, dest interface{}) error {
	body, err := json.Marshal(graphqlRequest{
		Query:     gql,
		Variables: variables,
	})
	if err != nil {
		return fmt.Errorf("marshal query: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AniList API error %d: %s", resp.StatusCode, string(respBody))
	}

	var gqlResp graphqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("AniList GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return json.Unmarshal(gqlResp.Data, dest)
}
