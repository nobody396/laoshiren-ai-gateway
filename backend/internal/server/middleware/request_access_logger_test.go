package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

type testLogSink struct {
	mu     sync.Mutex
	events []*logger.LogEvent
}

func (s *testLogSink) WriteLogEvent(event *logger.LogEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *testLogSink) list() []*logger.LogEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*logger.LogEvent, len(s.events))
	copy(out, s.events)
	return out
}

func initMiddlewareTestLogger(t *testing.T) *testLogSink {
	return initMiddlewareTestLoggerWithLevel(t, "debug")
}

func initMiddlewareTestLoggerWithLevel(t *testing.T, level string) *testLogSink {
	t.Helper()
	level = strings.TrimSpace(level)
	if level == "" {
		level = "debug"
	}
	if err := logger.Init(logger.InitOptions{
		Level:       level,
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: false,
			ToFile:   false,
		},
	}); err != nil {
		t.Fatalf("init logger: %v", err)
	}
	sink := &testLogSink{}
	logger.SetSink(sink)
	t.Cleanup(func() {
		logger.SetSink(nil)
	})
	return sink
}

func TestRequestLogger_GenerateAndPropagateRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/t", func(c *gin.Context) {
		reqID, ok := c.Request.Context().Value(ctxkey.RequestID).(string)
		if !ok || reqID == "" {
			t.Fatalf("request_id missing in context")
		}
		if got := c.Writer.Header().Get(requestIDHeader); got != reqID {
			t.Fatalf("response header request_id mismatch, header=%q ctx=%q", got, reqID)
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if w.Header().Get(requestIDHeader) == "" {
		t.Fatalf("X-Request-ID should be set")
	}
}

func TestRequestLogger_KeepIncomingRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/t", func(c *gin.Context) {
		reqID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
		if reqID != "rid-fixed" {
			t.Fatalf("request_id=%q, want rid-fixed", reqID)
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set(requestIDHeader, "rid-fixed")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if got := w.Header().Get(requestIDHeader); got != "rid-fixed" {
		t.Fatalf("header=%q, want rid-fixed", got)
	}
}

func TestLogger_AccessLogIncludesCoreFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := initMiddlewareTestLogger(t)

	r := gin.New()
	r.Use(Logger())
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, ctxkey.AccountID, int64(101))
		ctx = context.WithValue(ctx, ctxkey.Platform, "openai")
		ctx = context.WithValue(ctx, ctxkey.Model, "gpt-5")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d", w.Code)
	}

	events := sink.list()
	if len(events) == 0 {
		t.Fatalf("expected at least one log event")
	}
	found := false
	for _, event := range events {
		if event == nil || event.Message != "http request completed" {
			continue
		}
		found = true
		switch v := event.Fields["status_code"].(type) {
		case int:
			if v != http.StatusCreated {
				t.Fatalf("status_code field mismatch: %v", v)
			}
		case int64:
			if v != int64(http.StatusCreated) {
				t.Fatalf("status_code field mismatch: %v", v)
			}
		default:
			t.Fatalf("status_code type mismatch: %T", v)
		}
		switch v := event.Fields["account_id"].(type) {
		case int64:
			if v != 101 {
				t.Fatalf("account_id field mismatch: %v", v)
			}
		case int:
			if v != 101 {
				t.Fatalf("account_id field mismatch: %v", v)
			}
		default:
			t.Fatalf("account_id type mismatch: %T", v)
		}
		if event.Fields["platform"] != "openai" || event.Fields["model"] != "gpt-5" {
			t.Fatalf("platform/model mismatch: %+v", event.Fields)
		}
	}
	if !found {
		t.Fatalf("access log event not found")
	}
}

func TestLogger_CodexImageCapabilityQueryIsNotRecorded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := initMiddlewareTestLogger(t)
	token := strings.Repeat("a", 64)

	r := gin.New()
	r.Use(RequestLogger())
	r.Use(Logger())
	r.GET("/v1/codex-image/preview", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/codex-image/preview?token="+token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}

	found := false
	for _, event := range sink.list() {
		if event == nil || event.Message != "http request completed" {
			continue
		}
		found = true
		if got := event.Fields["path"]; got != "/v1/codex-image/preview" {
			t.Fatalf("path=%v", got)
		}
		serialized := event.Message + fmt.Sprint(event.Fields)
		if strings.Contains(serialized, token) || strings.Contains(serialized, "RawQuery") || strings.Contains(serialized, "?token=") {
			t.Fatalf("access log leaked capability query: %s", serialized)
		}
	}
	if !found {
		t.Fatalf("access log event not found")
	}
}

func TestLogger_HealthPathSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := initMiddlewareTestLogger(t)

	r := gin.New()
	r.Use(Logger())
	r.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if len(sink.list()) != 0 {
		t.Fatalf("health endpoint should not write access log")
	}
}

func TestLogger_AccessLogDroppedWhenLevelWarn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := initMiddlewareTestLoggerWithLevel(t, "warn")

	r := gin.New()
	r.Use(RequestLogger())
	r.Use(Logger())
	r.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d", w.Code)
	}

	events := sink.list()
	for _, event := range events {
		if event != nil && event.Message == "http request completed" {
			t.Fatalf("access log should not be indexed when level=warn: %+v", event)
		}
	}
}

func TestLogger_AuthenticatedActorOnly(t *testing.T) {
	for _, tt := range []struct {
		name     string
		subject  any
		method   string
		wantID   int64
		wantType string
	}{
		{name: "anonymous"},
		{name: "invalid subject", subject: "42"},
		{name: "authenticated user", subject: AuthSubject{UserID: 42}, wantID: 42},
		{name: "admin api principal", subject: AuthSubject{UserID: adminAPIKeyServicePrincipalUserID}, method: "admin_api_key", wantType: "admin_api_key"},
		{name: "unknown principal", subject: AuthSubject{UserID: -2}},
		{name: "unverified admin principal", subject: AuthSubject{UserID: adminAPIKeyServicePrincipalUserID}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			sink := initMiddlewareTestLogger(t)
			r := gin.New()
			r.Use(Logger())
			r.GET("/api/test", func(c *gin.Context) {
				if tt.subject != nil {
					c.Set(string(ContextKeyUser), tt.subject)
				}
				if tt.method != "" {
					c.Set("auth_method", tt.method)
				}
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Authorization", "Bearer never-log-this-token")
			req.Header.Set("X-Api-Key", "never-log-this-key")
			req.Header.Set("X-User-Id", "999")
			r.ServeHTTP(httptest.NewRecorder(), req)
			found := false
			for _, event := range sink.list() {
				if event == nil || event.Message != "http request completed" {
					continue
				}
				found = true
				id, hasID := event.Fields["user_id"]
				if tt.wantID > 0 {
					if id != tt.wantID {
						t.Fatalf("user_id = %v, want %v", id, tt.wantID)
					}
				} else if hasID {
					t.Fatal("unexpected user_id")
				}
				actorType, hasType := event.Fields["actor_type"]
				if tt.wantType != "" {
					if actorType != tt.wantType {
						t.Fatalf("actor_type = %v", actorType)
					}
				} else if hasType {
					t.Fatal("unexpected actor_type")
				}
				for _, value := range event.Fields {
					if value == "Bearer never-log-this-token" || value == "never-log-this-key" {
						t.Fatal("credential leaked")
					}
				}
			}
			if !found {
				t.Fatal("access log not found")
			}
		})
	}
}
