-- Add 'backfill' to watch_events.source CHECK constraint.
-- SQLite requires table recreation to alter a CHECK constraint.

CREATE TABLE watch_events_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    episode_id INTEGER REFERENCES episodes(id) ON DELETE SET NULL,
    source TEXT NOT NULL CHECK(source IN ('plex', 'manual', 'backfill')),
    plex_payload TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO watch_events_new SELECT * FROM watch_events;

DROP TABLE watch_events;

ALTER TABLE watch_events_new RENAME TO watch_events;
