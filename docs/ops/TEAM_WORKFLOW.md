# Team Development Workflow

This project is a private production repository. Human collaborators and AI agents should use the same rules.

## Roles

- Owner/admin: `nobody396`
- Current collaborators observed by GitHub API:
  - `JNHFlow21`: `write`
  - `Freddiefine777`: `write`

`write` permission usually allows a collaborator to clone the repo, push branches, open pull requests, review pull requests, and merge pull requests when branch protection does not block it. This repository currently does not have enforceable branch protection available through the checked API response, so collaborators should treat direct pushes to `main` as forbidden by team policy even if GitHub technically allows them.

## Default Rule

Do not deploy production unless the owner explicitly says one of these:

- `上线`
- `发布`
- `部署到生产`
- an equally direct production release instruction

Normal code work stops after:

1. local checks pass
2. commit is pushed
3. GitHub Actions image build succeeds

## Recommended Human Workflow

1. Create a branch from `main`.

```bash
git checkout main
git pull origin main
git checkout -b feat/short-description
```

2. Make focused changes.

3. Run relevant checks:

```bash
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
go test ./...
```

Choose the checks that match the touched area. Frontend-only changes do not always need full backend tests, but user-facing changes should at least run typecheck or build.

4. Push the branch.

```bash
git push origin feat/short-description
```

5. Open a pull request into `main`.

6. Ask an AI reviewer or owner to check the PR before merge.

7. Merge only when:

- the PR scope is clear
- tests or builds passed
- AI review does not find blocking risk
- a human accepts the product change

8. After merge, required GitHub Actions builds the exact commit and records its immutable `image@sha256` artifact. Moving tags are not release evidence.

9. Stop. Do not deploy production unless the owner explicitly asks.

## AI Review Prompt

Use this prompt when asking Codex or another AI to review a PR:

```text
Review this PR for production risk in the private laoshiren-ai-gateway repository.

Read AGENTS.md and docs/ops/ENVIRONMENTS.md first.

Focus on:
- whether the change can break existing login, dashboard, API, billing, model routing, provider account, PostgreSQL, Redis, or deployment behavior
- whether any secrets were added
- whether the change touches production deployment or database migration paths
- whether tests or builds are missing for the touched area

Return:
1. blocking issues
2. non-blocking issues
3. recommended checks before merge
4. whether this is safe to merge

Do not deploy production.
```

## Production Release

Production release is separate from merge.

Only after the owner explicitly requests production release, use the documented deploy process in `AGENTS.md` and `docs/ops/ENVIRONMENTS.md`.

Local release commands must run from the canonical checkout registered in `docs/ops/checkouts.json`; report-only clones and temporary worktrees fail closed. CI build checkouts are ephemeral and may build, but production consumes only their verified immutable artifact.

The production app service is:

```text
laoshirenai-app-tazu5m
```

Do not recreate or replace PostgreSQL or Redis for ordinary code changes.
