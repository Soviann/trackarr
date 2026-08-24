-- 039_cleanup_title_names_duplicates.up.sql

-- 1. Remove duplicate title names per (title_id, language), keeping the primary and newest entry.
DELETE FROM title_names
WHERE id IN (
    SELECT id FROM (
        SELECT id,
               ROW_NUMBER() OVER (
                   PARTITION BY title_id, language
                   ORDER BY is_primary DESC, id DESC
               ) as rn
        FROM title_names
    ) WHERE rn > 1
);

-- 2. Ensure at most one primary name per title (preferring English, then newest entry).
UPDATE title_names
SET is_primary = 0
WHERE id IN (
    SELECT id FROM (
        SELECT id,
               ROW_NUMBER() OVER (
                   PARTITION BY title_id
                   ORDER BY CASE WHEN language = 'en' THEN 1 ELSE 2 END, id DESC
               ) as rn
        FROM title_names
        WHERE is_primary = 1
    ) WHERE rn > 1
);

-- 3. Ensure at least one primary name per title if any names exist.
UPDATE title_names
SET is_primary = 1
WHERE id IN (
    SELECT id FROM (
        SELECT tn.id,
               ROW_NUMBER() OVER (
                   PARTITION BY tn.title_id
                   ORDER BY CASE WHEN tn.language = 'en' THEN 1 ELSE 2 END, tn.id ASC
               ) as rn
        FROM title_names tn
        WHERE NOT EXISTS (
            SELECT 1 FROM title_names p
            WHERE p.title_id = tn.title_id AND p.is_primary = 1
        )
    ) WHERE rn = 1
);
