package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReadinessCheckHealthy(t *testing.T) {
	readiness := newReadiness(
		time.Second,
		func(context.Context) error { return nil },
		func(context.Context) error { return nil },
	)

	checks, ready := readiness.Check(context.Background())

	require.True(t, ready)
	require.Equal(t, "ok", checks["application"])
	require.Equal(t, "ok", checks["postgres"])
	require.Equal(t, "ok", checks["redis"])
}

func TestReadinessCheckDependencyFailuresAreUnavailable(t *testing.T) {
	tests := []struct {
		name          string
		postgresCheck readinessCheck
		redisCheck    readinessCheck
		failed        string
	}{
		{
			name:          "postgres",
			postgresCheck: func(context.Context) error { return errors.New("postgresql://user:secret@db.internal/app") },
			redisCheck:    func(context.Context) error { return nil },
			failed:        "postgres",
		},
		{
			name:          "redis",
			postgresCheck: func(context.Context) error { return nil },
			redisCheck:    func(context.Context) error { return errors.New("redis://:secret@cache.internal:6379") },
			failed:        "redis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readiness := newReadiness(time.Second, tt.postgresCheck, tt.redisCheck)
			checks, ready := readiness.Check(context.Background())

			require.False(t, ready)
			require.Equal(t, "unavailable", checks[tt.failed])
			require.NotContains(t, fmt.Sprint(checks), "secret")
			require.NotContains(t, fmt.Sprint(checks), "internal")
		})
	}
}

func TestReadinessCheckUsesOneBoundedDeadline(t *testing.T) {
	const timeout = 40 * time.Millisecond
	blockedCheck := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}
	readiness := newReadiness(timeout, blockedCheck, blockedCheck)

	started := time.Now()
	checks, ready := readiness.Check(context.Background())
	elapsed := time.Since(started)

	require.False(t, ready)
	require.Equal(t, "unavailable", checks["postgres"])
	require.Equal(t, "unavailable", checks["redis"])
	require.GreaterOrEqual(t, elapsed, timeout/2)
	require.Less(t, elapsed, 5*timeout)
}

func TestReadinessBeginDrainSkipsDependencies(t *testing.T) {
	var calls atomic.Int32
	check := func(context.Context) error {
		calls.Add(1)
		return nil
	}
	readiness := newReadiness(time.Second, check, check)
	readiness.BeginDrain()

	checks, ready := readiness.Check(context.Background())

	require.False(t, ready)
	require.True(t, readiness.IsDraining())
	require.Equal(t, "draining", checks["application"])
	require.Equal(t, "not_checked", checks["postgres"])
	require.Equal(t, int32(0), calls.Load())
}

func TestReadinessDrainWinsRaceWithDependencyChecks(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	check := func(context.Context) error {
		started <- struct{}{}
		<-release
		return nil
	}
	readiness := newReadiness(time.Second, check, check)

	type checkResult struct {
		checks map[string]string
		ready  bool
	}
	result := make(chan checkResult, 1)
	go func() {
		checks, ready := readiness.Check(context.Background())
		result <- checkResult{checks: checks, ready: ready}
	}()

	<-started
	<-started
	readiness.BeginDrain()
	close(release)

	completed := <-result
	require.False(t, completed.ready)
	require.Equal(t, "draining", completed.checks["application"])
}

func TestAdmissionMiddlewareLetsInflightRequestFinishAndRejectsNewWork(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readiness := newReadiness(
		time.Second,
		func(context.Context) error { return nil },
		func(context.Context) error { return nil },
	)
	started := make(chan struct{})
	release := make(chan struct{})

	router := gin.New()
	router.Use(readiness.AdmissionMiddleware())
	router.GET("/stream", func(c *gin.Context) {
		close(started)
		<-release
		c.String(http.StatusOK, "finished")
	})
	router.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	testServer := httptest.NewServer(router)
	t.Cleanup(testServer.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	type response struct {
		status int
		body   string
		err    error
	}
	inflightResult := make(chan response, 1)
	go func() {
		resp, err := http.Get(testServer.URL + "/stream") //nolint:gosec // local httptest server
		if err != nil {
			inflightResult <- response{err: err}
			return
		}
		defer resp.Body.Close()
		body, readErr := io.ReadAll(resp.Body)
		inflightResult <- response{status: resp.StatusCode, body: string(body), err: readErr}
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("in-flight request did not start")
	}

	readiness.BeginDrain()
	newResp, err := http.Get(testServer.URL + "/stream") //nolint:gosec // local httptest server
	require.NoError(t, err)
	defer newResp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, newResp.StatusCode)
	require.Equal(t, "1", newResp.Header.Get("Retry-After"))
	require.Equal(t, "no-store", newResp.Header.Get("Cache-Control"))

	liveResp, err := http.Get(testServer.URL + "/livez") //nolint:gosec // local httptest server
	require.NoError(t, err)
	defer liveResp.Body.Close()
	require.Equal(t, http.StatusOK, liveResp.StatusCode)

	close(release)
	select {
	case result := <-inflightResult:
		require.NoError(t, result.err)
		require.Equal(t, http.StatusOK, result.status)
		require.Equal(t, "finished", result.body)
	case <-time.After(time.Second):
		t.Fatal("in-flight request was not allowed to finish")
	}
}
