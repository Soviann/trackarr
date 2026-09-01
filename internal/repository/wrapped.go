package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Soviann/trackarr/internal/database"
	"github.com/Soviann/trackarr/internal/model"
)

type WrappedRepository struct {
	db database.DBTX
}

func NewWrappedRepository(db database.DBTX) *WrappedRepository {
	return &WrappedRepository{db: db}
}

// GetSnapshot retrieves an immutable Wrapped snapshot by year.
func (r *WrappedRepository) GetSnapshot(ctx context.Context, year int) (*model.WrappedResponse, *time.Time, error) {
	query := `SELECT data_json, created_at FROM wrapped_snapshots WHERE year = ?`
	var dataJSON string
	var createdAt time.Time

	err := r.db.QueryRowContext(ctx, query, year).Scan(&dataJSON, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("get wrapped snapshot %d: %w", year, err)
	}

	var resp model.WrappedResponse
	if err := json.Unmarshal([]byte(dataJSON), &resp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal wrapped snapshot %d: %w", year, err)
	}
	resp.CreatedAt = &createdAt

	return &resp, &createdAt, nil
}

// SaveSnapshot saves or replaces an immutable Wrapped snapshot.
func (r *WrappedRepository) SaveSnapshot(ctx context.Context, year int, resp *model.WrappedResponse) error {
	dataJSON, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal wrapped snapshot %d: %w", year, err)
	}

	query := `
		INSERT INTO wrapped_snapshots (year, data_json, created_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(year) DO UPDATE SET
			data_json = excluded.data_json,
			created_at = excluded.created_at
	`
	_, err = r.db.ExecContext(ctx, query, year, string(dataJSON))
	if err != nil {
		return fmt.Errorf("save wrapped snapshot %d: %w", year, err)
	}
	return nil
}

// DeleteSnapshot removes a snapshot for a given year.
func (r *WrappedRepository) DeleteSnapshot(ctx context.Context, year int) error {
	query := `DELETE FROM wrapped_snapshots WHERE year = ?`
	_, err := r.db.ExecContext(ctx, query, year)
	if err != nil {
		return fmt.Errorf("delete wrapped snapshot %d: %w", year, err)
	}
	return nil
}

// HasSnapshot checks if a snapshot already exists for a year.
func (r *WrappedRepository) HasSnapshot(ctx context.Context, year int) (bool, error) {
	query := `SELECT 1 FROM wrapped_snapshots WHERE year = ? LIMIT 1`
	var exists int
	err := r.db.QueryRowContext(ctx, query, year).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("has wrapped snapshot %d: %w", year, err)
	}
	return true, nil
}

// ListArchives lists all stored Wrapped snapshots for the gallery.
func (r *WrappedRepository) ListArchives(ctx context.Context) ([]model.WrappedArchiveItem, error) {
	query := `SELECT year, data_json, created_at FROM wrapped_snapshots ORDER BY year DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list wrapped archives: %w", err)
	}
	defer rows.Close()

	var items []model.WrappedArchiveItem
	for rows.Next() {
		var year int
		var dataJSON string
		var createdAt time.Time
		if err := rows.Scan(&year, &dataJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan wrapped archive: %w", err)
		}

		var resp model.WrappedResponse
		if err := json.Unmarshal([]byte(dataJSON), &resp); err == nil {
			var topCover *string
			if len(resp.TopFavorites.Movies) > 0 && resp.TopFavorites.Movies[0].CoverURL != nil {
				topCover = resp.TopFavorites.Movies[0].CoverURL
			} else if len(resp.TopFavorites.Series) > 0 && resp.TopFavorites.Series[0].CoverURL != nil {
				topCover = resp.TopFavorites.Series[0].CoverURL
			}

			items = append(items, model.WrappedArchiveItem{
				Year:              year,
				PersonaTitle:      resp.Persona.Title,
				PersonaBadges:     resp.Persona.Badges,
				TotalWatchMinutes: resp.TotalWatchMinutes,
				TotalTitles:       resp.Overview.TotalTitles,
				TopCoverURL:       topCover,
				CreatedAt:         createdAt,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in wrapped archives: %w", err)
	}

	return items, nil
}
