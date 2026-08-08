package service

import (
	"fmt"
	"strings"
	"time"
)

const (
	BillingTypeBalance      int8 = 0 // 钱包余额
	BillingTypeSubscription int8 = 1 // 订阅套餐
)

type RequestType int16

const (
	RequestTypeUnknown RequestType = 0
	RequestTypeSync    RequestType = 1
	RequestTypeStream  RequestType = 2
	RequestTypeWSV2    RequestType = 3
)

func (t RequestType) IsValid() bool {
	switch t {
	case RequestTypeUnknown, RequestTypeSync, RequestTypeStream, RequestTypeWSV2:
		return true
	default:
		return false
	}
}

func (t RequestType) Normalize() RequestType {
	if t.IsValid() {
		return t
	}
	return RequestTypeUnknown
}

func (t RequestType) String() string {
	switch t.Normalize() {
	case RequestTypeSync:
		return "sync"
	case RequestTypeStream:
		return "stream"
	case RequestTypeWSV2:
		return "ws_v2"
	default:
		return "unknown"
	}
}

func RequestTypeFromInt16(v int16) RequestType {
	return RequestType(v).Normalize()
}

func ParseUsageRequestType(value string) (RequestType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "unknown":
		return RequestTypeUnknown, nil
	case "sync":
		return RequestTypeSync, nil
	case "stream":
		return RequestTypeStream, nil
	case "ws_v2":
		return RequestTypeWSV2, nil
	default:
		return RequestTypeUnknown, fmt.Errorf("invalid request_type, allowed values: unknown, sync, stream, ws_v2")
	}
}

func RequestTypeFromLegacy(stream bool, openAIWSMode bool) RequestType {
	if openAIWSMode {
		return RequestTypeWSV2
	}
	if stream {
		return RequestTypeStream
	}
	return RequestTypeSync
}

func ApplyLegacyRequestFields(requestType RequestType, fallbackStream bool, fallbackOpenAIWSMode bool) (stream bool, openAIWSMode bool) {
	switch requestType.Normalize() {
	case RequestTypeSync:
		return false, false
	case RequestTypeStream:
		return true, false
	case RequestTypeWSV2:
		return true, true
	default:
		return fallbackStream, fallbackOpenAIWSMode
	}
}

type UsageLog struct {
	ID        int64
	UserID    int64
	APIKeyID  int64
	AccountID int64
	RequestID string
	Model     string
	// RequestedModel is the client-requested model name recorded for stable user/admin display.
	// Empty should be treated as Model for backward compatibility with historical rows.
	RequestedModel string
	// UpstreamModel is the actual model sent to the upstream provider after mapping.
	// Nil means no mapping was applied.
	UpstreamModel *string
	// ChannelID is the logical channel identifier when channel-based routing is used.
	ChannelID *int64
	// ModelMappingChain records the model mapping path, e.g. "a→b→c".
	ModelMappingChain *string
	// BillingTier records the matched pricing tier label when applicable.
	BillingTier *string
	// BillingMode is the effective billing mode used for display/observability.
	// Current DragonCode uses token/image; per_request is reserved for future parity.
	BillingMode *string
	// ServiceTier records the OpenAI service tier used for billing, e.g. "priority" / "flex".
	ServiceTier *string
	// ReasoningEffort is the request's reasoning effort level.
	// OpenAI: "low" / "medium" / "high" / "xhigh"; Claude: "low" / "medium" / "high" / "max".
	// Nil means not provided / not applicable.
	ReasoningEffort *string
	// InboundEndpoint is the client-facing API endpoint path, e.g. /v1/chat/completions.
	InboundEndpoint *string
	// UpstreamEndpoint is the normalized upstream endpoint path, e.g. /v1/responses.
	UpstreamEndpoint *string

	GroupID        *int64
	SubscriptionID *int64

	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int

	CacheCreation5mTokens int `gorm:"column:cache_creation_5m_tokens"`
	CacheCreation1hTokens int `gorm:"column:cache_creation_1h_tokens"`
	ImageOutputTokens     int

	InputCost         float64
	OutputCost        float64
	CacheCreationCost float64
	CacheReadCost     float64
	TotalCost         float64
	ActualCost        float64
	ImageOutputCost   float64
	RateMultiplier    float64
	// AccountRateMultiplier 账号计费倍率快照（nil 表示历史数据，按 1.0 处理）
	AccountRateMultiplier *float64
	// AccountStatsCost is the pre-multiplier account-cost base for observability.
	// When nil, callers should fall back to TotalCost.
	AccountStatsCost *float64

	BillingType  int8
	RequestType  RequestType
	Stream       bool
	OpenAIWSMode bool
	DurationMs   *int
	FirstTokenMs *int
	UserAgent    *string
	IPAddress    *string

	// Cache TTL Override 标记（管理员强制替换了缓存 TTL 计费）
	CacheTTLOverridden bool

	// 图片生成字段
	ImageCount           int
	ImageSize            *string
	MediaType            *string
	ImageResultURLs      []string
	VideoCount           int
	VideoResolution      *string
	VideoDurationSeconds *int

	CreatedAt time.Time

	// AccountingCommand is an in-memory, redacted persistence instruction. It
	// is never serialized as part of a usage-log response. The repository
	// commits it atomically with this usage log.
	AccountingCommand *UsageBillingCommand `json:"-"`

	User         *User
	APIKey       *APIKey
	Account      *Account
	Group        *Group
	Subscription *UserSubscription
}

func (u *UsageLog) TotalTokens() int {
	return u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens
}

func (u *UsageLog) EffectiveRequestType() RequestType {
	if u == nil {
		return RequestTypeUnknown
	}
	if normalized := u.RequestType.Normalize(); normalized != RequestTypeUnknown {
		return normalized
	}
	return RequestTypeFromLegacy(u.Stream, u.OpenAIWSMode)
}

func (u *UsageLog) SyncRequestTypeAndLegacyFields() {
	if u == nil {
		return
	}
	requestType := u.EffectiveRequestType()
	u.RequestType = requestType
	u.Stream, u.OpenAIWSMode = ApplyLegacyRequestFields(requestType, u.Stream, u.OpenAIWSMode)
}

func InferUsageBillingMode(imageCount int, mediaType *string) *string {
	if imageCount > 0 {
		mode := string(BillingModeImage)
		return &mode
	}
	if mediaType != nil {
		switch strings.ToLower(strings.TrimSpace(*mediaType)) {
		case "image", "video":
			mode := string(BillingModeImage)
			return &mode
		}
	}
	mode := string(BillingModeToken)
	return &mode
}
