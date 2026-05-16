//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestRBACServiceCreateRoleSuperAdminRequiresSuperActor(t *testing.T) {
	repo := newRBACSuperAdminBoundaryRepo()
	svc := NewRBACService(repo, &rbacSuperAdminBoundaryCache{})

	err := svc.CreateRole(context.Background(), &AdminRole{
		Name:         "forbidden-super",
		IsSuperAdmin: true,
	})
	require.Error(t, err)
	require.Equal(t, 403, infraerrors.Code(err))
	require.Equal(t, "SUPER_ADMIN_ROLE_MUTATION_FORBIDDEN", infraerrors.Reason(err))
	require.Nil(t, repo.createdRole)

	err = svc.CreateRole(ContextWithRBACActorSuperAdmin(context.Background(), true), &AdminRole{
		Name:         "allowed-super",
		IsSuperAdmin: true,
	})
	require.NoError(t, err)
	require.NotNil(t, repo.createdRole)
	require.True(t, repo.createdRole.IsSuperAdmin)
}

func TestRBACServiceUpdateRoleSuperAdminRequiresSuperActor(t *testing.T) {
	repo := newRBACSuperAdminBoundaryRepo()
	repo.rolesByID[1] = &AdminRole{ID: 1, Name: "super", IsSuperAdmin: true, Status: ResourceStatusActive}
	repo.rolesByID[2] = &AdminRole{ID: 2, Name: "normal", IsSuperAdmin: false, Status: ResourceStatusActive}
	svc := NewRBACService(repo, &rbacSuperAdminBoundaryCache{})

	cases := []struct {
		name string
		role *AdminRole
	}{
		{
			name: "updates existing super role",
			role: &AdminRole{ID: 1, Name: "super-new-name", IsSuperAdmin: true, Status: ResourceStatusActive},
		},
		{
			name: "downgrades existing super role",
			role: &AdminRole{ID: 1, Name: "super", IsSuperAdmin: false, Status: ResourceStatusActive},
		},
		{
			name: "promotes normal role to super",
			role: &AdminRole{ID: 2, Name: "normal", IsSuperAdmin: true, Status: ResourceStatusActive},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo.updatedRole = nil

			err := svc.UpdateRole(context.Background(), tt.role)

			require.Error(t, err)
			require.Equal(t, 403, infraerrors.Code(err))
			require.Equal(t, "SUPER_ADMIN_ROLE_MUTATION_FORBIDDEN", infraerrors.Reason(err))
			require.Nil(t, repo.updatedRole)
		})
	}

	repo.updatedRole = nil
	err := svc.UpdateRole(context.Background(), &AdminRole{
		ID:           2,
		Name:         "normal-renamed",
		IsSuperAdmin: false,
		Status:       ResourceStatusActive,
	})
	require.NoError(t, err)
	require.NotNil(t, repo.updatedRole)
	require.False(t, repo.updatedRole.IsSuperAdmin)

	repo.updatedRole = nil
	err = svc.UpdateRole(ContextWithRBACActorSuperAdmin(context.Background(), true), &AdminRole{
		ID:           1,
		Name:         "super-renamed",
		IsSuperAdmin: false,
		Status:       ResourceStatusActive,
	})
	require.NoError(t, err)
	require.NotNil(t, repo.updatedRole)
	require.False(t, repo.updatedRole.IsSuperAdmin)
}

func TestRBACServiceAssignUserRolesSuperAdminRequiresSuperActor(t *testing.T) {
	repo := newRBACSuperAdminBoundaryRepo()
	repo.rolesByID[10] = &AdminRole{ID: 10, Name: "normal", Status: ResourceStatusActive}
	repo.rolesByID[99] = &AdminRole{ID: 99, Name: "super", IsSuperAdmin: true, Status: ResourceStatusActive}
	repo.userBaseRoles[200] = RoleAdmin
	repo.userBaseRoles[201] = RoleAdmin
	svc := NewRBACService(repo, &rbacSuperAdminBoundaryCache{})

	for _, tt := range []struct {
		name         string
		targetUserID int64
	}{
		{name: "to self", targetUserID: 200},
		{name: "to another admin", targetUserID: 201},
	} {
		t.Run("blocks assigning a super role "+tt.name, func(t *testing.T) {
			repo.userRoleIDs[tt.targetUserID] = []int64{10}
			repo.assignedUserID = 0

			err := svc.AssignUserRoles(context.Background(), tt.targetUserID, []int64{10, 99})

			require.Error(t, err)
			require.Equal(t, 403, infraerrors.Code(err))
			require.Equal(t, "SUPER_ADMIN_ROLE_MUTATION_FORBIDDEN", infraerrors.Reason(err))
			require.Zero(t, repo.assignedUserID)
		})
	}

	t.Run("blocks revoking an existing super role", func(t *testing.T) {
		repo.userRoleIDs[200] = []int64{99}
		repo.assignedUserID = 0

		err := svc.AssignUserRoles(context.Background(), 200, []int64{})

		require.Error(t, err)
		require.Equal(t, 403, infraerrors.Code(err))
		require.Equal(t, "SUPER_ADMIN_ROLE_MUTATION_FORBIDDEN", infraerrors.Reason(err))
		require.Zero(t, repo.assignedUserID)
	})

	t.Run("allows normal role assignment", func(t *testing.T) {
		repo.userRoleIDs[200] = []int64{}
		repo.assignedUserID = 0

		err := svc.AssignUserRoles(context.Background(), 200, []int64{10})

		require.NoError(t, err)
		require.Equal(t, int64(200), repo.assignedUserID)
		require.Equal(t, []int64{10}, repo.assignedRoleIDs)
	})

	t.Run("allows super actor to assign and revoke super roles", func(t *testing.T) {
		superCtx := ContextWithRBACActorSuperAdmin(context.Background(), true)
		repo.userRoleIDs[200] = []int64{10}
		repo.assignedUserID = 0

		err := svc.AssignUserRoles(superCtx, 200, []int64{10, 99})
		require.NoError(t, err)
		require.Equal(t, int64(200), repo.assignedUserID)
		require.Equal(t, []int64{10, 99}, repo.assignedRoleIDs)

		err = svc.AssignUserRoles(superCtx, 200, []int64{10})
		require.NoError(t, err)
		require.Equal(t, []int64{10}, repo.assignedRoleIDs)
	})
}

type rbacSuperAdminBoundaryRepo struct {
	RBACRepository

	rolesByID     map[int64]*AdminRole
	userRoleIDs   map[int64][]int64
	userBaseRoles map[int64]string

	createdRole     *AdminRole
	updatedRole     *AdminRole
	assignedUserID  int64
	assignedRoleIDs []int64
}

func newRBACSuperAdminBoundaryRepo() *rbacSuperAdminBoundaryRepo {
	return &rbacSuperAdminBoundaryRepo{
		rolesByID:     make(map[int64]*AdminRole),
		userRoleIDs:   make(map[int64][]int64),
		userBaseRoles: make(map[int64]string),
	}
}

func (r *rbacSuperAdminBoundaryRepo) GetAllRoles(context.Context) ([]*AdminRole, error) {
	roles := make([]*AdminRole, 0, len(r.rolesByID))
	for _, role := range r.rolesByID {
		clone := *role
		roles = append(roles, &clone)
	}
	return roles, nil
}

func (r *rbacSuperAdminBoundaryRepo) CreateRole(_ context.Context, role *AdminRole) error {
	clone := *role
	r.createdRole = &clone
	return nil
}

func (r *rbacSuperAdminBoundaryRepo) UpdateRole(_ context.Context, role *AdminRole) error {
	clone := *role
	r.updatedRole = &clone
	return nil
}

func (r *rbacSuperAdminBoundaryRepo) AssignUserRoles(_ context.Context, userID int64, roleIDs []int64) error {
	r.assignedUserID = userID
	r.assignedRoleIDs = append([]int64(nil), roleIDs...)
	r.userRoleIDs[userID] = append([]int64(nil), roleIDs...)
	return nil
}

func (r *rbacSuperAdminBoundaryRepo) GetUserRoles(_ context.Context, userID int64) ([]*AdminRole, error) {
	roleIDs := r.userRoleIDs[userID]
	roles := make([]*AdminRole, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		role, ok := r.rolesByID[roleID]
		if !ok {
			continue
		}
		clone := *role
		roles = append(roles, &clone)
	}
	return roles, nil
}

func (r *rbacSuperAdminBoundaryRepo) GetUserRole(_ context.Context, userID int64) (string, error) {
	role, ok := r.userBaseRoles[userID]
	if !ok {
		return "", errors.New("missing base role")
	}
	return role, nil
}

type rbacSuperAdminBoundaryCache struct {
	RBACCache
}

func (c *rbacSuperAdminBoundaryCache) InvalidateUserPermissions(context.Context, int64) error {
	return nil
}

func (c *rbacSuperAdminBoundaryCache) InvalidateAllPermissions(context.Context) error {
	return nil
}
