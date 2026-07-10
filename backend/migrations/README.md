# Database Migrations

## Overview

This directory contains SQL migration files for database schema changes. The migration system uses SHA256 checksums to ensure migration immutability and consistency across environments.

## Migration File Naming

Format: `NNN_description.sql`
- `NNN`: Sequential number (e.g., 001, 002, 003)
- `description`: Brief description in snake_case

Example: `017_add_gemini_tier_id.sql`

### `_notx.sql` 命名与执行语义（并发索引专用）

当迁移包含 `CREATE INDEX CONCURRENTLY` 或 `DROP INDEX CONCURRENTLY` 时，必须使用 `_notx.sql` 后缀，例如：

- `062_add_accounts_priority_indexes_notx.sql`
- `063_drop_legacy_indexes_notx.sql`

运行规则：

1. `*.sql`（不带 `_notx`）按事务执行。
2. `*_notx.sql` 按非事务执行，不会包裹在 `BEGIN/COMMIT` 中。
3. `*_notx.sql` 仅允许并发索引语句，不允许混入事务控制语句或其他 DDL/DML。

幂等要求（必须）：

- 创建索引：`CREATE INDEX CONCURRENTLY IF NOT EXISTS ...`
- 删除索引：`DROP INDEX CONCURRENTLY IF EXISTS ...`

这样可以保证灾备重放、重复执行时不会因对象已存在/不存在而失败。

## Migration File Structure

```sql
-- NNN_describe_forward_change.sql
-- New migrations contain forward-only, idempotent SQL with no Goose Down block.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE example ADD COLUMN IF NOT EXISTS new_field TEXT;
```

The runner still parses the immutable historical Goose files and executes only
their `Up` sections while retaining checksums over the original complete files.
`Down` sections are forbidden in new migration files. Application rollback uses
the previously recorded image digest against the expanded schema; it never runs
database rollback SQL.

## Important Rules

### ⚠️ Immutability Principle

**Once a migration is applied to ANY environment (dev, staging, production), it MUST NOT be modified.**

Why?
- Each migration has a SHA256 checksum stored in the `schema_migrations` table
- Modifying an applied migration causes checksum mismatch errors
- Different environments would have inconsistent database states
- Breaks audit trail and reproducibility

### ✅ Correct Workflow

1. **Create new migration**
   ```bash
   # Create new file with next sequential number
   touch migrations/018_your_change.sql
   ```

2. **Write a forward-only migration**
   - Use additive, idempotent SQL.
   - Preserve compatibility with the currently deployed application.
   - Correct mistakes with a later compensating migration; never add `Down`.
   - Add bounded `SET LOCAL lock_timeout` and `statement_timeout` when the
     migration takes locks or performs a backfill.

3. **Test locally**
   ```bash
   # Parser, direction, and checksum contracts
   go test ./internal/repository -run 'Migration|Migrations'

   # Real PostgreSQL migration/schema/idempotency checks
   go test -tags=integration ./internal/repository -run 'Migration|Migrations|IntegrationHarnessSentinel'
   ```

4. **Commit and deploy**
   ```bash
   git add migrations/018_your_change.sql
   git commit -m "feat(db): add your change"
   ```

### ❌ What NOT to Do

- ❌ Modify an already-applied migration file
- ❌ Delete migration files
- ❌ Change migration file names
- ❌ Reorder migration numbers
- ❌ Add `Down` to a new migration
- ❌ Reverse the database schema when rolling the application back

### 🔧 If You Accidentally Modified an Applied Migration

**Error message:**
```
migration 017_add_gemini_tier_id.sql checksum mismatch (db=abc123... file=def456...)
```

**Solution:**
```bash
# 1. Find the original version
git log --oneline -- migrations/017_add_gemini_tier_id.sql

# 2. Revert to the commit when it was first applied
git checkout <commit-hash> -- migrations/017_add_gemini_tier_id.sql

# 3. Create a NEW migration for your changes
touch migrations/018_your_new_change.sql
```

## Migration System Details

- **Checksum Algorithm**: SHA256 of trimmed file content
- **Tracking Table**: `schema_migrations` (filename, checksum, applied_at)
- **Runner**: `internal/repository/migrations_runner.go`
- **Auto-run**: Migrations run automatically on service startup
- **Execution Direction**: Unmarked files run as-is; the immutable historical
  Goose files run only their single validated `Up` section
- **Serialization**: Advisory lock, metadata queries, migration SQL, and unlock
  all use one dedicated PostgreSQL `*sql.Conn` session

### Declared Domain Prerequisite

`138_add_apex_monthly_card_groups.sql` deliberately requires the canonical
`GPT Ultra 月卡组` and `Claude Ultra 月卡组` domain rows. The integration harness:

1. starts with an empty schema and characterizes the raw-blank failure at 138;
2. verifies that failed migration 138 was not recorded;
3. inserts the two declared prerequisite fixtures after migrations 001–137;
4. reruns the runner and requires every non-empty embedded migration to finish.

Therefore the supported bootstrap claim is **empty schema plus the declared
domain prerequisite**, not “a raw blank database has zero prerequisites.”

## Best Practices

1. **Keep migrations small and focused**
   - One logical change per migration
   - Easier to review and to keep application rollback compatible

2. **Use expand-contract migrations**
   - Expand the schema first and keep old readers/writers compatible.
   - Deploy application changes separately.
   - Remove obsolete columns only in a later, isolated cleanup release.
   - Roll the application back by exact image digest, never with `Down` SQL.

3. **Use transactions**
   - Wrap DDL statements in transactions when possible
   - Ensures atomicity

4. **Add comments**
   - Explain WHY the change is needed
   - Document any special considerations

5. **Test in development first**
   - Apply migration locally
   - Verify data integrity
   - Re-run for idempotency and start the previous application against the expanded schema

6. **Split schema and backfill work**
   - Additive schema changes and historical data backfills should be separate migrations
   - This keeps failure domains smaller and simplifies rollback analysis

7. **Backfills must be idempotent**
   - Re-running a backfill must not duplicate rows or corrupt aggregates
   - Prefer stable dedupe keys, `ON CONFLICT`, and existence guards

8. **Destructive cleanup ships later**
   - Dropping columns/tables or deleting legacy data should happen in a later, isolated release
   - Roll out new readers/writers first, verify production behavior, then remove obsolete structures

## Example Migration

```sql
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

-- Add tier_id field to Gemini OAuth accounts for quota tracking
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{tier_id}',
    '"LEGACY"',
    true
)
WHERE platform = 'gemini'
  AND type = 'oauth'
  AND credentials->>'tier_id' IS NULL;
```

## Troubleshooting

### Checksum Mismatch
See "If You Accidentally Modified an Applied Migration" above.

### Migration Failed
```bash
# Check migration status
psql -d sub2api -c "SELECT * FROM schema_migrations ORDER BY applied_at DESC;"

# Do not execute historical Down SQL. Fix forward with a new migration.
```

### A Migration Has an Unmet Domain Prerequisite

Do not forge a `schema_migrations` row. Stop, declare and validate the missing
prerequisite, then rerun the unchanged migration. The failed transaction must
leave neither partial schema/data nor a migration record.

## References

- Migration runner: `internal/repository/migrations_runner.go`
- Goose syntax: https://github.com/pressly/goose
- PostgreSQL docs: https://www.postgresql.org/docs/
