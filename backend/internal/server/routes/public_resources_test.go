package routes

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterPublicResourceRoutesDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	require.NotPanics(t, func() {
		RegisterPublicResourceRoutes(router, &handler.Handlers{
			Resource: &handler.ResourceHandler{},
		})
	})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		"GET /downloads/claude-desktop/windows-x64/:sha256/Claude-Setup.exe",
		"GET /downloads/claude-desktop/:version/:sha256/:filename",
		"GET /downloads/cc-switch/:version/:sha256/:filename",
		"GET /downloads/codex/windows-x64/:version/:sha256/:filename",
		"GET /downloads/codex/:version/:sha256/:filename",
		"GET /downloads/git-for-windows/:version/:sha256/:filename",
		"GET /downloads/grok-build/:version/:sha256/:filename",
		"GET /downloads/codex-plus-plus/:version/:sha256/:filename",
	} {
		_, ok := routes[expected]
		require.True(t, ok, "missing route %s", expected)
	}
}
