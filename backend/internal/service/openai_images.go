package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const openAIImagesGenerationsEndpoint = "/v1/images/generations"

type OpenAIImagesRequest struct {
	Model          string
	Prompt         string
	N              int
	Size           string
	SizeTier       string
	ResponseFormat string
	Body           []byte
}

func (r *OpenAIImagesRequest) StickySessionSeed() string {
	if r == nil {
		return ""
	}
	return strings.Join([]string{
		"openai-images",
		strings.TrimSpace(r.Model),
		strings.TrimSpace(r.SizeTier),
		strings.TrimSpace(r.Prompt),
	}, "|")
}

func (s *OpenAIGatewayService) ParseOpenAIImagesRequest(body []byte) (*OpenAIImagesRequest, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}
	if !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("failed to parse request body")
	}

	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if err := validateOpenAIImagesModel(model); err != nil {
		return nil, err
	}
	if streamResult := gjson.GetBytes(body, "stream"); streamResult.Exists() && streamResult.Bool() {
		return nil, fmt.Errorf("image streaming is not supported")
	}

	n := 1
	if nResult := gjson.GetBytes(body, "n"); nResult.Exists() {
		if nResult.Type != gjson.Number {
			return nil, fmt.Errorf("invalid n field type")
		}
		n = int(nResult.Int())
		if n <= 0 {
			return nil, fmt.Errorf("n must be greater than 0")
		}
	}

	size := strings.TrimSpace(gjson.GetBytes(body, "size").String())
	return &OpenAIImagesRequest{
		Model:          model,
		Prompt:         strings.TrimSpace(gjson.GetBytes(body, "prompt").String()),
		N:              n,
		Size:           size,
		SizeTier:       normalizeOpenAIImageSizeTier(size),
		ResponseFormat: strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "response_format").String())),
		Body:           body,
	}, nil
}

func validateOpenAIImagesModel(model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("images endpoint requires an image model")
	}
	if strings.HasPrefix(strings.ToLower(model), "gpt-image-") {
		return nil
	}
	return fmt.Errorf("images endpoint requires an image model, got %q", model)
}

func normalizeOpenAIImageSizeTier(size string) string {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "1024x1024":
		return "1K"
	case "1536x1024", "1024x1536", "1792x1024", "1024x1792", "", "auto":
		return "2K"
	case "2048x2048", "2048x3072", "3072x2048":
		return "4K"
	default:
		return "2K"
	}
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerForImages(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	workingExcluded := make(map[int64]struct{}, len(excludedIDs))
	for id := range excludedIDs {
		workingExcluded[id] = struct{}{}
	}

	for {
		selection, decision, err := s.SelectAccountWithScheduler(ctx, groupID, "", sessionHash, requestedModel, workingExcluded, OpenAIUpstreamTransportAny)
		if err != nil || selection == nil || selection.Account == nil {
			return selection, decision, err
		}
		if supportsOpenAIImages(selection.Account) {
			return selection, decision, nil
		}
		workingExcluded[selection.Account.ID] = struct{}{}
	}
}

func supportsOpenAIImages(account *Account) bool {
	if account == nil {
		return false
	}
	if account.Platform != PlatformOpenAI {
		return false
	}
	if account.Type != AccountTypeAPIKey {
		return false
	}
	if raw, ok := account.Extra["supports_images"]; ok {
		if allowed, ok := raw.(bool); ok {
			return allowed
		}
	}
	return true
}

func (s *OpenAIGatewayService) ForwardImages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	parsed *OpenAIImagesRequest,
	channelMappedModel string,
) (*OpenAIForwardResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("parsed image request is required")
	}
	if !supportsOpenAIImages(account) {
		return nil, fmt.Errorf("selected account does not support image generation")
	}

	startTime := time.Now()
	requestModel := strings.TrimSpace(parsed.Model)
	if mapped := strings.TrimSpace(channelMappedModel); mapped != "" {
		requestModel = mapped
	}
	if err := validateOpenAIImagesModel(requestModel); err != nil {
		return nil, err
	}

	upstreamModel := account.GetMappedModel(requestModel)
	if err := validateOpenAIImagesModel(upstreamModel); err != nil {
		return nil, err
	}

	forwardBody, err := sjson.SetBytes(body, "model", upstreamModel)
	if err != nil {
		return nil, fmt.Errorf("rewrite image request model: %w", err)
	}
	setOpsUpstreamRequestBody(c, forwardBody)

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	req, err := s.buildOpenAIImagesRequest(ctx, c, account, forwardBody, token)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
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
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			}
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && isPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		return s.handleErrorResponse(ctx, resp, c, account, forwardBody)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)

	usage, _ := extractOpenAIUsageFromJSONBytes(respBody)
	imageCount := parsed.N
	if extracted := extractOpenAIImageCountFromJSONBytes(respBody); extracted > 0 {
		imageCount = extracted
	}

	return &OpenAIForwardResult{
		RequestID:       resp.Header.Get("x-request-id"),
		Usage:           usage,
		Model:           requestModel,
		UpstreamModel:   upstreamModel,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		ImageCount:      imageCount,
		ImageSize:       parsed.SizeTier,
	}, nil
}

func (s *OpenAIGatewayService) buildOpenAIImagesRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
) (*http.Request, error) {
	targetURL := openAIImagesGenerationsEndpoint
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(strings.TrimRight(validatedURL, "/"), "/v1") {
		targetURL = strings.TrimRight(validatedURL, "/") + "/images/generations"
	} else {
		targetURL = strings.TrimRight(validatedURL, "/") + openAIImagesGenerationsEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	for key, values := range c.Request.Header {
		if !openaiPassthroughAllowedHeaders[strings.ToLower(key)] {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		req.Header.Set("User-Agent", customUA)
	}
	return req, nil
}

func extractOpenAIImageCountFromJSONBytes(body []byte) int {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return 0
	}
	data := gjson.GetBytes(body, "data")
	if data.Exists() && data.IsArray() {
		return len(data.Array())
	}
	return 0
}
