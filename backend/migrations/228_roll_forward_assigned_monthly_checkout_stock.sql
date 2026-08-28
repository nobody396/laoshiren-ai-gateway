-- Assigned native-checkout inventory has already been snapshotted into its
-- order and cannot be sold again.  It must remain as historical evidence, but
-- it should not permanently freeze the public offer after all old stock has
-- been consumed.  Only unassigned inventory is still an in-flight commercial
-- contract that must block an entitlement change.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

CREATE OR REPLACE FUNCTION guard_native_checkout_stocked_offer_semantics()
RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM native_checkout_redeem_inventory
        WHERE offer_code = OLD.code
          AND assigned_order_id IS NULL
    ) AND (
        NEW.redeem_type IS DISTINCT FROM OLD.redeem_type
        OR NEW.redeem_value IS DISTINCT FROM OLD.redeem_value
        OR NEW.redeem_paid_value IS DISTINCT FROM OLD.redeem_paid_value
        OR NEW.redeem_purpose IS DISTINCT FROM OLD.redeem_purpose
        OR NEW.redeem_sales_status IS DISTINCT FROM OLD.redeem_sales_status
        OR NEW.redeem_group_ids IS DISTINCT FROM OLD.redeem_group_ids
        OR NEW.redeem_validity_days IS DISTINCT FROM OLD.redeem_validity_days
    ) THEN
        RAISE EXCEPTION 'cannot change stocked native checkout offer semantics: %', OLD.code
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- The production Plus stock created under the original GPT+Claude contract is
-- already assigned.  Preserve that historical row while advancing all future
-- Plus purchases to the current GPT+Claude+Grok entitlement.
UPDATE native_checkout_offers
SET redeem_group_ids = '[40,41,48]'::jsonb, updated_at = NOW()
WHERE code = 'plus' AND provider = 'easypay'
  AND NOT EXISTS (
      SELECT 1
      FROM native_checkout_redeem_inventory
      WHERE offer_code = 'plus'
        AND assigned_order_id IS NULL
  );

RESET statement_timeout;
RESET lock_timeout;
