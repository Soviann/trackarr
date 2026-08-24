-- Revert: remove 'jellyfin' from watch_events.source CHECK constraint.

DELETE FROM watch_events WHERE source = 'jellyfin';

CREATE TABLE watch_events_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    episode_id INTEGER REFERENCES episodes(id) ON DELETE SET NULL,
    source TEXT NOT NULL CHECK(source IN ('plex', 'manual', 'backfill')),
    plex_payload TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO watch_events_old SELECT * FROM watch_events;

DROP TABLE watch_events;

ALTER TABLE watch_events_old RENAME TO watch_events;
