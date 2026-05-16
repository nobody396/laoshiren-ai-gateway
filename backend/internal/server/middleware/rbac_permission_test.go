//go:build unit

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequireAPIPermission_NonSuperWildcardDoesNotBypassAPIAuth(t *testing.T) {
	router := buildRBACPermissionTestRouter(false, []string{"*"})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/123", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireAPIPermission_SuperAdminStillBypassesAPIAuth(t *testing.T) {
	router := buildRBACPermissionTestRouter(true, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/123", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAPIPermission_ExplicitAPIKeyAllowsNonSuperAdmin(t *testing.T) {
	router := buildRBACPermissionTestRouter(false, []string{
		service.APIPermissionKey(http.MethodDelete, "/admin/users/:id"),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/123", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func buildRBACPermissionTestRouter(isSuper bool, permKeys []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyIsSuperAdmin), isSuper)
		if permKeys != nil {
			c.Set(string(ContextKeyUserPermissions), permKeys)
		}
		c.Next()
	})
	router.Use(RequireAPIPermission())
	router.DELETE("/api/v1/admin/users/:id", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return router
}
