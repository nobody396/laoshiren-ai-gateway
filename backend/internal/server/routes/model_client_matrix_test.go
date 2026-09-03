//go:build unit

package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func modelMatrixRouter(auth gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/v1/admin")
	admin.Use(auth)
	admin.Use(middleware.RequireAPIPermission())
	registerModelClientMatrixRoutes(admin)
	return router
}

func TestModelClientMatrixRejectsUnauthenticatedAndOrdinaryUsers(t *testing.T) {
	tests := []struct {
		name string
		code int
		auth gin.HandlerFunc
	}{
		{
			name: "unauthenticated",
			code: http.StatusUnauthorized,
			auth: func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) },
		},
		{
			name: "ordinary_user",
			code: http.StatusForbidden,
			auth: func(c *gin.Context) { c.AbortWithStatus(http.StatusForbidden) },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-client-matrix", nil)
			modelMatrixRouter(tc.auth).ServeHTTP(response, request)
			require.Equal(t, tc.code, response.Code)
			require.NotContains(t, response.Body.String(), "client_matrix")
		})
	}
}

func TestModelClientMatrixRequiresPermissionAndReturns34By14ForAdmin(t *testing.T) {
	permissionAuth := func(permitted bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyIsSuperAdmin), false)
			permissions := []string{}
			if permitted {
				permissions = []string{"api:GET:/admin/model-client-matrix"}
			}
			c.Set(string(middleware.ContextKeyUserPermissions), permissions)
			c.Next()
		}
	}

	denied := httptest.NewRecorder()
	modelMatrixRouter(permissionAuth(false)).ServeHTTP(
		denied,
		httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-client-matrix", nil),
	)
	require.Equal(t, http.StatusForbidden, denied.Code)

	allowed := httptest.NewRecorder()
	modelMatrixRouter(permissionAuth(true)).ServeHTTP(
		allowed,
		httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-client-matrix", nil),
	)
	require.Equal(t, http.StatusOK, allowed.Code)
	require.Equal(t, "private, no-store", allowed.Header().Get("Cache-Control"))

	var payload struct {
		Counts struct {
			Models        int `json:"models"`
			Clients       int `json:"clients"`
			Intersections int `json:"intersections"`
		} `json:"counts"`
	}
	require.NoError(t, json.Unmarshal(allowed.Body.Bytes(), &payload))
	require.Equal(t, 34, payload.Counts.Models)
	require.Equal(t, 14, payload.Counts.Clients)
	require.Equal(t, 34*14, payload.Counts.Intersections)
}
