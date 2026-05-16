# Project Operating Rules

This repository powers the production site at `laoshirenai.com`. Treat production changes as controlled releases.

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
- Docker image: `ghcr.io/nobody396/laoshiren-ai-gateway:main`.
- Production is managed by Dokploy / Docker Swarm on the Hostinger server.
- Current production application service name: `laoshirenai-app-tazu5m`.

## Recommended Checks

Choose checks based on the change, but prefer:

- Backend service changes: `go test ./internal/service ./internal/handler/...`.
- Backend compile check: `go build -o /tmp/sub2api-server-check ./cmd/server`.
- Frontend changes: `pnpm --dir frontend run typecheck`.
- Frontend production build: `pnpm --dir frontend run build`.
- Lint only modified frontend files if the full lint is blocked by unrelated existing issues.

## Secrets

Never commit secrets, tokens, OAuth client secrets, SMTP passwords, SSH keys, or admin API keys. Keep local secrets in private local files only.
