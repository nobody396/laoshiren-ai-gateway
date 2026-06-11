-- Default new error display rules to fail closed.
-- Existing explicit rules keep their current behavior; this only changes DB defaults.

ALTER TABLE error_passthrough_rules
ALTER COLUMN passthrough_code SET DEFAULT false,
ALTER COLUMN passthrough_body SET DEFAULT false;
