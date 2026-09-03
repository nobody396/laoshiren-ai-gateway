# Grok model-contract gaps

## Evidence used

- xAI official model pages:
  - https://docs.x.ai/developers/grok-4-6
  - https://docs.x.ai/developers/models/grok-4.5
  - https://docs.x.ai/developers/model-capabilities/text/reasoning
- Production public catalog readback on 2026-08-31:
  - `Grok 标准线路`, `Grok Plus 月卡组`, `Grok Pro 月卡组`, and `Grok Max 月卡组`
  - all four expose `grok-4.6` and `grok-4.5` at `0.4x`
- Existing owned protocol evidence:
  - production streaming requests completed with usage attribution for both models;
  - current public Responses routing later repeatedly returned `502`, so Responses is intentionally not published;
  - Grok 4.6 completed real tool-result Agent loops in Grok Build 1.0.13, OpenCode 1.18.15, and ZCode 3.10.1;
  - Grok 4.5 completed real public-client requests through the dual-model Grok Build integration, but its most recent durable proof is not as strong as the Grok 4.6 tool-loop proof.

- xAI's current model registry embedded in the official pages records both models as:
  - `maxPromptLength=500000`;
  - native text/image input and text output;
  - Function Calling, Structured Outputs, and reasoning support;
  - reasoning efforts `low`, `medium`, `high`, and `xhigh` (default `high`);
  - long-context pricing threshold `200000` on the upstream product. This is upstream pricing metadata, not proof that the current public 老实人AI price rows expose the same rule.
- Live public customer-price inventory readback at `2026-08-31T11:36:37.816099323Z` still lists both models in all four Grok groups at `0.4x`. It does not expose a `long_context` object for either Grok row, so the contracts preserve only the fields actually present in that public snapshot.

## Contract limitations

1. **Output limit mismatch.** xAI now documents “no text output limit” for Grok 4.6 and does not publish a finite per-request output integer for Grok 4.5. The current contract schema requires a positive integer, so both contracts use the live product/import cap `128000`, not a native model ceiling. The card should label this as the configured maximum output, or the schema should learn `null`/`unlimited` before calling it an official model limit.
2. **Positive modality evidence is official-spec-backed.** xAI explicitly lists text and image input with text output. No new paid production-path image probe was run in this batch. Video is excluded because xAI documents dedicated Imagine video models rather than video input on these text models.
3. **Responses is upstream-supported but product-unhealthy.** xAI documents Responses and Chat Completions. The current public Responses route repeatedly returned `502`, with no healthy fallback completing the request. Only Chat Completions may render as verified until a fresh owned production probe closes the Responses path.
4. **Grok 4.5 client proof is weaker.** The dual-model Grok Build/CC Switch integration produced real final responses for Grok 4.5, but the durable evidence does not show a fresh local-tool call plus tool-result continuation. Before presenting Grok 4.5 as equal to Grok 4.6 for coding-agent work, rerun one bounded owned Grok Build Agent loop and record the exit status.
5. **Reasoning source corrected.** The current xAI model registry lists `low`, `medium`, `high`, and `xhigh` for both Grok 4.5 and Grok 4.6. The earlier Grok 4.5 `xhigh -> high` normalization was stale and has been removed. Kimi Code's extra `max` control remains blocked until an exact model-specific mapping is tested.
6. No paid full-context or maximum-output boundary probe was run.
