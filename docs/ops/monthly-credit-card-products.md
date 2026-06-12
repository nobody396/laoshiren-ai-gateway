# Monthly Credit Card Products

This runbook is the source of truth for the credits-based monthly card lineup.
Cards are distributed through subscription redeem codes. The redeem code grants
or extends a subscription group; the group controls daily credits and billing.

## Billing Rules

- GPT Pro usage: 0.4 credits = 1 USD nominal usage.
- Claude Max usage: 1.25 credits = 1 USD nominal usage.
- GPT Pro and Claude Max share the same daily credits pool.
- Daily credits do not roll over.
- Validity is 30 days unless the batch explicitly says otherwise.

## Product Lineup

| Product | Daily Credits | GPT Pro Usage | Claude Max Usage | Description |
| --- | ---: | ---: | ---: | --- |
| Lite 月卡 | 15 credits/day | about 37.5 USD/day | about 12 USD/day | 每天 15 个 credits，适合尝鲜体验。 |
| Pro 月卡 | 30 credits/day | about 75 USD/day | about 24 USD/day | 每天 30 个 credits。 |
| Max 月卡 | 40 credits/day | about 100 USD/day | about 32 USD/day | 每天 40 个 credits。 |
| Ultra 月卡 | 50 credits/day | about 125 USD/day | about 40 USD/day | 每天 50 个 credits。 |
| Apex 月卡 | 2898 credits/month | about 7245 USD/month | about 2318.4 USD/month | 只设月额度，不设周限制，适合超长任务和高频开发。 |

## Customer-Facing Copy

Lite 月卡：每天 15 credits。使用 GPT Pro 约可用 37.5 刀/天；使用 Claude Max
约可用 12 刀/天。

Pro 月卡：每天 30 credits。使用 GPT Pro 约可用 75 刀/天；使用 Claude Max
约可用 24 刀/天。

Max 月卡：每天 40 credits。使用 GPT Pro 约可用 100 刀/天；使用 Claude Max
约可用 32 刀/天。

Ultra 月卡：每天 50 credits。使用 GPT Pro 约可用 125 刀/天；使用 Claude Max
约可用 40 刀/天。

Apex 月卡：每月 2898 credits。使用 GPT Pro 约可用 7245 刀/月；使用
Claude Max 约可用 2318.4 刀/月。Apex 只设置月额度，不设置周限制。

统一说明：Claude Max 与 GPT Pro 共用每日 credits 池。使用 GPT Pro 时按
0.4 credits = 1 刀折算；使用 Claude Max 时按 1.25 credits = 1 刀折算。
每日额度不结转。Apex 使用同一 credits 规则，但只设置月额度。

## Group Rate Multipliers

| Group family | rate_multiplier |
| --- | ---: |
| GPT Lite/Pro/Max/Ultra 月卡组 | 0.4 |
| Claude Lite/Pro/Max/Ultra 月卡组 | 1.25 |

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
| Apex 月卡 | credit | empty | 2898 | 30 |

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
