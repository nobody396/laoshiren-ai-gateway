# Affiliate V3 commercial cutover

This runbook separates code deployment from monetary activation. Passing the
isolated staging stack authorizes neither a production deploy nor switching the
affiliate program to `live`.

## Final catalog

All three products last 31 days and have one monthly quota only. There is no
daily or weekly limit.

| Product | LDXP price | Direct price | Internal monthly credits | Customer display |
| --- | ---: | ---: | ---: | ---: |
| Plus | ¥259 | ¥249 | 300 | ⚡3,000 |
| Pro | ¥729 | ¥699 | 900 | ⚡9,000 |
| Max | ¥1,549 | ¥1,499 | 2,000 | ⚡20,000 |

The customer display scale is 10x. Accounting, subscription limits and
confirmed-consumption proration use internal credits; customer-facing pages
show scaled `⚡` values. The fixed public group multipliers are GPT `0.50` and
Claude/MAX `2.40`.

## Final referral and partner rules

- Ordinary first paid purchase: inviter and invitee each receive 5% platform
  credits at T+0. There is no fixed `⚡5` reward and no minimum purchase amount.
- Partner qualification route A: ten direct users, each with at least ¥20
  confirmed consumption, and at least ¥1,000 direct-team confirmed consumption.
- Partner qualification route B: at least ¥2,000 direct-team confirmed
  consumption. The applicant's own consumption does not count.
- Reaching a threshold only enables an application. The platform must approve
  the applicant before partner privileges become active.
- Each partner-bound confirmed-consumption event has one fixed 10% pool.
  Customer platform-credit rebate plus direct partner cash commission must equal
  10%; there is no recursive or self commission.
- Mature cash is T+0. A withdrawal is user-requested and user-visible only as
  `处理中 -> 已到账`. Cash-to-credit conversion uses the fixed 1.2x multiplier.

## Production prerequisites

1. Create or update the paired GPT and Claude/MAX `credit` subscription groups
   for Plus, Pro and Max.
2. Verify monthly limits are respectively `300`, `900` and `2000` internal
   credits; daily and weekly limits must be unset/zero; validity is 31 days.
3. Verify group multipliers are GPT `0.50` and Claude/MAX `2.40`.
4. Create or update one LDXP item for each SKU with prices ¥259, ¥729 and
   ¥1,549. Record the final item URL for the matching frontend catalog entry.
   The LDXP purchase button must remain disabled while a URL is blank.
5. Generate only sale-attributed subscription cards with the actual sale price
   recorded in `redeem_codes.value`. Inventory, gift, compensation, internal
   test and migration cards remain affiliate-ineligible.
6. Verify every LDXP delivery maps to the correct group IDs, 31-day validity and
   card face value before publishing the item.
7. Re-run the commercial policy gate with 3% shop fee, maximum 12% converted
   alliance burden, 2% operational reserve, ¥0.37 blended pressure cost per
   internal credit and 35% minimum stress margin.
8. Reconcile one owned test order through card delivery, redemption, monthly
   entitlement, pro-rata confirmed consumption, reward/commission projection
   and finance ledger. Never use a customer identity for release testing.

## Controlled release

1. Merge the exact tested tree to `main`, require successful push CI and select
   only the immutable image digest from its verified artifact.
2. Deploy code with Affiliate V3 mode `off`.
3. Verify migrations, indexes, constraints, triggers, `/livez`, `/readyz`,
   static assets and authentication boundaries. No reward writes are allowed.
4. Validate the production group/catalog mapping and LDXP delivery while the
   program remains `off`.
5. Enter `shadow` and run one complete owned paid-card and usage cycle. Shadow
   must calculate expected rewards and commissions without writing platform
   credits, cash wallet entries or withdrawals.
6. Reconcile the shadow projection against the finance ledger, subscription
   usage and the 35% margin gate. Any duplicate, omission or imbalance blocks
   activation.
7. Only after the owner has explicitly authorized this release and every prior
   gate is green, enter `live` with a frozen `started_at`. Historical purchases,
   balances and usage remain ineligible.
8. Run the smallest owned-identity production smoke, record non-sensitive
   evidence and complete the mandatory public Changelog assessment.

## Rollback boundary

- `live -> shadow` stops new monetary settlement while retaining observability.
- `shadow -> off` stops new Affiliate V3 performance recording.
- Existing posted rewards, cash commission and withdrawals are never deleted;
  corrections use audited holds and reversals.
- Application rollback uses the previously verified immutable image digest.
- If partner self-consumption commission has ever produced a
  `PARTNER_SELF_USAGE` purchase lot, follow
  `docs/ops/AFFILIATE_SELF_COMMISSION_CUTOVER.md`; rollback below its recorded
  minimum compatible digest is forbidden.
- Never point staging databases or volumes at production.
