//go:build integration

package rbacfixture

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// envTestDSN 指向一个已经跑过 migrations (含 116 + 118 + 119 + 120) 的 Postgres.
// 未设置时测试被跳过, 便于本地默认 go test 不依赖环境.
const envTestDSN = "TEST_DATABASE_URL"

// openTestDB 从环境变量读 DSN 并建立连接.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv(envTestDSN)
	if dsn == "" {
		t.Skipf("env %s 未设置, 跳过 (需要一个已 migrate 的 Postgres)", envTestDSN)
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err, "sql.Open")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(ctx), "ping db failed, check %s", envTestDSN)

	// 验证必要的表结构存在 (拆分后应同时存在 admin_menus / admin_apis).
	for _, tbl := range []string{"admin_menus", "admin_apis"} {
		var exists bool
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT EXISTS (
			  SELECT 1 FROM information_schema.tables
			  WHERE table_name = $1
			)
		`, tbl).Scan(&exists))
		require.Truef(t, exists,
			"%s 表不存在, 请先执行 migrations (含 118_split_rbac_menus_apis.sql)", tbl)
	}

	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestSeedRBACFixture 在真实 Postgres 里验证 fixture 导入:
//  1. UPSERT 成功, 无 SQL / 约束错误
//  2. admin_menus 与 admin_apis 两张表都被幂等写入
//  3. 幂等性: 第二次 Seed 数量不变, 内容被更新
//
// 全程在事务内进行并最终回滚, 不污染数据库.
func TestSeedRBACFixture(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err, "begin tx")
	t.Cleanup(func() { _ = tx.Rollback() })

	// --- 首次导入 ---
	require.NoError(t, SeedAll(ctx, tx), "首次 SeedAll 应成功")

	var menuCountAfterFirst, apiCountAfterFirst int
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_menus`).Scan(&menuCountAfterFirst))
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_apis`).Scan(&apiCountAfterFirst))

	require.GreaterOrEqual(t, menuCountAfterFirst, 20,
		"admin_menus 至少应有当前后台侧边栏的 20 条")
	require.GreaterOrEqual(t, apiCountAfterFirst, 300,
		"admin_apis 至少应有 fixture 灌入的 300+ 条")

	// --- 抽查几条关键菜单 ---
	mustMenus := []struct {
		key      string
		wantType string
	}{
		{"admin:users", "menu"},
		{"admin:roles", "menu"},
		{"admin:menus", "menu"},
		{"admin:apis", "menu"},
	}
	for _, tc := range mustMenus {
		var gotType string
		err := tx.QueryRowContext(ctx,
			`SELECT type FROM admin_menus WHERE permission_key = $1`, tc.key,
		).Scan(&gotType)
		require.NoErrorf(t, err, "找不到菜单 key=%s", tc.key)
		require.Equalf(t, tc.wantType, gotType, "type 不匹配 key=%s", tc.key)
	}

	// --- 抽查几条关键 API ---
	mustAPIs := []struct {
		method string
		path   string
	}{
		{"GET", "/admin/users"},
		{"DELETE", "/admin/users/:id"},
		{"POST", "/admin/accounts/generate-auth-url"},
		{"GET", "/admin/rbac/menu"},
	}
	for _, tc := range mustAPIs {
		var exists bool
		require.NoError(t, tx.QueryRowContext(ctx, `
			SELECT EXISTS (
			  SELECT 1 FROM admin_apis WHERE method=$1 AND path=$2
			)
		`, tc.method, tc.path).Scan(&exists))
		require.Truef(t, exists, "API %s %s 应存在", tc.method, tc.path)
	}

	// --- 二次导入验证幂等 ---
	require.NoError(t, SeedAll(ctx, tx), "二次 SeedAll 应幂等")

	var menuCountAfterSecond, apiCountAfterSecond int
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_menus`).Scan(&menuCountAfterSecond))
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM admin_apis`).Scan(&apiCountAfterSecond))
	require.Equal(t, menuCountAfterFirst, menuCountAfterSecond,
		"幂等导入后菜单总数不应变化")
	require.Equal(t, apiCountAfterFirst, apiCountAfterSecond,
		"幂等导入后 API 总数不应变化")

	// --- 校验菜单类型: 扁平化后所有记录 type 均为 'menu' ---
	var nonMenuCount int
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM admin_menus WHERE type <> 'menu'
	`).Scan(&nonMenuCount))
	require.Equal(t, 0, nonMenuCount, "扁平化后不应存在 type != 'menu' 的菜单")

	t.Logf("Seed OK: menus=%d, apis=%d",
		menuCountAfterSecond, apiCountAfterSecond)
}

// TestSeedRBACFixtureAndResetSequence 验证序列重置不会报错,
// 并确保重置后新插入的自增 id 不会撞到 fixture 已占用的区间.
func TestSeedRBACFixtureAndResetSequence(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err, "begin tx")
	t.Cleanup(func() { _ = tx.Rollback() })

	require.NoError(t, SeedAllAndResetSequence(ctx, tx))

	// --- 菜单序列校验 ---
	var maxMenuID int64
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT MAX(id) FROM admin_menus`).Scan(&maxMenuID))

	var newMenuID int64
	placeholderKey := fmt.Sprintf("test:seq:menu:%d", time.Now().UnixNano())
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO admin_menus
		    (name, type, permission_key, sort_order, status)
		VALUES ('seq 验证占位', 'menu', $1, 999, 'active')
		RETURNING id
	`, placeholderKey).Scan(&newMenuID))
	require.Greaterf(t, newMenuID, maxMenuID,
		"自增 menu id (%d) 应大于 fixture 最大 id (%d)", newMenuID, maxMenuID)

	// --- API 序列校验 ---
	var maxAPIID int64
	require.NoError(t, tx.QueryRowContext(ctx,
		`SELECT MAX(id) FROM admin_apis`).Scan(&maxAPIID))

	var newAPIID int64
	placeholderPath := fmt.Sprintf("/admin/seq-test/%d", time.Now().UnixNano())
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO admin_apis
		    ("group", path, method, description, sort_order, status)
		VALUES ('', $1, 'GET', 'seq 验证占位', 999, 'active')
		RETURNING id
	`, placeholderPath).Scan(&newAPIID))
	require.Greaterf(t, newAPIID, maxAPIID,
		"自增 api id (%d) 应大于 fixture 最大 id (%d)", newAPIID, maxAPIID)
}
