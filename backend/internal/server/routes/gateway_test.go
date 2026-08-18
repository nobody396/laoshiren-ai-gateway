package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler"
	servermiddleware "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGatewayRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: &handler.OpenAIGatewayHandler{},
		},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
	)

	return router
}

func TestGatewayRoutesOpenAIResponsesCompactPathIsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{"/v1/responses/compact", "/responses/compact"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusNotFound, w.Code, "path=%s should hit OpenAI responses handler", path)
	}
}

func TestGatewayRoutesGrokTextAndMediaAliasesAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()
	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	for _, expected := range []string{
		"POST /v1/messages",
		"POST /v1/messages/count_tokens",
		"POST /v1/responses",
		"POST /v1/responses/*subpath",
		"GET /v1/responses",
		"GET /v1/codex-image/previews/:token",
		"HEAD /v1/codex-image/previews/:token",
		"POST /v1/chat/completions",
		"POST /v1/images/generations",
		"POST /v1/images/edits",
		"POST /v1/videos/generations",
		"POST /v1/videos/edits",
		"POST /v1/videos/extensions",
		"GET /v1/videos/:request_id",
		"GET /v1/videos/:request_id/content",
		"POST /responses",
		"POST /responses/*subpath",
		"GET /responses",
		"POST /messages/count_tokens",
		"POST /chat/completions",
		"POST /images/generations",
		"POST /images/edits",
		"POST /videos/generations",
		"POST /videos/edits",
		"POST /videos/extensions",
		"GET /videos/:request_id",
		"GET /videos/:request_id/content",
	} {
		_, ok := routes[expected]
		require.True(t, ok, "missing Grok-compatible gateway route %s", expected)
	}
}

func TestGatewayRoutesResponsesSubpathRejectsNonConformingSubpaths(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, path := range []string{
		"/v1/responses/../../x/y",
		"/v1/responses/..%2f..%2fx/y",
		"/v1/responses/%2e%2e/%2e%2e/x",
		"/responses/%2e%2e%2fx",
		`/v1/responses/..\..\x`,
		"/v1/responses/%3fa=b",
		"/v1/responses/x%23frag",
		"/v1/responses/compact%2f..",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, "path=%s must be rejected at the edge", path)
		require.Contains(t, w.Body.String(), "Unsupported responses subpath", "path=%s", path)
	}
}
