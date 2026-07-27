-- Reconcile last_watched_at and first_watched_at for titles with watch events
UPDATE titles SET
    last_watched_at = (
        SELECT MAX(lw) FROM (
            SELECT titles.last_watched_at AS lw WHERE titles.last_watched_at IS NOT NULL
            UNION ALL
            SELECT created_at AS lw FROM watch_events WHERE title_id = titles.id
        )
    ),
    first_watched_at = (
        SELECT MIN(fw) FROM (
            SELECT titles.first_watched_at AS fw WHERE titles.first_watched_at IS NOT NULL
            UNION ALL
            SELECT created_at AS fw FROM watch_events WHERE title_id = titles.id
        )
    )
WHERE EXISTS (SELECT 1 FROM watch_events WHERE title_id = titles.id)
  AND (last_watched_at IS NULL OR last_watched_at < (SELECT MAX(created_at) FROM watch_events WHERE title_id = titles.id));
