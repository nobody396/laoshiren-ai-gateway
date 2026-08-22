# Universal API key group (PR2)

## Contract

- One user API key remains bound to one `platform=universal` access group.
- Each request resolves `(public model, inbound protocol)` to an existing
  concrete target group through `groups.universal_routes`.
- The target group becomes the request's route and billing group. Existing
  account bindings, channel pricing, exclusive access, wallet balance,
  subscription windows and monthly-card limits remain authoritative.
- Universal groups never copy or own upstream accounts.
- API-key quota/expiry/rate-limit enforcement still happens before route
  resolution.

## Supported protocol matrix

This PR only exposes combinations already handled by the production gateway:

| Inbound | Allowed target platforms |
| --- | --- |
| Anthropic Messages | Anthropic, Antigravity, OpenAI, Grok |
| OpenAI Responses | OpenAI, Grok |
| Chat Completions | OpenAI, Grok, Gemini |

Invalid combinations are rejected when saving group configuration. This avoids
claiming that Codex can use an Anthropic-only route before a verified
Responses-to-Anthropic bridge exists.

Responses WebSocket routing is deliberately rejected for universal keys in
PR2 because the model arrives in later WebSocket frames. HTTP Responses remains
supported. PR3 can add a connection-level candidate set while staying Shadow.

## Control plane

- Admin create/update accepts `universal_routes`.
- Admin route preview: `POST /api/v1/admin/groups/:id/universal-preview` with
  `{ "model": "...", "protocol": "responses" }`.
- The group UI provides a JSON editor so configuration is explicit and
  reviewable. Route target group IDs are never returned by ordinary user DTOs.
- `/v1/models` returns the enabled exact public-model union; prefix rules stay
  routable but are not advertised as literal model IDs.

## Dormant production boundary

The migration and code do not create a universal group, migrate an existing
key, enable intelligent routing, enable Fast, or deploy production. Without an
admin-created universal group and routes, existing behavior is unchanged.
