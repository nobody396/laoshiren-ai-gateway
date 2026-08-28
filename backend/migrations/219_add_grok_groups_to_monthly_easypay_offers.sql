-- Add the Grok monthly-card groups to the three native EasyPay monthly-card
-- offers. Migration 212 minted these offers with only the GPT/Claude group
-- pairs, so monthly cards purchased through the native checkout never
-- activated the Grok host bundled in the current Plus/Pro/Max catalog.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

-- Production may already hold inventory minted under the original GPT/Claude
-- entitlement. Migration 185 deliberately makes such offer semantics
-- immutable. Only upgrade an offer that has no registered stock; stocked
-- offers keep their existing contract and require an explicit inventory
-- versioning operation instead of blocking every subsequent deployment.

UPDATE native_checkout_offers
SET redeem_group_ids = '[40,41,48]'::jsonb, updated_at = NOW()
WHERE code = 'plus' AND provider = 'easypay'
  AND NOT EXISTS (
      SELECT 1 FROM native_checkout_redeem_inventory
      WHERE offer_code = 'plus'
  );

UPDATE native_checkout_offers
SET redeem_group_ids = '[42,43,49]'::jsonb, updated_at = NOW()
WHERE code = 'pro' AND provider = 'easypay'
  AND NOT EXISTS (
      SELECT 1 FROM native_checkout_redeem_inventory
      WHERE offer_code = 'pro'
  );

UPDATE native_checkout_offers
SET redeem_group_ids = '[44,45,50]'::jsonb, updated_at = NOW()
WHERE code = 'max' AND provider = 'easypay'
  AND NOT EXISTS (
      SELECT 1 FROM native_checkout_redeem_inventory
      WHERE offer_code = 'max'
  );

RESET statement_timeout;
RESET lock_timeout;
