-- Rename plex_rating_key to external_source_id in titles
ALTER TABLE titles RENAME COLUMN plex_rating_key TO external_source_id;
DROP INDEX IF EXISTS idx_titles_plex_rating_key;
CREATE INDEX IF NOT EXISTS idx_titles_external_source_id ON titles(external_source_id);

-- Rename plex_rating_key to external_source_id in episodes
ALTER TABLE episodes RENAME COLUMN plex_rating_key TO external_source_id;

-- Generalize watch_events table: rename plex_payload to raw_payload and extend source CHECK constraint
CREATE TABLE watch_events_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    episode_id INTEGER REFERENCES episodes(id) ON DELETE SET NULL,
    source TEXT NOT NULL CHECK(source IN ('plex', 'jellyfin', 'emby', 'manual', 'simkl', 'backfill')),
    raw_payload TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO watch_events_new (id, title_id, episode_id, source, raw_payload, created_at)
SELECT id, title_id, episode_id, source, plex_payload, created_at FROM watch_events;

DROP TABLE watch_events;

ALTER TABLE watch_events_new RENAME TO watch_events;

CREATE INDEX IF NOT EXISTS idx_watch_events_title_id ON watch_events(title_id);
CREATE INDEX IF NOT EXISTS idx_watch_events_episode_id ON watch_events(episode_id);
CREATE INDEX IF NOT EXISTS idx_watch_events_created_at ON watch_events(created_at);
