# DeepSeek contract evidence and remaining gaps

Checked in Beijing time on 2026-08-31. No route, account, key, group, price, or production application mutation was made by this contract task.

## Evidence used

- Official limits and modalities:
  - `deepseek-v4-pro-0813`: <https://help.aliyun.com/zh/model-studio/deepseek-v4-pro>
  - `deepseek-v4-flash-0731`: <https://help.aliyun.com/zh/model-studio/deepseek-v4-flash>
  - Both publish text input/output, a `1000000` context window, and `393216` maximum output.
- Official API and reasoning surface: <https://help.aliyun.com/zh/model-studio/deepseek-api>
  - Both exact snapshots are documented for Chat Completions and Responses.
  - `reasoning_effort` accepts `low`, `high`, and `max` for these snapshots; default is `high`.
  - Responses can use `web_search`, `web_extractor`, and `code_interpreter` in the supported Beijing/Singapore regions.
- Live public access readback on 2026-08-31:
  - `DeepSeek 企业高速线路`, multiplier `0.95`.
  - Both exact model IDs are present in that public group.
- Existing owned platform probes on 2026-08-30:
  - Chat ordinary/stream/function and continuation passed for both snapshots.
  - After per-model Responses routing was corrected, direct-source and production Responses core, SSE terminal completion, Function Calling, and native Web Search returned HTTP 200 for both snapshots.
  - Usage parsing was included in the 36-model/four-protocol owned audit; no customer key was used.
- Existing real-client loops:
  - Codex CLI `0.151.0`: both snapshots completed a randomized file-read Shell loop and exited 0. Codex displayed fallback-model-metadata warnings, but task execution and final answers succeeded.
  - OpenCode `1.18.15`: both snapshots completed an owned public-gateway Responses Read/tool-result loop, returned the exact random marker, and exited 0 on 2026-08-31.
  - Kimi Code `0.38.0`: both snapshots completed an owned public-gateway Responses file-read/tool-result loop and exited 0.
  - Grok Build `1.0.13`: both snapshots completed an owned public-gateway Responses file-read/tool-result loop and exited 0; auxiliary title/model refresh warnings did not affect the main task.
  - ZCode App `3.10.1` / CLI `0.16.5`: `deepseek-v4-flash-0731` completed the Responses tool-result continuation loop.

## Deliberately not published as verified

1. **Anthropic Messages on 老实人AI.** Alibaba documents Messages for both model IDs, but the current public DeepSeek group did not pass the platform Messages protocol audit. The contracts therefore mark it unsupported for the current public product instead of inferring support from the upstream documentation.
2. **ZCode with Pro.** Only Flash has durable ZCode evidence. Pro is omitted from ZCode claims until the exact model completes the same loop.
3. **Flash structured output.** The Flash model-specific page says unsupported while the generic DeepSeek API feature table says supported. Do not publish structured-output support until a model-specific live probe resolves the conflict.
4. **Messages and Responses feature parity.** Upstream compatibility does not mean the same server-side tools or event shapes are available on every protocol. The public card should show verified protocol names, not promise feature parity.

## Expensive boundary tests not run

- No paid 1,000,000-token boundary request.
- No 393,216-token output exhaustion test.
- No image/video paid probe, because the official model cards explicitly identify both models as text-only.

These omissions do not change the exact official limits displayed on the card. A future paid boundary probe requires explicit owner approval.
