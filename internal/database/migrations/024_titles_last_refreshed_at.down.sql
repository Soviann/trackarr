-- Requiert SQLite >= 3.35 (image Docker en 3.42+).
ALTER TABLE titles DROP COLUMN last_refreshed_at;
