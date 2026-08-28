package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// detachUpstreamContext lets a completed client upload finish its already
// selected upstream round trip without inheriting a cancellation race.
func detachUpstreamContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.Background(), func() {}
	}
	return context.WithoutCancel(ctx), func() {}
}

func openAIAccountStateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	} else {
		ctx = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(ctx, 5*time.Second)
}

func mapUpstreamStatus(status int) int {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return status
	case http.StatusTooManyRequests:
		return status
	default:
		return http.StatusBadGateway
	}
}

func isGrokImageGenerationModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return model == "grok-imagine" || model == "grok-imagine-edit" || strings.HasPrefix(model, "grok-imagine-image")
}

func isGrokOAuthAccount(account *Account) bool {
	return account != nil && account.IsGrokOAuth()
}

func openAIJSONValueMayContainImageInput(value any) bool {
	switch typed := value.(type) {
	case gjson.Result:
		if !typed.Exists() {
			return false
		}
		var decoded any
		if err := json.Unmarshal([]byte(typed.Raw), &decoded); err != nil {
			return false
		}
		return openAIJSONValueMayContainImageInput(decoded)
	case map[string]any:
		if kind, _ := typed["type"].(string); kind == "input_image" || kind == "image_url" {
			return true
		}
		for _, child := range typed {
			if openAIJSONValueMayContainImageInput(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if openAIJSONValueMayContainImageInput(child) {
				return true
			}
		}
	case string:
		value := strings.ToLower(strings.TrimSpace(typed))
		return strings.HasPrefix(value, "data:image/")
	}
	return false
}

func copyOpenAIUsageFromResponsesUsage(usage *apicompat.ResponsesUsage) OpenAIUsage {
	if usage == nil {
		return OpenAIUsage{}
	}
	result := OpenAIUsage{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}
	if usage.InputTokensDetails != nil {
		result.CacheReadInputTokens = usage.InputTokensDetails.CachedTokens
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

const (
	openCodeSessionAffinityHeader = "X-Session-Affinity"
	openCodeSessionIDHeader       = "X-Session-Id"
	openCodeNativeSessionHeader   = "X-OpenCode-Session"
	codeBuddyConversationHeader   = "X-Conversation-ID"
)

var explicitOpenAIHeaderSessionNames = []string{
	"session-id",
	"session_id",
	"conversation_id",
	openCodeSessionAffinityHeader,
	openCodeSessionIDHeader,
	openCodeNativeSessionHeader,
	codeBuddyConversationHeader,
}

func explicitOpenAIHeaderSessionID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	for _, name := range explicitOpenAIHeaderSessionNames {
		if value := strings.TrimSpace(c.GetHeader(name)); value != "" {
			return value
		}
	}
	return ""
}

type resolvedTargetPlatformContextKey struct{}

func WithResolvedTargetPlatform(ctx context.Context, platform string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, resolvedTargetPlatformContextKey{}, strings.TrimSpace(platform))
}

func ResolvedTargetPlatformFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	platform, _ := ctx.Value(resolvedTargetPlatformContextKey{}).(string)
	platform = strings.TrimSpace(platform)
	return platform, platform != ""
}

const actualOpenAIUpstreamEndpointKey = "openai_actual_upstream_endpoint"

func SetActualOpenAIUpstreamEndpoint(c *gin.Context, endpoint string) {
	if c != nil && strings.TrimSpace(endpoint) != "" {
		c.Set(actualOpenAIUpstreamEndpointKey, strings.TrimSpace(endpoint))
	}
}

func GetActualOpenAIUpstreamEndpoint(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, exists := c.Get(actualOpenAIUpstreamEndpointKey)
	if !exists {
		return ""
	}
	endpoint, _ := value.(string)
	return strings.TrimSpace(endpoint)
}

func (s *OpenAIGatewayService) handleOpenAIUpstreamTransportError(_ context.Context, c *gin.Context, account *Account, err error, passthrough bool) error {
	if err == nil {
		return nil
	}
	safeErr := sanitizeUpstreamErrorMessage(err.Error())
	setOpsUpstreamError(c, 0, safeErr, "")
	if account != nil {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform: account.Platform, AccountID: account.ID, AccountName: account.Name,
			Passthrough: passthrough, Kind: "request_error", Message: safeErr,
		})
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	return &UpstreamFailoverError{
		StatusCode:   http.StatusBadGateway,
		ResponseBody: []byte(`{"error":{"type":"upstream_error","message":"Upstream request failed"}}`),
	}
}

func countOpenAIResponseImageOutputsFromJSONBytes(body []byte) int {
	if !gjson.ValidBytes(body) {
		return 0
	}
	if data := gjson.GetBytes(body, "data"); data.IsArray() {
		return len(data.Array())
	}
	count := 0
	for _, item := range gjson.GetBytes(body, "output").Array() {
		if item.Get("type").String() == "image_generation_call" && strings.TrimSpace(item.Get("result").String()) != "" {
			count++
		}
	}
	return count
}

func collectOpenAIResponseImageOutputSizesFromJSONBytes(body []byte) []string {
	if !gjson.ValidBytes(body) {
		return nil
	}
	var sizes []string
	for _, item := range gjson.GetBytes(body, "data").Array() {
		if size := strings.TrimSpace(item.Get("size").String()); size != "" {
			sizes = append(sizes, size)
		}
	}
	for _, item := range gjson.GetBytes(body, "output").Array() {
		if item.Get("type").String() != "image_generation_call" {
			continue
		}
		if size := strings.TrimSpace(item.Get("size").String()); size != "" {
			sizes = append(sizes, size)
		}
	}
	return sizes
}

func openAIImageUploadToDataURL(upload OpenAIImagesUpload) (string, error) {
	if len(upload.Data) == 0 {
		return "", fmt.Errorf("upload %q is empty", strings.TrimSpace(upload.FileName))
	}
	contentType := strings.TrimSpace(upload.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(upload.Data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "", fmt.Errorf("upload %q is not an image", strings.TrimSpace(upload.FileName))
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(upload.Data), nil
}

func marshalOpenAIUpstreamJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func (s *OpenAIGatewayService) readUpstreamErrorBody(response *http.Response) []byte {
	if response == nil || response.Body == nil {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return body
}

func (s *OpenAIGatewayService) readOpenAIUpstreamError(response *http.Response) ([]byte, string) {
	body := s.readUpstreamErrorBody(response)
	if response != nil {
		if response.Body != nil {
			_ = response.Body.Close()
		}
		response.Body = io.NopCloser(bytes.NewReader(body))
	}
	message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	return body, message
}

func isStrictlyIncreasingAccountPage(accounts []Account, afterID int64) bool {
	previous := afterID
	for i := range accounts {
		if accounts[i].ID <= previous {
			return false
		}
		previous = accounts[i].ID
	}
	return true
}
