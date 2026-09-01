# Customer error audit and visible Request ID contract

Audit time: 2026-09-01 09:19 Beijing time.

## Production evidence

`ops_error_logs` retained 10,397 rows from 2026-08-02 02:33 through
2026-09-01 08:34 Beijing time. After excluding internal users 1 and 2,
monthly probes, recovered failover attempts, and HTTP 200 observations, 3,040
final external-customer errors remained.

| Customer-visible category | Count | Share |
| --- | ---: | ---: |
| Request, model, endpoint, or protocol error | 1,247 | 41.02% |
| Upstream rate limit or unavailable capacity | 610 | 20.07% |
| Platform forwarding, network, or scheduling failure | 571 | 18.78% |
| Insufficient pay-as-you-go balance | 509 | 16.74% |
| Model or resource not found | 69 | 2.27% |
| Upstream failure or timeout | 23 | 0.76% |
| Authentication, group, or permission error | 8 | 0.26% |
| Monthly-plan quota or subscription limit | 3 | 0.10% |

Every one of the 3,040 stored final-customer response bodies contained the
platform Request ID, and every corresponding row had a non-empty
`ops_error_logs.request_id`. The missing-ID screenshots therefore did not come
from missing server correlation. The ID was only a separate JSON field and
many SDKs and CLIs rendered `error.message` while discarding unknown fields.

Anthropic-shaped errors also placed the ID only inside `error.request_id`, while
clients commonly expect or preserve a top-level `request_id` more reliably.

## Customer error contract

All gateway-created OpenAI, Anthropic, Gemini-compatible, and streaming error
envelopes must keep one platform Request ID visible in both forms:

1. a structured `request_id` field at the protocol-appropriate location; and
2. a final `[Request ID: ...]` suffix in the human-readable message.

Anthropic envelopes additionally expose the same value at top level. A suffix
that already contains the exact platform Request ID is not duplicated.

The platform Request ID identifies the final Customer Request across retries
and failover. Client-supplied correlation IDs and provider-returned upstream
request IDs remain separate evidence and must not replace it.

## Stable error codes and actions

| Code | Meaning | Customer action |
| --- | --- | --- |
| `invalid_request` | Request does not match the selected model or endpoint | Correct the model, endpoint, or parameters |
| `model_not_supported` | Model is not available to the current key/group | Select a model returned by `/v1/models` |
| `request_body_too_large` | Payload exceeds the gateway limit | Start a new task or remove large attachments |
| `context_length_exceeded` | Conversation exceeds the model context | Start a new task or shorten context |
| `authentication_failed` | API authentication failed | Check the API key and Base URL |
| `permission_denied` | Group or endpoint is not permitted | Use the endpoint and group shown in the model directory |
| `insufficient_balance` | Pay-as-you-go balance is insufficient | Recharge or use an eligible plan key |
| `subscription_limit` | Monthly-plan quota or entitlement blocks the request | Renew the plan or switch to pay-as-you-go |
| `rate_limit_exceeded` | A request or concurrency limit was reached | Wait, lower concurrency, then retry |
| `service_overloaded` | No route currently has available capacity | Retry after about 60 seconds or switch models |
| `request_timeout` | The request did not finish before timeout | Retry later or switch models |
| `resource_not_found` | Model or resource is not available to the key/group | Check `/v1/models` and the selected group |
| `service_unavailable` | No healthy service route is available | Retry later or switch models |
| `upstream_failure` | An upstream attempt failed after recovery was exhausted | Retry later and provide the Request ID if persistent |

Customer messages remain supplier-neutral. Supplier, account, route, raw
upstream body, and upstream credentials stay in operator evidence only.
