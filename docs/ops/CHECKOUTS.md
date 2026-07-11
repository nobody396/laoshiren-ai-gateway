# Checkout governance

`docs/ops/checkouts.json` is the machine-readable registry. Every local clone or
worktree has one owner, an action allowlist and, for temporary worktrees, a TTL.

- Only the `canonical` checkout may execute a local production `release` action.
- `report-only` clones may read and produce reports; they must not modify source,
  push, merge, build release images or deploy.
- Temporary worktrees may develop and test until their TTL, then must be removed
  or explicitly renewed in review.
- CI may build an exact commit in its ephemeral checkout, but deployment still
  consumes only the verified CI `image@sha256` artifact.

Validate the current checkout with `make checkout-validate`. Release tooling
uses `tools/validate_checkout_registry.py --action release` and fails closed.
