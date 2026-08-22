# API Key upstream foundations (PR1)

Source reference: formal Wei-Shaw/Sub2API release `v0.1.179`.

## Included

- API-key-only protocol vocabulary and deterministic native-first decision:
  `chat_completions`, `anthropic`, `responses`, `adaptive`.
- Per-protocol upstream Base URL lookup from `credentials.api_base_urls`.
- Tenant-scoped Redis reasoning cache keyed by user ID, API key ID, model and
  Responses reasoning item ID. Plaintext is capped at 256 KiB and 24 hours.
- Channel probe classification where only HTTP 401/403 invalidates a
  credential; 429, 5xx and transport errors remain operational states.
- Channel `fast_multiplier` / `flex_multiplier` persistence, admin editing and
  billing support, including account-stat pricing.
- Fast policy consistency: a filtered service tier is neither forwarded nor
  recorded for Fast billing.

## Deliberately dormant

- Adaptive protocol decisions are not yet wired into customer routing. PR2
  owns the universal group and protocol bridge integration.
- The reasoning cache has no caller until PR2 adds the Responses-to-Chat
  thinking bridge.
- The channel-health classifier has no automatic account-disable side effect.
- Production OpenAI Fast policy remains `filter`; this PR does not enable Fast,
  create a universal group, migrate existing keys, or deploy anything.

## Excluded

- OAuth, setup-token and Codex-session account management.
- Upstream migration numbers 226-228.
- Upstream long-context billing gate change from AND to OR.
- Composite account copying and production intelligent-routing enforcement.
