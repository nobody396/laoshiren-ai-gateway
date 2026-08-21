package apicompat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
	items := req["input"].([]any)
	require.Equal(t, "function_call", items[0].(map[string]any)["type"])
	require.Equal(t, "function_call_output", items[1].(map[string]any)["type"])
	require.NotContains(t, items[1].(map[string]any), "id")
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
	require.Equal(t, "custom_tool_call", req["input"].([]any)[0].(map[string]any)["type"])
}
