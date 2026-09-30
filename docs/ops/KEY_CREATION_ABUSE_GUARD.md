# API key creation abuse guard

Scope: key rotation abuse and deleted-user audit visibility. Account balances,
subscription usage and user concurrency remain unchanged.

- Repository creation locks the owner row before counting and inserting within
  the existing transaction boundary. Random, custom, personal, team and admin
  creation paths share the same user budget.
- All persisted keys, including soft-deleted ones, count toward user limits:
  20 creations per rolling hour, 100 per rolling day, 1,000 lifetime creations.
  Successful creations count; failed inserts roll back. At the lifetime cap,
  support must review the account rather than resetting counters by deletion.
- The authenticated creation route reuses the existing atomic IP limiter:
  120 attempts per hour and 500 per day, fail-closed if Redis is unavailable.
  Reads and deletions are unaffected. IP windows expire; IP addresses are not
  permanently banned. No new table, migration or policy/configuration framework.
- Key deletion keeps failed-custom-key counters and invalidates authentication
  caches after deletion, not before the database commit.
- Admin user listing defaults to excluding deleted users. Explicit
  `include_deleted=true` includes them and returns admin-only `deleted_at`.
  Ordinary authentication/lookups continue excluding deleted users; the UI
  labels deleted records and hides editing/deposit actions.

## Regression and ablation

Real PostgreSQL tests cover all three user budgets including deleted records,
delete/recreate cycles, window expiry, failed inserts and concurrent creation.
Real Redis route tests cover both IP windows and random/custom creation, while
unit tests cover fail-closed dependency failures without blocking read/delete.
Admin API and mounted UI tests cover the opt-in and readonly deleted-user state.

Four independent temporary removals must make the associated tests fail:
owner lock, counting deleted keys, preservation of deletion counters, and
explicit deleted-user query context. Restore each variant immediately and run
positive regressions on the final tree. Do not deploy an ablation variant or
exercise high-volume churn against customers in production.

Creation-specific SQLite tests were moved to real PostgreSQL: no production
fallback or weakened lock is added merely to support a test database.
