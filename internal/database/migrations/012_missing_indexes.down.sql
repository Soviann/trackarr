DROP INDEX IF EXISTS idx_title_names_title_id_primary;
DROP INDEX IF EXISTS idx_episodes_watched_at;
DROP INDEX IF EXISTS idx_watch_events_episode_id;

-- Restore index dropped in up migration
CREATE INDEX IF NOT EXISTS idx_titles_last_watched_at ON titles(last_watched_at);
