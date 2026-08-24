-- Titres : supprime first_watched_at
ALTER TABLE titles DROP COLUMN first_watched_at;

-- Épisodes : restore l'index et les colonnes
DROP INDEX IF EXISTS idx_episodes_first_watched_at;
CREATE INDEX IF NOT EXISTS idx_episodes_watched_at ON episodes(watched, watched_at);
ALTER TABLE episodes DROP COLUMN last_watched_at;
ALTER TABLE episodes RENAME COLUMN first_watched_at TO watched_at;
