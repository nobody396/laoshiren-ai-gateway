package apicompat

import (
	"encoding/json"
	"testing"
)

func TestChatCompletionsResponseToResponsesPreservesTextToolsAndUsage(t *testing.T) {
	content, _ := json.Marshal("hello")
	resp := &ChatCompletionsResponse{
		ID: "chatcmpl-1", Model: "upstream-model",
		Choices: []ChatChoice{{Index: 0, FinishReason: "tool_calls", Message: ChatMessage{
			Role: "assistant", Content: content, ReasoningContent: "thought",
			ToolCalls: []ChatToolCall{{ID: "call_1", Type: "function", Function: ChatFunctionCall{Name: "lookup", Arguments: `{"q":"x"}`}}},
		}}},
		Usage: &ChatUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14, PromptTokensDetails: &ChatTokenDetails{CachedTokens: 3}},
	}
	got := ChatCompletionsResponseToResponses(resp, "public-model")
	if got.Model != "public-model" || got.Status != "completed" || got.Usage == nil || got.Usage.InputTokens != 10 || got.Usage.InputTokensDetails.CachedTokens != 3 {
		t.Fatalf("unexpected response metadata: %#v", got)
	}
	if len(got.Output) != 3 || got.Output[0].Type != "reasoning" || got.Output[1].Type != "message" || got.Output[2].Type != "function_call" {
		t.Fatalf("unexpected outputs: %#v", got.Output)
	}
	if got.Output[2].CallID != "call_1" || got.Output[2].Arguments != `{"q":"x"}` {
		t.Fatalf("tool call not preserved: %#v", got.Output[2])
	}
}

func TestChatChunksToResponsesEventsAccumulateTerminalOutputAndUsage(t *testing.T) {
	state := NewChatChunkToResponsesState("public-model")
	text := "hello"
	reasoning := "think"
	idx := 0
	stop := "tool_calls"
	chunks := []ChatCompletionsChunk{
		{ID: "chatcmpl-abc", Model: "upstream", Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{Role: "assistant"}}}},
		{ID: "chatcmpl-abc", Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{ReasoningContent: &reasoning}}}},
		{ID: "chatcmpl-abc", Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{Content: &text}}}},
		{ID: "chatcmpl-abc", Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{ToolCalls: []ChatToolCall{{Index: &idx, ID: "call_1", Type: "function", Function: ChatFunctionCall{Name: "lookup", Arguments: `{"q":`}}}}}}},
		{ID: "chatcmpl-abc", Choices: []ChatChunkChoice{{Index: 0, Delta: ChatDelta{ToolCalls: []ChatToolCall{{Index: &idx, Function: ChatFunctionCall{Arguments: `"x"}`}}}}, FinishReason: &stop}}},
		{ID: "chatcmpl-abc", Choices: []ChatChunkChoice{}, Usage: &ChatUsage{PromptTokens: 9, CompletionTokens: 5, TotalTokens: 14}},
	}
	var eventTypes []string
	for i := range chunks {
		for _, event := range ChatChunkToResponsesEvents(&chunks[i], state) {
			eventTypes = append(eventTypes, event.Type)
		}
	}
	final := FinalizeChatResponsesStream(state)
	for _, event := range final {
		eventTypes = append(eventTypes, event.Type)
	}
	if len(eventTypes) == 0 || eventTypes[0] != "response.created" || eventTypes[len(eventTypes)-1] != "response.completed" {
		t.Fatalf("unexpected event sequence: %#v", eventTypes)
	}
	terminal := final[len(final)-1].Response
	if terminal == nil || terminal.Usage == nil || terminal.Usage.TotalTokens != 14 || len(terminal.Output) != 3 {
		t.Fatalf("unexpected terminal response: %#v", terminal)
	}
	if terminal.Output[1].Content[0].Text != "hello" || terminal.Output[2].Arguments != `{"q":"x"}` {
		t.Fatalf("stream output was not accumulated: %#v", terminal.Output)
	}
}
