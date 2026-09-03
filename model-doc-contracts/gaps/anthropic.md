# Claude model-documentation evidence and gaps

Verified at: 2026-08-31 (Beijing time)

## Completed public-gateway evidence

The following model IDs completed an owned, real `https://api.laoshirenai.com` Claude Code `2.1.251` loop through Anthropic Messages. Each run used `--bare`, allowed only the built-in `Read` tool, read a local one-line fixture, returned the tool result on the next turn, answered `CLAUDE_CONTRACT_42`, reported `terminal_reason: completed`, `num_turns: 2`, and exited successfully:

- `claude-fable-5`
- `claude-opus-5`
- `claude-opus-4-8`
- `claude-opus-4-7`
- `claude-opus-4-6`
- `claude-sonnet-5`
- `claude-sonnet-4-6`
- `claude-haiku-4-5`

For all except the Opus 4.6 caveat below, separate owned gateway probes also observed minimal text `HTTP 200`, SSE `message_start` / `message_delta` / `message_stop`, a forced `tool_use`, successful `tool_result` continuation, usage fields, and image input. A malformed Messages payload returned structured `HTTP 400` with `invalid_request_error`.

Existing repository evidence additionally records real `claude-sonnet-5` Agent loops in Grok Build `1.0.13`, Kimi Code `0.38.0+`, OpenCode `1.18.15`, and ZCode `3.10.1`.

The eight owned Claude Code loops above were run on this macOS host. Their exact
`test_matrix.clients.Claude Code.messages.macos` cells now retain the client
version, OS, date, two-turn tool continuation, terminal marker, and exit result.
The other compatible clients remain `blocked` for those seven models; no model
inherits Sonnet 5 client evidence by family similarity.

## Access readback

`GET https://api.laoshirenai.com/api/v1/public/model-pricing` was read without mutation on 2026-08-31. The contracts copy its exact public group names and multipliers:

- `claude-fable-5`: Claude 标准线路 `2.4x`; Claude 兼容线路 `2.6x`.
- The other eight requested IDs appear in some combination of Claude 标准线路 `2.4x`, Claude 兼容线路 `2.6x`, Claude Plus / Pro / Max 月卡组 `2.4x`, and Claude 经济线路 `1x`.
- The emitted eight contracts use the exact groups returned for each model. Gateway E2E proves an owned Claude standard-key path, not every listed group independently.

## Official specification sources

- Current lineup and shared modality statement: https://platform.claude.com/docs/en/models/overview
- Fable 5: https://platform.claude.com/docs/en/models/fable-5/overview
- Opus 5: https://platform.claude.com/docs/en/models/opus-5/overview
- Opus 4.8: https://platform.claude.com/docs/en/models/opus-4-8/overview
- Opus 4.7: https://platform.claude.com/docs/en/models/opus-4-7/overview
- Opus 4.6: https://platform.claude.com/docs/en/models/opus-4-6/overview
- Opus 4.5: https://platform.claude.com/docs/en/models/opus-4-5/overview
- Sonnet 5: https://platform.claude.com/docs/en/models/sonnet-5/overview
- Sonnet 4.6: https://platform.claude.com/docs/en/models/sonnet-4-6/overview
- Haiku 4.5: https://platform.claude.com/docs/en/models/haiku-4-5/overview
- Effort levels: https://platform.claude.com/docs/en/build-with-claude/effort

These official pages define the synchronous limits and `text and images -> text`. Video is therefore recorded as unsupported. No PDF/audio preprocessing is presented as a native model modality.

## Draft contract: Claude Opus 4.5

A contract is now emitted for `claude-opus-4-5`, but it is intentionally a
non-publishable draft (`verification.gateway_e2e=false`, `clients=[]`). The
official model facts are complete: the public alias resolves to the pinned
`claude-opus-4-5-20251101` snapshot, synchronous Messages supports a 200,000
token context window and 64,000 output tokens, native input is text/image and
native output is text, and the supported effort levels are low, medium, high,
and max. Public-price inventory rows are also retained for Claude Plus / Pro /
Max monthly groups and Claude 经济线路.

The draft cannot claim a verified client because:

1. Public pricing still lists the alias in Claude Plus / Pro / Max monthly groups and Claude 经济线路.
2. The owned standard-key `/v1/models` response does not include either `claude-opus-4-5` or `claude-opus-4-5-20251101`.
3. Both IDs returned structured `HTTP 400 invalid_request_error` stating the model is not supported when called with that owned key.
4. No owned key for one of the groups that advertises Opus 4.5 was used, so there is no public-gateway client tool-result loop for this exact model.

Historical production evidence does record one real Hostinger streaming request
with exact `OK` on monthly account 14. That is enough to retain the observed
Messages minimal/streaming cells, but not enough for tool call, tool-result
continuation, usage, invalid-request shape, group-Key readback, or a client icon.

Required closure: obtain an explicitly owned test key bound to one advertised Opus 4.5 group, confirm `/v1/models`, run the Messages protocol matrix, and complete a real client tool-result loop. If that cannot pass, remove the stale public-pricing exposure instead of fabricating a contract.

## Opus 4.6 request-envelope caveat

Very small plain requests (`max_tokens` 16-256 without adaptive thinking) repeatedly returned `HTTP 400`. An explicit adaptive-thinking request with `output_config.effort=low` and `max_tokens=4096` returned `HTTP 200`, and Claude Code completed its real streamed tool loop. The model is therefore gateway/client verified, but user-facing examples should not use unrealistically small output budgets for this model.

## Deliberately unclaimed

- No paid full-context boundary tests were run; exact limits come from Anthropic official documentation.
- No server-side web-search claim was added.
- No client other than Claude Code is shown for seven models without a real per-model Agent loop.
- No listed public group other than the owned standard-key route is claimed independently healthy.
