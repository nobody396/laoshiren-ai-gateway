# Kimi and MiniMax contract evidence and remaining gaps

Checked in Beijing time on 2026-08-31. No production configuration, routing, group, price, alias, Key, or deployment was changed.

## Exact model-directory fields

| Public model | Context | Maximum output | Native input | Native output | Primary specification |
| --- | ---: | ---: | --- | --- | --- |
| `kimi-k2.7-code` | 262144 | 16384 | text, image, video | text | <https://help.aliyun.com/zh/model-studio/kimi-k2-7-code> |
| `kimi-k3` | 1048576 | 1048576 | text, image, video | text | <https://help.aliyun.com/zh/model-studio/kimi-k3>, exact K3 examples at <https://help.aliyun.com/zh/model-studio/kimi-api-by-moonshot-ai> |
| `minimax-m3` | 1048576 | 524288 | text, image, video | text | context: <https://help.aliyun.com/zh/model-studio/minimax-m3>; output: <https://platform.minimax.io/docs/api-reference/text-chat-openai> |

Important scope notes:

- K2.7 Code's `16384` is the current Alibaba deployment limit. Moonshot's `32768` default must not be presented as this route's hard maximum.
- MiniMax's official Chat and Messages references explicitly give `131072` as recommended and `524288` as the hard maximum. The Alibaba deployment summary still shows an em dash for maximum output, so the contract presents the model-native official limit while keeping the current group route blocked until a bounded deployment check is preserved.
- No paid full-context or 524288-token boundary request was run.

## Protocols are not feature parity

| Model | Chat Completions | Responses | Messages | GenerateContent |
| --- | --- | --- | --- | --- |
| Kimi K2.7 Code | verified base matrix on owned direct Alibaba | unsupported by current native Alibaba route | blocked; Moonshot compatibility guide exists but public-group matrix is absent | unsupported |
| Kimi K3 | verified current route and production-public Agent loops | Moonshot direct supports it, but current native Alibaba route returns HTTP 400; recorded unsupported for this group | blocked pending current-group matrix | unsupported |
| MiniMax M3 | verified base matrix on owned direct Alibaba | blocked; MiniMax direct supports it and gateway bridge basics passed, but current Alibaba native route rejects it | blocked; official direct API exists but public-group full matrix is absent | unsupported |

The gateway can bridge some Chat-backed requests into Responses or Messages shapes. That compatibility must not be described as native protocol or full feature parity. Bridge-only evidence is kept in blocked cells until streaming terminal, tool result continuation, Usage/error, reasoning mapping, and exact client loops all pass through an owned public-group Key.

Repository evidence:

- `/Users/fujunhao/laoshirenai/log.md:9502,10062,10084`
- `/Users/fujunhao/laoshirenai/local/reports/aliyun-beijing-preflight-20260828.md`
- `backend/internal/service/openai_gateway_responses_chat_bridge_live_test.go`

## Tools and server features

| Model / route | Function Calling | Image/video | Prompt cache | `web_search` |
| --- | --- | --- | --- | --- |
| K2.7 Code Chat | base tool-result continuation verified direct upstream | native capability official; exact public terminal receipts missing | officially supported; public cache-hit receipt missing | official Alibaba documents conflict; public exact receipt missing, so blocked |
| K3 Chat | base continuation and three public coding-client loops passed | native model capability official; exact public image/video terminal receipts missing | officially supported; public cache-hit receipt missing | Moonshot direct tool exists, Alibaba documents conflict; public exact receipt missing, so blocked |
| MiniMax M3 Chat | base tool-result continuation verified direct upstream | requests returned HTTP 200 and Usage, but a low token cap ended inside reasoning before visible answers | implicit cache official; public cache-hit receipt missing | current Alibaba route explicitly unsupported |

Function tools and server Web Search are separate claims. A verified Function Calling loop never upgrades `web_search`.

## Reasoning and client mapping

- K2.7 Code is always-thinking. It does not expose adjustable `reasoning_effort`; multi-turn tool use must preserve `reasoning_content`. The directory uses native state `always_on`, not fabricated low/high levels.
- K3's model-native direct API documents `low`, `high`, and `max`; the current Alibaba route confirms only `max`. Public `low/high` cells remain blocked.
- MiniMax M3 exposes `thinking.type=disabled` and `thinking.type=adaptive`, not genuine low/medium/high depth levels.
- Kimi Code exposes fixed `low/medium/high/xhigh/max`. No exact tested mapping exists for K2.7's always-on state, K3's current max-only Alibaba route, or MiniMax's disabled/adaptive control. Those client-reasoning cells stay blocked.

## Public groups and current customer prices

The public inventory snapshot at `2026-08-31T11:36:37.816099323Z` returned:

| Group | ID | Multiplier | Model | Input | Output | Cache read |
| --- | ---: | ---: | --- | ---: | ---: | ---: |
| Kimi 企业高速线路 | 62 | 0.95x | kimi-k2.7-code | 6.175 | 25.65 | 1.235 |
| Kimi 企业高速线路 | 62 | 0.95x | kimi-k3 | 19 | 95 | 1.9 |
| MiniMax 企业高速线路 | 63 | 0.95x | minimax-m3 | 3.99 | 15.96 | 0.798 |

All prices are CNY per 1M tokens. Kimi K3 has production-public Chat Agent evidence. K2.7 and MiniMax have public inventory visibility but no matching owned group/Key route Receipt, so access remains blocked rather than inferred.

## Remaining release blockers

1. Repair or replace the owned public E2E Key that currently returns HTTP 403 for `/v1/models`.
2. Run one bounded public Chat Agent loop and independent billing readback for K2.7 and MiniMax.
3. Preserve terminal public image/video receipts with enough output budget for K2.7, K3, and MiniMax.
4. Resolve Kimi exact-route Web Search conflicts with a bounded public probe; MiniMax Alibaba Web Search remains an authoritative negative.
5. Verify client reasoning mappings before showing a configurable effort selector.
6. Complete exact client version and operating-system cells; do not promote direct-provider or version-mismatched evidence as public proof.
