-- Backfill cache tokens for historical data so dashboard/rankings/logs all
-- reflect total tokens (prompt + completion + cache).
--
-- Context: the cache_tokens column was added to the logs table so that
-- dashboard/stat queries can sum cache tokens without parsing JSON on every
-- row. This script populates it retroactively, then adds the historical
-- cache to quota_data.token_used (which feeds dashboard/models + rankings).
--
-- TWO PARTS, run in order:
--   Part 1 (logs.cache_tokens):      idempotent — safe to run repeatedly.
--   Part 2 (quota_data.token_used):  NOT idempotent — run exactly once,
--                                    only after Part 1.
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
-- also reflect cache. quota_data aggregates by the tuple
-- (user_id, username, model_name, hour_bucket, use_group, token_id,
--  channel_id, node_name), so the cache must be aggregated on the SAME
-- dimensions, otherwise multiple buckets sharing user+model+hour each get
-- the full cache sum and the total is inflated.
--
-- In the `logs` table the group column is named "group" (a reserved word in
-- Postgres, hence the double quotes) and maps to quota_data.use_group.
-- username and node_name are omitted because user_id+model_name already pin
-- username, and node_name is constant per deployment in practice.
--
-- Run ONCE, AFTER Part 1, and only once: it is `token_used += cache`, so a
-- second run double-counts.
-- =====================================================================

UPDATE quota_data qd
SET token_used = qd.token_used + agg.cache_sum
FROM (
  SELECT
    user_id,
    model_name,
    (created_at - (created_at % 3600)) AS hour_bucket,
    "group" AS grp,
    token_id,
    channel_id,
    SUM(cache_tokens) AS cache_sum
  FROM logs
  WHERE type = 2          -- LogTypeConsume
    AND cache_tokens > 0
  GROUP BY user_id, model_name, (created_at - (created_at % 3600)), "group", token_id, channel_id
) agg
WHERE qd.user_id = agg.user_id
  AND qd.model_name = agg.model_name
  AND qd.created_at = agg.hour_bucket
  AND COALESCE(qd.use_group, '') = COALESCE(agg.grp, '')
  AND qd.token_id = agg.token_id
  AND qd.channel_id = agg.channel_id;

-- Verify quota_data now includes cache (should be within ~1% of logs totals):
-- SELECT SUM(token_used) AS quota_data_tokens FROM quota_data;
-- SELECT SUM(prompt_tokens + completion_tokens + cache_tokens) AS logs_tokens FROM logs;
