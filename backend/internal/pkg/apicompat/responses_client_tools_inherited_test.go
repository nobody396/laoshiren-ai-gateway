package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesClientToolIDsFollowRestoredItemTypes(t *testing.T) {
	mapping := ResponsesClientToolMapping{CustomTools: map[string]bool{"exec": true}, ToolSearch: true}
	payload := []byte(`{"output":[{"type":"function_call","id":"fc_custom","call_id":"call_1","name":"exec","arguments":"{\"input\":\"pwd\"}"},{"type":"function_call","id":"fc_search","call_id":"call_2","name":"` + toolSearchProxyName + `","arguments":"{}"}]}`)

	restored, changed, err := RestoreResponsesClientToolPayload(payload, mapping)
	require.NoError(t, err)
	require.True(t, changed)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(restored, &decoded))
	outputs, ok := decoded["output"].([]any)
	require.True(t, ok)
	custom, ok := outputs[0].(map[string]any)
	require.True(t, ok)
	search, ok := outputs[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ctc_custom", custom["id"])
	require.Equal(t, "tsc_search", search["id"])
}

func TestResponsesClientToolHistoryRecoversFunctionIDPrefix(t *testing.T) {
	req := map[string]any{
		"tools": []any{map[string]any{"type": "custom", "name": "exec"}},
		"input": []any{map[string]any{"type": "custom_tool_call", "id": "ctc_history", "call_id": "call_1", "name": "exec", "input": "pwd"}},
	}

	_, changed, err := AdaptResponsesClientTools(req)
	require.NoError(t, err)
	require.True(t, changed)
	items, ok := req["input"].([]any)
	require.True(t, ok)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function_call", item["type"])
	require.Equal(t, "fc_history", item["id"])
}

func TestAdaptResponsesClientToolsWithInheritedMapping_LowersFollowupHistoryWithoutTools(t *testing.T) {
	req := map[string]any{
		"input": []any{
			map[string]any{"type": "custom_tool_call", "name": "exec", "call_id": "call_1", "input": "pwd"},
			map[string]any{"type": "custom_tool_call_output", "id": "ctco_client", "call_id": "call_1", "output": "ok"},
		},
	}
	inherited := ResponsesClientToolMapping{CustomTools: map[string]bool{"exec": true}}

	mapping, changed, err := AdaptResponsesClientToolsWithInheritedMapping(req, inherited)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, inherited, mapping)
	items, ok := req["input"].([]any)
	require.True(t, ok)
	call, ok := items[0].(map[string]any)
	require.True(t, ok)
	output, ok := items[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function_call", call["type"])
	require.Equal(t, "function_call_output", output["type"])
	require.NotContains(t, output, "id")
}

func TestAdaptResponsesClientToolsWithInheritedMapping_ExplicitToolsReplaceInherited(t *testing.T) {
	req := map[string]any{
		"tools": []any{},
		"input": []any{map[string]any{"type": "custom_tool_call", "name": "exec", "input": "pwd"}},
	}

	mapping, changed, err := AdaptResponsesClientToolsWithInheritedMapping(
		req,
		ResponsesClientToolMapping{CustomTools: map[string]bool{"exec": true}},
	)

	require.NoError(t, err)
	require.False(t, changed)
	require.Empty(t, mapping)
	items, ok := req["input"].([]any)
	require.True(t, ok)
	call, ok := items[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "custom_tool_call", call["type"])
}
