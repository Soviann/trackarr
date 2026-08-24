-- Index for quickly finding unwatched episodes in a season
CREATE INDEX IF NOT EXISTS idx_episodes_season_id_watched ON episodes(season_id, watched);

-- Index for ordering titles by last_watched_at (Nulls Last pattern)
-- (Replacing existing index if needed, but IF NOT EXISTS is safer)
CREATE INDEX IF NOT EXISTS idx_titles_last_watched_at_desc ON titles(last_watched_at DESC);
