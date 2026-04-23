package repository

import (
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
)

type SeasonRepository struct {
	db database.DBTX
}

func NewSeasonRepository(db database.DBTX) *SeasonRepository {
	return &SeasonRepository{db: db}
}

func (r *SeasonRepository) GetByID(id int64) (*model.Season, error) {
	var s model.Season
	err := r.db.QueryRow(`SELECT id, title_id, season_number, total_episodes, my_rating FROM seasons WHERE id = ?`, id).
		Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.MyRating)
	if err != nil {
		return nil, fmt.Errorf("get season: %w", err)
	}
	return &s, nil
}
