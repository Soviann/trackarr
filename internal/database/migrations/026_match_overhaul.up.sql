ALTER TABLE titles ADD COLUMN simkl_id INTEGER;
ALTER TABLE titles ADD COLUMN simkl_slug TEXT;

CREATE TABLE match_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER REFERENCES titles(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    detail TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_match_events_created_at ON match_events(created_at DESC);

CREATE TABLE season_audit_dismissals (
    source_title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    target_title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source_title_id, target_title_id)
);
