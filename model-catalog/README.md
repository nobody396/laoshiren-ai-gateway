# Model catalog

`catalog.json` is the single source for routine new-model metadata. It owns:

- pinned billing and public display pricing;
- public model whitelist and mapping preset;
- client default model, context window, and output limit;
- macOS/Linux and Windows installer model constants and cache-busting version;
- Codex client catalog entries supplied by OpenAI release manifests;
- preferred public group name and legacy fallback names.

Start from the `laoshirenai-model-onboarding` release manifest, then run:

```bash
python3 scripts/model_catalog.py apply --manifest /tmp/model-release.json
python3 scripts/model_catalog.py check
```

`apply --manifest` merges the release into `catalog.json`, automatically replaces
the previous platform default, bumps the installer patch version only when the
client contract changes, and refreshes the generated Go, TypeScript, Codex client
catalog, and bounded installer blocks. The same flow works for Grok, OpenAI, and
Anthropic models. OpenAI client defaults must carry the complete provider-owned
`model.codex_catalog_entry`; the generator never guesses capabilities.

Commit the source catalog and generated files together. CI rejects stale output
or mismatched public installer URLs. Protocol adapters, production account
mappings, group IDs, scheduler priorities, credentials, and aliases remain
explicit gated changes executed from the same release manifest; they are not safe
static catalog metadata.

## Price scopes and nulls

`pricing` remains the gateway's default billing/display basis because generated
runtime code consumes it. When the official price differs, `provider_pricing`
records the independent `provider_public` book without changing production
billing. The price matrix keeps three
layers:

- `provider_public`: official/provider public price from this catalog;
- `gateway_base`: the gateway basis read back as `group_customer / multiplier`;
- `group_customer`: the exact customer-visible price returned by the public
  pricing inventory.

A numeric zero is a verified free component. JSON `null` is not zero: it is
`unknown` unless `pricing.component_status` explicitly says `not_applicable`.
Provider long-context rules may coexist with an unknown gateway/customer tier;
that is an honest gap, not a provider-price conflict.

Use `public_price_visibility: "hidden"` for intentionally catalog-only models.
Use top-level `price_gaps` when a public runtime model cannot yet receive a full
catalog row. `unknown` and `blocked` remain actionable gaps. `not_published` is
terminal only with an official source explicitly stating that the price is not
published/final. Likewise, `not_exposed` means the customer surface deliberately
omits a provider rule while preserving it in `provider_pricing`. Never invent a
context, maximum-output limit, or price merely to silence the matrix.

`codex-client-base.json` owns the baseline Codex model list. The generator uses
that same list for both the downloadable Codex catalog and CC Switch imports, so
removing a model there removes it from both client configuration paths.
