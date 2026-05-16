//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRBACServiceGetUserPermissionKeys_IgnoresMenuWildcardForNonSuperAdmin(t *testing.T) {
	repo := &rbacWildcardRepo{
		roles: []*AdminRole{
			{ID: 7, Name: "menu-only", Status: ResourceStatusActive},
		},
		menus: []*AdminMenu{
			{ID: 11, PermissionKey: "*", Status: ResourceStatusActive},
			{ID: 12, PermissionKey: "admin:menus", Status: ResourceStatusActive},
		},
	}
	svc := NewRBACService(repo, &rbacWildcardCache{})

	keys, err := svc.GetUserPermissionKeys(context.Background(), 42)
	require.NoError(t, err)
	require.NotContains(t, keys, "*", "menu wildcard must not enter non-super-admin permission keys")
	require.Contains(t, keys, "admin:menus")

	allowed, err := svc.CheckPermission(context.Background(), 42, APIPermissionKey("DELETE", "/admin/users/:id"))
	require.NoError(t, err)
	require.False(t, allowed, "menu wildcard must not satisfy API permission checks")
}

func TestRBACServiceGetUserPermissionKeys_SuperAdminKeepsWildcard(t *testing.T) {
	repo := &rbacWildcardRepo{
		roles: []*AdminRole{
			{ID: 1, Name: "super", IsSuperAdmin: true, Status: ResourceStatusActive},
		},
	}
	svc := NewRBACService(repo, &rbacWildcardCache{})

	keys, err := svc.GetUserPermissionKeys(context.Background(), 99)
	require.NoError(t, err)
	require.Equal(t, []string{"*"}, keys)
}

func TestRBACServiceGetUserPermissionKeys_RemovesCachedWildcardForNonSuperAdmin(t *testing.T) {
	repo := &rbacWildcardRepo{
		roles: []*AdminRole{
			{ID: 7, Name: "menu-only", Status: ResourceStatusActive},
		},
	}
	cache := &rbacWildcardCache{cachedKeys: []string{"*", "admin:menus"}}
	svc := NewRBACService(repo, cache)

	keys, err := svc.GetUserPermissionKeys(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, []string{"admin:menus"}, keys)
	require.Equal(t, []string{"admin:menus"}, cache.setKeys)
}

func TestRBACServiceCreateUpdateMenuRejectsWildcardPermissionKey(t *testing.T) {
	repo := &rbacWildcardRepo{}
	svc := NewRBACService(repo, &rbacWildcardCache{})

	err := svc.CreateMenu(context.Background(), &AdminMenu{
		Name:          "wildcard-menu",
		Type:          MenuTypeMenu,
		PermissionKey: "*",
	})
	require.Error(t, err)
	require.Nil(t, repo.createdMenu, "wildcard menu must not be persisted")

	err = svc.UpdateMenu(context.Background(), &AdminMenu{
		ID:            11,
		Name:          "wildcard-menu",
		Type:          MenuTypeMenu,
		PermissionKey: "*",
	})
	require.Error(t, err)
	require.Nil(t, repo.updatedMenu, "wildcard menu must not be updated")
}

type rbacWildcardRepo struct {
	RBACRepository

	roles []*AdminRole
	menus []*AdminMenu
	apis  []*AdminAPI

	createdMenu *AdminMenu
	updatedMenu *AdminMenu
}

func (r *rbacWildcardRepo) GetRolesByUserID(context.Context, int64) ([]*AdminRole, error) {
	return r.roles, nil
}

func (r *rbacWildcardRepo) GetMenusByRoleIDs(context.Context, []int64) ([]*AdminMenu, error) {
	return r.menus, nil
}

func (r *rbacWildcardRepo) GetAPIsByRoleIDs(context.Context, []int64) ([]*AdminAPI, error) {
	return r.apis, nil
}

func (r *rbacWildcardRepo) CreateMenu(_ context.Context, m *AdminMenu) error {
	clone := *m
	r.createdMenu = &clone
	return nil
}

func (r *rbacWildcardRepo) UpdateMenu(_ context.Context, m *AdminMenu) error {
	clone := *m
	r.updatedMenu = &clone
	return nil
}

type rbacWildcardCache struct {
	RBACCache

	cachedKeys []string
	setKeys    []string
}

func (c *rbacWildcardCache) GetUserPermissionKeys(context.Context, int64) ([]string, error) {
	return c.cachedKeys, nil
}

func (c *rbacWildcardCache) SetUserPermissionKeys(_ context.Context, _ int64, keys []string) error {
	c.setKeys = append([]string(nil), keys...)
	return nil
}

func (c *rbacWildcardCache) InvalidateAllPermissions(context.Context) error {
	return nil
}
