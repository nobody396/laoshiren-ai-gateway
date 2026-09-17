package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type nativeResponsesRecoveryUpstream struct {
	service.HTTPUpstream
	calls   []int64
	partial bool
}

func (u *nativeResponsesRecoveryUpstream) Do(_ *http.Request, _ string, id int64, _ int) (*http.Response, error) {
	u.calls = append(u.calls, id)
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	switch len(u.calls) {
	case 1:
		resp.StatusCode = 524
		resp.Body = io.NopCloser(strings.NewReader("upstream timeout"))
	case 2:
		r, w := io.Pipe()
		resp.Body = r
		go func() {
			defer w.Close()
			if u.partial {
				_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n"))
			} else {
				_, _ = w.Write([]byte("data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"discard-me\"}}\n\n"))
				time.Sleep(1100 * time.Millisecond)
			}
		}()
	default:
		resp.Body = io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-ok\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"))
	}
	return resp, nil
}

func TestResponsesRecoveryAfter524AndTruncatedStream(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "comment_only_retries"
		if partial {
			name = "partial_output_does_not_replay"
		}
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			group := &service.Group{ID: 6, Platform: service.PlatformOpenAI}
			accounts := make([]service.Account, 3)
			for i := range accounts {
				id := int64(i + 1)
				accounts[i] = service.Account{ID: id, Name: "test", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 4, Priority: i + 1,
					Credentials:   map[string]any{"api_key": "test-only-key", "base_url": "https://upstream.example.test/v1", "model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"}},
					AccountGroups: []service.AccountGroup{{AccountID: id, GroupID: 6, Priority: i + 1}}}
			}
			upstream := &nativeResponsesRecoveryUpstream{partial: partial}
			h := newCodexResponsesTestHandler(t, accounts, upstream)
			h.cfg.Gateway.StreamKeepaliveInterval = 1
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest("POST", "/responses", strings.NewReader(`{"model":"gpt-5.6-sol","stream":true,"input":"hi"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 8, GroupID: &group.ID, Group: group, User: &service.User{ID: 9, Status: service.StatusActive}})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9, Concurrency: 4})
			c.Header("X-Request-Id", "req-recovery")
			h.Responses(c)
			require.Equal(t, 200, rec.Code)
			outcome := buildFinalReliabilityObservation(c, time.Now(), nil)
			if partial {
				require.Len(t, upstream.calls, 2)
				require.Contains(t, rec.Body.String(), `"type":"response.failed"`)
				require.Equal(t, service.ReliabilityOutcomeFailure, outcome.Outcome)
				require.Nil(t, buildSuccessfulAttemptReliabilityObservation(c))
			} else {
				require.Len(t, upstream.calls, 3)
				require.Contains(t, rec.Body.String(), `"type":"response.completed"`)
				require.NotContains(t, rec.Body.String(), "discard-me")
				require.NotContains(t, rec.Body.String(), `"type":"response.failed"`)
				require.Equal(t, service.ReliabilityOutcomeRecovered, outcome.Outcome)
			}
		})
	}
}
