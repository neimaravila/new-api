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
