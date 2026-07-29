# Monthly Credit Card Products

This runbook is the source of truth for the current credits-based monthly card
lineup. Cards are distributed through subscription redeem codes. A redeemed
card grants or queues a 31-day subscription entitlement.

## Billing rules

- The public lineup is exactly `Plus`, `Pro` and `Max`.
- Each SKU has a fixed monthly quota. There is no daily or weekly limit.
- Customer display is 10x the internal subscription-credit value.
- Fixed public multipliers are GPT `0.50` and Claude/MAX `2.40`.
- GPT and Claude/MAX groups for the same SKU represent the same logical quota;
  consumption must not create two independent allowances.
- Unused quota expires at the end of 31 days and does not roll over.
- An early same-SKU renewal queues the next period instead of mixing two active
  periods.

## Product lineup

| Product | LDXP price | Direct price | Internal monthly credits | Customer display | Validity |
| --- | ---: | ---: | ---: | ---: | ---: |
| Plus | ¥259 | ¥249 | 300 | ⚡3,000 | 31 days |
| Pro | ¥729 | ¥699 | 900 | ⚡9,000 | 31 days |
| Max | ¥1,549 | ¥1,499 | 2,000 | ⚡20,000 | 31 days |

## Group configuration

Create paired GPT and Claude/MAX `credit` subscription groups for each product.
Daily and weekly limits are unset/zero.

| Product | GPT group | Claude/MAX group | monthly_limit_usd | validity_days |
| --- | --- | --- | ---: | ---: |
| Plus | `GPT Plus 月卡组` | `Claude Plus 月卡组` | 300 | 31 |
| Pro | `GPT Pro V3 月卡组` | `Claude Pro V3 月卡组` | 900 | 31 |
| Max | `GPT Max V3 月卡组` | `Claude Max V3 月卡组` | 2,000 | 31 |

Group `rate_multiplier` targets:

| Group family | rate_multiplier |
| --- | ---: |
| GPT Plus/Pro/Max | 0.50 |
| Claude/MAX Plus/Pro/Max | 2.40 |

## LDXP publication gate

- One LDXP product must exist for each SKU.
- Prices must match the table above.
- Delivery must use a sale-attributed subscription card bound to the matching
  group and 31-day validity.
- The frontend LDXP button remains disabled until the matching product URL is
  recorded in the catalog.
- Before public sale, reconcile one owned test order per SKU. Do not use
  customer identities or balances for release testing.

## Redeem-code payload template

Use `type=subscription`. The `value` field records the actual face value used
for reconciliation and affiliate eligibility; entitlement quota comes from the
bound group.

```json
{
  "count": 100,
  "type": "subscription",
  "value": 259,
  "group_id": 0,
  "validity_days": 31,
  "batch_name": "monthly-plus-20260728",
  "purpose": "sale_recharge",
  "sales_status": "inventory",
  "sales_channel": "liandong_shop",
  "internal_notes": "Plus; 300 internal monthly credits; display 3000; 31 days"
}
```

Every new sellable batch must pass the Affiliate V3 commercial margin guard
before generation. Gift, compensation, test and migration cards are excluded
from affiliate rewards and partner commissions.
