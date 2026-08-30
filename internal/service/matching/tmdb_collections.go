package matching

import (
	"context"
	"fmt"
	"net/url"
)

// TMDBCollectionInfo represents the belongs_to_collection field in TMDB movie details.
type TMDBCollectionInfo struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	PosterPath   *string `json:"poster_path"`
	BackdropPath *string `json:"backdrop_path"`
}

// TMDBCollectionDetails represents the full response from /collection/{collection_id}.
type TMDBCollectionDetails struct {
	ID           int64                `json:"id"`
	Name         string               `json:"name"`
	Overview     string               `json:"overview"`
	PosterPath   *string              `json:"poster_path"`
	BackdropPath *string              `json:"backdrop_path"`
	Parts        []TMDBCollectionPart `json:"parts"`
}

// TMDBCollectionPart represents a movie part within a TMDB collection.
type TMDBCollectionPart struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Overview     string  `json:"overview"`
	ReleaseDate  string  `json:"release_date"`
	PosterPath   *string `json:"poster_path"`
	BackdropPath *string `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
}

// GetMovieCollection fetches all movies in a TMDB collection by collection ID.
func (c *TMDBClient) GetMovieCollection(ctx context.Context, collectionID int64, language string) (*TMDBCollectionDetails, error) {
	if collectionID <= 0 {
		return nil, fmt.Errorf("invalid tmdb collection id: %d", collectionID)
	}

	params := url.Values{}
	if language != "" {
		params.Set("language", language)
	} else {
		params.Set("language", "fr-FR")
	}

	var details TMDBCollectionDetails
	if err := c.get(ctx, fmt.Sprintf("/collection/%d", collectionID), params, &details); err != nil {
		return nil, fmt.Errorf("tmdb get movie collection %d: %w", collectionID, err)
	}

	return &details, nil
}
