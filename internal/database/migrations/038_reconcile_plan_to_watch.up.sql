-- 038_reconcile_plan_to_watch.up.sql

-- 1. Ended or cancelled series with all aired episodes watched -> completed
UPDATE titles
SET status = 'completed', updated_at = CURRENT_TIMESTAMP
WHERE status = 'plan_to_watch'
  AND series_status IN ('ended', 'cancelled')
  AND EXISTS (
      SELECT 1 FROM episodes e
      JOIN seasons s ON e.season_id = s.id
      WHERE s.title_id = titles.id AND e.watched = 1
  )
  AND NOT EXISTS (
      SELECT 1 FROM episodes e
      JOIN seasons s ON e.season_id = s.id
      WHERE s.title_id = titles.id
        AND e.watched = 0
        AND e.air_date IS NOT NULL
        AND e.air_date != ''
        AND e.air_date <= date('now')
  );

-- 2. Any remaining plan_to_watch titles with watched episodes -> watching
UPDATE titles
SET status = 'watching', updated_at = CURRENT_TIMESTAMP
WHERE status = 'plan_to_watch'
  AND EXISTS (
      SELECT 1 FROM episodes e
      JOIN seasons s ON e.season_id = s.id
      WHERE s.title_id = titles.id AND e.watched = 1
  );
