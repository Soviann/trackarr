package matching

import (
	"context"
	"fmt"
)

const syncRatingMutation = `
mutation ($mediaId: Int, $scoreRaw: Int) {
  SaveMediaListEntry(mediaId: $mediaId, scoreRaw: $scoreRaw) {
    id
    score(format: POINT_100)
  }
}
`

// SyncRating sends a rating (1-100 scale) to AniList for the given media.
// Requires a valid OAuth access token.
func (c *AniListClient) SyncRating(ctx context.Context, anilistID int64, rating int, accessToken string) error {
	var resp struct {
		SaveMediaListEntry struct {
			ID    int64 `json:"id"`
			Score int   `json:"score"`
		} `json:"SaveMediaListEntry"`
	}

	err := c.query(ctx, syncRatingMutation, map[string]interface{}{
		"mediaId":  anilistID,
		"scoreRaw": rating,
	}, accessToken, &resp)
	if err != nil {
		return fmt.Errorf("sync rating: %w", err)
	}
	return nil
}
