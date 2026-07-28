-- Backfill logs.cache_tokens from the JSON `other` column.
--
-- Context: the cache_tokens column was added to the logs table so that
-- dashboard/stat queries can sum cache tokens without parsing JSON on every
-- row. This script populates it retroactively for rows created BEFORE the
-- column existed (those rows still carry cache_tokens inside the `other`
-- JSON payload).
--
-- Run ONCE after deploying the column. Idempotent: only updates rows where
-- cache_tokens is currently 0 but the JSON carries a non-zero value.
--
-- PostgreSQL version. For MySQL/SQLite, adapt the JSON access syntax
-- (MySQL: JSON_EXTRACT(other, '$.cache_tokens'); SQLite: json_extract(...)).

UPDATE logs
SET cache_tokens = COALESCE((other::jsonb ->> 'cache_tokens')::bigint, 0)
WHERE cache_tokens = 0
  AND other IS NOT NULL
  AND other <> ''
  AND other ~ '^\s*\{'
  AND jsonb_typeof(other::jsonb) = 'object'
  AND COALESCE((other::jsonb ->> 'cache_tokens')::bigint, 0) > 0;

-- Verify the result (tokens total now includes cache):
-- SELECT
--   SUM(prompt_tokens + completion_tokens) AS without_cache,
--   SUM(prompt_tokens + completion_tokens + cache_tokens) AS with_cache,
--   SUM(cache_tokens) AS cache_only
-- FROM logs;

-- =====================================================================
-- Part 2: Backfill quota_data.token_used so dashboard/models and rankings
-- also reflect cache. quota_data aggregates by (user, model, hour bucket).
-- We add the cache_tokens summed from logs (already backfilled in Part 1),
-- matched on the same hourly bucket, user and model.
--
-- NOTE: only run AFTER Part 1. Idempotent via the guard on cache_tokens=0
-- being preserved in logs (Part 1 sets it once), but to be safe this UPDATE
-- is written as: token_used += aggregated cache from logs. If run twice it
-- would double-count, so run Part 2 only once.
-- =====================================================================

UPDATE quota_data qd
SET token_used = qd.token_used + agg.cache_sum
FROM (
  SELECT
    user_id,
    model_name,
    (created_at - (created_at % 3600)) AS hour_bucket,
    SUM(cache_tokens) AS cache_sum
  FROM logs
  WHERE type = 2          -- LogTypeConsume
    AND cache_tokens > 0
  GROUP BY user_id, model_name, (created_at - (created_at % 3600))
) agg
WHERE qd.user_id = agg.user_id
  AND qd.model_name = agg.model_name
  AND qd.created_at = agg.hour_bucket;

-- Verify quota_data now includes cache (compare with logs totals):
-- SELECT SUM(token_used) AS quota_data_tokens FROM quota_data;
-- SELECT SUM(prompt_tokens + completion_tokens + cache_tokens) AS logs_tokens FROM logs;
