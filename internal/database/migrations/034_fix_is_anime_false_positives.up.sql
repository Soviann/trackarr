-- Create a temporary table or CTE to identify the false positives
WITH false_positives AS (
    SELECT id 
    FROM titles 
    WHERE (is_anime = 1 OR anilist_id IS NOT NULL)
      AND (
          (origin_country NOT IN ('JP', 'CN', 'KR', 'TW', '') AND origin_country IS NOT NULL)
          OR 
          ((origin_country IS NULL OR origin_country = '') AND id NOT IN (SELECT title_id FROM title_genres WHERE genre = 'Animation'))
      )
)
-- 1. Remove anilist mappings for seasons of these titles
DELETE FROM season_external_ids 
WHERE provider = 'anilist' 
  AND season_id IN (
      SELECT s.id 
      FROM seasons s 
      JOIN false_positives fp ON s.title_id = fp.id
  );

-- 2. Reset the flags and anilist_id on the titles themselves
WITH false_positives AS (
    SELECT id 
    FROM titles 
    WHERE (is_anime = 1 OR anilist_id IS NOT NULL)
      AND (
          (origin_country NOT IN ('JP', 'CN', 'KR', 'TW', '') AND origin_country IS NOT NULL)
          OR 
          ((origin_country IS NULL OR origin_country = '') AND id NOT IN (SELECT title_id FROM title_genres WHERE genre = 'Animation'))
      )
)
UPDATE titles
SET is_anime = 0, anilist_id = NULL
WHERE id IN (SELECT id FROM false_positives);
