# Newcomer guide screenshot plan

Internal capture checklist for `frontend/src/docs/content/newcomer-overview.md`.

| ID | Route | Focus | Mandatory redaction |
|---|---|---|---|
| newcomer-01-dashboard | `/dashboard` | Main navigation and entry points | Email, balance |
| newcomer-02-pricing | `/pricing` | PAYG groups and Plus/Pro/Max plans | Account data if visible |
| newcomer-03-redeem | `/redeem` | Redeem input and entitlement state | Code, balance, email |
| newcomer-04-create-key | `/keys` | Name, group selector, create action | Existing keys |
| newcomer-05-key-created | `/keys` | Copy/use actions | Full key and unrelated keys |
| newcomer-06-one-click | `/keys` | One-click/CC Switch actions | Full key |
| newcomer-07-usage | `/usage` | Usage row and detail | Email, key, content, private IDs |
| newcomer-08-image-group | `/keys` | GPT-Image group and Base URL | Existing keys |

## Capture contract

- Use only the admin-owned test identity and disposable test keys.
- Capture the real production UI; do not composite impossible states.
- Never expose a full API key, redeem code, email, balance, order number, request body, customer content or private request ID.
- Store optimized WebP/PNG assets only after redaction review.
- Replace every visible `📷 截图待补` block before deployment; keep HTML comments as maintenance anchors.
