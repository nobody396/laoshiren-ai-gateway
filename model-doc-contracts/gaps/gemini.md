# Gemini model-document contract gaps

Verified on 2026-08-31 Beijing time. No production mutation, customer credential, paid full-context request, or new authenticated model request was made for this contract pass.

## Sources used

- Gemini 3.7 Flash official model page: <https://ai.google.dev/gemini-api/docs/models/gemini-3.7-flash>
  - Official ID `gemini-3.7-flash`.
  - Input limit `1048576`; output limit `65536`.
  - Inputs text, image and video (also audio/PDF, intentionally omitted from the public three-modality card); output text.
  - Thinking levels `low`, `medium`, `high`; `minimal` is rejected.
- Gemini 3.1 Pro Preview official model page: <https://ai.google.dev/gemini-api/docs/models/gemini-3.1-pro-preview>
  - Official upstream ID `gemini-3.1-pro-preview`.
  - Input limit `1048576`; output limit `65536`.
  - Inputs text, image and video (also audio/PDF, intentionally omitted); output text.
- Gemini thinking guide: <https://ai.google.dev/gemini-api/docs/generate-content/thinking>
  - Gemini 3.1 Pro accepts `low`, `medium`, `high`; `minimal` is unsupported.
- Live read-only public catalog: <https://api.laoshirenai.com/api/v1/public/model-pricing>
  - `Gemini 标准线路`, multiplier `0.6`, currently exposes public IDs `gemini-3.7-flash` and `gemini-3.1-pro`.
  - Read back again at `2026-08-31T11:36:37.816099323Z`: Gemini 3.7 Flash is `0.45 / 2.25` CNY per 1M input/output tokens and Gemini 3.1 Pro is `1.2 / 7.2`; both public rows expose cache read `0`, cache write `null`, and no `long_context` object. Contracts preserve this public snapshot only and do not mutate production pricing.
- Existing owned-client evidence:
  - `frontend/src/docs/content/integration-antigravity.md`: Antigravity `1.1.22` completed low/high `gemini-3.7-flash` Agent loops.
  - `frontend/src/docs/content/integration-kimi-code.md`: Kimi Code `0.39.1` completed a `gemini-3.7-flash` GenerateContent Agent loop.
  - `frontend/src/docs/content/integration-gemini-cli.md`: Gemini CLI `0.57.0` completed a `gemini-3.1-pro` Agent loop.

## Gaps that block an unqualified claim

1. **Public alias versus official ID:** the public `gemini-3.1-pro` alias is documented by Google as `gemini-3.1-pro-preview`. The real Gemini CLI loop proves the public alias currently works, but Antigravity `1.1.22` rewrites it to `gemini-3.1-pro-preview`, which the current Key does not expose. Therefore only Gemini CLI is public-client-verified for this model; do not show Antigravity on its card.
2. **Catalog limits are stale:** `model-catalog/catalog.json` currently says `256000/8192` for `gemini-3.1-pro` and `128000/8192` for `gemini-3.7-flash`. The contracts intentionally use Google's exact current limits `1048576/65536`. Shared catalog generation must be corrected before the card renderer relies on it.
3. **Positive modality evidence is official, not a new owned live probe:** Google's model pages explicitly support text/image/video, so the contracts preserve those capabilities and pass the current schema. This pass did not separately send owned image and video requests. The current validator has no `official` versus `live` modality-evidence field; public QA should keep this distinction visible until separate bounded probes are recorded.
4. **Gemini CLI on 3.7 is only a candidate:** the Key can list the model, but the existing evidence records the complete Gemini CLI Agent loop on `gemini-3.1-pro`, not `gemini-3.7-flash`. Do not show Gemini CLI on the 3.7 card until that exact pair completes a real loop.
5. **Kimi Code reasoning edge levels:** the successful 3.7 loop proves GenerateContent integration, but there is no recorded evidence that Kimi Code's `xhigh`/`max` values are mapped safely to Gemini 3.7's supported `low`/`medium`/`high`. Do not display a mapping until tested.
6. **No full-context boundary test:** exact limits come from Google. A paid 1M-token request was intentionally not run.
7. **Official feature support is not gateway proof:** both official model pages list caching, Function Calling, Search grounding, Structured Outputs, and thinking. These feature cells remain `blocked` where no exact current public-gateway receipt exists. The existing GenerateContent matrix does explicitly include representative error parsing, so `invalid_request` is verified rather than left as an automatic migration gap.

## Safe public card claims now

- Both models: Gemini GenerateContent, text/image/video input, text output, `1048576` context, `65536` maximum output, Gemini 标准线路 `0.6x`, Base URL `https://api.laoshirenai.com`.
- Gemini 3.7 Flash clients: Antigravity `1.1.22`; Kimi Code `0.39.1`.
- Gemini 3.1 Pro client: Gemini CLI `0.57.0` only.
