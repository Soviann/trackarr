CREATE TABLE titles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('movie', 'series', 'anime')),
    year INTEGER NOT NULL,
    cover_url TEXT,
    imdb_id TEXT,
    anilist_id INTEGER,
    tmdb_id INTEGER,
    tvdb_id INTEGER,
    plex_rating_key TEXT,
    my_rating INTEGER CHECK(my_rating IS NULL OR (my_rating >= 1 AND my_rating <= 10)),
    status TEXT NOT NULL DEFAULT 'watching' CHECK(status IN ('watching', 'completed', 'dropped', 'plan_to_watch')),
    series_status TEXT CHECK(series_status IS NULL OR series_status IN ('returning', 'ended', 'cancelled', 'in_production')),
    match_status TEXT NOT NULL DEFAULT 'confirmed' CHECK(match_status IN ('confirmed', 'pending_review', 'unconfirmed')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE title_names (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    language TEXT NOT NULL,
    is_primary INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL,
    total_episodes INTEGER,
    my_rating INTEGER CHECK(my_rating IS NULL OR (my_rating >= 1 AND my_rating <= 10)),
    UNIQUE(title_id, season_number)
);

CREATE TABLE episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    episode INTEGER NOT NULL,
    name TEXT,
    air_date TEXT,
    watched INTEGER NOT NULL DEFAULT 0,
    watched_at DATETIME,
    plex_rating_key TEXT,
    UNIQUE(season_id, episode)
);

CREATE TABLE watch_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    episode_id INTEGER REFERENCES episodes(id) ON DELETE SET NULL,
    source TEXT NOT NULL CHECK(source IN ('plex', 'manual')),
    plex_payload TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Indexes
CREATE INDEX idx_titles_status ON titles(status);
CREATE INDEX idx_titles_type ON titles(type);
CREATE INDEX idx_titles_match_status ON titles(match_status);
CREATE INDEX idx_titles_imdb_id ON titles(imdb_id);
CREATE INDEX idx_titles_tmdb_id ON titles(tmdb_id);
CREATE INDEX idx_titles_anilist_id ON titles(anilist_id);
CREATE INDEX idx_titles_plex_rating_key ON titles(plex_rating_key);
CREATE INDEX idx_title_names_title_id ON title_names(title_id);
CREATE INDEX idx_seasons_title_id ON seasons(title_id);
CREATE INDEX idx_episodes_season_id ON episodes(season_id);
CREATE INDEX idx_watch_events_title_id ON watch_events(title_id);
