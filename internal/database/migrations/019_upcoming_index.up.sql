-- Partial index pour accélérer /api/titles/upcoming (filtre sur next_air_date)
CREATE INDEX IF NOT EXISTS idx_titles_next_air_date
    ON titles(next_air_date)
    WHERE next_air_date IS NOT NULL;
