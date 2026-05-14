//go:build integration

package admin

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 1. 菜单 & 当前用户权限
// ---------------------------------------------------------------------------

func TestRBACIntegration_GetCurrentUserMenu_SuperAdmin(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "super-menu", true)
	st.assignUserRoles(t, userID, []int64{roleID})

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/menu", nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	var tree []service.MenuTreeNode
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &tree)
	require.Zero(t, code)

	require.GreaterOrEqual(t, len(tree), 18, "super admin 应看到全部扁平菜单")

	names := make(map[string]bool, len(tree))
	for _, n := range tree {
		names[n.Name] = true
	}
	require.True(t, names["管理仪表盘"] || names["角色管理"], "tree 必须含至少一个已知扁平菜单")
}

func TestRBACIntegration_GetCurrentUserMenu_Unauthenticated(t *testing.T) {
	st := getIntegrationState(t)
	router := st.buildRBACRouter(0)

	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/menu", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRBACIntegration_GetCurrentUserPermissions_SuperAdmin(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "super-keys", true)
	st.assignUserRoles(t, userID, []int64{roleID})

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/me/permissions", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var keys []string
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &keys)
	require.Zero(t, code)
	require.Equal(t, []string{"*"}, keys, "super admin 的权限 key 集合应仅含通配符")
}

// ---------------------------------------------------------------------------
// 2. 菜单 CRUD
// ---------------------------------------------------------------------------

func TestRBACIntegration_ListMenus(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/menus", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var list []service.AdminMenu
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &list)
	require.Zero(t, code)
	require.GreaterOrEqual(t, len(list), 18, "应至少包含 fixture 的 18 条扁平菜单")
}

func TestRBACIntegration_GetMenuTree(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/menus/tree", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var tree []service.MenuTreeNode
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &tree)
	require.Zero(t, code)
	require.NotEmpty(t, tree)
}

func TestRBACIntegration_CreateMenu_Success(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{
		"name":           "IT 创建测试",
		"permission_key": fmt.Sprintf("it:create:%d", time.Now().UnixNano()),
		"sort_order":     99,
	})
	rec := doRequest(t, router, http.MethodPost, "/admin/rbac/menus", body)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	var m service.AdminMenu
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &m)
	require.Zero(t, code)
	require.NotZero(t, m.ID)
	require.Equal(t, "IT 创建测试", m.Name)
}

func TestRBACIntegration_CreateMenu_MissingRequired(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{
		"sort_order": 99, // 缺 name + permission_key
	})
	rec := doRequest(t, router, http.MethodPost, "/admin/rbac/menus", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRBACIntegration_UpdateMenu_Success(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	m := &service.AdminMenu{
		Name:          "to-update",
		Type:          service.MenuTypeMenu,
		PermissionKey: fmt.Sprintf("it:update:%d", time.Now().UnixNano()),
		Status:        service.ResourceStatusActive,
	}
	require.NoError(t, st.repo.CreateMenu(integrationCtx, m))

	router := st.buildRBACRouter(userID)
	newName := "已更新"
	body := mustMarshal(t, map[string]any{
		"name": newName,
	})
	rec := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/admin/rbac/menus/%d", m.ID), body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	// ent client 直查, 绕过 service 过滤
	persisted, err := st.entClient.AdminMenu.Get(integrationCtx, m.ID)
	require.NoError(t, err, "updated menu 应仍可查到")
	require.Equal(t, newName, persisted.Name)
}

func TestRBACIntegration_UpdateMenu_InvalidID(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{"name": "x"})
	rec := doRequest(t, router, http.MethodPut, "/admin/rbac/menus/not-a-number", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRBACIntegration_DeleteMenu_Success(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	m := &service.AdminMenu{
		Name:          "to-delete",
		Type:          service.MenuTypeMenu,
		PermissionKey: fmt.Sprintf("it:delete:%d", time.Now().UnixNano()),
		Status:        service.ResourceStatusActive,
	}
	require.NoError(t, st.repo.CreateMenu(integrationCtx, m))

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/admin/rbac/menus/%d", m.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
}

// ---------------------------------------------------------------------------
// 3. API CRUD
// ---------------------------------------------------------------------------

func TestRBACIntegration_ListAPIs(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/apis?page=1&page_size=20", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var page struct {
		List     []service.AdminAPI `json:"list"`
		Total    int                `json:"total"`
		Page     int                `json:"page"`
		PageSize int                `json:"page_size"`
	}
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &page)
	require.Zero(t, code)
	require.GreaterOrEqual(t, page.Total, 290, "应至少包含 fixture 的 290+ 条 API")
	require.LessOrEqual(t, len(page.List), 20)
}

func TestRBACIntegration_ListAllAPIs(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/apis/all", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var list []service.AdminAPI
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &list)
	require.Zero(t, code)
	require.GreaterOrEqual(t, len(list), 290)
}

func TestRBACIntegration_GetAPIGroups(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/apis/groups", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var groups []string
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &groups)
	require.Zero(t, code)
}

func TestRBACIntegration_CreateAPI_Success(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{
		"group":       "IT",
		"path":        fmt.Sprintf("/admin/it/create/%d", time.Now().UnixNano()),
		"method":      "GET",
		"description": "it-create-test",
	})
	rec := doRequest(t, router, http.MethodPost, "/admin/rbac/apis", body)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	var a service.AdminAPI
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &a)
	require.Zero(t, code)
	require.NotZero(t, a.ID)
}

func TestRBACIntegration_CreateAPI_MissingRequired(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{"group": "X"}) // 缺 path + method
	rec := doRequest(t, router, http.MethodPost, "/admin/rbac/apis", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRBACIntegration_UpdateAPI_Success(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	a := &service.AdminAPI{
		Path:   fmt.Sprintf("/admin/it/update/%d", time.Now().UnixNano()),
		Method: "GET",
		Group:  "IT",
		Status: service.ResourceStatusActive,
	}
	require.NoError(t, st.repo.CreateAPI(integrationCtx, a))

	router := st.buildRBACRouter(userID)
	newDesc := "已更新"
	body := mustMarshal(t, map[string]any{"description": newDesc})
	rec := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/admin/rbac/apis/%d", a.ID), body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	persisted, err := st.entClient.AdminAPI.Get(integrationCtx, a.ID)
	require.NoError(t, err)
	require.Equal(t, newDesc, persisted.Description)
}

func TestRBACIntegration_DeleteAPI_Success(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	a := &service.AdminAPI{
		Path:   fmt.Sprintf("/admin/it/delete/%d", time.Now().UnixNano()),
		Method: "GET",
		Status: service.ResourceStatusActive,
	}
	require.NoError(t, st.repo.CreateAPI(integrationCtx, a))

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/admin/rbac/apis/%d", a.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
}

// ---------------------------------------------------------------------------
// 4. 角色 CRUD
// ---------------------------------------------------------------------------

func TestRBACIntegration_ListRoles(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	_ = st.createTestRole(t, "role-in-list", false)

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/roles", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var list []service.AdminRole
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &list)
	require.Zero(t, code)
	require.NotEmpty(t, list)
}

func TestRBACIntegration_CreateRole_Success(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{
		"name":        fmt.Sprintf("role-create-%d", time.Now().UnixNano()),
		"description": "integration-test-role",
	})
	rec := doRequest(t, router, http.MethodPost, "/admin/rbac/roles", body)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	var r service.AdminRole
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &r)
	require.Zero(t, code)
	require.NotZero(t, r.ID)
}

func TestRBACIntegration_CreateRole_MissingName(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{"description": "no-name"})
	rec := doRequest(t, router, http.MethodPost, "/admin/rbac/roles", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRBACIntegration_UpdateRole_Success(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	seq := st.nextSeq()
	origName := fmt.Sprintf("role-to-update-%d-%d", time.Now().UnixNano(), seq)
	role := &service.AdminRole{
		Name:         origName,
		Description:  "pre-update",
		IsSuperAdmin: false,
		Status:       service.ResourceStatusActive,
	}
	require.NoError(t, st.repo.CreateRole(integrationCtx, role))

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{
		"name":        origName,
		"description": "updated-desc",
	})
	rec := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/admin/rbac/roles/%d", role.ID), body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
}

func TestRBACIntegration_DeleteRole_Normal(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "role-to-delete", false)

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/admin/rbac/roles/%d", roleID), nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
}

func TestRBACIntegration_DeleteRole_SuperAdmin_Rejected(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "role-super-del", true)

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/admin/rbac/roles/%d", roleID), nil)
	require.NotEqual(t, http.StatusOK, rec.Code,
		"super admin role 不应被成功删除 body=%s", rec.Body.String())

	roles, err := st.repo.GetAllRoles(integrationCtx)
	require.NoError(t, err)
	var stillThere bool
	for _, r := range roles {
		if r.ID == roleID {
			stillThere = true
			break
		}
	}
	require.True(t, stillThere, "super admin role 删除失败后应仍存在")
}

// ---------------------------------------------------------------------------
// 5. 角色-菜单 / 角色-API / 用户-角色分配
// ---------------------------------------------------------------------------

func TestRBACIntegration_SetAndGetRoleMenus(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "role-menu-assign", false)

	mid1 := st.findMenuIDByKey(t, "admin:users")
	mid2 := st.findMenuIDByKey(t, "admin:roles")

	router := st.buildRBACRouter(userID)

	// SET
	body := mustMarshal(t, map[string]any{"menu_ids": []int64{mid1, mid2}})
	rec := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/admin/rbac/roles/%d/menus", roleID), body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	// GET
	rec2 := doRequest(t, router, http.MethodGet,
		fmt.Sprintf("/admin/rbac/roles/%d/menus", roleID), nil)
	require.Equal(t, http.StatusOK, rec2.Code)

	var gotIDs []int64
	code, _ := decodeEnvelope(t, rec2.Body.Bytes(), &gotIDs)
	require.Zero(t, code)
	require.Len(t, gotIDs, 2)
	require.Contains(t, gotIDs, mid1)
	require.Contains(t, gotIDs, mid2)
}

func TestRBACIntegration_GetRoleMenus_Empty(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "role-empty-menus", false)

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet,
		fmt.Sprintf("/admin/rbac/roles/%d/menus", roleID), nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var gotIDs []int64
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &gotIDs)
	require.Zero(t, code)
	require.Empty(t, gotIDs, "新建角色未分配任何菜单时应返回空列表")
}

func TestRBACIntegration_SetRoleMenus_MissingField(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "role-set-menus-missing", false)

	router := st.buildRBACRouter(userID)
	body := mustMarshal(t, map[string]any{}) // 缺 menu_ids
	rec := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/admin/rbac/roles/%d/menus", roleID), body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRBACIntegration_SetAndGetRoleAPIs(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "role-api-assign", false)

	aid1 := st.findAPIIDByMethodPath(t, "GET", "/admin/users")
	aid2 := st.findAPIIDByMethodPath(t, "DELETE", "/admin/users/:id")

	router := st.buildRBACRouter(userID)

	body := mustMarshal(t, map[string]any{"api_ids": []int64{aid1, aid2}})
	rec := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/admin/rbac/roles/%d/apis", roleID), body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	rec2 := doRequest(t, router, http.MethodGet,
		fmt.Sprintf("/admin/rbac/roles/%d/apis", roleID), nil)
	require.Equal(t, http.StatusOK, rec2.Code)

	var gotIDs []int64
	code, _ := decodeEnvelope(t, rec2.Body.Bytes(), &gotIDs)
	require.Zero(t, code)
	require.Len(t, gotIDs, 2)
	require.Contains(t, gotIDs, aid1)
	require.Contains(t, gotIDs, aid2)
}

func TestRBACIntegration_GetRoleAPIs_Empty(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	roleID := st.createTestRole(t, "role-empty-apis", false)

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet,
		fmt.Sprintf("/admin/rbac/roles/%d/apis", roleID), nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var gotIDs []int64
	code, _ := decodeEnvelope(t, rec.Body.Bytes(), &gotIDs)
	require.Zero(t, code)
	require.Empty(t, gotIDs, "新建角色未分配任何 API 时应返回空列表")
}

func TestRBACIntegration_SetAndGetUserRoles(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")
	r1 := st.createTestRole(t, "user-role-a", false)
	r2 := st.createTestRole(t, "user-role-b", false)

	router := st.buildRBACRouter(userID)

	body := mustMarshal(t, map[string]any{
		"role_ids": []int64{r1, r2},
	})
	rec := doRequest(t, router, http.MethodPut,
		fmt.Sprintf("/admin/rbac/users/%d/roles", userID), body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	rec2 := doRequest(t, router, http.MethodGet,
		fmt.Sprintf("/admin/rbac/users/%d/roles", userID), nil)
	require.Equal(t, http.StatusOK, rec2.Code)

	var roles []service.AdminRole
	code, _ := decodeEnvelope(t, rec2.Body.Bytes(), &roles)
	require.Zero(t, code)
	require.Len(t, roles, 2)

	ids := []int64{roles[0].ID, roles[1].ID}
	require.Contains(t, ids, r1)
	require.Contains(t, ids, r2)
}

func TestRBACIntegration_GetUserRoles_InvalidID(t *testing.T) {
	st := getIntegrationState(t)
	userID := st.createTestUser(t, "admin")

	router := st.buildRBACRouter(userID)
	rec := doRequest(t, router, http.MethodGet, "/admin/rbac/users/not-a-number/roles", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
