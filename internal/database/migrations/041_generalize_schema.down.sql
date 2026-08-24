ALTER TABLE titles RENAME COLUMN external_source_id TO plex_rating_key;
DROP INDEX IF EXISTS idx_titles_external_source_id;
CREATE INDEX IF NOT EXISTS idx_titles_plex_rating_key ON titles(plex_rating_key);

ALTER TABLE episodes RENAME COLUMN external_source_id TO plex_rating_key;

CREATE TABLE watch_events_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    episode_id INTEGER REFERENCES episodes(id) ON DELETE SET NULL,
    source TEXT NOT NULL CHECK(source IN ('plex', 'jellyfin', 'manual', 'backfill')),
    plex_payload TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO watch_events_old (id, title_id, episode_id, source, plex_payload, created_at)
SELECT id, title_id, episode_id, source, raw_payload, created_at FROM watch_events;

DROP TABLE watch_events;

ALTER TABLE watch_events_old RENAME TO watch_events;

CREATE INDEX IF NOT EXISTS idx_watch_events_title_id ON watch_events(title_id);
CREATE INDEX IF NOT EXISTS idx_watch_events_episode_id ON watch_events(episode_id);
CREATE INDEX IF NOT EXISTS idx_watch_events_created_at ON watch_events(created_at);
