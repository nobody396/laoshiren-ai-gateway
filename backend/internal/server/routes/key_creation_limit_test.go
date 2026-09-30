package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler"
	servermiddleware "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newKeyCreationRoutesTestRouter(rdb *redis.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterUserRoutes(r.Group("/api/v1"), &handler.Handlers{}, servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, rdb)
	return r
}

func TestKeyCreationRateLimitFailCloseWhenRedisUnavailable(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 20 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	r := newKeyCreationRoutesTestRouter(rdb)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/keys", nil))
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	// The dependency outage only restricts creation; listing/deleting are not
	// wired to this limiter (the route registration is the seam under test).
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		w = httptest.NewRecorder()
		path := "/api/v1/keys"
		if method == http.MethodDelete {
			path += "/1"
		}
		r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
}
