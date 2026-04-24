package repository

import (
	"context"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

// SeasonWithProgress augments a season's identity fields with episode totals
// computed from the episodes table. Used by AniListPushService to derive the
// per-season MediaListStatus.
type SeasonWithProgress struct {
	ID              int64
	TitleID         int64
	SeasonNumber    int
	TotalEpisodes   int
	WatchedEpisodes int
}

type SeasonRepository struct {
	db database.DBTX
}

func NewSeasonRepository(db database.DBTX) *SeasonRepository {
	return &SeasonRepository{db: db}
}

func (r *SeasonRepository) GetByID(id int64) (*model.Season, error) {
	var s model.Season
	err := r.db.QueryRow(`SELECT id, title_id, season_number, total_episodes FROM seasons WHERE id = ?`, id).
		Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes)
	if err != nil {
		return nil, fmt.Errorf("get season: %w", err)
	}
	return &s, nil
}

// GetWithProgress returns the season augmented with its total + watched
// episode counts. TotalEpisodes falls back to the actual episodes row count
// when seasons.total_episodes is NULL — matches the plan's backfill guarantee.
func (r *SeasonRepository) GetWithProgress(ctx context.Context, id int64) (*SeasonWithProgress, error) {
	var s SeasonWithProgress
	err := r.db.QueryRowContext(ctx, `
		SELECT
			s.id, s.title_id, s.season_number,
			COALESCE(s.total_episodes, (SELECT COUNT(*) FROM episodes WHERE season_id = s.id)) AS total_episodes,
			COALESCE((SELECT COUNT(*) FROM episodes WHERE season_id = s.id AND watched = 1), 0) AS watched_episodes
		FROM seasons s
		WHERE s.id = ?`, id).
		Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.WatchedEpisodes)
	if err != nil {
		return nil, fmt.Errorf("get season with progress: %w", err)
	}
	return &s, nil
}
