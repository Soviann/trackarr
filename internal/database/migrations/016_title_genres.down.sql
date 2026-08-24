-- Restore genres JSON column
ALTER TABLE titles ADD COLUMN genres TEXT;

UPDATE titles
SET genres = (
  SELECT json_group_array(genre)
  FROM title_genres
  WHERE title_id = titles.id
);

DROP TABLE IF EXISTS title_genres;
