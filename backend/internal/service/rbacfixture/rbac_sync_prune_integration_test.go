//go:build integration

// Package rbacfixture_test 承载 RBAC 相关的 integration 测试.
// 本文件专注于路由扫描器 SyncAPIsWithOptions 的 prune 分支端到端行为,
// 放在 rbacfixture 子包 (external test package) 是为了避开 service 包内
// unit-tag 测试文件的历史遗留编译问题 (build-tag 不一致 / 符号重声明等),
// 确保本测试能独立用 -tags=integration 运行.
package rbacfixture_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	_ "github.com/bozhouDev/DragonCode-sub2api/ent/runtime"
	"github.com/bozhouDev/DragonCode-sub2api/internal/repository"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// envPruneIntegrationDSN 指向一个已完成 migrations (含 116 + 118) 的 Postgres.
// 不做全量 seed, 只在现有库里创建局部孤儿 + 角色, 测试结束自清理.
const envPruneIntegrationDSN = "TEST_DATABASE_URL"

// ---------------------------------------------------------------------------
// harness: 独立 DB 连接 + RBACService, 绕开 SeedAll 避免与活库冲突
// ---------------------------------------------------------------------------

type pruneHarness struct {
	db        *sql.DB
	entClient *dbent.Client
	repo      service.RBACRepository
	svc       *service.RBACService
}

func openPruneHarness(t *testing.T) *pruneHarness {
	t.Helper()
	dsn := os.Getenv(envPruneIntegrationDSN)
	if dsn == "" {
		t.Skipf("env %s 未设置, 跳过 (需指向一个已 migrate 的 Postgres)", envPruneIntegrationDSN)
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err, "sql.Open")

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(pingCtx), "ping db")

	for _, tbl := range []string{"admin_apis", "admin_role_apis", "admin_roles"} {
		var exists bool
		require.NoError(t, db.QueryRowContext(pingCtx, `
			SELECT EXISTS (
			  SELECT 1 FROM information_schema.tables
			  WHERE table_schema = 'public' AND table_name = $1
			)
		`, tbl).Scan(&exists))
		require.Truef(t, exists, "required table %q missing; 请先执行 backend/migrations/*.sql", tbl)
	}

	drv := entsql.OpenDB(dialect.Postgres, db)
	entClient := dbent.NewClient(dbent.Driver(drv))
	repo := repository.NewRBACRepository(entClient)
	cache := newPruneInMemoryCache()
	svc := service.NewRBACService(repo, cache)

	t.Cleanup(func() {
		_ = entClient.Close()
		_ = db.Close()
	})
	return &pruneHarness{db: db, entClient: entClient, repo: repo, svc: svc}
}

// ---------------------------------------------------------------------------
// 简易 in-memory RBACCache, 避免依赖 Redis
// ---------------------------------------------------------------------------

type pruneInMemoryCache struct {
	mu    sync.Mutex
	perms map[int64][]string
	menu  map[int64][]byte
}

func newPruneInMemoryCache() *pruneInMemoryCache {
	return &pruneInMemoryCache{
		perms: make(map[int64][]string),
		menu:  make(map[int64][]byte),
	}
}

func (c *pruneInMemoryCache) GetUserPermissionKeys(_ context.Context, userID int64) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.perms[userID]; ok {
		return v, nil
	}
	return nil, nil
}

func (c *pruneInMemoryCache) SetUserPermissionKeys(_ context.Context, userID int64, keys []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perms[userID] = keys
	return nil
}

func (c *pruneInMemoryCache) InvalidateUserPermissions(_ context.Context, userID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.perms, userID)
	delete(c.menu, userID)
	return nil
}

func (c *pruneInMemoryCache) InvalidateAllPermissions(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perms = make(map[int64][]string)
	c.menu = make(map[int64][]byte)
	return nil
}

func (c *pruneInMemoryCache) GetUserMenuTree(_ context.Context, userID int64) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.menu[userID]; ok {
		return v, nil
	}
	return nil, nil
}

func (c *pruneInMemoryCache) SetUserMenuTree(_ context.Context, userID int64, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.menu[userID] = data
	return nil
}

// ---------------------------------------------------------------------------
// TestSyncAPIsWithOptions_PruneOrphan
// ---------------------------------------------------------------------------

// TestSyncAPIsWithOptions_PruneOrphan 验证路由扫描器 prune 分支端到端行为:
//  1. 默认 Prune=false 不删孤儿
//  2. Prune=true 精确删除 DB 中存在但 items 未出现的记录, 返回 PrunedIDs
//  3. admin_role_apis 通过外键 ON DELETE CASCADE 随之自动清理
//  4. 幂等重放: 孤儿已清理后第二次 Pruned=0
//
// 用例主动创建一条"扫描器永远不会生成"的孤儿 API 并将其绑定到新角色, 在
// prune=true 之后断言 repo.GetAPI/GetRoleAPIIDs 均查不到该孤儿.
func TestSyncAPIsWithOptions_PruneOrphan(t *testing.T) {
	h := openPruneHarness(t)
	ctx := context.Background()

	// --- 1. 创建孤儿 API (path 带时间戳+pid, 防止并发/重跑冲突) ---
	orphanPath := fmt.Sprintf("/admin/prune-orphan-%d-%d", time.Now().UnixNano(), os.Getpid())
	orphan := &service.AdminAPI{
		Group:       "test-prune",
		Path:        orphanPath,
		Method:      "GET",
		Description: "prune integration orphan",
		Status:      service.ResourceStatusActive,
	}
	require.NoError(t, h.repo.CreateAPI(ctx, orphan), "create orphan api")
	require.NotZero(t, orphan.ID)

	// 兜底清理: 测试中途失败时不要把孤儿遗留在 DB 里.
	orphanRemoved := false
	t.Cleanup(func() {
		if !orphanRemoved {
			_ = h.repo.DeleteAPI(ctx, orphan.ID)
		}
	})

	// --- 2. 创建角色并绑定孤儿, 用于验证 CASCADE ---
	role := &service.AdminRole{
		Name:        fmt.Sprintf("prune-role-%d", time.Now().UnixNano()),
		Description: "prune integration test role",
		Status:      service.ResourceStatusActive,
	}
	require.NoError(t, h.repo.CreateRole(ctx, role))
	require.NotZero(t, role.ID)
	t.Cleanup(func() { _ = h.repo.DeleteRole(ctx, role.ID) })

	require.NoError(t,
		h.repo.AssignRoleAPIs(ctx, role.ID, []int64{orphan.ID}),
		"assign orphan to role")

	bound, err := h.repo.GetRoleAPIIDs(ctx, role.ID)
	require.NoError(t, err)
	require.Contains(t, bound, orphan.ID, "绑定后 admin_role_apis 应含孤儿 ID")

	// --- 3. 构造 items: DB 当前全量剔除孤儿, 模拟真实路由扫描 ---
	all, err := h.repo.ListAllAPIs(ctx)
	require.NoError(t, err)
	items := make([]*service.AdminAPI, 0, len(all))
	for _, a := range all {
		if a.ID == orphan.ID {
			continue
		}
		items = append(items, &service.AdminAPI{
			Group:       a.Group,
			Path:        a.Path,
			Method:      a.Method,
			Description: a.Description,
			Status:      a.Status,
		})
	}
	require.NotEmpty(t, items, "items 应至少包含运行库中已存在的 API")

	// --- 4. Prune=false: 不删 ---
	resNoPrune, err := h.svc.SyncAPIsWithOptions(ctx, items, service.SyncAPIsOptions{Prune: false})
	require.NoError(t, err)
	require.Equal(t, len(items), resNoPrune.Synced)
	require.Equal(t, 0, resNoPrune.Pruned, "Prune=false 时 pruned 必须为 0")
	require.Nil(t, resNoPrune.PrunedIDs)

	still, err := h.repo.GetAPI(ctx, orphan.ID)
	require.NoError(t, err)
	require.NotNil(t, still, "Prune=false 不应删除孤儿")

	// --- 5. Prune=true: 精确删除孤儿 ---
	resPrune, err := h.svc.SyncAPIsWithOptions(ctx, items, service.SyncAPIsOptions{Prune: true})
	require.NoError(t, err)
	require.Equal(t, len(items), resPrune.Synced)
	require.GreaterOrEqual(t, resPrune.Pruned, 1, "至少应 prune 掉本测试创建的孤儿")
	require.Contains(t, resPrune.PrunedIDs, orphan.ID, "PrunedIDs 必须包含孤儿 ID")
	orphanRemoved = true

	gone, err := h.repo.GetAPI(ctx, orphan.ID)
	require.NoError(t, err)
	require.Nil(t, gone, "Prune=true 后孤儿应被删除")

	boundAfter, err := h.repo.GetRoleAPIIDs(ctx, role.ID)
	require.NoError(t, err)
	require.NotContains(t, boundAfter, orphan.ID,
		"admin_role_apis 应已由 ON DELETE CASCADE 自动清理")

	// --- 6. 幂等: 再次 prune=true, Pruned=0 ---
	resAgain, err := h.svc.SyncAPIsWithOptions(ctx, items, service.SyncAPIsOptions{Prune: true})
	require.NoError(t, err)
	require.Equal(t, 0, resAgain.Pruned, "孤儿已清理, 再次 prune 应返回 0")
	require.Empty(t, resAgain.PrunedIDs)
}

// TestSyncAPIsWithOptions_EmptyItemsProtection 验证保护性短路:
// items 为空时即使 Prune=true 也不动表, 防止扫描器异常 (如路由注册失败) 误删整个 admin_apis.
func TestSyncAPIsWithOptions_EmptyItemsProtection(t *testing.T) {
	h := openPruneHarness(t)
	ctx := context.Background()

	before, err := h.repo.ListAllAPIs(ctx)
	require.NoError(t, err)

	res, err := h.svc.SyncAPIsWithOptions(ctx, nil, service.SyncAPIsOptions{Prune: true})
	require.NoError(t, err)
	require.Equal(t, 0, res.Synced)
	require.Equal(t, 0, res.Pruned)
	require.Nil(t, res.PrunedIDs)

	after, err := h.repo.ListAllAPIs(ctx)
	require.NoError(t, err)
	require.Equal(t, len(before), len(after),
		"items 为空 + Prune=true 时不应删除任何 API")
}
