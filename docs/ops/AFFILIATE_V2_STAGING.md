# Affiliate V2 isolated staging

This staging stack is exclusively for the Affiliate V2 worktree and is not a
production release path.

## Isolation boundary

- Worktree: `/Users/fujunhao/laoshirenai/worktrees/affiliate-program-v2`
- Branch: `feat/affiliate-program-v2-20260726`
- Compose project: `laoshirenai-affiliate-v2-staging`
- URL: `http://127.0.0.1:18080`
- Dedicated PostgreSQL, Redis, application data, network, image and volumes
- No production database, Redis, user data, upstream accounts or credentials
- All staging secrets live only in Agent Switch

On a blank database, the launcher supplies and then retires two disabled,
staging-only source-group fixtures required by immutable historical migration
138. The fixtures never reach production and are soft-deleted immediately after
the migration stream completes.

The stack binds only to loopback. It cannot replace or update the Dokploy/
Docker Swarm production service. Production remains subject to the explicit
release policy in `AGENTS.md`.

## Commands

```bash
cd /Users/fujunhao/laoshirenai/worktrees/affiliate-program-v2
./scripts/affiliate-v2-staging.sh init-secrets
./scripts/affiliate-v2-staging.sh validate
./scripts/affiliate-v2-staging.sh up
./scripts/affiliate-v2-staging.sh status
./scripts/affiliate-v2-staging.sh logs
./scripts/affiliate-v2-staging.sh down
```

Reset only the isolated staging data:

```bash
./scripts/affiliate-v2-staging.sh reset --confirm-staging-data-reset
```

Do not create a project `.env`. The script loads the five staging-only secret
values through non-TTY file descriptors and exports them only to the Compose
process.

## Acceptance policy

1. Every feature is committed before it enters this staging stack.
2. Staging starts with a blank, isolated database; migrations must pass from
   zero and remain idempotent.
3. Affiliate mode remains `off` until the feature set is ready.
4. Acceptance first uses `shadow` mode to prove calculations create no money.
5. `live` mode is used only inside this isolated staging database.
6. A staging pass does not authorize production deployment or data migration.
