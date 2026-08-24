-- Fix potential index corruption that causes "database disk image is malformed"
REINDEX idx_titles_is_anime;

-- 1. Remove anilist mappings for seasons of these titles
DELETE FROM season_external_ids 
WHERE provider = 'anilist' 
  AND season_id IN (
      SELECT s.id 
      FROM seasons s 
      JOIN titles t ON s.title_id = t.id
      WHERE (t.is_anime = 1 OR t.anilist_id IS NOT NULL)
        AND (
            (t.origin_country NOT IN ('JP', 'CN', 'KR', 'TW', '') AND t.origin_country IS NOT NULL)
            OR 
            ((t.origin_country IS NULL OR t.origin_country = '') AND t.id NOT IN (SELECT title_id FROM title_genres WHERE genre = 'Animation'))
        )
  );

-- 2. Reset the flags and anilist_id on the titles themselves
UPDATE titles
SET is_anime = 0, anilist_id = NULL
WHERE (is_anime = 1 OR anilist_id IS NOT NULL)
  AND (
      (origin_country NOT IN ('JP', 'CN', 'KR', 'TW', '') AND origin_country IS NOT NULL)
      OR 
      ((origin_country IS NULL OR origin_country = '') AND id NOT IN (SELECT title_id FROM title_genres WHERE genre = 'Animation'))
  );
