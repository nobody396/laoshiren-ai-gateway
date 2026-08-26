-- Shadow promotion assessments correlate immutable routing decisions with the
-- authoritative final Customer Request outcome. Keep both identity paths
-- indexed so the bounded assessment does not rescan the observation ledger for
-- every decision.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE INDEX IF NOT EXISTS idx_reliability_observations_customer_client_request
    ON reliability_observations (client_request_id, observed_at DESC, id DESC)
    WHERE fact_type = 'customer_request' AND client_request_id <> '';

CREATE INDEX IF NOT EXISTS idx_reliability_observations_customer_request
    ON reliability_observations (request_id, observed_at DESC, id DESC)
    WHERE fact_type = 'customer_request' AND request_id <> '';
