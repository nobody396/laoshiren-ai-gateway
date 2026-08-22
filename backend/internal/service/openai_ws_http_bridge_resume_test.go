package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOpenAIWSCurrentTurnRetryPayloadRejectsOrphanToolOutput(t *testing.T) {
	payload := []byte(`{"type":"response.create","model":"mapped-model","previous_response_id":"resp_old"}`)
	fullInput := []json.RawMessage{json.RawMessage(`{"type":"function_call_output","call_id":"missing_call","output":"done"}`)}

	retryPayload, retrySafe, err := buildOpenAIWSCurrentTurnRetryPayload(payload, fullInput, true, "gpt-5.6-sol")

	require.NoError(t, err)
	require.False(t, retrySafe)
	require.Nil(t, retryPayload)
}

func TestProxyOpenAIWSHTTPBridgeTurnLaterTurn429FailsOverBeforeClientWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"60"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached"}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 129, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	payload := []byte(`{"type":"response.create","model":"gpt-5.6-sol","previous_response_id":"resp_old","input":[{"role":"user","content":"continue"}]}`)
	writes := 0

	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(
		context.Background(), c, account, "access-token", payload, len(payload),
		"gpt-5.6-sol", "", "", "", "", 2,
		func([]byte) error { writes++; return nil },
	)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Zero(t, writes)
}

func TestProxyOpenAIWSHTTPBridgeTurnLaterTurnDoesNotFailOverAfterOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n" +
				"data: {\"type\":\"error\",\"error\":{\"type\":\"rate_limit_error\",\"message\":\"limited\"}}\n\n",
		)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi"}`)
	var writes [][]byte

	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(
		context.Background(), c, account, "sk-test", payload, len(payload),
		"gpt-5", "", "", "", "", 2,
		func(message []byte) error { writes = append(writes, append([]byte(nil), message...)); return nil },
	)

	require.NotNil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.Len(t, writes, 2)
}

func TestOpenAIWSHTTPBridgeLaterTurn429RetryPreservesTurnAndExactlyOneSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_first\",\"output\":[{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"first-ok\"}]},{\"id\":\"fc_1\",\"type\":\"function_call\",\"call_id\":\"call_1\",\"name\":\"inspect\",\"arguments\":\"{}\"}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n",
		))},
		{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": []string{"60"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"type":"usage_limit_reached","message":"limited"}}`))},
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_second\",\"output\":[{\"id\":\"msg_2\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"second-ok\"}]}],\"usage\":{\"input_tokens\":4,\"output_tokens\":1}}}\n\n",
		))},
	}}
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector()}
	account := &Account{ID: 129, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1, Extra: map[string]any{"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeHTTPBridge}}
	nextAccount := *account
	nextAccount.ID = 130

	serverErrCh := make(chan error, 1)
	turnSuccesses := make(map[int]int)
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			serverErrCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		_, firstMessage, readErr := conn.Read(readCtx)
		cancel()
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		ginCtx.Request = r.Clone(r.Context())
		hooks := &OpenAIWSIngressHooks{AfterTurn: func(turn int, result *OpenAIForwardResult, turnErr error) {
			if turnErr == nil && result != nil {
				turnSuccesses[turn]++
			}
		}}
		proxyErr := svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, account, "token-a", firstMessage, hooks)
		retryPayload, retryTurn, ok := OpenAIWSCurrentTurnRetryPayload(proxyErr)
		if !ok || len(retryPayload) == 0 {
			serverErrCh <- proxyErr
			return
		}
		hooks.TurnOffset = retryTurn - 1
		serverErrCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, &nextAccount, "token-b", retryPayload, hooks)
	}))
	defer wsServer.Close()

	dialCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	cancel()
	require.NoError(t, err)
	defer func() { _ = clientConn.CloseNow() }()

	writeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	require.NoError(t, clientConn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.6-sol","input":[{"role":"user","content":"first"}]}`)))
	cancel()
	readCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, firstCompleted, err := clientConn.Read(readCtx)
	cancel()
	require.NoError(t, err)
	require.Equal(t, "resp_first", gjson.GetBytes(firstCompleted, "response.id").String())

	writeCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	require.NoError(t, clientConn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.6-sol","previous_response_id":"resp_first","input":[{"type":"function_call_output","call_id":"call_1","output":"second"}]}`)))
	cancel()
	readCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	_, retriedCompleted, err := clientConn.Read(readCtx)
	cancel()
	require.NoError(t, err)
	require.Equal(t, "resp_second", gjson.GetBytes(retriedCompleted, "response.id").String())
	_ = clientConn.Close(coderws.StatusNormalClosure, "done")

	select {
	case proxyErr := <-serverErrCh:
		require.NoError(t, proxyErr)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for replacement account completion")
	}
	require.Len(t, upstream.bodies, 3)
	require.Equal(t, 1, turnSuccesses[1])
	require.Equal(t, 1, turnSuccesses[2], "failed attempt must not create a second successful usage turn")
	require.NotContains(t, string(upstream.bodies[2]), "previous_response_id")
}
