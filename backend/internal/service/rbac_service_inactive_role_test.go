//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRBACServiceInactiveSuperRoleDoesNotAuthorize(t *testing.T) {
	repo := &rbacInactiveRoleRepo{
		roles: []*AdminRole{
			{ID: 9, Name: "disabled-super", IsSuperAdmin: true, Status: ResourceStatusInactive},
		},
		menusByRole: []*AdminMenu{
			{ID: 21, Name: "Users", Type: MenuTypeMenu, PermissionKey: "admin:users", SortOrder: 1, Status: ResourceStatusActive},
		},
		apisByRole: []*AdminAPI{
			{ID: 31, Method: "DELETE", Path: "/admin/users/:id", Status: ResourceStatusActive},
		},
		allMenus: []*AdminMenu{
			{ID: 22, Name: "All", Type: MenuTypeMenu, PermissionKey: "admin:all", SortOrder: 1, Status: ResourceStatusActive},
		},
	}
	svc := NewRBACService(repo, &rbacInactiveRoleCache{})

	isSuper, err := svc.IsSuperAdmin(context.Background(), 42)
	require.NoError(t, err)
	require.False(t, isSuper)

	keys, err := svc.GetUserPermissionKeys(context.Background(), 42)
	require.NoError(t, err)
	require.Empty(t, keys)
	require.NotContains(t, keys, "*")

	allowed, err := svc.CheckPermission(context.Background(), 42, APIPermissionKey("DELETE", "/admin/users/:id"))
	require.NoError(t, err)
	require.False(t, allowed)

	tree, err := svc.GetUserMenuTree(context.Background(), 42)
	require.NoError(t, err)
	require.Empty(t, tree)
	require.Zero(t, repo.listMenusCalls, "inactive super role must not receive the super-admin menu path")
}

func TestRBACServiceInactiveNormalRoleDoesNotGrantAPIMenuPermissions(t *testing.T) {
	repo := &rbacInactiveRoleRepo{
		roles: []*AdminRole{
			{ID: 7, Name: "disabled-normal", Status: ResourceStatusInactive},
		},
		menusByRole: []*AdminMenu{
			{ID: 21, Name: "Users", Type: MenuTypeMenu, PermissionKey: "admin:users", SortOrder: 1, Status: ResourceStatusActive},
		},
		apisByRole: []*AdminAPI{
			{ID: 31, Method: "GET", Path: "/admin/users", Status: ResourceStatusActive},
		},
	}
	svc := NewRBACService(repo, &rbacInactiveRoleCache{})

	keys, err := svc.GetUserPermissionKeys(context.Background(), 42)
	require.NoError(t, err)
	require.Empty(t, keys)

	allowed, err := svc.CheckPermission(context.Background(), 42, APIPermissionKey("GET", "/admin/users"))
	require.NoError(t, err)
	require.False(t, allowed)

	tree, err := svc.GetUserMenuTree(context.Background(), 42)
	require.NoError(t, err)
	require.Empty(t, tree)
	require.Empty(t, repo.lastMenuRoleIDs)
	require.Empty(t, repo.lastAPIRoleIDs)
}

func TestRBACServiceActiveRolesStillAuthorize(t *testing.T) {
	t.Run("normal role", func(t *testing.T) {
		repo := &rbacInactiveRoleRepo{
			roles: []*AdminRole{
				{ID: 7, Name: "normal", Status: ResourceStatusActive},
			},
			menusByRole: []*AdminMenu{
				{ID: 21, Name: "Users", Type: MenuTypeMenu, PermissionKey: "admin:users", SortOrder: 1, Status: ResourceStatusActive},
			},
			apisByRole: []*AdminAPI{
				{ID: 31, Method: "GET", Path: "/admin/users", Status: ResourceStatusActive},
			},
		}
		svc := NewRBACService(repo, &rbacInactiveRoleCache{})

		keys, err := svc.GetUserPermissionKeys(context.Background(), 42)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"admin:users", APIPermissionKey("GET", "/admin/users")}, keys)

		allowed, err := svc.CheckPermission(context.Background(), 42, APIPermissionKey("GET", "/admin/users"))
		require.NoError(t, err)
		require.True(t, allowed)

		tree, err := svc.GetUserMenuTree(context.Background(), 42)
		require.NoError(t, err)
		require.Len(t, tree, 1)
		require.Equal(t, "admin:users", tree[0].PermissionKey)
	})

	t.Run("super role", func(t *testing.T) {
		repo := &rbacInactiveRoleRepo{
			roles: []*AdminRole{
				{ID: 9, Name: "super", IsSuperAdmin: true, Status: ResourceStatusActive},
			},
			allMenus: []*AdminMenu{
				{ID: 22, Name: "All", Type: MenuTypeMenu, PermissionKey: "admin:all", SortOrder: 1, Status: ResourceStatusActive},
			},
		}
		svc := NewRBACService(repo, &rbacInactiveRoleCache{})

		isSuper, err := svc.IsSuperAdmin(context.Background(), 42)
		require.NoError(t, err)
		require.True(t, isSuper)

		keys, err := svc.GetUserPermissionKeys(context.Background(), 42)
		require.NoError(t, err)
		require.Equal(t, []string{"*"}, keys)

		tree, err := svc.GetUserMenuTree(context.Background(), 42)
		require.NoError(t, err)
		require.Len(t, tree, 1)
		require.Equal(t, "admin:all", tree[0].PermissionKey)
		require.Equal(t, 1, repo.listMenusCalls)
	})
}

func TestRBACServiceUpdateRoleInvalidatesAllPermissionCaches(t *testing.T) {
	repo := &rbacInactiveRoleRepo{
		allRoles: []*AdminRole{
			{ID: 9, Name: "super", IsSuperAdmin: true, Status: ResourceStatusActive},
		},
	}
	cache := &rbacInactiveRoleCache{}
	svc := NewRBACService(repo, cache)

	err := svc.UpdateRole(ContextWithRBACActorSuperAdmin(context.Background(), true), &AdminRole{
		ID:           9,
		Name:         "super",
		IsSuperAdmin: false,
		Status:       ResourceStatusInactive,
	})

	require.NoError(t, err)
	require.NotNil(t, repo.updatedRole)
	require.False(t, repo.updatedRole.IsSuperAdmin)
	require.Equal(t, ResourceStatusInactive, repo.updatedRole.Status)
	require.Equal(t, 1, cache.invalidateAllCalls)
}

type rbacInactiveRoleRepo struct {
	RBACRepository

	roles       []*AdminRole
	allRoles    []*AdminRole
	menusByRole []*AdminMenu
	apisByRole  []*AdminAPI
	allMenus    []*AdminMenu

	updatedRole     *AdminRole
	lastMenuRoleIDs []int64
	lastAPIRoleIDs  []int64
	listMenusCalls  int
}

func (r *rbacInactiveRoleRepo) GetRolesByUserID(context.Context, int64) ([]*AdminRole, error) {
	return cloneAdminRoles(r.roles), nil
}

func (r *rbacInactiveRoleRepo) GetAllRoles(context.Context) ([]*AdminRole, error) {
	return cloneAdminRoles(r.allRoles), nil
}

func (r *rbacInactiveRoleRepo) UpdateRole(_ context.Context, role *AdminRole) error {
	clone := *role
	r.updatedRole = &clone
	return nil
}

func (r *rbacInactiveRoleRepo) ListMenus(context.Context) ([]*AdminMenu, error) {
	r.listMenusCalls++
	return r.allMenus, nil
}

func (r *rbacInactiveRoleRepo) GetMenusByRoleIDs(_ context.Context, roleIDs []int64) ([]*AdminMenu, error) {
	r.lastMenuRoleIDs = append([]int64(nil), roleIDs...)
	return r.menusByRole, nil
}

func (r *rbacInactiveRoleRepo) GetAPIsByRoleIDs(_ context.Context, roleIDs []int64) ([]*AdminAPI, error) {
	r.lastAPIRoleIDs = append([]int64(nil), roleIDs...)
	return r.apisByRole, nil
}

func cloneAdminRoles(roles []*AdminRole) []*AdminRole {
	out := make([]*AdminRole, 0, len(roles))
	for _, role := range roles {
		if role == nil {
			continue
		}
		clone := *role
		out = append(out, &clone)
	}
	return out
}

type rbacInactiveRoleCache struct {
	RBACCache

	permissionKeys     []string
	menuTree           []byte
	invalidateAllCalls int
}

func (c *rbacInactiveRoleCache) GetUserPermissionKeys(context.Context, int64) ([]string, error) {
	return c.permissionKeys, nil
}

func (c *rbacInactiveRoleCache) SetUserPermissionKeys(_ context.Context, _ int64, keys []string) error {
	c.permissionKeys = append([]string(nil), keys...)
	return nil
}

func (c *rbacInactiveRoleCache) GetUserMenuTree(context.Context, int64) ([]byte, error) {
	return c.menuTree, nil
}

func (c *rbacInactiveRoleCache) SetUserMenuTree(_ context.Context, _ int64, data []byte) error {
	c.menuTree = append([]byte(nil), data...)
	return nil
}

func (c *rbacInactiveRoleCache) InvalidateAllPermissions(context.Context) error {
	c.invalidateAllCalls++
	c.permissionKeys = nil
	c.menuTree = nil
	return nil
}
