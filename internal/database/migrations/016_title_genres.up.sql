CREATE TABLE title_genres (
  title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
  genre    TEXT NOT NULL
);

CREATE INDEX idx_title_genres_genre    ON title_genres(genre);
CREATE INDEX idx_title_genres_title_id ON title_genres(title_id);

-- Populate from existing JSON genres column
INSERT INTO title_genres (title_id, genre)
SELECT titles.id, value
FROM titles, json_each(titles.genres)
WHERE titles.genres IS NOT NULL AND titles.genres != '[]' AND titles.genres != '';

-- Drop the JSON column (requires SQLite >= 3.35)
ALTER TABLE titles DROP COLUMN genres;
