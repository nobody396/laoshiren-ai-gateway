//go:build integration

package repository

import (
	"context"
	"errors"
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	embeddedmigrations "github.com/bozhouDev/DragonCode-sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestApplyMigrationsFS_KeepsAdvisoryLockAndSQLOnOneSession(t *testing.T) {
	ctx := context.Background()
	const filename = "998_test_migration_session.sql"
	_, err := integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename = $1", filename)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM schema_migrations WHERE filename = $1", filename)
	})

	// pg_advisory_unlock returns true only when the transaction executes on the
	// same PostgreSQL session that acquired the runner lock. Reacquire it before
	// returning so the runner's deferred unlock still balances the lock count.
	fsys := fstest.MapFS{
		filename: &fstest.MapFile{Data: []byte(`
DO $assert_migration_session$
BEGIN
    IF NOT pg_advisory_unlock(694208311321144027) THEN
        RAISE EXCEPTION 'migration SQL is not running on the advisory-lock session';
    END IF;
    PERFORM pg_advisory_lock(694208311321144027);
END
$assert_migration_session$;
`)},
	}

	require.NoError(t, applyMigrationsFS(ctx, integrationDB, fsys))

	var recorded int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE filename = $1", filename).Scan(&recorded))
	require.Equal(t, 1, recorded)
}

func TestPgAdvisoryLock_CancellationLeavesDedicatedConnectionReusable(t *testing.T) {
	ctx := context.Background()
	lockConn, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = lockConn.Close() }()

	waitConn, err := integrationDB.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = waitConn.Close() }()

	require.NoError(t, pgAdvisoryLock(ctx, lockConn))
	defer func() { _ = pgAdvisoryUnlock(context.Background(), lockConn) }()

	waitCtx, cancel := context.WithTimeout(ctx, 75*time.Millisecond)
	err = pgAdvisoryLock(waitCtx, waitConn)
	cancel()
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded), err)

	require.NoError(t, pgAdvisoryUnlock(ctx, lockConn))
	require.NoError(t, pgAdvisoryLock(ctx, waitConn), "cancelled waiter connection must remain reusable")
	require.NoError(t, pgAdvisoryUnlock(ctx, waitConn))
}

func TestApplyMigrationsFS_ConcurrentRunnersSerialize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const filename = "997_test_concurrent_runner.sql"
	_, err := integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename = $1", filename)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM schema_migrations WHERE filename = $1", filename)
	})

	fsys := fstest.MapFS{
		filename: &fstest.MapFile{Data: []byte("SELECT pg_sleep(0.15);")},
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- applyMigrationsFS(ctx, integrationDB, fsys)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for migrationErr := range errs {
		require.NoError(t, migrationErr)
	}

	var recorded int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE filename = $1", filename).Scan(&recorded))
	require.Equal(t, 1, recorded)
}

func TestApplyMigrationsFS_SQLFailureRollsBackSchemaAndRecord(t *testing.T) {
	ctx := context.Background()
	const filename = "996_test_atomic_failure.sql"
	const table = "u1_migration_atomic_failure"
	_, _ = integrationDB.ExecContext(ctx, "DROP TABLE IF EXISTS "+table)
	_, err := integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename = $1", filename)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+table)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM schema_migrations WHERE filename = $1", filename)
	})

	fsys := fstest.MapFS{
		filename: &fstest.MapFile{Data: []byte(`
CREATE TABLE u1_migration_atomic_failure (id BIGINT PRIMARY KEY);
SELECT * FROM u1_relation_that_does_not_exist;
`)},
	}
	err = applyMigrationsFS(ctx, integrationDB, fsys)
	require.Error(t, err)
	require.Contains(t, err.Error(), "apply migration")

	var tableExists bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT to_regclass('public."+table+"') IS NOT NULL").Scan(&tableExists))
	require.False(t, tableExists)

	var recorded int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE filename = $1", filename).Scan(&recorded))
	require.Zero(t, recorded)
}

func TestMigration142_RepairsLegacyRollbackSideEffectsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	compensationSQL := migration142SQL(t)

	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, wechat)
VALUES ('u1-migration-damaged@example.invalid', 'not-a-real-password-hash', 'legacy-wechat')
RETURNING id
`).Scan(&userID))

	var accountID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, credentials)
VALUES (
    'u1 migration gemini',
    'gemini',
    'oauth',
    '{"oauth_type":"code_assist","project_id":"migration-test"}'::jsonb
)
RETURNING id
`).Scan(&accountID))

	// Reproduce the known effects of accidentally executing 019/024/037 Down.
	_, err := tx.ExecContext(ctx, `
DELETE FROM user_attribute_values
WHERE attribute_id IN (
    SELECT id FROM user_attribute_definitions
    WHERE key = 'wechat' AND deleted_at IS NULL
)
`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
UPDATE user_attribute_definitions
SET deleted_at = NOW()
WHERE key = 'wechat' AND deleted_at IS NULL
`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
UPDATE accounts
SET credentials = credentials - 'tier_id'
WHERE id = $1
`, accountID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `DROP TABLE ops_alert_silences`)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, compensationSQL)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, compensationSQL)
	require.NoError(t, err, "migration 142 must remain idempotent")

	var activeDefinitions int
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM user_attribute_definitions
WHERE key = 'wechat' AND deleted_at IS NULL AND enabled = true
`).Scan(&activeDefinitions))
	require.Equal(t, 1, activeDefinitions)

	var attributeValue string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT uav.value
FROM user_attribute_values AS uav
JOIN user_attribute_definitions AS uad ON uad.id = uav.attribute_id
WHERE uav.user_id = $1
  AND uad.key = 'wechat'
  AND uad.deleted_at IS NULL
`, userID).Scan(&attributeValue))
	require.Equal(t, "legacy-wechat", attributeValue)

	var tierID string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT credentials->>'tier_id'
FROM accounts
WHERE id = $1
`, accountID).Scan(&tierID))
	require.Equal(t, "LEGACY", tierID)

	var opsTableExists bool
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT to_regclass('public.ops_alert_silences') IS NOT NULL
`).Scan(&opsTableExists))
	require.True(t, opsTableExists)

	var opsIndexExists bool
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT to_regclass('public.idx_ops_alert_silences_lookup') IS NOT NULL
`).Scan(&opsIndexExists))
	require.True(t, opsIndexExists)
}

func TestMigration142_RestoresLegacyColumnOnFreshForwardSchema(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	compensationSQL := migration142SQL(t)

	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, wechat)
VALUES ('u1-migration-fresh@example.invalid', 'not-a-real-password-hash', '')
RETURNING id
`).Scan(&userID))

	var attributeID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT id
FROM user_attribute_definitions
WHERE key = 'wechat' AND deleted_at IS NULL
ORDER BY id
LIMIT 1
`).Scan(&attributeID))

	_, err := tx.ExecContext(ctx, `
INSERT INTO user_attribute_values (user_id, attribute_id, value)
VALUES ($1, $2, 'attribute-only-wechat')
`, userID, attributeID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `ALTER TABLE users DROP COLUMN wechat`)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, compensationSQL)
	require.NoError(t, err)

	var legacyValue string
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT wechat FROM users WHERE id = $1", userID).Scan(&legacyValue))
	require.Equal(t, "attribute-only-wechat", legacyValue)
}

func TestMigration142_RejectsConflictingWechatValuesAtomically(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	compensationSQL := migration142SQL(t)

	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO users (email, password_hash, wechat)
VALUES ('u1-migration-conflict@example.invalid', 'not-a-real-password-hash', 'legacy-value')
RETURNING id
`).Scan(&userID))

	var attributeID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT id
FROM user_attribute_definitions
WHERE key = 'wechat' AND deleted_at IS NULL
ORDER BY id
LIMIT 1
`).Scan(&attributeID))

	_, err := tx.ExecContext(ctx, `
INSERT INTO user_attribute_values (user_id, attribute_id, value)
VALUES ($1, $2, 'different-attribute-value')
`, userID, attributeID)
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, compensationSQL)
	require.Error(t, err)
	require.Contains(t, err.Error(), "conflicting non-empty WeChat values")
}

func migration142SQL(t *testing.T) string {
	t.Helper()
	return embeddedMigrationSQL(t, "142_repair_legacy_goose_migrations.sql")
}

func embeddedMigrationSQL(t *testing.T, name string) string {
	t.Helper()
	content, err := fs.ReadFile(embeddedmigrations.FS, name)
	require.NoError(t, err)
	return string(content)
}
