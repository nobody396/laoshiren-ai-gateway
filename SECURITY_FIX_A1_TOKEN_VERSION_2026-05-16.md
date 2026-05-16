# Security Fix Record: A1 TokenVersion Session Revocation

Date: 2026-05-16
Repository: `/Users/fujunhao/laoshirenai/DragonCode-sub2api`
Status: Fixed locally, not deployed

## Fixed Issue

P0 A1 from `SECURITY_AUDIT_HANDOFF_2026-05-16.md`:

`TokenVersion` was checked by JWT and refresh-token validation, but it was not persisted in the `users` database table. Because of that, password changes, password resets, and "revoke all sessions" could fail to invalidate old access tokens or refresh tokens reliably.

## What Changed

- Added persisted `users.token_version`.
- Added migration: `backend/migrations/120_add_users_token_version.sql`.
- Regenerated Ent ORM files for the new field.
- Updated user repository mapping so `TokenVersion` is loaded and saved.
- Added atomic repository operations:
  - update password and increment `token_version`
  - increment `token_version`
- Updated password change, password reset, and revoke-all-session flows:
  - password change invalidates old access tokens and refresh tokens
  - password reset invalidates old access tokens and refresh tokens
  - `POST /api/v1/auth/revoke-all-sessions` now invalidates old access tokens too
- Added regression tests for old token rejection.

## Important Deployment Note

This fix requires the production database to have this column:

```sql
users.token_version BIGINT NOT NULL DEFAULT 0
```

I did not connect to production, did not run production migrations, and did not deploy. This was intentional because the repair instructions explicitly forbid production database access, production migrations, and deployment.

Before this code is deployed to production, make sure the migration is applied through the normal controlled release process. Do not deploy the new backend code against a production database that lacks `users.token_version`, because login/auth queries will read/write that column.

## What You Need To Do Before Going Online

1. Confirm the code changes are reviewed and committed.
2. Confirm production release approval.
3. During the approved release, ensure `backend/migrations/120_add_users_token_version.sql` runs against the production database.
4. Deploy the backend only after the migration path is confirmed.
5. After deploy, verify login, password change, password reset if enabled, and revoke-all-sessions behavior.

If the app automatically applies embedded migrations at startup, the release still needs to include a check that the migration succeeded. If production migrations are manual, run this migration as part of the release checklist.

## Validation Already Run Locally

Passed:

```bash
go test -tags=unit ./internal/service ./internal/server/middleware -run 'Test(AuthRevocation|JWTAuth_TokenVersionMismatch|AdminAuthJWTValidatesTokenVersion)' -count=1
go test -tags=integration ./internal/repository -run 'TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate|TestUserRepoSuite/Test(ChangePasswordPersistsTokenVersionAndRejectsOldAccessToken|ChangePasswordRejectsOldRefreshToken|RevokeAllUserSessionsRejectsOldAccessToken)' -count=1
go test ./internal/repository -count=1
go build -o /tmp/sub2api-server-check ./cmd/server
```

Known unrelated test issue:

```bash
go test -tags=unit ./internal/service ./internal/server/middleware -count=1
```

This wider command failed on an existing unrelated service test:

`TestAdminService_UpdateUser_RejectsAdminRoleDowngrade`

The targeted A1 tests passed.

## Next Security Queue Item

Do not continue automatically. Wait for owner confirmation.

Next P0 item from the handoff:

`RBAC menu permission_key="*" wildcard privilege escalation`
