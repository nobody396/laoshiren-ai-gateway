package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterSetupProbeRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerSetupProbeRoutes(router)

	tests := []struct {
		path       string
		wantStatus int
	}{
		{path: "/livez", wantStatus: http.StatusOK},
		{path: "/readyz", wantStatus: http.StatusServiceUnavailable},
		{path: "/health", wantStatus: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			require.Equal(t, tt.wantStatus, w.Code)
			require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
			require.Contains(t, w.Body.String(), `"mode":"setup"`)
		})
	}
}

func TestShutdownHTTPServerWaitsForInflightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "finished")
	})
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	requestDone := make(chan error, 1)
	go func() {
		resp, err := http.Get(testServer.URL) //nolint:gosec // local httptest server
		if err == nil {
			_, err = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		requestDone <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- shutdownHTTPServer(testServer.Config, time.Second)
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("shutdown returned before in-flight request finished: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-shutdownDone)
	require.NoError(t, <-requestDone)
}

func TestProvideServiceBuildInfo(t *testing.T) {
	in := handler.BuildInfo{
		Version:   "v-test",
		BuildType: "release",
	}
	out := provideServiceBuildInfo(in)
	require.Equal(t, in.Version, out.Version)
	require.Equal(t, in.BuildType, out.BuildType)
}

func TestProvideCleanup_WithMinimalDependencies_NoPanic(t *testing.T) {
	cleanup := provideCleanup(nil, nil, service.NewLifecycle())
	require.NotPanics(t, cleanup)
}
