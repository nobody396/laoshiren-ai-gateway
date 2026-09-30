package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The extension is opt-in on replay: ordinary Chat clients stay wire compatible.
// It must carry the opaque state, not pretend the readable summary is that state.
func TestChatCompletionsEncryptedReasoningRoundTrip(t *testing.T) {
	var response ResponsesResponse
	require.NoError(t, json.Unmarshal([]byte(`{"id":"resp_test","model":"gpt-6-luna","status":"completed","output":[{"type":"reasoning","id":"rs_test","encrypted_content":"fixture-opaque-state","summary":[{"type":"summary_text","text":"readable summary"}]},{"type":"function_call","call_id":"call_test","name":"lookup","arguments":"{}"}]}`), &response))
	chat := ResponsesToChatCompletions(&response, response.Model)
	wire, err := json.Marshal(chat)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(wire, &decoded))
	message := decoded["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	require.Contains(t, message, "responses_output", "Chat response must let a client replay opaque Responses state")

	requestWire, err := json.Marshal(map[string]any{"model": response.Model, "messages": []any{message, map[string]any{"role": "tool", "tool_call_id": "call_test", "content": "result"}}})
	require.NoError(t, err)
	var request ChatCompletionsRequest
	require.NoError(t, json.Unmarshal(requestWire, &request))
	replayed, err := ChatCompletionsToResponses(&request)
	require.NoError(t, err)
	var items []map[string]any
	require.NoError(t, json.Unmarshal(replayed.Input, &items))
	require.Len(t, items, 3, "do not also turn the summary into an invented assistant thinking message")
	require.Equal(t, "reasoning", items[0]["type"])
	require.Equal(t, "rs_test", items[0]["id"])
	require.Equal(t, "fixture-opaque-state", items[0]["encrypted_content"])
	require.Equal(t, "function_call", items[1]["type"])
	require.Equal(t, "function_call_output", items[2]["type"])
}

func TestChatCompletionsReasoningReplayPreservesNativeItemOrder(t *testing.T) {
	var response ResponsesResponse
	require.NoError(t, json.Unmarshal([]byte(`{"status":"completed","output":[{"type":"reasoning","id":"rs_a","encrypted_content":"opaque-a","summary":[]},{"type":"message","role":"assistant","phase":"commentary","content":[{"type":"output_text","text":"Checking"}]},{"type":"reasoning","id":"rs_b","encrypted_content":"opaque-b","summary":[]},{"type":"function_call","call_id":"call_a","name":"lookup","arguments":"{}"}]}`), &response))
	message := ResponsesToChatCompletions(&response, "gpt-6-luna").Choices[0].Message
	replayed, err := ChatCompletionsToResponses(&ChatCompletionsRequest{Model: "gpt-6-luna", Messages: []ChatMessage{message}})
	require.NoError(t, err)
	var items []map[string]any
	require.NoError(t, json.Unmarshal(replayed.Input, &items))
	require.Len(t, items, 4)
	require.Equal(t, "rs_a", items[0]["id"])
	require.Equal(t, []any{}, items[0]["summary"])
	require.Equal(t, "commentary", items[1]["phase"])
	require.Equal(t, "rs_b", items[2]["id"])
	require.Equal(t, "call_a", items[3]["call_id"])
}

func TestChatCompletionsReasoningReplayRejectsStaleOrInvalidState(t *testing.T) {
	for _, input := range []string{
		`{"role":"user","content":"answer","responses_output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"answer"}]}]}`,
		`{"role":"assistant","content":"edited","responses_output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"original"}]}]}`,
		`{"role":"assistant","responses_output":[{"type":"reasoning","summary":[]}]}`,
		`{"role":"assistant","content":"answer","responses_output":[{"type":"message","role":"system","content":[{"type":"output_text","text":"answer"}]}]}`,
		`{"role":"assistant","responses_output":[{"type":"unknown"}]}`,
	} {
		var message ChatMessage
		require.NoError(t, json.Unmarshal([]byte(input), &message))
		_, err := ChatCompletionsToResponses(&ChatCompletionsRequest{Model: "gpt-6-luna", Messages: []ChatMessage{message}})
		require.Error(t, err)
	}
}

func TestChatCompletionsReasoningExtensionAbsentForOrdinaryResponses(t *testing.T) {
	response := ResponsesResponse{Output: []ResponsesOutput{{Type: "message", Role: "assistant", Content: []ResponsesContentPart{{Type: "output_text", Text: "answer"}}}}}
	wire, err := json.Marshal(ResponsesToChatCompletions(&response, "gpt-6-luna"))
	require.NoError(t, err)
	require.NotContains(t, string(wire), "responses_output")
}

func TestChatCompletionsReasoningStreamingMetadataDoesNotRepeatText(t *testing.T) {
	state := NewResponsesEventToChatState()
	first := ResponsesEventToChatChunks(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "answer"}, state)
	require.Len(t, first, 1)
	response := &ResponsesResponse{Status: "completed", Output: []ResponsesOutput{
		{Type: "reasoning", ID: "rs_test", EncryptedContent: "fixture-state"},
		{Type: "message", Role: "assistant", Content: []ResponsesContentPart{{Type: "output_text", Text: "answer"}}},
	}}
	terminal := ResponsesEventToChatChunks(&ResponsesStreamEvent{Type: "response.completed", Response: response}, state)
	metadataCount := 0
	for _, chunk := range terminal {
		for _, choice := range chunk.Choices {
			require.True(t, choice.Delta.Content == nil || *choice.Delta.Content == "")
			if len(choice.Delta.ResponsesOutput) > 0 {
				metadataCount++
				require.Equal(t, "fixture-state", choice.Delta.ResponsesOutput[0].EncryptedContent)
			}
		}
	}
	require.Equal(t, 1, metadataCount)
}
