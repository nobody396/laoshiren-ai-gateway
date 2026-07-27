# Checkout governance

`docs/ops/checkouts.json` is the machine-readable registry. Every local clone or
worktree has one owner, an action allowlist and, for temporary worktrees, a TTL.

- The `canonical` checkout and the registered `release-only` checkout may
  execute a local production `release` action.
- `/Users/fujunhao/laoshirenai/local/release-worktree` is a clean, detached,
  machine-managed checkout. It is the preferred release controller so unrelated
  development changes in the canonical checkout never delay or contaminate a
  deployment. Do not develop or hand-edit files there.
- `report-only` clones may read and produce reports; they must not modify source,
  push, merge, build release images or deploy.
- Temporary worktrees may develop and test until their TTL, then must be removed
  or explicitly renewed in review.
- CI may build an exact commit in its ephemeral checkout, but deployment still
  consumes only the verified CI `image@sha256` artifact.

Validate the current checkout with `make checkout-validate`. Release tooling
uses `tools/validate_checkout_registry.py --action release` and fails closed.
The deploy Skill creates or refreshes the registered release-only worktree from
the exact remote `main` commit before consuming CI's immutable artifact.
