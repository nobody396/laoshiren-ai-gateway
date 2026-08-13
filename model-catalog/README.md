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
