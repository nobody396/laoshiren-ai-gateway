-- Optional read-path index for the active-probe-to-Shadow prior bridge.
-- Some upstream installations do not have the operational benchmark table;
-- the migration must remain a no-op there.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

DO $$
BEGIN
  IF to_regclass('public.base_url_benchmarks') IS NOT NULL
     AND NOT EXISTS (
       SELECT required.column_name
       FROM unnest(ARRAY[
         'product', 'account_id', 'model', 'base_url', 'sampled_at',
         'passed', 'first_text_ms', 'total_ms', 'error_type', 'stream_complete'
       ]) AS required(column_name)
       WHERE NOT EXISTS (
         SELECT 1
         FROM information_schema.columns AS actual
         WHERE actual.table_schema = 'public'
           AND actual.table_name = 'base_url_benchmarks'
           AND actual.column_name = required.column_name
       )
     ) THEN
    CREATE INDEX IF NOT EXISTS idx_base_url_benchmarks_shadow_prior
      ON base_url_benchmarks (account_id, model, sampled_at DESC)
      INCLUDE (base_url, passed, first_text_ms, total_ms, error_type, stream_complete)
      WHERE product = 'gpt';
  END IF;
END
$$;
