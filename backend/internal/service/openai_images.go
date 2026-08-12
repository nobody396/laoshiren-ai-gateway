package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	openAIImagesGenerationsEndpoint  = "/v1/images/generations"
	openAIImageMaxUploadPartSize     = 20 << 20
	codexNativeImageBridgeModel      = "gpt-5.6-sol"
	codexNativeImageMaxBase64Bytes   = 32 << 20
	codexNativeImageMaxResponseBytes = 48 << 20
	codexNativeImageMaxPixels        = 64 * 1024 * 1024
	openAIRawUpstreamHTTPStatusKey   = "openai_raw_upstream_http_status"
)

var (
	errCodexImageToolNotInvoked = errors.New("image bridge response did not invoke image generation")
	errCodexNativeImageNoOutput = errors.New("native image response did not contain an image")
)

// OpenAIImagesUpload is shared by native OpenAI image edits and Grok media
// normalization. The byte slice is bounded by the request-body limits before
// it reaches either upstream.
type OpenAIImagesUpload struct {
	FieldName   string
	FileName    string
	ContentType string
	Data        []byte
	Width       int
	Height      int
}

func (u OpenAIImagesUpload) ModerationDataURL() string {
	if len(u.Data) == 0 {
		return ""
	}
	contentType := strings.TrimSpace(u.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(u.Data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return ""
	}
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(u.Data))
}

type OpenAIImagesRequest struct {
	Model          string
	Prompt         string
	N              int
	Size           string
	SizeTier       string
	ResponseFormat string
	Body           []byte
}

// CodexNativeImageBridgeModel is the Responses model used for the official
// Codex ImageGen compatibility bridge.
func CodexNativeImageBridgeModel() string { return codexNativeImageBridgeModel }

// ShouldBridgeCodexNativeImageGeneration deliberately recognizes only the
// model emitted by Codex's built-in ImageGen tool. Other gpt-image models keep
// their existing native Images API behavior.
func ShouldBridgeCodexNativeImageGeneration(officialCodex bool, parsed *OpenAIImagesRequest) bool {
	return officialCodex && parsed != nil && strings.EqualFold(strings.TrimSpace(parsed.Model), gptImageOnlyModel)
}

// ValidateCodexNativeImageBridgeRequest rejects shapes that the single-image,
// base64 bridge cannot reproduce without silently changing client semantics.
func ValidateCodexNativeImageBridgeRequest(parsed *OpenAIImagesRequest) error {
	if parsed == nil {
		return fmt.Errorf("parsed image request is required")
	}
	if parsed.N != 1 {
		return fmt.Errorf("codex image generation currently supports n=1")
	}
	if parsed.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	if parsed.ResponseFormat != "" && parsed.ResponseFormat != "b64_json" {
		return fmt.Errorf("codex image generation only supports response_format=b64_json")
	}
	return nil
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

type codexImageBridgeCaptureWriter struct {
	gin.ResponseWriter
	header      http.Header
	body        bytes.Buffer
	status      int
	size        int
	wroteHeader bool
}

func newCodexImageBridgeCaptureWriter(parent gin.ResponseWriter) *codexImageBridgeCaptureWriter {
	return &codexImageBridgeCaptureWriter{ResponseWriter: parent, header: make(http.Header), status: http.StatusOK}
}

func (w *codexImageBridgeCaptureWriter) Header() http.Header { return w.header }
func (w *codexImageBridgeCaptureWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
}
func (w *codexImageBridgeCaptureWriter) WriteHeaderNow() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
}
func (w *codexImageBridgeCaptureWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.body.Write(data)
	w.size += n
	return n, err
}
func (w *codexImageBridgeCaptureWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}
func (w *codexImageBridgeCaptureWriter) Status() int { return w.status }
func (w *codexImageBridgeCaptureWriter) Size() int   { return w.size }
func (w *codexImageBridgeCaptureWriter) Written() bool {
	return w.wroteHeader
}
func (w *codexImageBridgeCaptureWriter) Flush() {}

type codexNativeImagesResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

// ForwardCodexNativeImageGenerationBridge converts the built-in Codex Images
// request into a forced Responses image_generation call. Forward is reused so
// OAuth/API-key transports, model mapping, JSON/SSE normalization, usage
// extraction, and account error classification remain identical to Responses.
func (s *OpenAIGatewayService) ForwardCodexNativeImageGenerationBridge(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
) (*OpenAIForwardResult, error) {
	if err := ValidateCodexNativeImageBridgeRequest(parsed); err != nil {
		return nil, err
	}
	if account == nil || account.Platform != PlatformOpenAI {
		return nil, fmt.Errorf("selected account does not support Codex image generation")
	}

	requestBody, err := json.Marshal(map[string]any{
		"model":       codexNativeImageBridgeModel,
		"input":       parsed.Prompt,
		"tools":       []map[string]any{{"type": "image_generation"}},
		"tool_choice": map[string]any{"type": "image_generation"},
		"stream":      false,
	})
	if err != nil {
		return nil, fmt.Errorf("encode Codex image bridge request: %w", err)
	}

	// Image generation must finish and be billed even when the client closes.
	upstreamCtx, release := detachUpstreamContext(ctx)
	defer release()

	originalWriter := c.Writer
	capture := newCodexImageBridgeCaptureWriter(originalWriter)
	c.Set(upstreamResponseReadLimitContextKey, int64(codexNativeImageMaxResponseBytes))
	c.Set(openAIRawUpstreamHTTPStatusKey, 0)
	var forwardResult *OpenAIForwardResult
	var forwardErr error
	func() {
		c.Writer = capture
		defer func() { c.Writer = originalWriter }()
		forwardResult, forwardErr = s.Forward(upstreamCtx, c, account, requestBody)
	}()
	if forwardErr != nil {
		var failoverErr *UpstreamFailoverError
		if errors.As(forwardErr, &failoverErr) {
			return nil, failoverErr
		}
		// Some API-key passthrough providers return raw 5xx responses that the
		// generic passthrough path preserves instead of classifying for failover.
		// Use the recorded *upstream* status (not the captured local 502) so
		// protocol/content-policy failures are never regenerated on another key.
		if upstreamStatus := c.GetInt(openAIRawUpstreamHTTPStatusKey); upstreamStatus >= http.StatusInternalServerError {
			return nil, &UpstreamFailoverError{StatusCode: upstreamStatus}
		}
		if capture.Written() && capture.body.Len() > 0 {
			copyCodexImageBridgeCapturedResponse(originalWriter, capture)
		} else {
			safeErr := SafeClientUpstreamError(http.StatusBadGateway)
			c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
		}
		return nil, forwardErr
	}
	if forwardResult == nil {
		return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway}
	}

	imageBase64, err := extractCompletedCodexImageResult(capture.body.Bytes())
	if err != nil {
		// A completed response with zero image_generation_call items proves that
		// this account did not generate an image. It is therefore safe to try the
		// next image-capable account without risking duplicate image generation.
		// Any response that contains an image call remains non-failover below,
		// because an incomplete/malformed result may already be billable upstream.
		if errors.Is(err, errCodexImageToolNotInvoked) {
			return nil, &UpstreamFailoverError{
				StatusCode:             http.StatusBadGateway,
				RequestScopedTransient: true,
				Stage:                  GatewayFailureStageInference,
				Scope:                  GatewayFailureScopeRequest,
				Reason:                 GatewayFailureReason("image_tool_not_invoked"),
				NextAccountAction:      NextAccountRetry,
				ClientStatusCode:       http.StatusBadGateway,
				ClientMessage:          "Upstream image generation did not produce an image",
			}
		}
		safeErr := SafeClientUpstreamError(http.StatusBadGateway)
		c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
		return nil, fmt.Errorf("invalid completed image response: %w", err)
	}
	clientResponse := codexNativeImagesResponse{Created: time.Now().Unix()}
	clientResponse.Data = append(clientResponse.Data, struct {
		B64JSON string `json:"b64_json"`
	}{B64JSON: imageBase64})
	responseBody, err := json.Marshal(clientResponse)
	if err != nil {
		return nil, fmt.Errorf("encode Codex Images response: %w", err)
	}
	if requestID := strings.TrimSpace(forwardResult.RequestID); requestID != "" {
		originalWriter.Header().Set("x-request-id", requestID)
	}
	originalWriter.Header().Set("Content-Type", "application/json")
	originalWriter.WriteHeader(http.StatusOK)
	// The upstream completed successfully; retain the result so usage is
	// recorded even when the Images client disconnects during delivery.
	_, _ = originalWriter.Write(responseBody)

	forwardResult.Model = parsed.Model
	forwardResult.BillingModel = codexNativeImageBridgeModel
	forwardResult.ImageCount = 1
	forwardResult.ImageSize = parsed.SizeTier
	forwardResult.UpstreamEndpoint = "/v1/responses"
	return forwardResult, nil
}

// ForwardCodexNativeImageGeneration dispatches the Codex ImageGen request by
// the selected account's image-route protocol. Customer-facing and billing
// models remain stable while the actual upstream model/endpoint are retained
// for diagnostics and account-cost attribution.
func (s *OpenAIGatewayService) ForwardCodexNativeImageGeneration(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
) (*OpenAIForwardResult, error) {
	if err := ValidateCodexNativeImageBridgeRequest(parsed); err != nil {
		return nil, err
	}
	transport, configured := account.OpenAIImageGenerationTransport(parsed.Model)
	if !configured {
		return nil, fmt.Errorf("selected account has no valid Codex image route")
	}
	if transport == OpenAIImageGenerationTransportResponses {
		return s.ForwardCodexNativeImageGenerationBridge(ctx, c, account, parsed)
	}

	body := append([]byte(nil), parsed.Body...)
	if len(body) == 0 {
		var err error
		body, err = json.Marshal(map[string]any{
			"model":           parsed.Model,
			"prompt":          parsed.Prompt,
			"n":               1,
			"size":            parsed.Size,
			"response_format": "b64_json",
		})
		if err != nil {
			return nil, fmt.Errorf("encode native Codex image request: %w", err)
		}
	} else {
		var err error
		body, err = sjson.SetBytes(body, "n", 1)
		if err == nil {
			body, err = sjson.SetBytes(body, "response_format", "b64_json")
		}
		if err != nil {
			return nil, fmt.Errorf("normalize native Codex image request: %w", err)
		}
	}

	// Native Images providers are allowed to fail over only before any image is
	// known to have been generated. Capture and validate the complete response
	// before committing it to the Codex client, mirroring the Responses bridge.
	originalWriter := c.Writer
	capture := newCodexImageBridgeCaptureWriter(originalWriter)
	c.Set(upstreamResponseReadLimitContextKey, int64(codexNativeImageMaxResponseBytes))
	var result *OpenAIForwardResult
	var err error
	func() {
		c.Writer = capture
		defer func() { c.Writer = originalWriter }()
		result, err = s.ForwardImages(ctx, c, account, body, parsed, "")
	}()
	if err != nil {
		var failoverErr *UpstreamFailoverError
		if errors.As(err, &failoverErr) {
			return nil, failoverErr
		}
		if capture.Written() && capture.body.Len() > 0 {
			copyCodexImageBridgeCapturedResponse(originalWriter, capture)
		}
		return nil, err
	}
	if result == nil {
		return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway}
	}
	if err := validateCompletedCodexNativeImagesResponse(capture.body.Bytes()); err != nil {
		if errors.Is(err, errCodexNativeImageNoOutput) {
			return nil, &UpstreamFailoverError{
				StatusCode:             http.StatusBadGateway,
				RequestScopedTransient: true,
				Stage:                  GatewayFailureStageInference,
				Scope:                  GatewayFailureScopeRequest,
				Reason:                 GatewayFailureReason("native_image_no_output"),
				NextAccountAction:      NextAccountRetry,
				ClientStatusCode:       http.StatusBadGateway,
				ClientMessage:          "Upstream image generation did not produce an image",
			}
		}
		safeErr := SafeClientUpstreamError(http.StatusBadGateway)
		c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
		return nil, fmt.Errorf("invalid completed native image response: %w", err)
	}
	copyCodexImageBridgeCapturedResponse(originalWriter, capture)
	if result != nil {
		// A fallback transport must not change the price or model presented to
		// the customer. UpstreamModel still records gpt-image-2-count (or its
		// configured successor), and the account multiplier captures cost.
		result.Model = parsed.Model
		result.BillingModel = codexNativeImageBridgeModel
		result.UpstreamEndpoint = openAIImagesGenerationsEndpoint
	}
	return result, err
}

func validateCompletedCodexNativeImagesResponse(body []byte) error {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return errCodexNativeImageNoOutput
	}
	data := gjson.GetBytes(body, "data")
	if !data.IsArray() || len(data.Array()) == 0 {
		// Any structured error or usage marker means the request may already be
		// deterministic or billable; never replay it on another account.
		if gjson.GetBytes(body, "error").Exists() || gjson.GetBytes(body, "usage").Exists() {
			return fmt.Errorf("native image response contains error or usage without image data")
		}
		return errCodexNativeImageNoOutput
	}
	items := data.Array()
	if len(items) != 1 {
		return fmt.Errorf("native image response contains %d images, expected one", len(items))
	}
	item := items[0]
	imageBase64 := strings.TrimSpace(item.Get("b64_json").String())
	if imageBase64 == "" {
		return fmt.Errorf("native image response is missing b64_json")
	}
	return validateCodexImageBase64(imageBase64)
}

func copyCodexImageBridgeCapturedResponse(dst gin.ResponseWriter, src *codexImageBridgeCaptureWriter) {
	if dst == nil || src == nil {
		return
	}
	for key, values := range src.header {
		for _, value := range values {
			dst.Header().Add(key, value)
		}
	}
	dst.WriteHeader(src.status)
	_, _ = dst.Write(src.body.Bytes())
}

func extractCompletedCodexImageResult(body []byte) (string, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return "", fmt.Errorf("image bridge returned invalid JSON")
	}
	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "status").String()))
	if status != "completed" && status != "done" {
		return "", fmt.Errorf("image bridge response did not complete")
	}
	output := gjson.GetBytes(body, "output")
	if !output.IsArray() {
		return "", fmt.Errorf("image bridge response has no output")
	}
	var result string
	var imageCalls int
	var invalid bool
	output.ForEach(func(_, item gjson.Result) bool {
		if strings.TrimSpace(item.Get("type").String()) != "image_generation_call" {
			return true
		}
		imageCalls++
		if strings.ToLower(strings.TrimSpace(item.Get("status").String())) != "completed" {
			invalid = true
			return false
		}
		value := strings.TrimSpace(item.Get("result").String())
		if value == "" || result != "" {
			invalid = true
			return false
		}
		result = value
		return true
	})
	if imageCalls == 0 && !codexImageBridgeHasImageUsage(body) {
		return "", errCodexImageToolNotInvoked
	}
	if invalid || imageCalls != 1 || result == "" {
		return "", fmt.Errorf("image bridge response has no single completed image result")
	}
	if len(result) > codexNativeImageMaxBase64Bytes {
		return "", fmt.Errorf("image bridge result exceeds maximum size")
	}
	if err := validateCodexImageBase64(result); err != nil {
		return "", err
	}
	return result, nil
}

func validateCodexImageBase64(value string) error {
	if len(value) > codexNativeImageMaxBase64Bytes {
		return fmt.Errorf("image result exceeds maximum size")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil {
		return fmt.Errorf("image result is invalid base64")
	}
	contentType := http.DetectContentType(decoded)
	if contentType != "image/png" && contentType != "image/jpeg" {
		return fmt.Errorf("image result is not a supported image")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(decoded))
	if err != nil || config.Width <= 0 || config.Height <= 0 ||
		int64(config.Width)*int64(config.Height) > codexNativeImageMaxPixels {
		return fmt.Errorf("image result is not decodable")
	}
	return nil
}

func codexImageBridgeHasImageUsage(body []byte) bool {
	// Providers currently report image usage through one of these shapes. Any
	// positive token/count signal, or a provider-specific image tool usage
	// object, means an image may already have been generated and billed; never
	// replay that response on another account.
	for _, path := range []string{
		"tool_usage.image_gen",
		"tool_usage.image_generation",
	} {
		if gjson.GetBytes(body, path).Exists() {
			return true
		}
	}
	for _, path := range []string{
		"usage.output_tokens_details.image_tokens",
		"usage.image_tokens",
		"image_count",
	} {
		if gjson.GetBytes(body, path).Int() > 0 {
			return true
		}
	}
	return false
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
	if c != nil {
		c.Set("ops_upstream_model", strings.TrimSpace(upstreamModel))
	}

	forwardBody, err := sjson.SetBytes(body, "model", upstreamModel)
	if err != nil {
		return nil, fmt.Errorf("rewrite image request model: %w", err)
	}
	// Azure-backed image routes always return base64 payloads but reject the
	// otherwise standard response_format=b64_json field. Strip it only when the
	// selected account explicitly opts in and doing so preserves client
	// semantics; non-base64 formats continue upstream unchanged.
	if account.OmitOpenAIImageGenerationResponseFormat() &&
		(parsed.ResponseFormat == "" || parsed.ResponseFormat == "b64_json") {
		forwardBody, err = sjson.DeleteBytes(forwardBody, "response_format")
		if err != nil {
			return nil, fmt.Errorf("normalize image response format: %w", err)
		}
	}
	setOpsUpstreamRequestBody(c, forwardBody)

	// Image generation can keep consuming upstream resources after the client
	// disconnects. Keep the selected upstream round trip alive so a completed
	// image is still observed and billed; transport timeouts remain the bound.
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()

	token, _, err := s.GetAccessToken(upstreamCtx, account)
	if err != nil {
		return nil, err
	}
	req, err := s.buildOpenAIImagesRequest(upstreamCtx, c, account, forwardBody, token)
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
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	targetURL := buildOpenAIEndpointURL(validatedURL, openAIImagesGenerationsEndpoint)

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
