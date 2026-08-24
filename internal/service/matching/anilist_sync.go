package matching

import (
	"context"
	"fmt"
)

const saveMediaListEntryMutation = `
mutation ($mediaId: Int!, $status: MediaListStatus!, $progress: Int, $scoreRaw: Int) {
  SaveMediaListEntry(mediaId: $mediaId, status: $status, progress: $progress, scoreRaw: $scoreRaw) {
    id
  }
}`

// SaveMediaListEntryInput captures every field SaveMediaListEntry ever sends.
// Score is on our 1..10 scale; AniList stores out of 100 (scoreRaw), so we
// multiply by 10 before sending. A nil Score omits the field entirely.
type SaveMediaListEntryInput struct {
	MediaID  int64
	Status   string // CURRENT | COMPLETED | PLANNING | DROPPED | PAUSED | REPEATING
	Progress int
	Score    *int // nil → omit; otherwise 1..10 (converted to /100 on the wire)
}

// SaveMediaListEntry upserts a media list entry on AniList.
// Returns TokenInvalidError when the token is rejected (HTTP 401).
func (c *AniListClient) SaveMediaListEntry(ctx context.Context, in SaveMediaListEntryInput, accessToken string) error {
	if in.MediaID == 0 {
		return fmt.Errorf("anilist: missing mediaId")
	}
	vars := map[string]any{
		"mediaId":  in.MediaID,
		"status":   in.Status,
		"progress": in.Progress,
	}
	if in.Score != nil {
		vars["scoreRaw"] = *in.Score * 10
	}
	return c.queryAuthenticated(ctx, saveMediaListEntryMutation, vars, accessToken)
}

// SyncRating remains as a thin wrapper for backward compatibility with the
// existing test (TestAniListSyncRating). New callers MUST use SaveMediaListEntry.
func (c *AniListClient) SyncRating(ctx context.Context, anilistID int64, rating int, accessToken string) error {
	return c.SaveMediaListEntry(ctx, SaveMediaListEntryInput{
		MediaID: anilistID,
		Status:  "COMPLETED",
		Score:   &rating,
	}, accessToken)
}
