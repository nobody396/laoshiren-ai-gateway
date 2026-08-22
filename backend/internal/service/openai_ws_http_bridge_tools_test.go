package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIWSHTTPBridgeReusesClientToolsWhenFollowupOmitsDeclaration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	firstSSE := `data: {"type":"response.completed","response":{"id":"resp_first","model":"gpt-5.6-sol","output":[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"exec","arguments":"{\"input\":\"pwd\"}"}],"usage":{"input_tokens":2,"output_tokens":1}}}` + "\n\n"
	secondSSE := `data: {"type":"response.completed","response":{"id":"resp_second","model":"gpt-5.6-sol","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n"
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(firstSSE))},
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(secondSSE))},
	}}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{
		ID: 9001, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-upstream", "base_url": "https://relay.example/v1"},
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	var clientEvents [][]byte
	writeClient := func(message []byte) error {
		clientEvents = append(clientEvents, append([]byte(nil), message...))
		return nil
	}

	firstPayload := []byte(`{"type":"response.create","model":"gpt-5.6-sol","tools":[{"type":"custom","name":"exec"}],"input":"run pwd"}`)
	firstResult, err := svc.proxyOpenAIWSHTTPBridgeTurn(
		context.Background(), c, account, "sk-upstream", firstPayload, len(firstPayload), "gpt-5.6-sol", "", "", "", "", 1, writeClient,
	)
	require.NoError(t, err)
	require.NotNil(t, firstResult)
	require.Equal(t, "custom_tool_call", gjson.GetBytes(clientEvents[0], "response.output.0.type").String())

	secondPayload := []byte(`{"type":"response.create","model":"gpt-5.6-sol","previous_response_id":"resp_first","input":[{"type":"custom_tool_call_output","id":"ctco_client","call_id":"call_1","output":"ok"}]}`)
	secondResult, err := svc.proxyOpenAIWSHTTPBridgeTurn(
		context.Background(), c, account, "sk-upstream", secondPayload, len(secondPayload), "gpt-5.6-sol", "", "", "", "", 2, writeClient,
	)
	require.NoError(t, err)
	require.NotNil(t, secondResult)

	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "function", gjson.GetBytes(upstream.bodies[0], "tools.0.type").String())
	require.Equal(t, "function", gjson.GetBytes(upstream.bodies[1], "tools.0.type").String())
	require.Equal(t, "exec", gjson.GetBytes(upstream.bodies[1], "tools.0.name").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(upstream.bodies[1], "input.0.type").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "input.0.id").Exists())
}
