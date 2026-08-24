-- DB-9: index for is_primary lookups in title_names
CREATE INDEX IF NOT EXISTS idx_title_names_title_id_primary ON title_names(title_id, is_primary);

-- DB-10: standalone index for global episode watch counts (stats queries filter by watched+watched_at without season_id)
CREATE INDEX IF NOT EXISTS idx_episodes_watched_at ON episodes(watched, watched_at);

-- DB-11: index on watch_events.episode_id to speed up ON DELETE SET NULL scan
CREATE INDEX IF NOT EXISTS idx_watch_events_episode_id ON watch_events(episode_id);

-- DB-14: drop redundant idx_titles_last_watched_at (superseded by idx_titles_last_watched_at_desc from migration 011)
DROP INDEX IF EXISTS idx_titles_last_watched_at;
