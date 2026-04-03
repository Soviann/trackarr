package repository

import (
	"database/sql"
	"fmt"
)

type SettingRepository struct {
	db *sql.DB
}

func NewSettingRepository(db *sql.DB) *SettingRepository {
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

func (r *SettingRepository) Set(key, value string) error {
	_, err := r.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}

func (r *SettingRepository) Delete(key string) error {
	_, err := r.db.Exec(`DELETE FROM settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete setting %s: %w", key, err)
	}
	return nil
}
