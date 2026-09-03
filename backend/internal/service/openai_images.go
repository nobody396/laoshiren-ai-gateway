package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/bozhouDev/DragonCode-sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
	_ "golang.org/x/image/webp"
)

const (
	openAIImagesGenerationsEndpoint        = "/v1/images/generations"
	openAIImagesEditsEndpoint              = "/v1/images/edits"
	openAIImageMaxUploadPartSize           = 50 << 20
	openAIImagesMaxInputImages             = 16
	openAIImagesMaxPromptRunes             = 32000
	codexNativeImageBridgeModel            = "gpt-5.6-sol"
	codexNativeImageMaxBase64Bytes         = 32 << 20
	codexNativeImageMaxResponseBytes       = 48 << 20
	codexNativeImageMaxPixels              = 64 * 1024 * 1024
	openAIRawUpstreamHTTPStatusKey         = "openai_raw_upstream_http_status"
	openAIImagesVerbatimPromptInstructions = "When invoking the image_generation tool, use the user's image prompt verbatim. Do not rewrite, expand, summarize, embellish, translate, normalize punctuation, or add or remove visual details or constraints. Preserve the original language, wording, capitalization, quotes, and punctuation exactly."
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
	Format      string
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
	Endpoint          string
	ContentType       string
	Multipart         bool
	Model             string
	Prompt            string
	N                 int
	Size              string
	SizeTier          string
	ResponseFormat    string
	OutputCompression *int
	PartialImages     *int
	InputImageURLs    []string
	MaskImageURL      string
	Uploads           []OpenAIImagesUpload
	MaskUpload        *OpenAIImagesUpload
	Body              []byte
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
		strings.TrimSpace(r.Endpoint),
		strings.TrimSpace(r.Model),
		strings.TrimSpace(r.SizeTier),
		strings.TrimSpace(r.Prompt),
	}, "|")
}

func (r *OpenAIImagesRequest) IsEdits() bool {
	return r != nil && r.Endpoint == openAIImagesEditsEndpoint
}

func (s *OpenAIGatewayService) ParseOpenAIImagesRequest(body []byte) (*OpenAIImagesRequest, error) {
	return s.parseOpenAIImagesRequest(openAIImagesGenerationsEndpoint, "application/json", body)
}

func (s *OpenAIGatewayService) ParseOpenAIImagesHTTPRequest(c *gin.Context, body []byte) (*OpenAIImagesRequest, error) {
	if c == nil || c.Request == nil {
		return nil, fmt.Errorf("missing request context")
	}
	endpoint := normalizeOpenAIImagesEndpointPath(c.Request.URL.Path)
	if endpoint == "" {
		return nil, fmt.Errorf("unsupported images endpoint")
	}
	return s.parseOpenAIImagesRequest(endpoint, c.GetHeader("Content-Type"), body)
}

func (s *OpenAIGatewayService) parseOpenAIImagesRequest(endpoint, contentType string, body []byte) (*OpenAIImagesRequest, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}
	mediaType, _, mediaErr := mime.ParseMediaType(strings.TrimSpace(contentType))
	if mediaErr == nil && strings.EqualFold(mediaType, "multipart/form-data") {
		return parseOpenAIImagesMultipartRequest(endpoint, contentType, body)
	}
	if !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("failed to parse request body")
	}

	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String {
		return nil, fmt.Errorf("invalid model field type")
	}
	model := strings.TrimSpace(modelResult.String())
	if err := validateOpenAIImagesModel(model); err != nil {
		return nil, err
	}
	if streamResult := gjson.GetBytes(body, "stream"); streamResult.Exists() {
		if streamResult.Type != gjson.True && streamResult.Type != gjson.False {
			return nil, fmt.Errorf("invalid stream field type")
		}
		if streamResult.Bool() {
			return nil, fmt.Errorf("image streaming is not supported")
		}
	}

	n := 1
	if nResult := gjson.GetBytes(body, "n"); nResult.Exists() {
		if nResult.Type != gjson.Number {
			return nil, fmt.Errorf("invalid n field type")
		}
		parsedN, parseErr := strconv.ParseInt(strings.TrimSpace(nResult.Raw), 10, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("n must be an integer")
		}
		if parsedN <= 0 {
			return nil, fmt.Errorf("n must be greater than 0")
		}
		if parsedN > 10 {
			return nil, fmt.Errorf("n must be less than or equal to 10")
		}
		n = int(parsedN)
	}

	promptResult := gjson.GetBytes(body, "prompt")
	if promptResult.Exists() && promptResult.Type != gjson.String {
		return nil, fmt.Errorf("invalid prompt field type")
	}
	sizeResult := gjson.GetBytes(body, "size")
	if sizeResult.Exists() && sizeResult.Type != gjson.String {
		return nil, fmt.Errorf("invalid size field type")
	}
	responseFormatResult := gjson.GetBytes(body, "response_format")
	if responseFormatResult.Exists() && responseFormatResult.Type != gjson.String {
		return nil, fmt.Errorf("invalid response_format field type")
	}
	for _, field := range []string{"quality", "background", "output_format", "moderation", "input_fidelity", "style", "user"} {
		value := gjson.GetBytes(body, field)
		if value.Exists() && value.Type != gjson.String {
			return nil, fmt.Errorf("invalid %s field type", field)
		}
	}
	outputCompression, err := parseOpenAIImagesOptionalInteger(body, "output_compression", 0, 100)
	if err != nil {
		return nil, err
	}
	partialImages, err := parseOpenAIImagesOptionalInteger(body, "partial_images", 0, 3)
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(promptResult.String())
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if utf8.RuneCountInString(prompt) > openAIImagesMaxPromptRunes {
		return nil, fmt.Errorf("prompt must be at most %d characters", openAIImagesMaxPromptRunes)
	}

	size := strings.TrimSpace(sizeResult.String())
	request := &OpenAIImagesRequest{
		Endpoint:          endpoint,
		ContentType:       normalizedOpenAIImagesContentType(contentType),
		Model:             model,
		Prompt:            prompt,
		N:                 n,
		Size:              size,
		SizeTier:          normalizeOpenAIImageSizeTier(size),
		ResponseFormat:    strings.ToLower(strings.TrimSpace(responseFormatResult.String())),
		OutputCompression: outputCompression,
		PartialImages:     partialImages,
		Body:              body,
	}
	if request.IsEdits() {
		if err := parseOpenAIImagesJSONEditInputs(body, request); err != nil {
			return nil, err
		}
	}
	return request, nil
}

func parseOpenAIImagesOptionalInteger(body []byte, field string, minValue, maxValue int) (*int, error) {
	value := gjson.GetBytes(body, field)
	if !value.Exists() {
		return nil, nil
	}
	if value.Type != gjson.Number {
		return nil, fmt.Errorf("invalid %s field type", field)
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value.Raw), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("%s must be an integer", field)
	}
	if parsed < int64(minValue) || parsed > int64(maxValue) {
		return nil, fmt.Errorf("%s must be between %d and %d", field, minValue, maxValue)
	}
	result := int(parsed)
	return &result, nil
}

func parseOpenAIImagesJSONEditInputs(body []byte, req *OpenAIImagesRequest) error {
	images := gjson.GetBytes(body, "images")
	if !images.Exists() || !images.IsArray() {
		return fmt.Errorf("images must be an array")
	}
	items := images.Array()
	if len(items) == 0 {
		return fmt.Errorf("at least one image is required")
	}
	if len(items) > openAIImagesMaxInputImages {
		return fmt.Errorf("at most %d images are supported", openAIImagesMaxInputImages)
	}
	for index, item := range items {
		if item.Get("file_id").Exists() {
			return fmt.Errorf("images[%d].file_id is not supported; use image_url instead", index)
		}
		imageURL := strings.TrimSpace(item.Get("image_url").String())
		if imageURL == "" {
			return fmt.Errorf("images[%d].image_url is required", index)
		}
		if !validOpenAIImagesInputURL(imageURL) {
			return fmt.Errorf("images[%d].image_url must be an http(s) URL or image data URL", index)
		}
		req.InputImageURLs = append(req.InputImageURLs, imageURL)
	}
	mask := gjson.GetBytes(body, "mask")
	if mask.Exists() {
		if mask.Get("file_id").Exists() {
			return fmt.Errorf("mask.file_id is not supported; use image_url instead")
		}
		maskURL := strings.TrimSpace(mask.Get("image_url").String())
		if maskURL == "" {
			return fmt.Errorf("mask.image_url is required")
		}
		if !validOpenAIImagesInputURL(maskURL) {
			return fmt.Errorf("mask.image_url must be an http(s) URL or image data URL")
		}
		req.MaskImageURL = maskURL
	}
	return nil
}

func validOpenAIImagesInputURL(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "data:image/")
}

func parseOpenAIImagesMultipartRequest(endpoint, contentType string, body []byte) (*OpenAIImagesRequest, error) {
	_, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return nil, fmt.Errorf("invalid multipart content-type: %w", err)
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, fmt.Errorf("multipart boundary is required")
	}
	req := &OpenAIImagesRequest{
		Endpoint:    endpoint,
		ContentType: contentType,
		Multipart:   true,
		N:           1,
		Body:        body,
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return nil, fmt.Errorf("read multipart body: %w", nextErr)
		}
		name := strings.TrimSpace(part.FormName())
		if name == "" {
			_ = part.Close()
			continue
		}
		data, readErr := readOpenAIImagesMultipartPart(part, name)
		fileName := strings.TrimSpace(part.FileName())
		partContentType := strings.TrimSpace(part.Header.Get("Content-Type"))
		_ = part.Close()
		if readErr != nil {
			return nil, readErr
		}
		if fileName != "" {
			upload := OpenAIImagesUpload{
				FieldName: name, FileName: fileName, ContentType: partContentType, Data: data,
			}
			if name == "mask" {
				req.MaskUpload = &upload
				continue
			}
			if name == "image" || name == "image[]" || strings.HasPrefix(name, "image[") {
				req.Uploads = append(req.Uploads, upload)
			}
			continue
		}
		value := strings.TrimSpace(string(data))
		switch name {
		case "model":
			req.Model = value
		case "prompt":
			req.Prompt = value
		case "size":
			req.Size = value
		case "response_format":
			req.ResponseFormat = strings.ToLower(value)
		case "n":
			parsedN, parseErr := strconv.Atoi(value)
			if parseErr != nil || parsedN <= 0 || parsedN > 10 {
				return nil, fmt.Errorf("n must be between 1 and 10")
			}
			req.N = parsedN
		case "stream":
			stream, parseErr := strconv.ParseBool(value)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid stream field value")
			}
			if stream {
				return nil, fmt.Errorf("image streaming is not supported")
			}
		case "output_compression":
			parsedValue, parseErr := strconv.Atoi(value)
			if parseErr != nil || parsedValue < 0 || parsedValue > 100 {
				return nil, fmt.Errorf("output_compression must be between 0 and 100")
			}
			req.OutputCompression = &parsedValue
		case "partial_images":
			parsedValue, parseErr := strconv.Atoi(value)
			if parseErr != nil || parsedValue < 0 || parsedValue > 3 {
				return nil, fmt.Errorf("partial_images must be between 0 and 3")
			}
			req.PartialImages = &parsedValue
		}
	}
	if err := validateOpenAIImagesModel(req.Model); err != nil {
		return nil, err
	}
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if utf8.RuneCountInString(req.Prompt) > openAIImagesMaxPromptRunes {
		return nil, fmt.Errorf("prompt must be at most %d characters", openAIImagesMaxPromptRunes)
	}
	if req.IsEdits() && len(req.Uploads) == 0 {
		return nil, fmt.Errorf("image file is required")
	}
	if len(req.Uploads) > openAIImagesMaxInputImages {
		return nil, fmt.Errorf("at most %d images are supported", openAIImagesMaxInputImages)
	}
	for index := range req.Uploads {
		if err := validateOpenAIImagesUpload(&req.Uploads[index]); err != nil {
			return nil, fmt.Errorf("invalid image %d: %w", index+1, err)
		}
	}
	if req.MaskUpload != nil {
		if err := validateOpenAIImagesUpload(req.MaskUpload); err != nil {
			return nil, fmt.Errorf("invalid mask: %w", err)
		}
		if len(req.Uploads) > 0 {
			first := req.Uploads[0]
			if req.MaskUpload.Format != first.Format || req.MaskUpload.Width != first.Width || req.MaskUpload.Height != first.Height {
				return nil, fmt.Errorf("mask must match the first image format and dimensions")
			}
		}
	}
	req.SizeTier = normalizeOpenAIImageSizeTier(req.Size)
	return req, nil
}

func readOpenAIImagesMultipartPart(part *multipart.Part, fieldName string) ([]byte, error) {
	return readOpenAIImagesMultipartPartWithLimit(part, fieldName, openAIImageMaxUploadPartSize)
}

func readOpenAIImagesMultipartPartWithLimit(part *multipart.Part, fieldName string, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(part, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read multipart field %s: %w", fieldName, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("multipart field %s exceeds %d bytes: %w", fieldName, limit, &http.MaxBytesError{Limit: limit})
	}
	return data, nil
}

func validateOpenAIImagesUpload(upload *OpenAIImagesUpload) error {
	if upload == nil || len(upload.Data) == 0 {
		return fmt.Errorf("file is empty")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(upload.Data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return fmt.Errorf("file is not a supported decodable image")
	}
	detectedType := http.DetectContentType(upload.Data)
	if !strings.HasPrefix(strings.ToLower(detectedType), "image/") {
		return fmt.Errorf("file is not an image")
	}
	upload.ContentType = detectedType
	upload.Width = config.Width
	upload.Height = config.Height
	upload.Format = strings.ToLower(format)
	return nil
}

func normalizedOpenAIImagesContentType(contentType string) string {
	if strings.TrimSpace(contentType) == "" {
		return "application/json"
	}
	return contentType
}

func normalizeOpenAIImagesEndpointPath(path string) string {
	switch {
	case strings.Contains(strings.TrimSpace(path), "/images/edits"):
		return openAIImagesEditsEndpoint
	case strings.Contains(strings.TrimSpace(path), "/images/generations"):
		return openAIImagesGenerationsEndpoint
	default:
		return ""
	}
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
	endpoint string,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if endpoint != openAIImagesEditsEndpoint {
		endpoint = openAIImagesGenerationsEndpoint
	}
	workingExcluded := make(map[int64]struct{}, len(excludedIDs))
	for id := range excludedIDs {
		workingExcluded[id] = struct{}{}
	}

	for {
		selection, decision, err := s.selectAccountWithSchedulerForRouting(
			ctx, groupID, "", "", requestedModel, workingExcluded,
			OpenAIUpstreamTransportAny, false, true, endpoint,
		)
		if err != nil || selection == nil || selection.Account == nil {
			return selection, decision, err
		}
		if supportsOpenAIImagesEndpoint(selection.Account, endpoint) {
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

func supportsOpenAIImagesEndpoint(account *Account, endpoint string) bool {
	if !supportsOpenAIImages(account) {
		return false
	}
	if endpoint != openAIImagesEditsEndpoint {
		return true
	}
	if raw, exists := account.Extra["supports_image_edits"]; exists {
		allowed, ok := raw.(bool)
		return ok && allowed
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

type codexExecRenderedImage struct {
	MediaType string
	B64JSON   string
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
		"model":        codexNativeImageBridgeModel,
		"instructions": openAIImagesVerbatimPromptInstructions,
		"input":        parsed.Prompt,
		"tools":        []map[string]any{{"type": "image_generation"}},
		"tool_choice":  map[string]any{"type": "image_generation"},
		"stream":       false,
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
	if codexImageBridgeUsesCompletedOuterGeneratingInner(capture.body.Bytes()) {
		logger.FromContext(ctx).Warn(
			"openai.codex_image_bridge_normalized_stale_inner_status",
			zap.Int64("account_id", account.ID),
			zap.String("upstream_request_id", strings.TrimSpace(forwardResult.RequestID)),
			zap.String("inner_status", "generating"),
		)
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

// ForwardFixedOpenAIImageGenerationResponses renders a generation-only Codex
// Responses request through the dedicated gpt-image-2 pool, then delivers the
// validated bytes through Codex Desktop's code-mode generatedImage helper. The
// entire upstream response is buffered before any client bytes are written,
// preserving safe sequential failover and preventing duplicate paid images.
func (s *OpenAIGatewayService) ForwardFixedOpenAIImageGenerationResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	responsesBody []byte,
	requestedModel string,
	stream bool,
) (*OpenAIForwardResult, error) {
	if c == nil || c.Writer == nil {
		return nil, fmt.Errorf("response context is required")
	}
	prompt, err := LatestOpenAIUserPromptForImageRenderer(responsesBody)
	if err != nil {
		return nil, err
	}

	imageBody, err := json.Marshal(map[string]any{
		"model":           OpenAIFixedImageRendererModel,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
	})
	if err != nil {
		return nil, fmt.Errorf("encode fixed image request: %w", err)
	}
	parsed := &OpenAIImagesRequest{
		Model:          OpenAIFixedImageRendererModel,
		Prompt:         prompt,
		N:              1,
		SizeTier:       ImageBillingSize2K,
		ResponseFormat: "b64_json",
		Body:           imageBody,
	}

	originalWriter := c.Writer
	capture := newCodexImageBridgeCaptureWriter(originalWriter)
	var result *OpenAIForwardResult
	func() {
		c.Writer = capture
		defer func() { c.Writer = originalWriter }()
		result, err = s.ForwardCodexNativeImageGeneration(ctx, c, account, parsed)
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

	imageBase64, err := extractCompletedCodexNativeImage(capture.body.Bytes())
	if err != nil {
		// ForwardCodexNativeImageGeneration already validated this payload. Treat
		// any disagreement as non-replayable because the upstream may have billed
		// it, and never expose malformed image data to Codex.
		safeErr := SafeClientUpstreamError(http.StatusBadGateway)
		c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
		return nil, fmt.Errorf("extract validated fixed image response: %w", err)
	}

	responseID := "resp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	createdAt := time.Now().Unix()
	completedResponse := map[string]any{
		"id":                   responseID,
		"object":               "response",
		"created_at":           createdAt,
		"completed_at":         createdAt,
		"status":               "completed",
		"error":                nil,
		"incomplete_details":   nil,
		"instructions":         nil,
		"model":                strings.TrimSpace(requestedModel),
		"output":               []any{},
		"parallel_tool_calls":  true,
		"previous_response_id": nil,
		"store":                false,
		"temperature":          1,
		"text": map[string]any{
			"format": map[string]any{"type": "text"},
		},
		"tool_choice": "auto",
		"tools":       []any{},
		"top_p":       1,
		"truncation":  "disabled",
		// The renderer has no trustworthy token usage for the synthetic
		// Responses envelope. Response.usage is nullable; omitting invented
		// token details also keeps strict SDK decoders protocol-compatible.
		"usage":    nil,
		"metadata": map[string]any{},
	}
	if strings.TrimSpace(requestedModel) == "" {
		completedResponse["model"] = OpenAIFixedImageRendererModel
	}

	mediaType, renderErr := openAIImageBase64MediaType(imageBase64)
	if renderErr == nil {
		// Validate and serialize the fixed data only. No user prompt, remote URL
		// or user-controlled JavaScript is included in the exec payload.
		_, renderErr = codexExecGeneratedImageInput([]codexExecRenderedImage{{
			MediaType: mediaType,
			B64JSON:   imageBase64,
		}})
	}
	if renderErr != nil {
		// The upstream image is already complete and may be billable. Do not
		// replay it on another provider. Return a terminal text result so the
		// client receives an honest failure without duplicate generation.
		logger.FromContext(ctx).Error("openai.codex_generated_image_conversion_failed", zap.Error(renderErr))
		failedItemID := "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		failedText := "图片已经生成，但客户端图片转换失败。为避免重复生成和重复扣费，本次不会自动重试。"
		failedItem := completedOpenAIAssistantTextItem(failedItemID, failedText)
		completedResponse["output"] = []any{failedItem}
		renderErr = writeFixedOpenAIAssistantTextResponses(c, stream, completedResponse, failedItem, failedText)
	} else {
		renderErr = writeCodexExecRenderedImageResponses(c, stream, completedResponse, []codexExecRenderedImage{{
			MediaType: mediaType,
			B64JSON:   imageBase64,
		}}, s.codexGeneratedImageAckSigningSecret())
	}
	if renderErr != nil {
		// The image is already complete and billable upstream. Preserve the
		// successful result even if the downstream client disconnected while the
		// buffered native response was being delivered.
		logger.FromContext(ctx).Warn("openai.fixed_image_response_delivery_failed", zap.Error(renderErr))
	}

	result.ResponseID = responseID
	if strings.TrimSpace(result.RequestID) == "" {
		result.RequestID = responseID
	}
	result.Model = strings.TrimSpace(requestedModel)
	if result.Model == "" {
		result.Model = OpenAIFixedImageRendererModel
	}
	result.BillingModel = OpenAIFixedImageRendererModel
	result.ImageCount = 1
	result.ImageSize = ImageBillingSize2K
	result.Stream = stream
	return result, nil
}

// ForwardNativeOpenAIImageGenerationResponses preserves a Responses-capable
// image provider's native protocol instead of converting it through the Images
// adapter. Streaming requests keep the provider's real partial/tool events. A
// non-streaming response is buffered long enough to allow a safe fallback only
// when the forced image tool was not invoked at all.
func (s *OpenAIGatewayService) ForwardNativeOpenAIImageGenerationResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	stream bool,
) (*OpenAIForwardResult, error) {
	if c == nil || c.Writer == nil {
		return nil, fmt.Errorf("response context is required")
	}
	if account == nil {
		return nil, fmt.Errorf("image account is required")
	}

	finalize := func(result *OpenAIForwardResult) *OpenAIForwardResult {
		if result == nil {
			return nil
		}
		if result.ImageCount > 0 {
			result.BillingModel = OpenAIFixedImageRendererModel
			if strings.TrimSpace(result.ImageSize) == "" {
				result.ImageSize = ImageBillingSize2K
			}
		}
		result.UpstreamEndpoint = "/v1/responses"
		return result
	}

	renderViaExec := HasOpenAICodexExecImageRenderTool(body)
	if stream && !renderViaExec {
		result, err := s.Forward(ctx, c, account, body)
		return finalize(result), err
	}
	upstreamBody := body
	if renderViaExec && stream {
		var err error
		upstreamBody, err = sjson.SetBytes(body, "stream", false)
		if err != nil {
			return nil, fmt.Errorf("buffer Codex image response for rendering: %w", err)
		}
	}

	originalWriter := c.Writer
	capture := newCodexImageBridgeCaptureWriter(originalWriter)
	var result *OpenAIForwardResult
	var err error
	func() {
		c.Writer = capture
		defer func() { c.Writer = originalWriter }()
		result, err = s.Forward(ctx, c, account, upstreamBody)
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
	imageCalls, validImages := completedOpenAIResponseImageCallStats(capture.body.Bytes())
	if result.ImageCount <= 0 || imageCalls != validImages || validImages != result.ImageCount {
		if imageCalls == 0 &&
			!codexImageBridgeHasImageUsage(capture.body.Bytes()) {
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
		return nil, fmt.Errorf(
			"native Responses image output was incomplete or malformed: calls=%d valid_images=%d accounted_images=%d",
			imageCalls, validImages, result.ImageCount,
		)
	}

	if renderViaExec {
		images, extractErr := extractCompletedOpenAIResponseImages(capture.body.Bytes())
		if extractErr != nil {
			safeErr := SafeClientUpstreamError(http.StatusBadGateway)
			c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
			return nil, fmt.Errorf("render completed Codex image response: %w", extractErr)
		}
		completedResponse, decodeErr := decodeCompletedOpenAIResponse(capture.body.Bytes())
		if decodeErr != nil {
			safeErr := SafeClientUpstreamError(http.StatusBadGateway)
			c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))
			return nil, decodeErr
		}
		if deliveryErr := writeCodexExecRenderedImageResponses(c, stream, completedResponse, images, s.codexGeneratedImageAckSigningSecret()); deliveryErr != nil {
			logger.FromContext(ctx).Warn("openai.codex_exec_image_delivery_failed", zap.Error(deliveryErr))
		}
		result.Stream = stream
		return finalize(result), nil
	}

	copyCodexImageBridgeCapturedResponse(originalWriter, capture)
	return finalize(result), nil
}

func completedOpenAIResponseImageCallStats(body []byte) (calls int, valid int) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return 0, 0
	}
	response := gjson.ParseBytes(body)
	if nested := response.Get("response"); nested.Exists() && nested.IsObject() {
		response = nested
	}
	for _, item := range response.Get("output").Array() {
		if strings.TrimSpace(item.Get("type").String()) != "image_generation_call" {
			continue
		}
		calls++
		itemStatus := strings.ToLower(strings.TrimSpace(item.Get("status").String()))
		if itemStatus != "completed" && itemStatus != "generating" {
			continue
		}
		if result := strings.TrimSpace(item.Get("result").String()); result != "" && validateOpenAIImageBase64(result) == nil {
			valid++
		}
	}
	return calls, valid
}

func extractCompletedOpenAIResponseImages(body []byte) ([]codexExecRenderedImage, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("completed image response is invalid JSON")
	}
	response := gjson.ParseBytes(body)
	if nested := response.Get("response"); nested.Exists() && nested.IsObject() {
		response = nested
	}
	status := strings.ToLower(strings.TrimSpace(response.Get("status").String()))
	if status != "completed" && status != "done" {
		return nil, fmt.Errorf("image response did not complete")
	}
	images := make([]codexExecRenderedImage, 0, 1)
	imageCalls := 0
	for _, item := range response.Get("output").Array() {
		if strings.TrimSpace(item.Get("type").String()) != "image_generation_call" {
			continue
		}
		imageCalls++
		itemStatus := strings.ToLower(strings.TrimSpace(item.Get("status").String()))
		if itemStatus != "completed" && itemStatus != "generating" {
			return nil, fmt.Errorf("image call is not complete")
		}
		result := strings.TrimSpace(item.Get("result").String())
		mediaType, err := openAIImageBase64MediaType(result)
		if err != nil {
			return nil, err
		}
		images = append(images, codexExecRenderedImage{MediaType: mediaType, B64JSON: result})
	}
	if imageCalls == 0 {
		return nil, errCodexImageToolNotInvoked
	}
	if len(images) != imageCalls {
		return nil, fmt.Errorf("image response contains incomplete results")
	}
	return images, nil
}

func openAIImageBase64MediaType(value string) (string, error) {
	if err := validateOpenAIImageBase64(value); err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("image result is invalid base64")
	}
	switch contentType := http.DetectContentType(decoded); contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return contentType, nil
	default:
		return "", fmt.Errorf("image result is not a supported image")
	}
}

func decodeCompletedOpenAIResponse(body []byte) (map[string]any, error) {
	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode completed image response: %w", err)
	}
	if nested, ok := response["response"].(map[string]any); ok {
		response = nested
	}
	status := strings.ToLower(strings.TrimSpace(firstNonEmptyString(response["status"])))
	if status != "completed" && status != "done" {
		return nil, fmt.Errorf("completed image response has non-terminal status")
	}
	return response, nil
}

func finalizeOpenAIResponseImageBilling(result *OpenAIForwardResult) *OpenAIForwardResult {
	if result == nil || result.ImageCount <= 0 {
		return result
	}
	if strings.TrimSpace(result.BillingModel) == "" {
		result.BillingModel = OpenAIFixedImageRendererModel
	}
	if strings.TrimSpace(result.ImageSize) == "" {
		result.ImageSize = ImageBillingSize2K
	}
	return result
}

func completedOpenAIAssistantTextItem(itemID string, text string) map[string]any {
	return map[string]any{
		"id":     itemID,
		"type":   "message",
		"status": "completed",
		"role":   "assistant",
		"content": []any{map[string]any{
			"type":        "output_text",
			"text":        text,
			"annotations": []any{},
			"logprobs":    []any{},
		}},
	}
}

// WriteOpenAICodexGeneratedImageAcknowledgement completes the second
// Responses Lite turn after Codex Desktop has already executed generatedImage
// and returned a validated input_image tool output. It deliberately performs
// no upstream request and reports no invented token usage, so the original
// image request remains the only billed generation.
func WriteOpenAICodexGeneratedImageAcknowledgement(c *gin.Context, stream bool, requestedModel string) error {
	responseID := "resp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	itemID := "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	createdAt := time.Now().Unix()
	text := "图片已生成并显示。"
	completedItem := completedOpenAIAssistantTextItem(itemID, text)
	completedResponse := map[string]any{
		"id":                   responseID,
		"object":               "response",
		"created_at":           createdAt,
		"completed_at":         createdAt,
		"status":               "completed",
		"error":                nil,
		"incomplete_details":   nil,
		"instructions":         nil,
		"model":                strings.TrimSpace(requestedModel),
		"output":               []any{completedItem},
		"parallel_tool_calls":  true,
		"previous_response_id": nil,
		"store":                false,
		"temperature":          1,
		"text": map[string]any{
			"format": map[string]any{"type": "text"},
		},
		"tool_choice": "auto",
		"tools":       []any{},
		"top_p":       1,
		"truncation":  "disabled",
		"usage":       nil,
		"metadata":    map[string]any{},
	}
	return writeFixedOpenAIAssistantTextResponses(c, stream, completedResponse, completedItem, text)
}

func writeFixedOpenAIAssistantTextResponses(
	c *gin.Context,
	stream bool,
	completedResponse map[string]any,
	completedItem map[string]any,
	text string,
) error {
	if c == nil || c.Writer == nil {
		return fmt.Errorf("response writer is required")
	}
	requestID := strings.TrimSpace(firstNonEmptyString(completedResponse["id"]))
	if requestID != "" {
		c.Header("x-request-id", requestID)
	}
	if !stream {
		body, err := json.Marshal(completedResponse)
		if err != nil {
			return fmt.Errorf("encode fixed assistant image response: %w", err)
		}
		c.Header("Content-Type", "application/json")
		MarkResponseCommitted(c)
		c.Status(http.StatusOK)
		_, err = c.Writer.Write(body)
		return err
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	inProgressResponse := cloneOpenAIResponseMap(completedResponse)
	inProgressResponse["status"] = "in_progress"
	inProgressResponse["completed_at"] = nil
	inProgressResponse["output"] = []any{}
	inProgressItem := map[string]any{
		"id":      completedItem["id"],
		"type":    "message",
		"status":  "in_progress",
		"role":    "assistant",
		"content": []any{},
	}
	itemID := strings.TrimSpace(firstNonEmptyString(completedItem["id"]))
	inProgressPart := map[string]any{
		"type":        "output_text",
		"text":        "",
		"annotations": []any{},
		"logprobs":    []any{},
	}
	completedPart := map[string]any{
		"type":        "output_text",
		"text":        text,
		"annotations": []any{},
		"logprobs":    []any{},
	}
	events := []map[string]any{
		{"type": "response.created", "sequence_number": 0, "response": inProgressResponse},
		{"type": "response.in_progress", "sequence_number": 1, "response": inProgressResponse},
		{"type": "response.output_item.added", "sequence_number": 2, "output_index": 0, "item": inProgressItem},
		{"type": "response.content_part.added", "sequence_number": 3, "output_index": 0, "item_id": itemID, "content_index": 0, "part": inProgressPart},
		{"type": "response.output_text.delta", "sequence_number": 4, "output_index": 0, "item_id": itemID, "content_index": 0, "delta": text, "logprobs": []any{}},
		{"type": "response.output_text.done", "sequence_number": 5, "output_index": 0, "item_id": itemID, "content_index": 0, "text": text, "logprobs": []any{}},
		{"type": "response.content_part.done", "sequence_number": 6, "output_index": 0, "item_id": itemID, "content_index": 0, "part": completedPart},
		{"type": "response.output_item.done", "sequence_number": 7, "output_index": 0, "item": completedItem},
		{"type": "response.completed", "sequence_number": 8, "response": completedResponse},
	}

	MarkResponseCommitted(c)
	c.Status(http.StatusOK)
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode fixed assistant image stream event: %w", err)
		}
		eventType := strings.TrimSpace(firstNonEmptyString(event["type"]))
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, payload); err != nil {
			return err
		}
	}
	c.Writer.Flush()
	return nil
}

func writeCodexExecRenderedImageResponses(
	c *gin.Context,
	stream bool,
	upstreamResponse map[string]any,
	images []codexExecRenderedImage,
	ackSigningSecret string,
) error {
	if c == nil || c.Writer == nil {
		return fmt.Errorf("response writer is required")
	}
	if len(images) == 0 {
		return fmt.Errorf("at least one rendered image is required")
	}

	execInput, err := codexExecGeneratedImageInput(images)
	if err != nil {
		return err
	}
	response := cloneOpenAIResponseMap(upstreamResponse)
	responseID := strings.TrimSpace(firstNonEmptyString(response["id"]))
	if responseID == "" {
		responseID = "resp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		response["id"] = responseID
	}
	response["status"] = "completed"
	response["error"] = nil
	response["incomplete_details"] = nil
	dataURLs, ok := parseCodexExecGeneratedImageInput(execInput)
	if !ok {
		return fmt.Errorf("validate Codex rendered image exec input")
	}
	callID, err := newCodexGeneratedImageAckCallID(ackSigningSecret, dataURLs)
	if err != nil {
		return err
	}
	itemID := "ctc_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	completedItem := map[string]any{
		"id":      itemID,
		"type":    "custom_tool_call",
		"status":  "completed",
		"call_id": callID,
		"name":    "exec",
		"input":   execInput,
	}
	response["output"] = []any{completedItem}
	c.Header("x-request-id", responseID)

	if !stream {
		body, err := json.Marshal(response)
		if err != nil {
			return fmt.Errorf("encode Codex rendered image response: %w", err)
		}
		c.Header("Content-Type", "application/json")
		MarkResponseCommitted(c)
		c.Status(http.StatusOK)
		_, err = c.Writer.Write(body)
		return err
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	inProgressResponse := cloneOpenAIResponseMap(response)
	inProgressResponse["status"] = "in_progress"
	inProgressResponse["completed_at"] = nil
	inProgressResponse["output"] = []any{}
	inProgressItem := map[string]any{
		"id":      itemID,
		"type":    "custom_tool_call",
		"status":  "in_progress",
		"call_id": callID,
		"name":    "exec",
		"input":   "",
	}
	events := []map[string]any{
		{"type": "response.created", "sequence_number": 0, "response": inProgressResponse},
		{"type": "response.in_progress", "sequence_number": 1, "response": inProgressResponse},
		{"type": "response.output_item.added", "sequence_number": 2, "output_index": 0, "item": inProgressItem},
		{"type": "response.output_item.done", "sequence_number": 3, "output_index": 0, "item": completedItem},
		{"type": "response.completed", "sequence_number": 4, "response": response},
	}

	MarkResponseCommitted(c)
	c.Status(http.StatusOK)
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode Codex rendered image stream event: %w", err)
		}
		eventType := strings.TrimSpace(firstNonEmptyString(event["type"]))
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, payload); err != nil {
			return err
		}
	}
	c.Writer.Flush()
	return nil
}

func (s *OpenAIGatewayService) codexGeneratedImageAckSigningSecret() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	return strings.TrimSpace(s.cfg.JWT.Secret)
}

func codexExecGeneratedImageInput(images []codexExecRenderedImage) (string, error) {
	var input strings.Builder
	for index, image := range images {
		mediaType := strings.ToLower(strings.TrimSpace(image.MediaType))
		switch mediaType {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
		default:
			return "", fmt.Errorf("unsupported rendered image media type %q", image.MediaType)
		}
		if err := validateOpenAIImageBase64(image.B64JSON); err != nil {
			return "", err
		}
		dataURL, err := json.Marshal("data:" + mediaType + ";base64," + image.B64JSON)
		if err != nil {
			return "", fmt.Errorf("encode rendered image data URL: %w", err)
		}
		outputHint, err := json.Marshal(fmt.Sprintf("已生成第 %d 张图片。", index+1))
		if err != nil {
			return "", fmt.Errorf("encode rendered image output hint: %w", err)
		}
		if _, err := fmt.Fprintf(
			&input,
			"generatedImage({image_url:%s,output_hint:%s});\n",
			dataURL,
			outputHint,
		); err != nil {
			return "", fmt.Errorf("build rendered image tool input: %w", err)
		}
	}
	return input.String(), nil
}

func cloneOpenAIResponseMap(src map[string]any) map[string]any {
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func validateCompletedCodexNativeImagesResponse(body []byte) error {
	_, err := extractCompletedCodexNativeImage(body)
	return err
}

func extractCompletedCodexNativeImage(body []byte) (string, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return "", errCodexNativeImageNoOutput
	}
	data := gjson.GetBytes(body, "data")
	if !data.IsArray() || len(data.Array()) == 0 {
		// Any structured error or usage marker means the request may already be
		// deterministic or billable; never replay it on another account.
		if gjson.GetBytes(body, "error").Exists() || gjson.GetBytes(body, "usage").Exists() {
			return "", fmt.Errorf("native image response contains error or usage without image data")
		}
		return "", errCodexNativeImageNoOutput
	}
	items := data.Array()
	if len(items) != 1 {
		return "", fmt.Errorf("native image response contains %d images, expected one", len(items))
	}
	item := items[0]
	imageBase64 := strings.TrimSpace(item.Get("b64_json").String())
	if imageBase64 == "" {
		return "", fmt.Errorf("native image response is missing b64_json")
	}
	if err := validateCodexImageBase64(imageBase64); err != nil {
		return "", err
	}
	return imageBase64, nil
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
		itemStatus := strings.ToLower(strings.TrimSpace(item.Get("status").String()))
		// MoreCode has been observed returning an outer completed response with a
		// complete, decodable image while leaving the inner image call at the stale
		// status "generating". Accept only that exact compatibility shape. A real
		// in_progress/unknown status, a non-terminal outer response, or invalid image
		// bytes still fails closed and is never retried on another paid provider.
		if itemStatus != "completed" && itemStatus != "generating" {
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

func codexImageBridgeUsesCompletedOuterGeneratingInner(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "status").String()))
	if status != "completed" && status != "done" {
		return false
	}
	imageCalls := 0
	staleGenerating := false
	for _, item := range gjson.GetBytes(body, "output").Array() {
		if strings.TrimSpace(item.Get("type").String()) != "image_generation_call" {
			continue
		}
		imageCalls++
		staleGenerating = strings.EqualFold(strings.TrimSpace(item.Get("status").String()), "generating")
	}
	return imageCalls == 1 && staleGenerating
}

// normalizeCompletedOpenAIResponseImageStatuses repairs a narrowly observed
// provider compatibility defect without weakening terminal validation. It only
// changes image_generation_call.status from "generating" to "completed" when
// the enclosing response is terminal and the result is a real decodable image.
// It returns the patched payload and the number of normalized calls.
func normalizeCompletedOpenAIResponseImageStatuses(body []byte) ([]byte, int) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, 0
	}

	responsePath := ""
	response := gjson.ParseBytes(body)
	eventType := strings.ToLower(strings.TrimSpace(response.Get("type").String()))
	if eventType == "response.completed" || eventType == "response.done" {
		responsePath = "response"
		response = response.Get("response")
	}
	if !response.Exists() || !response.IsObject() {
		return body, 0
	}
	status := strings.ToLower(strings.TrimSpace(response.Get("status").String()))
	if status != "completed" && status != "done" {
		return body, 0
	}

	updated := body
	normalized := 0
	for index, item := range response.Get("output").Array() {
		if strings.TrimSpace(item.Get("type").String()) != "image_generation_call" ||
			!strings.EqualFold(strings.TrimSpace(item.Get("status").String()), "generating") {
			continue
		}
		result := strings.TrimSpace(item.Get("result").String())
		if result == "" || validateOpenAIImageBase64(result) != nil {
			continue
		}
		path := fmt.Sprintf("output.%d.status", index)
		if responsePath != "" {
			path = responsePath + "." + path
		}
		patched, err := sjson.SetBytes(updated, path, "completed")
		if err != nil {
			return body, 0
		}
		updated = patched
		normalized++
	}
	return updated, normalized
}

// countCompletedOpenAIResponseImages counts only validated image outputs in a
// terminal Responses payload. Token usage or a merely-started image call never
// becomes per-image billing evidence.
func countCompletedOpenAIResponseImages(body []byte) int {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return 0
	}
	response := gjson.ParseBytes(body)
	eventType := strings.ToLower(strings.TrimSpace(response.Get("type").String()))
	if eventType == "response.completed" || eventType == "response.done" {
		response = response.Get("response")
	}
	if !response.Exists() || !response.IsObject() {
		return 0
	}
	status := strings.ToLower(strings.TrimSpace(response.Get("status").String()))
	if status != "completed" && status != "done" {
		return 0
	}

	count := 0
	for _, item := range response.Get("output").Array() {
		if strings.TrimSpace(item.Get("type").String()) != "image_generation_call" {
			continue
		}
		itemStatus := strings.ToLower(strings.TrimSpace(item.Get("status").String()))
		if itemStatus != "completed" && itemStatus != "generating" {
			continue
		}
		result := strings.TrimSpace(item.Get("result").String())
		if result != "" && validateOpenAIImageBase64(result) == nil {
			count++
		}
	}
	return count
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
	endpoint := parsed.Endpoint
	if endpoint != openAIImagesEditsEndpoint {
		endpoint = openAIImagesGenerationsEndpoint
	}
	if !supportsOpenAIImagesEndpoint(account, endpoint) {
		return nil, fmt.Errorf("selected account does not support %s", strings.TrimPrefix(endpoint, "/v1/"))
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

	forwardBody, forwardContentType, err := rewriteOpenAIImagesRequestBody(
		body,
		parsed.ContentType,
		upstreamModel,
		account.OmitOpenAIImageGenerationResponseFormat() &&
			(parsed.ResponseFormat == "" || parsed.ResponseFormat == "b64_json"),
		parsed.IsEdits() && account.RepeatOpenAIImageEditField(),
	)
	if err != nil {
		return nil, fmt.Errorf("rewrite image request model: %w", err)
	}
	// Azure-backed image routes always return base64 payloads but reject the
	// otherwise standard response_format=b64_json field. Strip it only when the
	// selected account explicitly opts in and doing so preserves client
	// semantics; non-base64 formats continue upstream unchanged.
	if parsed.Multipart || parsed.IsEdits() {
		setOpsUpstreamRequestBody(c, nil)
	} else {
		setOpsUpstreamRequestBody(c, forwardBody)
	}

	// Image generation can keep consuming upstream resources after the client
	// disconnects. Keep the selected upstream round trip alive so a completed
	// image is still observed and billed; transport timeouts remain the bound.
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()

	token, _, err := s.GetAccessToken(upstreamCtx, account)
	if err != nil {
		return nil, err
	}
	req, err := s.buildOpenAIImagesRequest(upstreamCtx, c, account, forwardBody, forwardContentType, token, endpoint)
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
				RetryableOnSameAccount: allowOpenAISameAccountRetry(account.IsPoolMode() && isPoolModeRetryableStatus(resp.StatusCode), respBody),
			}
		}
		errorRequestBody := forwardBody
		if parsed.Multipart || parsed.IsEdits() {
			errorRequestBody = nil
		}
		return s.handleErrorResponse(ctx, resp, c, account, errorRequestBody)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	usage, _ := extractOpenAIUsageFromJSONBytes(respBody)
	imageCount := extractOpenAIImageCountFromJSONBytes(respBody)
	if imageCount <= 0 {
		// A response with no image and no billable/error marker is safe to replay
		// on the next account. Once usage/error metadata exists, do not risk a
		// duplicate upstream charge; return a sanitized non-billable failure.
		if !openAIImageResponseMayAlreadyBeBillable(respBody, usage) {
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
		return nil, fmt.Errorf("native image response contained no valid image output")
	}
	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)

	return &OpenAIForwardResult{
		RequestID:        resp.Header.Get("x-request-id"),
		Usage:            usage,
		Model:            requestModel,
		UpstreamModel:    upstreamModel,
		ResponseHeaders:  resp.Header.Clone(),
		Duration:         time.Since(startTime),
		ImageCount:       imageCount,
		ImageSize:        parsed.SizeTier,
		UpstreamEndpoint: endpoint,
	}, nil
}

func (s *OpenAIGatewayService) buildOpenAIImagesRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	contentType string,
	token string,
	endpoint string,
) (*http.Request, error) {
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if endpoint != openAIImagesEditsEndpoint {
		endpoint = openAIImagesGenerationsEndpoint
	}
	targetURL := buildOpenAIEndpointURL(validatedURL, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", normalizedOpenAIImagesContentType(contentType))
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

func rewriteOpenAIImagesRequestBody(body []byte, contentType, model string, omitResponseFormat, repeatImageField bool) ([]byte, string, error) {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err == nil && strings.EqualFold(mediaType, "multipart/form-data") {
		return rewriteOpenAIImagesMultipartBody(body, contentType, model, omitResponseFormat, repeatImageField)
	}
	rewritten, err := sjson.SetBytes(body, "model", strings.TrimSpace(model))
	if err != nil {
		return nil, "", err
	}
	if omitResponseFormat {
		rewritten, err = sjson.DeleteBytes(rewritten, "response_format")
		if err != nil {
			return nil, "", err
		}
	}
	return rewritten, normalizedOpenAIImagesContentType(contentType), nil
}

func rewriteOpenAIImagesMultipartBody(body []byte, contentType, model string, omitResponseFormat, repeatImageField bool) ([]byte, string, error) {
	_, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return nil, "", fmt.Errorf("parse multipart content-type: %w", err)
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, "", fmt.Errorf("multipart boundary is required")
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	modelWritten := false
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return nil, "", fmt.Errorf("read multipart body: %w", nextErr)
		}
		formName := strings.TrimSpace(part.FormName())
		if omitResponseFormat && formName == "response_format" && part.FileName() == "" {
			_ = part.Close()
			continue
		}
		partHeader := cloneOpenAIImagesMultipartHeader(part.Header)
		if repeatImageField && part.FileName() != "" &&
			(formName == "image[]" || strings.HasPrefix(formName, "image[")) {
			if err := replaceOpenAIImagesMultipartFormName(partHeader, "image"); err != nil {
				_ = part.Close()
				return nil, "", err
			}
		}
		target, createErr := writer.CreatePart(partHeader)
		if createErr != nil {
			_ = part.Close()
			return nil, "", fmt.Errorf("create multipart part: %w", createErr)
		}
		if formName == "model" && part.FileName() == "" {
			_, createErr = target.Write([]byte(strings.TrimSpace(model)))
			modelWritten = true
		} else {
			_, createErr = io.Copy(target, part)
		}
		_ = part.Close()
		if createErr != nil {
			return nil, "", fmt.Errorf("copy multipart part: %w", createErr)
		}
	}
	if !modelWritten {
		if err := writer.WriteField("model", strings.TrimSpace(model)); err != nil {
			return nil, "", fmt.Errorf("append multipart model field: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finalize multipart body: %w", err)
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func replaceOpenAIImagesMultipartFormName(header textproto.MIMEHeader, name string) error {
	disposition, params, err := mime.ParseMediaType(header.Get("Content-Disposition"))
	if err != nil {
		return fmt.Errorf("parse multipart content-disposition: %w", err)
	}
	if !strings.EqualFold(disposition, "form-data") {
		return fmt.Errorf("unsupported multipart content-disposition %q", disposition)
	}
	params["name"] = name
	header.Set("Content-Disposition", mime.FormatMediaType(disposition, params))
	return nil
}

func cloneOpenAIImagesMultipartHeader(src textproto.MIMEHeader) textproto.MIMEHeader {
	dst := make(textproto.MIMEHeader, len(src))
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
	return dst
}

func extractOpenAIImageCountFromJSONBytes(body []byte) int {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return 0
	}
	for _, path := range []string{"data", "images", "result", "output"} {
		items := gjson.GetBytes(body, path)
		if !items.Exists() || !items.IsArray() {
			continue
		}
		count := 0
		for _, item := range items.Array() {
			if validOpenAIImageOutputItem(item) {
				count++
			}
		}
		return count
	}
	return 0
}

func validOpenAIImageOutputItem(item gjson.Result) bool {
	if item.Type == gjson.String {
		return validOpenAIImageOutputURL(item.String())
	}
	if !item.IsObject() {
		return false
	}
	if encoded := strings.TrimSpace(item.Get("b64_json").String()); encoded != "" {
		return validateOpenAIImageBase64(encoded) == nil
	}
	for _, path := range []string{"url", "image_url"} {
		if validOpenAIImageOutputURL(item.Get(path).String()) {
			return true
		}
	}
	return false
}

func validOpenAIImageOutputURL(value string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "data:image/") {
		parts := strings.SplitN(value, ",", 2)
		return len(parts) == 2 && strings.Contains(strings.ToLower(parts[0]), ";base64") &&
			validateOpenAIImageBase64(parts[1]) == nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

func validateOpenAIImageBase64(value string) error {
	if len(value) > codexNativeImageMaxBase64Bytes {
		return fmt.Errorf("image result exceeds maximum size")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil {
		return fmt.Errorf("image result is invalid base64")
	}
	contentType := http.DetectContentType(decoded)
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return fmt.Errorf("image result is not a supported image")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(decoded))
	if err != nil || config.Width <= 0 || config.Height <= 0 ||
		int64(config.Width)*int64(config.Height) > codexNativeImageMaxPixels {
		return fmt.Errorf("image result is not decodable")
	}
	return nil
}

func openAIImageResponseMayAlreadyBeBillable(body []byte, usage OpenAIUsage) bool {
	if usage.InputTokens > 0 || usage.OutputTokens > 0 ||
		usage.ImageInputTokens > 0 || usage.ImageOutputTokens > 0 ||
		gjson.GetBytes(body, "usage").Exists() ||
		gjson.GetBytes(body, "error").Exists() {
		return true
	}
	// A non-empty candidate output may represent a generated/billable image
	// whose payload is malformed or unsupported. Never replay it on another
	// account; only a genuinely empty output is safe for bounded failover.
	for _, path := range []string{"data", "images", "result", "output"} {
		items := gjson.GetBytes(body, path)
		if items.IsArray() && len(items.Array()) > 0 {
			return true
		}
	}
	return false
}
