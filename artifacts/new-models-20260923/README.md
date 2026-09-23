# New-model preflight — 2026-09-23 (Beijing)

Status: implementation and local preflight complete; PR/CI and production acceptance pending.
No production mappings or deployment have been changed yet.

## Upstream admission

Exact IDs: `gpt-6-sol`, `gpt-6-luna`, `claude-opus-5-5`.
`upstream-admission.json` contains Hostinger-origin real stream probes; an HTTP
200 is counted only with the terminal event. Icodeeasy, XiuRouter and Xingwan
passed both GPT IDs; Pomo enterprise passed Sol but rejected Luna. XiuRouter
passed Opus 5.5. Other failed routes are retained, not mapped or reactivated.

## Provider contract evidence

The 36 initial cases use the existing live harness with owned upstream keys,
resolved only in memory through Agent Switch. Receipts retain response hashes,
non-sensitive response shape, usage, and verifier outcomes, not raw bodies or keys.
These are direct-upstream results, NOT gateway or billing acceptance.

Opus 5.5 rejected forced tool choice (HTTP 400). The bounded harness adjustment
uses `auto` only for this exact model and retains the tool/result assertions.
Both P-04 and P-05 then passed; immutable follow-up receipts are under
`../new-models-20260923-auto-tools/claude-opus-5-5/`. Thinking blocks are preserved.

GPT cached-token counts remained zero; Luna's medium reasoning probe did not
return reasoning evidence. These are inconclusive, not proof of unsupported
features. The messages P-10 harness does not send a schema, so its failure is
also inconclusive. No synthetic pass was written for any of these.

## Pending release gates

All three schema-v2 drafts validate but `public_release_ready` remains false.
Context boundary, resilience, owned gateway accounting/balance reconciliation,
additional reasoning/caching evidence and Messages structured-output testing
remain incomplete. Supplier tariffs still need readback; official API pricing
is recorded only as the proposed base tariff, not supplier cost.

No old defaults, aliases, group bindings, account mappings, or public model
catalog entries have been changed. No production support has been claimed.

## Local validation / ablation

- 56 provider-contract unit tests passed, including a red/green regression for
  the exact Opus 5.5 tool-choice restriction and thinking preservation.
- All three new draft manifests validate and correctly report missing gates.
- Existing generated catalog check passes (39 models, unchanged).
- `git diff --check` passes.
- Scope retained: one exact-model probe adaptation, its test, draft evidence,
  and temporary checkout registration. No generic adapter rewrite or default
  migration. Backend/frontend builds and public E2E have not run.

## Continued implementation

- Release manifests now pass the native apply gate and generated catalog contains 42 models.
- Additional owned upstream probes validate reasoning and Opus structured output.
- GPT cache-hit acceptance failed twice; no cache-hit guarantee is published.
- Context limits use official documentation, not a maximum-size live load test.
- Existing adapter resilience/billing unit regressions passed (132.922s).
- New exact-model billing and conflicting auxiliary cache-rate regression passed.
- Code review found and fixed dynamic cache-write rate precedence and Opus 1h breakdown preservation.
- No old catalog row, alias or default changed. Account mappings will be applied only after immutable release.
