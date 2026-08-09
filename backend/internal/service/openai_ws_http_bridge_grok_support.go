package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	openAIWSClientReadLimitBytesDefault     int64 = 64 * 1024 * 1024
	openAIWSHTTPBridgeThresholdBytesDefault int64 = 15 * 1024 * 1024
	openAIWSHTTPBridgeErrorBodyLimitBytes         = 64 * 1024
)

func ResolveOpenAIWSClientReadLimitBytes(cfg *config.Config) int64 {
	if cfg == nil || cfg.Gateway.OpenAIWS.ClientReadLimitBytes <= 0 {
		return openAIWSClientReadLimitBytesDefault
	}
	return cfg.Gateway.OpenAIWS.ClientReadLimitBytes
}

func (s *OpenAIGatewayService) openAIWSHTTPBridgeEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.HTTPBridgeEnabled
}

func (s *OpenAIGatewayService) openAIWSHTTPBridgeThresholdBytes() int64 {
	if s == nil || s.cfg == nil || s.cfg.Gateway.OpenAIWS.HTTPBridgeThresholdBytes <= 0 {
		return openAIWSHTTPBridgeThresholdBytesDefault
	}
	return s.cfg.Gateway.OpenAIWS.HTTPBridgeThresholdBytes
}

func (s *OpenAIGatewayService) shouldBridgeOpenAIWSHTTP(payloadBytes int, previousResponseID string) bool {
	if !s.openAIWSHTTPBridgeEnabled() {
		return false
	}
	if strings.TrimSpace(previousResponseID) != "" {
		return false
	}
	threshold := s.openAIWSHTTPBridgeThresholdBytes()
	return threshold > 0 && int64(payloadBytes) >= threshold
}

func prepareOpenAIWSHTTPBridgeBody(payload []byte) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	if body == nil {
		return nil, errors.New("response.create payload must be a JSON object")
	}
	delete(body, "type")
	delete(body, "generate")
	delete(body, "previous_response_id")
	body["stream"] = true
	return json.Marshal(body)
}

func buildOpenAIWSHTTPBridgeErrorEvent(statusCode int, message string) []byte {
	message = strings.TrimSpace(message)
	if message == "" {
		message = http.StatusText(statusCode)
	}
	event := map[string]any{
		"type":   "error",
		"status": statusCode,
		"error":  map[string]any{"type": "upstream_error", "message": message},
	}
	body, err := json.Marshal(event)
	if err != nil {
		return []byte(`{"type":"error","error":{"type":"upstream_error","message":"upstream request failed"}}`)
	}
	return body
}

type openAIWSToolCallReplayCollector struct {
	items []json.RawMessage
	seen  map[string]struct{}
}

func (c *openAIWSToolCallReplayCollector) AddEvent(eventType string, message []byte) {
	switch strings.TrimSpace(eventType) {
	case "response.output_item.done":
		c.addItem(gjson.GetBytes(message, "item"))
	case "response.completed", "response.done":
		output := gjson.GetBytes(message, "response.output")
		if !output.IsArray() {
			return
		}
		for _, item := range output.Array() {
			c.addItem(item)
		}
	}
}

func (c *openAIWSToolCallReplayCollector) Items() []json.RawMessage {
	return cloneOpenAIWSRawMessages(c.items)
}

func (c *openAIWSToolCallReplayCollector) addItem(item gjson.Result) {
	if !item.Exists() || item.Type != gjson.JSON {
		return
	}
	raw := strings.TrimSpace(item.Raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return
	}
	if !isCodexToolCallContextItemType(item.Get("type").String()) {
		return
	}
	key := strings.TrimSpace(item.Get("id").String())
	if key == "" {
		key = strings.TrimSpace(item.Get("call_id").String())
	}
	if key == "" {
		key = raw
	}
	if c.seen == nil {
		c.seen = make(map[string]struct{})
	}
	if _, ok := c.seen[key]; ok {
		return
	}
	c.seen[key] = struct{}{}
	c.items = append(c.items, json.RawMessage(raw))
}

// proxyOpenAIWSHTTPBridgeTurn is the callback-oriented bridge used by the WS
// ingress and by protocol tests. Grok always uses the HTTP/SSE Responses
// endpoint; it must never be sent to the OpenAI upstream WebSocket URL.
func (s *OpenAIGatewayService) proxyOpenAIWSHTTPBridgeTurn(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	payload []byte,
	payloadBytes int,
	originalModel string,
	imageBillingModel string,
	imageSizeTier string,
	imageInputSize string,
	grokCacheIdentity string,
	turn int,
	writeClientMessage func([]byte) error,
) (*OpenAIForwardResult, error) {
	if s == nil || s.httpUpstream == nil || account == nil || writeClientMessage == nil {
		return nil, errors.New("openai websocket HTTP bridge is not configured")
	}

	body, err := prepareOpenAIWSHTTPBridgeBody(payload)
	if err != nil {
		return nil, fmt.Errorf("prepare HTTP bridge body: %w", err)
	}

	requestedModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if requestedModel == "" {
		requestedModel = strings.TrimSpace(originalModel)
	}
	upstreamModel := strings.TrimSpace(account.GetMappedModel(requestedModel))
	if upstreamModel == "" {
		upstreamModel = requestedModel
	}
	if account.Platform == PlatformGrok && upstreamModel == "" {
		upstreamModel = grokDefaultResponsesModel
	}

	upstreamCtx, release := detachUpstreamContext(ctx)
	var req *http.Request
	var clientToolMapping apicompat.ResponsesClientToolMapping
	if account.Platform == PlatformGrok {
		intentBody := append([]byte(nil), body...)
		body, clientToolMapping, err = patchGrokResponsesBodyWithClientTools(body, upstreamModel)
		if err == nil {
			setGrokResponsesClientToolMapping(c, clientToolMapping)
			if strings.TrimSpace(grokCacheIdentity) == "" {
				grokCacheIdentity = resolveGrokCacheIdentity(c, body, "", upstreamModel)
			}
			mixedIntentBody := append([]byte(nil), body...)
			body, err = applyGrokResponsesCacheIdentity(body, intentBody, grokCacheIdentity, account.IsGrokOAuth())
			if err == nil {
				body, err = applyGrokFreeRequestToolCacheRoute(c, body, mixedIntentBody, account, grokCacheIdentity)
			}
		}
		if err == nil {
			req, err = buildGrokResponsesRequest(upstreamCtx, c, account, body, token, grokCacheIdentity, s.cfg)
		}
	} else {
		req, err = s.buildUpstreamRequestOpenAIPassthrough(upstreamCtx, c, account, body, token)
	}
	release()
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	if c != nil {
		c.Set("openai_passthrough", true)
		c.Set("openai_ws_http_bridge", true)
		if account.Platform == PlatformGrok {
			SetActualOpenAIUpstreamEndpoint(c, grokChatResponsesEndpoint)
		}
	}
	startedAt := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		if turn == 1 {
			return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, true)
		}
		_ = writeClientMessage(buildOpenAIWSHTTPBridgeErrorEvent(http.StatusBadGateway, "Upstream request failed"))
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, openAIWSHTTPBridgeErrorBodyLimitBytes))
		message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(responseBody)))
		if account.Platform == PlatformGrok {
			s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, responseBody)
			if turn == 1 && s.shouldFailoverGrokUpstreamError(resp.StatusCode, responseBody) {
				return nil, &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: responseBody, ResponseHeaders: resp.Header.Clone()}
			}
		} else if turn == 1 && s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, message, responseBody) {
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, responseBody)
			}
			return nil, &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: responseBody, ResponseHeaders: resp.Header.Clone()}
		}
		_ = writeClientMessage(buildOpenAIWSHTTPBridgeErrorEvent(resp.StatusCode, message))
		return nil, fmt.Errorf("upstream HTTP bridge error: status=%d message=%s", resp.StatusCode, message)
	}
	if account.Platform == PlatformGrok {
		s.updateGrokUsageFromResponse(ctx, account, resp.Header, resp.StatusCode)
	}

	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	if account.Platform == PlatformGrok {
		resp.Body = newGrokResponsesBillingPingFilterBody(resp.Body, account, maxLineSize)
		if hasGrokResponsesClientToolMapping(clientToolMapping) {
			resp.Body = newGrokResponsesClientToolStreamBody(resp.Body, clientToolMapping, maxLineSize)
		}
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)
	usage := OpenAIUsage{}
	responseID := ""
	requestID := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id"))
	var firstTokenMs *int
	sawTerminal := false
	replayCollector := &openAIWSToolCallReplayCollector{}
	for scanner.Scan() {
		data, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(data)
		if trimmed == "" || trimmed == "[DONE]" || !gjson.Valid(trimmed) {
			continue
		}
		message := []byte(trimmed)
		eventType, eventResponseID, _ := parseOpenAIWSEventEnvelope(message)
		if responseID == "" {
			responseID = eventResponseID
		}
		if eventType == "error" {
			statusCode := int(gjson.GetBytes(message, "status").Int())
			if statusCode == 0 {
				statusCode = http.StatusBadGateway
				code := strings.ToLower(gjson.GetBytes(message, "error.code").String() + " " + gjson.GetBytes(message, "error.type").String())
				if strings.Contains(code, "rate_limit") || strings.Contains(code, "too_many") {
					statusCode = http.StatusTooManyRequests
				}
			}
			if account.Platform == PlatformGrok && isGrokContentPolicyRejection(http.StatusForbidden, message) {
				_ = writeClientMessage(message)
				return &OpenAIForwardResult{RequestID: requestID, ResponseID: responseID, Model: originalModel, UpstreamModel: upstreamModel, Stream: true, OpenAIWSMode: true}, fmt.Errorf("grok content policy rejection: %s", grokContentPolicyClientMessage(message))
			}
			if account.Platform == PlatformGrok {
				s.handleGrokAccountUpstreamError(ctx, account, statusCode, resp.Header, message)
			} else if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, statusCode, resp.Header, message)
			}
			return nil, &UpstreamFailoverError{StatusCode: statusCode, ResponseBody: message, ResponseHeaders: resp.Header.Clone()}
		}
		if firstTokenMs == nil && isOpenAIWSTokenEvent(eventType) {
			ms := int(time.Since(startedAt).Milliseconds())
			firstTokenMs = &ms
		}
		if openAIWSEventShouldParseUsage(eventType) {
			parseOpenAIWSResponseUsageFromCompletedEvent(message, &usage)
		}
		if upstreamModel != originalModel && bytes.Contains(message, []byte(upstreamModel)) && openAIWSEventMayContainModel(eventType) {
			message = replaceOpenAIWSMessageModel(message, upstreamModel, originalModel)
		}
		replayCollector.AddEvent(eventType, message)
		if err := writeClientMessage(message); err != nil {
			return nil, err
		}
		if isOpenAIWSTerminalEvent(eventType) {
			sawTerminal = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !sawTerminal {
		return nil, errors.New("upstream HTTP bridge ended without terminal event")
	}
	if account.Platform != PlatformGrok && responseID != "" {
		// OpenAI WS ingress uses the terminal response id as its request id,
		// matching the native WS and upstream http_bridge implementations.
		requestID = responseID
	}
	result := &OpenAIForwardResult{
		RequestID: requestID, ResponseID: responseID, Usage: usage,
		Model: originalModel, UpstreamModel: upstreamModel,
		BillingModel: imageBillingModel, ImageSize: imageSizeTier, ImageInputSize: imageInputSize,
		ServiceTier: extractOpenAIServiceTierFromBody(body), ReasoningEffort: extractOpenAIReasoningEffortFromBody(body, originalModel),
		Stream: true, OpenAIWSMode: true, ResponseHeaders: resp.Header.Clone(), Duration: time.Since(startedAt), FirstTokenMs: firstTokenMs,
	}
	if replayInput := replayCollector.Items(); len(replayInput) > 0 {
		result.wsReplayInput = replayInput
		result.wsReplayInputExists = true
	}
	return result, nil
}
