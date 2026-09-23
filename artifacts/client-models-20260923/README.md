# Client model directory completion — 2026-09-23

Scope: GPT-6 Sol/Luna in Codex and Claude Opus 5.5 in Claude Code. No default,
pricing, route, group binding or monthly entitlement changes.

## Source and generation

Published schema-v2 release contracts now supply a fallback client projection
when long-form model docs do not exist. Admission requires public catalog
membership, public lifecycle, an explicit preferred supported protocol and
passing basic/stream/terminal/tool/continuation cells. Only supported native
reasoning wire values are projected. Existing long-form model facts retain
precedence; existing protocol-union behavior is preserved.

The single projection drives Codex catalog, installer reasoning maps, frontend
client profiles and backend setup allowlists. No duplicate full model matrices
were created. Optional Codex-only capabilities stay conservative.

Codex rejects the provider modality `video` in the client catalog. Reproduced
with native 0.155.1, then filtered only at the Codex projection boundary; provider
model documentation and API modality facts remain untouched.

Windows fixture testing reproduced the existing StrictMode property-count
failure in Write-ClaudeConfig. The sole production-script correction wraps
PSObject.Properties as an array. Repeat writes are semantically idempotent.

## Verification

- 38 Python catalog/release tests, 70 frontend installer/modal tests passed
- Backend setup option/selection/ticket tests cover all three exact IDs across
  macOS, Linux and Windows; one-time ticket replay rejected
- Backend build and frontend typecheck/production build passed
- PowerShell new-model writer fixtures passed locally; native Windows 5.1 and
  PowerShell 7 remain required CI gates
- Native current Codex 0.155.1 and Claude Code 2.1.278 completed real owned API
  local-file tool loops for their new models
- Actual installer-generated model/config fields completed real owned API loops
  on canonical Codex 0.153.4 and Claude Code 2.1.263; receipts in sibling
  client-models-20260923-generated-config
- Credentials were supplied only via process environment. The Codex credential
  resolver was explicitly overridden to env-only for the live loop; real
  auth.json writes were not performed. Config/auth file merge tests use only
  synthetic fixtures. Native Windows/Linux client API loops were not run
- Independent review: no remaining blockers; no unrelated defaults changed

Post-deploy gates: exact served catalog/script bytes, new model entries and
version readback, public health, then changelog assessment. A CLI tool loop is
not a claim of visual inspection of desktop dropdowns.
