package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type openAICompatBufferedReadErrorCloser struct{ err error }

func (r *openAICompatBufferedReadErrorCloser) Read([]byte) (int, error) { return 0, r.err }
func (r *openAICompatBufferedReadErrorCloser) Close() error             { return nil }

func TestChatCompletionsBufferedResponsesReadErrorReturnsFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name string
		err  error
		code string
	}{
		{name: "unexpected_eof", err: io.ErrUnexpectedEOF, code: OpenAIUpstreamStreamReadErrorCode},
		{name: "http2_reset", err: errors.New("stream error: stream ID 7; INTERNAL_ERROR; received from peer"), code: OpenAIUpstreamHTTP2StreamErrorCode},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"upstream-rid"}},
				Body:       &openAICompatBufferedReadErrorCloser{err: tt.err},
			}
			account := &Account{ID: 40, Name: "openai-oauth", Platform: PlatformOpenAI}

			result, err := (&OpenAIGatewayService{}).handleChatBufferedStreamingResponse(
				resp, c, account, "gpt-5.6-sol", "gpt-5.6-sol", "gpt-5.6-sol", time.Now(),
			)

			require.Error(t, err)
			require.Nil(t, result)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
			require.Equal(t, "upstream-rid", failoverErr.ResponseHeaders.Get("x-request-id"))
			require.Equal(t, tt.code, gjson.GetBytes(failoverErr.ResponseBody, "error.code").String())
			require.Empty(t, rec.Body.String())
			require.False(t, c.Writer.Written())
		})
	}
}

func TestChatCompletionsBufferedResponsesReadErrorDoesNotFailoverAfterClientCancel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	requestContext, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestContext)
	resp := &http.Response{StatusCode: http.StatusOK, Body: &openAICompatBufferedReadErrorCloser{err: io.ErrUnexpectedEOF}}

	result, err := (&OpenAIGatewayService{}).handleChatBufferedStreamingResponse(
		resp, c, &Account{ID: 40, Platform: PlatformOpenAI}, "gpt-5.6-sol", "gpt-5.6-sol", "gpt-5.6-sol", time.Now(),
	)

	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.NotErrorAs(t, err, &failoverErr)
	require.Empty(t, rec.Body.String())
}

func TestChatCompletionsBufferedResponsesOversizedLineDoesNotFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	oversized := strings.Repeat("x", 256)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(oversized))}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: 64}}}

	result, err := svc.handleChatBufferedStreamingResponse(
		resp, c, &Account{ID: 40, Platform: PlatformOpenAI}, "gpt-5.6-sol", "gpt-5.6-sol", "gpt-5.6-sol", time.Now(),
	)

	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.NotErrorAs(t, err, &failoverErr)
}

func TestAnthropicBufferedResponsesReadErrorKeepsExistingBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	resp := &http.Response{StatusCode: http.StatusOK, Body: &openAICompatBufferedReadErrorCloser{err: io.ErrUnexpectedEOF}}

	result, err := (&OpenAIGatewayService{}).handleAnthropicBufferedStreamingResponse(
		resp, c, "gpt-5.6-sol", "gpt-5.6-sol", "gpt-5.6-sol", time.Now(),
	)

	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.NotErrorAs(t, err, &failoverErr)
}
