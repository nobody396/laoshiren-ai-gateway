//go:build integration

package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	_ "github.com/bozhouDev/DragonCode-sub2api/ent/runtime"
	"github.com/bozhouDev/DragonCode-sub2api/internal/repository"
	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service/rbacfixture"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// 环境变量约定: 与 rbacfixture.seed_integration_test.go 一致.
// 若 TEST_DATABASE_URL 未设置, 所有 RBAC handler 集成测试都会被跳过.
const envRBACIntegrationDSN = "TEST_DATABASE_URL"

// integrationCtx 是所有集成用例共享的 ctx (取消需手动; 测试进程结束即自动回收).
var integrationCtx = context.Background()

// rbacIntegrationState 汇聚所有 integration 测试共享的句柄.
// 通过 TestMain 初始化一次, 后续所有测试通过 getIntegrationState(t) 访问.
type rbacIntegrationState struct {
	ready   bool
	initErr error

	db        *sql.DB
	entClient *dbent.Client
	repo      service.RBACRepository
	cache     *inMemoryRBACCache
	svc       *service.RBACService

	uniqueSeq uint64
}

var (
	rbacState rbacIntegrationState
	initOnce  sync.Once
)

// TestMain 仅在 //go:build integration 编译下生效, 对普通 unit 测试无影响.
// 策略: lazily 初始化连接, 失败或未配置 DSN 都允许 m.Run() 正常继续,
// 具体用例通过 getIntegrationState(t).skipIfUnavailable(t) 自行 Skip.
func TestMain(m *testing.M) {
	initIntegration()
	code := m.Run()
	teardownIntegration()
	os.Exit(code)
}

func initIntegration() {
	initOnce.Do(func() {
		dsn := os.Getenv(envRBACIntegrationDSN)
		if dsn == "" {
			log.Printf("[rbac-integration] %s 未设置, RBAC handler integration tests 将被 Skip", envRBACIntegrationDSN)
			return
		}

		db, err := sql.Open("postgres", dsn)
		if err != nil {
			rbacState.initErr = fmt.Errorf("sql.Open: %w", err)
			return
		}
		db.SetMaxOpenConns(8)
		db.SetMaxIdleConns(2)

		pingCtx, cancel := context.WithTimeout(integrationCtx, 5*time.Second)
		defer cancel()
		if err := db.PingContext(pingCtx); err != nil {
			rbacState.initErr = fmt.Errorf("ping db: %w", err)
			_ = db.Close()
			return
		}

		// 校验必要的 schema 已存在 (要求 TEST_DATABASE_URL 已跑过 migrations).
		// 不再在此处调用 ApplyMigrations: 历史上 migration 116 内嵌 BEGIN/COMMIT,
		// 与 applyMigrationsFS 的外层事务不兼容, 只能依赖预先已 migrate 的库.
		if err := ensureRBACSchemaReady(integrationCtx, db); err != nil {
			rbacState.initErr = err
			_ = db.Close()
			return
		}

		// 灌入 fixture 菜单 + API (UPSERT 幂等, 同时重置两表主键序列).
		if err := rbacfixture.SeedAllAndResetSequence(integrationCtx, db); err != nil {
			rbacState.initErr = fmt.Errorf("seed rbac fixture: %w", err)
			_ = db.Close()
			return
		}

		drv := entsql.OpenDB(dialect.Postgres, db)
		entClient := dbent.NewClient(dbent.Driver(drv))

		repo := repository.NewRBACRepository(entClient)
		cache := newInMemoryRBACCache()
		svc := service.NewRBACService(repo, cache)

		rbacState.db = db
		rbacState.entClient = entClient
		rbacState.repo = repo
		rbacState.cache = cache
		rbacState.svc = svc
		rbacState.ready = true
	})
}

func teardownIntegration() {
	if rbacState.entClient != nil {
		_ = rbacState.entClient.Close()
	}
	if rbacState.db != nil {
		_ = rbacState.db.Close()
	}
}

// 不再在此处调用 ApplyMigrations: 历史上 migration 116 内嵌 BEGIN/COMMIT,
// 与 applyMigrationsFS 的外层事务不兼容, 要求 TEST_DATABASE_URL 指向的库已 migrate 完成.
// 拆分后要求 admin_menus/admin_apis/admin_role_menus/admin_role_apis 均已存在.
func ensureRBACSchemaReady(ctx context.Context, db *sql.DB) error {
	required := []string{
		"users",
		"admin_roles",
		"admin_menus",
		"admin_apis",
		"admin_role_menus",
		"admin_role_apis",
		"admin_user_roles",
	}
	for _, name := range required {
		var exists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check table %s: %w", name, err)
		}
		if !exists {
			return fmt.Errorf("required table %q missing; 请先对 TEST_DATABASE_URL 指向的库执行 backend/migrations/*.sql", name)
		}
	}
	return nil
}

// getIntegrationState 返回初始化后的共享状态; 若未配置 DSN 或初始化失败, Skip 当前测试.
func getIntegrationState(t *testing.T) *rbacIntegrationState {
	t.Helper()
	if rbacState.initErr != nil {
		t.Fatalf("RBAC integration init error: %v", rbacState.initErr)
	}
	if !rbacState.ready {
		t.Skipf("env %s 未设置 / RBAC integration 未就绪, 跳过 (需指向一个可写 Postgres)", envRBACIntegrationDSN)
	}
	return &rbacState
}

// nextSeq 返回唯一递增序列号, 用于防止并发测试的 email / role name 冲突.
func (s *rbacIntegrationState) nextSeq() uint64 {
	return atomic.AddUint64(&s.uniqueSeq, 1)
}

// ---------------------------------------------------------------------------
// in-memory RBACCache 实现: 避免依赖真实 Redis
// ---------------------------------------------------------------------------

type inMemoryRBACCache struct {
	mu    sync.Mutex
	perms map[int64][]string // nil: miss, []: empty hit, non-empty: hit
	menu  map[int64][]byte
}

func newInMemoryRBACCache() *inMemoryRBACCache {
	return &inMemoryRBACCache{
		perms: make(map[int64][]string),
		menu:  make(map[int64][]byte),
	}
}

func (c *inMemoryRBACCache) GetUserPermissionKeys(_ context.Context, userID int64) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.perms[userID]; ok {
		return v, nil
	}
	return nil, nil
}

func (c *inMemoryRBACCache) SetUserPermissionKeys(_ context.Context, userID int64, keys []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perms[userID] = keys
	return nil
}

func (c *inMemoryRBACCache) InvalidateUserPermissions(_ context.Context, userID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.perms, userID)
	delete(c.menu, userID)
	return nil
}

func (c *inMemoryRBACCache) InvalidateAllPermissions(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perms = make(map[int64][]string)
	c.menu = make(map[int64][]byte)
	return nil
}

func (c *inMemoryRBACCache) GetUserMenuTree(_ context.Context, userID int64) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.menu[userID]; ok {
		return v, nil
	}
	return nil, nil
}

func (c *inMemoryRBACCache) SetUserMenuTree(_ context.Context, userID int64, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.menu[userID] = data
	return nil
}

// ---------------------------------------------------------------------------
// Gin router & auth helpers
// ---------------------------------------------------------------------------

// buildRBACRouter 构造一个最小 gin 引擎, 注册 RBAC handler 全部 15 个路由.
// authSubjectUserID 为 0 时不注入 AuthSubject, 用于模拟未认证场景.
func (s *rbacIntegrationState) buildRBACRouter(authSubjectUserID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 测试专用鉴权中间件: 若 authSubjectUserID > 0 则注入 AuthSubject.
	router.Use(func(c *gin.Context) {
		if authSubjectUserID > 0 {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{
				UserID:      authSubjectUserID,
				Concurrency: 1,
			})
		}
		c.Next()
	})

	h := NewRBACHandler(s.svc)
	rbac := router.Group("/admin/rbac")
	{
		rbac.GET("/menu", h.GetCurrentUserMenu)
		rbac.GET("/me/permissions", h.GetCurrentUserPermissions)

		// 菜单管理
		rbac.GET("/menus", h.ListMenus)
		rbac.GET("/menus/tree", h.GetMenuTree)
		rbac.POST("/menus", h.CreateMenu)
		rbac.PUT("/menus/:id", h.UpdateMenu)
		rbac.DELETE("/menus/:id", h.DeleteMenu)

		// API 管理
		rbac.GET("/apis", h.ListAPIs)
		rbac.GET("/apis/all", h.ListAllAPIs)
		rbac.GET("/apis/groups", h.GetAPIGroups)
		rbac.POST("/apis", h.CreateAPI)
		rbac.PUT("/apis/:id", h.UpdateAPI)
		rbac.DELETE("/apis/:id", h.DeleteAPI)
		rbac.DELETE("/apis", h.DeleteAPIs)
		rbac.POST("/apis/sync", h.SyncAPIs)

		// 角色管理
		rbac.GET("/roles", h.ListRoles)
		rbac.POST("/roles", h.CreateRole)
		rbac.PUT("/roles/:id", h.UpdateRole)
		rbac.DELETE("/roles/:id", h.DeleteRole)

		// 角色-菜单 / 角色-API
		rbac.GET("/roles/:id/menus", h.GetRoleMenus)
		rbac.PUT("/roles/:id/menus", h.SetRoleMenus)
		rbac.GET("/roles/:id/apis", h.GetRoleAPIs)
		rbac.PUT("/roles/:id/apis", h.SetRoleAPIs)

		rbac.GET("/users/:id/roles", h.GetUserRoles)
		rbac.PUT("/users/:id/roles", h.SetUserRoles)
	}
	return router
}

// ---------------------------------------------------------------------------
// Test data fixtures
// ---------------------------------------------------------------------------

// createTestUser 创建一个临时 User, 返回其 ID.
// email 带时间戳/递增序列避免重复跑测试导致的唯一冲突.
func (s *rbacIntegrationState) createTestUser(t *testing.T, role string) int64 {
	t.Helper()
	seq := s.nextSeq()
	email := fmt.Sprintf("rbac-it-%d-%d@example.test", time.Now().UnixNano(), seq)
	u, err := s.entClient.User.Create().
		SetEmail(email).
		SetPasswordHash("x"). // 仅占位, 不参与登录
		SetRole(role).
		SetStatus("active").
		Save(integrationCtx)
	require.NoError(t, err, "create test user")
	return u.ID
}

// createTestRole 通过 repository 创建角色, 返回 ID.
// name 内部会自动附加序列号以避免唯一冲突.
func (s *rbacIntegrationState) createTestRole(t *testing.T, name string, isSuperAdmin bool) int64 {
	t.Helper()
	seq := s.nextSeq()
	r := &service.AdminRole{
		Name:         fmt.Sprintf("%s-%d-%d", name, time.Now().UnixNano(), seq),
		Description:  "rbac integration test role",
		IsSuperAdmin: isSuperAdmin,
		Status:       service.ResourceStatusActive,
	}
	require.NoError(t, s.repo.CreateRole(integrationCtx, r), "create role")
	return r.ID
}

// assignUserRoles 将用户关联到角色集合, 并清除该用户的缓存项以保证立即生效.
func (s *rbacIntegrationState) assignUserRoles(t *testing.T, userID int64, roleIDs []int64) {
	t.Helper()
	require.NoError(t, s.repo.AssignUserRoles(integrationCtx, userID, roleIDs))
	_ = s.cache.InvalidateUserPermissions(integrationCtx, userID)
}

// findMenuIDByKey 根据 permission_key 在 admin_menus 里查 ID (便于分配菜单用例).
func (s *rbacIntegrationState) findMenuIDByKey(t *testing.T, key string) int64 {
	t.Helper()
	menus, err := s.repo.ListMenus(integrationCtx)
	require.NoError(t, err, "list menus")
	for _, m := range menus {
		if m.PermissionKey == key {
			return m.ID
		}
	}
	t.Fatalf("menu permission_key %q not found", key)
	return 0
}

// findAPIIDByMethodPath 根据 method+path 在 admin_apis 里查 ID.
func (s *rbacIntegrationState) findAPIIDByMethodPath(t *testing.T, method, path string) int64 {
	t.Helper()
	apis, err := s.repo.ListAllAPIs(integrationCtx)
	require.NoError(t, err, "list all apis")
	for _, a := range apis {
		if a.Method == method && a.Path == path {
			return a.ID
		}
	}
	t.Fatalf("api %s %s not found", method, path)
	return 0
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// doRequest 发起一次 HTTP 调用并返回 recorder; body 为 nil 时不设置 Content-Type.
func doRequest(t *testing.T, router *gin.Engine, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// mustMarshal 快捷 JSON 编码, 失败 t.Fatal.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// decodeEnvelope 解析标准响应体到 code/message + 指定 data 类型.
// 返回 (code, message). dataOut 可为 nil 表示不需要 data.
func decodeEnvelope(t *testing.T, body []byte, dataOut any) (int, string) {
	t.Helper()
	var raw struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &raw), "decode envelope: %s", string(body))
	if dataOut != nil && len(raw.Data) > 0 && string(raw.Data) != "null" {
		require.NoError(t, json.Unmarshal(raw.Data, dataOut), "decode data: %s", string(raw.Data))
	}
	return raw.Code, raw.Message
}
