package repository

import (
	"database/sql"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/model"
)

type SeasonRepository struct {
	db *sql.DB
}

func NewSeasonRepository(db *sql.DB) *SeasonRepository {
	return &SeasonRepository{db: db}
}

// GetOrCreate returns the season for the given title and number, creating it if needed.
func (r *SeasonRepository) GetOrCreate(titleID int64, seasonNumber int) (*model.Season, error) {
	var s model.Season
	err := r.db.QueryRow(`SELECT id, title_id, season_number, total_episodes, my_rating FROM seasons WHERE title_id = ? AND season_number = ?`,
		titleID, seasonNumber).Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.MyRating)
	if err == nil {
		return &s, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("get season: %w", err)
	}

	res, err := r.db.Exec(`INSERT INTO seasons (title_id, season_number) VALUES (?, ?)`, titleID, seasonNumber)
	if err != nil {
		return nil, fmt.Errorf("create season: %w", err)
	}

	id, _ := res.LastInsertId()
	return &model.Season{ID: id, TitleID: titleID, SeasonNumber: seasonNumber}, nil
}

func (r *SeasonRepository) UpdateRating(id int64, rating int) error {
	_, err := r.db.Exec(`UPDATE seasons SET my_rating = ? WHERE id = ?`, rating, id)
	if err != nil {
		return fmt.Errorf("update season rating: %w", err)
	}
	return nil
}

func (r *SeasonRepository) UpdateTotalEpisodes(id int64, total int) error {
	_, err := r.db.Exec(`UPDATE seasons SET total_episodes = ? WHERE id = ?`, total, id)
	if err != nil {
		return fmt.Errorf("update total episodes: %w", err)
	}
	return nil
}
