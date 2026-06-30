-- Align paid monthly-card products to the customer-facing 31-day cycle.
--
-- Keep this migration data-only: it updates monthly-card groups and unused
-- monthly-card inventory so future redemptions/orders grant the same 31-day
-- entitlement shown in the UI.

WITH monthly_groups(id) AS (
    VALUES
        (7),  (8),  (9),  (10),
        (11), (12), (13), (14),
        (18), (19)
)
UPDATE groups g
SET
    default_validity_days = 31,
    updated_at = NOW()
FROM monthly_groups mg
WHERE g.id = mg.id
  AND g.deleted_at IS NULL
  AND g.subscription_type = 'credit'
  AND g.default_validity_days <> 31;

WITH monthly_groups(id) AS (
    VALUES
        (7),  (8),  (9),  (10),
        (11), (12), (13), (14),
        (18), (19)
)
UPDATE redeem_codes rc
SET
    validity_days = 31,
    internal_notes = CASE
        WHEN COALESCE(rc.internal_notes, '') = '' THEN 'monthly-card cycle aligned to 31 days'
        WHEN rc.internal_notes LIKE '%monthly-card cycle aligned to 31 days%' THEN rc.internal_notes
        ELSE rc.internal_notes || E'\nmonthly-card cycle aligned to 31 days'
    END,
    updated_at = NOW()
WHERE rc.type = 'subscription'
  AND rc.status = 'unused'
  AND rc.validity_days = 30
  AND (
      rc.group_id IN (SELECT id FROM monthly_groups)
      OR EXISTS (
          SELECT 1
          FROM monthly_groups mg
          WHERE rc.group_ids @> jsonb_build_array(mg.id)
      )
  );

WITH monthly_groups(id) AS (
    VALUES
        (7),  (8),  (9),  (10),
        (11), (12), (13), (14),
        (18), (19)
)
UPDATE payment_orders po
SET
    validity_days = 31,
    updated_at = NOW()
WHERE po.group_id IN (SELECT id FROM monthly_groups)
  AND po.status = 'pending'
  AND po.validity_days = 30;
