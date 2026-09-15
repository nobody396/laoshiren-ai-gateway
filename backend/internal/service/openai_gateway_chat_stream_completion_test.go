package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChatStreamCompletionRejectsUnusableResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, body string }{
		{"missing_terminal", "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\"}}\n\n"},
		{"empty_with_input_usage", "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":90000,\"output_tokens\":0}}}\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.body))}
			result, err := (&OpenAIGatewayService{}).handleChatStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5.5", "gpt-5.5", "gpt-5.5", true, time.Now())
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover, "unusable output must reach the existing account-switch branch")
			require.Nil(t, result, "failed attempt must not be billed as a successful completion")
			require.False(t, c.Writer.Written(), "do not commit a placeholder 200 before fallback")
			require.Empty(t, rec.Body.String())
		})
	}
}

func TestChatStreamCompletionPreservesTerminalOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, output, want string }{
		{"text", `[{"type":"message","content":[{"type":"output_text","text":"answer"}]}]`, `"content":"answer"`},
		{"tool", `[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{}"}]`, `"name":"lookup"`},
		{"refusal", `[{"type":"message","content":[{"type":"refusal","refusal":"cannot help"}]}]`, `"refusal":"cannot help"`},
		{"reasoning", `[{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}]}]`, `"reasoning_content":"thinking"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			body := `data: {"type":"response.completed","response":{"id":"resp_test","status":"completed","output":` + tc.output + `,"usage":{"input_tokens":9,"output_tokens":0}}}` + "\n\n"
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
			result, err := (&OpenAIGatewayService{}).handleChatStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5.5", "gpt-5.5", "gpt-5.5", true, time.Now())
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Contains(t, rec.Body.String(), tc.want)
			require.Contains(t, rec.Body.String(), "data: [DONE]")
			require.Equal(t, 9, result.Usage.InputTokens)
		})
	}
}

func TestChatStreamCompletionBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	created := `{"type":"response.created","response":{"id":"resp_test"}}`
	text := `{"type":"response.output_text.delta","delta":"answer"}`
	tool := `{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call_1","name":"lookup"}}`
	args := `{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{}"}`
	terminal := `{"type":"response.completed","response":{"status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":9,"output_tokens":1}}}`
	for _, asynchronous := range []bool{false, true} {
		for _, tc := range []struct {
			name                     string
			events                   []string
			fail, written, cancelled bool
			contains                 string
			count                    int
		}{
			{name: "small_request_placeholder_only", events: []string{created}, fail: true},
			{name: "done_without_terminal", events: []string{created, "[DONE]"}, fail: true},
			{name: "failed_with_usage", events: []string{`{"type":"response.failed","response":{"status":"failed","usage":{"input_tokens":9}}}`}, fail: true},
			{name: "error_event", events: []string{`{"type":"error","code":"server_error"}`}, fail: true},
			{name: "encrypted_reasoning_is_not_visible", events: []string{`{"type":"response.completed","response":{"status":"completed","output":[{"type":"reasoning","encrypted_content":"opaque"}],"usage":{"input_tokens":9,"output_tokens":4}}}`}, fail: true},
			{name: "text_not_duplicated", events: []string{created, text, terminal}, written: true, contains: `"content":"answer"`, count: 1},
			{name: "tool_not_duplicated", events: []string{created, tool, args, `{"type":"response.completed","response":{"status":"completed","output":[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}],"usage":{"input_tokens":9,"output_tokens":0}}}`}, written: true, contains: `"name":"lookup"`, count: 1},
			{name: "refusal_delta", events: []string{`{"type":"response.refusal.delta","delta":"cannot help"}`, `{"type":"response.completed","response":{"status":"completed","output":[{"type":"message","content":[{"type":"refusal","refusal":"cannot help"}]}]}}`}, written: true, contains: `"refusal":"cannot help"`, count: 1},
			{name: "partial_text_no_restart", events: []string{created, text}, fail: true, written: true, contains: `"content":"answer"`, count: 1},
			{name: "partial_then_failure", events: []string{text, `{"type":"response.failed","response":{"status":"failed"}}`}, fail: true, written: true},
			{name: "cancelled_before_output", events: []string{created}, fail: true, cancelled: true},
			{name: "incomplete_with_output", events: []string{text, `{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`}, written: true, contains: `"finish_reason":"length"`, count: 1},
			{name: "done_alias_output", events: []string{strings.Replace(terminal, "response.completed", "response.done", 1)}, written: true, contains: `"content":"answer"`, count: 1},
		} {
			t.Run(fmt.Sprintf("async=%t/%s", asynchronous, tc.name), func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
				if tc.cancelled {
					ctx, cancel := context.WithCancel(c.Request.Context())
					cancel()
					c.Request = c.Request.WithContext(ctx)
				}
				var body strings.Builder
				for _, e := range tc.events {
					fmt.Fprintf(&body, "data: %s\n\n", e)
				}
				svc := &OpenAIGatewayService{}
				if asynchronous {
					svc.cfg = &config.Config{Gateway: config.GatewayConfig{StreamKeepaliveInterval: 1, StreamDataIntervalTimeout: 2}}
				}
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body.String()))}
				result, err := svc.handleChatStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5.5", "gpt-5.5", "gpt-5.5", true, time.Now())
				if tc.fail {
					require.Error(t, err)
					var fe *UpstreamFailoverError
					if tc.cancelled {
						require.NotErrorAs(t, err, &fe)
					} else {
						require.ErrorAs(t, err, &fe)
						if !tc.written {
							require.Nil(t, result)
						}
					}
					require.NotContains(t, rec.Body.String(), "data: [DONE]")
				} else {
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Contains(t, rec.Body.String(), "data: [DONE]")
				}
				require.Equal(t, tc.written, c.Writer.Written())
				if tc.contains != "" {
					require.Equal(t, tc.count, strings.Count(rec.Body.String(), tc.contains))
				}
			})
		}
	}
}

func TestChatStreamCompletionPreservesTerminalToolArguments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, prefix := range []string{"", `{"q":`} {
		t.Run(prefix, func(t *testing.T) {
			events := []string{`data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call_1","name":"lookup"}}` + "\n\n"}
			if prefix != "" {
				e, _ := json.Marshal(map[string]any{"type": "response.function_call_arguments.delta", "output_index": 0, "delta": prefix})
				events = append(events, "data: "+string(e)+"\n\n")
			}
			events = append(events, `data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":1}"}],"usage":{"input_tokens":9,"output_tokens":0}}}`+"\n\n")
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.Join(events, "")))}
			result, err := (&OpenAIGatewayService{}).handleChatStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5.5", "gpt-5.5", "gpt-5.5", true, time.Now())
			require.NoError(t, err)
			require.NotNil(t, result)
			var arguments string
			for _, line := range strings.Split(rec.Body.String(), "\n") {
				if !strings.HasPrefix(line, "data: {") {
					continue
				}
				var chunk apicompat.ChatCompletionsChunk
				require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk))
				for _, choice := range chunk.Choices {
					for _, tool := range choice.Delta.ToolCalls {
						arguments += tool.Function.Arguments
					}
				}
			}
			require.JSONEq(t, `{"q":1}`, arguments)
			require.Equal(t, 1, strings.Count(rec.Body.String(), `"name":"lookup"`))
		})
	}
}

func TestChatStreamCompletionTimeoutKeepsFallbackPossible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	wrote := make(chan error, 1)
	go func() {
		_, err := io.WriteString(writer, "data: {\"type\":\"response.created\",\"response\":{}}\n\n")
		wrote <- err
	}()
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 2, StreamKeepaliveInterval: 1}}}
	resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: reader}
	result, err := svc.handleChatStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5.5", "gpt-5.5", "gpt-5.5", true, time.Now())
	var fe *UpstreamFailoverError
	require.ErrorAs(t, err, &fe)
	require.Nil(t, result)
	require.False(t, c.Writer.Written(), "keepalive must not commit a placeholder 200 before fallback")
	require.NoError(t, <-wrote)
}

type chatStreamDisconnectedWriter struct{ gin.ResponseWriter }

func (w *chatStreamDisconnectedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestChatStreamCompletionDisconnectedWriterDoesNotRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Writer = &chatStreamDisconnectedWriter{ResponseWriter: c.Writer}
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":9,\"output_tokens\":1}}}\n\n"
	resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	result, err := (&OpenAIGatewayService{}).handleChatStreamingResponse(resp, c, &Account{ID: 1, Platform: PlatformOpenAI}, "gpt-5.5", "gpt-5.5", "gpt-5.5", true, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 9, result.Usage.InputTokens)
	require.Equal(t, 1, result.Usage.OutputTokens)
}
