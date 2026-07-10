package routes

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type readinessProbe interface {
	Check(context.Context) (map[string]string, bool)
}

// RegisterCommonRoutes 注册通用路由（健康检查、状态等）
func RegisterCommonRoutes(r *gin.Engine, readiness readinessProbe) {
	// Liveness only reports that the process can answer HTTP. External
	// dependencies deliberately do not participate in this probe.
	r.GET("/livez", func(c *gin.Context) {
		setProbeNoStoreHeaders(c)
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"checks": gin.H{"application": "alive"},
		})
	})

	readinessHandler := func(c *gin.Context) {
		setProbeNoStoreHeaders(c)
		if readiness == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"checks": gin.H{
					"application": "unavailable",
					"postgres":    "not_checked",
					"redis":       "not_checked",
				},
			})
			return
		}
		checks, ready := readiness.Check(c.Request.Context())
		if !ready {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"checks": checks,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"checks": checks,
		})
	}

	r.GET("/readyz", readinessHandler)
	// Compatibility alias: legacy monitors now get truthful readiness instead
	// of the former constant 200 response.
	r.GET("/health", readinessHandler)

	// Claude Code 遥测日志（忽略，直接返回200）
	r.POST("/api/event_logging/batch", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Setup status endpoint (always returns needs_setup: false in normal mode)
	// This is used by the frontend to detect when the service has restarted after setup
	r.GET("/setup/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"needs_setup": false,
				"step":        "completed",
			},
		})
	})
}

func setProbeNoStoreHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}
