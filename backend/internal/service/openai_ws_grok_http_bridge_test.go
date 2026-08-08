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
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/xai"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestProxyResponsesWebSocketFromClient_GrokUsesHTTPBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sseBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_grok_ws_1","model":"grok-4.3"}}`,
		"",
		`data: {"type":"response.output_text.delta","response":{"id":"resp_grok_ws_1"},"delta":"ok"}`,
		"",
		`data: {"type":"response.completed","response":{"id":"resp_grok_ws_1","model":"grok-4.3","usage":{"input_tokens":4,"output_tokens":2}}}`,
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"text/event-stream"},
			"Xai-Request-Id": []string{"xai-ws-req-1"},
		},
		Body: io.NopCloser(strings.NewReader(sseBody)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID: 71, Name: "grok", Platform: PlatformGrok, Type: AccountTypeOAuth,
		Concurrency: 1, Status: StatusActive,
		Credentials: map[string]any{"base_url": xai.DefaultCLIBaseURL},
	}

	errCh := make(chan error, 1)
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			errCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		msgType, firstMessage, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			errCh <- err
			return
		}
		if msgType != coderws.MessageText {
			errCh <- errors.New("first message was not text")
			return
		}
		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		ginCtx.Request = r.Clone(r.Context())
		errCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, account, "access-token", firstMessage, &OpenAIWSIngressHooks{
			InitialRequestModel: "channel-alias",
			MapRequestModel: func(_ int, originalModel string) (string, error) {
				require.Equal(t, "channel-alias", originalModel)
				return "grok-4.3", nil
			},
		})
	}))
	defer wsServer.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	clientConn, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	cancelDial()
	require.NoError(t, err)
	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
	require.NoError(t, clientConn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"channel-alias","stream":true,"input":"hi"}`)))
	cancelWrite()

	var completed []byte
	for range 3 {
		readCtx, cancelRead := context.WithTimeout(context.Background(), 3*time.Second)
		msgType, event, readErr := clientConn.Read(readCtx)
		cancelRead()
		require.NoError(t, readErr)
		require.Equal(t, coderws.MessageText, msgType)
		if gjson.GetBytes(event, "type").String() == "response.completed" {
			completed = event
		}
	}
	require.Equal(t, "channel-alias", gjson.GetBytes(completed, "response.model").String())
	require.Equal(t, "grok-4.3", gjson.GetBytes(upstream.lastBody, "model").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "type").Exists())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, xai.DefaultCLIBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer access-token", upstream.lastReq.Header.Get("Authorization"))

	require.NoError(t, clientConn.Close(coderws.StatusNormalClosure, "done"))
	select {
	case proxyErr := <-errCh:
		require.NoError(t, proxyErr)
	case <-time.After(3 * time.Second):
		t.Fatal("Grok websocket bridge did not exit after client close")
	}
}
