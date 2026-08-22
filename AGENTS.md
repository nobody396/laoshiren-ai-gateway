# Project Operating Rules

This repository powers the production site at `laoshirenai.com`. Treat production changes as controlled releases.

## Required Context

Before changing code, reviewing pull requests, deploying, backing up data, or touching infrastructure, read:

- `docs/ops/ENVIRONMENTS.md` for production environment context, domains, server, database, Redis, CDN, GitHub/GHCR, and backup entry points.
- `docs/ops/TEAM_WORKFLOW.md` for team collaboration, PR review, merge, and release policy.
- `docs/ops/CHECKOUTS.md` for checkout ownership and release eligibility.

## Default Workflow

1. Make code changes locally.
2. Run relevant local checks before committing.
3. Use a clear Conventional Commit message.
4. Report the change summary, checks run, and commit hash to the owner.
5. Do not deploy production until the owner explicitly says it can go online.
6. After production deployment, verify `/health` and the relevant user-facing page or API.

## Commit Messages

Use Conventional Commit style:

- `fix(scope): ...` for bug fixes.
- `feat(scope): ...` for new features.
- `chore(scope): ...` for maintenance, config, or deployment work.
- `docs(scope): ...` for documentation only.
- `refactor(scope): ...` for behavior-preserving code restructuring.

Messages must describe the user-visible or operational effect, not just the file changed.

Good examples:

- `fix(agent): use configured invitee bonus rate`
- `feat(auth): add email password reset`
- `chore(deploy): add manual production release workflow`

## Production Deployment Policy

Production deployment is manual by default.

- Local changes do not affect production.
- A commit pushed to GitHub may build a Docker image, but must not automatically update production.
- Production service updates require explicit owner approval, such as "可以上线", "现在上线", or an equivalent direct instruction.
- Do not restart or update the production Docker service without that approval.

## Current Deployment Shape

- Source repository: `nobody396/laoshiren-ai-gateway`.
- Docker image: CI publishes an exact-commit artifact; production and rollback use only the verified `image@sha256` reference.
- Production is managed by Dokploy / Docker Swarm on the Hostinger server.
- Current production application service name: `laoshirenai-app-tazu5m`.

## Recommended Checks

Choose checks based on the change, but prefer:

- Backend service changes: `go test ./internal/service ./internal/handler/...`.
- Backend compile check: `go build -o /tmp/sub2api-server-check ./cmd/server`.
- Frontend changes: `pnpm --dir frontend run typecheck`.
- Frontend production build: `pnpm --dir frontend run build`.
- Lint only modified frontend files if the full lint is blocked by unrelated existing issues.

## Frontend theming

`frontend/src/styles/theme.css` is the only place colours and motion timings are
defined. `tailwind.config.js` holds no literal values — it points at the tokens.
Do not introduce a hex, `rgb()`, or `rgba()` literal in a component; reach for a
token. The only standing exemptions are third-party brand marks (`ModelIcon.vue`,
the OAuth provider icons) and the QR code, which must stay black-on-white to scan.

Four things that are easy to get wrong here, each of which shipped a visible bug:

- **Tokens store bare RGB triples** (`248 243 231`), because Tailwind's opacity
  modifiers need that form. So a colour must be written `rgb(var(--token))`, or
  `rgb(var(--token) / 0.4)`. A bare `var(--token)` expands to an invalid
  declaration, the browser drops the whole rule, and the element falls back to
  white.
- **Numbered ramps do not flip; semantic tokens do.** `--color-gray-700` holds one
  value in both themes and is meant to be selected via Tailwind's `dark:` variant.
  Hand-written CSS that uses a ramp stop as a text colour pins dark text in place
  while the surface behind it goes dark. Use `--color-ink` / `--color-muted` /
  `--color-parchment` in stylesheets.
- **Scrims must not flip.** A modal backdrop built on `--color-ink-deep` becomes a
  *brightener* under `.dark`. Use `--lacquer-base`, which deliberately holds its
  value.
- **A component's own `transition:` shorthand overrides a shared class's**,
  including its `transition-delay`. Scroll-reveal and other cross-component motion
  therefore use `animation`, not `transition`.

Canvas cannot read CSS variables, so charts resolve tokens at runtime through
`frontend/src/utils/chartPalette.ts`. That module is the only place allowed to
read a theme token from JavaScript; never paste chart hexes into a component.

## Secrets

Never commit secrets, tokens, OAuth client secrets, SMTP passwords, SSH keys, or admin API keys. Agent Switch is the only local secret/MCP control plane. Inspect names with `agent-switch secret list`; write values only with `agent-switch secret set --stdin NAME` or `--fd`, never command arguments or project `.env` files.

## Domain docs

**Reliability, Service Status, Channel Monitoring, incidents, routing evidence, Customer Tier, or compensation:** follow `docs/agents/domain.md`.
