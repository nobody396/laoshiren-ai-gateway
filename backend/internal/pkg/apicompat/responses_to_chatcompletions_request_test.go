package apicompat

import (
	"encoding/json"
	"testing"
)

func TestResponsesToChatCompletionsRequestPreservesMessagesToolsAndFormat(t *testing.T) {
	input := json.RawMessage(`[
		{"role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_image","image_url":"https://example.com/a.png"}]},
		{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"},
		{"type":"function_call_output","call_id":"call_1","output":"done"}
	]`)
	max := 512
	parallel := true
	req := &ResponsesRequest{
		Model: "ZHIPU/GLM-5.3", Instructions: "be concise", Input: input,
		MaxOutputTokens: &max, Stream: true, ParallelToolCalls: &parallel,
		Reasoning:  &ResponsesReasoning{Effort: "high"},
		Tools:      []ResponsesTool{{Type: "function", Name: "lookup", Parameters: json.RawMessage(`{"type":"object"}`)}},
		ToolChoice: json.RawMessage(`{"type":"function","name":"lookup"}`),
		Text:       &ResponsesText{Format: json.RawMessage(`{"type":"json_schema","name":"answer","schema":{"type":"object"},"strict":true}`)},
	}

	got, err := ResponsesToChatCompletionsRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != req.Model || !got.Stream || got.MaxCompletionTokens == nil || *got.MaxCompletionTokens != max {
		t.Fatalf("unexpected scalar conversion: %#v", got)
	}
	if len(got.Messages) != 4 || got.Messages[0].Role != "system" || got.Messages[1].Role != "user" || got.Messages[2].Role != "assistant" || got.Messages[3].Role != "tool" {
		t.Fatalf("unexpected message conversion: %#v", got.Messages)
	}
	if len(got.Messages[2].ToolCalls) != 1 || got.Messages[2].ToolCalls[0].ID != "call_1" || got.Messages[3].ToolCallID != "call_1" {
		t.Fatalf("tool round trip was not preserved: %#v", got.Messages)
	}
	if got.ReasoningEffort != "high" || len(got.Tools) != 1 {
		t.Fatalf("reasoning/tools not preserved: %#v", got)
	}
	var choice map[string]any
	if err := json.Unmarshal(got.ToolChoice, &choice); err != nil || choice["type"] != "function" {
		t.Fatalf("unexpected tool choice: %s", got.ToolChoice)
	}
	var format map[string]any
	if err := json.Unmarshal(got.ResponseFormat, &format); err != nil || format["type"] != "json_schema" {
		t.Fatalf("unexpected response format: %s", got.ResponseFormat)
	}
}

func TestResponsesToChatCompletionsRequestRejectsProviderNativeTools(t *testing.T) {
	req := &ResponsesRequest{Model: "glm", Input: json.RawMessage(`"hello"`), Tools: []ResponsesTool{{Type: "web_search"}}}
	if _, err := ResponsesToChatCompletionsRequest(req); err == nil {
		t.Fatal("expected web_search to be rejected rather than silently removed")
	}
}

func TestResponsesToChatCompletionsRequestRejectsEncryptedReasoningReplay(t *testing.T) {
	req := &ResponsesRequest{Model: "glm", Input: json.RawMessage(`[{"type":"reasoning","encrypted_content":"secret"}]`)}
	if _, err := ResponsesToChatCompletionsRequest(req); err == nil {
		t.Fatal("expected encrypted reasoning replay to be rejected")
	}
}
