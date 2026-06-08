# Monthly Credit Card Products

This runbook is the source of truth for the credits-based monthly card lineup.
Cards are distributed through subscription redeem codes. The redeem code grants
or extends a subscription group; the group controls daily credits and billing.

## Billing Rules

- GPT Pro usage: 0.6 credits = 1 USD nominal usage.
- Claude Max usage: 1.5 credits = 1 USD nominal usage.
- GPT Pro and Claude Max share the same daily credits pool.
- Daily credits do not roll over.
- Validity is 30 days unless the batch explicitly says otherwise.

## Product Lineup

| Product | Daily Credits | GPT Pro Usage | Claude Max Usage | Description |
| --- | ---: | ---: | ---: | --- |
| Lite 月卡 | 15 credits/day | about 25 USD/day | about 10 USD/day | 每天 15 个 credits，适合尝鲜体验。 |
| Pro 月卡 | 30 credits/day | about 50 USD/day | about 20 USD/day | 每天 30 个 credits。 |
| Max 月卡 | 40 credits/day | about 66.67 USD/day | about 26.67 USD/day | 每天 40 个 credits。 |
| Ultra 月卡 | 50 credits/day | about 83.33 USD/day | about 33.33 USD/day | 每天 50 个 credits。 |

## Customer-Facing Copy

Lite 月卡：每天 15 credits。使用 GPT Pro 约可用 25 刀/天；使用 Claude Max
约可用 10 刀/天。

Pro 月卡：每天 30 credits。使用 GPT Pro 约可用 50 刀/天；使用 Claude Max
约可用 20 刀/天。

Max 月卡：每天 40 credits。使用 GPT Pro 约可用 66.67 刀/天；使用 Claude Max
约可用 26.67 刀/天。

Ultra 月卡：每天 50 credits。使用 GPT Pro 约可用 83.33 刀/天；使用 Claude Max
约可用 33.33 刀/天。

统一说明：Claude Max 与 GPT Pro 共用每日 credits 池。使用 GPT Pro 时按
0.6 credits = 1 刀折算；使用 Claude Max 时按 1.5 credits = 1 刀折算。
每日额度不结转。

## Group Configuration

Create one `credit` subscription group per product. The daily and monthly limits
are stored in the existing limit fields, but their unit is credits for these
groups.

| Product | subscription_type | daily_limit_usd | monthly_limit_usd | validity_days |
| --- | --- | ---: | ---: | ---: |
| Lite 月卡 | credit | 15 | 450 | 30 |
| Pro 月卡 | credit | 30 | 900 | 30 |
| Max 月卡 | credit | 40 | 1200 | 30 |
| Ultra 月卡 | credit | 50 | 1500 | 30 |

## Redeem Code Payload Template

Use `type=subscription` and bind the code to the target product group with
`group_id`. The `value` field is only the card face value for inventory and
reconciliation; the monthly entitlement comes from the group limits.

```json
{
  "count": 100,
  "type": "subscription",
  "value": 0,
  "group_id": 0,
  "validity_days": 30,
  "batch_name": "monthly-credit-lite-20260608",
  "purpose": "sale_recharge",
  "sales_status": "inventory",
  "sales_channel": "liandong_shop",
  "internal_notes": "Lite 月卡，每天15 credits；GPT Pro约25刀/天；Claude Max约10刀/天"
}
```
