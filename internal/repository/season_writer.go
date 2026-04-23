package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/model"
)

// SeasonWriter performs write operations on seasons within a caller-owned
// transaction. Accepting only *sql.Tx makes "write to the pool without a
// transaction" a compile-time error — the same class of bug that used to
// surface as SQLite BUSY deadlocks or partially-applied multi-statement writes.
type SeasonWriter struct {
	tx *sql.Tx
}

func NewSeasonWriter(tx *sql.Tx) *SeasonWriter {
	return &SeasonWriter{tx: tx}
}

// GetOrCreate returns the season for the given title and number, creating it
// if needed. Reads and inserts share the caller's transaction so the row is
// never raced by a concurrent writer.
func (w *SeasonWriter) GetOrCreate(ctx context.Context, titleID int64, seasonNumber int) (*model.Season, error) {
	var s model.Season
	err := w.tx.QueryRowContext(ctx,
		`SELECT id, title_id, season_number, total_episodes, my_rating FROM seasons WHERE title_id = ? AND season_number = ?`,
		titleID, seasonNumber,
	).Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.MyRating)
	if err == nil {
		return &s, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("get season: %w", err)
	}

	res, err := w.tx.ExecContext(ctx, `INSERT INTO seasons (title_id, season_number) VALUES (?, ?)`, titleID, seasonNumber)
	if err != nil {
		return nil, fmt.Errorf("create season: %w", err)
	}

	id, _ := res.LastInsertId()
	return &model.Season{ID: id, TitleID: titleID, SeasonNumber: seasonNumber}, nil
}

func (w *SeasonWriter) UpdateRating(ctx context.Context, id int64, rating int) error {
	if _, err := w.tx.ExecContext(ctx, `UPDATE seasons SET my_rating = ? WHERE id = ?`, rating, id); err != nil {
		return fmt.Errorf("update season rating: %w", err)
	}
	return nil
}

func (w *SeasonWriter) UpdateTotalEpisodes(ctx context.Context, id int64, total int) error {
	if _, err := w.tx.ExecContext(ctx, `UPDATE seasons SET total_episodes = ? WHERE id = ?`, total, id); err != nil {
		return fmt.Errorf("update total episodes: %w", err)
	}
	return nil
}

// Upsert creates or updates a season, returning the season with its ID.
// Collapses the GetOrCreate + UpdateTotalEpisodes pattern into one round-trip.
func (w *SeasonWriter) Upsert(ctx context.Context, titleID int64, seasonNumber, totalEpisodes int) (*model.Season, error) {
	var s model.Season
	err := w.tx.QueryRowContext(ctx,
		`INSERT INTO seasons (title_id, season_number, total_episodes)
		 VALUES (?, ?, ?)
		 ON CONFLICT(title_id, season_number) DO UPDATE SET total_episodes = excluded.total_episodes
		 RETURNING id, title_id, season_number, total_episodes, my_rating`,
		titleID, seasonNumber, totalEpisodes,
	).Scan(&s.ID, &s.TitleID, &s.SeasonNumber, &s.TotalEpisodes, &s.MyRating)
	if err != nil {
		return nil, fmt.Errorf("upsert season: %w", err)
	}
	return &s, nil
}
