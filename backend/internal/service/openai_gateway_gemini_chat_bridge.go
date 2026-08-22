package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// forwardGeminiChatCompletions 把 OpenAI Chat Completions 请求透传到 Gemini
// apikey 账号的 OpenAI 兼容上游（{credentials.base_url}/v1/chat/completions）。
// 上游（new-api 系）自行完成 Gemini 协议翻译，网关不做协议转换：仅应用账号
// model_mapping、为流式请求强制 usage 上报，并复用与 openai 账号一致的
// 失败切换（UpstreamFailoverError）、错误透传与 OpenAI 格式用量提取路径。
func (s *OpenAIGatewayService) forwardGeminiChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	originalModel := gjson.GetBytes(body, "model").String()
	if originalModel == "" {
		writeChatCompletionsError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}
	clientStream := gjson.GetBytes(body, "stream").Bool()
	reasoningEffort := extractOpenAIReasoningEffortFromBody(body, originalModel)

	billingModel := resolveOpenAIForwardModel(account, originalModel, defaultMappedModel)
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)

	upstreamBody := body
	if upstreamModel != originalModel {
		upstreamBody = ReplaceModelInBody(body, upstreamModel)
	}
	updatedBody, policyErr := s.applyOpenAIFastPolicyToBody(ctx, account, upstreamModel, upstreamBody)
	if policyErr != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(policyErr, &blocked) {
			writeChatCompletionsError(c, http.StatusForbidden, "permission_error", blocked.Message)
		}
		return nil, policyErr
	}
	upstreamBody = updatedBody
	serviceTier := extractOpenAIServiceTierFromBody(upstreamBody)
	if clientStream {
		var usageErr error
		upstreamBody, usageErr = ensureOpenAIChatStreamUsage(upstreamBody)
		if usageErr != nil {
			return nil, fmt.Errorf("enable stream usage: %w", usageErr)
		}
	}

	logger.L().Debug("gemini chat_completions bridge: forwarding without protocol conversion",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", clientStream),
	)
	if c != nil && strings.TrimSpace(upstreamModel) != "" {
		c.Set("ops_upstream_model", strings.TrimSpace(upstreamModel))
	}

	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return nil, fmt.Errorf("account %d missing api_key credential", account.ID)
	}
	baseURL := account.GetGeminiBaseURL("")
	if baseURL == "" {
		return nil, fmt.Errorf("account %d missing base_url credential", account.ID)
	}
	validatedURL, validateErr := s.validateUpstreamBaseURL(baseURL)
	if validateErr != nil {
		return nil, fmt.Errorf("invalid base_url: %w", validateErr)
	}
	targetURL := buildOpenAIChatCompletionsURL(validatedURL)
	SetActualOpenAIUpstreamEndpoint(c, geminiChatCompletionsEndpoint)

	upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, clientStream)
	defer releaseUpstreamCtx()

	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	account.ApplyHeaderOverrides(upstreamReq.Header)
	if clientStream {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}

	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if openaiCCRawAllowedHeaders[lowerKey] {
			for _, v := range values {
				upstreamReq.Header.Add(key, v)
			}
		}
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Kind:               "request_error",
			Message:            safeErr,
		})
		safeClientErr := SafeClientUpstreamError(http.StatusBadGateway)
		writeChatCompletionsError(c, safeClientErr.StatusCode, safeClientErr.Type, safeClientErr.Message)
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		respBody, upstreamMsg := s.readOpenAIUpstreamError(resp)
		if failoverErr := s.failoverOpenAIUpstreamHTTPError(ctx, c, account, resp, respBody, upstreamMsg, upstreamModel); failoverErr != nil {
			return nil, failoverErr
		}
		return s.handleChatCompletionsErrorResponse(resp, c, account, billingModel)
	}

	var result *OpenAIForwardResult
	if clientStream {
		result, err = s.streamRawChatCompletions(c, resp, account, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime, len(body))
	} else {
		result, err = s.bufferRawChatCompletions(c, resp, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
	}
	if result != nil {
		result.UpstreamEndpoint = geminiChatCompletionsEndpoint
	}
	return result, err
}
