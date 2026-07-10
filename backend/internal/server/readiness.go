package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const defaultReadinessTimeout = 2 * time.Second

type readinessCheck func(context.Context) error

// Readiness tracks whether this process is eligible to receive traffic and
// verifies the dependencies required to serve normal requests.
//
// Liveness intentionally does not depend on this type: a PostgreSQL or Redis
// outage must make the process unready without causing a container restart
// loop.
type Readiness struct {
	timeout       time.Duration
	postgresCheck readinessCheck
	redisCheck    readinessCheck
	draining      atomic.Bool
}

// ProvideReadiness wires the production PostgreSQL and Redis checks.
func ProvideReadiness(db *sql.DB, redisClient *redis.Client) *Readiness {
	postgresCheck := func(ctx context.Context) error {
		if db == nil {
			return errors.New("postgres client is unavailable")
		}
		return db.PingContext(ctx)
	}
	redisCheck := func(ctx context.Context) error {
		if redisClient == nil {
			return errors.New("redis client is unavailable")
		}
		return redisClient.Ping(ctx).Err()
	}

	return newReadiness(defaultReadinessTimeout, postgresCheck, redisCheck)
}

func newReadiness(timeout time.Duration, postgresCheck, redisCheck readinessCheck) *Readiness {
	if timeout <= 0 {
		timeout = defaultReadinessTimeout
	}
	return &Readiness{
		timeout:       timeout,
		postgresCheck: postgresCheck,
		redisCheck:    redisCheck,
	}
}

// BeginDrain immediately removes the process from readiness-qualified traffic.
// It is safe to call more than once.
func (r *Readiness) BeginDrain() {
	if r != nil {
		r.draining.Store(true)
	}
}

// IsDraining reports whether graceful shutdown has begun.
func (r *Readiness) IsDraining() bool {
	return r == nil || r.draining.Load()
}

// Check runs PostgreSQL and Redis checks under one shared deadline. The result
// contains only fixed component states; dependency errors are deliberately not
// exposed because they may contain hosts, DSNs, or other sensitive details.
func (r *Readiness) Check(parent context.Context) (map[string]string, bool) {
	checks := map[string]string{
		"application": "unavailable",
		"postgres":    "unavailable",
		"redis":       "unavailable",
	}
	if r == nil {
		return checks, false
	}
	if r.draining.Load() {
		checks["application"] = "draining"
		checks["postgres"] = "not_checked"
		checks["redis"] = "not_checked"
		return checks, false
	}

	checks["application"] = "ok"
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	defer cancel()

	type result struct {
		component string
		err       error
	}
	results := make(chan result, 2)
	run := func(component string, check readinessCheck) {
		if check == nil {
			results <- result{component: component, err: errors.New("dependency check is unavailable")}
			return
		}
		results <- result{component: component, err: check(ctx)}
	}

	go run("postgres", r.postgresCheck)
	go run("redis", r.redisCheck)

	remaining := 2
	for remaining > 0 {
		select {
		case checked := <-results:
			if checked.err == nil && ctx.Err() == nil {
				checks[checked.component] = "ok"
			}
			remaining--
		case <-ctx.Done():
			remaining = 0
		}
	}
	if r.draining.Load() {
		checks["application"] = "draining"
		return checks, false
	}

	ready := checks["application"] == "ok" &&
		checks["postgres"] == "ok" &&
		checks["redis"] == "ok"
	return checks, ready
}

// AdmissionMiddleware rejects requests that arrive after draining begins while
// keeping probe endpoints reachable. Requests already executing are unaffected
// and can finish during http.Server.Shutdown.
func (r *Readiness) AdmissionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !r.IsDraining() || isProbePath(c.Request.URL.Path) {
			c.Next()
			return
		}

		c.Header("Retry-After", "1")
		c.Header("Cache-Control", "no-store")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"status": "unavailable",
			"checks": gin.H{"application": "draining"},
		})
	}
}

func isProbePath(path string) bool {
	switch path {
	case "/livez", "/readyz", "/health":
		return true
	default:
		return false
	}
}
