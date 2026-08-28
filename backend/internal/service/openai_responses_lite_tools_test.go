package service

import (
	"bytes"
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

func TestOpenAIGatewayResponsesLiteNormalizesOAuthPassthroughAndForwardsHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set(responsesLiteHeader, "true")
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.144.1")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\ndata: [DONE]\n\n")),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID: 501, Name: "lite-oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Concurrency: 1, Status: StatusActive, Schedulable: true, RateMultiplier: f64p(1),
		Credentials: map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-account"},
		Extra:       map[string]any{"openai_passthrough": true},
	}
	body := []byte(`{"model":"gpt-5.6","stream":true,"instructions":"test","sequence":900719925474099312345,"tools":[{"type":"namespace","name":"collaboration"}],"input":"hello"}`)

	result, err := svc.Forward(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "true", upstream.lastReq.Header.Get(responsesLiteHeader))
	require.Equal(t, "900719925474099312345", gjson.GetBytes(upstream.lastBody, "sequence").Raw)
	require.Equal(t, "collaboration", gjson.GetBytes(upstream.lastBody, `input.#(type=="additional_tools").tools.0.name`).String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "parallel_tool_calls").Bool())
}

func TestOpenAIGatewayResponsesLitePassthroughRetriesExplicitRejectedStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set(responsesLiteHeader, "true")
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"code":"unknown_parameter","param":"input[0].status","message":"Unknown parameter"}}`))},
		{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"output":[],"usage":{"input_tokens":1,"output_tokens":1}}`))},
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID: 502, Name: "lite-apikey", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Concurrency: 1, Status: StatusActive, Schedulable: true, RateMultiplier: f64p(1),
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.openai.com"},
		Extra:       map[string]any{"openai_passthrough": true},
	}
	body := []byte(`{"model":"gpt-5.6","stream":false,"input":[{"type":"tool_search_output","status":"completed","call_id":"call_1"},{"type":"tool_search_output","status":"completed","call_id":"call_2"},{"type":"message","status":"completed","role":"user","content":"hi"}]}`)

	result, err := svc.Forward(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.False(t, gjson.GetBytes(upstream.bodies[1], "input.0.status").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "input.1.status").Exists())
	require.Equal(t, "completed", gjson.GetBytes(upstream.bodies[1], "input.2.status").String())
}

func TestNormalizeOpenAIResponsesLiteOAuthMovesNamespaceAndPreservesPrecision(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"sequence":900719925474099312345,"reasoning":{"effort":"high","context":"current_turn"},"parallel_tool_calls":true,"tools":[{"type":"function","name":"shell"},{"type":"namespace","name":"collaboration"}],"input":"hello"}`)

	updated, changed, err := normalizeOpenAIResponsesLitePayloadForAccount(body, account)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "900719925474099312345", gjson.GetBytes(updated, "sequence").Raw)
	require.Equal(t, "all_turns", gjson.GetBytes(updated, "reasoning.context").String())
	require.False(t, gjson.GetBytes(updated, "parallel_tool_calls").Bool())
	require.Equal(t, "shell", gjson.GetBytes(updated, `tools.#(type=="function").name`).String())
	require.Equal(t, "collaboration", gjson.GetBytes(updated, `input.#(type=="additional_tools").tools.0.name`).String())
}

func TestNormalizeOpenAIResponsesLiteAPIKeyOnlyPinsParallelCalls(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	body := []byte(`{"sequence":900719925474099312345,"parallel_tool_calls":true,"tools":[{"type":"namespace","name":"keep-standard-shape"}]}`)

	updated, changed, err := normalizeOpenAIResponsesLitePayloadForAccount(body, account)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "900719925474099312345", gjson.GetBytes(updated, "sequence").Raw)
	require.False(t, gjson.GetBytes(updated, "parallel_tool_calls").Bool())
	require.Equal(t, "namespace", gjson.GetBytes(updated, "tools.0.type").String())
}

func TestNormalizeOpenAIResponsesLiteRejectsNonBooleanParallelCalls(t *testing.T) {
	_, _, err := normalizeOpenAIResponsesLitePayloadForAccount(
		[]byte(`{"parallel_tool_calls":"false"}`),
		&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
	)
	require.ErrorContains(t, err, "parallel_tool_calls to be a boolean")
}

func TestPrepareOpenAIWSHTTPBridgeBodyPreservesLargeNumber(t *testing.T) {
	body, err := prepareOpenAIWSHTTPBridgeBody([]byte(`{"type":"response.create","sequence":900719925474099312345,"model":"gpt-5.6"}`))
	require.NoError(t, err)
	require.Equal(t, "900719925474099312345", gjson.GetBytes(body, "sequence").Raw)
	require.False(t, gjson.GetBytes(body, "type").Exists())
	require.True(t, gjson.GetBytes(body, "stream").Bool())
}

func TestIsOpenAIResponsesLiteWebSocketPayload(t *testing.T) {
	require.True(t, isOpenAIResponsesLiteWebSocketPayload([]byte(`{"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"}}`)))
	require.False(t, isOpenAIResponsesLiteWebSocketPayload([]byte(`{"client_metadata":{}}`)))
}
