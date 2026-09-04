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

## Required task closure

When finishing a merged task or when the owner requests cleanup:

1. Read live `git worktree list --porcelain`, local refs, each checkout status,
   and commits not contained in `origin/main`. A successful deployment alone
   does not mean worktrees or branches were cleaned.
2. Keep the canonical checkout and clean detached release-only checkout.
   Do not delete another active task's workspace or silently discard unique work.
3. Preserve unique commits in a verified Git bundle and record tips, diffs,
   checksums, and the disposition of each branch before removing it. A QA branch
   that overwrote CI must not be merged wholesale into production.
4. Keep reproducible build caches out of archives. Preserve local staging
   `deploy/data`, `deploy/postgres_data`, and `deploy/redis_data` outside a retiring
   worktree; cleanup is not authorization to reset test data. Do not export
   credentials into reports or bundles; use Agent Switch for secret values.
5. Stop this task's preview processes, then use `git worktree remove`. Use normal
   `git branch -d` for merged branches. An unmerged local ref may be removed only
   after its work was reviewed and its recoverable archive verified.
6. Remove retired temporary entries from `checkouts.json`. Update the canonical
   configuration/documentation through a reviewed PR, not a direct main push.
   Delete the cleanup branch after its merge too.
7. Re-read actual worktrees, local refs and dirty status. Report local cleanup,
   remote branch disposition, and production deployment separately.

The registry is an **action allowlist**, not a list of physically present Git
worktrees. The two `report-only` automation paths remain reserved guardrails;
absent directories there must not be reported as active worktrees. Do not
upgrade their permissions to develop or release merely to make a check pass.

## Configuration ownership and 2026-09-04 cleanup

- Gateway source: `/Users/fujunhao/laoshirenai/code/laoshirenai-Sub2API`.
- Release controller: `/Users/fujunhao/laoshirenai/local/release-worktree`.
- Client capability source: Skill Hub
  `/Users/fujunhao/AgentWorkspace/skill-hub/own/laoshirenai-skills/skills/laoshirenai-client-integration/references/client-matrix.json`.
- Tested one-line setup source: the sibling
  `laoshirenai-one-line-client-setup` directory and its
  `references/tested-adapters.json`. The WorkBuddy QA renderer and Windows test
  were byte-identical to these authoritative copies before cleanup; removing
  the gateway QA duplicate does not remove the supported adapter.
- Gemini CLI 0.58.0 repair files are retained only as version-specific QA
  evidence, not enabled as a general installer or a new global Skill. Its
  branch-local replacement of `.github/workflows/ci.yml` is not production CI.
  Run archived QA tests only in isolated fixtures/runners, never against a
  customer or the owner's real client configuration directory.
- Recovery archive:
  `/Users/fujunhao/laoshirenai/local/cleanup-audits/gateway-20260904-final`.
  `dispositions.json`, verified incremental `*.bundle` files, extracted QA
  files and `SHA256SUMS` preserve the two unmerged QA histories (3 and 13 commits).
  Restore into a new reviewed branch by fetching the recorded ref from its
  bundle in a clone containing the recorded main-history prerequisites.
- Preserved staging volumes:
  `/Users/fujunhao/laoshirenai/local/staging-data/multi-group-20260904`.
  These are local runtime data, not source code or public deliverables.

Closure target: two actual worktrees and only the local `main` branch. Remote
QA refs are retained as additional recovery copies; this local cleanup does not
claim to delete remote branches or deploy documentation-only changes.
