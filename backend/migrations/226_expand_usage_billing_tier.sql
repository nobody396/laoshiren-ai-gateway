SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE usage_logs
    ALTER COLUMN billing_tier TYPE VARCHAR(255);

COMMENT ON COLUMN usage_logs.billing_tier IS
    'Frozen context/time pricing evidence selected for the request; includes request-start pricing instant when time pricing matched';
