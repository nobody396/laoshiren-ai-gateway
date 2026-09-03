# Qwen model-doc evidence and remaining gaps

Verified at 2026-08-31 (Beijing time). No routing, account, group, price, key, or production application state was changed by this contract pass.

## Contract set

- `qwen3.8-max.json`
- `qwen3.7-max.json`
- `qwen3.7-plus.json`
- `qwen3.7-flash.json`
- `qwen3.6-plus.json`
- `qwen3.6-flash.json`

All six pass `laoshirenai-model-doc-contract` validation.

## Official limits and modalities

Dedicated Alibaba Cloud Model Studio pages are the authoritative limit source:

| Model | Context | Max output | Input | Output |
| --- | ---: | ---: | --- | --- |
| `qwen3.8-max` | 1,000,000 | 131,072 | text, image, video | text |
| `qwen3.7-max` | 1,000,000 | 131,072 | text | text |
| `qwen3.7-plus` | 1,000,000 | 131,072 | text, image, video | text |
| `qwen3.7-flash` | 1,000,000 | 131,072 | text, image, video | text |
| `qwen3.6-plus` | 1,000,000 | 65,536 | text, image, video | text |
| `qwen3.6-flash` | 1,000,000 | 65,536 | text, image, video | text |

Important boundaries:

- `qwen3.7-max` currently resolves to the text-only `2026-05-20` snapshot. Do not inherit multimodal support from the separate `2026-06-08` snapshot.
- A generic Alibaba vision overview shows a conflicting 64K value for Qwen 3.7 Plus/Flash. Their dedicated model pages state 131,072, so the contracts use 131,072.
- Context is decimal `1000000`, not `1048576`.
- No paid full-context boundary probe was run. The exact context/output limits remain official-source values.

## Live modality probes

Bounded, read-only calls were sent directly to Alibaba Chat Completions with the managed Beijing domestic key. Only status/model/finish/usage summaries were retained; no credential or response body was stored in the worktree.

- Text: all six already passed direct text requests and real Codex Agent loops.
- Image: Qwen 3.8 Max, 3.7 Plus, 3.7 Flash, 3.6 Plus, and 3.6 Flash returned HTTP 200. Qwen 3.7 Max returned HTTP 400 `Unexpected item type in content`, matching its official text-only specification.
- Video: the five officially multimodal aliases returned HTTP 200 with a valid five-second MP4. Qwen 3.7 Max rejected video with the same HTTP 400 content-type error.
- A sub-second video was rejected as too short and is not treated as a capability failure; the valid-duration retry passed.

The public card's video flag is a model capability. Alibaba Responses currently cannot transport video input, so video must use Chat Completions or DashScope even when Responses is also a verified model protocol.

## Protocol evidence

### Responses

- Historical direct production-network preflight passed all six (`local/reports/aliyun-beijing-preflight-20260828.md:12-25`).
- Qwen 3.6 Flash/Plus and Qwen 3.7 Flash/Max/Plus later passed native core Responses plus native `web_search`, with `response.completed` and `web_search_call` (`log.md:10019`).
- Qwen 3.8 Max passed non-stream, SSE terminal completion, Function Calling, native web search, production-public routing, and Usage (`log.md:10061`).
- All six completed a real Codex 0.151.0 production-public streaming Shell loop, tool-result continuation, final answer, exact random-file replay, and exit 0 (`log.md:10063`; `frontend/src/docs/content/integration-codex.md:3-18`).

### Chat Completions

- The 36-model public matrix passed Qwen text, streaming, Function Calling, and Usage (`log.md:9998`).
- This pass repeated direct official Function Calling for all six. Every model produced a tool call, accepted the tool result, emitted final text, and returned Usage with HTTP 200. `qwen3.7-flash` needed `tool_choice=none` on the continuation to prevent a second unnecessary tool call; the corrected continuation completed normally.

### Protocols not published

- Do not publish Anthropic Messages. An older direct preflight returned text (`aliyun-beijing-preflight-20260828.md:50-54`), while the later public-route audit rejected Messages. There is no current complete public Agent contract for it.
- Do not publish Gemini GenerateContent. The public-route matrix rejected that group/protocol path.

## Access evidence

Production public pricing and the local preview proxy both return one current group for all six models:

- group: `Qwen 企业高速线路`
- group id: `64`
- multiplier: `0.95x`
- Base URL: `https://api.laoshirenai.com/v1`

The existing single-model catalog still says `千问 Qwen`; that is stale and was deliberately not copied into these contracts.

## Client and reasoning matrix completion

- OpenCode `1.18.15`: all six Qwen models completed owned public-gateway Read/tool-result loops.
- ZCode App `3.10.1` / CLI `0.16.5`: all six completed owned public-gateway Responses Read/tool-result loops. The packaged CLI advertises `--settings`, `--max-turns`, and `--allowed-tools` but rejects those flags; the working verification path used the standard secret-free `~/.zcode/cli/config.json` plus `ZCODE_API_KEY`, then removed the temporary config.
- Grok Build `1.0.13`: Qwen 3.6 Flash/Plus, 3.7 Flash/Plus, and 3.8 Max completed clean loops. Qwen 3.7 Max repeatedly returned the marker but its resident actor exited `DeadFailed`, so that exact pair is recorded as unsupported.
- Kimi Code `0.38.0`: Qwen 3.7 Max/Plus and 3.8 Max completed loops. Qwen 3.6 Flash/Plus and 3.7 Flash repeatedly returned 502/`response.failed`, so those exact pairs are recorded as unsupported.
- Public Responses reasoning matrix: all seven levels passed for Qwen 3.7 Max/Plus and 3.8 Max. Qwen 3.6 Flash/Plus and 3.7 Flash accept `none/minimal/low/medium` but reject `high/xhigh/max` with HTTP 400. A model-specific gateway normalization now caps those three aliases at `medium`; it requires deployment and post-deploy Kimi Code retest before changing the current unsupported client evidence.

The catalog group name and Qwen 3.7 Flash output limit have been reconciled in the shared model catalog. The renderer consumes only matrix-complete contracts.
