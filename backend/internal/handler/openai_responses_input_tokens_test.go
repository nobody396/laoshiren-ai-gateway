package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesInputTokensUsesLocalEstimateWithoutGatewayDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/input_tokens", strings.NewReader(`{
		"model":"gpt-5.6-sol",
		"instructions":"Be concise.",
		"input":[{"role":"user","content":[{"type":"input_text","text":"hello world"}]}],
		"tools":[{"type":"function","name":"lookup","description":"Look up a value","parameters":{"type":"object"}}]
	}`))

	(&OpenAIGatewayHandler{}).ResponsesInputTokens(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "local-estimate", recorder.Header().Get("X-Token-Count-Source"))
	require.Equal(t, "response.input_tokens", gjson.Get(recorder.Body.String(), "object").String())
	require.Positive(t, gjson.Get(recorder.Body.String(), "input_tokens").Int())
}

func TestResponsesInputTokensRejectsMissingModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/input_tokens", strings.NewReader(`{"input":"hello"}`))

	(&OpenAIGatewayHandler{}).ResponsesInputTokens(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "invalid_request_error")
}

func TestResponsesInputTokensIsClassifiedAsTokenCountRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/input_tokens", nil)

	require.True(t, isCountTokensRequest(c))
	require.True(t, isTokenCountRequestPath("/responses/input_tokens"))
	require.False(t, isTokenCountRequestPath("/v1/responses"))
}
