package rbacfixture

import (
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

// TestMenuFixtureIntegrity 校验 BaselineMenus 的合法性、唯一性与引用完整性.
func TestMenuFixtureIntegrity(t *testing.T) {
	menus := BaselineMenus()
	if len(menus) == 0 {
		t.Fatal("menu fixture 为空")
	}

	validTypes := map[string]struct{}{
		service.MenuTypeMenu: {},
	}

	ids := make(map[int64]struct{}, len(menus))
	keys := make(map[string]struct{}, len(menus))

	for _, m := range menus {
		if m == nil {
			t.Fatal("fixture 存在 nil 菜单")
		}
		if m.ID <= 0 {
			t.Errorf("菜单 id 非法: %+v", m)
		}
		if _, dup := ids[m.ID]; dup {
			t.Errorf("菜单 id 重复: %d", m.ID)
		}
		ids[m.ID] = struct{}{}

		if m.PermissionKey == "" {
			t.Errorf("permission_key 为空: id=%d", m.ID)
		}
		if _, dup := keys[m.PermissionKey]; dup {
			t.Errorf("permission_key 重复: %s", m.PermissionKey)
		}
		keys[m.PermissionKey] = struct{}{}

		if _, ok := validTypes[m.Type]; !ok {
			t.Errorf("type 非法: id=%d, type=%s", m.ID, m.Type)
		}
		if m.Status != service.ResourceStatusActive &&
			m.Status != service.ResourceStatusInactive {
			t.Errorf("status 非法: id=%d, status=%s", m.ID, m.Status)
		}
		if m.Name == "" {
			t.Errorf("name 为空: id=%d", m.ID)
		}
	}

	// 20260505 扁平化后不再有 parent_id 字段, 无需外键悬挂校验.

	// baseline 条数至少包含当前后台侧边栏的 20 条菜单
	if len(menus) < 20 {
		t.Errorf("baseline 菜单过少: got=%d want>=20", len(menus))
	}

	t.Logf("menu fixture OK: total=%d", len(menus))
}

// TestAPIFixtureIntegrity 校验 BaselineAPIs 的合法性与唯一性.
func TestAPIFixtureIntegrity(t *testing.T) {
	apis := BaselineAPIs()
	if len(apis) == 0 {
		t.Fatal("api fixture 为空")
	}

	validMethods := map[string]struct{}{
		"GET": {}, "POST": {}, "PUT": {}, "DELETE": {}, "PATCH": {},
	}

	ids := make(map[int64]struct{}, len(apis))
	mp := make(map[string]struct{}, len(apis)) // method+path 唯一性

	for _, a := range apis {
		if a == nil {
			t.Fatal("fixture 存在 nil API")
		}
		if a.ID <= 0 {
			t.Errorf("api id 非法: %+v", a)
		}
		if _, dup := ids[a.ID]; dup {
			t.Errorf("api id 重复: %d", a.ID)
		}
		ids[a.ID] = struct{}{}

		if _, ok := validMethods[a.Method]; !ok {
			t.Errorf("method 非法: id=%d, method=%s", a.ID, a.Method)
		}
		if a.Path == "" || !strings.HasPrefix(a.Path, "/admin/") {
			t.Errorf("path 非法: id=%d, path=%s", a.ID, a.Path)
		}
		key := a.Method + " " + a.Path
		if _, dup := mp[key]; dup {
			t.Errorf("method+path 重复: %s", key)
		}
		mp[key] = struct{}{}

		if a.Status != service.ResourceStatusActive &&
			a.Status != service.ResourceStatusInactive {
			t.Errorf("status 非法: id=%d, status=%s", a.ID, a.Status)
		}
	}

	if len(apis) < 300 {
		t.Errorf("api 条数过少: got=%d want>=300", len(apis))
	}

	t.Logf("api fixture OK: total=%d", len(apis))
}

// TestFullFixtureCoverage 抽查 BaselineMenus + BaselineAPIs 的关键条目都已覆盖.
func TestFullFixtureCoverage(t *testing.T) {
	menus, apis := FullFixture()

	menuKeys := make(map[string]struct{}, len(menus))
	for _, m := range menus {
		menuKeys[m.PermissionKey] = struct{}{}
	}
	mustMenuKeys := []string{
		"admin:dashboard",
		"admin:users",
		"admin:roles",
		"admin:menus",
		"admin:apis",
	}
	for _, k := range mustMenuKeys {
		if _, ok := menuKeys[k]; !ok {
			t.Errorf("缺少关键菜单 permission_key: %s", k)
		}
	}

	apiKeys := make(map[string]struct{}, len(apis))
	for _, a := range apis {
		apiKeys[a.Method+" "+a.Path] = struct{}{}
	}
	mustAPIs := []string{
		"GET /admin/users",
		"POST /admin/users",
		"PUT /admin/users/:id",
		"DELETE /admin/users/:id",
		"POST /admin/users/:id/balance",
		"GET /admin/rbac/menu",
		"GET /admin/rbac/me/permissions",
		"PUT /admin/rbac/users/:id/roles",
		"GET /admin/ops/concurrency",
		"POST /admin/accounts/generate-auth-url",
		"GET /admin/channels",
		"PUT /admin/api-keys/:id",
		"GET /admin/invoice/requests",
		"POST /admin/backups/:id/restore",
		"POST /admin/system/restart",
	}
	for _, k := range mustAPIs {
		if _, ok := apiKeys[k]; !ok {
			t.Errorf("缺少关键 API: %s", k)
		}
	}
}
