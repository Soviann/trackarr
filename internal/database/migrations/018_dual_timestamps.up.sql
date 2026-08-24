-- Épisodes : renomme watched_at → first_watched_at, ajoute last_watched_at
ALTER TABLE episodes RENAME COLUMN watched_at TO first_watched_at;
ALTER TABLE episodes ADD COLUMN last_watched_at DATETIME;
UPDATE episodes SET last_watched_at = first_watched_at WHERE first_watched_at IS NOT NULL;

-- Met à jour l'index pour refléter la nouvelle colonne
DROP INDEX IF EXISTS idx_episodes_watched_at;
CREATE INDEX IF NOT EXISTS idx_episodes_first_watched_at ON episodes(watched, first_watched_at);

-- Titres : ajoute first_watched_at (approximation : created_at pour les titres déjà vus)
ALTER TABLE titles ADD COLUMN first_watched_at DATETIME;
UPDATE titles SET first_watched_at = created_at WHERE last_watched_at IS NOT NULL;
