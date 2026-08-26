package service

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func chatBridgeResponsesRequestForModel(t *testing.T, model string) *apicompat.ResponsesRequest {
	t.Helper()
	chatReq := &apicompat.ChatCompletionsRequest{
		Model: model,
		Messages: []apicompat.ChatMessage{
			{Role: "user", Content: []byte(`"hi"`)},
		},
	}
	responsesReq, err := apicompat.ChatCompletionsToResponses(chatReq)
	require.NoError(t, err)
	responsesReq.Model = model // call site sets the final upstream model id
	stripChatBridgeUnsupportedInclude(responsesReq)
	return responsesReq
}

func TestChatBridgeStripsIncludeForQwenModels(t *testing.T) {
	t.Parallel()

	for _, model := range []string{
		"qwen3.8-max",
		"qwen3.7-max",
		"qwen3.7-plus",
		"qwen3.6-plus",
		"qwen3.6-flash",
		"Qwen3.8-Max", // case-insensitive
	} {
		req := chatBridgeResponsesRequestForModel(t, model)
		require.Nil(t, req.Include, model)
	}
}

func TestChatBridgeKeepsIncludeForOtherFamilies(t *testing.T) {
	t.Parallel()

	for _, model := range []string{
		"gpt-5.5",
		"gpt-5.6-sol",
		"kimi-k3",
		"kimi-k2.7-code",
		"deepseek-v4-flash",
		"glm-5.2",
		"glm-5.3",
	} {
		req := chatBridgeResponsesRequestForModel(t, model)
		require.Equal(t, []string{"reasoning.encrypted_content"}, req.Include, model)
	}
}
