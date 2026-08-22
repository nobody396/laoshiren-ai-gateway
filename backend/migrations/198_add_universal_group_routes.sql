ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS universal_routes JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_universal_routes_array;

ALTER TABLE groups
    ADD CONSTRAINT groups_universal_routes_array
    CHECK (jsonb_typeof(universal_routes) = 'array');

COMMENT ON COLUMN groups.universal_routes IS
    'Universal group routes from public model and inbound protocol to an existing concrete target group';
