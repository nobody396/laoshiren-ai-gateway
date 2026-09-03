# Model release acceptance

This directory is **release evidence data, not a public-model allowlist**.
`gemini-3.8-flash.json` and `releases/gemini-3.8-flash.draft.json` remain drafts.
The runtime catalog, account mappings, prices, aliases and defaults are unchanged.

## Why these files were missing from main

The prior implementation lived in an uncommitted feature worktree. Broad legacy
`.gitignore` rules also ignored new `scripts/` and `tests/` files. This change
ports only acceptance tooling, schema and a draft; it does not merge unrelated
frontend, billing, routing or historical-evidence changes from that worktree.
`import-provenance.json` records hashes of the original source snapshots.

The client matrix is a version-locked snapshot of Skill Hub's canonical
`laoshirenai-client-integration/references/client-matrix.json`. Change the
canonical source first, then refresh this snapshot and its provenance hash in a
reviewed change. This is not a second editable client capability registry.
Legacy artifact references remain unresolved until the exact redacted artifacts
are imported and checked. They are **not** silently trusted.

## Commands (from repository root)

```sh
python3 scripts/model_release.py validate model-doc-contracts/releases/gemini-3.8-flash.draft.json
python3 scripts/model_release.py plan model-doc-contracts/releases/gemini-3.8-flash.draft.json
python3 scripts/provider_contract_runner.py plan model-doc-contracts/releases/gemini-3.8-flash.draft.json
python3 scripts/model_doc_contract.py validate model-doc-contracts/gemini-3.8-flash.json
python3 scripts/model_doc_matrix.py audit --contracts model-doc-contracts \
  --client-matrix model-doc-contracts/client-matrix.json \
  --pricing-url https://api.laoshirenai.com/api/v1/public/model-pricing \
  --require-model gemini-3.8-flash
python3 scripts/model_price_matrix.py audit --catalog model-catalog/catalog.json \
  --pricing-url https://api.laoshirenai.com/api/v1/public/model-pricing
```

`plan` can report gaps and exit successfully; it is not an acceptance result.
`audit` fails nonzero on missing models, evidence, client versions/OS cells,
protocol checks or price drift. An empty inventory never passes. For a new model
use `--require-model` so absence from the public inventory cannot hide it.
Current full-inventory audit intentionally also reports legacy contracts not yet
migrated; a successful tool installation is not a successful model acceptance.

The model price matrix separates provider-public USD, gateway-base and customer
prices. It does not equate a supplier multiplier with an exchange rate and does
not infer official prices from sample usage costs. An optional redacted
`--db-export` checks database/public readback without connecting to a database.

## Offline versus live

Ordinary CI runs `test_provider_contract*.py`, `test_model*.py` and
`test_matrix_schema.py` with synthetic fixtures only. `run-fixture` receipts and
any evidence exported from them retain an `offline_fixture` origin and cannot
serve as release proof. `model_catalog.py apply --manifest` validates the v2
manifest and its readiness before any writes; live evidence requires a local
`artifact_path` and matching SHA256 under the manifest directory.

The public gateway harness requires `run --execute --acknowledge-paid-probes`.
It uses only the owner-controlled user2-codex identity, validates the starting
identity/group, limits authenticated requests to the approved HTTPS API origin,
and refuses redirects. It restores group 6 only if a switch was attempted after
successful ownership validation. Production cross-run reuse is disabled until
receipts bind the current gateway deployment; old receipts are kept for audit.
Do not run different tools concurrently against the same owned gateway test key.

`provider_contract_direct.py` separately probes the existing named upstream
profile without using or mutating a gateway/customer key. `plan` never reads a
secret. `run` uses the same two explicit paid-probe flags, a fixed provider/route
allowlist and Agent Switch's inherited non-TTY descriptor. It emits
`scope=direct_upstream_only`; these receipts cannot establish public group access,
client compatibility, accounting, or balance changes. Existing receipt
directories cannot be overwritten.

```sh
python3 scripts/provider_contract_direct.py plan \
  --manifest model-doc-contracts/releases/gemini-3.8-flash.draft.json \
  --provider pomoai-gemini-normal --route jp --output-dir /tmp/gemini38-plan
```

Paid execution is a separate operator action after review of the bounded plan.
Never put credential values in flags, fixture files, environment files, receipts
or logs. These commands do not send email or publish a changelog.

## Evidence semantics and remaining work

- P02 proves SSE framing; P03 separately requires real delta text and terminal.
- P04 checks the exact tool name, arguments and correlation ID. P05 checks P04
  first, preserves model content/signatures, and uses an undisclosed tool result
  marker rather than leaking the answer into a normal user prompt.
- P06 sends the real wire field and rejects mismatched effort or configuration
  echo. Missing thinking-output/token evidence remains blocked, not unsupported.
- P12 proves usage parsing/attribution only. Its costs are `not_reconciled`, never
  a billing pass. Full billing needs one usage row, one completed accounting
  command, correct tariff/cost and a matching owner balance delta.
- The old evidence importer was deliberately not ported: it promoted usage-cost
  observations into billing reconciliation. Resolve typed receipts directly;
  do not reuse that importer for release proof.
- `model_doc_contract.py validate` is structural. Public-card readiness additionally
  needs the complete exact model/version/OS matrix and resolvable dated evidence.
- Official limits, media, cache, resilience, actual client loops and accounting
  are separate cases. No full-context paid probe is run automatically.
- A matching hash proves artifact integrity, not independent authenticity.
  Receipt provenance and applicability must still be reviewed by the operator.

All nine canonical dimensions are defined in `matrix-schema.json`. A model can
use an existing protocol while a specific feature has a verified negative
boundary. Untested, missing, stale and unresolved remain gaps; they never become
`unsupported` merely to close the matrix.
