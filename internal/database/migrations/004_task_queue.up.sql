CREATE TABLE task_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_type TEXT NOT NULL CHECK(task_type IN ('enrichment', 'refresh', 'cover_fetch')),
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

CREATE INDEX idx_task_queue_status_run_at ON task_queue(status, run_at);
CREATE UNIQUE INDEX idx_task_queue_dedup ON task_queue(dedup_key) WHERE status IN ('pending', 'running', 'sleeping');
