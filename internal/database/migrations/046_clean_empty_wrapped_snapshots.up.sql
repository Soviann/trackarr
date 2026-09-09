-- Supprime les instantanés Wrapped créés par erreur avec 0 titres (années sans visionnages).
DELETE FROM wrapped_snapshots
WHERE json_extract(data_json, '$.overview.total_titles') = 0
   OR json_extract(data_json, '$.overview.total_titles') IS NULL;
