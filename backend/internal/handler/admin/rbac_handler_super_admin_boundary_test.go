//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRBACHandlerUpdateRolePartialRequestPreservesExistingRoleFields(t *testing.T) {
	repo := newHandlerRBACBoundaryRepo()
	repo.rolesByID[7] = &service.AdminRole{
		ID:           7,
		Name:         "normal-role",
		Description:  "old",
		IsSuperAdmin: false,
		Status:       service.ResourceStatusActive,
	}
	router := buildRBACBoundaryRouter(repo)

	rec := doRBACBoundaryRequest(t, router, http.MethodPut, "/admin/rbac/roles/7", map[string]any{
		"description": "new",
	})

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NotNil(t, repo.updatedRole)
	require.Equal(t, "normal-role", repo.updatedRole.Name)
	require.Equal(t, "new", repo.updatedRole.Description)
	require.False(t, repo.updatedRole.IsSuperAdmin)
	require.Equal(t, service.ResourceStatusActive, repo.updatedRole.Status)
}

func TestRBACHandlerUpdateRolePartialRequestCannotMutateExistingSuperRoleAsNonSuper(t *testing.T) {
	repo := newHandlerRBACBoundaryRepo()
	repo.rolesByID[99] = &service.AdminRole{
		ID:           99,
		Name:         "super-role",
		Description:  "old",
		IsSuperAdmin: true,
		Status:       service.ResourceStatusActive,
	}
	router := buildRBACBoundaryRouter(repo)

	rec := doRBACBoundaryRequest(t, router, http.MethodPut, "/admin/rbac/roles/99", map[string]any{
		"description": "new",
	})

	require.Equal(t, http.StatusForbidden, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "SUPER_ADMIN_ROLE_MUTATION_FORBIDDEN")
	require.Nil(t, repo.updatedRole)
}

type handlerRBACBoundaryRepo struct {
	service.RBACRepository

	rolesByID   map[int64]*service.AdminRole
	updatedRole *service.AdminRole
}

func newHandlerRBACBoundaryRepo() *handlerRBACBoundaryRepo {
	return &handlerRBACBoundaryRepo{
		rolesByID: make(map[int64]*service.AdminRole),
	}
}

func (r *handlerRBACBoundaryRepo) GetAllRoles(context.Context) ([]*service.AdminRole, error) {
	roles := make([]*service.AdminRole, 0, len(r.rolesByID))
	for _, role := range r.rolesByID {
		clone := *role
		roles = append(roles, &clone)
	}
	return roles, nil
}

func (r *handlerRBACBoundaryRepo) UpdateRole(_ context.Context, role *service.AdminRole) error {
	clone := *role
	r.updatedRole = &clone
	return nil
}

func (r *handlerRBACBoundaryRepo) GetUserRole(context.Context, int64) (string, error) {
	return "", errors.New("not implemented")
}

func buildRBACBoundaryRouter(repo *handlerRBACBoundaryRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := NewRBACHandler(service.NewRBACService(repo, &handlerRBACBoundaryCache{}))
	router.PUT("/admin/rbac/roles/:id", h.UpdateRole)
	return router
}

type handlerRBACBoundaryCache struct {
	service.RBACCache
}

func (c *handlerRBACBoundaryCache) InvalidateAllPermissions(context.Context) error {
	return nil
}

func doRBACBoundaryRequest(t *testing.T, router *gin.Engine, method, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
