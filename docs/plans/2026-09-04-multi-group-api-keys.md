# Multi-group API key contract

Status: implementation branch; NOT deployed. Owner approved one key with explicitly selected groups, default selecting all currently eligible groups in the UI. Not Auto model selection and no new protocol bridge.

- `group_ids` is a non-empty ordered snapshot (max 100), not a wildcard that silently authorizes future groups. `group_id` and `group_ids` are mutually exclusive. Omitted group_ids preserves legacy keys; explicit group_id updates may return a key to legacy mode.
- Same-model conflicts use user-visible group priority. Selection uses declared model rate cards, not transient upstream health, so a failure must not silently switch price/funding source. Authorization/billing is rechecked on the selected group; exhausted subscriptions do NOT fall through to wallet. Account failover within that group remains existing behavior.
- Existing keys, monthly entitlements and accounting commands remain unchanged. Group/key/user revocation must invalidate auth correctly. Team key authorization is the current payer's, not the actor's.
- Phase-one native HTTP endpoints: Messages, Chat, Responses and Gemini generation/counting, plus authorized union model discovery. Unsupported stateful/multipart/media paths fail closed rather than use arbitrary first group; keep legacy keys for them until an explicit tested continuation contract is added.
- No secrets in logs/fixtures. UI cannot claim every protocol/model/client combination works.
- Required tests: CRUD authorization/order/empty/duplicate, cache roundtrip, deterministic route selection and no billing fallback, protocol recognition, discovery access, legacy behavior, frontend defaults/order/edit/scope changes, schema migration and CI.
