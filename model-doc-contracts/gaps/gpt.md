# GPT model documentation contract gaps

Verified on 2026-08-31 Beijing time. This file records why a public model is not yet represented by a validated card contract. A model stays out of the shared detailed-card renderer until every public protocol, modality, and client claim has evidence.

## Evidence already fixed

- Live public group and multiplier readback: `GET https://api.laoshirenai.com/api/v1/public/model-pricing`, updated at `2026-08-31T02:44:25.122054364Z`.
- Official model specifications:
  - `https://developers.openai.com/api/docs/models/gpt-5.6-sol`
  - `https://developers.openai.com/api/docs/models/gpt-5.6-terra`
  - `https://developers.openai.com/api/docs/models/gpt-5.6-luna`
  - `https://developers.openai.com/api/docs/models/gpt-5.5`
  - `https://developers.openai.com/api/docs/models/gpt-5.4`
  - `https://developers.openai.com/api/docs/models/gpt-5.4-mini`
  - `https://developers.openai.com/api/docs/models/gpt-daybreak-blue-latest`
  - `https://openai.com/index/introducing-gpt-5-3-codex-spark/`
- Existing client-loop evidence for `gpt-5.6-sol` and `gpt-5.6-terra` is recorded in `frontend/src/docs/content/integration-codex.md`, `integration-grok-build.md`, `integration-kimi-code.md`, and `integration-opencode.md`.

## Model-by-model status

| Model | Official limits and modalities | Live groups | Remaining release blockers |
| --- | --- | --- | --- |
| `gpt-5.6-sol` | 1,050,000 context; 128,000 max output; text/image in, text out; no video | Standard 0.5x; Plus/Pro/Max 0.5x; Daybreak 2x; Economy 0.35x; Enterprise 1x | Responses and four coding-client tool continuations are retained. Function calling, Web Search, and image generation have exact gateway evidence. Chat Completions and the remaining official built-in tools stay blocked until exact gateway tests; OS-specific client cells and group-key readbacks remain incomplete. |
| `gpt-5.6-terra` | 1,050,000 context; 128,000 max output; text/image in, text out; no video | Standard 0.5x; Plus/Pro/Max 0.5x; Economy 0.35x; Enterprise 1x | Responses and four coding-client tool continuations are retained. Function calling is verified; official built-in tools, ZCode, exact OS cells, and group-key readbacks remain blocked. |
| `gpt-5.6-luna` | 1,050,000 context; 128,000 max output; text/image in, text out; no video | Enterprise 1x | Responses and four coding-client tool continuations are retained. Function calling is verified; official built-in tools, ZCode, exact OS cells, and group-key readback remain blocked. |
| `gpt-5.5` | 1,050,000 context; 128,000 max output; text/image in, text out; no video | Standard 0.5x; Plus/Pro/Max 0.5x; Economy 0.35x; Enterprise 1x | Responses and four coding-client tool continuations are retained. Function calling is verified; official built-in tools, ZCode, exact OS cells, and group-key readbacks remain blocked. |
| `gpt-5.4` | 1,050,000 context; 128,000 max output; text/image in, text out; no video | Standard 0.5x; Plus/Pro/Max 0.5x; Economy 0.35x; Enterprise 1x | Responses and four coding-client tool continuations are retained. Function calling is verified; official built-in tools, ZCode, exact OS cells, and group-key readbacks remain blocked. The older 272,000 Codex metadata value is a client-side effective window, not the model limit. |
| `gpt-5.4-mini` | 400,000 context; 128,000 max output; text/image in, text out; no video | Enterprise 1x | Responses and four coding-client tool continuations are retained. Function calling is verified; official built-in tools, ZCode, exact OS cells, and group-key readback remain blocked. |
| `gpt-5.3-codex-spark` | Official release says 128,000 context and text-only; maximum output is not published | Standard 0.5x; Plus/Pro/Max 0.5x; Enterprise 1x | Contract is a validated draft with `max_output_tokens=null`. Basic Responses transport evidence exists, but every client loop, tool continuation, usage/error receipt, reasoning mapping, and group-key route readback remains blocked. Do not reuse another GPT-5.3 limit. |
| `gpt-daybreak-blue-latest` | 1,050,000 context; 128,000 max output; text/image in, text out; no video | Daybreak Blue security research 2x | OpenAI currently documents both Responses and Chat Completions. Only a basic production Responses stream and billing receipt exist locally; tool continuation, invalid-request shape, image request, reasoning levels, every real client loop, and Chat Completions remain blocked. |

## Minimal next probe set

For each blocked model, use an owner-controlled key in its exact public group and the public Base URL. Run only: minimal text, terminal streaming, tool call, tool-result continuation, usage parsing, one invalid request, one small image, and one short client Shell-read loop. Do not run full-context or large-output probes.

The old `LAOSHIRENAI_CC_SWITCH_E2E_KEY` and owner key 44 remain blocked with `403 SUBSCRIPTION_NOT_FOUND`. The Terra run used owner key 9 in its existing GPT standard group. No customer credential was used.
