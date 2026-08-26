-- Customer-impact evidence must resolve to a real user. Keep the constraint
-- NOT VALID so historical append-only evidence remains intact; PostgreSQL still
-- enforces it for every new insert or update.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE reliability_observations
    DROP CONSTRAINT IF EXISTS reliability_observation_customer_identity_check;

ALTER TABLE reliability_observations
    ADD CONSTRAINT reliability_observation_customer_identity_check CHECK (
        NOT customer_impact OR (user_id IS NOT NULL AND user_id > 0)
    ) NOT VALID;
