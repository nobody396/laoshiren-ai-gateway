# Affiliate V2 commercial cutover

This runbook separates code acceptance from external commerce changes. None of
the items below is authorized for production merely because the Affiliate V2
staging stack passes.

## Target catalog

| Product | LDXP price | Direct price | Internal monthly credits | Customer display | Daily internal | Daily display |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Starter | ¥259 | ¥249 | 240 | ⚡2400 | 8 | ⚡80 |
| Lite | ¥469 | ¥459 | 450 | ⚡4500 | 15 | ⚡150 |
| Pro | ¥869 | ¥839 | 850 | ⚡8500 | 28 | ⚡280 |

The customer display scale is 10x. Accounting, subscription limits and
confirmed-consumption proration use the internal units; customer-facing pages
show the scaled `⚡` values. GPT target multiplier is 0.42 and Claude/MAX is
2.40.

## Production prerequisites

1. Create or update the paired GPT and Claude/MAX credit subscription groups
   for Starter, Lite and Pro with one shared logical quota per product.
2. Verify the group limits are respectively 240/8, 450/15 and 850/28 internal
   monthly/daily credits and that the customer UI shows the 10x values.
3. Create a dedicated Starter LDXP item. The staging branch intentionally keeps
   its external URL blank until that item exists.
4. Update the existing Lite and Pro LDXP item prices. Their current URLs remain
   wired in the staging branch, but the external item prices are not changed by
   this repository.
5. Generate only sale-attributed subscription cards with the actual sale price
   recorded in `redeem_codes.value`; inventory, gift, compensation, internal
   test and migration cards must remain affiliate-ineligible.
6. Verify each LDXP delivery maps to the correct paired group IDs, validity
   window and card face value before publishing the item.
7. Re-run the commercial policy gate with 3% shop fee, full 10% alliance pool
   and 35% minimum stress margin.
8. Reconcile one test order per SKU through LDXP order, redeem code, monthly
   entitlement, pro-rata confirmed consumption and finance ledger.

## Controlled release

1. Deploy code with Affiliate V2 mode `off`.
2. Apply migrations and verify all new tables, indexes and triggers.
3. Validate the external catalog and group mapping while rewards remain off.
4. Enter `shadow` and observe at least one complete paid and usage cycle without
   any reward or cash writes.
5. Reconcile shadow calculations against the finance ledger and margin gate.
6. Only after explicit production approval, enter `live` with a frozen
   `started_at`; historical purchases, balances and usage remain ineligible.
7. Keep legacy `commission_records` and `agent_settlements` read-only for V2
   events. The gateway and first-paid hooks stop legacy reward generation once
   V2 is live.

## Rollback boundary

- `live -> shadow` stops new monetary settlement while retaining observability.
- `shadow -> off` stops new Affiliate V2 performance recording.
- Existing posted rewards, cash commission and withdrawals are never deleted;
  corrections use audited holds and reversals.
- Do not roll back by pointing the staging database or volumes at production.
