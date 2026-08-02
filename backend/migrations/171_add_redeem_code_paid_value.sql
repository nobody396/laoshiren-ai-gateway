-- Store actual cash paid separately from the balance credited by a redeem code.
-- Existing codes keep zero and use the historical value fallback in service code.
ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS paid_value DECIMAL(20,8) NOT NULL DEFAULT 0;

ALTER TABLE redeem_codes
    DROP CONSTRAINT IF EXISTS redeem_codes_paid_value_valid;

ALTER TABLE redeem_codes
    ADD CONSTRAINT redeem_codes_paid_value_valid
    CHECK (paid_value >= 0 AND paid_value <= value);
