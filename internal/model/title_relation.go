package model

import "time"

type RelationType string

const (
	RelationSideStory   RelationType = "SIDE_STORY"
	RelationSpinOff     RelationType = "SPIN_OFF"
	RelationAlternative RelationType = "ALTERNATIVE"
	RelationSummary     RelationType = "SUMMARY"
	RelationOther       RelationType = "OTHER"
	RelationPrequel     RelationType = "PREQUEL"
	RelationSequel      RelationType = "SEQUEL"
	RelationCharacter   RelationType = "CHARACTER"
	RelationCollection  RelationType = "COLLECTION"
)

type TitleRelation struct {
	ID           int64        `json:"id"`
	TitleID      int64        `json:"title_id"`
	SeasonID     *int64       `json:"season_id,omitempty"`
	SeasonNumber *int         `json:"season_number,omitempty"`
	Provider     string       `json:"provider"`
	ExternalID   int64        `json:"external_id"`
	RelationType RelationType `json:"relation_type"`
	Format       string       `json:"format"`
	Title        string       `json:"title"`
	RomajiTitle  *string      `json:"romaji_title,omitempty"`
	CoverURL     *string      `json:"cover_url,omitempty"`
	Year         *int         `json:"year,omitempty"`
	Score        *int         `json:"score,omitempty"`
	EpisodeCount *int         `json:"episode_count,omitempty"`
	Duration     *int         `json:"duration,omitempty"`
	Overview     *string      `json:"overview,omitempty"`
	SortOrder    int          `json:"sort_order"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`

	// Joined from matched local title if present in Trackarr
	MatchedTitleID *int64       `json:"matched_title_id,omitempty"`
	MatchedStatus  *TitleStatus `json:"matched_status,omitempty"`
	MatchedRating  *int         `json:"matched_rating,omitempty"`
	MatchedType    *TitleType   `json:"matched_type,omitempty"`
	RadarrID       *int64       `json:"radarr_id,omitempty"`
	SonarrID       *int64       `json:"sonarr_id,omitempty"`
}
