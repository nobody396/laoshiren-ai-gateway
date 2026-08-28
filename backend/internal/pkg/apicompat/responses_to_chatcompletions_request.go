package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResponsesToChatCompletionsRequest converts a stateless Responses request to
// the semantically equivalent Chat Completions request. Only function tools
// are bridgeable; provider-executed Responses tools are rejected instead of
// being silently removed.
func ResponsesToChatCompletionsRequest(req *ResponsesRequest) (*ChatCompletionsRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("responses request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}

	messages, err := responsesInputToChatMessages(req.Instructions, req.Input)
	if err != nil {
		return nil, err
	}
	tools, err := responsesToolsToChatTools(req.Tools)
	if err != nil {
		return nil, err
	}
	toolChoice, err := responsesToolChoiceToChat(req.ToolChoice)
	if err != nil {
		return nil, err
	}
	responseFormat, err := responsesTextFormatToChat(req.Text)
	if err != nil {
		return nil, err
	}

	out := &ChatCompletionsRequest{
		Model:             req.Model,
		Messages:          messages,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		Stream:            req.Stream,
		Tools:             tools,
		ParallelToolCalls: req.ParallelToolCalls,
		ToolChoice:        toolChoice,
		ServiceTier:       req.ServiceTier,
		ResponseFormat:    responseFormat,
	}
	if req.MaxOutputTokens != nil {
		value := *req.MaxOutputTokens
		out.MaxCompletionTokens = &value
	}
	if req.Reasoning != nil {
		out.ReasoningEffort = req.Reasoning.Effort
	}
	return out, nil
}

func responsesInputToChatMessages(instructions string, raw json.RawMessage) ([]ChatMessage, error) {
	messages := make([]ChatMessage, 0, 4)
	if strings.TrimSpace(instructions) != "" {
		content, _ := json.Marshal(instructions)
		messages = append(messages, ChatMessage{Role: "system", Content: content})
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		content, _ := json.Marshal(text)
		return append(messages, ChatMessage{Role: "user", Content: content}), nil
	}

	var items []ResponsesInputItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("input must be a string or an array of input items: %w", err)
	}
	for _, item := range items {
		switch item.Type {
		case "", "message":
			if item.Role != "system" && item.Role != "user" && item.Role != "assistant" {
				return nil, fmt.Errorf("unsupported Responses message role %q", item.Role)
			}
			content, err := responsesContentToChat(item.Content)
			if err != nil {
				return nil, fmt.Errorf("convert %s message content: %w", item.Role, err)
			}
			messages = append(messages, ChatMessage{Role: item.Role, Content: content})
		case "function_call":
			arguments := item.Arguments
			if arguments == "" {
				arguments = "{}"
			}
			messages = append(messages, ChatMessage{
				Role: "assistant",
				ToolCalls: []ChatToolCall{{
					ID:       item.CallID,
					Type:     "function",
					Function: ChatFunctionCall{Name: item.Name, Arguments: arguments},
				}},
			})
		case "function_call_output":
			content, _ := json.Marshal(item.Output)
			messages = append(messages, ChatMessage{Role: "tool", ToolCallID: item.CallID, Content: content})
		case "reasoning":
			return nil, fmt.Errorf("encrypted Responses reasoning cannot be represented by Chat Completions")
		default:
			return nil, fmt.Errorf("unsupported Responses input item type %q", item.Type)
		}
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("input must contain at least one message")
	}
	return messages, nil
}

func responsesContentToChat(raw json.RawMessage) (json.RawMessage, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return json.Marshal(text)
	}
	var parts []ResponsesContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("content must be a string or an array")
	}
	chatParts := make([]ChatContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text", "text":
			chatParts = append(chatParts, ChatContentPart{Type: "text", Text: part.Text})
		case "input_image":
			if strings.TrimSpace(part.ImageURL) == "" {
				return nil, fmt.Errorf("input_image requires image_url")
			}
			chatParts = append(chatParts, ChatContentPart{Type: "image_url", ImageURL: &ChatImageURL{URL: part.ImageURL}})
		case "input_file":
			chatParts = append(chatParts, ChatContentPart{Type: "file", File: &ChatFile{
				Filename: part.Filename, FileData: part.FileData, FileID: part.FileID,
			}})
		default:
			return nil, fmt.Errorf("unsupported Responses content part %q", part.Type)
		}
	}
	return json.Marshal(chatParts)
}

func responsesToolsToChatTools(tools []ResponsesTool) ([]ChatTool, error) {
	out := make([]ChatTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" {
			return nil, fmt.Errorf("Responses tool type %q requires a provider-native endpoint", tool.Type)
		}
		if strings.TrimSpace(tool.Name) == "" {
			return nil, fmt.Errorf("function tool name is required")
		}
		out = append(out, ChatTool{Type: "function", Function: &ChatFunction{
			Name: tool.Name, Description: tool.Description, Parameters: tool.Parameters, Strict: tool.Strict,
		}})
	}
	return out, nil
}

func responsesToolChoiceToChat(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var choice string
	if err := json.Unmarshal(raw, &choice); err == nil {
		switch choice {
		case "auto", "none", "required":
			return json.Marshal(choice)
		default:
			return nil, fmt.Errorf("unsupported tool_choice %q", choice)
		}
	}
	var object struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &object); err != nil || object.Type != "function" || object.Name == "" {
		return nil, fmt.Errorf("unsupported Responses tool_choice")
	}
	return json.Marshal(map[string]any{
		"type":     "function",
		"function": map[string]string{"name": object.Name},
	})
}

func responsesTextFormatToChat(text *ResponsesText) (json.RawMessage, error) {
	if text == nil || len(text.Format) == 0 || strings.TrimSpace(string(text.Format)) == "null" {
		return nil, nil
	}
	var format map[string]json.RawMessage
	if err := json.Unmarshal(text.Format, &format); err != nil {
		return nil, fmt.Errorf("invalid text.format: %w", err)
	}
	var kind string
	if err := json.Unmarshal(format["type"], &kind); err != nil {
		return nil, fmt.Errorf("text.format.type is required")
	}
	switch kind {
	case "text", "json_object":
		return json.Marshal(map[string]string{"type": kind})
	case "json_schema":
		delete(format, "type")
		return json.Marshal(map[string]any{"type": "json_schema", "json_schema": format})
	default:
		return nil, fmt.Errorf("unsupported text.format type %q", kind)
	}
}
