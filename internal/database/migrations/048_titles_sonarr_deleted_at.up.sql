ALTER TABLE titles ADD COLUMN sonarr_deleted_at TIMESTAMP;

-- Rétrocompatibilité : si une tâche sonarr_delete existe encore dans la file, horodate la suppression.
UPDATE titles
SET sonarr_deleted_at = (
    SELECT created_at FROM task_queue
    WHERE task_type = 'sonarr_delete'
      AND json_extract(payload, '$.title_id') = titles.id
    LIMIT 1
)
WHERE id IN (
    SELECT json_extract(payload, '$.title_id')
    FROM task_queue
    WHERE task_type = 'sonarr_delete'
);
