//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// Exercise the actual request/response forwarding boundary for three turns.
// This proves state fidelity, not a mocked claim about real provider cache hits.
func TestChatReasoningReplayGatewayThreeTurns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			account := protocolBridgeTestAccount()
			account.Extra["openai_responses_supported"] = true
			upstream := &httpUpstreamRecorder{}
			svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}
			messages := []apicompat.ChatMessage{{Role: "user", Content: json.RawMessage(`"Inspect"`)}}
			for turn := 1; turn <= 3; turn++ {
				output := fmt.Sprintf(`[{"type":"reasoning","id":"rs_%d","encrypted_content":"fixture-state-%d","summary":[]},{"type":"message","id":"msg_%d","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"answer-%d"}]}]`, turn, turn, turn, turn)
				sse := `data: {"type":"response.completed","response":{"id":"resp_test","status":"completed","output":` + output + `,"usage":{"input_tokens":100,"output_tokens":10}}}` + "\n\n"
				upstream.resp = &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(sse))}
				body, err := json.Marshal(apicompat.ChatCompletionsRequest{Model: "gpt-6-luna", Messages: messages, Stream: stream})
				require.NoError(t, err)
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
				result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "/v1/responses", upstream.lastReq.URL.Path)
				for prior := 1; prior < turn; prior++ {
					require.Contains(t, string(upstream.lastBody), fmt.Sprintf(`"encrypted_content":"fixture-state-%d"`, prior))
				}
				message := apicompat.ChatMessage{Role: "assistant"}
				if !stream {
					var response apicompat.ChatCompletionsResponse
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
					message = response.Choices[0].Message
				} else {
					var text strings.Builder
					for _, line := range strings.Split(rec.Body.String(), "\n") {
						if !strings.HasPrefix(line, "data: {") {
							continue
						}
						var chunk apicompat.ChatCompletionsChunk
						require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk))
						for _, choice := range chunk.Choices {
							if choice.Delta.Content != nil {
								text.WriteString(*choice.Delta.Content)
							}
							if len(choice.Delta.ResponsesOutput) > 0 {
								message.ResponsesOutput = choice.Delta.ResponsesOutput
							}
						}
					}
					message.Content, _ = json.Marshal(text.String())
				}
				require.Len(t, message.ResponsesOutput, 2)
				require.Equal(t, fmt.Sprintf("fixture-state-%d", turn), message.ResponsesOutput[0].EncryptedContent)
				require.Equal(t, "final_answer", message.ResponsesOutput[1].Phase)
				require.Equal(t, fmt.Sprintf("answer-%d", turn), gjson.GetBytes(message.Content, "@this").String())
				messages = append(messages, message, apicompat.ChatMessage{Role: "user", Content: json.RawMessage(`"Continue"`)})
			}
		})
	}
}

func TestChatReasoningReplayRejectsStaleStateBeforeUpstream(t *testing.T) {
	account := protocolBridgeTestAccount()
	account.Extra["openai_responses_supported"] = true
	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}
	body := []byte(`{"model":"gpt-6-luna","messages":[{"role":"assistant","content":"edited","responses_output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"original"}]}]}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.NotContains(t, rec.Body.String(), "original")
}

func TestChatReasoningReplaySidecarNotForwardedToNativeChat(t *testing.T) {
	account := protocolBridgeTestAccount()
	body := []byte(`{"model":"ZHIPU/GLM-5.3","messages":[{"role":"assistant","content":"answer","responses_output":[{"type":"reasoning","encrypted_content":"fixture-state"}]}],"stream":false}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"chat_test","choices":[{"message":{"role":"assistant","content":"next"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.NotContains(t, string(upstream.lastBody), "responses_output")
	require.Equal(t, "answer", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
}
