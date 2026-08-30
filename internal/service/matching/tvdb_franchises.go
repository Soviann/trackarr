package matching

import (
	"context"
	"fmt"
	"net/url"
)

// TVDBListBrief represents a list reference in TVDB extended series/movie response.
type TVDBListBrief struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	IsOfficial bool   `json:"isOfficial"`
	Overview   string `json:"overview"`
	URL        string `json:"url"`
}

// TVDBListEntity represents an item inside a TVDB official list.
type TVDBListEntity struct {
	Order    int    `json:"order"`
	SeriesID *int64 `json:"seriesId"`
	MovieID  *int64 `json:"movieId"`
}

// TVDBListDetail represents the full response from /lists/{id}/extended.
type TVDBListDetail struct {
	ID         int64            `json:"id"`
	Name       string           `json:"name"`
	Overview   string           `json:"overview"`
	IsOfficial bool             `json:"isOfficial"`
	Image      string           `json:"image"`
	Entities   []TVDBListEntity `json:"entities"`
}

type tvdbListExtendedResponse struct {
	Data *TVDBListDetail `json:"data"`
}

// GetListExtended fetches extended details for a TVDB list (including entities with order).
func (c *TVDBClient) GetListExtended(ctx context.Context, listID int64) (*TVDBListDetail, error) {
	if listID <= 0 {
		return nil, fmt.Errorf("invalid tvdb list id: %d", listID)
	}

	var resp tvdbListExtendedResponse
	params := url.Values{"meta": {"translations"}}
	if err := c.get(ctx, fmt.Sprintf("/lists/%d/extended", listID), params, &resp); err != nil {
		return nil, fmt.Errorf("tvdb get list %d: %w", listID, err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("tvdb list %d not found", listID)
	}

	return resp.Data, nil
}
