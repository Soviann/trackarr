package repository

import (
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
)

// SettingRepository reads the settings key-value store. Writes live on
// SettingWriter, which requires a *sql.Tx.
type SettingRepository struct {
	db database.DBTX
}

func NewSettingRepository(db database.DBTX) *SettingRepository {
	return &SettingRepository{db: db}
}

func (r *SettingRepository) Get(key string) (string, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}
	return value, nil
}
