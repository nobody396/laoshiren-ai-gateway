package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type readinessProbeStub struct {
	checks map[string]string
	ready  bool
}

func (s readinessProbeStub) Check(context.Context) (map[string]string, bool) {
	return s.checks, s.ready
}

func TestCommonRoutesLiveAndReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, readinessProbeStub{
		ready: true,
		checks: map[string]string{
			"application": "ok",
			"postgres":    "ok",
			"redis":       "ok",
		},
	})

	tests := []struct {
		path     string
		wantJSON string
	}{
		{
			path:     "/livez",
			wantJSON: `{"status":"ok","checks":{"application":"alive"}}`,
		},
		{
			path:     "/readyz",
			wantJSON: `{"status":"ok","checks":{"application":"ok","postgres":"ok","redis":"ok"}}`,
		},
		{
			path:     "/health",
			wantJSON: `{"status":"ok","checks":{"application":"ok","postgres":"ok","redis":"ok"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
			require.JSONEq(t, tt.wantJSON, w.Body.String())
		})
	}
}

func TestCommonRoutesReadinessFailureReturnsRedacted503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, readinessProbeStub{
		ready: false,
		checks: map[string]string{
			"application": "ok",
			"postgres":    "unavailable",
			"redis":       "ok",
		},
	})

	for _, path := range []string{"/readyz", "/health"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
		require.JSONEq(t, `{
			"status":"unavailable",
			"checks":{"application":"ok","postgres":"unavailable","redis":"ok"}
		}`, w.Body.String())
		require.NotContains(t, w.Body.String(), "postgresql://")
		require.NotContains(t, w.Body.String(), "secret")
	}
}

func TestCommonRoutesMissingReadinessCannotReportGreen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.JSONEq(t, `{
		"status":"unavailable",
		"checks":{
			"application":"unavailable",
			"postgres":"not_checked",
			"redis":"not_checked"
		}
	}`, w.Body.String())
}
