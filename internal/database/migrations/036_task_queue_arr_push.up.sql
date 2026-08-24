-- Étend la contrainte CHECK sur task_queue.task_type pour autoriser les push
-- Radarr et Sonarr. SQLite n'acceptant pas la modification directe d'une 
-- contrainte CHECK, on recrée la table via copie.
CREATE TABLE task_queue_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_type TEXT NOT NULL CHECK(task_type IN ('enrichment', 'refresh', 'cover_fetch', 'anilist_push_season', 'anilist_push_movie', 'radarr_push', 'sonarr_push')),
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'sleeping', 'dead')),
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    day INTEGER NOT NULL DEFAULT 1,
    last_error TEXT,
    run_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    dedup_key TEXT
);

INSERT INTO task_queue_new (id, task_type, payload, status, attempts, max_attempts, day, last_error, run_at, created_at, updated_at, dedup_key)
SELECT id, task_type, payload, status, attempts, max_attempts, day, last_error, run_at, created_at, updated_at, dedup_key
FROM task_queue;

DROP TABLE task_queue;
ALTER TABLE task_queue_new RENAME TO task_queue;

CREATE INDEX idx_task_queue_status_run_at ON task_queue(status, run_at);
CREATE UNIQUE INDEX idx_task_queue_dedup ON task_queue(dedup_key) WHERE status IN ('pending', 'running', 'sleeping');
