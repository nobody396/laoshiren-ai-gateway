package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractOpenAIUsageMergesHostedImageGenerationTokens(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "non streaming response",
			body: `{"usage":{"input_tokens":10,"output_tokens":5},"tool_usage":{"image_gen":{"output_tokens_details":{"image_tokens":77}}}}`,
		},
		{
			name: "stream terminal response",
			body: `{"type":"response.completed","response":{"usage":{"input_tokens":10,"output_tokens":5},"tool_usage":{"image_gen":{"output_tokens_details":{"image_tokens":77}}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage, ok := extractOpenAIUsageFromJSONBytes([]byte(tt.body))
			require.True(t, ok)
			require.Equal(t, 77, usage.ImageOutputTokens)
		})
	}
}

func TestExtractOpenAIUsageDoesNotOverridePrimaryImageTokens(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":10,"output_tokens":5,"output_tokens_details":{"image_tokens":9}},"tool_usage":{"image_gen":{"output_tokens_details":{"image_tokens":77}}}}`)

	usage, ok := extractOpenAIUsageFromJSONBytes(body)

	require.True(t, ok)
	require.Equal(t, 9, usage.ImageOutputTokens)
}

func TestClassifyAnthropicResponseInputAsCacheRead(t *testing.T) {
	usage := &ClaudeUsage{InputTokens: 100, CacheReadInputTokens: 25, OutputTokens: 3}

	body, err := classifyAnthropicResponseInputAsCacheRead([]byte(`{"usage":{"input_tokens":100,"cache_read_input_tokens":25,"output_tokens":3}}`), usage)

	require.NoError(t, err)
	require.Equal(t, 0, usage.InputTokens)
	require.Equal(t, 125, usage.CacheReadInputTokens)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	usageBody, ok := decoded["usage"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(0), usageBody["input_tokens"])
	require.Equal(t, float64(125), usageBody["cache_read_input_tokens"])
}
