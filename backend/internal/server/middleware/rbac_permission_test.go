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

func TestRequireAPIPermission_ChannelMonitoringRequiresExactOpsAPI(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		permissions []string
		wantStatus  int
	}{
		{name: "granted", permissions: []string{service.APIPermissionKey(http.MethodGet, "/admin/ops/channel-monitoring")}, wantStatus: http.StatusOK},
		{name: "denied", permissions: []string{service.APIPermissionKey(http.MethodGet, "/admin/ops/concurrency")}, wantStatus: http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyIsSuperAdmin), false)
				c.Set(string(ContextKeyUserPermissions), testCase.permissions)
				c.Next()
			})
			router.Use(RequireAPIPermission())
			router.GET("/api/v1/admin/ops/channel-monitoring", func(c *gin.Context) { c.Status(http.StatusOK) })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/channel-monitoring", nil))
			require.Equal(t, testCase.wantStatus, recorder.Code)
		})
	}
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
