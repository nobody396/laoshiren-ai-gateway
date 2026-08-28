package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/apicompat"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/bozhouDev/DragonCode-sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const openAIResponsesViaChatEndpoint = "/v1/chat/completions"

// doOpenAIResponsesViaChatCompletions converts one Responses request to Chat
// Completions, always requests an upstream usage-bearing SSE stream, and wraps
// that stream back into Responses SSE. The caller can either relay the stream
// or buffer its terminal response for a non-streaming client.
func (s *OpenAIGatewayService) doOpenAIResponsesViaChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	responsesBody []byte,
	publicModel string,
) (*http.Response, error) {
	if account == nil || account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("responses-to-chat bridge requires an API-key account")
	}
	if field := responsesBridgeUnsupportedStatefulField(responsesBody); field != "" {
		return nil, fmt.Errorf("%s requires a native Responses upstream", field)
	}

	var responsesReq apicompat.ResponsesRequest
	if err := json.Unmarshal(responsesBody, &responsesReq); err != nil {
		return nil, fmt.Errorf("parse Responses bridge request: %w", err)
	}
	effectiveTools, err := apicompat.EffectiveResponsesTools(&responsesReq)
	if err != nil {
		return nil, fmt.Errorf("resolve Responses bridge tools: %w", err)
	}
	customTools := apicompat.CustomToolNames(effectiveTools)
	functionTools := apicompat.FunctionToolNames(effectiveTools)
	toolSearch := apicompat.HasToolSearchTool(effectiveTools)
	namespaceTools := apicompat.NamespaceToolNames(effectiveTools)

	reasoningScope := reasoningBridgeCacheScope(c, responsesReq.Model)
	s.recacheResponsesBridgeReasoningInput(responsesReq.Input, reasoningScope)
	if err := s.validateResponsesBridgeFailClosed(&responsesReq, effectiveTools, reasoningScope); err != nil {
		return nil, err
	}
	chatReq, err := apicompat.ResponsesToChatCompletionsRequestWithOptions(&responsesReq, &apicompat.ResponsesToChatOptions{
		ReasoningContentByID: func(itemID string) string {
			return s.responsesBridgeReasoningContent(reasoningScope, itemID)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("convert Responses request to Chat Completions: %w", err)
	}
	chatReq.Stream = true
	chatReq.StreamOptions = &apicompat.ChatStreamOptions{IncludeUsage: true}
	chatBody, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal Chat Completions bridge request: %w", err)
	}
	setOpsUpstreamRequestBody(c, chatBody)

	token, _, err := s.getRequestCredential(ctx, c, account)
	if err != nil {
		return nil, err
	}
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	targetURL := buildOpenAIChatCompletionsURL(validatedURL)
	SetActualOpenAIUpstreamEndpoint(c, openAIResponsesViaChatEndpoint)

	upstreamCtx, release := detachStreamUpstreamContext(ctx, true)
	request, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(chatBody))
	if err != nil {
		release()
		return nil, fmt.Errorf("build Chat Completions bridge request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	account.ApplyHeaderOverrides(request.Header)
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			if openaiCCRawAllowedHeaders[strings.ToLower(key)] {
				for _, value := range values {
					request.Header.Add(key, value)
				}
			}
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		request.Header.Set("user-agent", customUA)
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		release()
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	resp.Body = &releaseOnCloseReadCloser{ReadCloser: resp.Body, release: release}
	if resp.StatusCode >= http.StatusBadRequest {
		return resp, nil
	}

	resp.Body = chatCompletionsSSEToResponsesBody(
		resp.Body,
		publicModel,
		customTools,
		functionTools,
		toolSearch,
		namespaceTools,
		func(events []apicompat.ResponsesStreamEvent) {
			s.cacheResponsesBridgeReasoningEvents(reasoningScope, events)
		},
		s.maxOpenAICompatLineSize(),
	)
	resp.Header.Set("Content-Type", "text/event-stream")
	return resp, nil
}

func responsesBridgeUnsupportedStatefulField(body []byte) string {
	for _, field := range []string{"previous_response_id", "conversation", "prompt", "context_management"} {
		value := gjson.GetBytes(body, field)
		if !value.Exists() || value.Type == gjson.Null {
			continue
		}
		if value.Type != gjson.String || strings.TrimSpace(value.String()) != "" {
			return field
		}
	}
	return ""
}

func (s *OpenAIGatewayService) validateResponsesBridgeFailClosed(req *apicompat.ResponsesRequest, tools []apicompat.ResponsesTool, scope ReasoningCacheScope) error {
	for _, tool := range tools {
		switch tool.Type {
		case "function", "custom", "tool_search", "namespace":
			// These have explicit downgrade/restore implementations in apicompat.
		default:
			return fmt.Errorf("responses server tool %q requires a native Responses upstream", tool.Type)
		}
	}

	input := bytes.TrimSpace(req.Input)
	if len(input) == 0 || input[0] != '[' {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return fmt.Errorf("parse Responses input for reasoning validation: %w", err)
	}
	for _, rawItem := range items {
		var item map[string]json.RawMessage
		if err := json.Unmarshal(rawItem, &item); err != nil {
			return fmt.Errorf("parse Responses reasoning item: %w", err)
		}
		var itemType string
		_ = json.Unmarshal(item["type"], &itemType)
		if itemType != "reasoning" {
			continue
		}
		var encrypted, id string
		_ = json.Unmarshal(item["encrypted_content"], &encrypted)
		_ = json.Unmarshal(item["id"], &id)
		if strings.TrimSpace(encrypted) == "" {
			continue
		}
		_, plaintext, _ := apicompat.ExtractResponsesReasoningItem(rawItem)
		if strings.TrimSpace(plaintext) != "" {
			continue
		}
		if strings.TrimSpace(id) == "" || s.responsesBridgeReasoningContent(scope, id) == "" {
			return fmt.Errorf("encrypted reasoning item %q cannot be replayed safely through Chat Completions without cached plaintext", id)
		}
	}
	return nil
}

type releaseOnCloseReadCloser struct {
	io.ReadCloser
	release func()
}

func (r *releaseOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	if r.release != nil {
		r.release()
		r.release = nil
	}
	return err
}

func (s *OpenAIGatewayService) maxOpenAICompatLineSize() int {
	if s != nil && s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		return s.cfg.Gateway.MaxLineSize
	}
	return defaultMaxLineSize
}

func chatCompletionsSSEToResponsesBody(
	source io.ReadCloser,
	model string,
	customTools map[string]bool,
	functionTools map[string]bool,
	toolSearch bool,
	namespaceTools map[string]apicompat.NamespacedToolName,
	onEvents func([]apicompat.ResponsesStreamEvent),
	maxLineSize int,
) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		defer func() { _ = source.Close() }()
		scanner := bufio.NewScanner(source)
		scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
		state := apicompat.NewChatCompletionsToResponsesStreamState(model)
		state.CustomTools = customTools
		state.FunctionTools = functionTools
		state.ToolSearchDeclared = toolSearch
		state.NamespaceTools = namespaceTools
		sawDone := false
		writeEvents := func(events []apicompat.ResponsesStreamEvent) error {
			if onEvents != nil && len(events) > 0 {
				onEvents(events)
			}
			for _, event := range events {
				frame, err := apicompat.ResponsesEventToSSE(event)
				if err != nil {
					return err
				}
				if _, err := io.WriteString(writer, frame); err != nil {
					return err
				}
			}
			return nil
		}
		for scanner.Scan() {
			payload, ok := extractOpenAISSEDataLine(scanner.Text())
			if !ok {
				continue
			}
			if strings.TrimSpace(payload) == "[DONE]" {
				sawDone = true
				break
			}
			var chunk apicompat.ChatCompletionsChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				_ = writer.CloseWithError(fmt.Errorf("decode Chat Completions stream chunk: %w", err))
				return
			}
			if err := writeEvents(apicompat.ChatCompletionsChunkToResponsesEvents(&chunk, state)); err != nil {
				_ = writer.CloseWithError(err)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		if strings.TrimSpace(state.FinishReason) == "" {
			_ = writer.CloseWithError(fmt.Errorf("chat-completions upstream stream ended without finish_reason (done=%v)", sawDone))
			return
		}
		if state.Usage == nil {
			_ = writer.CloseWithError(fmt.Errorf("chat-completions upstream stream ended without usage (done=%v)", sawDone))
			return
		}
		if err := state.ValidateToolCallArguments(); err != nil {
			_ = writer.CloseWithError(fmt.Errorf("invalid tool call arguments from Chat Completions upstream: %w", err))
			return
		}
		if err := writeEvents(apicompat.FinalizeChatCompletionsResponsesStream(state)); err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_, _ = io.WriteString(writer, "data: [DONE]\n\n")
		_ = writer.Close()
	}()
	return reader
}

const responsesBridgeReasoningCacheTTL = 7 * 24 * time.Hour

func reasoningBridgeCacheScope(c *gin.Context, model string) ReasoningCacheScope {
	if c == nil {
		return ReasoningCacheScope{}
	}
	value, exists := c.Get("api_key")
	if !exists {
		return ReasoningCacheScope{}
	}
	apiKey, ok := value.(*APIKey)
	if !ok || apiKey == nil {
		return ReasoningCacheScope{}
	}
	return ReasoningCacheScope{UserID: apiKey.UserID, APIKeyID: apiKey.ID, Model: strings.TrimSpace(model)}
}

func (s *OpenAIGatewayService) responsesBridgeReasoningCache() ReasoningContentCache {
	if s == nil || s.cache == nil {
		return nil
	}
	cache, _ := s.cache.(ReasoningContentCache)
	return cache
}

func (s *OpenAIGatewayService) responsesBridgeReasoningContent(scope ReasoningCacheScope, itemID string) string {
	cache := s.responsesBridgeReasoningCache()
	if cache == nil || !scope.Valid() || strings.TrimSpace(itemID) == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	content, err := cache.GetReasoningContent(ctx, scope, itemID)
	if err != nil {
		return ""
	}
	return content
}

func (s *OpenAIGatewayService) recacheResponsesBridgeReasoningInput(input json.RawMessage, scope ReasoningCacheScope) {
	if !scope.Valid() {
		return
	}
	input = bytes.TrimSpace(input)
	if len(input) == 0 || input[0] != '[' {
		return
	}
	var items []json.RawMessage
	if json.Unmarshal(input, &items) != nil {
		return
	}
	for _, item := range items {
		id, content, ok := apicompat.ExtractResponsesReasoningItem(item)
		if ok && id != "" && content != "" {
			s.setResponsesBridgeReasoningContent(scope, id, content)
		}
	}
}

func (s *OpenAIGatewayService) cacheResponsesBridgeReasoningEvents(scope ReasoningCacheScope, events []apicompat.ResponsesStreamEvent) {
	if !scope.Valid() {
		return
	}
	for _, event := range events {
		if event.Type != "response.output_item.done" || event.Item == nil || event.Item.Type != "reasoning" {
			continue
		}
		var parts []string
		for _, summary := range event.Item.Summary {
			if text := strings.TrimSpace(summary.Text); text != "" {
				parts = append(parts, text)
			}
		}
		if event.Item.ID != "" && len(parts) > 0 {
			s.setResponsesBridgeReasoningContent(scope, event.Item.ID, strings.Join(parts, "\n"))
		}
	}
}

func (s *OpenAIGatewayService) setResponsesBridgeReasoningContent(scope ReasoningCacheScope, itemID, content string) {
	cache := s.responsesBridgeReasoningCache()
	if cache == nil || !scope.Valid() || strings.TrimSpace(itemID) == "" || strings.TrimSpace(content) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cache.SetReasoningContent(ctx, scope, itemID, content, responsesBridgeReasoningCacheTTL); err != nil {
		logger.L().Warn("openai responses-via-chat: cache reasoning content failed", zap.Error(err), zap.String("item_id", itemID))
	}
}

func (s *OpenAIGatewayService) handleResponsesBufferedStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), s.maxOpenAICompatLineSize())
	var finalResponse *apicompat.ResponsesResponse
	var usage OpenAIUsage
	acc := apicompat.NewBufferedResponseAccumulator()
	var parser openAICompatSSEFrameParser

	process := func(payload string) bool {
		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			logger.L().Warn("openai responses-via-chat buffered: failed to parse event", zap.Error(err), zap.String("request_id", requestID))
			return false
		}
		acc.ProcessEvent(&event)
		if isOpenAICompatResponsesTerminalEvent(event.Type) && event.Response != nil {
			finalResponse = event.Response
			if event.Response.Usage != nil {
				usage = OpenAIUsage{
					InputTokens: event.Response.Usage.InputTokens, OutputTokens: event.Response.Usage.OutputTokens,
					CacheCreationInputTokens: event.Response.Usage.CacheCreationInputTokens,
				}
				if event.Response.Usage.InputTokensDetails != nil {
					usage.CacheReadInputTokens = event.Response.Usage.InputTokensDetails.CachedTokens
				}
			}
			return true
		}
		return false
	}
	for scanner.Scan() {
		frame, ok := parser.AddLine(scanner.Text())
		if !ok {
			continue
		}
		if strings.TrimSpace(frame.Data) == "[DONE]" || process(openAICompatPayloadWithEventType(frame.Data, frame.EventType)) {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, s.newOpenAICompatBufferedReadFailoverError(c, account, resp, requestID, err)
	}
	if finalResponse == nil {
		safe := SafeClientUpstreamError(http.StatusBadGateway)
		c.JSON(safe.StatusCode, OpenAIClientErrorEnvelope(c, safe.Type, safe.Message))
		return nil, fmt.Errorf("chat-completions bridge stream ended without terminal event")
	}
	acc.SupplementResponseOutput(finalResponse)
	finalResponse.Model = originalModel
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.JSON(http.StatusOK, finalResponse)
	return &OpenAIForwardResult{
		RequestID: requestID, Usage: usage, Model: originalModel, BillingModel: billingModel,
		UpstreamModel: upstreamModel, UpstreamEndpoint: openAIResponsesViaChatEndpoint,
		Stream: false, Duration: time.Since(startTime),
	}, nil
}
