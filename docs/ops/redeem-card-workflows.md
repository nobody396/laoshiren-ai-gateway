# Redeem Card Workflows

This runbook defines the two supported balance-card workflows. It is the source of truth for card generation and billing reconciliation.

## Store Inventory Cards

Use this when cards will be listed in the external card shop and sold for real money.

Generation fields:

- `type`: `balance`
- `purpose`: `sale_recharge`
- `sales_status`: `inventory`
- `sales_channel`: `liandong_shop` when the batch is for Liandong card shop
- `batch_name`: include amount, channel, and date
- `external_url`: use the shop product or batch URL when available
- `internal_notes`: short reconciliation note

Runtime behavior:

- Newly generated cards stay as inventory and are not counted as revenue.
- When a user redeems one of these cards, the redeem transaction marks it `sold`, sets `sold_at`, records `used_by` and `used_at`, and adds balance to the user.
- The billing page counts only `purpose=sale_recharge` and `sales_status=sold`.

Important limitation:

- Until an external shop payment callback is integrated, the site can only mark revenue when the buyer redeems the card. A paid but unredeemed external order is not visible to this system yet.

## Gift Cards

Use this when giving balance to a user for free.

Generation fields:

- `type`: `balance`
- `purpose`: `gift`
- `sales_status`: `gifted`
- `sold_to_note`: optional recipient or context
- `internal_notes`: gift reason

Runtime behavior:

- Gift cards can still be redeemed normally and add balance to the user.
- Gift cards never become `sold` automatically.
- Gift cards are excluded from the billing revenue page.

## Compensation And Test Cards

Use `purpose=compensation` for customer make-good credits and `purpose=internal_test` for internal testing. Do not mark either as `sold`.

## API Examples

Store inventory batch:

```json
{
  "count": 100,
  "type": "balance",
  "value": 20,
  "batch_name": "liandong-shop-20-20260531",
  "purpose": "sale_recharge",
  "sales_status": "inventory",
  "sales_channel": "liandong_shop",
  "external_url": "https://example.com/product",
  "internal_notes": "card shop inventory"
}
```

Gift batch:

```json
{
  "count": 10,
  "type": "balance",
  "value": 20,
  "batch_name": "gift-20-20260531",
  "purpose": "gift",
  "sales_status": "gifted",
  "internal_notes": "free gift cards"
}
```

The admin generate endpoint accepts at most 100 cards per request. Split larger batches into repeated requests with separate idempotency keys.
