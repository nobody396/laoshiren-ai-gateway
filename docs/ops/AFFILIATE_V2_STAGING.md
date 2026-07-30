# Affiliate V2 isolated staging

This staging stack is exclusively for a registered, non-release feature/fix
worktree and is not a production release path.

## Isolation boundary

- Worktree: the repository containing the invoked staging script
- Branch: the currently checked-out non-release feature/fix branch
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
cd /path/to/a/registered/temporary/worktree
./scripts/affiliate-v2-staging.sh init-secrets
./scripts/affiliate-v2-staging.sh validate
./scripts/affiliate-v2-staging.sh up
./scripts/affiliate-v2-staging.sh status
./scripts/affiliate-v2-staging.sh smoke
./scripts/affiliate-v2-staging.sh logs
./scripts/affiliate-v2-staging.sh down
```

Use `AFFILIATE_STAGING_PROJECT_NAME` and `AFFILIATE_STAGING_PORT` to run an
additional isolated stack without replacing another local staging stack. The
script rejects `main`, `master`, and `release/*` branches and still requires the
checkout registry to allow development.

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

## Acceptance sequence

The admin entry is `/login`; the staging-only admin email is
`affiliate-staging@local.invalid`. Its password remains in Agent Switch and is
never written to the checkout or printed by the launcher.

1. Keep the program in `off` and verify the commercial catalog, 35% margin
   floor, platform-credit symbol and configuration defaults.
2. Change `off -> shadow`. Register ordinary users, bind both ordinary and Agent
   links, redeem paid cards and generate usage. Confirm performance events are
   observable but no platform reward, cash commission or wallet balance is
   credited.
3. Change `shadow -> live` inside this isolated database. Repeat with new
   purchases and usage after `started_at`.
4. Verify ordinary first-paid settlement: inviter and invitee each receive 5%
   platform credits at T+0. There is no minimum amount and no fixed `⚡5`.
5. Verify Agent qualification using either:
   - ten direct consumers, each at least ¥20, and at least ¥1,000 direct-team
     confirmed consumption; or
   - at least ¥2,000 direct-team confirmed consumption. The applicant's own
     consumption does not count.
6. Activate the qualified Agent and verify one permanent upstream edge, one
   always-active default link, at most five campaign links, and dynamic
   customer rebate from 0% to 10% in 1% increments.
7. Verify each Agent-bound consumption creates exactly one fixed 10% pool:
   customer `⚡` plus direct Agent cash; no recursive or self commission.
8. Submit Alipay payout details and QR, verify them in Admin, create a partial
   withdrawal, and confirm the user-visible path is only
   `处理中 -> 已到账`. A failed payment must restore available cash.
9. Convert available commission to `⚡` and verify the configured 1.2x value,
   non-withdrawable source lot and no further affiliate eligibility.
10. Put an Agent in review/blocked risk state, verify new links, withdrawals and
    settlement are blocked, then test hold release and one-time reversal.

`./scripts/affiliate-v2-staging.sh smoke` performs the non-mutating health,
admin-login, program-setting, commercial-margin and risk-queue checks without
printing credentials or access tokens.

## External commerce boundary

The isolated stack intentionally does not contact LDXP, production Alipay,
upstream model accounts or production storage. Purchase callbacks and QR
payments must be validated with staging fixtures or manual card redemption.
The external catalog cutover is separately gated by
`docs/ops/AFFILIATE_V2_COMMERCIAL_CUTOVER.md`.
