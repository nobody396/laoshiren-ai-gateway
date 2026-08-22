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
	"github.com/tidwall/sjson"
)

const (
	openAIWSClientReadLimitBytesDefault     int64 = 64 * 1024 * 1024
	openAIWSHTTPBridgeThresholdBytesDefault int64 = 15 * 1024 * 1024
	openAIWSHTTPBridgeErrorBodyLimitBytes         = 64 * 1024
)

const openAIWSHTTPBridgeToolStateContextKey = "openai_ws_http_bridge_tool_state"

type openAIWSHTTPBridgeToolState struct {
	ClientMapping apicompat.ResponsesClientToolMapping
	LoweredTools  json.RawMessage
}

func openAIWSHTTPBridgeToolStateFromContext(c *gin.Context) (openAIWSHTTPBridgeToolState, bool) {
	if c == nil {
		return openAIWSHTTPBridgeToolState{}, false
	}
	value, ok := c.Get(openAIWSHTTPBridgeToolStateContextKey)
	state, typed := value.(openAIWSHTTPBridgeToolState)
	return state, ok && typed
}

func setOpenAIWSHTTPBridgeToolState(c *gin.Context, state openAIWSHTTPBridgeToolState) {
	if c == nil {
		return
	}
	state.LoweredTools = append(json.RawMessage(nil), state.LoweredTools...)
	c.Set(openAIWSHTTPBridgeToolStateContextKey, state)
}

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
	items    []json.RawMessage
	seen     map[string]struct{}
	allItems []json.RawMessage
	allSeen  map[string]struct{}
}

func (c *openAIWSToolCallReplayCollector) AddEvent(eventType string, message []byte) {
	switch strings.TrimSpace(eventType) {
	case "response.output_item.done":
		item := gjson.GetBytes(message, "item")
		c.addAllItem(item)
		c.addItem(item)
	case "response.completed", "response.done":
		output := gjson.GetBytes(message, "response.output")
		if !output.IsArray() {
			return
		}
		for _, item := range output.Array() {
			c.addAllItem(item)
			c.addItem(item)
		}
	}
}

func (c *openAIWSToolCallReplayCollector) Items() []json.RawMessage {
	return cloneOpenAIWSRawMessages(c.items)
}

func (c *openAIWSToolCallReplayCollector) AllItems() []json.RawMessage {
	return cloneOpenAIWSRawMessages(c.allItems)
}

func (c *openAIWSToolCallReplayCollector) addAllItem(item gjson.Result) {
	if !item.Exists() || item.Type != gjson.JSON {
		return
	}
	raw := strings.TrimSpace(item.Raw)
	if raw == "" || !strings.HasPrefix(raw, "{") || strings.TrimSpace(item.Get("type").String()) == "" {
		return
	}
	key := strings.TrimSpace(item.Get("id").String())
	if key == "" {
		key = strings.TrimSpace(item.Get("call_id").String())
	}
	if key == "" {
		key = raw
	}
	if c.allSeen == nil {
		c.allSeen = make(map[string]struct{})
	}
	if _, ok := c.allSeen[key]; ok {
		return
	}
	c.allSeen[key] = struct{}{}
	c.allItems = append(c.allItems, json.RawMessage(raw))
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
	var clientToolMapping apicompat.ResponsesClientToolMapping
	if account.Platform == PlatformOpenAI && account.Type == AccountTypeAPIKey {
		inheritedState, _ := openAIWSHTTPBridgeToolStateFromContext(c)
		toolsPresent := gjson.GetBytes(body, "tools").Exists()
		body, clientToolMapping, err = adaptResponsesClientToolsForFunctionUpstreamWithMapping(
			body,
			"OpenAI WS HTTP bridge",
			inheritedState.ClientMapping,
		)
		if err != nil {
			return nil, fmt.Errorf("adapt OpenAI WS HTTP bridge client tools: %w", err)
		}
		loweredTools := inheritedState.LoweredTools
		if toolsPresent {
			loweredTools = json.RawMessage(gjson.GetBytes(body, "tools").Raw)
		} else if len(loweredTools) > 0 {
			body, err = sjson.SetRawBytes(body, "tools", loweredTools)
			if err != nil {
				return nil, fmt.Errorf("inherit OpenAI WS HTTP bridge tools: %w", err)
			}
		}
		setOpenAIWSHTTPBridgeToolState(c, openAIWSHTTPBridgeToolState{
			ClientMapping: clientToolMapping,
			LoweredTools:  loweredTools,
		})
	}

	upstreamCtx, release := detachUpstreamContext(ctx)
	var req *http.Request
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
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		failoverErr := s.failoverOpenAIUpstreamHTTPError(
			ctx,
			c,
			account,
			resp,
			responseBody,
			message,
			upstreamModel,
		)
		if failoverErr != nil && (turn == 1 || resp.StatusCode == http.StatusTooManyRequests) {
			return nil, failoverErr
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
	} else if hasGrokResponsesClientToolMapping(clientToolMapping) {
		resp.Body = newGrokResponsesClientToolStreamBody(resp.Body, clientToolMapping, maxLineSize)
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)
	usage := OpenAIUsage{}
	responseID := ""
	requestID := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id"))
	var firstTokenMs *int
	imageCount := 0
	sawTerminal := false
	sawDone := false
	wroteDownstream := false
	replayCollector := &openAIWSToolCallReplayCollector{}
	resultWithUsage := func() *OpenAIForwardResult {
		effectiveRequestID := requestID
		if account.Platform != PlatformGrok && responseID != "" {
			effectiveRequestID = responseID
		}
		result := &OpenAIForwardResult{
			RequestID: effectiveRequestID, ResponseID: responseID, Usage: usage,
			Model: originalModel, UpstreamModel: upstreamModel,
			BillingModel: imageBillingModel, ImageSize: imageSizeTier, ImageInputSize: imageInputSize,
			ServiceTier: extractOpenAIServiceTierFromBody(body), ReasoningEffort: extractOpenAIReasoningEffortFromBody(body, originalModel),
			Stream: true, OpenAIWSMode: true, ResponseHeaders: resp.Header.Clone(), Duration: time.Since(startedAt), FirstTokenMs: firstTokenMs,
			ImageCount: imageCount,
		}
		if replayInput := replayCollector.Items(); len(replayInput) > 0 {
			result.wsReplayInput = replayInput
			result.wsReplayInputExists = true
		}
		result.wsAccountFailoverReplayInput = replayCollector.AllItems()
		return finalizeOpenAIResponseImageBilling(result)
	}
	for scanner.Scan() {
		data, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(data)
		if trimmed == "" {
			continue
		}
		if trimmed == "[DONE]" {
			sawDone = true
			continue
		}
		if !gjson.Valid(trimmed) {
			continue
		}
		message := []byte(trimmed)
		eventType, eventResponseID, _ := parseOpenAIWSEventEnvelope(message)
		if responseID == "" {
			responseID = eventResponseID
		}
		var upstreamEventErr error
		contentPolicyRejection := false
		if eventType == "error" {
			errCodeRaw, errTypeRaw, errMsgRaw := parseOpenAIWSErrorEventFields(message)
			errMessage := strings.TrimSpace(errMsgRaw)
			if errMessage == "" {
				errMessage = "upstream error event"
			}
			statusCode := openAIWSErrorHTTPStatusFromRaw(errCodeRaw, errTypeRaw)
			contentPolicyRejection = account.Platform == PlatformGrok && isGrokContentPolicyRejection(http.StatusForbidden, message)
			// SSE errors have no HTTP status, so an unknown xAI policy code maps to
			// 502. Keep request-scoped policy refusals out of the account failover
			// helper instead of cooling down or consuming another account.
			if !contentPolicyRejection {
				errorResp := &http.Response{StatusCode: statusCode, Header: resp.Header}
				failoverErr := s.failoverOpenAIUpstreamHTTPError(
					ctx,
					c,
					account,
					errorResp,
					message,
					errMessage,
					upstreamModel,
				)
				if !wroteDownstream && failoverErr != nil && (turn == 1 || statusCode == http.StatusTooManyRequests) {
					return nil, failoverErr
				}
			}
			upstreamEventErr = errors.New(errMessage)
		}
		if firstTokenMs == nil && isOpenAIWSTokenEvent(eventType) {
			ms := int(time.Since(startedAt).Milliseconds())
			firstTokenMs = &ms
		}
		if isOpenAIWSTerminalEvent(eventType) {
			if normalizedMessage, normalized := normalizeCompletedOpenAIResponseImageStatuses(message); normalized > 0 {
				message = normalizedMessage
			}
			if completedImages := countCompletedOpenAIResponseImages(message); completedImages > imageCount {
				imageCount = completedImages
			}
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
		wroteDownstream = true
		if contentPolicyRejection {
			return resultWithUsage(), fmt.Errorf("grok content policy rejection: %s", grokContentPolicyClientMessage(message))
		}
		if upstreamEventErr != nil {
			return resultWithUsage(), upstreamEventErr
		}
		if isOpenAIWSTerminalEvent(eventType) {
			sawTerminal = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		streamErr := fmt.Errorf("read upstream HTTP bridge stream: %w", err)
		if turn == 1 && !wroteDownstream {
			return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, streamErr, true)
		}
		return resultWithUsage(), streamErr
	}
	if !sawTerminal {
		terminalErr := errors.New("upstream HTTP bridge stream ended before terminal event")
		if sawDone {
			terminalErr = errors.New("upstream HTTP bridge stream sent [DONE] before terminal event")
		}
		if turn == 1 && !wroteDownstream {
			return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, terminalErr, true)
		}
		return resultWithUsage(), terminalErr
	}
	return resultWithUsage(), nil
}
