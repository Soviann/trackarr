CREATE INDEX IF NOT EXISTS idx_titles_tvdb_id ON titles(tvdb_id) WHERE tvdb_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_watch_events_created_at ON watch_events(created_at);
