package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNoStoreAPIResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sets_no_store_headers_for_api_routes", func(t *testing.T) {
		router := gin.New()
		router.Use(NoStoreAPIResponses())
		router.GET("/api/v1/auth/me", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, apiNoStoreCacheControl, rec.Header().Get("Cache-Control"))
		require.Equal(t, "no-cache", rec.Header().Get("Pragma"))
		require.Equal(t, "0", rec.Header().Get("Expires"))
		require.ElementsMatch(t, []string{"Authorization", "Cookie", "Origin"}, rec.Header().Values("Vary"))
	})

	t.Run("overrides_handler_cache_header_for_api_routes", func(t *testing.T) {
		router := gin.New()
		router.Use(NoStoreAPIResponses())
		router.GET("/api/v1/auth/me", func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age=3600")
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		router.ServeHTTP(rec, req)

		require.Equal(t, apiNoStoreCacheControl, rec.Header().Get("Cache-Control"))
	})

	t.Run("leaves_frontend_routes_cache_unchanged", func(t *testing.T) {
		router := gin.New()
		router.Use(NoStoreAPIResponses())
		router.GET("/dashboard", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		router.ServeHTTP(rec, req)

		require.Empty(t, rec.Header().Get("Cache-Control"))
	})
}
