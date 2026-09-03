-- Explicit authorization snapshot. Existing single-group keys remain NULL.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS group_ids JSONB;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'api_keys'::regclass AND conname = 'api_keys_group_ids_array') THEN
        ALTER TABLE api_keys ADD CONSTRAINT api_keys_group_ids_array
            CHECK (group_ids IS NULL OR (jsonb_typeof(group_ids) = 'array'
                AND jsonb_array_length(group_ids) BETWEEN 1 AND 100
                AND group_id IS NULL));
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_api_keys_group_ids ON api_keys USING gin (group_ids);
