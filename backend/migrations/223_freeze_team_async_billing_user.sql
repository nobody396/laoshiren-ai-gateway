-- Freeze the payer captured in usage_logs.user_id for already accepted async
-- Team tasks. Team ownership changes must not rewrite an incurred charge.
CREATE OR REPLACE FUNCTION fill_usage_log_team_attribution()
RETURNS TRIGGER AS $$
DECLARE
    resolved_team_id BIGINT;
    resolved_actor_user_id BIGINT;
    resolved_billing_user_id BIGINT;
BEGIN
    SELECT ak.team_id, ak.user_id INTO resolved_team_id, resolved_actor_user_id
    FROM api_keys ak WHERE ak.id = NEW.api_key_id;
    NEW.team_id := COALESCE(NEW.team_id, resolved_team_id);
    NEW.actor_user_id := COALESCE(NEW.actor_user_id, resolved_actor_user_id, NEW.user_id);
    IF NEW.team_id IS NOT NULL THEN
        SELECT tm.user_id INTO resolved_billing_user_id
        FROM team_memberships tm
        WHERE tm.team_id = NEW.team_id AND tm.left_at IS NULL AND tm.role = 'owner'
        LIMIT 1;
    END IF;
    -- Local user_id is the payer frozen at request acceptance. Current Owner is
    -- only a fallback for legacy writers that do not provide a user_id.
    NEW.billing_user_id := COALESCE(NEW.billing_user_id, NEW.user_id, resolved_billing_user_id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
