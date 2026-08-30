-- 042_title_relations.up.sql
-- Store franchise relations, side stories, movies, OVAs, and spin-offs linked to titles and seasons.

CREATE TABLE IF NOT EXISTS title_relations (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id      INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    season_id     INTEGER REFERENCES seasons(id) ON DELETE CASCADE,
    provider      TEXT    NOT NULL DEFAULT 'anilist',
    external_id   INTEGER NOT NULL,
    relation_type TEXT    NOT NULL,
    format        TEXT    NOT NULL,
    title         TEXT    NOT NULL,
    romaji_title  TEXT,
    cover_url     TEXT,
    year          INTEGER,
    score         INTEGER,
    episode_count INTEGER,
    duration      INTEGER,
    overview      TEXT,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(title_id, provider, external_id)
);

CREATE INDEX IF NOT EXISTS idx_title_relations_title_id ON title_relations(title_id);
CREATE INDEX IF NOT EXISTS idx_title_relations_season_id ON title_relations(season_id);
CREATE INDEX IF NOT EXISTS idx_title_relations_provider_external ON title_relations(provider, external_id);
