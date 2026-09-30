//go:build integration

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKeyCreationIPLimitsCoverRandomAndCustomKeys(t *testing.T) {
	ctx := context.Background()
	rdb := startAuthRouteRedis(t, ctx)
	router := newKeyCreationRoutesTestRouter(rdb)
	ip := "198.51.100.20"
	for i := 1; i <= 121; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader([]string{`{"name":"random"}`, `{"name":"custom","custom_key":"custom-key-test-123"}`}[i%2]))
		req.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if i <= 120 {
			require.Equal(t, http.StatusUnauthorized, w.Code)
		} else {
			require.Equal(t, http.StatusTooManyRequests, w.Code)
		}
	}
	// Another IP is independent. Once its day window is full, it is denied even
	// with a fresh hour window. Expiry releases the IP instead of permanent bans.
	ip = "198.51.100.21"
	require.NoError(t, rdb.Set(ctx, "rate_limit:key-create-day:"+ip, 500, time.Hour).Err())
	request := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", nil)
		req.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	require.Equal(t, http.StatusTooManyRequests, request())
	require.NoError(t, rdb.Del(ctx, "rate_limit:key-create-day:"+ip).Err())
	require.Equal(t, http.StatusUnauthorized, request())
}
