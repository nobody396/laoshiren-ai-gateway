package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/apicompat"
	"github.com/tiktoken-go/tokenizer"
)

const (
	grokResponsesInputItemTokenOverhead = 3
	grokResponsesContentPartOverhead    = 1
	grokInputTokensFallbackMinimum      = 1
)

type grokInputTokensCountRequest struct {
	Model        string                    `json:"model"`
	Instructions string                    `json:"instructions,omitempty"`
	Input        json.RawMessage           `json:"input,omitempty"`
	Tools        []apicompat.ResponsesTool `json:"tools,omitempty"`
	ToolChoice   json.RawMessage           `json:"tool_choice,omitempty"`
}

// EstimateOpenAIResponsesInputTokens estimates the native Responses
// /input_tokens request locally. The endpoint is deliberately account-free and
// cannot create usage, billing, provider penalties, or upstream traffic.
func EstimateOpenAIResponsesInputTokens(body []byte) (int, string, error) {
	var req grokInputTokensCountRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return 0, "", fmt.Errorf("parse responses input_tokens request: %w", err)
	}
	req.Model = strings.TrimSpace(req.Model)
	if req.Model == "" {
		return 0, "", fmt.Errorf("parse responses input_tokens request: model is required")
	}
	estimated, err := estimateGrokInputTokens(req)
	if err != nil {
		return 0, "", fmt.Errorf("estimate responses input_tokens: %w", err)
	}
	if estimated < grokInputTokensFallbackMinimum {
		estimated = grokInputTokensFallbackMinimum
	}
	return estimated, req.Model, nil
}

// EstimateGrokCountTokens estimates an Anthropic-compatible count_tokens
// request locally. xAI does not expose an Anthropic-compatible token-counting
// endpoint, so this deliberately avoids account selection, credentials and
// upstream calls.
func EstimateGrokCountTokens(body []byte) (int, error) {
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return 0, fmt.Errorf("parse anthropic count_tokens request: %w", err)
	}
	if strings.TrimSpace(anthropicReq.Model) == "" {
		return 0, fmt.Errorf("parse anthropic count_tokens request: model is required")
	}

	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return 0, fmt.Errorf("convert anthropic request to responses: %w", err)
	}
	estimated, err := estimateGrokInputTokens(grokInputTokensCountRequest{
		Model:        anthropicReq.Model,
		Instructions: responsesReq.Instructions,
		Input:        responsesReq.Input,
		Tools:        responsesReq.Tools,
		ToolChoice:   responsesReq.ToolChoice,
	})
	if err != nil {
		return 0, fmt.Errorf("estimate grok input tokens: %w", err)
	}
	if estimated < grokInputTokensFallbackMinimum {
		estimated = grokInputTokensFallbackMinimum
	}
	return estimated, nil
}

func estimateGrokInputTokens(req grokInputTokensCountRequest) (int, error) {
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		return 0, err
	}
	total := 0
	addCount := func(text string) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		n, err := codec.Count(text)
		if err != nil {
			return err
		}
		total += n
		return nil
	}
	if err := addCount(req.Instructions); err != nil {
		return 0, err
	}
	inputTokens, err := estimateGrokInput(codec, req.Input)
	if err != nil {
		return 0, err
	}
	total += inputTokens
	for _, tool := range req.Tools {
		raw, err := json.Marshal(tool)
		if err != nil {
			return 0, err
		}
		if err := addCount(string(raw)); err != nil {
			return 0, err
		}
	}
	if len(req.ToolChoice) > 0 {
		compacted, err := compactGrokTokenJSON(req.ToolChoice)
		if err != nil {
			return 0, err
		}
		if err := addCount(compacted); err != nil {
			return 0, err
		}
	}
	return total, nil
}

func estimateGrokInput(codec tokenizer.Codec, raw json.RawMessage) (int, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return 0, nil
	}
	var plainText string
	if err := json.Unmarshal(raw, &plainText); err == nil {
		return codec.Count(plainText)
	}
	var items []apicompat.ResponsesInputItem
	if err := json.Unmarshal(raw, &items); err == nil {
		return estimateGrokInputItems(codec, items)
	}
	compacted, err := compactGrokTokenJSON(raw)
	if err != nil {
		return 0, err
	}
	return codec.Count(compacted)
}

func estimateGrokInputItems(codec tokenizer.Codec, items []apicompat.ResponsesInputItem) (int, error) {
	total := 0
	countText := func(text string) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		n, err := codec.Count(text)
		if err != nil {
			return err
		}
		total += n
		return nil
	}
	for _, item := range items {
		total += grokResponsesInputItemTokenOverhead
		for _, text := range []string{item.Role, item.Type, item.Name, item.Arguments, item.Output, item.CallID, item.ID} {
			if err := countText(text); err != nil {
				return 0, err
			}
		}
		if len(bytes.TrimSpace(item.Content)) == 0 {
			continue
		}
		var contentText string
		if err := json.Unmarshal(item.Content, &contentText); err == nil {
			if err := countText(contentText); err != nil {
				return 0, err
			}
			continue
		}
		var parts []apicompat.ResponsesContentPart
		if err := json.Unmarshal(item.Content, &parts); err == nil {
			for _, part := range parts {
				total += grokResponsesContentPartOverhead
				text := part.Text
				if part.Type == "input_image" {
					text = grokInputImageTokenText(part.ImageURL)
				} else if text == "" {
					text = part.Type
				}
				if err := countText(text); err != nil {
					return 0, err
				}
			}
			continue
		}
		compacted, err := compactGrokTokenJSON(item.Content)
		if err != nil {
			return 0, err
		}
		if err := countText(compacted); err != nil {
			return 0, err
		}
	}
	return total, nil
}

func grokInputImageTokenText(imageURL string) string {
	trimmed := strings.TrimSpace(imageURL)
	if strings.HasPrefix(strings.ToLower(trimmed), "data:") {
		if comma := strings.Index(trimmed, ","); comma > 0 {
			return trimmed[:comma]
		}
	}
	return trimmed
}

func compactGrokTokenJSON(raw json.RawMessage) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return "", err
	}
	return buf.String(), nil
}
