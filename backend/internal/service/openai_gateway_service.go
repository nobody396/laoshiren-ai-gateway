package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/apicompat"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ip"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai_compat"
	"github.com/bozhouDev/DragonCode-sub2api/internal/platform/liveattestation"
	"github.com/bozhouDev/DragonCode-sub2api/internal/util/responseheaders"
	"github.com/bozhouDev/DragonCode-sub2api/internal/util/urlvalidator"
	"github.com/cespare/xxhash/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

const (
	// ChatGPT internal API for OAuth accounts
	chatgptCodexURL = "https://chatgpt.com/backend-api/codex/responses"
	// OpenAI Platform API for API Key accounts (fallback)
	openaiPlatformAPIURL   = "https://api.openai.com/v1/responses"
	openaiStickySessionTTL = time.Hour // 粘性会话TTL
	codexCLIUserAgent      = "codex_cli_rs/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color"
	// codex_cli_only 拒绝时单个请求头日志长度上限（字符）
	codexCLIOnlyHeaderValueMaxBytes = 256

	// OpenAIParsedRequestBodyKey 缓存 handler 侧已解析的请求体，避免重复解析。
	OpenAIParsedRequestBodyKey = "openai_parsed_request_body"
	// OpenAI WS Mode 失败后的重连次数上限（不含首次尝试）。
	// 与 Codex 客户端保持一致：失败后最多重连 5 次。
	openAIWSReconnectRetryLimit = 5
	// OpenAI WS Mode 重连退避默认值（可由配置覆盖）。
	openAIWSRetryBackoffInitialDefault = 120 * time.Millisecond
	openAIWSRetryBackoffMaxDefault     = 2 * time.Second
	openAIWSRetryJitterRatioDefault    = 0.2
	openAICompactSessionSeedKey        = "openai_compact_session_seed"
	codexCLIVersion                    = "0.144.1"
	// Codex 限额快照仅用于后台展示/诊断，不需要每个成功请求都立即落库。
	openAICodexSnapshotPersistMinInterval = 30 * time.Second
)

var ErrNoAvailableCompactAccounts = errors.New("no available OpenAI accounts support /responses/compact")

// OpenAI allowed headers whitelist (for non-passthrough).
var openaiAllowedHeaders = map[string]bool{
	"accept-language":                        true,
	"content-type":                           true,
	"conversation_id":                        true,
	"user-agent":                             true,
	"originator":                             true,
	"session_id":                             true,
	"x-codex-turn-state":                     true,
	"x-codex-turn-metadata":                  true,
	"x-openai-internal-codex-responses-lite": true,
}

// OpenAI passthrough allowed headers whitelist.
// 透传模式下仅放行这些低风险请求头，避免将非标准/环境噪声头传给上游触发风控。
var openaiPassthroughAllowedHeaders = map[string]bool{
	"accept":                                 true,
	"accept-language":                        true,
	"content-type":                           true,
	"conversation_id":                        true,
	"openai-beta":                            true,
	"user-agent":                             true,
	"originator":                             true,
	"session_id":                             true,
	"x-codex-turn-state":                     true,
	"x-codex-turn-metadata":                  true,
	"x-openai-internal-codex-responses-lite": true,
}

// codex_cli_only 拒绝时记录的请求头白名单（仅用于诊断日志，不参与上游透传）
var codexCLIOnlyDebugHeaderWhitelist = []string{
	"User-Agent",
	"Content-Type",
	"Accept",
	"Accept-Language",
	"OpenAI-Beta",
	"Originator",
	"Session_ID",
	"Conversation_ID",
	"X-Request-ID",
	"X-Client-Request-ID",
	"X-Forwarded-For",
	"X-Real-IP",
}

// OpenAICodexUsageSnapshot represents Codex API usage limits from response headers
type OpenAICodexUsageSnapshot struct {
	PrimaryUsedPercent          *float64 `json:"primary_used_percent,omitempty"`
	PrimaryResetAfterSeconds    *int     `json:"primary_reset_after_seconds,omitempty"`
	PrimaryWindowMinutes        *int     `json:"primary_window_minutes,omitempty"`
	SecondaryUsedPercent        *float64 `json:"secondary_used_percent,omitempty"`
	SecondaryResetAfterSeconds  *int     `json:"secondary_reset_after_seconds,omitempty"`
	SecondaryWindowMinutes      *int     `json:"secondary_window_minutes,omitempty"`
	PrimaryOverSecondaryPercent *float64 `json:"primary_over_secondary_percent,omitempty"`
	UpdatedAt                   string   `json:"updated_at,omitempty"`
}

// NormalizedCodexLimits contains normalized 5h/7d rate limit data
type NormalizedCodexLimits struct {
	Used5hPercent   *float64
	Reset5hSeconds  *int
	Window5hMinutes *int
	Used7dPercent   *float64
	Reset7dSeconds  *int
	Window7dMinutes *int
}

// Normalize converts primary/secondary fields to canonical 5h/7d fields.
// Strategy: Compare window_minutes to determine which is 5h vs 7d.
// Returns nil if snapshot is nil or has no useful data.
func (s *OpenAICodexUsageSnapshot) Normalize() *NormalizedCodexLimits {
	if s == nil {
		return nil
	}

	result := &NormalizedCodexLimits{}

	primaryMins := 0
	secondaryMins := 0
	hasPrimaryWindow := false
	hasSecondaryWindow := false

	if s.PrimaryWindowMinutes != nil {
		primaryMins = *s.PrimaryWindowMinutes
		hasPrimaryWindow = true
	}
	if s.SecondaryWindowMinutes != nil {
		secondaryMins = *s.SecondaryWindowMinutes
		hasSecondaryWindow = true
	}

	// Determine mapping based on window_minutes
	use5hFromPrimary := false
	use7dFromPrimary := false

	if hasPrimaryWindow && hasSecondaryWindow {
		// Both known: smaller window is 5h, larger is 7d
		if primaryMins < secondaryMins {
			use5hFromPrimary = true
		} else {
			use7dFromPrimary = true
		}
	} else if hasPrimaryWindow {
		// Only primary known: classify by threshold (<=360 min = 6h -> 5h window)
		if primaryMins <= 360 {
			use5hFromPrimary = true
		} else {
			use7dFromPrimary = true
		}
	} else if hasSecondaryWindow {
		// Only secondary known: classify by threshold
		if secondaryMins <= 360 {
			// 5h from secondary, so primary (if any data) is 7d
			use7dFromPrimary = true
		} else {
			// 7d from secondary, so primary (if any data) is 5h
			use5hFromPrimary = true
		}
	} else {
		// No window_minutes: fall back to legacy assumption (primary=7d, secondary=5h)
		use7dFromPrimary = true
	}

	// Assign values
	if use5hFromPrimary {
		result.Used5hPercent = s.PrimaryUsedPercent
		result.Reset5hSeconds = s.PrimaryResetAfterSeconds
		result.Window5hMinutes = s.PrimaryWindowMinutes
		result.Used7dPercent = s.SecondaryUsedPercent
		result.Reset7dSeconds = s.SecondaryResetAfterSeconds
		result.Window7dMinutes = s.SecondaryWindowMinutes
	} else if use7dFromPrimary {
		result.Used7dPercent = s.PrimaryUsedPercent
		result.Reset7dSeconds = s.PrimaryResetAfterSeconds
		result.Window7dMinutes = s.PrimaryWindowMinutes
		result.Used5hPercent = s.SecondaryUsedPercent
		result.Reset5hSeconds = s.SecondaryResetAfterSeconds
		result.Window5hMinutes = s.SecondaryWindowMinutes
	}

	return result
}

// OpenAIUsage represents OpenAI API response usage
type OpenAIUsage struct {
	InputTokens              int `json:"input_tokens"`
	ImageInputTokens         int `json:"image_input_tokens,omitempty"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	ImageOutputTokens        int `json:"image_output_tokens,omitempty"`
}

// OpenAIForwardResult represents the result of forwarding
type OpenAIForwardResult struct {
	RequestID  string
	ResponseID string
	Usage      OpenAIUsage
	Model      string // 原始模型（用于响应和日志显示）
	// BillingModel is the model used for cost calculation.
	// When non-empty, CalculateCost uses this instead of Model.
	// This is set by the Anthropic Messages conversion path where
	// the mapped upstream model differs from the client-facing model.
	BillingModel string
	// UpstreamModel is the actual model sent to the upstream provider after mapping.
	// Empty when no mapping was applied (requested model was used as-is).
	UpstreamModel string
	// UpstreamEndpoint is the actual upstream API path selected for this request.
	UpstreamEndpoint string
	// ServiceTier records the OpenAI Responses API service tier, e.g. "priority" / "flex".
	// Nil means the request did not specify a recognized tier.
	ServiceTier *string
	// ReasoningEffort is extracted from request body (reasoning.effort) or derived from model suffix.
	// Stored for usage records display; nil means not provided / not applicable.
	ReasoningEffort      *string
	Stream               bool
	OpenAIWSMode         bool
	ResponseHeaders      http.Header
	Duration             time.Duration
	FirstTokenMs         *int
	ImageCount           int
	ImageSize            string
	ImageInputSize       string
	ImageOutputSize      string
	ImageOutputSizes     []string
	ImageSizeSource      string
	ImageSizeBreakdown   map[string]int
	VideoCount           int
	VideoResolution      string
	VideoDurationSeconds int
	GPTImageTaskIDs      []string
	ResponseBody         []byte
	ResponseStatus       int
	ResponseType         string

	wsReplayInput                []json.RawMessage
	wsReplayInputExists          bool
	wsAccountFailoverReplayInput []json.RawMessage
}

type OpenAIWSRetryMetricsSnapshot struct {
	RetryAttemptsTotal            int64 `json:"retry_attempts_total"`
	RetryBackoffMsTotal           int64 `json:"retry_backoff_ms_total"`
	RetryExhaustedTotal           int64 `json:"retry_exhausted_total"`
	NonRetryableFastFallbackTotal int64 `json:"non_retryable_fast_fallback_total"`
}

type OpenAICompatibilityFallbackMetricsSnapshot struct {
	SessionHashLegacyReadFallbackTotal int64   `json:"session_hash_legacy_read_fallback_total"`
	SessionHashLegacyReadFallbackHit   int64   `json:"session_hash_legacy_read_fallback_hit"`
	SessionHashLegacyDualWriteTotal    int64   `json:"session_hash_legacy_dual_write_total"`
	SessionHashLegacyReadHitRate       float64 `json:"session_hash_legacy_read_hit_rate"`

	MetadataLegacyFallbackIsMaxTokensOneHaikuTotal int64 `json:"metadata_legacy_fallback_is_max_tokens_one_haiku_total"`
	MetadataLegacyFallbackThinkingEnabledTotal     int64 `json:"metadata_legacy_fallback_thinking_enabled_total"`
	MetadataLegacyFallbackPrefetchedStickyAccount  int64 `json:"metadata_legacy_fallback_prefetched_sticky_account_total"`
	MetadataLegacyFallbackPrefetchedStickyGroup    int64 `json:"metadata_legacy_fallback_prefetched_sticky_group_total"`
	MetadataLegacyFallbackSingleAccountRetryTotal  int64 `json:"metadata_legacy_fallback_single_account_retry_total"`
	MetadataLegacyFallbackAccountSwitchCountTotal  int64 `json:"metadata_legacy_fallback_account_switch_count_total"`
	MetadataLegacyFallbackTotal                    int64 `json:"metadata_legacy_fallback_total"`
}

type openAIWSRetryMetrics struct {
	retryAttempts            atomic.Int64
	retryBackoffMs           atomic.Int64
	retryExhausted           atomic.Int64
	nonRetryableFastFallback atomic.Int64
}

type accountWriteThrottle struct {
	minInterval time.Duration
	mu          sync.Mutex
	lastByID    map[int64]time.Time
}

func newAccountWriteThrottle(minInterval time.Duration) *accountWriteThrottle {
	return &accountWriteThrottle{
		minInterval: minInterval,
		lastByID:    make(map[int64]time.Time),
	}
}

func (t *accountWriteThrottle) Allow(id int64, now time.Time) bool {
	if t == nil || id <= 0 || t.minInterval <= 0 {
		return true
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.lastByID[id]; ok && now.Sub(last) < t.minInterval {
		return false
	}
	t.lastByID[id] = now

	if len(t.lastByID) > 4096 {
		cutoff := now.Add(-4 * t.minInterval)
		for accountID, writtenAt := range t.lastByID {
			if writtenAt.Before(cutoff) {
				delete(t.lastByID, accountID)
			}
		}
	}

	return true
}

var defaultOpenAICodexSnapshotPersistThrottle = newAccountWriteThrottle(openAICodexSnapshotPersistMinInterval)

// OpenAIGatewayService handles OpenAI API gateway operations
type OpenAIGatewayService struct {
	accountRepo               AccountRepository
	usageLogRepo              UsageLogRepository
	usageBillingRepo          UsageBillingRepository
	userRepo                  UserRepository
	userSubRepo               UserSubscriptionRepository
	cache                     GatewayCache
	cfg                       *config.Config
	codexDetector             CodexClientRestrictionDetector
	schedulerSnapshot         *SchedulerSnapshotService
	concurrencyService        *ConcurrencyService
	billingService            *BillingService
	rateLimitService          *RateLimitService
	billingCacheService       *BillingCacheService
	userGroupRateResolver     *userGroupRateResolver
	httpUpstream              HTTPUpstream
	deferredService           *DeferredService
	openAITokenProvider       *OpenAITokenProvider
	grokTokenProvider         *GrokTokenProvider
	toolCorrector             *CodexToolCorrector
	openaiWSResolver          OpenAIWSProtocolResolver
	resolver                  *ModelPricingResolver
	channelService            *ChannelService
	serviceTierPricing        ChannelModelPricingProvider
	serviceTierCapabilityGate bool
	accountQuotaAlertService  *AccountQuotaAlertService
	balanceAlertService       *BalanceAlertService
	commissionService         *CommissionService
	gptImageTaskRepo          GPTImageTaskRepository
	gptImageS3Storage         *GPTImageS3Storage
	settingService            *SettingService
	liveAttestation           liveattestation.Provider
	liveAttestationCipher     SecretEncryptor
	openAIRouteEvaluator      OpenAIRouteShadowEvaluator
	openAIRouteAuditService   *OpenAIRouteAuditService
	openAIRouteObservations   *OpenAIRouteObservationCollector
	pipeline                  *GatewayPipeline
	usageBillingNow           func() time.Time

	openaiWSPoolOnce                    sync.Once
	openaiWSStateStoreOnce              sync.Once
	openaiSchedulerOnce                 sync.Once
	openaiWSPassthroughDialerOnce       sync.Once
	openaiWSPool                        *openAIWSConnPool
	openaiWSStateStore                  OpenAIWSStateStore
	openaiScheduler                     OpenAIAccountScheduler
	openaiWSPassthroughDialer           openAIWSClientDialer
	openaiAccountStats                  *openAIAccountRuntimeStats
	openaiAccountRuntimeBlockUntil      sync.Map
	openaiAccountRuntimeBlockLocks      sync.Map
	openaiAccountRuntimeBlockGeneration sync.Map
	openaiAccountRuntimeBlockSequence   atomic.Uint64
	grokCredentialMutationLocks         sync.Map

	openaiWSFallbackUntil sync.Map // key: int64(accountID), value: time.Time
	openaiWSRetryMetrics  openAIWSRetryMetrics
	responseHeaderFilter  *responseheaders.CompiledHeaderFilter
	codexSnapshotThrottle *accountWriteThrottle

	groupPlatformAccountVerdicts sync.Map // key: string(groupID|platform), value: groupPlatformAccountsVerdict
}

// SetGrokTokenProvider injects the Grok OAuth request-path token provider
// without widening the existing constructor used by local tests and wire code.
func (s *OpenAIGatewayService) SetGrokTokenProvider(provider *GrokTokenProvider) {
	if s != nil {
		s.grokTokenProvider = provider
	}
}

// SetOpenAIRouteEvaluator enables diagnostic shadow evaluation. The evaluator
// cannot change the selected account in this release.
func (s *OpenAIGatewayService) SetOpenAIRouteEvaluator(evaluator OpenAIRouteShadowEvaluator) {
	if s != nil {
		s.openAIRouteEvaluator = evaluator
	}
}

func (s *OpenAIGatewayService) SetOpenAIRouteAuditService(audit *OpenAIRouteAuditService) {
	if s != nil {
		s.openAIRouteAuditService = audit
	}
}

func (s *OpenAIGatewayService) SetOpenAIRouteObservationCollector(collector *OpenAIRouteObservationCollector) {
	if s != nil {
		s.openAIRouteObservations = collector
	}
}

func (s *OpenAIGatewayService) SetGatewayPipeline(pipeline *GatewayPipeline) {
	s.pipeline = pipeline
}

// NewOpenAIGatewayService creates a new OpenAIGatewayService
func NewOpenAIGatewayService(
	accountRepo AccountRepository,
	usageLogRepo UsageLogRepository,
	usageBillingRepo UsageBillingRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	userGroupRateRepo UserGroupRateRepository,
	cache GatewayCache,
	cfg *config.Config,
	schedulerSnapshot *SchedulerSnapshotService,
	concurrencyService *ConcurrencyService,
	billingService *BillingService,
	rateLimitService *RateLimitService,
	billingCacheService *BillingCacheService,
	httpUpstream HTTPUpstream,
	deferredService *DeferredService,
	openAITokenProvider *OpenAITokenProvider,
	resolver *ModelPricingResolver,
	channelService *ChannelService,
	accountQuotaAlertService *AccountQuotaAlertService,
	balanceAlertService *BalanceAlertService,
	commissionService *CommissionService,
	gptImageTaskRepo GPTImageTaskRepository,
	gptImageS3Storage *GPTImageS3Storage,
	settingService *SettingService,
) *OpenAIGatewayService {
	svc := &OpenAIGatewayService{
		accountRepo:         accountRepo,
		usageLogRepo:        usageLogRepo,
		usageBillingRepo:    usageBillingRepo,
		userRepo:            userRepo,
		userSubRepo:         userSubRepo,
		cache:               cache,
		cfg:                 cfg,
		codexDetector:       NewOpenAICodexClientRestrictionDetector(cfg),
		schedulerSnapshot:   schedulerSnapshot,
		concurrencyService:  concurrencyService,
		billingService:      billingService,
		rateLimitService:    rateLimitService,
		billingCacheService: billingCacheService,
		userGroupRateResolver: newUserGroupRateResolver(
			userGroupRateRepo,
			nil,
			resolveUserGroupRateCacheTTL(cfg),
			nil,
			"service.openai_gateway",
		),
		httpUpstream:              httpUpstream,
		deferredService:           deferredService,
		openAITokenProvider:       openAITokenProvider,
		toolCorrector:             NewCodexToolCorrector(),
		openaiWSResolver:          NewOpenAIWSProtocolResolver(cfg),
		resolver:                  resolver,
		channelService:            channelService,
		serviceTierPricing:        channelService,
		serviceTierCapabilityGate: true,
		accountQuotaAlertService:  accountQuotaAlertService,
		balanceAlertService:       balanceAlertService,
		commissionService:         commissionService,
		gptImageTaskRepo:          gptImageTaskRepo,
		gptImageS3Storage:         gptImageS3Storage,
		settingService:            settingService,
		liveAttestation:           liveattestation.NewProvider(),
		liveAttestationCipher:     newLiveAttestationCipher(cfg),
		responseHeaderFilter:      compileResponseHeaderFilter(cfg),
		codexSnapshotThrottle:     newAccountWriteThrottle(openAICodexSnapshotPersistMinInterval),
	}
	svc.logOpenAIWSModeBootstrap()
	return svc
}

// ResolveChannelMapping 解析渠道级模型映射（代理到 ChannelService）
func (s *OpenAIGatewayService) ResolveChannelMapping(ctx context.Context, groupID int64, model string) ChannelMappingResult {
	if s.channelService == nil {
		return ChannelMappingResult{MappedModel: model}
	}
	return s.channelService.ResolveChannelMapping(ctx, groupID, model)
}

// IsModelRestricted 检查模型是否被渠道限制（代理到 ChannelService）
func (s *OpenAIGatewayService) IsModelRestricted(ctx context.Context, groupID int64, model string) bool {
	if s.channelService == nil {
		return false
	}
	return s.channelService.IsModelRestricted(ctx, groupID, model)
}

// ResolveChannelMappingAndRestrict 解析渠道映射。
// 模型限制检查已移至调度阶段，restricted 始终返回 false。
func (s *OpenAIGatewayService) ResolveChannelMappingAndRestrict(ctx context.Context, groupID *int64, model string) (ChannelMappingResult, bool) {
	if s.channelService == nil {
		return ChannelMappingResult{MappedModel: model}, false
	}
	return s.channelService.ResolveChannelMappingAndRestrict(ctx, groupID, model)
}

func (s *OpenAIGatewayService) checkChannelPricingRestriction(ctx context.Context, groupID *int64, requestedModel string) bool {
	if groupID == nil || s.channelService == nil || requestedModel == "" {
		return false
	}
	mapping := s.channelService.ResolveChannelMapping(ctx, *groupID, requestedModel)
	billingModel := billingModelForRestriction(mapping.BillingModelSource, requestedModel, mapping.MappedModel)
	if billingModel == "" {
		return false
	}
	return s.channelService.IsModelRestricted(ctx, *groupID, billingModel)
}

func (s *OpenAIGatewayService) isUpstreamModelRestrictedByChannel(ctx context.Context, groupID int64, account *Account, requestedModel string, requireCompact bool) bool {
	if s.channelService == nil {
		return false
	}
	upstreamModel := resolveOpenAIAccountUpstreamModelForRequest(account, requestedModel, requireCompact)
	if upstreamModel == "" {
		return false
	}
	return s.channelService.IsModelRestricted(ctx, groupID, upstreamModel)
}

// allOpenAICandidatesModelUnsupported 判断排除 excluded 之后，剩余候选账号是否
// 全部因 model_mapping 不支持请求模型而被过滤（无映射的账号视为支持任意模型）。
// 注意仅统计 OpenAI 平台账号，非 OpenAI 账号属于平台不匹配而非模型不支持。
func allOpenAICandidatesModelUnsupported(accounts []Account, excludedIDs map[int64]struct{}, requestedModel string) bool {
	if requestedModel == "" || len(accounts) == 0 {
		return false
	}
	total, unsupported := 0, 0
	for i := range accounts {
		acc := &accounts[i]
		if excludedIDs != nil {
			if _, excluded := excludedIDs[acc.ID]; excluded {
				continue
			}
		}
		if !acc.IsOpenAI() || !acc.IsSchedulable() {
			continue
		}
		total++
		if !acc.IsModelSupported(requestedModel) {
			unsupported++
		}
	}
	return total > 0 && unsupported == total
}

func noAvailableOpenAISelectionError(requestedModel string, compactBlocked bool, accounts []Account, excludedIDs map[int64]struct{}) error {
	if compactBlocked {
		return ErrNoAvailableCompactAccounts
	}
	if requestedModel != "" {
		if allOpenAICandidatesModelUnsupported(accounts, excludedIDs, requestedModel) {
			return &ModelNotSupportedError{RequestedModel: requestedModel, Platform: PlatformOpenAI}
		}
		return &noAvailableOpenAIAccountsError{requestedModel: requestedModel}
	}
	return &noAvailableOpenAIAccountsError{}
}

// noAvailableOpenAIAccountsError 保留既有错误文案，同时通过 Unwrap 暴露
// ErrNoAvailableAccounts 语义，让调度入口能识别“零候选账号”类失败，
// 用于区分结构性不可服务（客户端 4xx）与暂时性不可用（503）。
type noAvailableOpenAIAccountsError struct {
	requestedModel string
}

func (e *noAvailableOpenAIAccountsError) Error() string {
	if e != nil && e.requestedModel != "" {
		return fmt.Sprintf("no available OpenAI accounts supporting model: %s", e.requestedModel)
	}
	return "no available OpenAI accounts"
}

// Unwrap 保持 errors.Is(err, ErrNoAvailableAccounts) 与旧行为一致，
// 让既有依赖 ErrNoAvailableAccounts 的错误处理与日志分类不受影响。
func (e *noAvailableOpenAIAccountsError) Unwrap() error {
	return ErrNoAvailableAccounts
}

type groupPlatformAccountsVerdict struct {
	hasAccounts bool
	expiresAt   time.Time
}

// groupPlatformAccountsVerdictTTL 是“分组是否拥有某平台账号”判定结果的进程内
// 缓存时长。该判定只发生在调度零候选的失败路径上，短 TTL 既限制失败风暴时的
// 额外数据库查询，也允许分组绑号变化在 30 秒内生效。
const groupPlatformAccountsVerdictTTL = 30 * time.Second

// groupHasAccountsForPlatform 判断分组内是否存在该平台的账号（忽略限流/过载等
// 瞬态不可用窗口），用于区分“结构性不可服务”（分组在该平台下没有任何账号）
// 与“暂时性不可用”（账号存在但被限流/过载/排队）。无法证明分组结构性为空时
// 一律返回 true，保持既有 503 语义，避免把平台侧饥饿误判为客户端误用。
func (s *OpenAIGatewayService) groupHasAccountsForPlatform(ctx context.Context, groupID *int64, platform string) bool {
	if s == nil || s.accountRepo == nil || groupID == nil || *groupID <= 0 || platform == "" {
		return true
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		// simple 模式从全平台池调度，分组归属不代表可服务性。
		return true
	}
	cacheKey := strconv.FormatInt(*groupID, 10) + "|" + platform
	if cached, ok := s.groupPlatformAccountVerdicts.Load(cacheKey); ok {
		if verdict, ok := cached.(groupPlatformAccountsVerdict); ok && time.Now().Before(verdict.expiresAt) {
			return verdict.hasAccounts
		}
	}
	accounts, err := s.accountRepo.ListByGroup(ctx, *groupID)
	if err != nil {
		return true
	}
	hasAccounts := false
	for i := range accounts {
		if accounts[i].Platform == platform {
			hasAccounts = true
			break
		}
	}
	s.groupPlatformAccountVerdicts.Store(cacheKey, groupPlatformAccountsVerdict{
		hasAccounts: hasAccounts,
		expiresAt:   time.Now().Add(groupPlatformAccountsVerdictTTL),
	})
	return hasAccounts
}

func openAICompactSupportTier(account *Account) int {
	if account == nil || !account.IsOpenAI() {
		return 0
	}
	supported, known := account.OpenAICompactSupportKnown()
	if !known {
		return 1
	}
	if supported {
		return 2
	}
	return 0
}

func isOpenAIAccountEligibleForRequest(account *Account, requestedModel string, requireCompact bool) bool {
	if account == nil || !account.IsSchedulable() || !account.IsOpenAI() {
		return false
	}
	if requestedModel != "" && !account.IsModelSupported(requestedModel) {
		return false
	}
	if requireCompact && openAICompactSupportTier(account) == 0 {
		return false
	}
	return true
}

func resolveOpenAIAccountUpstreamModelForRequest(account *Account, requestedModel string, requireCompact bool) string {
	upstreamModel := resolveOpenAIForwardModel(account, requestedModel, "")
	if upstreamModel == "" {
		return ""
	}
	if requireCompact {
		return resolveOpenAICompactForwardModel(account, upstreamModel)
	}
	return upstreamModel
}

func (s *OpenAIGatewayService) needsUpstreamChannelRestrictionCheck(ctx context.Context, groupID *int64) bool {
	if groupID == nil || s.channelService == nil {
		return false
	}
	ch, err := s.channelService.GetChannelForGroup(ctx, *groupID)
	if err != nil {
		slog.Warn("failed to check openai channel upstream restriction", "group_id", *groupID, "error", err)
		return false
	}
	if ch == nil || !ch.RestrictModels {
		return false
	}
	return ch.BillingModelSource == BillingModelSourceUpstream
}

// ReplaceModelInBody 替换请求体中的 JSON model 字段（通用 gjson/sjson 实现）。
func (s *OpenAIGatewayService) ReplaceModelInBody(body []byte, newModel string) []byte {
	return ReplaceModelInBody(body, newModel)
}

func (s *OpenAIGatewayService) getCodexSnapshotThrottle() *accountWriteThrottle {
	if s != nil && s.codexSnapshotThrottle != nil {
		return s.codexSnapshotThrottle
	}
	return defaultOpenAICodexSnapshotPersistThrottle
}

func (s *OpenAIGatewayService) billingDeps() *billingDeps {
	return &billingDeps{
		accountRepo:              s.accountRepo,
		userRepo:                 s.userRepo,
		userSubRepo:              s.userSubRepo,
		billingCacheService:      s.billingCacheService,
		deferredService:          s.deferredService,
		accountQuotaAlertService: s.accountQuotaAlertService,
		balanceAlertService:      s.balanceAlertService,
		commissionService:        s.commissionService,
	}
}

// CloseOpenAIWSPool 关闭 OpenAI WebSocket 连接池的后台 worker 和空闲连接。
// 应在应用优雅关闭时调用。
func (s *OpenAIGatewayService) CloseOpenAIWSPool() {
	if s != nil && s.openaiWSPool != nil {
		s.openaiWSPool.Close()
	}
}

func (s *OpenAIGatewayService) logOpenAIWSModeBootstrap() {
	if s == nil || s.cfg == nil {
		return
	}
	wsCfg := s.cfg.Gateway.OpenAIWS
	logOpenAIWSModeInfo(
		"bootstrap enabled=%v oauth_enabled=%v apikey_enabled=%v force_http=%v responses_websockets_v2=%v responses_websockets=%v payload_log_sample_rate=%.3f event_flush_batch_size=%d event_flush_interval_ms=%d prewarm_cooldown_ms=%d retry_backoff_initial_ms=%d retry_backoff_max_ms=%d retry_jitter_ratio=%.3f retry_total_budget_ms=%d ws_read_limit_bytes=%d",
		wsCfg.Enabled,
		wsCfg.OAuthEnabled,
		wsCfg.APIKeyEnabled,
		wsCfg.ForceHTTP,
		wsCfg.ResponsesWebsocketsV2,
		wsCfg.ResponsesWebsockets,
		wsCfg.PayloadLogSampleRate,
		wsCfg.EventFlushBatchSize,
		wsCfg.EventFlushIntervalMS,
		wsCfg.PrewarmCooldownMS,
		wsCfg.RetryBackoffInitialMS,
		wsCfg.RetryBackoffMaxMS,
		wsCfg.RetryJitterRatio,
		wsCfg.RetryTotalBudgetMS,
		openAIWSMessageReadLimitBytes,
	)
}

func (s *OpenAIGatewayService) getCodexClientRestrictionDetector() CodexClientRestrictionDetector {
	if s != nil && s.codexDetector != nil {
		return s.codexDetector
	}
	var cfg *config.Config
	if s != nil {
		cfg = s.cfg
	}
	return NewOpenAICodexClientRestrictionDetector(cfg)
}

func (s *OpenAIGatewayService) getOpenAIWSProtocolResolver() OpenAIWSProtocolResolver {
	if s != nil && s.openaiWSResolver != nil {
		return s.openaiWSResolver
	}
	var cfg *config.Config
	if s != nil {
		cfg = s.cfg
	}
	return NewOpenAIWSProtocolResolver(cfg)
}

func classifyOpenAIWSReconnectReason(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	var fallbackErr *openAIWSFallbackError
	if !errors.As(err, &fallbackErr) || fallbackErr == nil {
		return "", false
	}
	reason := strings.TrimSpace(fallbackErr.Reason)
	if reason == "" {
		return "", false
	}

	baseReason := strings.TrimPrefix(reason, "prewarm_")

	switch baseReason {
	case "policy_violation",
		"message_too_big",
		"upgrade_required",
		"ws_unsupported",
		"auth_failed",
		"invalid_encrypted_content",
		"previous_response_not_found":
		return reason, false
	}

	switch baseReason {
	case "read_event",
		"write_request",
		"write",
		"acquire_timeout",
		"acquire_conn",
		"conn_queue_full",
		"dial_failed",
		"upstream_5xx",
		"event_error",
		"error_event",
		"upstream_error_event",
		"ws_connection_limit_reached",
		"missing_final_response":
		return reason, true
	default:
		return reason, false
	}
}

func resolveOpenAIWSFallbackErrorResponse(err error) (statusCode int, errType string, clientMessage string, upstreamMessage string, ok bool) {
	if err == nil {
		return 0, "", "", "", false
	}
	var fallbackErr *openAIWSFallbackError
	if !errors.As(err, &fallbackErr) || fallbackErr == nil {
		return 0, "", "", "", false
	}

	reason := strings.TrimSpace(fallbackErr.Reason)
	reason = strings.TrimPrefix(reason, "prewarm_")
	if reason == "" {
		return 0, "", "", "", false
	}

	var dialErr *openAIWSDialError
	if fallbackErr.Err != nil && errors.As(fallbackErr.Err, &dialErr) && dialErr != nil {
		if dialErr.StatusCode > 0 {
			statusCode = dialErr.StatusCode
		}
		if dialErr.Err != nil {
			upstreamMessage = sanitizeUpstreamErrorMessage(strings.TrimSpace(dialErr.Err.Error()))
		}
	}

	switch reason {
	case "invalid_encrypted_content":
		if statusCode == 0 {
			statusCode = http.StatusBadRequest
		}
		errType = "invalid_request_error"
		if upstreamMessage == "" {
			upstreamMessage = "encrypted content could not be verified"
		}
	case "previous_response_not_found":
		if statusCode == 0 {
			statusCode = http.StatusBadRequest
		}
		errType = "invalid_request_error"
		if upstreamMessage == "" {
			upstreamMessage = "previous response not found"
		}
	case "upgrade_required":
		if statusCode == 0 {
			statusCode = http.StatusUpgradeRequired
		}
	case "ws_unsupported":
		if statusCode == 0 {
			statusCode = http.StatusBadRequest
		}
	case "auth_failed":
		if statusCode == 0 {
			statusCode = http.StatusUnauthorized
		}
	case "upstream_rate_limited":
		if statusCode == 0 {
			statusCode = http.StatusTooManyRequests
		}
	default:
		if statusCode == 0 {
			return 0, "", "", "", false
		}
	}

	if upstreamMessage == "" && fallbackErr.Err != nil {
		upstreamMessage = sanitizeUpstreamErrorMessage(strings.TrimSpace(fallbackErr.Err.Error()))
	}
	if upstreamMessage == "" {
		switch reason {
		case "upgrade_required":
			upstreamMessage = "upstream websocket upgrade required"
		case "ws_unsupported":
			upstreamMessage = "upstream websocket not supported"
		case "auth_failed":
			upstreamMessage = "upstream authentication failed"
		case "upstream_rate_limited":
			upstreamMessage = "upstream rate limit exceeded, please retry later"
		default:
			upstreamMessage = "Upstream request failed"
		}
	}

	clientSafe := SafeClientUpstreamError(statusCode)
	if reason != "previous_response_not_found" {
		statusCode = clientSafe.StatusCode
	}
	if errType == "" {
		errType = clientSafe.Type
	}
	clientMessage = clientSafe.Message
	return statusCode, errType, clientMessage, upstreamMessage, true
}

func (s *OpenAIGatewayService) writeOpenAIWSFallbackErrorResponse(c *gin.Context, account *Account, wsErr error) bool {
	if c == nil || c.Writer == nil || c.Writer.Written() {
		return false
	}
	statusCode, errType, clientMessage, upstreamMessage, ok := resolveOpenAIWSFallbackErrorResponse(wsErr)
	if !ok {
		return false
	}
	if strings.TrimSpace(clientMessage) == "" {
		clientMessage = SafeClientUpstreamError(statusCode).Message
	}
	if strings.TrimSpace(upstreamMessage) == "" {
		upstreamMessage = clientMessage
	}

	setOpsUpstreamError(c, statusCode, upstreamMessage, "")
	if account != nil {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: statusCode,
			Kind:               "ws_error",
			Message:            upstreamMessage,
		})
	}
	c.JSON(statusCode, OpenAIClientErrorEnvelope(c, errType, clientMessage))
	return true
}

func (s *OpenAIGatewayService) openAIWSRetryBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	initial := openAIWSRetryBackoffInitialDefault
	maxBackoff := openAIWSRetryBackoffMaxDefault
	jitterRatio := openAIWSRetryJitterRatioDefault
	if s != nil && s.cfg != nil {
		wsCfg := s.cfg.Gateway.OpenAIWS
		if wsCfg.RetryBackoffInitialMS > 0 {
			initial = time.Duration(wsCfg.RetryBackoffInitialMS) * time.Millisecond
		}
		if wsCfg.RetryBackoffMaxMS > 0 {
			maxBackoff = time.Duration(wsCfg.RetryBackoffMaxMS) * time.Millisecond
		}
		if wsCfg.RetryJitterRatio >= 0 {
			jitterRatio = wsCfg.RetryJitterRatio
		}
	}
	if initial <= 0 {
		return 0
	}
	if maxBackoff <= 0 {
		maxBackoff = initial
	}
	if maxBackoff < initial {
		maxBackoff = initial
	}
	if jitterRatio < 0 {
		jitterRatio = 0
	}
	if jitterRatio > 1 {
		jitterRatio = 1
	}

	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	backoff := initial
	if shift > 0 {
		backoff = initial * time.Duration(1<<shift)
	}
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	if jitterRatio <= 0 {
		return backoff
	}
	jitter := time.Duration(float64(backoff) * jitterRatio)
	if jitter <= 0 {
		return backoff
	}
	delta := time.Duration(rand.Int63n(int64(jitter)*2+1)) - jitter
	withJitter := backoff + delta
	if withJitter < 0 {
		return 0
	}
	return withJitter
}

func (s *OpenAIGatewayService) openAIWSRetryTotalBudget() time.Duration {
	if s != nil && s.cfg != nil {
		ms := s.cfg.Gateway.OpenAIWS.RetryTotalBudgetMS
		if ms <= 0 {
			return 0
		}
		return time.Duration(ms) * time.Millisecond
	}
	return 0
}

func (s *OpenAIGatewayService) recordOpenAIWSRetryAttempt(backoff time.Duration) {
	if s == nil {
		return
	}
	s.openaiWSRetryMetrics.retryAttempts.Add(1)
	if backoff > 0 {
		s.openaiWSRetryMetrics.retryBackoffMs.Add(backoff.Milliseconds())
	}
}

func (s *OpenAIGatewayService) recordOpenAIWSRetryExhausted() {
	if s == nil {
		return
	}
	s.openaiWSRetryMetrics.retryExhausted.Add(1)
}

func (s *OpenAIGatewayService) recordOpenAIWSNonRetryableFastFallback() {
	if s == nil {
		return
	}
	s.openaiWSRetryMetrics.nonRetryableFastFallback.Add(1)
}

func (s *OpenAIGatewayService) SnapshotOpenAIWSRetryMetrics() OpenAIWSRetryMetricsSnapshot {
	if s == nil {
		return OpenAIWSRetryMetricsSnapshot{}
	}
	return OpenAIWSRetryMetricsSnapshot{
		RetryAttemptsTotal:            s.openaiWSRetryMetrics.retryAttempts.Load(),
		RetryBackoffMsTotal:           s.openaiWSRetryMetrics.retryBackoffMs.Load(),
		RetryExhaustedTotal:           s.openaiWSRetryMetrics.retryExhausted.Load(),
		NonRetryableFastFallbackTotal: s.openaiWSRetryMetrics.nonRetryableFastFallback.Load(),
	}
}

func SnapshotOpenAICompatibilityFallbackMetrics() OpenAICompatibilityFallbackMetricsSnapshot {
	legacyReadFallbackTotal, legacyReadFallbackHit, legacyDualWriteTotal := openAIStickyCompatStats()
	isMaxTokensOneHaiku, thinkingEnabled, prefetchedStickyAccount, prefetchedStickyGroup, singleAccountRetry, accountSwitchCount := RequestMetadataFallbackStats()

	readHitRate := float64(0)
	if legacyReadFallbackTotal > 0 {
		readHitRate = float64(legacyReadFallbackHit) / float64(legacyReadFallbackTotal)
	}
	metadataFallbackTotal := isMaxTokensOneHaiku + thinkingEnabled + prefetchedStickyAccount + prefetchedStickyGroup + singleAccountRetry + accountSwitchCount

	return OpenAICompatibilityFallbackMetricsSnapshot{
		SessionHashLegacyReadFallbackTotal: legacyReadFallbackTotal,
		SessionHashLegacyReadFallbackHit:   legacyReadFallbackHit,
		SessionHashLegacyDualWriteTotal:    legacyDualWriteTotal,
		SessionHashLegacyReadHitRate:       readHitRate,

		MetadataLegacyFallbackIsMaxTokensOneHaikuTotal: isMaxTokensOneHaiku,
		MetadataLegacyFallbackThinkingEnabledTotal:     thinkingEnabled,
		MetadataLegacyFallbackPrefetchedStickyAccount:  prefetchedStickyAccount,
		MetadataLegacyFallbackPrefetchedStickyGroup:    prefetchedStickyGroup,
		MetadataLegacyFallbackSingleAccountRetryTotal:  singleAccountRetry,
		MetadataLegacyFallbackAccountSwitchCountTotal:  accountSwitchCount,
		MetadataLegacyFallbackTotal:                    metadataFallbackTotal,
	}
}

func (s *OpenAIGatewayService) detectCodexClientRestriction(c *gin.Context, account *Account) CodexClientRestrictionDetectionResult {
	return s.getCodexClientRestrictionDetector().Detect(c, account)
}

func getAPIKeyIDFromContext(c *gin.Context) int64 {
	if c == nil {
		return 0
	}
	v, exists := c.Get("api_key")
	if !exists {
		return 0
	}
	apiKey, ok := v.(*APIKey)
	if !ok || apiKey == nil {
		return 0
	}
	return apiKey.ID
}

// isolateOpenAISessionID 将 apiKeyID 混入 session 标识符，
// 确保不同 API Key 的用户即使使用相同的原始 session_id/conversation_id，
// 到达上游的标识符也不同，防止跨用户会话碰撞。
func isolateOpenAISessionID(apiKeyID int64, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	h := xxhash.New()
	_, _ = fmt.Fprintf(h, "k%d:", apiKeyID)
	_, _ = h.WriteString(raw)
	return fmt.Sprintf("%016x", h.Sum64())
}

func logCodexCLIOnlyDetection(ctx context.Context, c *gin.Context, account *Account, apiKeyID int64, result CodexClientRestrictionDetectionResult, body []byte) {
	if !result.Enabled {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	fields := []zap.Field{
		zap.String("component", "service.openai_gateway"),
		zap.Int64("account_id", accountID),
		zap.Bool("codex_cli_only_enabled", result.Enabled),
		zap.Bool("codex_official_client_match", result.Matched),
		zap.String("reject_reason", result.Reason),
	}
	if apiKeyID > 0 {
		fields = append(fields, zap.Int64("api_key_id", apiKeyID))
	}
	if !result.Matched {
		fields = appendCodexCLIOnlyRejectedRequestFields(fields, c, body)
	}
	log := logger.FromContext(ctx).With(fields...)
	if result.Matched {
		return
	}
	log.Warn("OpenAI codex_cli_only 拒绝非官方客户端请求")
}

func appendCodexCLIOnlyRejectedRequestFields(fields []zap.Field, c *gin.Context, body []byte) []zap.Field {
	if c == nil || c.Request == nil {
		return fields
	}

	req := c.Request
	requestModel, requestStream, promptCacheKey := extractOpenAIRequestMetaFromBody(body)
	fields = append(fields,
		zap.String("request_method", strings.TrimSpace(req.Method)),
		zap.String("request_path", strings.TrimSpace(req.URL.Path)),
		zap.String("request_query", strings.TrimSpace(req.URL.RawQuery)),
		zap.String("request_host", strings.TrimSpace(req.Host)),
		zap.String("request_client_ip", strings.TrimSpace(ip.GetClientIP(c))),
		zap.String("request_remote_addr", strings.TrimSpace(req.RemoteAddr)),
		zap.String("request_user_agent", strings.TrimSpace(req.Header.Get("User-Agent"))),
		zap.String("request_content_type", strings.TrimSpace(req.Header.Get("Content-Type"))),
		zap.Int64("request_content_length", req.ContentLength),
		zap.Bool("request_stream", requestStream),
	)
	if requestModel != "" {
		fields = append(fields, zap.String("request_model", requestModel))
	}
	if promptCacheKey != "" {
		fields = append(fields, zap.String("request_prompt_cache_key_sha256", hashSensitiveValueForLog(promptCacheKey)))
	}

	if headers := snapshotCodexCLIOnlyHeaders(req.Header); len(headers) > 0 {
		fields = append(fields, zap.Any("request_headers", headers))
	}
	fields = append(fields, zap.Int("request_body_size", len(body)))
	return fields
}

func snapshotCodexCLIOnlyHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}
	result := make(map[string]string, len(codexCLIOnlyDebugHeaderWhitelist))
	for _, key := range codexCLIOnlyDebugHeaderWhitelist {
		value := strings.TrimSpace(header.Get(key))
		if value == "" {
			continue
		}
		result[strings.ToLower(key)] = truncateString(value, codexCLIOnlyHeaderValueMaxBytes)
	}
	return result
}

func hashSensitiveValueForLog(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func logOpenAIInstructionsRequiredDebug(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	upstreamStatusCode int,
	upstreamMsg string,
	requestBody []byte,
	upstreamBody []byte,
) {
	msg := strings.TrimSpace(upstreamMsg)
	if !isOpenAIInstructionsRequiredError(upstreamStatusCode, msg, upstreamBody) {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	accountID := int64(0)
	accountName := ""
	if account != nil {
		accountID = account.ID
		accountName = strings.TrimSpace(account.Name)
	}

	userAgent := ""
	originator := ""
	if c != nil {
		userAgent = strings.TrimSpace(c.GetHeader("User-Agent"))
		originator = strings.TrimSpace(c.GetHeader("originator"))
	}

	fields := []zap.Field{
		zap.String("component", "service.openai_gateway"),
		zap.Int64("account_id", accountID),
		zap.String("account_name", accountName),
		zap.Int("upstream_status_code", upstreamStatusCode),
		zap.String("upstream_error_message", msg),
		zap.String("request_user_agent", userAgent),
		zap.Bool("codex_official_client_match", openai.IsCodexOfficialClientByHeaders(userAgent, originator)),
	}
	fields = appendCodexCLIOnlyRejectedRequestFields(fields, c, requestBody)

	logger.FromContext(ctx).With(fields...).Warn("OpenAI 上游返回 Instructions are required，已记录请求详情用于排查")
}

func isOpenAIInstructionsRequiredError(upstreamStatusCode int, upstreamMsg string, upstreamBody []byte) bool {
	if upstreamStatusCode != http.StatusBadRequest {
		return false
	}

	hasInstructionRequired := func(text string) bool {
		lower := strings.ToLower(strings.TrimSpace(text))
		if lower == "" {
			return false
		}
		if strings.Contains(lower, "instructions are required") {
			return true
		}
		if strings.Contains(lower, "required parameter: 'instructions'") {
			return true
		}
		if strings.Contains(lower, "required parameter: instructions") {
			return true
		}
		if strings.Contains(lower, "missing required parameter") && strings.Contains(lower, "instructions") {
			return true
		}
		return strings.Contains(lower, "instruction") && strings.Contains(lower, "required")
	}

	if hasInstructionRequired(upstreamMsg) {
		return true
	}
	if len(upstreamBody) == 0 {
		return false
	}

	errMsg := gjson.GetBytes(upstreamBody, "error.message").String()
	errMsgLower := strings.ToLower(strings.TrimSpace(errMsg))
	errCode := strings.ToLower(strings.TrimSpace(gjson.GetBytes(upstreamBody, "error.code").String()))
	errParam := strings.ToLower(strings.TrimSpace(gjson.GetBytes(upstreamBody, "error.param").String()))
	errType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(upstreamBody, "error.type").String()))

	if errParam == "instructions" {
		return true
	}
	if hasInstructionRequired(errMsg) {
		return true
	}
	if strings.Contains(errCode, "missing_required_parameter") && strings.Contains(errMsgLower, "instructions") {
		return true
	}
	if strings.Contains(errType, "invalid_request") && strings.Contains(errMsgLower, "instructions") && strings.Contains(errMsgLower, "required") {
		return true
	}

	return false
}

func isOpenAITransientProcessingError(upstreamStatusCode int, upstreamMsg string, upstreamBody []byte) bool {
	if upstreamStatusCode != http.StatusBadRequest {
		return false
	}

	match := func(text string) bool {
		lower := strings.ToLower(strings.TrimSpace(text))
		if lower == "" {
			return false
		}
		if strings.Contains(lower, "an error occurred while processing your request") {
			return true
		}
		return strings.Contains(lower, "you can retry your request") &&
			strings.Contains(lower, "help.openai.com") &&
			strings.Contains(lower, "request id")
	}

	if match(upstreamMsg) {
		return true
	}
	if len(upstreamBody) == 0 {
		return false
	}
	if match(gjson.GetBytes(upstreamBody, "error.message").String()) {
		return true
	}
	return match(string(upstreamBody))
}

const openAISessionBlockedByCyberPolicyCode = "session_blocked_by_cyber_policy"

func isOpenAISessionBlockedByCyberPolicy(upstreamBody []byte) bool {
	if len(upstreamBody) == 0 {
		return false
	}
	for _, path := range []string{"error.code", "response.error.code"} {
		code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(upstreamBody, path).String()))
		if code == openAISessionBlockedByCyberPolicyCode {
			return true
		}
	}
	return false
}

// allowOpenAISameAccountRetry keeps the existing retry classification except
// for request-scoped session policy blocks. Replaying an identical blocked
// session on the same supplier cannot recover it; the handler should switch to
// the next eligible upstream account immediately instead.
func allowOpenAISameAccountRetry(retryable bool, upstreamBody []byte) bool {
	return retryable && !isOpenAISessionBlockedByCyberPolicy(upstreamBody)
}

// ExtractSessionID extracts the raw session ID from headers or body without hashing.
// Used by ForwardAsAnthropic to pass as prompt_cache_key for upstream cache.
func (s *OpenAIGatewayService) ExtractSessionID(c *gin.Context, body []byte) string {
	return explicitOpenAIRequestSessionID(c, body)
}

func explicitOpenAIRequestSessionID(c *gin.Context, body []byte) string {
	if c == nil {
		return ""
	}
	sessionID := explicitOpenAIHeaderSessionID(c)
	if sessionID == "" && isGrokRequestContext(c) {
		sessionID = strings.TrimSpace(c.GetHeader(grokConversationIDHeader))
	}
	if sessionID == "" && len(body) > 0 {
		sessionID = strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
	}
	return sessionID
}

// GenerateExplicitSessionHash generates a sticky-session hash only from explicit
// client session signals. Stateless endpoints such as images should not fall
// back to content-derived sticky sessions.
func (s *OpenAIGatewayService) GenerateExplicitSessionHash(c *gin.Context, body []byte) string {
	sessionID := explicitOpenAIRequestSessionID(c, body)
	if sessionID == "" {
		return ""
	}
	currentHash, legacyHash := deriveOpenAISessionHashes(sessionID)
	attachOpenAILegacySessionHashToGin(c, legacyHash)
	return currentHash
}

// GenerateSessionHash generates a sticky-session hash for OpenAI requests.
//
// Priority:
//  1. Header: session_id
//  2. Header: conversation_id
//  3. Body:   prompt_cache_key (opencode)
//  4. Body:   content-based fallback (model + system + tools + first user message)
func (s *OpenAIGatewayService) GenerateSessionHash(c *gin.Context, body []byte) string {
	if c == nil {
		return ""
	}

	sessionID := explicitOpenAIRequestSessionID(c, body)
	if sessionID == "" && len(body) > 0 {
		sessionID = deriveOpenAIContentSessionSeed(body)
	}
	if sessionID == "" {
		return ""
	}

	currentHash, legacyHash := deriveOpenAISessionHashes(sessionID)
	attachOpenAILegacySessionHashToGin(c, legacyHash)
	return currentHash
}

// GenerateSessionHashWithFallback 先按常规信号生成会话哈希；
// 当未携带 session_id/conversation_id/prompt_cache_key 时，使用 fallbackSeed 生成稳定哈希。
// 该方法用于 WS ingress，避免会话信号缺失时发生跨账号漂移。
func (s *OpenAIGatewayService) GenerateSessionHashWithFallback(c *gin.Context, body []byte, fallbackSeed string) string {
	sessionHash := s.GenerateSessionHash(c, body)
	if sessionHash != "" {
		return sessionHash
	}

	seed := strings.TrimSpace(fallbackSeed)
	if seed == "" {
		return ""
	}

	currentHash, legacyHash := deriveOpenAISessionHashes(seed)
	attachOpenAILegacySessionHashToGin(c, legacyHash)
	return currentHash
}

func resolveOpenAIUpstreamOriginator(c *gin.Context, isOfficialClient bool) string {
	if c != nil {
		if originator := strings.TrimSpace(c.GetHeader("originator")); originator != "" {
			return originator
		}
	}
	if isOfficialClient {
		return "codex_cli_rs"
	}
	return "opencode"
}

// BindStickySession sets session -> account binding with standard TTL.
func (s *OpenAIGatewayService) BindStickySession(ctx context.Context, groupID *int64, sessionHash string, accountID int64) error {
	if sessionHash == "" || accountID <= 0 {
		return nil
	}
	ttl := openaiStickySessionTTL
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds > 0 {
		ttl = time.Duration(s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds) * time.Second
	}
	return s.setStickySessionAccountID(ctx, groupID, sessionHash, accountID, ttl)
}

// SelectAccount selects an OpenAI account with sticky session support
func (s *OpenAIGatewayService) SelectAccount(ctx context.Context, groupID *int64, sessionHash string) (*Account, error) {
	return s.SelectAccountForModel(ctx, groupID, sessionHash, "")
}

// SelectAccountForModel selects an account supporting the requested model
func (s *OpenAIGatewayService) SelectAccountForModel(ctx context.Context, groupID *int64, sessionHash string, requestedModel string) (*Account, error) {
	return s.SelectAccountForModelWithExclusions(ctx, groupID, sessionHash, requestedModel, nil)
}

// SelectAccountForModelWithExclusions selects an account supporting the requested model while excluding specified accounts.
// SelectAccountForModelWithExclusions 选择支持指定模型的账号，同时排除指定的账号。
func (s *OpenAIGatewayService) SelectAccountForModelWithExclusions(ctx context.Context, groupID *int64, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}) (*Account, error) {
	return s.selectAccountForModelWithExclusions(ctx, groupID, sessionHash, requestedModel, excludedIDs, false, 0)
}

func (s *OpenAIGatewayService) selectAccountForModelWithExclusions(ctx context.Context, groupID *int64, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}, requireCompact bool, stickyAccountID int64) (*Account, error) {
	if s.checkChannelPricingRestriction(ctx, groupID, requestedModel) {
		slog.Warn("channel pricing restriction blocked request",
			"group_id", derefGroupID(groupID),
			"model", requestedModel)
		return nil, fmt.Errorf("%w supporting model: %s (channel pricing restriction)", ErrNoAvailableAccounts, requestedModel)
	}

	// 1. 尝试粘性会话命中
	// Try sticky session hit
	if account := s.tryStickySessionHit(ctx, groupID, sessionHash, requestedModel, excludedIDs, requireCompact, stickyAccountID); account != nil {
		return account, nil
	}

	// 2. 获取可调度的 OpenAI 账号
	// Get schedulable OpenAI accounts
	accounts, err := s.listSchedulableAccounts(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("query accounts failed: %w", err)
	}

	// 3. 按优先级 + LRU 选择最佳账号
	// Select by priority + LRU
	selected, compactBlocked := s.selectBestAccount(ctx, groupID, accounts, requestedModel, excludedIDs, requireCompact)

	if selected == nil {
		return nil, noAvailableOpenAISelectionError(requestedModel, compactBlocked, accounts, excludedIDs)
	}

	// 4. 设置粘性会话绑定
	// Set sticky session binding
	if sessionHash != "" {
		_ = s.setStickySessionAccountID(ctx, groupID, sessionHash, selected.ID, openaiStickySessionTTL)
	}

	return s.hydrateSelectedAccount(ctx, selected)
}

// tryStickySessionHit 尝试从粘性会话获取账号。
// 如果命中且账号可用则返回账号；如果账号不可用则清理会话并返回 nil。
//
// tryStickySessionHit attempts to get account from sticky session.
// Returns account if hit and usable; clears session and returns nil if account is unavailable.
func (s *OpenAIGatewayService) tryStickySessionHit(ctx context.Context, groupID *int64, sessionHash, requestedModel string, excludedIDs map[int64]struct{}, requireCompact bool, stickyAccountID int64) *Account {
	if sessionHash == "" {
		return nil
	}

	accountID := stickyAccountID
	if accountID <= 0 {
		var err error
		accountID, err = s.getStickySessionAccountID(ctx, groupID, sessionHash)
		if err != nil || accountID <= 0 {
			return nil
		}
	}

	if _, excluded := excludedIDs[accountID]; excluded {
		return nil
	}

	account, err := s.getSchedulableAccount(ctx, accountID)
	if err != nil {
		return nil
	}

	// 检查账号是否需要清理粘性会话
	// Check if sticky session should be cleared
	if shouldClearStickySession(account, requestedModel) {
		_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
		return nil
	}

	// 验证账号是否可用于当前请求
	// Verify account is usable for current request
	if !isOpenAIAccountEligibleForRequest(account, requestedModel, requireCompact) {
		return nil
	}
	account = s.recheckSelectedOpenAIAccountFromDB(ctx, account, requestedModel, requireCompact)
	if account == nil {
		_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
		return nil
	}
	if groupID != nil && s.needsUpstreamChannelRestrictionCheck(ctx, groupID) &&
		s.isUpstreamModelRestrictedByChannel(ctx, *groupID, account, requestedModel, requireCompact) {
		_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
		return nil
	}

	// 刷新会话 TTL 并返回账号
	// Refresh session TTL and return account
	_ = s.refreshStickySessionTTL(ctx, groupID, sessionHash, openaiStickySessionTTL)
	return account
}

// selectBestAccount 从候选账号中选择最佳账号（优先级 + LRU）。
// 返回 nil 表示无可用账号。
//
// selectBestAccount selects the best account from candidates (priority + LRU).
// Returns nil if no available account.
func (s *OpenAIGatewayService) selectBestAccount(ctx context.Context, groupID *int64, accounts []Account, requestedModel string, excludedIDs map[int64]struct{}, requireCompact bool) (*Account, bool) {
	var selected *Account
	needsUpstreamCheck := s.needsUpstreamChannelRestrictionCheck(ctx, groupID)
	selectedCompactTier := -1
	compactBlocked := false

	for i := range accounts {
		acc := &accounts[i]

		// 跳过被排除的账号
		// Skip excluded accounts
		if _, excluded := excludedIDs[acc.ID]; excluded {
			continue
		}

		fresh := s.resolveFreshSchedulableOpenAIAccount(ctx, acc, requestedModel, requireCompact)
		if fresh == nil {
			if requireCompact && acc.IsOpenAI() && openAICompactSupportTier(acc) == 0 {
				compactBlocked = true
			}
			continue
		}
		fresh = s.recheckSelectedOpenAIAccountFromDB(ctx, fresh, requestedModel, requireCompact)
		if fresh == nil {
			continue
		}
		if needsUpstreamCheck && s.isUpstreamModelRestrictedByChannel(ctx, *groupID, fresh, requestedModel, requireCompact) {
			continue
		}
		compactTier := 0
		if requireCompact {
			compactTier = openAICompactSupportTier(fresh)
			if compactTier == 0 {
				compactBlocked = true
				continue
			}
		}

		// 选择优先级最高且最久未使用的账号
		// Select highest priority and least recently used
		if selected == nil {
			selected = fresh
			selectedCompactTier = compactTier
			continue
		}

		if requireCompact && compactTier != selectedCompactTier {
			if compactTier > selectedCompactTier {
				selected = fresh
				selectedCompactTier = compactTier
			}
			continue
		}
		if s.isBetterAccount(fresh, selected, groupID) {
			selected = fresh
			selectedCompactTier = compactTier
		}
	}

	return selected, compactBlocked
}

// isBetterAccount 判断 candidate 是否比 current 更优。
// 规则：分组绑定优先级更高（数值更小）优先；同优先级时，未使用过的优先，其次是最久未使用的。
//
// isBetterAccount checks if candidate is better than current.
// Rules: higher priority (lower value) wins; same priority: never used > least recently used.
func (s *OpenAIGatewayService) isBetterAccount(candidate, current *Account, groupID *int64) bool {
	// 分组绑定优先级更高（数值更小）
	// Higher group binding priority (lower value)
	cp, cp2 := candidate.EffectivePriorityForGroup(groupID), current.EffectivePriorityForGroup(groupID)
	if cp < cp2 {
		return true
	}
	if cp > cp2 {
		return false
	}

	// 同优先级，比较最后使用时间
	// Same priority, compare last used time
	switch {
	case candidate.LastUsedAt == nil && current.LastUsedAt != nil:
		// candidate 从未使用，优先
		return true
	case candidate.LastUsedAt != nil && current.LastUsedAt == nil:
		// current 从未使用，保持
		return false
	case candidate.LastUsedAt == nil && current.LastUsedAt == nil:
		// 都未使用，保持
		return false
	default:
		// 都使用过，选择最久未使用的
		return candidate.LastUsedAt.Before(*current.LastUsedAt)
	}
}

// SelectAccountWithLoadAwareness selects an account with load-awareness and wait plan.
func (s *OpenAIGatewayService) SelectAccountWithLoadAwareness(ctx context.Context, groupID *int64, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}) (*AccountSelectionResult, error) {
	return s.selectAccountWithLoadAwareness(ctx, groupID, sessionHash, requestedModel, excludedIDs, false)
}

func (s *OpenAIGatewayService) selectAccountWithLoadAwareness(ctx context.Context, groupID *int64, sessionHash string, requestedModel string, excludedIDs map[int64]struct{}, requireCompact bool) (*AccountSelectionResult, error) {
	if s.checkChannelPricingRestriction(ctx, groupID, requestedModel) {
		slog.Warn("channel pricing restriction blocked request",
			"group_id", derefGroupID(groupID),
			"model", requestedModel)
		return nil, fmt.Errorf("%w supporting model: %s (channel pricing restriction)", ErrNoAvailableAccounts, requestedModel)
	}

	cfg := s.schedulingConfig()
	needsUpstreamCheck := s.needsUpstreamChannelRestrictionCheck(ctx, groupID)
	var stickyAccountID int64
	if sessionHash != "" && s.cache != nil {
		if accountID, err := s.getStickySessionAccountID(ctx, groupID, sessionHash); err == nil {
			stickyAccountID = accountID
		}
	}
	if s.concurrencyService == nil || !cfg.LoadBatchEnabled {
		account, err := s.selectAccountForModelWithExclusions(ctx, groupID, sessionHash, requestedModel, excludedIDs, requireCompact, stickyAccountID)
		if err != nil {
			return nil, err
		}
		result, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
		if err == nil && result.Acquired {
			return s.newSelectionResult(ctx, account, true, result.ReleaseFunc, nil)
		}
		if stickyAccountID > 0 && stickyAccountID == account.ID && s.concurrencyService != nil {
			waitingCount, _ := s.concurrencyService.GetAccountWaitingCount(ctx, account.ID)
			if waitingCount < cfg.StickySessionMaxWaiting {
				return s.newSelectionResult(ctx, account, false, nil, &AccountWaitPlan{
					AccountID:      account.ID,
					MaxConcurrency: account.Concurrency,
					Timeout:        cfg.StickySessionWaitTimeout,
					MaxWaiting:     cfg.StickySessionMaxWaiting,
				})
			}
		}
		return s.newSelectionResult(ctx, account, false, nil, &AccountWaitPlan{
			AccountID:      account.ID,
			MaxConcurrency: account.Concurrency,
			Timeout:        cfg.FallbackWaitTimeout,
			MaxWaiting:     cfg.FallbackMaxWaiting,
		})
	}

	accounts, err := s.listSchedulableAccounts(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrNoAvailableAccounts
	}

	isExcluded := func(accountID int64) bool {
		if excludedIDs == nil {
			return false
		}
		_, excluded := excludedIDs[accountID]
		return excluded
	}

	compactBlocked := false

	// ============ Layer 1: Sticky session ============
	// A full sticky wait queue may spill one request to another healthy account,
	// but that temporary capacity decision must not migrate the durable binding.
	stickySpillover := false
	if sessionHash != "" {
		accountID := stickyAccountID
		if accountID > 0 && !isExcluded(accountID) {
			account, err := s.getSchedulableAccount(ctx, accountID)
			if err == nil {
				clearSticky := shouldClearStickySession(account, requestedModel)
				if clearSticky {
					_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
				}
				if !clearSticky && isOpenAIAccountEligibleForRequest(account, requestedModel, requireCompact) {
					account = s.recheckSelectedOpenAIAccountFromDB(ctx, account, requestedModel, requireCompact)
					if account == nil {
						_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
					} else if needsUpstreamCheck && s.isUpstreamModelRestrictedByChannel(ctx, *groupID, account, requestedModel, requireCompact) {
						_ = s.deleteStickySessionAccountID(ctx, groupID, sessionHash)
					} else {
						result, err := s.tryAcquireAccountSlot(ctx, accountID, account.Concurrency)
						if err == nil && result.Acquired {
							_ = s.refreshStickySessionTTL(ctx, groupID, sessionHash, openaiStickySessionTTL)
							return s.newSelectionResult(ctx, account, true, result.ReleaseFunc, nil)
						}

						waitingCount, _ := s.concurrencyService.GetAccountWaitingCount(ctx, accountID)
						if waitingCount < cfg.StickySessionMaxWaiting {
							return s.newSelectionResult(ctx, account, false, nil, &AccountWaitPlan{
								AccountID:      accountID,
								MaxConcurrency: account.Concurrency,
								Timeout:        cfg.StickySessionWaitTimeout,
								MaxWaiting:     cfg.StickySessionMaxWaiting,
							})
						}
						stickySpillover = true
					}
				}
			}
		}
	}

	// ============ Layer 2: Load-aware selection ============
	candidates := make([]*Account, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		if isExcluded(acc.ID) {
			continue
		}
		// Scheduler snapshots can be temporarily stale (bucket rebuild is throttled);
		// re-check schedulability here so recently rate-limited/overloaded accounts
		// are not selected again before the bucket is rebuilt.
		if !acc.IsSchedulable() {
			continue
		}
		if !isOpenAIAccountEligibleForRequest(acc, requestedModel, requireCompact) {
			if requireCompact && openAICompactSupportTier(acc) == 0 {
				compactBlocked = true
			}
			continue
		}
		if needsUpstreamCheck && s.isUpstreamModelRestrictedByChannel(ctx, *groupID, acc, requestedModel, requireCompact) {
			continue
		}
		candidates = append(candidates, acc)
	}

	if len(candidates) == 0 {
		return nil, noAvailableOpenAISelectionError(requestedModel, compactBlocked, accounts, excludedIDs)
	}

	accountLoads := make([]AccountWithConcurrency, 0, len(candidates))
	for _, acc := range candidates {
		accountLoads = append(accountLoads, AccountWithConcurrency{
			ID:             acc.ID,
			MaxConcurrency: acc.EffectiveLoadFactor(),
		})
	}

	loadMap, err := s.concurrencyService.GetAccountsLoadBatch(ctx, accountLoads)
	if err != nil {
		ordered := append([]*Account(nil), candidates...)
		sortAccountsByPriorityAndLastUsed(ordered, false, groupID)
		for _, acc := range ordered {
			fresh := s.resolveFreshSchedulableOpenAIAccount(ctx, acc, requestedModel, requireCompact)
			if fresh == nil {
				continue
			}
			if needsUpstreamCheck && s.isUpstreamModelRestrictedByChannel(ctx, *groupID, fresh, requestedModel, requireCompact) {
				continue
			}
			result, err := s.tryAcquireAccountSlot(ctx, fresh.ID, fresh.Concurrency)
			if err == nil && result.Acquired {
				if sessionHash != "" && !stickySpillover {
					_ = s.setStickySessionAccountID(ctx, groupID, sessionHash, fresh.ID, openaiStickySessionTTL)
				}
				return s.newSelectionResult(ctx, fresh, true, result.ReleaseFunc, nil)
			}
		}
	} else {
		var available []accountWithLoad
		for _, acc := range candidates {
			loadInfo := loadMap[acc.ID]
			if loadInfo == nil {
				loadInfo = &AccountLoadInfo{AccountID: acc.ID}
			}
			if loadInfo.LoadRate < 100 {
				available = append(available, accountWithLoad{
					account:  acc,
					loadInfo: loadInfo,
				})
			}
		}

		if len(available) > 0 {
			sort.SliceStable(available, func(i, j int) bool {
				a, b := available[i], available[j]
				pa, pb := a.account.EffectivePriorityForGroup(groupID), b.account.EffectivePriorityForGroup(groupID)
				if pa != pb {
					return pa < pb
				}
				if a.loadInfo.LoadRate != b.loadInfo.LoadRate {
					return a.loadInfo.LoadRate < b.loadInfo.LoadRate
				}
				switch {
				case a.account.LastUsedAt == nil && b.account.LastUsedAt != nil:
					return true
				case a.account.LastUsedAt != nil && b.account.LastUsedAt == nil:
					return false
				case a.account.LastUsedAt == nil && b.account.LastUsedAt == nil:
					return false
				default:
					return a.account.LastUsedAt.Before(*b.account.LastUsedAt)
				}
			})
			shuffleWithinSortGroups(available, groupID)

			for _, item := range available {
				fresh := s.resolveFreshSchedulableOpenAIAccount(ctx, item.account, requestedModel, requireCompact)
				if fresh == nil {
					continue
				}
				if needsUpstreamCheck && s.isUpstreamModelRestrictedByChannel(ctx, *groupID, fresh, requestedModel, requireCompact) {
					continue
				}
				result, err := s.tryAcquireAccountSlot(ctx, fresh.ID, fresh.Concurrency)
				if err == nil && result.Acquired {
					if sessionHash != "" && !stickySpillover {
						_ = s.setStickySessionAccountID(ctx, groupID, sessionHash, fresh.ID, openaiStickySessionTTL)
					}
					return s.newSelectionResult(ctx, fresh, true, result.ReleaseFunc, nil)
				}
			}
		}
	}

	// ============ Layer 3: Fallback wait ============
	sortAccountsByPriorityAndLastUsed(candidates, false, groupID)
	for _, acc := range candidates {
		fresh := s.resolveFreshSchedulableOpenAIAccount(ctx, acc, requestedModel, requireCompact)
		if fresh == nil {
			continue
		}
		if needsUpstreamCheck && s.isUpstreamModelRestrictedByChannel(ctx, *groupID, fresh, requestedModel, requireCompact) {
			continue
		}
		return s.newSelectionResult(ctx, fresh, false, nil, &AccountWaitPlan{
			AccountID:      fresh.ID,
			MaxConcurrency: fresh.Concurrency,
			Timeout:        cfg.FallbackWaitTimeout,
			MaxWaiting:     cfg.FallbackMaxWaiting,
		})
	}

	return nil, noAvailableOpenAISelectionError(requestedModel, compactBlocked, accounts, excludedIDs)
}

func (s *OpenAIGatewayService) listSchedulableAccounts(ctx context.Context, groupID *int64) ([]Account, error) {
	if s.schedulerSnapshot != nil {
		accounts, _, err := s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, PlatformOpenAI, false)
		return accounts, err
	}
	var accounts []Account
	var err error
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformOpenAI)
	} else if groupID != nil {
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformOpenAI)
	} else {
		accounts, err = s.accountRepo.ListSchedulableUngroupedByPlatform(ctx, PlatformOpenAI)
	}
	if err != nil {
		return nil, fmt.Errorf("query accounts failed: %w", err)
	}
	return accounts, nil
}

func (s *OpenAIGatewayService) listGlobalImageAccounts(ctx context.Context) ([]Account, error) {
	if s.schedulerSnapshot != nil {
		accounts, err := s.schedulerSnapshot.ListGlobalImageAccounts(ctx, PlatformOpenAI)
		return accounts, err
	}
	accounts, err := s.accountRepo.ListSchedulableByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return nil, fmt.Errorf("query global image accounts failed: %w", err)
	}
	return accounts, nil
}

func (s *OpenAIGatewayService) tryAcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int) (*AcquireResult, error) {
	if s.concurrencyService == nil {
		return &AcquireResult{Acquired: true, ReleaseFunc: func() {}}, nil
	}
	return s.concurrencyService.AcquireAccountSlot(ctx, accountID, maxConcurrency)
}

func (s *OpenAIGatewayService) resolveFreshSchedulableOpenAIAccount(ctx context.Context, account *Account, requestedModel string, requireCompact ...bool) *Account {
	if account == nil {
		return nil
	}
	compactRequired := len(requireCompact) > 0 && requireCompact[0]

	fresh := account
	if s.schedulerSnapshot != nil {
		current, err := s.getSchedulableAccount(ctx, account.ID)
		if err != nil || current == nil {
			return nil
		}
		fresh = current
	}

	if !isOpenAIAccountEligibleForRequest(fresh, requestedModel, compactRequired) {
		return nil
	}
	return fresh
}

func (s *OpenAIGatewayService) recheckSelectedOpenAIAccountFromDB(ctx context.Context, account *Account, requestedModel string, requireCompact ...bool) *Account {
	if account == nil {
		return nil
	}
	compactRequired := len(requireCompact) > 0 && requireCompact[0]
	if s.schedulerSnapshot == nil || s.accountRepo == nil {
		if !isOpenAIAccountEligibleForRequest(account, requestedModel, compactRequired) {
			return nil
		}
		return account
	}

	latest, err := s.accountRepo.GetByID(ctx, account.ID)
	if err != nil || latest == nil {
		return nil
	}
	if !isOpenAIAccountEligibleForRequest(latest, requestedModel, compactRequired) {
		return nil
	}
	return latest
}

func (s *OpenAIGatewayService) getSchedulableAccount(ctx context.Context, accountID int64) (*Account, error) {
	var (
		account *Account
		err     error
	)
	if s.schedulerSnapshot != nil {
		account, err = s.schedulerSnapshot.GetAccount(ctx, accountID)
	} else {
		account, err = s.accountRepo.GetByID(ctx, accountID)
	}
	if err != nil || account == nil {
		return account, err
	}
	return account, nil
}

func (s *OpenAIGatewayService) hydrateSelectedAccount(ctx context.Context, account *Account) (*Account, error) {
	if account == nil || s.schedulerSnapshot == nil {
		return account, nil
	}
	hydrated, err := s.schedulerSnapshot.GetAccount(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	if hydrated == nil {
		return nil, fmt.Errorf("selected openai account %d not found during hydration", account.ID)
	}
	return hydrated, nil
}

func (s *OpenAIGatewayService) newSelectionResult(ctx context.Context, account *Account, acquired bool, release func(), waitPlan *AccountWaitPlan) (*AccountSelectionResult, error) {
	hydrated, err := s.hydrateSelectedAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	return &AccountSelectionResult{
		Account:     hydrated,
		Acquired:    acquired,
		ReleaseFunc: release,
		WaitPlan:    waitPlan,
	}, nil
}

func (s *OpenAIGatewayService) schedulingConfig() config.GatewaySchedulingConfig {
	if s.cfg != nil {
		return s.cfg.Gateway.Scheduling
	}
	return config.GatewaySchedulingConfig{
		StickySessionMaxWaiting:  3,
		StickySessionWaitTimeout: 45 * time.Second,
		FallbackWaitTimeout:      30 * time.Second,
		FallbackMaxWaiting:       100,
		LoadBatchEnabled:         true,
		SlotCleanupInterval:      30 * time.Second,
	}
}

// GetAccessToken gets the access token for an OpenAI account
func (s *OpenAIGatewayService) GetAccessToken(ctx context.Context, account *Account) (string, string, error) {
	if account != nil && account.Platform == PlatformGrok {
		if account.Type == AccountTypeOAuth {
			if s.grokTokenProvider == nil {
				return "", "", errors.New("grok token provider is not configured")
			}
			token, err := s.grokTokenProvider.GetAccessToken(ctx, account)
			if err != nil {
				return "", "", err
			}
			return token, "oauth", nil
		}
		if account.Type == AccountTypeAPIKey {
			token := account.GetCredential("api_key")
			if strings.TrimSpace(token) == "" {
				return "", "", errors.New("api_key not found in credentials")
			}
			return token, "apikey", nil
		}
	}
	switch account.Type {
	case AccountTypeOAuth:
		// 使用 TokenProvider 获取缓存的 token
		if s.openAITokenProvider != nil {
			accessToken, err := s.openAITokenProvider.GetAccessToken(ctx, account)
			if err != nil {
				return "", "", err
			}
			return accessToken, "oauth", nil
		}
		// 降级：TokenProvider 未配置时直接从账号读取
		accessToken := account.GetOpenAIAccessToken()
		if accessToken == "" {
			return "", "", errors.New("access_token not found in credentials")
		}
		return accessToken, "oauth", nil
	case AccountTypeAPIKey:
		apiKey := account.GetOpenAIApiKey()
		if apiKey == "" {
			return "", "", errors.New("api_key not found in credentials")
		}
		return apiKey, "apikey", nil
	default:
		return "", "", fmt.Errorf("unsupported account type: %s", account.Type)
	}
}

func (s *OpenAIGatewayService) shouldFailoverUpstreamError(statusCode int) bool {
	switch statusCode {
	case 401, 402, 403, 405, 429, 529:
		return true
	default:
		return statusCode >= 500
	}
}

func (s *OpenAIGatewayService) shouldFailoverOpenAIUpstreamResponse(statusCode int, upstreamMsg string, upstreamBody []byte) bool {
	if IsOpenAIContextWindowExceeded(upstreamMsg, string(upstreamBody)) {
		return false
	}
	// Some OpenAI-compatible providers expose one public model alias while
	// dispatching requests across backend pools with different tool protocols,
	// account permissions, or reasoning-effort capabilities. These responses
	// are HTTP 400, but they are account/backend capability rejections rather
	// than malformed customer requests. Treat only the exact known signatures
	// as failover-safe; generic 400s must remain terminal.
	if isOpenAIAccountCapabilityRejection(statusCode, upstreamMsg, upstreamBody) {
		return true
	}
	if s.shouldFailoverUpstreamError(statusCode) {
		return true
	}
	return isOpenAITransientProcessingError(statusCode, upstreamMsg, upstreamBody)
}

func isOpenAIAccountCapabilityRejection(statusCode int, upstreamMsg string, upstreamBody []byte) bool {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusUnprocessableEntity {
		return false
	}

	combined := strings.ToLower(strings.Join([]string{upstreamMsg, string(upstreamBody)}, " "))
	if strings.TrimSpace(combined) == "" {
		return false
	}

	if strings.Contains(combined, "client-executed tool_search is not supported by the kiro upstream") {
		return true
	}
	if strings.Contains(combined, "external_web_access=false is not supported by the configured kiro mcp backend") {
		return true
	}
	if strings.Contains(combined, "gpt-daybreak-blue-latest") &&
		strings.Contains(combined, "not supported when using codex with a chatgpt account") {
		return true
	}
	if strings.Contains(combined, "expected an id that begins with 'tsc'") ||
		strings.Contains(combined, `expected an id that begins with "tsc"`) {
		return true
	}
	if strings.Contains(combined, "unsupported value: 'max'") &&
		strings.Contains(combined, "not supported with the 'gpt-5.4' model") {
		return true
	}

	return false
}

func (s *OpenAIGatewayService) handleFailoverSideEffects(ctx context.Context, resp *http.Response, account *Account) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
}

// Forward forwards request to OpenAI API
func (s *OpenAIGatewayService) Forward(ctx context.Context, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
	if account != nil && account.Platform == PlatformGrok {
		model, stream, _ := extractOpenAIRequestMetaFromBody(body)
		return s.forwardGrokResponses(ctx, c, account, body, model, stream, time.Now())
	}
	if account != nil && account.IsOpenAI() && c != nil && isOpenAIResponsesLiteHeader(c.GetHeader(responsesLiteHeader)) {
		normalized, _, err := normalizeOpenAIResponsesLitePayloadForAccount(body, account)
		if err != nil {
			param := "tools"
			var validationErr *openAIResponsesLiteValidationError
			if errors.As(err, &validationErr) && validationErr != nil && validationErr.param != "" {
				param = validationErr.param
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": err.Error(), "param": param}})
			return nil, err
		}
		body = normalized
		// The handler cache was decoded before Lite normalization and may contain
		// float64 values. Force a UseNumber decode from the rewritten wire body.
		c.Set(OpenAIParsedRequestBodyKey, nil)
	}
	if s == nil || s.pipeline == nil || s.cfg == nil || !s.cfg.Gateway.Pipeline.OpenAIResponsesEnabled {
		result, err := s.forwardLegacy(ctx, c, account, body)
		return finalizeOpenAIResponseImageBilling(result), err
	}
	_, stream, _ := extractOpenAIRequestMetaFromBody(body)
	endpoint := "/v1/responses"
	if c != nil && c.Request != nil && c.Request.URL != nil {
		endpoint = c.Request.URL.Path
	}
	request := GatewayPipelineRequest{Platform: PlatformOpenAI, Endpoint: endpoint, AccountID: account.ID, Stream: stream}
	result, err := s.pipeline.Execute(ctx, request, GatewayPipelineTransportFunc(func(ctx context.Context, _ GatewayPipelineRequest, selection GatewayPipelineSelection) (GatewayPipelineResult, error) {
		if selection.AccountID != account.ID {
			return GatewayPipelineResult{}, fmt.Errorf("pipeline selection changed preselected account")
		}
		legacyResult, err := s.forwardLegacy(ctx, c, account, body)
		return GatewayPipelineResult{Value: legacyResult}, err
	}))
	if err != nil {
		return nil, err
	}
	if result.Value == nil {
		return nil, nil
	}
	forwardResult, ok := result.Value.(*OpenAIForwardResult)
	if !ok {
		return nil, errors.New("gateway pipeline returned unexpected result type")
	}
	return finalizeOpenAIResponseImageBilling(forwardResult), nil
}

func (s *OpenAIGatewayService) forwardLegacy(ctx context.Context, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	restrictionResult := s.detectCodexClientRestriction(c, account)
	apiKeyID := getAPIKeyIDFromContext(c)
	logCodexCLIOnlyDetection(ctx, c, account, apiKeyID, restrictionResult, body)
	if restrictionResult.Enabled && !restrictionResult.Matched {
		c.JSON(http.StatusForbidden, OpenAIClientErrorEnvelope(c, "forbidden_error", "This account only allows Codex official clients"))
		return nil, errors.New("codex_cli_only restriction: only codex official clients are allowed")
	}

	originalBody := body
	reqModel, reqStream, promptCacheKey := extractOpenAIRequestMetaFromBody(body)
	originalModel := reqModel

	isCodexCLI := openai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator")) || (s.cfg != nil && s.cfg.Gateway.ForceCodexCLI)
	wsDecision := s.getOpenAIWSProtocolResolver().Resolve(account)
	clientTransport := GetOpenAIClientTransport(c)
	// 仅允许 WS 入站请求走 WS 上游，避免出现 HTTP -> WS 协议混用。
	wsDecision = resolveOpenAIWSDecisionByClientTransport(wsDecision, clientTransport)
	if c != nil {
		c.Set("openai_ws_transport_decision", string(wsDecision.Transport))
		c.Set("openai_ws_transport_reason", wsDecision.Reason)
	}
	if wsDecision.Transport == OpenAIUpstreamTransportResponsesWebsocketV2 {
		logOpenAIWSModeDebug(
			"selected account_id=%d account_type=%s transport=%s reason=%s model=%s stream=%v",
			account.ID,
			account.Type,
			normalizeOpenAIWSLogValue(string(wsDecision.Transport)),
			normalizeOpenAIWSLogValue(wsDecision.Reason),
			reqModel,
			reqStream,
		)
	}
	// 当前仅支持 WSv2；WSv1 命中时直接返回错误，避免出现“配置可开但行为不确定”。
	if wsDecision.Transport == OpenAIUpstreamTransportResponsesWebsocket {
		if c != nil {
			c.JSON(http.StatusBadRequest, OpenAIClientErrorEnvelope(c, "invalid_request_error", ClientMessageRequestFailed))
		}
		return nil, errors.New("openai ws v1 is temporarily unsupported; use ws v2")
	}
	passthroughEnabled := account.IsOpenAIPassthroughEnabled()
	if passthroughEnabled {
		// 透传分支只需要轻量提取字段，避免热路径全量 Unmarshal。
		reasoningEffort := extractOpenAIReasoningEffortFromBody(body, reqModel)
		return s.forwardOpenAIPassthrough(ctx, c, account, originalBody, reqModel, reasoningEffort, reqStream, startTime)
	}

	reqBody, err := getOpenAIRequestBodyMap(c, body)
	if err != nil {
		return nil, err
	}

	if v, ok := reqBody["model"].(string); ok {
		reqModel = v
		originalModel = reqModel
	}
	if v, ok := reqBody["stream"].(bool); ok {
		reqStream = v
	}
	if promptCacheKey == "" {
		if v, ok := reqBody["prompt_cache_key"].(string); ok {
			promptCacheKey = strings.TrimSpace(v)
		}
	}

	// Track if body needs re-serialization
	bodyModified := false
	// 单字段补丁快速路径：只要整个变更集最终可归约为同一路径的 set/delete，就避免全量 Marshal。
	patchDisabled := false
	patchHasOp := false
	patchDelete := false
	patchPath := ""
	var patchValue any
	markPatchSet := func(path string, value any) {
		if strings.TrimSpace(path) == "" {
			patchDisabled = true
			return
		}
		if patchDisabled {
			return
		}
		if !patchHasOp {
			patchHasOp = true
			patchDelete = false
			patchPath = path
			patchValue = value
			return
		}
		if patchDelete || patchPath != path {
			patchDisabled = true
			return
		}
		patchValue = value
	}
	markPatchDelete := func(path string) {
		if strings.TrimSpace(path) == "" {
			patchDisabled = true
			return
		}
		if patchDisabled {
			return
		}
		if !patchHasOp {
			patchHasOp = true
			patchDelete = true
			patchPath = path
			return
		}
		if !patchDelete || patchPath != path {
			patchDisabled = true
		}
	}
	disablePatch := func() {
		patchDisabled = true
	}

	// 非透传模式下，instructions 为空时注入默认指令。
	if isInstructionsEmpty(reqBody) {
		reqBody["instructions"] = "You are a helpful coding assistant."
		bodyModified = true
		markPatchSet("instructions", "You are a helpful coding assistant.")
	}

	// 对所有请求执行模型映射（包含 Codex CLI）。
	billingModel := account.GetMappedModel(reqModel)
	if billingModel != reqModel {
		logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Model mapping applied: %s -> %s (account: %s, isCodexCLI: %v)", reqModel, billingModel, account.Name, isCodexCLI)
		reqBody["model"] = billingModel
		bodyModified = true
		markPatchSet("model", billingModel)
	}
	upstreamModel := billingModel
	isCompactRequest := isOpenAIResponsesCompactPath(c)
	compactMapped := false
	if isCompactRequest {
		compactMappedModel := resolveOpenAICompactForwardModel(account, billingModel)
		if compactMappedModel != "" && compactMappedModel != billingModel {
			compactMapped = true
			upstreamModel = compactMappedModel
			reqBody["model"] = compactMappedModel
			bodyModified = true
			markPatchSet("model", compactMappedModel)
			logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Compact model mapping applied: %s -> %s (account: %s, isCodexCLI: %v)", billingModel, compactMappedModel, account.Name, isCodexCLI)
		}
	}

	// OpenAI OAuth 账号走 ChatGPT internal Codex endpoint，需要将模型名规范化为
	// 上游可识别的 Codex/GPT 系列。API Key 账号则应保留原始/映射后的模型名，
	// 以兼容自定义 base_url 的 OpenAI-compatible 上游。
	if model, ok := reqBody["model"].(string); ok {
		if !compactMapped {
			upstreamModel = normalizeOpenAIModelForUpstream(account, model)
			if upstreamModel != "" && upstreamModel != model {
				logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Upstream model resolved: %s -> %s (account: %s, type: %s, isCodexCLI: %v)",
					model, upstreamModel, account.Name, account.Type, isCodexCLI)
				reqBody["model"] = upstreamModel
				bodyModified = true
				markPatchSet("model", upstreamModel)
			}
		}

		// 移除 gpt-5.2-codex 以下的版本 verbosity 参数
		// 确保高版本模型向低版本模型映射不报错
		if !SupportsVerbosity(upstreamModel) {
			if text, ok := reqBody["text"].(map[string]any); ok {
				delete(text, "verbosity")
			}
		}
	}
	useChatCompletionsBridge := account.Type == AccountTypeAPIKey &&
		!openai_compat.ShouldUseResponsesAPIForModel(account.Extra, originalModel, billingModel, upstreamModel)
	if useChatCompletionsBridge && wsDecision.Transport != OpenAIUpstreamTransportHTTPSSE {
		return nil, fmt.Errorf("chat_completions protocol bridge only supports HTTP Responses clients")
	}
	if useChatCompletionsBridge {
		if previousResponseID, _ := reqBody["previous_response_id"].(string); strings.TrimSpace(previousResponseID) != "" {
			return nil, fmt.Errorf("previous_response_id requires a native Responses upstream")
		}
	}

	// 规范化 reasoning.effort 参数（minimal -> none），与上游允许值对齐。
	if reasoning, ok := reqBody["reasoning"].(map[string]any); ok {
		if effort, ok := reasoning["effort"].(string); ok && effort == "minimal" {
			reasoning["effort"] = "none"
			bodyModified = true
			markPatchSet("reasoning.effort", "none")
			logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Normalized reasoning.effort: minimal -> none (account: %s)", account.Name)
		}
	}

	if account.Type == AccountTypeOAuth {
		codexResult := applyCodexOAuthTransform(reqBody, isCodexCLI, isCompactRequest)
		if codexResult.Modified {
			bodyModified = true
			disablePatch()
		}
		if codexResult.NormalizedModel != "" {
			upstreamModel = codexResult.NormalizedModel
		}
		if codexResult.PromptCacheKey != "" {
			promptCacheKey = codexResult.PromptCacheKey
		}
	}

	// Handle max_output_tokens based on platform and account type
	if !isCodexCLI {
		if maxOutputTokens, hasMaxOutputTokens := reqBody["max_output_tokens"]; hasMaxOutputTokens {
			switch account.Platform {
			case PlatformOpenAI:
				// For OpenAI API Key, remove max_output_tokens (not supported)
				// For OpenAI OAuth (Responses API), keep it (supported)
				if account.Type == AccountTypeAPIKey && !useChatCompletionsBridge {
					delete(reqBody, "max_output_tokens")
					bodyModified = true
					markPatchDelete("max_output_tokens")
				}
			case PlatformAnthropic:
				// For Anthropic (Claude), convert to max_tokens
				delete(reqBody, "max_output_tokens")
				markPatchDelete("max_output_tokens")
				if _, hasMaxTokens := reqBody["max_tokens"]; !hasMaxTokens {
					reqBody["max_tokens"] = maxOutputTokens
					disablePatch()
				}
				bodyModified = true
			case PlatformGemini:
				// For Gemini, remove (will be handled by Gemini-specific transform)
				delete(reqBody, "max_output_tokens")
				bodyModified = true
				markPatchDelete("max_output_tokens")
			default:
				// For unknown platforms, remove to be safe
				delete(reqBody, "max_output_tokens")
				bodyModified = true
				markPatchDelete("max_output_tokens")
			}
		}

		// Also handle max_completion_tokens (similar logic)
		if _, hasMaxCompletionTokens := reqBody["max_completion_tokens"]; hasMaxCompletionTokens {
			if account.Type == AccountTypeAPIKey || account.Platform != PlatformOpenAI {
				delete(reqBody, "max_completion_tokens")
				bodyModified = true
				markPatchDelete("max_completion_tokens")
			}
		}

		// Remove unsupported fields (not supported by upstream OpenAI API)
		unsupportedFields := []string{"prompt_cache_retention", "safety_identifier"}
		for _, unsupportedField := range unsupportedFields {
			if _, has := reqBody[unsupportedField]; has {
				delete(reqBody, unsupportedField)
				bodyModified = true
				markPatchDelete(unsupportedField)
			}
		}
	}

	// 仅在 WSv2 模式保留 previous_response_id，其他模式（HTTP/WSv1）统一过滤。
	// 注意：该规则同样适用于 Codex CLI 请求，避免 WSv1 向上游透传不支持字段。
	if wsDecision.Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
		if _, has := reqBody["previous_response_id"]; has {
			delete(reqBody, "previous_response_id")
			bodyModified = true
			markPatchDelete("previous_response_id")
		}
	}

	if sanitizeEmptyBase64InputImagesInOpenAIRequestBodyMap(reqBody) {
		bodyModified = true
		disablePatch()
	}

	if rawTier, ok := reqBody["service_tier"].(string); ok {
		if normTier := normalizedOpenAIServiceTierValue(rawTier); normTier != "" {
			action, errMsg := s.evaluateOpenAIFastPolicy(ctx, account, upstreamModel, normTier)
			switch action {
			case BetaPolicyActionBlock:
				msg := errMsg
				if msg == "" {
					msg = fmt.Sprintf("openai service_tier=%s is not allowed for model %s", normTier, upstreamModel)
				}
				blocked := &OpenAIFastBlockedError{Message: msg}
				writeOpenAIFastPolicyBlockedResponse(c, blocked)
				return nil, blocked
			case BetaPolicyActionFilter:
				delete(reqBody, "service_tier")
				bodyModified = true
				markPatchDelete("service_tier")
			default:
				if normTier != rawTier {
					reqBody["service_tier"] = normTier
					bodyModified = true
					markPatchSet("service_tier", normTier)
				}
			}
		}
	}

	// Re-serialize body only if modified
	if bodyModified {
		serializedByPatch := false
		if !patchDisabled && patchHasOp {
			var patchErr error
			if patchDelete {
				body, patchErr = sjson.DeleteBytes(body, patchPath)
			} else {
				body, patchErr = sjson.SetBytes(body, patchPath, patchValue)
			}
			if patchErr == nil {
				serializedByPatch = true
			}
		}
		if !serializedByPatch {
			var marshalErr error
			body, marshalErr = json.Marshal(reqBody)
			if marshalErr != nil {
				return nil, fmt.Errorf("serialize request body: %w", marshalErr)
			}
		}
	}

	if useChatCompletionsBridge {
		resp, bridgeErr := s.doOpenAIResponsesViaChatCompletions(ctx, c, account, body, originalModel)
		if bridgeErr != nil {
			if c != nil && c.Writer != nil && !c.Writer.Written() {
				var failoverErr *UpstreamFailoverError
				if !errors.As(bridgeErr, &failoverErr) {
					c.JSON(http.StatusBadRequest, OpenAIClientErrorEnvelope(c, "invalid_request_error", bridgeErr.Error()))
				}
			}
			return nil, bridgeErr
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= http.StatusBadRequest {
			respBody, upstreamMsg := s.readOpenAIUpstreamError(resp)
			if failoverErr := s.failoverOpenAIUpstreamHTTPError(ctx, c, account, resp, respBody, upstreamMsg, upstreamModel); failoverErr != nil {
				return nil, failoverErr
			}
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
			return s.handleErrorResponse(ctx, resp, c, account, body)
		}

		if reqStream {
			streamResult, streamErr := s.handleStreamingResponse(ctx, resp, c, account, startTime, originalModel, upstreamModel)
			if streamErr != nil {
				return nil, streamErr
			}
			usage := OpenAIUsage{}
			if streamResult.usage != nil {
				usage = *streamResult.usage
			}
			return &OpenAIForwardResult{
				RequestID: resp.Header.Get("x-request-id"), Usage: usage, Model: originalModel,
				BillingModel: billingModel, UpstreamModel: upstreamModel, UpstreamEndpoint: openAIResponsesViaChatEndpoint,
				ServiceTier: extractOpenAIServiceTier(reqBody), ReasoningEffort: extractOpenAIReasoningEffort(reqBody, originalModel),
				Stream: true, Duration: time.Since(startTime), FirstTokenMs: streamResult.firstTokenMs,
			}, nil
		}
		result, bufferedErr := s.handleResponsesBufferedStreamingResponse(resp, c, account, originalModel, billingModel, upstreamModel, startTime)
		if result != nil {
			result.ServiceTier = extractOpenAIServiceTier(reqBody)
			result.ReasoningEffort = extractOpenAIReasoningEffort(reqBody, originalModel)
		}
		return result, bufferedErr
	}

	// Get access token
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	// Capture upstream request body for ops retry of this attempt.
	setOpsUpstreamRequestBody(c, body)

	// 命中 WS 时仅走 WebSocket Mode；不再自动回退 HTTP。
	if wsDecision.Transport == OpenAIUpstreamTransportResponsesWebsocketV2 {
		wsReqBody := reqBody
		if len(reqBody) > 0 {
			wsReqBody = make(map[string]any, len(reqBody))
			for k, v := range reqBody {
				wsReqBody[k] = v
			}
		}
		_, hasPreviousResponseID := wsReqBody["previous_response_id"]
		logOpenAIWSModeDebug(
			"forward_start account_id=%d account_type=%s model=%s stream=%v has_previous_response_id=%v",
			account.ID,
			account.Type,
			upstreamModel,
			reqStream,
			hasPreviousResponseID,
		)
		maxAttempts := openAIWSReconnectRetryLimit + 1
		wsAttempts := 0
		var wsResult *OpenAIForwardResult
		var wsErr error
		wsLastFailureReason := ""
		wsPrevResponseRecoveryTried := false
		wsInvalidEncryptedContentRecoveryTried := false
		recoverPrevResponseNotFound := func(attempt int) bool {
			if wsPrevResponseRecoveryTried {
				return false
			}
			previousResponseID := openAIWSPayloadString(wsReqBody, "previous_response_id")
			if previousResponseID == "" {
				logOpenAIWSModeInfo(
					"reconnect_prev_response_recovery_skip account_id=%d attempt=%d reason=missing_previous_response_id previous_response_id_present=false",
					account.ID,
					attempt,
				)
				return false
			}
			if HasFunctionCallOutput(wsReqBody) {
				logOpenAIWSModeInfo(
					"reconnect_prev_response_recovery_skip account_id=%d attempt=%d reason=has_function_call_output previous_response_id_present=true",
					account.ID,
					attempt,
				)
				return false
			}
			delete(wsReqBody, "previous_response_id")
			wsPrevResponseRecoveryTried = true
			logOpenAIWSModeInfo(
				"reconnect_prev_response_recovery account_id=%d attempt=%d action=drop_previous_response_id retry=1 previous_response_id=%s previous_response_id_kind=%s",
				account.ID,
				attempt,
				truncateOpenAIWSLogValue(previousResponseID, openAIWSIDValueMaxLen),
				normalizeOpenAIWSLogValue(ClassifyOpenAIPreviousResponseIDKind(previousResponseID)),
			)
			return true
		}
		recoverInvalidEncryptedContent := func(attempt int) bool {
			if wsInvalidEncryptedContentRecoveryTried {
				return false
			}
			removedReasoningItems := trimOpenAIEncryptedReasoningItems(wsReqBody)
			if !removedReasoningItems {
				logOpenAIWSModeInfo(
					"reconnect_invalid_encrypted_content_recovery_skip account_id=%d attempt=%d reason=missing_encrypted_reasoning_items",
					account.ID,
					attempt,
				)
				return false
			}
			previousResponseID := openAIWSPayloadString(wsReqBody, "previous_response_id")
			hasFunctionCallOutput := HasFunctionCallOutput(wsReqBody)
			if previousResponseID != "" && !hasFunctionCallOutput {
				delete(wsReqBody, "previous_response_id")
			}
			wsInvalidEncryptedContentRecoveryTried = true
			logOpenAIWSModeInfo(
				"reconnect_invalid_encrypted_content_recovery account_id=%d attempt=%d action=drop_encrypted_reasoning_items retry=1 previous_response_id_present=%v previous_response_id=%s previous_response_id_kind=%s has_function_call_output=%v dropped_previous_response_id=%v",
				account.ID,
				attempt,
				previousResponseID != "",
				truncateOpenAIWSLogValue(previousResponseID, openAIWSIDValueMaxLen),
				normalizeOpenAIWSLogValue(ClassifyOpenAIPreviousResponseIDKind(previousResponseID)),
				hasFunctionCallOutput,
				previousResponseID != "" && !hasFunctionCallOutput,
			)
			return true
		}
		retryBudget := s.openAIWSRetryTotalBudget()
		retryStartedAt := time.Now()
	wsRetryLoop:
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			wsAttempts = attempt
			wsResult, wsErr = s.forwardOpenAIWSV2(
				ctx,
				c,
				account,
				wsReqBody,
				token,
				wsDecision,
				isCodexCLI,
				reqStream,
				originalModel,
				upstreamModel,
				startTime,
				attempt,
				wsLastFailureReason,
			)
			if wsErr == nil {
				break
			}
			if c != nil && c.Writer != nil && c.Writer.Written() {
				break
			}

			reason, retryable := classifyOpenAIWSReconnectReason(wsErr)
			if reason != "" {
				wsLastFailureReason = reason
			}
			// previous_response_not_found 说明续链锚点不可用：
			// 对非 function_call_output 场景，允许一次“去掉 previous_response_id 后重放”。
			if reason == "previous_response_not_found" && recoverPrevResponseNotFound(attempt) {
				continue
			}
			if reason == "invalid_encrypted_content" && recoverInvalidEncryptedContent(attempt) {
				continue
			}
			if retryable && attempt < maxAttempts {
				backoff := s.openAIWSRetryBackoff(attempt)
				if retryBudget > 0 && time.Since(retryStartedAt)+backoff > retryBudget {
					s.recordOpenAIWSRetryExhausted()
					logOpenAIWSModeInfo(
						"reconnect_budget_exhausted account_id=%d attempts=%d max_retries=%d reason=%s elapsed_ms=%d budget_ms=%d",
						account.ID,
						attempt,
						openAIWSReconnectRetryLimit,
						normalizeOpenAIWSLogValue(reason),
						time.Since(retryStartedAt).Milliseconds(),
						retryBudget.Milliseconds(),
					)
					break
				}
				s.recordOpenAIWSRetryAttempt(backoff)
				logOpenAIWSModeInfo(
					"reconnect_retry account_id=%d retry=%d max_retries=%d reason=%s backoff_ms=%d",
					account.ID,
					attempt,
					openAIWSReconnectRetryLimit,
					normalizeOpenAIWSLogValue(reason),
					backoff.Milliseconds(),
				)
				if backoff > 0 {
					timer := time.NewTimer(backoff)
					select {
					case <-ctx.Done():
						if !timer.Stop() {
							<-timer.C
						}
						wsErr = wrapOpenAIWSFallback("retry_backoff_canceled", ctx.Err())
						break wsRetryLoop
					case <-timer.C:
					}
				}
				continue
			}
			if retryable {
				s.recordOpenAIWSRetryExhausted()
				logOpenAIWSModeInfo(
					"reconnect_exhausted account_id=%d attempts=%d max_retries=%d reason=%s",
					account.ID,
					attempt,
					openAIWSReconnectRetryLimit,
					normalizeOpenAIWSLogValue(reason),
				)
			} else if reason != "" {
				s.recordOpenAIWSNonRetryableFastFallback()
				logOpenAIWSModeInfo(
					"reconnect_stop account_id=%d attempt=%d reason=%s",
					account.ID,
					attempt,
					normalizeOpenAIWSLogValue(reason),
				)
			}
			break
		}
		if wsErr == nil {
			firstTokenMs := int64(0)
			hasFirstTokenMs := wsResult != nil && wsResult.FirstTokenMs != nil
			if hasFirstTokenMs {
				firstTokenMs = int64(*wsResult.FirstTokenMs)
			}
			requestID := ""
			if wsResult != nil {
				requestID = strings.TrimSpace(wsResult.RequestID)
			}
			logOpenAIWSModeDebug(
				"forward_succeeded account_id=%d request_id=%s stream=%v has_first_token_ms=%v first_token_ms=%d ws_attempts=%d",
				account.ID,
				requestID,
				reqStream,
				hasFirstTokenMs,
				firstTokenMs,
				wsAttempts,
			)
			wsResult.UpstreamModel = upstreamModel
			return wsResult, nil
		}
		s.writeOpenAIWSFallbackErrorResponse(c, account, wsErr)
		return nil, wsErr
	}

	httpInvalidEncryptedContentRetryTried := false
	rejectedStatusRetryState := openAIResponsesRejectedStatusRetryStateForRequest(c, body)
	for {
		// Build upstream request
		upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, reqStream)
		upstreamReq, err := s.buildUpstreamRequest(upstreamCtx, c, account, body, token, reqStream, promptCacheKey, isCodexCLI)
		releaseUpstreamCtx()
		if err != nil {
			return nil, err
		}

		// Get proxy URL
		proxyURL := ""
		if account.ProxyID != nil && account.Proxy != nil {
			proxyURL = account.Proxy.URL()
		}

		// Send request
		upstreamStart := time.Now()
		resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		if err != nil {
			// Ensure the client receives an error response (handlers assume Forward writes on non-failover errors).
			safeErr := sanitizeUpstreamErrorMessage(err.Error())
			shouldFailover := shouldFailoverOpenAITransportError(ctx, err)
			setOpsUpstreamError(c, 0, safeErr, "")
			kind := "request_error"
			if shouldFailover {
				kind = "failover"
			}
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: 0,
				Kind:               kind,
				Message:            safeErr,
			})
			// A live client request can still be recovered by the handler's existing
			// account failover loop. Do not commit a 502 before that loop has a chance
			// to select the configured fallback account. If the client context itself
			// is already done, failover would only create duplicate upstream work.
			if shouldFailover {
				return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway}
			}
			safeClientErr := SafeClientUpstreamError(http.StatusBadGateway)
			c.JSON(safeClientErr.StatusCode, OpenAIClientErrorEnvelope(c, safeClientErr.Type, safeClientErr.Message))
			return nil, fmt.Errorf("upstream request failed: %s", safeErr)
		}

		// Handle error response
		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			_ = resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(respBody))

			upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
			upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
			upstreamCode := extractUpstreamErrorCode(respBody)
			retryBody, changed, retryErr := normalizeOpenAIResponsesRejectedStatusRetryBody(resp.StatusCode, body, respBody)
			if retryErr != nil {
				return nil, retryErr
			}
			if changed && rejectedStatusRetryState.Allow(retryBody) {
				body = retryBody
				if decodeErr := decodeOpenAIJSONUseNumber(body, &reqBody); decodeErr != nil {
					return nil, fmt.Errorf("decode rejected-status retry body: %w", decodeErr)
				}
				setOpsUpstreamRequestBody(c, body)
				continue
			}
			if !httpInvalidEncryptedContentRetryTried && resp.StatusCode == http.StatusBadRequest && upstreamCode == "invalid_encrypted_content" {
				if trimOpenAIEncryptedReasoningItems(reqBody) {
					body, err = json.Marshal(reqBody)
					if err != nil {
						return nil, fmt.Errorf("serialize invalid_encrypted_content retry body: %w", err)
					}
					setOpsUpstreamRequestBody(c, body)
					httpInvalidEncryptedContentRetryTried = true
					logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Retrying non-WSv2 request once after invalid_encrypted_content (account: %s)", account.Name)
					continue
				}
				logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Skip non-WSv2 invalid_encrypted_content retry because encrypted reasoning items are missing (account: %s)", account.Name)
			}
			if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
				upstreamDetail := ""
				if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
					maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
					if maxBytes <= 0 {
						maxBytes = 2048
					}
					upstreamDetail = truncateString(string(respBody), maxBytes)
				}
				appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
					Platform:           account.Platform,
					AccountID:          account.ID,
					AccountName:        account.Name,
					UpstreamStatusCode: resp.StatusCode,
					UpstreamRequestID:  resp.Header.Get("x-request-id"),
					Kind:               "failover",
					Message:            upstreamMsg,
					Detail:             upstreamDetail,
				})

				s.handleFailoverSideEffects(ctx, resp, account)
				return nil, &UpstreamFailoverError{
					StatusCode:             resp.StatusCode,
					ResponseBody:           respBody,
					RetryableOnSameAccount: allowOpenAISameAccountRetry(isTransientOpenAIOAuth429(account, resp.StatusCode, resp.Header, respBody) || (account.IsPoolMode() && (isPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody))), respBody),
				}
			}
			return s.handleErrorResponse(ctx, resp, c, account, body)
		}
		defer func() { _ = resp.Body.Close() }()

		// Handle normal response
		var usage *OpenAIUsage
		var firstTokenMs *int
		imageCount := 0
		if reqStream {
			streamResult, err := s.handleStreamingResponse(ctx, resp, c, account, startTime, originalModel, upstreamModel)
			if err != nil {
				return nil, err
			}
			usage = streamResult.usage
			firstTokenMs = streamResult.firstTokenMs
			imageCount = streamResult.imageCount
		} else {
			nonStreamResult, nonStreamErr := s.handleNonStreamingResponse(ctx, resp, c, account, originalModel, upstreamModel)
			err = nonStreamErr
			if err != nil {
				return nil, err
			}
			usage = nonStreamResult.usage
			imageCount = nonStreamResult.imageCount
		}

		// Extract and save Codex usage snapshot from response headers (for OAuth accounts)
		if account.Type == AccountTypeOAuth {
			if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
				s.updateCodexUsageSnapshot(ctx, account.ID, snapshot)
			}
		}

		if usage == nil {
			usage = &OpenAIUsage{}
		}

		reasoningEffort := extractOpenAIReasoningEffort(reqBody, originalModel)
		serviceTier := extractOpenAIServiceTier(reqBody)

		return &OpenAIForwardResult{
			RequestID:       resp.Header.Get("x-request-id"),
			Usage:           *usage,
			Model:           originalModel,
			UpstreamModel:   upstreamModel,
			ServiceTier:     serviceTier,
			ReasoningEffort: reasoningEffort,
			Stream:          reqStream,
			OpenAIWSMode:    false,
			Duration:        time.Since(startTime),
			FirstTokenMs:    firstTokenMs,
			ImageCount:      imageCount,
		}, nil
	}
}

func (s *OpenAIGatewayService) forwardOpenAIPassthrough(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	reqModel string,
	reasoningEffort *string,
	reqStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	if account != nil && account.Type == AccountTypeOAuth {
		if rejectReason := detectOpenAIPassthroughInstructionsRejectReason(reqModel, body); rejectReason != "" {
			rejectMsg := "OpenAI codex passthrough requires a non-empty instructions field"
			setOpsUpstreamError(c, http.StatusForbidden, rejectMsg, "")
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: http.StatusForbidden,
				Passthrough:        true,
				Kind:               "request_error",
				Message:            rejectMsg,
				Detail:             rejectReason,
			})
			logOpenAIPassthroughInstructionsRejected(ctx, c, account, reqModel, rejectReason, body)
			c.JSON(http.StatusForbidden, OpenAIClientErrorEnvelope(c, "forbidden_error", ClientMessageRequestFailed))
			return nil, fmt.Errorf("openai passthrough rejected before upstream: %s", rejectReason)
		}

		normalizedBody, normalized, err := normalizeOpenAIPassthroughOAuthBody(body, isOpenAIResponsesCompactPath(c))
		if err != nil {
			return nil, err
		}
		if normalized {
			body = normalizedBody
		}
		reqStream = gjson.GetBytes(body, "stream").Bool()
	}
	upstreamPassthroughModel := ""
	if isOpenAIResponsesCompactPath(c) {
		compactMappedModel := resolveOpenAICompactForwardModel(account, reqModel)
		if compactMappedModel != "" && compactMappedModel != reqModel {
			updatedBody, err := sjson.SetBytes(body, "model", compactMappedModel)
			if err != nil {
				return nil, fmt.Errorf("set compact passthrough model: %w", err)
			}
			body = updatedBody
			upstreamPassthroughModel = compactMappedModel
			logger.LegacyPrintf("service.openai_gateway", "[OpenAI passthrough] Compact model mapping applied: %s -> %s (account: %s)", reqModel, compactMappedModel, account.Name)
		}
	}

	sanitizedBody, sanitized, err := sanitizeEmptyBase64InputImagesInOpenAIBody(body)
	if err != nil {
		return nil, err
	}
	if sanitized {
		body = sanitizedBody
	}

	policyModel := upstreamPassthroughModel
	if policyModel == "" {
		policyModel = normalizeOpenAIModelForUpstream(account, account.GetMappedModel(strings.TrimSpace(gjson.GetBytes(body, "model").String())))
	}
	updatedBody, policyErr := s.applyOpenAIFastPolicyToBody(ctx, account, policyModel, body)
	if policyErr != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(policyErr, &blocked) {
			writeOpenAIFastPolicyBlockedResponse(c, blocked)
		}
		return nil, policyErr
	}
	body = updatedBody

	logger.LegacyPrintf("service.openai_gateway",
		"[OpenAI 自动透传] 命中自动透传分支: account=%d name=%s type=%s model=%s stream=%v",
		account.ID,
		account.Name,
		account.Type,
		reqModel,
		reqStream,
	)
	if reqStream && c != nil && c.Request != nil {
		if timeoutHeaders := collectOpenAIPassthroughTimeoutHeaders(c.Request.Header); len(timeoutHeaders) > 0 {
			streamWarnLogger := logger.FromContext(ctx).With(
				zap.String("component", "service.openai_gateway"),
				zap.Int64("account_id", account.ID),
				zap.Strings("timeout_headers", timeoutHeaders),
			)
			if s.isOpenAIPassthroughTimeoutHeadersAllowed() {
				streamWarnLogger.Warn("OpenAI passthrough 透传请求包含超时相关请求头，且当前配置为放行，可能导致上游提前断流")
			} else {
				streamWarnLogger.Warn("OpenAI passthrough 检测到超时相关请求头，将按配置过滤以降低断流风险")
			}
		}
	}

	// Get access token
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	setOpsUpstreamRequestBody(c, body)
	if c != nil {
		c.Set("openai_passthrough", true)
	}

	upstreamStart := time.Now()
	resp, retriedBody, err := s.doOpenAIPassthroughRequestWithStatusRetry(ctx, c, account, body, token, proxyURL, reqStream)
	body = retriedBody
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err == nil && resp != nil && c != nil {
		c.Set(openAIRawUpstreamHTTPStatusKey, resp.StatusCode)
	}
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		shouldFailover := shouldFailoverOpenAITransportError(ctx, err)
		setOpsUpstreamError(c, 0, safeErr, "")
		kind := "request_error"
		if shouldFailover {
			kind = "failover"
		}
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Passthrough:        true,
			Kind:               kind,
			Message:            safeErr,
		})
		if shouldFailover {
			return nil, &UpstreamFailoverError{StatusCode: http.StatusBadGateway}
		}
		safeClientErr := SafeClientUpstreamError(http.StatusBadGateway)
		c.JSON(safeClientErr.StatusCode, OpenAIClientErrorEnvelope(c, safeClientErr.Type, safeClientErr.Message))
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		// 透传模式默认保持原样代理；但 429/529 属于网关必须兜底的
		// 上游容量类错误，应先触发多账号 failover 以维持基础 SLA。
		if shouldFailoverOpenAIPassthroughResponse(resp.StatusCode) {
			return nil, s.handleFailoverErrorResponsePassthrough(ctx, resp, c, account, body)
		}
		return nil, s.handleErrorResponsePassthrough(ctx, resp, c, account, body)
	}

	var usage *OpenAIUsage
	var firstTokenMs *int
	imageCount := 0
	if reqStream {
		result, err := s.handleStreamingResponsePassthrough(ctx, resp, c, account, startTime)
		if err != nil {
			return nil, err
		}
		usage = result.usage
		firstTokenMs = result.firstTokenMs
		imageCount = result.imageCount
	} else {
		result, nonStreamErr := s.handleNonStreamingResponsePassthrough(ctx, resp, c, account)
		err = nonStreamErr
		if err != nil {
			return nil, err
		}
		usage = result.usage
		imageCount = result.imageCount
	}

	if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
		s.updateCodexUsageSnapshot(ctx, account.ID, snapshot)
	}

	if usage == nil {
		usage = &OpenAIUsage{}
	}

	return &OpenAIForwardResult{
		RequestID:       resp.Header.Get("x-request-id"),
		Usage:           *usage,
		Model:           reqModel,
		UpstreamModel:   upstreamPassthroughModel,
		ServiceTier:     extractOpenAIServiceTierFromBody(body),
		ReasoningEffort: reasoningEffort,
		Stream:          reqStream,
		OpenAIWSMode:    false,
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
		ImageCount:      imageCount,
	}, nil
}

func (s *OpenAIGatewayService) doOpenAIPassthroughRequestWithStatusRetry(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
	proxyURL string,
	stream bool,
) (*http.Response, []byte, error) {
	state := openAIResponsesRejectedStatusRetryStateForRequest(c, body)
	for {
		upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, stream)
		request, err := s.buildUpstreamRequestOpenAIPassthrough(upstreamCtx, c, account, body, token)
		releaseUpstreamCtx()
		if err != nil {
			return nil, body, err
		}
		resp, err := s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
		if err != nil || resp == nil || resp.StatusCode != http.StatusBadRequest {
			return resp, body, err
		}
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, body, readErr
		}
		retryBody, changed, normalizeErr := normalizeOpenAIResponsesRejectedStatusRetryBody(resp.StatusCode, body, respBody)
		if normalizeErr != nil {
			return nil, body, normalizeErr
		}
		if !changed || !state.Allow(retryBody) {
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
			return resp, body, nil
		}
		body = retryBody
		setOpsUpstreamRequestBody(c, body)
	}
}

// shouldFailoverOpenAITransportError reports whether an outbound transport
// failure may be recovered by selecting another account. A canceled client
// context is never retried: the caller is no longer waiting and another
// upstream attempt could create duplicate billable work.
func shouldFailoverOpenAITransportError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	return ctx != nil && ctx.Err() == nil
}

func logOpenAIPassthroughInstructionsRejected(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	reqModel string,
	rejectReason string,
	body []byte,
) {
	if ctx == nil {
		ctx = context.Background()
	}
	accountID := int64(0)
	accountName := ""
	accountType := ""
	if account != nil {
		accountID = account.ID
		accountName = strings.TrimSpace(account.Name)
		accountType = strings.TrimSpace(string(account.Type))
	}
	fields := []zap.Field{
		zap.String("component", "service.openai_gateway"),
		zap.Int64("account_id", accountID),
		zap.String("account_name", accountName),
		zap.String("account_type", accountType),
		zap.String("request_model", strings.TrimSpace(reqModel)),
		zap.String("reject_reason", strings.TrimSpace(rejectReason)),
	}
	fields = appendCodexCLIOnlyRejectedRequestFields(fields, c, body)
	logger.FromContext(ctx).With(fields...).Warn("OpenAI passthrough 本地拦截：Codex 请求缺少有效 instructions")
}

func (s *OpenAIGatewayService) buildUpstreamRequestOpenAIPassthrough(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
) (*http.Request, error) {
	targetURL := openaiPlatformAPIURL
	switch account.Type {
	case AccountTypeOAuth:
		targetURL = chatgptCodexURL
	case AccountTypeAPIKey:
		baseURL := account.GetOpenAIBaseURL()
		if baseURL != "" {
			validatedURL, err := s.validateUpstreamBaseURL(baseURL)
			if err != nil {
				return nil, err
			}
			targetURL = buildOpenAIResponsesURL(validatedURL)
		}
	}
	targetURL = appendOpenAIResponsesRequestPathSuffix(targetURL, openAIResponsesRequestPathSuffix(c))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// 透传客户端请求头（安全白名单）。
	allowTimeoutHeaders := s.isOpenAIPassthroughTimeoutHeadersAllowed()
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			lower := strings.ToLower(strings.TrimSpace(key))
			if !isOpenAIPassthroughAllowedRequestHeader(lower, allowTimeoutHeaders) {
				continue
			}
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}

	// 覆盖入站鉴权残留，并注入上游认证
	req.Header.Del("authorization")
	req.Header.Del("x-api-key")
	req.Header.Del("x-goog-api-key")
	req.Header.Set("authorization", "Bearer "+token)

	// OAuth 透传到 ChatGPT internal API 时补齐必要头。
	if account.Type == AccountTypeOAuth {
		promptCacheKey := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
		req.Host = "chatgpt.com"
		if chatgptAccountID := account.GetChatGPTAccountID(); chatgptAccountID != "" {
			req.Header.Set("chatgpt-account-id", chatgptAccountID)
		}
		apiKeyID := getAPIKeyIDFromContext(c)
		// 先保存客户端原始值，再做 compact 补充，避免后续统一隔离时读到已处理的值。
		clientSessionID := explicitOpenAIHeaderSessionID(c)
		clientConversationID := strings.TrimSpace(req.Header.Get("conversation_id"))
		if isOpenAIResponsesCompactPath(c) {
			req.Header.Set("accept", "application/json")
			if req.Header.Get("version") == "" {
				req.Header.Set("version", codexCLIVersion)
			}
			if clientSessionID == "" {
				clientSessionID = resolveOpenAICompactSessionID(c)
			}
		} else if req.Header.Get("accept") == "" {
			req.Header.Set("accept", "text/event-stream")
		}
		if req.Header.Get("OpenAI-Beta") == "" {
			req.Header.Set("OpenAI-Beta", "responses=experimental")
		}
		if req.Header.Get("originator") == "" {
			req.Header.Set("originator", "codex_cli_rs")
		}
		// 用隔离后的 session 标识符覆盖客户端透传值，防止跨用户会话碰撞。
		if clientSessionID == "" {
			clientSessionID = promptCacheKey
		}
		if clientConversationID == "" {
			clientConversationID = promptCacheKey
		}
		if clientSessionID != "" {
			req.Header.Set("session_id", isolateOpenAISessionID(apiKeyID, clientSessionID))
		}
		if clientConversationID != "" {
			req.Header.Set("conversation_id", isolateOpenAISessionID(apiKeyID, clientConversationID))
		}
	}

	// 透传模式也支持账户自定义 User-Agent 与 ForceCodexCLI 兜底。
	customUA := account.GetOpenAIUserAgent()
	if customUA != "" {
		req.Header.Set("user-agent", customUA)
	}
	if s.cfg != nil && s.cfg.Gateway.ForceCodexCLI {
		req.Header.Set("user-agent", codexCLIUserAgent)
	}
	// OAuth 安全透传：对非 Codex UA 统一兜底，降低被上游风控拦截概率。
	if account.Type == AccountTypeOAuth && !openai.IsCodexCLIRequest(req.Header.Get("user-agent")) {
		req.Header.Set("user-agent", codexCLIUserAgent)
	}

	if req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", "application/json")
	}

	return req, nil
}

func shouldFailoverOpenAIPassthroughResponse(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, 529:
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) handleFailoverErrorResponsePassthrough(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestBody []byte,
) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))

	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(body), maxBytes)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
	logOpenAIInstructionsRequiredDebug(ctx, c, account, resp.StatusCode, upstreamMsg, requestBody, body)
	if s.rateLimitService != nil {
		_ = s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:             account.Platform,
		AccountID:            account.ID,
		AccountName:          account.Name,
		UpstreamStatusCode:   resp.StatusCode,
		UpstreamRequestID:    resp.Header.Get("x-request-id"),
		Passthrough:          true,
		Kind:                 "failover",
		Message:              upstreamMsg,
		Detail:               upstreamDetail,
		UpstreamResponseBody: upstreamDetail,
	})
	return &UpstreamFailoverError{
		StatusCode:             resp.StatusCode,
		ResponseBody:           body,
		ResponseHeaders:        resp.Header.Clone(),
		RetryableOnSameAccount: isTransientOpenAIOAuth429(account, resp.StatusCode, resp.Header, body),
	}
}

func (s *OpenAIGatewayService) handleErrorResponsePassthrough(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestBody []byte,
) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))

	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(body), maxBytes)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
	logOpenAIInstructionsRequiredDebug(ctx, c, account, resp.StatusCode, upstreamMsg, requestBody, body)
	if s.rateLimitService != nil {
		// Passthrough mode preserves the raw upstream error response, but runtime
		// account state still needs to be updated so sticky routing can stop
		// reusing a freshly rate-limited account.
		_ = s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:             account.Platform,
		AccountID:            account.ID,
		AccountName:          account.Name,
		UpstreamStatusCode:   resp.StatusCode,
		UpstreamRequestID:    resp.Header.Get("x-request-id"),
		Passthrough:          true,
		Kind:                 "http_error",
		Message:              upstreamMsg,
		Detail:               upstreamDetail,
		UpstreamResponseBody: upstreamDetail,
	})

	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	MarkResponseCommitted(c)
	safeErr := SafeClientUpstreamError(resp.StatusCode)
	c.JSON(safeErr.StatusCode, OpenAIClientErrorEnvelope(c, safeErr.Type, safeErr.Message))

	if upstreamMsg == "" {
		return fmt.Errorf("upstream error: %d", resp.StatusCode)
	}
	return fmt.Errorf("upstream error: %d message=%s", resp.StatusCode, upstreamMsg)
}

func isOpenAIPassthroughAllowedRequestHeader(lowerKey string, allowTimeoutHeaders bool) bool {
	if lowerKey == "" {
		return false
	}
	if isOpenAIPassthroughTimeoutHeader(lowerKey) {
		return allowTimeoutHeaders
	}
	return openaiPassthroughAllowedHeaders[lowerKey]
}

func isOpenAIPassthroughTimeoutHeader(lowerKey string) bool {
	switch lowerKey {
	case "x-stainless-timeout", "x-stainless-read-timeout", "x-stainless-connect-timeout", "x-request-timeout", "request-timeout", "grpc-timeout":
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) isOpenAIPassthroughTimeoutHeadersAllowed() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIPassthroughAllowTimeoutHeaders
}

func collectOpenAIPassthroughTimeoutHeaders(h http.Header) []string {
	if h == nil {
		return nil
	}
	var matched []string
	for key, values := range h {
		lowerKey := strings.ToLower(strings.TrimSpace(key))
		if isOpenAIPassthroughTimeoutHeader(lowerKey) {
			entry := lowerKey
			if len(values) > 0 {
				entry = fmt.Sprintf("%s=%s", lowerKey, strings.Join(values, "|"))
			}
			matched = append(matched, entry)
		}
	}
	sort.Strings(matched)
	return matched
}

type openaiStreamingResultPassthrough struct {
	usage        *OpenAIUsage
	firstTokenMs *int
	imageCount   int
}

type openaiNonStreamingResultPassthrough struct {
	usage      *OpenAIUsage
	imageCount int
}

func openAIStreamClientOutputStarted(c *gin.Context, localStarted bool) bool {
	if localStarted {
		return true
	}
	return c != nil && c.Writer != nil && c.Writer.Written()
}

func openAIStreamEventIsPreamble(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.created", "response.in_progress":
		return true
	default:
		return false
	}
}

func openAIStreamDataStartsClientOutput(data, eventType string) bool {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return false
	}
	switch strings.TrimSpace(eventType) {
	case "response.failed":
		return false
	case "error":
		// OpenAI can emit a retryable error frame immediately before
		// response.failed. Flushing that frame would commit the downstream
		// response and make the following pre-output account failover unsafe.
		payload := []byte(trimmed)
		return !openAIStreamFailedEventShouldFailover(payload, extractOpenAISSEErrorMessage(payload))
	}
	return !openAIStreamEventIsPreamble(eventType)
}

func openAIStreamFailedEventErrorCode(payload []byte) string {
	code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(payload, "response.error.code").String()))
	if code == "" {
		code = strings.ToLower(strings.TrimSpace(gjson.GetBytes(payload, "error.code").String()))
	}
	return code
}

// isOpenAIUpstreamCapacityShedEvent identifies the HTTP-200 stream failure
// OpenAI uses when a request is shed before model output.
func isOpenAIUpstreamCapacityShedEvent(payload []byte) bool {
	switch openAIStreamFailedEventErrorCode(payload) {
	case "server_is_overloaded", "slow_down":
		return true
	default:
		return false
	}
}

const openAICapacityShedRetryableClientCode = "server_error"

// sanitizeOpenAICapacityShedErrorCodeForClient preserves the upstream message
// while replacing capacity codes that Codex treats as terminal. Monitoring and
// failover classification continue to use the original payload.
func sanitizeOpenAICapacityShedErrorCodeForClient(payload []byte) ([]byte, bool) {
	if len(payload) == 0 || !gjson.ValidBytes(payload) || !isOpenAIUpstreamCapacityShedEvent(payload) {
		return payload, false
	}
	updated := payload
	changed := false
	for _, path := range []string{"response.error.code", "error.code"} {
		switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(updated, path).String())) {
		case "server_is_overloaded", "slow_down":
		default:
			continue
		}
		next, err := sjson.SetBytes(updated, path, openAICapacityShedRetryableClientCode)
		if err != nil {
			return payload, false
		}
		updated = next
		changed = true
	}
	return updated, changed
}

func openAIStreamFailedEventShouldFailover(payload []byte, message string) bool {
	code := strings.ToLower(strings.TrimSpace(gjson.GetBytes(payload, "response.error.code").String()))
	if code == "" {
		code = strings.ToLower(strings.TrimSpace(gjson.GetBytes(payload, "error.code").String()))
	}
	errType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(payload, "response.error.type").String()))
	if errType == "" {
		errType = strings.ToLower(strings.TrimSpace(gjson.GetBytes(payload, "error.type").String()))
	}
	combined := strings.ToLower(strings.TrimSpace(message + " " + code + " " + errType))
	if combined == "" {
		return true
	}
	if IsOpenAIContextWindowExceeded(combined, string(payload)) {
		return false
	}
	nonRetryableMarkers := []string{
		"invalid_request",
		"content_policy",
		"policy",
		"safety",
		"high-risk cyber",
		"not allowed",
		"violat",
	}
	for _, marker := range nonRetryableMarkers {
		if strings.Contains(combined, marker) {
			return false
		}
	}
	return true
}

func (s *OpenAIGatewayService) newOpenAIStreamFailoverError(
	c *gin.Context,
	account *Account,
	passthrough bool,
	upstreamRequestID string,
	payload []byte,
	message string,
) *UpstreamFailoverError {
	message = sanitizeUpstreamErrorMessage(strings.TrimSpace(message))
	if message == "" {
		message = "OpenAI stream disconnected before completion"
	}
	detail := ""
	if len(payload) > 0 && s != nil && s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		detail = truncateString(string(payload), maxBytes)
	}
	if c != nil {
		setOpsUpstreamError(c, http.StatusBadGateway, message, detail)
		event := OpsUpstreamErrorEvent{
			Platform:           PlatformOpenAI,
			UpstreamStatusCode: http.StatusBadGateway,
			UpstreamRequestID:  strings.TrimSpace(upstreamRequestID),
			Passthrough:        passthrough,
			Kind:               "failover",
			Message:            message,
			Detail:             detail,
		}
		if account != nil {
			event.Platform = account.Platform
			event.AccountID = account.ID
			event.AccountName = account.Name
		}
		appendOpsUpstreamError(c, event)
	}
	safeClientErr := SafeOpenAIClientUpstreamError(http.StatusBadGateway, message, string(payload))
	body, _ := json.Marshal(OpenAIClientUpstreamErrorEnvelope(c, safeClientErr))
	return &UpstreamFailoverError{
		StatusCode:             safeClientErr.StatusCode,
		ResponseBody:           body,
		RetryableOnSameAccount: isTransientOpenAIOAuth429(account, safeClientErr.StatusCode, nil, payload) || (safeClientErr.Code == "" && isOpenAIUpstreamCapacityShedEvent(payload)),
	}
}

func (s *OpenAIGatewayService) handleStreamingResponsePassthrough(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	startTime time.Time,
) (*openaiStreamingResultPassthrough, error) {
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)

	// SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	if v := resp.Header.Get("x-request-id"); v != "" {
		c.Header("x-request-id", v)
	}

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}
	bufferedWriter := bufio.NewWriterSize(w, 4*1024)
	flushBuffered := func() error {
		if err := bufferedWriter.Flush(); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	usage := &OpenAIUsage{}
	var firstTokenMs *int
	imageCount := 0
	clientDisconnected := false
	sawDone := false
	sawTerminalEvent := false
	sawFailedEvent := false
	failedMessage := ""
	clientOutputStarted := false
	upstreamRequestID := strings.TrimSpace(resp.Header.Get("x-request-id"))

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanBuf := getSSEScannerBuf64K()
	scanner.Buffer(scanBuf[:0], maxLineSize)
	defer putSSEScannerBuf64K(scanBuf)

	for scanner.Scan() {
		line := scanner.Text()
		if data, ok := extractOpenAISSEDataLine(line); ok {
			dataBytes := []byte(data)
			if normalizedPayload, normalized := normalizeCompletedOpenAIResponseImageStatuses(dataBytes); normalized > 0 {
				dataBytes = normalizedPayload
				data = string(normalizedPayload)
				line = "data: " + data
				logger.FromContext(ctx).Warn(
					"openai.responses_normalized_stale_image_status",
					zap.Int64("account_id", account.ID),
					zap.Int("image_call_count", normalized),
				)
			}
			if completedImages := countCompletedOpenAIResponseImages(dataBytes); completedImages > imageCount {
				imageCount = completedImages
			}
			trimmedData := strings.TrimSpace(data)
			if trimmedData == "[DONE]" {
				sawDone = true
			}
			if openAIStreamEventIsTerminal(trimmedData) {
				sawTerminalEvent = true
			}
			eventType := strings.TrimSpace(gjson.GetBytes(dataBytes, "type").String())
			usageDataBytes := dataBytes
			forceFlushFailedEvent := false
			switch eventType {
			case "response.failed":
				failedMessage = extractOpenAISSEErrorMessage(dataBytes)
				if !openAIStreamClientOutputStarted(c, clientOutputStarted) && openAIStreamFailedEventShouldFailover(dataBytes, failedMessage) {
					return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount},
						s.newOpenAIStreamFailoverError(c, account, true, upstreamRequestID, dataBytes, failedMessage)
				}
				forceFlushFailedEvent = true
				sawFailedEvent = true
				safeClientErr := SafeOpenAIClientUpstreamError(http.StatusBadGateway, failedMessage, string(dataBytes))
				clientCode := safeClientErr.Code
				if clientCode == "" {
					clientCode = "server_error"
				}
				responseID := strings.TrimSpace(gjson.GetBytes(dataBytes, "response.id").String())
				model := strings.TrimSpace(gjson.GetBytes(dataBytes, "response.model").String())
				safePayload, _ := json.Marshal(OpenAIResponsesFailedEnvelope(c, responseID, model, clientCode, safeClientErr.Message))
				line = "data: " + string(safePayload)
			case "error":
				if safePayload, changed := sanitizeOpenAICapacityShedErrorCodeForClient(dataBytes); changed {
					trimmedData = string(safePayload)
					line = "data: " + trimmedData
				}
			}
			startsClientOutput := forceFlushFailedEvent || openAIStreamDataStartsClientOutput(trimmedData, eventType)
			if firstTokenMs == nil && startsClientOutput {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			s.parseSSEUsageBytes(usageDataBytes, usage)

			if !clientDisconnected {
				shouldFlush := clientOutputStarted || startsClientOutput
				if _, err := bufferedWriter.WriteString(line + "\n"); err != nil {
					clientDisconnected = true
					logger.LegacyPrintf("service.openai_gateway", "[OpenAI passthrough] Client disconnected during streaming, continue draining upstream for usage: account=%d", account.ID)
				} else if shouldFlush {
					if err := flushBuffered(); err != nil {
						clientDisconnected = true
						logger.LegacyPrintf("service.openai_gateway", "[OpenAI passthrough] Client disconnected during streaming flush, continue draining upstream for usage: account=%d", account.ID)
					} else {
						clientOutputStarted = true
					}
				}
			}
			continue
		}

		if !clientDisconnected {
			if _, err := bufferedWriter.WriteString(line + "\n"); err != nil {
				clientDisconnected = true
				logger.LegacyPrintf("service.openai_gateway", "[OpenAI passthrough] Client disconnected during streaming, continue draining upstream for usage: account=%d", account.ID)
			} else if clientOutputStarted {
				if err := flushBuffered(); err != nil {
					clientDisconnected = true
					logger.LegacyPrintf("service.openai_gateway", "[OpenAI passthrough] Client disconnected during streaming flush, continue draining upstream for usage: account=%d", account.ID)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if sawTerminalEvent {
			return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount}, nil
		}
		if clientDisconnected {
			return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount}, fmt.Errorf("stream usage incomplete after disconnect: %w", err)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount}, fmt.Errorf("stream usage incomplete: %w", err)
		}
		if errors.Is(err, bufio.ErrTooLong) {
			logger.LegacyPrintf("service.openai_gateway", "[OpenAI passthrough] SSE line too long: account=%d max_size=%d error=%v", account.ID, maxLineSize, err)
			return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount}, err
		}
		if !openAIStreamClientOutputStarted(c, clientOutputStarted) {
			msg := "OpenAI stream disconnected before completion"
			if errText := strings.TrimSpace(err.Error()); errText != "" {
				msg += ": " + errText
			}
			return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount},
				s.newOpenAIStreamFailoverError(c, account, true, upstreamRequestID, nil, msg)
		}
		logger.LegacyPrintf("service.openai_gateway",
			"[OpenAI passthrough] 流读取异常中断: account=%d request_id=%s err=%v",
			account.ID,
			upstreamRequestID,
			err,
		)
		return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount}, fmt.Errorf("stream read error: %w", err)
	}
	if sawFailedEvent {
		return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount}, fmt.Errorf("upstream response failed: %s", failedMessage)
	}
	if !clientDisconnected && !sawDone && !sawTerminalEvent && ctx.Err() == nil {
		logger.FromContext(ctx).With(
			zap.String("component", "service.openai_gateway"),
			zap.Int64("account_id", account.ID),
			zap.String("upstream_request_id", upstreamRequestID),
		).Info("OpenAI passthrough 上游流在未收到 [DONE] 时结束，疑似断流")
		if !openAIStreamClientOutputStarted(c, clientOutputStarted) {
			return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount},
				s.newOpenAIStreamFailoverError(c, account, true, upstreamRequestID, nil, "OpenAI stream ended before a terminal event")
		}
		return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount}, errors.New("stream usage incomplete: missing terminal event")
	}
	if !clientDisconnected && bufferedWriter.Buffered() > 0 {
		if err := flushBuffered(); err != nil {
			logger.LegacyPrintf("service.openai_gateway", "[OpenAI passthrough] Client disconnected during final flush, returning collected usage: account=%d", account.ID)
		}
	}

	return &openaiStreamingResultPassthrough{usage: usage, firstTokenMs: firstTokenMs, imageCount: imageCount}, nil
}

func (s *OpenAIGatewayService) handleNonStreamingResponsePassthrough(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
) (*openaiNonStreamingResultPassthrough, error) {
	body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}

	// Detect SSE responses from upstream and convert to JSON.
	// Some upstreams (e.g. other sub2api instances) may return SSE even when
	// stream=false was requested. Without this conversion the client would
	// receive raw SSE text or a terminal event with empty output.
	if isEventStreamResponse(resp.Header) {
		return s.handlePassthroughSSEToJSON(ctx, resp, c, account, body)
	}
	if normalizedBody, normalized := normalizeCompletedOpenAIResponseImageStatuses(body); normalized > 0 {
		body = normalizedBody
		logger.FromContext(ctx).Warn(
			"openai.responses_normalized_stale_image_status",
			zap.Int64("account_id", account.ID),
			zap.Int("image_call_count", normalized),
		)
	}
	imageCount := countCompletedOpenAIResponseImages(body)

	usage := &OpenAIUsage{}
	usageParsed := false
	if len(body) > 0 {
		if parsedUsage, ok := extractOpenAIUsageFromJSONBytes(body); ok {
			*usage = parsedUsage
			usageParsed = true
		}
	}
	if !usageParsed {
		// 兜底：尝试从 SSE 文本中解析 usage
		usage = s.parseSSEUsageFromBody(string(body))
	}

	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, body)
	return &openaiNonStreamingResultPassthrough{usage: usage, imageCount: imageCount}, nil
}

// handlePassthroughSSEToJSON converts an SSE response body into a JSON
// response for the passthrough path. It mirrors handleSSEToJSON but skips
// model replacement (passthrough does not remap models).
func (s *OpenAIGatewayService) handlePassthroughSSEToJSON(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	body []byte,
) (*openaiNonStreamingResultPassthrough, error) {
	bodyText := string(body)
	finalResponse, ok := extractCodexFinalResponse(bodyText)

	usage := &OpenAIUsage{}
	if ok {
		if parsedUsage, parsed := extractOpenAIUsageFromJSONBytes(finalResponse); parsed {
			*usage = parsedUsage
		}
		// When the terminal event has an empty output array, reconstruct
		// output from accumulated delta events so the client gets full content.
		if len(gjson.GetBytes(finalResponse, "output").Array()) == 0 {
			if outputJSON, reconstructed := reconstructResponseOutputFromSSE(bodyText); reconstructed {
				if patched, err := sjson.SetRawBytes(finalResponse, "output", outputJSON); err == nil {
					finalResponse = patched
				}
			}
		}
		if normalizedResponse, normalized := normalizeCompletedOpenAIResponseImageStatuses(finalResponse); normalized > 0 {
			finalResponse = normalizedResponse
			logger.FromContext(ctx).Warn(
				"openai.responses_normalized_stale_image_status",
				zap.Int64("account_id", account.ID),
				zap.Int("image_call_count", normalized),
			)
		}
		body = finalResponse
		// Correct tool calls in final response
		body = s.correctToolCallsInResponseBody(body)
	} else {
		terminalType, terminalPayload, terminalOK := extractOpenAISSETerminalEvent(bodyText)
		if terminalOK && terminalType == "response.failed" {
			msg := extractOpenAISSEErrorMessage(terminalPayload)
			if msg == "" {
				msg = "Upstream compact response failed"
			}
			return nil, s.writeOpenAINonStreamingProtocolError(resp, c, msg)
		}
		usage = s.parseSSEUsageFromBody(bodyText)
	}

	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)

	contentType := "application/json; charset=utf-8"
	if !ok {
		contentType = resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "text/event-stream"
		}
	}
	c.Data(resp.StatusCode, contentType, body)

	return &openaiNonStreamingResultPassthrough{
		usage:      usage,
		imageCount: countCompletedOpenAIResponseImages(body),
	}, nil
}

func writeOpenAIPassthroughResponseHeaders(dst http.Header, src http.Header, filter *responseheaders.CompiledHeaderFilter) {
	if dst == nil || src == nil {
		return
	}
	if filter != nil {
		responseheaders.WriteFilteredHeaders(dst, src, filter)
	} else {
		// 兜底：尽量保留最基础的 content-type
		if v := strings.TrimSpace(src.Get("Content-Type")); v != "" {
			dst.Set("Content-Type", v)
		}
	}
	// 透传模式强制放行 x-codex-* 响应头（若上游返回）。
	// 注意：真实 http.Response.Header 的 key 一般会被 canonicalize；但为了兼容测试/自建响应，
	// 这里用 EqualFold 做一次大小写不敏感的查找。
	getCaseInsensitiveValues := func(h http.Header, want string) []string {
		if h == nil {
			return nil
		}
		for k, vals := range h {
			if strings.EqualFold(k, want) {
				return vals
			}
		}
		return nil
	}

	for _, rawKey := range []string{
		"x-codex-primary-used-percent",
		"x-codex-primary-reset-after-seconds",
		"x-codex-primary-window-minutes",
		"x-codex-secondary-used-percent",
		"x-codex-secondary-reset-after-seconds",
		"x-codex-secondary-window-minutes",
		"x-codex-primary-over-secondary-limit-percent",
	} {
		vals := getCaseInsensitiveValues(src, rawKey)
		if len(vals) == 0 {
			continue
		}
		key := http.CanonicalHeaderKey(rawKey)
		dst.Del(key)
		for _, v := range vals {
			dst.Add(key, v)
		}
	}
}

func (s *OpenAIGatewayService) buildUpstreamRequest(ctx context.Context, c *gin.Context, account *Account, body []byte, token string, isStream bool, promptCacheKey string, isCodexCLI bool) (*http.Request, error) {
	// Determine target URL based on account type
	var targetURL string
	switch account.Type {
	case AccountTypeOAuth:
		// OAuth accounts use ChatGPT internal API
		targetURL = chatgptCodexURL
	case AccountTypeAPIKey:
		// API Key accounts use Platform API or custom base URL
		baseURL := account.GetOpenAIBaseURL()
		if baseURL == "" {
			targetURL = openaiPlatformAPIURL
		} else {
			validatedURL, err := s.validateUpstreamBaseURL(baseURL)
			if err != nil {
				return nil, err
			}
			targetURL = buildOpenAIResponsesURL(validatedURL)
		}
	default:
		targetURL = openaiPlatformAPIURL
	}
	targetURL = appendOpenAIResponsesRequestPathSuffix(targetURL, openAIResponsesRequestPathSuffix(c))

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Set authentication header
	req.Header.Set("authorization", "Bearer "+token)

	// Set headers specific to OAuth accounts (ChatGPT internal API)
	if account.Type == AccountTypeOAuth {
		// Required: set Host for ChatGPT API (must use req.Host, not Header.Set)
		req.Host = "chatgpt.com"
		// Required: set chatgpt-account-id header
		chatgptAccountID := account.GetChatGPTAccountID()
		if chatgptAccountID != "" {
			req.Header.Set("chatgpt-account-id", chatgptAccountID)
		}
	}

	// Whitelist passthrough headers
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if openaiAllowedHeaders[lowerKey] {
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}
	if account.Type == AccountTypeOAuth {
		// 清除客户端透传的 session 头，后续用隔离后的值重新设置，防止跨用户会话碰撞。
		req.Header.Del("conversation_id")
		req.Header.Del("session_id")

		clientSessionID := explicitOpenAIHeaderSessionID(c)
		clientConversationID := strings.TrimSpace(c.GetHeader("conversation_id"))
		req.Header.Set("OpenAI-Beta", "responses=experimental")
		req.Header.Set("originator", resolveOpenAIUpstreamOriginator(c, isCodexCLI))
		apiKeyID := getAPIKeyIDFromContext(c)
		if isOpenAIResponsesCompactPath(c) {
			req.Header.Set("accept", "application/json")
			if req.Header.Get("version") == "" {
				req.Header.Set("version", codexCLIVersion)
			}
			compactSession := resolveOpenAICompactSessionID(c)
			req.Header.Set("session_id", isolateOpenAISessionID(apiKeyID, compactSession))
		} else {
			req.Header.Set("accept", "text/event-stream")
			if clientSessionID != "" {
				req.Header.Set("session_id", isolateOpenAISessionID(apiKeyID, clientSessionID))
			}
			if clientConversationID != "" {
				req.Header.Set("conversation_id", isolateOpenAISessionID(apiKeyID, clientConversationID))
			}
		}
		if promptCacheKey != "" {
			isolated := isolateOpenAISessionID(apiKeyID, promptCacheKey)
			if req.Header.Get("conversation_id") == "" {
				req.Header.Set("conversation_id", isolated)
			}
			if req.Header.Get("session_id") == "" {
				req.Header.Set("session_id", isolated)
			}
		}
	}

	// Apply custom User-Agent if configured
	customUA := account.GetOpenAIUserAgent()
	if customUA != "" {
		req.Header.Set("user-agent", customUA)
	}

	// 若开启 ForceCodexCLI，则强制将上游 User-Agent 伪装为 Codex CLI。
	// 用于网关未透传/改写 User-Agent 时，仍能命中 Codex 侧识别逻辑。
	if s.cfg != nil && s.cfg.Gateway.ForceCodexCLI {
		req.Header.Set("user-agent", codexCLIUserAgent)
	}

	// Ensure required headers exist
	if req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", "application/json")
	}

	return req, nil
}

func (s *OpenAIGatewayService) handleErrorResponse(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestBody []byte,
	requestedModel ...string,
) (*OpenAIForwardResult, error) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if account != nil && account.Platform == PlatformGrok && isGrokContentPolicyRejection(resp.StatusCode, body) {
		clientMsg := grokContentPolicyClientMessage(body)
		setOpsUpstreamError(c, resp.StatusCode, clientMsg, truncateString(string(body), 2048))
		MarkResponseCommitted(c)
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{"type": "invalid_request_error", "message": clientMsg},
		})
		return nil, fmt.Errorf("grok content policy rejection: %s", clientMsg)
	}

	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(body), maxBytes)
	}

	if safeErr := SafeOpenAIClientUpstreamError(resp.StatusCode, upstreamMsg, string(body)); safeErr.Code == ClientCodeContextWindowExceeded {
		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  resp.Header.Get("x-request-id"),
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		MarkResponseCommitted(c)
		c.JSON(safeErr.StatusCode, OpenAIClientUpstreamErrorEnvelope(c, safeErr))
		return nil, fmt.Errorf("upstream context window exceeded: %s", upstreamMsg)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
	logOpenAIInstructionsRequiredDebug(ctx, c, account, resp.StatusCode, upstreamMsg, requestBody, body)

	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		logger.LegacyPrintf("service.openai_gateway",
			"OpenAI upstream error %d (account=%d platform=%s type=%s): %s",
			resp.StatusCode,
			account.ID,
			account.Platform,
			account.Type,
			truncateForLog(body, s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes),
		)
	}

	if status, errType, errMsg, matched := applyErrorPassthroughRule(
		c,
		PlatformOpenAI,
		resp.StatusCode,
		body,
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed",
	); matched {
		MarkResponseCommitted(c)
		c.JSON(status, OpenAIClientErrorEnvelope(c, errType, errMsg))
		if upstreamMsg == "" {
			upstreamMsg = errMsg
		}
		if upstreamMsg == "" {
			return nil, fmt.Errorf("upstream error: %d (passthrough rule matched)", resp.StatusCode)
		}
		return nil, fmt.Errorf("upstream error: %d (passthrough rule matched) message=%s", resp.StatusCode, upstreamMsg)
	}

	// Check custom error codes
	if !account.ShouldHandleErrorCode(resp.StatusCode) {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  resp.Header.Get("x-request-id"),
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		MarkResponseCommitted(c)
		safeErr := SafeClientUpstreamError(resp.StatusCode)
		c.JSON(safeErr.StatusCode, OpenAIClientUpstreamErrorEnvelope(c, safeErr))
		if upstreamMsg == "" {
			return nil, fmt.Errorf("upstream error: %d (not in custom error codes)", resp.StatusCode)
		}
		return nil, fmt.Errorf("upstream error: %d (not in custom error codes) message=%s", resp.StatusCode, upstreamMsg)
	}

	// Handle upstream error (mark account status)
	shouldDisable := false
	if account != nil && account.Platform == PlatformGrok {
		s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
		shouldDisable = s.shouldFailoverGrokUpstreamError(resp.StatusCode, body)
	} else if s.rateLimitService != nil {
		shouldDisable = s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
	}
	kind := "http_error"
	if shouldDisable {
		kind = "failover"
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  resp.Header.Get("x-request-id"),
		Kind:               kind,
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})
	if shouldDisable {
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           body,
			RetryableOnSameAccount: allowOpenAISameAccountRetry(account.IsPoolMode() && isPoolModeRetryableStatus(resp.StatusCode), body),
		}
	}

	MarkResponseCommitted(c)

	safeErr := SafeClientUpstreamError(resp.StatusCode)
	c.JSON(safeErr.StatusCode, OpenAIClientUpstreamErrorEnvelope(c, safeErr))

	if upstreamMsg == "" {
		return nil, fmt.Errorf("upstream error: %d", resp.StatusCode)
	}
	return nil, fmt.Errorf("upstream error: %d message=%s", resp.StatusCode, upstreamMsg)
}

// compatErrorWriter is the signature for format-specific error writers used by
// the compat paths (Chat Completions and Anthropic Messages).
type compatErrorWriter func(c *gin.Context, statusCode int, errType, message string)

// handleCompatErrorResponse is the shared non-failover error handler for the
// Chat Completions and Anthropic Messages compat paths. It mirrors the logic of
// handleErrorResponse (passthrough rules, ShouldHandleErrorCode, rate-limit
// tracking, secondary failover) but delegates the final error write to the
// format-specific writer function.
func (s *OpenAIGatewayService) handleCompatErrorResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	writeError compatErrorWriter,
	requestedModel ...string,
) (*OpenAIForwardResult, error) {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if account != nil && account.Platform == PlatformGrok && isGrokContentPolicyRejection(resp.StatusCode, body) {
		clientMsg := grokContentPolicyClientMessage(body)
		setOpsUpstreamError(c, resp.StatusCode, clientMsg, truncateString(string(body), 2048))
		MarkResponseCommitted(c)
		writeError(c, http.StatusForbidden, "invalid_request_error", clientMsg)
		return nil, fmt.Errorf("grok content policy rejection: %s", clientMsg)
	}

	upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(body))
	if upstreamMsg == "" {
		upstreamMsg = fmt.Sprintf("Upstream error: %d", resp.StatusCode)
	}
	upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)

	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(body), maxBytes)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
	if safeErr := SafeOpenAIClientUpstreamError(resp.StatusCode, upstreamMsg, string(body)); safeErr.Code == ClientCodeContextWindowExceeded {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  resp.Header.Get("x-request-id"),
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		MarkResponseCommitted(c)
		writeError(c, safeErr.StatusCode, safeErr.Type, safeErr.Message)
		return nil, fmt.Errorf("upstream context window exceeded: %s", upstreamMsg)
	}

	// Apply error passthrough rules
	if status, errType, errMsg, matched := applyErrorPassthroughRule(
		c, account.Platform, resp.StatusCode, body,
		SafeClientUpstreamError(resp.StatusCode).StatusCode, SafeClientUpstreamError(resp.StatusCode).Type, SafeClientUpstreamError(resp.StatusCode).Message,
	); matched {
		MarkResponseCommitted(c)
		writeError(c, status, errType, errMsg)
		if upstreamMsg == "" {
			upstreamMsg = errMsg
		}
		if upstreamMsg == "" {
			return nil, fmt.Errorf("upstream error: %d (passthrough rule matched)", resp.StatusCode)
		}
		return nil, fmt.Errorf("upstream error: %d (passthrough rule matched) message=%s", resp.StatusCode, upstreamMsg)
	}

	// Check custom error codes — if the account does not handle this status,
	// return a generic error without exposing upstream details.
	if !account.ShouldHandleErrorCode(resp.StatusCode) {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  resp.Header.Get("x-request-id"),
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		MarkResponseCommitted(c)
		safeErr := SafeClientUpstreamError(resp.StatusCode)
		writeError(c, safeErr.StatusCode, safeErr.Type, safeErr.Message)
		if upstreamMsg == "" {
			return nil, fmt.Errorf("upstream error: %d (not in custom error codes)", resp.StatusCode)
		}
		return nil, fmt.Errorf("upstream error: %d (not in custom error codes) message=%s", resp.StatusCode, upstreamMsg)
	}

	// Track rate limits and decide whether to trigger secondary failover.
	shouldDisable := false
	if account != nil && account.Platform == PlatformGrok {
		s.handleGrokAccountUpstreamError(c.Request.Context(), account, resp.StatusCode, resp.Header, body)
		shouldDisable = s.shouldFailoverGrokUpstreamError(resp.StatusCode, body)
	} else if s.rateLimitService != nil {
		shouldDisable = s.rateLimitService.HandleUpstreamError(
			c.Request.Context(), account, resp.StatusCode, resp.Header, body,
		)
	}
	kind := "http_error"
	if shouldDisable {
		kind = "failover"
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  resp.Header.Get("x-request-id"),
		Kind:               kind,
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})
	if shouldDisable {
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           body,
			RetryableOnSameAccount: allowOpenAISameAccountRetry(account.IsPoolMode() && isPoolModeRetryableStatus(resp.StatusCode), body),
		}
	}

	MarkResponseCommitted(c)

	safeErr := SafeClientUpstreamError(resp.StatusCode)
	writeError(c, safeErr.StatusCode, safeErr.Type, safeErr.Message)
	return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
}

// openaiStreamingResult streaming response result
type openaiStreamingResult struct {
	usage        *OpenAIUsage
	firstTokenMs *int
	responseID   string
	imageCount   int
}

// openaiNonStreamingResult embeds usage for backwards-compatible callers and
// also carries protocol metadata needed by native Responses clients.
type openaiNonStreamingResult struct {
	*OpenAIUsage
	usage      *OpenAIUsage
	responseID string
	imageCount int
}

func (s *OpenAIGatewayService) handleStreamingResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, startTime time.Time, originalModel, mappedModel string) (*openaiStreamingResult, error) {
	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}

	// Set SSE response headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Pass through other headers
	if v := resp.Header.Get("x-request-id"); v != "" {
		c.Header("x-request-id", v)
	}

	w := c.Writer
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}
	bufferedWriter := bufio.NewWriterSize(w, 4*1024)
	flushBuffered := func() error {
		if err := bufferedWriter.Flush(); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	usage := &OpenAIUsage{}
	var firstTokenMs *int
	imageCount := 0
	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanBuf := getSSEScannerBuf64K()
	scanner.Buffer(scanBuf[:0], maxLineSize)

	streamInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamDataIntervalTimeout > 0 {
		streamInterval = time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout) * time.Second
	}
	// 仅监控上游数据间隔超时，不被下游写入阻塞影响
	var intervalTicker *time.Ticker
	if streamInterval > 0 {
		intervalTicker = time.NewTicker(streamInterval)
		defer intervalTicker.Stop()
	}
	var intervalCh <-chan time.Time
	if intervalTicker != nil {
		intervalCh = intervalTicker.C
	}

	keepaliveInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamKeepaliveInterval > 0 {
		keepaliveInterval = time.Duration(s.cfg.Gateway.StreamKeepaliveInterval) * time.Second
	}
	// 下游 keepalive 仅用于防止代理空闲断开
	var keepaliveTicker *time.Ticker
	if keepaliveInterval > 0 {
		keepaliveTicker = time.NewTicker(keepaliveInterval)
		defer keepaliveTicker.Stop()
	}
	var keepaliveCh <-chan time.Time
	if keepaliveTicker != nil {
		keepaliveCh = keepaliveTicker.C
	}
	lastDownstreamWriteAt := time.Now()

	// 仅发送一次错误事件，避免多次写入导致协议混乱。
	// 注意：OpenAI `/v1/responses` streaming 事件必须符合 OpenAI Responses schema；
	// 否则下游 SDK（例如 OpenCode）会因为类型校验失败而报错。
	errorEventSent := false
	clientDisconnected := false // 客户端断开后继续 drain 上游以收集 usage
	sawTerminalEvent := false
	sawFailedEvent := false
	failedMessage := ""
	clientOutputStarted := false
	upstreamRequestID := strings.TrimSpace(resp.Header.Get("x-request-id"))
	responseID := ""
	var streamFailoverErr error
	sendErrorEvent := func(reason string) {
		if errorEventSent || clientDisconnected {
			return
		}
		errorEventSent = true
		safeClientErr := SafeClientUpstreamError(http.StatusBadGateway)
		payloadBytes, _ := json.Marshal(OpenAIResponsesFailedEnvelope(c, "", originalModel, "server_error", safeClientErr.Message))
		payload := string(payloadBytes)
		if err := flushBuffered(); err != nil {
			clientDisconnected = true
			return
		}
		if _, err := bufferedWriter.WriteString("data: " + payload + "\n\n"); err != nil {
			clientDisconnected = true
			return
		}
		if err := flushBuffered(); err != nil {
			clientDisconnected = true
		}
		clientOutputStarted = true
		lastDownstreamWriteAt = time.Now()
	}

	needModelReplace := originalModel != mappedModel
	resultWithUsage := func() *openaiStreamingResult {
		return &openaiStreamingResult{usage: usage, firstTokenMs: firstTokenMs, responseID: responseID, imageCount: imageCount}
	}
	finalizeStream := func() (*openaiStreamingResult, error) {
		if !sawTerminalEvent {
			if !openAIStreamClientOutputStarted(c, clientOutputStarted) {
				return resultWithUsage(), s.newOpenAIStreamFailoverError(c, account, false, upstreamRequestID, nil, "OpenAI stream ended before a terminal event")
			}
			return resultWithUsage(), fmt.Errorf("stream usage incomplete: missing terminal event")
		}
		if sawFailedEvent {
			return resultWithUsage(), fmt.Errorf("upstream response failed: %s", failedMessage)
		}
		if !clientDisconnected {
			hadBufferedData := bufferedWriter.Buffered() > 0
			if err := flushBuffered(); err != nil {
				clientDisconnected = true
				logger.LegacyPrintf("service.openai_gateway", "Client disconnected during final flush, returning collected usage")
			} else if hadBufferedData {
				clientOutputStarted = true
				lastDownstreamWriteAt = time.Now()
			}
		}
		return resultWithUsage(), nil
	}
	handleScanErr := func(scanErr error) (*openaiStreamingResult, error, bool) {
		if scanErr == nil {
			return nil, nil, false
		}
		if sawTerminalEvent && !sawFailedEvent {
			logger.LegacyPrintf("service.openai_gateway", "Upstream scan ended after terminal event: %v", scanErr)
			return resultWithUsage(), nil, true
		}
		if sawFailedEvent {
			return resultWithUsage(), fmt.Errorf("upstream response failed: %s", failedMessage), true
		}
		// 客户端断开/取消请求时，上游读取往往会返回 context canceled。
		// /v1/responses 的 SSE 事件必须符合 OpenAI 协议；这里不注入自定义 error event，避免下游 SDK 解析失败。
		if errors.Is(scanErr, context.Canceled) || errors.Is(scanErr, context.DeadlineExceeded) {
			return resultWithUsage(), fmt.Errorf("stream usage incomplete: %w", scanErr), true
		}
		// 客户端已断开时，上游出错仅影响体验，不影响计费；返回已收集 usage
		if clientDisconnected {
			return resultWithUsage(), fmt.Errorf("stream usage incomplete after disconnect: %w", scanErr), true
		}
		if errors.Is(scanErr, bufio.ErrTooLong) {
			logger.LegacyPrintf("service.openai_gateway", "SSE line too long: account=%d max_size=%d error=%v", account.ID, maxLineSize, scanErr)
			sendErrorEvent("response_too_large")
			return resultWithUsage(), scanErr, true
		}
		if !openAIStreamClientOutputStarted(c, clientOutputStarted) {
			msg := "OpenAI stream disconnected before completion"
			if errText := strings.TrimSpace(scanErr.Error()); errText != "" {
				msg += ": " + errText
			}
			return resultWithUsage(), s.newOpenAIStreamFailoverError(c, account, false, upstreamRequestID, nil, msg), true
		}
		sendErrorEvent("stream_read_error")
		return resultWithUsage(), fmt.Errorf("stream read error: %w", scanErr), true
	}
	processSSELine := func(line string, queueDrained bool) {
		if streamFailoverErr != nil {
			return
		}

		// Extract data from SSE line (supports both "data: " and "data:" formats)
		if data, ok := extractOpenAISSEDataLine(line); ok {

			// Replace model in response if needed.
			// Fast path: most events do not contain model field values.
			if needModelReplace && mappedModel != "" && strings.Contains(data, mappedModel) {
				line = s.replaceModelInSSELine(line, mappedModel, originalModel)
			}

			dataBytes := []byte(data)
			if normalizedPayload, normalized := normalizeCompletedOpenAIResponseImageStatuses(dataBytes); normalized > 0 {
				dataBytes = normalizedPayload
				data = string(normalizedPayload)
				line = "data: " + data
				logger.FromContext(ctx).Warn(
					"openai.responses_normalized_stale_image_status",
					zap.Int64("account_id", account.ID),
					zap.Int("image_call_count", normalized),
				)
			}
			if completedImages := countCompletedOpenAIResponseImages(dataBytes); completedImages > imageCount {
				imageCount = completedImages
			}
			if responseID == "" {
				responseID = strings.TrimSpace(gjson.GetBytes(dataBytes, "response.id").String())
				if responseID == "" {
					responseID = strings.TrimSpace(gjson.GetBytes(dataBytes, "id").String())
				}
			}
			if openAIStreamEventIsTerminal(data) {
				sawTerminalEvent = true
			}
			eventType := strings.TrimSpace(gjson.GetBytes(dataBytes, "type").String())
			usageDataBytes := dataBytes
			forceFlushFailedEvent := false
			switch eventType {
			case "response.failed":
				failedMessage = extractOpenAISSEErrorMessage(dataBytes)
				if !openAIStreamClientOutputStarted(c, clientOutputStarted) && openAIStreamFailedEventShouldFailover(dataBytes, failedMessage) {
					sawFailedEvent = true
					streamFailoverErr = s.newOpenAIStreamFailoverError(c, account, false, upstreamRequestID, dataBytes, failedMessage)
					return
				}
				forceFlushFailedEvent = true
				sawFailedEvent = true
				safeClientErr := SafeOpenAIClientUpstreamError(http.StatusBadGateway, failedMessage, string(dataBytes))
				clientCode := safeClientErr.Code
				if clientCode == "" {
					clientCode = "server_error"
				}
				responseID := strings.TrimSpace(gjson.GetBytes(dataBytes, "response.id").String())
				model := originalModel
				if model == "" {
					model = strings.TrimSpace(gjson.GetBytes(dataBytes, "response.model").String())
				}
				safePayload, _ := json.Marshal(OpenAIResponsesFailedEnvelope(c, responseID, model, clientCode, safeClientErr.Message))
				dataBytes = safePayload
				data = string(safePayload)
				line = "data: " + data
			case "error":
				if safePayload, changed := sanitizeOpenAICapacityShedErrorCodeForClient(dataBytes); changed {
					dataBytes = safePayload
					data = string(safePayload)
					line = "data: " + data
				}
			}

			// Correct Codex tool calls if needed (apply_patch -> edit, etc.)
			if correctedData, corrected := s.toolCorrector.CorrectToolCallsInSSEBytes(dataBytes); corrected {
				dataBytes = correctedData
				data = string(correctedData)
				line = "data: " + data
				eventType = strings.TrimSpace(gjson.GetBytes(dataBytes, "type").String())
			}
			startsClientOutput := forceFlushFailedEvent || openAIStreamDataStartsClientOutput(data, eventType)

			// 写入客户端（客户端断开后继续 drain 上游）
			if !clientDisconnected {
				shouldFlush := queueDrained && (clientOutputStarted || startsClientOutput)
				if firstTokenMs == nil && startsClientOutput {
					// 保证首个 token 事件尽快出站，避免影响 TTFT。
					shouldFlush = true
				}
				if _, err := bufferedWriter.WriteString(line); err != nil {
					clientDisconnected = true
					logger.LegacyPrintf("service.openai_gateway", "Client disconnected during streaming, continuing to drain upstream for billing")
				} else if _, err := bufferedWriter.WriteString("\n"); err != nil {
					clientDisconnected = true
					logger.LegacyPrintf("service.openai_gateway", "Client disconnected during streaming, continuing to drain upstream for billing")
				} else if shouldFlush {
					if err := flushBuffered(); err != nil {
						clientDisconnected = true
						logger.LegacyPrintf("service.openai_gateway", "Client disconnected during streaming flush, continuing to drain upstream for billing")
					} else {
						clientOutputStarted = true
						lastDownstreamWriteAt = time.Now()
					}
				}
			}

			// Record first token time
			if firstTokenMs == nil && startsClientOutput {
				ms := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &ms
			}
			s.parseSSEUsageBytes(usageDataBytes, usage)
			return
		}

		// Forward non-data lines as-is
		if !clientDisconnected {
			if _, err := bufferedWriter.WriteString(line); err != nil {
				clientDisconnected = true
				logger.LegacyPrintf("service.openai_gateway", "Client disconnected during streaming, continuing to drain upstream for billing")
			} else if _, err := bufferedWriter.WriteString("\n"); err != nil {
				clientDisconnected = true
				logger.LegacyPrintf("service.openai_gateway", "Client disconnected during streaming, continuing to drain upstream for billing")
			} else if queueDrained && clientOutputStarted {
				if err := flushBuffered(); err != nil {
					clientDisconnected = true
					logger.LegacyPrintf("service.openai_gateway", "Client disconnected during streaming flush, continuing to drain upstream for billing")
				} else {
					lastDownstreamWriteAt = time.Now()
				}
			}
		}
	}

	// 无超时/无 keepalive 的常见路径走同步扫描，减少 goroutine 与 channel 开销。
	if streamInterval <= 0 && keepaliveInterval <= 0 {
		defer putSSEScannerBuf64K(scanBuf)
		for scanner.Scan() {
			processSSELine(scanner.Text(), true)
			if streamFailoverErr != nil {
				return resultWithUsage(), streamFailoverErr
			}
		}
		if result, err, done := handleScanErr(scanner.Err()); done {
			return result, err
		}
		return finalizeStream()
	}

	type scanEvent struct {
		line string
		err  error
	}
	// 独立 goroutine 读取上游，避免读取阻塞影响 keepalive/超时处理
	events := make(chan scanEvent, 16)
	done := make(chan struct{})
	sendEvent := func(ev scanEvent) bool {
		select {
		case events <- ev:
			return true
		case <-done:
			return false
		}
	}
	var lastReadAt int64
	atomic.StoreInt64(&lastReadAt, time.Now().UnixNano())
	go func(scanBuf *sseScannerBuf64K) {
		defer putSSEScannerBuf64K(scanBuf)
		defer close(events)
		for scanner.Scan() {
			atomic.StoreInt64(&lastReadAt, time.Now().UnixNano())
			if !sendEvent(scanEvent{line: scanner.Text()}) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = sendEvent(scanEvent{err: err})
		}
	}(scanBuf)
	defer close(done)

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return finalizeStream()
			}
			if result, err, done := handleScanErr(ev.err); done {
				return result, err
			}
			processSSELine(ev.line, len(events) == 0)
			if streamFailoverErr != nil {
				return resultWithUsage(), streamFailoverErr
			}

		case <-intervalCh:
			lastRead := time.Unix(0, atomic.LoadInt64(&lastReadAt))
			if time.Since(lastRead) < streamInterval {
				continue
			}
			if clientDisconnected {
				return resultWithUsage(), fmt.Errorf("stream usage incomplete after timeout")
			}
			logger.LegacyPrintf("service.openai_gateway", "Stream data interval timeout: account=%d model=%s interval=%s", account.ID, originalModel, streamInterval)
			// 处理流超时，可能标记账户为临时不可调度或错误状态
			if s.rateLimitService != nil {
				s.rateLimitService.HandleStreamTimeout(ctx, account, originalModel)
			}
			sendErrorEvent("stream_timeout")
			return resultWithUsage(), fmt.Errorf("stream data interval timeout")

		case <-keepaliveCh:
			if clientDisconnected {
				continue
			}
			if time.Since(lastDownstreamWriteAt) < keepaliveInterval {
				continue
			}
			if _, err := bufferedWriter.WriteString(":\n\n"); err != nil {
				clientDisconnected = true
				logger.LegacyPrintf("service.openai_gateway", "Client disconnected during streaming, continuing to drain upstream for billing")
				continue
			}
			if err := flushBuffered(); err != nil {
				clientDisconnected = true
				logger.LegacyPrintf("service.openai_gateway", "Client disconnected during keepalive flush, continuing to drain upstream for billing")
			} else {
				lastDownstreamWriteAt = time.Now()
			}
		}
	}

}

// extractOpenAISSEDataLine 低开销提取 SSE `data:` 行内容。
// 兼容 `data: xxx` 与 `data:xxx` 两种格式。
func extractOpenAISSEDataLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	start := len("data:")
	for start < len(line) {
		if line[start] != ' ' && line[start] != '	' {
			break
		}
		start++
	}
	return line[start:], true
}

func extractOpenAISSEEventLine(line string) (string, bool) {
	if !strings.HasPrefix(line, "event:") {
		return "", false
	}
	start := len("event:")
	for start < len(line) {
		if line[start] != ' ' && line[start] != '	' {
			break
		}
		start++
	}
	return strings.TrimSpace(line[start:]), true
}

type openAICompatSSEFrame struct {
	EventType string
	Data      string
}

type openAICompatSSEFrameParser struct {
	eventType string
	dataLines []string
}

func (p *openAICompatSSEFrameParser) AddLine(line string) (openAICompatSSEFrame, bool) {
	if line == "" {
		return p.dispatch()
	}
	if strings.HasPrefix(line, ":") {
		return openAICompatSSEFrame{}, false
	}
	if eventType, ok := extractOpenAISSEEventLine(line); ok {
		p.eventType = eventType
		return openAICompatSSEFrame{}, false
	}
	if data, ok := extractOpenAISSEDataLine(line); ok {
		p.dataLines = append(p.dataLines, data)
	}
	return openAICompatSSEFrame{}, false
}

func (p *openAICompatSSEFrameParser) Finish() (openAICompatSSEFrame, bool) {
	return p.dispatch()
}

func (p *openAICompatSSEFrameParser) dispatch() (openAICompatSSEFrame, bool) {
	frame := openAICompatSSEFrame{
		EventType: p.eventType,
		Data:      strings.Join(p.dataLines, "\n"),
	}
	p.eventType = ""
	p.dataLines = nil
	return frame, frame.Data != ""
}

func scanOpenAICompatSSEFrames(body string, visit func(openAICompatSSEFrame) bool) {
	var parser openAICompatSSEFrameParser
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if parser.dataLines != nil && (strings.HasPrefix(line, "data:") || strings.HasPrefix(line, "event:")) {
			if frame, ok := parser.dispatch(); ok {
				if visit(frame) {
					return
				}
			}
		}
		frame, ok := parser.AddLine(line)
		if !ok {
			continue
		}
		if visit(frame) {
			return
		}
	}
	if frame, ok := parser.Finish(); ok {
		visit(frame)
	}
}

func openAICompatPayloadWithEventType(payload, eventType string) string {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" || strings.TrimSpace(payload) == "" || strings.TrimSpace(payload) == "[DONE]" {
		return payload
	}
	if gjson.Get(payload, "type").Exists() {
		return payload
	}
	patched, err := sjson.Set(payload, "type", eventType)
	if err != nil {
		return payload
	}
	return patched
}

func isOpenAICompatResponsesTerminalEvent(eventType string) bool {
	switch eventType {
	case "response.completed", "response.done", "response.incomplete", "response.failed":
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) replaceModelInSSELine(line, fromModel, toModel string) string {
	data, ok := extractOpenAISSEDataLine(line)
	if !ok {
		return line
	}
	if data == "" || data == "[DONE]" {
		return line
	}

	// 使用 gjson 精确检查 model 字段，避免全量 JSON 反序列化
	if m := gjson.Get(data, "model"); m.Exists() && m.Str == fromModel {
		newData, err := sjson.Set(data, "model", toModel)
		if err != nil {
			return line
		}
		return "data: " + newData
	}

	// 检查嵌套的 response.model 字段
	if m := gjson.Get(data, "response.model"); m.Exists() && m.Str == fromModel {
		newData, err := sjson.Set(data, "response.model", toModel)
		if err != nil {
			return line
		}
		return "data: " + newData
	}

	return line
}

// correctToolCallsInResponseBody 修正响应体中的工具调用
func (s *OpenAIGatewayService) correctToolCallsInResponseBody(body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	corrected, changed := s.toolCorrector.CorrectToolCallsInSSEBytes(body)
	if changed {
		return corrected
	}
	return body
}

func (s *OpenAIGatewayService) parseSSEUsage(data string, usage *OpenAIUsage) {
	s.parseSSEUsageBytes([]byte(data), usage)
}

func (s *OpenAIGatewayService) parseSSEUsageBytes(data []byte, usage *OpenAIUsage) {
	if usage == nil || len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
		return
	}
	// 选择性解析：仅在数据中包含终止事件标识时才进入字段提取。
	if len(data) < 72 {
		return
	}
	eventType := gjson.GetBytes(data, "type").String()
	if eventType != "response.completed" && eventType != "response.done" && eventType != "response.failed" &&
		eventType != "response.incomplete" && eventType != "response.cancelled" && eventType != "response.canceled" {
		return
	}

	if parsedUsage, ok := extractOpenAIUsageFromJSONBytes(data); ok {
		*usage = parsedUsage
	}
}

func extractOpenAIUsageFromJSONBytes(body []byte) (OpenAIUsage, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return OpenAIUsage{}, false
	}
	// Preserve native Responses precedence, then accept compatible gateways that
	// wrap the response in data. This avoids recording a successful request as
	// zero usage while ensuring a wrapper cannot shadow a native usage object.
	candidates := []struct {
		usagePath      string
		imageUsagePath string
	}{
		{usagePath: "usage", imageUsagePath: "tool_usage.image_gen"},
		{usagePath: "response.usage", imageUsagePath: "response.tool_usage.image_gen"},
		{usagePath: "data.usage", imageUsagePath: "data.tool_usage.image_gen"},
		{usagePath: "data.response.usage", imageUsagePath: "data.response.tool_usage.image_gen"},
	}
	for _, candidate := range candidates {
		if usage, ok := openAIUsageFromGJSON(gjson.GetBytes(body, candidate.usagePath)); ok {
			mergeHostedImageGenToolUsage(gjson.GetBytes(body, candidate.imageUsagePath), &usage)
			return usage, true
		}
	}
	return OpenAIUsage{}, false
}

func mergeHostedImageGenToolUsage(imageGen gjson.Result, usage *OpenAIUsage) {
	if usage == nil || !imageGen.Exists() || !imageGen.IsObject() || usage.ImageOutputTokens > 0 {
		return
	}
	if imageTokens := imageGen.Get("output_tokens_details.image_tokens").Int(); imageTokens > 0 {
		usage.ImageOutputTokens = int(imageTokens)
	}
}

func openAIUsageFromGJSON(value gjson.Result) (OpenAIUsage, bool) {
	if !value.Exists() || !value.IsObject() {
		return OpenAIUsage{}, false
	}
	inputTokens := value.Get("input_tokens").Int()
	if inputTokens == 0 {
		inputTokens = value.Get("prompt_tokens").Int()
	}
	outputTokens := value.Get("output_tokens").Int()
	if outputTokens == 0 {
		outputTokens = value.Get("completion_tokens").Int()
	}
	cacheReadTokens := value.Get("input_tokens_details.cached_tokens").Int()
	if cacheReadTokens == 0 {
		cacheReadTokens = value.Get("prompt_tokens_details.cached_tokens").Int()
	}
	imageOutputTokens := value.Get("output_tokens_details.image_tokens").Int()
	if imageOutputTokens == 0 {
		imageOutputTokens = value.Get("completion_tokens_details.image_tokens").Int()
	}
	return OpenAIUsage{
		InputTokens:              int(inputTokens),
		OutputTokens:             int(outputTokens),
		CacheCreationInputTokens: int(value.Get("cache_creation_input_tokens").Int()),
		CacheReadInputTokens:     int(cacheReadTokens),
		ImageOutputTokens:        int(imageOutputTokens),
	}, true
}

func (s *OpenAIGatewayService) handleNonStreamingResponse(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, originalModel, mappedModel string) (*openaiNonStreamingResult, error) {
	body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}

	// Detect SSE responses for ALL account types via Content-Type header.
	// Some OpenAI-compatible upstreams (including other sub2api instances)
	// may return SSE even when stream=false was requested.
	if isEventStreamResponse(resp.Header) {
		return s.handleSSEToJSON(resp, c, body, originalModel, mappedModel)
	}
	bodyLooksLikeSSE := bytes.Contains(body, []byte("data:")) || bytes.Contains(body, []byte("event:"))

	// For OAuth accounts, also fall back to a body-content heuristic because
	// the upstream may omit the Content-Type header while still sending SSE.
	// This heuristic is NOT applied to API-key accounts to avoid false
	// positives on JSON responses that coincidentally contain "data:" or
	// "event:" in their text content.
	if account.Type == AccountTypeOAuth && bodyLooksLikeSSE {
		return s.handleSSEToJSON(resp, c, body, originalModel, mappedModel)
	}
	if account != nil && account.IsGrok() && isOpenAIResponsesCompactPath(c) {
		body, err = convertGrokResponseToOpenAICompact(body)
		if err != nil {
			return nil, fmt.Errorf("convert Grok compact response: %w", err)
		}
	}
	if account != nil && account.IsGrok() {
		if patched, changed := ensureGrokResponsesCreatedAt(body, time.Now().Unix()); changed {
			body = patched
		}
		if patched, changed := ensureGrokResponsesStrictFields(body); changed {
			body = patched
		}
	}
	if normalizedBody, normalized := normalizeCompletedOpenAIResponseImageStatuses(body); normalized > 0 {
		body = normalizedBody
		logger.FromContext(ctx).Warn(
			"openai.responses_normalized_stale_image_status",
			zap.Int64("account_id", account.ID),
			zap.Int("image_call_count", normalized),
		)
	}
	imageCount := countCompletedOpenAIResponseImages(body)

	usageValue, usageOK := extractOpenAIUsageFromJSONBytes(body)
	if !usageOK {
		if bodyLooksLikeSSE {
			return s.handleSSEToJSON(resp, c, body, originalModel, mappedModel)
		}
		return nil, fmt.Errorf("parse response: invalid json response")
	}
	usage := &usageValue

	// Replace model in response if needed
	if originalModel != mappedModel {
		body = s.replaceModelInResponseBody(body, mappedModel, originalModel)
	}
	// Restore client-only custom/tool-search/namespace calls that were lowered
	// to xAI function calls before the request was sent.
	body = s.correctToolCallsInResponseBody(body)
	if restoredBody, restoreErr := restoreGrokResponsesClientToolPayload(c, body); restoreErr != nil {
		return nil, fmt.Errorf("restore Grok Responses client tool response: %w", restoreErr)
	} else {
		body = restoredBody
	}

	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)

	contentType := "application/json"
	if s.cfg != nil && !s.cfg.Security.ResponseHeaders.Enabled {
		if upstreamType := resp.Header.Get("Content-Type"); upstreamType != "" {
			contentType = upstreamType
		}
	}

	c.Data(resp.StatusCode, contentType, body)

	return &openaiNonStreamingResult{
		OpenAIUsage: usage,
		usage:       usage,
		responseID:  extractOpenAIResponseIDFromJSONBytes(body),
		imageCount:  imageCount,
	}, nil
}

func isEventStreamResponse(header http.Header) bool {
	contentType := strings.ToLower(header.Get("Content-Type"))
	return strings.Contains(contentType, "text/event-stream")
}

func (s *OpenAIGatewayService) handleSSEToJSON(resp *http.Response, c *gin.Context, body []byte, originalModel, mappedModel string) (*openaiNonStreamingResult, error) {
	bodyText := string(body)
	finalResponse, ok := extractCodexFinalResponse(bodyText)

	usage := &OpenAIUsage{}
	if ok {
		if parsedUsage, parsed := extractOpenAIUsageFromJSONBytes(finalResponse); parsed {
			*usage = parsedUsage
		}
		// When the terminal event has an empty output array, reconstruct
		// output from accumulated delta events so the client gets full content.
		// gjson Array() returns empty slice for null, missing, or empty arrays.
		if len(gjson.GetBytes(finalResponse, "output").Array()) == 0 {
			if outputJSON, reconstructed := reconstructResponseOutputFromSSE(bodyText); reconstructed {
				if patched, err := sjson.SetRawBytes(finalResponse, "output", outputJSON); err == nil {
					finalResponse = patched
				}
			}
		}
		if normalizedResponse, normalized := normalizeCompletedOpenAIResponseImageStatuses(finalResponse); normalized > 0 {
			finalResponse = normalizedResponse
		}
		body = finalResponse
		if originalModel != mappedModel {
			body = s.replaceModelInResponseBody(body, mappedModel, originalModel)
		}
		// Correct tool calls in final response
		body = s.correctToolCallsInResponseBody(body)
		if restoredBody, restoreErr := restoreGrokResponsesClientToolPayload(c, body); restoreErr != nil {
			return nil, fmt.Errorf("restore Grok Responses client tool response: %w", restoreErr)
		} else {
			body = restoredBody
		}
	} else {
		terminalType, terminalPayload, terminalOK := extractOpenAISSETerminalEvent(bodyText)
		if terminalOK && terminalType == "response.failed" {
			msg := extractOpenAISSEErrorMessage(terminalPayload)
			if msg == "" {
				msg = "Upstream compact response failed"
			}
			return nil, s.writeOpenAINonStreamingProtocolError(resp, c, msg)
		}
		usage = s.parseSSEUsageFromBody(bodyText)
		if originalModel != mappedModel {
			bodyText = s.replaceModelInSSEBody(bodyText, mappedModel, originalModel)
		}
		body = []byte(bodyText)
	}

	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)

	contentType := "application/json; charset=utf-8"
	if !ok {
		contentType = resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "text/event-stream"
		}
	}
	c.Data(resp.StatusCode, contentType, body)

	return &openaiNonStreamingResult{
		OpenAIUsage: usage,
		usage:       usage,
		responseID:  extractOpenAIResponseIDFromJSONBytes(body),
		imageCount:  countCompletedOpenAIResponseImages(body),
	}, nil
}

func (s *OpenAIGatewayService) handleOAuthSSEToJSON(resp *http.Response, c *gin.Context, body []byte, originalModel, mappedModel string) (*openaiNonStreamingResult, error) {
	return s.handleSSEToJSON(resp, c, body, originalModel, mappedModel)
}

func extractOpenAIResponseIDFromJSONBytes(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	if id := strings.TrimSpace(gjson.GetBytes(body, "id").String()); id != "" {
		return id
	}
	return strings.TrimSpace(gjson.GetBytes(body, "response.id").String())
}

func extractOpenAISSETerminalEvent(body string) (string, []byte, bool) {
	var terminalType string
	var terminalPayload []byte
	scanOpenAICompatSSEFrames(body, func(frame openAICompatSSEFrame) bool {
		data := openAICompatPayloadWithEventType(frame.Data, frame.EventType)
		if strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
			return false
		}
		eventType := strings.TrimSpace(gjson.Get(data, "type").String())
		switch eventType {
		case "response.completed", "response.done", "response.failed":
			terminalType = eventType
			terminalPayload = []byte(data)
			return true
		}
		return false
	})
	if terminalPayload != nil {
		return terminalType, terminalPayload, true
	}
	return "", nil, false
}

func extractOpenAISSEErrorMessage(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	for _, path := range []string{"response.error.message", "error.message", "message"} {
		if msg := strings.TrimSpace(gjson.GetBytes(payload, path).String()); msg != "" {
			return sanitizeUpstreamErrorMessage(msg)
		}
	}
	return sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(payload)))
}

func (s *OpenAIGatewayService) writeOpenAINonStreamingProtocolError(resp *http.Response, c *gin.Context, message string) error {
	message = sanitizeUpstreamErrorMessage(strings.TrimSpace(message))
	if message == "" {
		message = "Upstream returned an invalid non-streaming response"
	}
	setOpsUpstreamError(c, http.StatusBadGateway, message, "")
	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	safeErr := SafeOpenAIClientUpstreamError(http.StatusBadGateway, message)
	c.JSON(safeErr.StatusCode, OpenAIClientUpstreamErrorEnvelope(c, safeErr))
	return fmt.Errorf("non-streaming openai protocol error: %s", message)
}

func extractCodexFinalResponse(body string) ([]byte, bool) {
	var finalResponse []byte
	scanOpenAICompatSSEFrames(body, func(frame openAICompatSSEFrame) bool {
		data := openAICompatPayloadWithEventType(frame.Data, frame.EventType)
		if strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
			return false
		}
		eventType := gjson.Get(data, "type").String()
		if eventType == "response.done" || eventType == "response.completed" {
			if response := gjson.Get(data, "response"); response.Exists() && response.Type == gjson.JSON && response.Raw != "" {
				finalResponse = []byte(response.Raw)
				return true
			}
		}
		return false
	})
	if finalResponse != nil {
		return finalResponse, true
	}
	return nil, false
}

// reconstructResponseOutputFromSSE scans raw SSE body text for delta events and
// returns a JSON-encoded output array reconstructed from accumulated deltas.
// Returns (nil, false) if no content was found in deltas.
func reconstructResponseOutputFromSSE(bodyText string) ([]byte, bool) {
	acc := apicompat.NewBufferedResponseAccumulator()
	scanOpenAICompatSSEFrames(bodyText, func(frame openAICompatSSEFrame) bool {
		data := openAICompatPayloadWithEventType(frame.Data, frame.EventType)
		if strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
			return false
		}
		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return false
		}
		acc.ProcessEvent(&event)
		return false
	})
	if !acc.HasContent() {
		return nil, false
	}
	output := acc.BuildOutput()
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return nil, false
	}
	return outputJSON, true
}

func (s *OpenAIGatewayService) parseSSEUsageFromBody(body string) *OpenAIUsage {
	usage := &OpenAIUsage{}
	scanOpenAICompatSSEFrames(body, func(frame openAICompatSSEFrame) bool {
		data := openAICompatPayloadWithEventType(frame.Data, frame.EventType)
		if strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
			return false
		}
		s.parseSSEUsageBytes([]byte(data), usage)
		return false
	})
	return usage
}

func (s *OpenAIGatewayService) replaceModelInSSEBody(body, fromModel, toModel string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if _, ok := extractOpenAISSEDataLine(line); !ok {
			continue
		}
		lines[i] = s.replaceModelInSSELine(line, fromModel, toModel)
	}
	return strings.Join(lines, "\n")
}

func (s *OpenAIGatewayService) validateUpstreamBaseURL(raw string) (string, error) {
	if s.cfg != nil && !s.cfg.Security.URLAllowlist.Enabled {
		normalized, err := urlvalidator.ValidateURLFormat(raw, s.cfg.Security.URLAllowlist.AllowInsecureHTTP)
		if err != nil {
			return "", fmt.Errorf("invalid base_url: %w", err)
		}
		return normalized, nil
	}
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
		AllowedHosts:     s.cfg.Security.URLAllowlist.UpstreamHosts,
		RequireAllowlist: true,
		AllowPrivate:     s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	return normalized, nil
}

// buildOpenAIResponsesURL 组装 OpenAI Responses 端点。
// - base 以 /v1 结尾：追加 /responses
// - base 已是 /responses：原样返回
// - 其他情况：追加 /v1/responses
func buildOpenAIResponsesURL(base string) string {
	normalized := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(normalized, "/chat/completions") {
		return strings.TrimSuffix(normalized, "/chat/completions") + "/responses"
	}
	return buildOpenAIEndpointURL(base, "/v1/responses")
}

func trimOpenAIEncryptedReasoningItems(reqBody map[string]any) bool {
	if len(reqBody) == 0 {
		return false
	}

	inputValue, has := reqBody["input"]
	if !has {
		return false
	}

	switch input := inputValue.(type) {
	case []any:
		filtered := input[:0]
		changed := false
		for _, item := range input {
			nextItem, itemChanged, keep := sanitizeEncryptedReasoningInputItem(item)
			if itemChanged {
				changed = true
			}
			if !keep {
				continue
			}
			filtered = append(filtered, nextItem)
		}
		if !changed {
			return false
		}
		if len(filtered) == 0 {
			delete(reqBody, "input")
			return true
		}
		reqBody["input"] = filtered
		return true
	case []map[string]any:
		filtered := input[:0]
		changed := false
		for _, item := range input {
			nextItem, itemChanged, keep := sanitizeEncryptedReasoningInputItem(item)
			if itemChanged {
				changed = true
			}
			if !keep {
				continue
			}
			nextMap, ok := nextItem.(map[string]any)
			if !ok {
				filtered = append(filtered, item)
				continue
			}
			filtered = append(filtered, nextMap)
		}
		if !changed {
			return false
		}
		if len(filtered) == 0 {
			delete(reqBody, "input")
			return true
		}
		reqBody["input"] = filtered
		return true
	case map[string]any:
		nextItem, changed, keep := sanitizeEncryptedReasoningInputItem(input)
		if !changed {
			return false
		}
		if !keep {
			delete(reqBody, "input")
			return true
		}
		nextMap, ok := nextItem.(map[string]any)
		if !ok {
			return false
		}
		reqBody["input"] = nextMap
		return true
	default:
		return false
	}
}

func sanitizeEncryptedReasoningInputItem(item any) (next any, changed bool, keep bool) {
	inputItem, ok := item.(map[string]any)
	if !ok {
		return item, false, true
	}

	itemType, _ := inputItem["type"].(string)
	if strings.TrimSpace(itemType) != "reasoning" {
		return item, false, true
	}

	_, hasEncryptedContent := inputItem["encrypted_content"]
	if !hasEncryptedContent {
		return item, false, true
	}

	delete(inputItem, "encrypted_content")
	if len(inputItem) == 1 {
		return nil, true, false
	}
	return inputItem, true, true
}

func IsOpenAIResponsesCompactPathForTest(c *gin.Context) bool {
	return isOpenAIResponsesCompactPath(c)
}

func OpenAICompactSessionSeedKeyForTest() string {
	return openAICompactSessionSeedKey
}

func NormalizeOpenAICompactRequestBodyForTest(body []byte) ([]byte, bool, error) {
	return normalizeOpenAICompactRequestBody(body)
}

func isOpenAIResponsesCompactPath(c *gin.Context) bool {
	suffix := strings.TrimSpace(openAIResponsesRequestPathSuffix(c))
	return suffix == "/compact" || strings.HasPrefix(suffix, "/compact/")
}

func normalizeOpenAICompactRequestBody(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	normalized := []byte(`{}`)
	for _, field := range []string{
		"model",
		"input",
		"instructions",
		"tools",
		"parallel_tool_calls",
		"reasoning",
		"text",
		"previous_response_id",
	} {
		value := gjson.GetBytes(body, field)
		if !value.Exists() {
			continue
		}
		next, err := sjson.SetRawBytes(normalized, field, []byte(value.Raw))
		if err != nil {
			return body, false, fmt.Errorf("normalize compact body %s: %w", field, err)
		}
		normalized = next
	}

	if bytes.Equal(bytes.TrimSpace(body), bytes.TrimSpace(normalized)) {
		return body, false, nil
	}
	return normalized, true, nil
}

func resolveOpenAICompactSessionID(c *gin.Context) string {
	if c != nil {
		if sessionID := strings.TrimSpace(c.GetHeader("session-id")); sessionID != "" {
			return sessionID
		}
		if sessionID := strings.TrimSpace(c.GetHeader("session_id")); sessionID != "" {
			return sessionID
		}
		if conversationID := strings.TrimSpace(c.GetHeader("conversation_id")); conversationID != "" {
			return conversationID
		}
		if seed, ok := c.Get(openAICompactSessionSeedKey); ok {
			if seedStr, ok := seed.(string); ok && strings.TrimSpace(seedStr) != "" {
				return strings.TrimSpace(seedStr)
			}
		}
	}
	return uuid.NewString()
}

// openAIResponsesRequestPathSuffix returns only subpaths that are safe to append
// to the upstream /responses URL. The route guard rejects unsafe requests; this
// check remains as defense in depth for future call sites.
func openAIResponsesRequestPathSuffix(c *gin.Context) string {
	suffix, ok := sanitizedUpstreamPathSuffix(rawOpenAIResponsesRequestPathSuffix(c))
	if !ok {
		return ""
	}
	return suffix
}

// IsForwardableOpenAIResponsesRequestPath reports whether the inbound
// /responses subpath can be forwarded without changing the upstream URL shape.
func IsForwardableOpenAIResponsesRequestPath(c *gin.Context) bool {
	_, ok := sanitizedUpstreamPathSuffix(rawOpenAIResponsesRequestPathSuffix(c))
	return ok
}

// IsOpenAIResponsesInputTokensRequestPath identifies the native token-count
// preflight before the generic Responses wildcard forwards a subpath upstream.
func IsOpenAIResponsesInputTokensRequestPath(c *gin.Context) bool {
	return openAIResponsesRequestPathSuffix(c) == "/input_tokens"
}

// rawOpenAIResponsesRequestPathSuffix extracts the suffix without validating it.
func rawOpenAIResponsesRequestPathSuffix(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	normalizedPath := strings.TrimRight(strings.TrimSpace(c.Request.URL.Path), "/")
	if normalizedPath == "" {
		return ""
	}
	idx := strings.LastIndex(normalizedPath, "/responses")
	if idx < 0 {
		return ""
	}
	suffix := normalizedPath[idx+len("/responses"):]
	if suffix == "" || suffix == "/" {
		return ""
	}
	if !strings.HasPrefix(suffix, "/") {
		return ""
	}
	return suffix
}

func appendOpenAIResponsesRequestPathSuffix(baseURL, suffix string) string {
	trimmedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	trimmedSuffix, ok := sanitizedUpstreamPathSuffix(suffix)
	if !ok || trimmedBase == "" || trimmedSuffix == "" {
		return trimmedBase
	}
	return trimmedBase + trimmedSuffix
}

func (s *OpenAIGatewayService) replaceModelInResponseBody(body []byte, fromModel, toModel string) []byte {
	// 使用 gjson/sjson 精确替换 model 字段，避免全量 JSON 反序列化
	if m := gjson.GetBytes(body, "model"); m.Exists() && m.Str == fromModel {
		newBody, err := sjson.SetBytes(body, "model", toModel)
		if err != nil {
			return body
		}
		return newBody
	}
	return body
}

// OpenAIRecordUsageInput input for recording usage
type OpenAIRecordUsageInput struct {
	Result           *OpenAIForwardResult
	APIKey           *APIKey
	User             *User
	Account          *Account
	Subscription     *UserSubscription
	InboundEndpoint  string
	UpstreamEndpoint string
	// RouteObservationModel is the exact model identity used during account
	// scheduling. It can differ from OriginalModel for compatibility dispatch
	// (for example Claude Messages mapped to an OpenAI model) and keeps settled
	// cost samples in the same route bucket as attempts and Shadow candidates.
	RouteObservationModel string
	UserAgent             string // 请求的 User-Agent
	IPAddress             string // 请求的客户端 IP 地址
	RequestPayloadHash    string
	APIKeyService         APIKeyQuotaUpdater
	ChannelUsageFields
}

// RecordUsage records usage and deducts balance
func (s *OpenAIGatewayService) RecordUsage(ctx context.Context, input *OpenAIRecordUsageInput) error {
	if input == nil {
		return nil
	}
	result := input.Result
	if s.rateLimitService != nil && input.Account != nil && input.Account.Platform == PlatformOpenAI {
		s.rateLimitService.ResetOpenAI403Counter(ctx, input.Account.ID)
	}

	// 跳过所有 token 均为零的用量记录——上游未返回 usage 时不应写入数据库
	if result.Usage.InputTokens == 0 && result.Usage.OutputTokens == 0 &&
		result.Usage.CacheCreationInputTokens == 0 && result.Usage.CacheReadInputTokens == 0 &&
		result.ImageCount == 0 && result.VideoCount == 0 {
		return nil
	}

	apiKey := input.APIKey
	user := input.User
	account := input.Account
	subscription := input.Subscription

	// 计算实际的新输入token（减去缓存读取的token）
	// 因为 input_tokens 包含了 cache_read_tokens，而缓存读取的token不应按输入价格计费
	actualInputTokens := result.Usage.InputTokens - result.Usage.CacheReadInputTokens
	if actualInputTokens < 0 {
		actualInputTokens = 0
	}

	// Calculate cost
	tokens := UsageTokens{
		InputTokens:         actualInputTokens,
		OutputTokens:        result.Usage.OutputTokens,
		CacheCreationTokens: result.Usage.CacheCreationInputTokens,
		CacheReadTokens:     result.Usage.CacheReadInputTokens,
		ImageOutputTokens:   result.Usage.ImageOutputTokens,
	}

	// Get rate multiplier
	multiplier := 1.0
	if s.cfg != nil {
		multiplier = s.cfg.Default.RateMultiplier
	}
	if apiKey.GroupID != nil && apiKey.Group != nil {
		resolver := s.userGroupRateResolver
		if resolver == nil {
			resolver = newUserGroupRateResolver(nil, nil, resolveUserGroupRateCacheTTL(s.cfg), nil, "service.openai_gateway")
		}
		multiplier = resolver.Resolve(ctx, user.ID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
	}
	imageMultiplier := multiplier
	if result.ImageCount > 0 && result.VideoCount == 0 && apiKey.Group != nil && apiKey.Group.ImageRateIndependent {
		imageMultiplier = apiKey.Group.ImageRateMultiplier
	}

	var cost *CostBreakdown
	var err error
	billingModel := forwardResultBillingModel(result.Model, result.UpstreamModel)
	if result.BillingModel != "" {
		billingModel = strings.TrimSpace(result.BillingModel)
	}
	if input.BillingModelSource == BillingModelSourceChannelMapped && input.ChannelMappedModel != "" && input.ChannelMappedModel != input.OriginalModel {
		billingModel = input.ChannelMappedModel
	}
	if input.BillingModelSource == BillingModelSourceRequested && input.OriginalModel != "" {
		billingModel = input.OriginalModel
	}
	serviceTier := ""
	if result.ServiceTier != nil {
		serviceTier = strings.TrimSpace(*result.ServiceTier)
	}
	requestCount := 1
	sizeTier := ""
	if result.ImageCount > 0 {
		requestCount = result.ImageCount
		sizeTier = strings.TrimSpace(result.ImageSize)
	}
	if result.VideoCount > 0 {
		videoMultiplier := multiplier
		var videoConfig *VideoPriceConfig
		if apiKey.Group != nil {
			videoConfig = &VideoPriceConfig{
				Price480P:  apiKey.Group.VideoPrice480P,
				Price720P:  apiKey.Group.VideoPrice720P,
				Price1080P: apiKey.Group.VideoPrice1080P,
			}
			if apiKey.Group.VideoRateIndependent {
				videoMultiplier = apiKey.Group.VideoRateMultiplier
			}
		}
		cost = s.billingService.CalculateVideoCost(
			billingModel, result.VideoResolution, result.VideoCount,
			result.VideoDurationSeconds, videoConfig, videoMultiplier,
		)
		multiplier = videoMultiplier
	} else if result.ImageCount > 0 && apiKey.Group != nil &&
		apiKey.Group.GPTImageCallPrice != nil && *apiKey.Group.GPTImageCallPrice > 0 {
		// An explicit per-image price is the product contract for both integrated
		// OpenAI groups and the standalone image group. It deliberately takes
		// precedence over token/resolution pricing, but only after the forwarder
		// has validated one or more completed image payloads.
		cost = s.billingService.CalculateGPTImageCallCost(
			apiKey.Group.GPTImageCallPrice,
			result.ImageCount,
			imageMultiplier,
		)
	} else if result.ImageCount > 0 && apiKey.Group != nil && apiKey.Group.Platform == PlatformGPTImage {
		groupConfig := &ImagePriceConfig{
			Price1K: apiKey.Group.ImagePrice1K,
			Price2K: apiKey.Group.ImagePrice2K,
			Price4K: apiKey.Group.ImagePrice4K,
		}
		if groupConfig.hasAnyPrice() {
			cost = s.billingService.CalculateImageCost(billingModel, sizeTier, result.ImageCount, groupConfig, imageMultiplier)
		} else {
			cost = s.billingService.CalculateGPTImageCallCost(apiKey.Group.GPTImageCallPrice, result.ImageCount, imageMultiplier)
		}
	} else if s.resolver != nil && apiKey.Group != nil {
		pricingAt := usagePricingAt(s.usageBillingNow, result.Duration)
		gid := apiKey.Group.ID
		cost, err = s.billingService.CalculateCostUnified(CostInput{
			Ctx:            ctx,
			Model:          billingModel,
			GroupID:        &gid,
			Tokens:         tokens,
			RequestCount:   requestCount,
			RateMultiplier: imageMultiplier,
			ServiceTier:    serviceTier,
			SizeTier:       sizeTier,
			PricingAt:      pricingAt,
			Resolver:       s.resolver,
		})
	} else {
		if result.ImageCount > 0 {
			var groupConfig *ImagePriceConfig
			if apiKey.Group != nil {
				groupConfig = &ImagePriceConfig{
					Price1K: apiKey.Group.ImagePrice1K,
					Price2K: apiKey.Group.ImagePrice2K,
					Price4K: apiKey.Group.ImagePrice4K,
				}
			}
			cost = s.billingService.CalculateImageCost(billingModel, sizeTier, result.ImageCount, groupConfig, imageMultiplier)
		} else {
			cost, err = s.billingService.CalculateCostWithServiceTier(billingModel, tokens, multiplier, serviceTier)
		}
	}
	if err != nil {
		cost = &CostBreakdown{ActualCost: 0}
	}
	if result.ImageCount > 0 && result.VideoCount == 0 {
		multiplier = imageMultiplier
	}

	// Determine billing type
	isSubscriptionBilling := subscription != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType()
	billingType := BillingTypeBalance
	if isSubscriptionBilling {
		billingType = BillingTypeSubscription
	}

	// Create usage log
	durationMs := int(result.Duration.Milliseconds())
	accountRateMultiplier := account.BillingRateMultiplier()
	requestID := resolveUsageBillingRequestID(ctx, result.RequestID)

	// 确定 RequestedModel（渠道映射前的原始模型）
	requestedModel := result.Model
	if input.OriginalModel != "" {
		requestedModel = input.OriginalModel
	}

	usageLog := &UsageLog{
		UserID:              user.ID,
		APIKeyID:            apiKey.ID,
		AccountID:           account.ID,
		RequestID:           requestID,
		Model:               result.Model,
		RequestedModel:      requestedModel,
		UpstreamModel:       optionalNonEqualStringPtr(result.UpstreamModel, result.Model),
		ServiceTier:         result.ServiceTier,
		ReasoningEffort:     result.ReasoningEffort,
		InboundEndpoint:     optionalTrimmedStringPtr(input.InboundEndpoint),
		UpstreamEndpoint:    optionalTrimmedStringPtr(input.UpstreamEndpoint),
		InputTokens:         actualInputTokens,
		OutputTokens:        result.Usage.OutputTokens,
		CacheCreationTokens: result.Usage.CacheCreationInputTokens,
		CacheReadTokens:     result.Usage.CacheReadInputTokens,
		ImageOutputTokens:   result.Usage.ImageOutputTokens,
		ImageCount:          maxInt(result.ImageCount, result.VideoCount),
	}
	if result.VideoCount > 0 {
		mediaType := "video"
		usageLog.MediaType = &mediaType
		usageLog.VideoCount = result.VideoCount
		videoResolution := NormalizeVideoBillingResolutionOrDefault(result.VideoResolution)
		videoDurationSeconds := NormalizeVideoBillingDurationSecondsOrDefault(result.VideoDurationSeconds)
		usageLog.VideoResolution = &videoResolution
		usageLog.VideoDurationSeconds = &videoDurationSeconds
	} else if result.ImageCount > 0 {
		mediaType := "image"
		usageLog.MediaType = &mediaType
	}
	if strings.TrimSpace(result.ImageSize) != "" {
		imageSize := strings.TrimSpace(result.ImageSize)
		usageLog.ImageSize = &imageSize
	}
	if cost != nil {
		usageLog.InputCost = cost.InputCost
		usageLog.OutputCost = cost.OutputCost
		usageLog.ImageOutputCost = cost.ImageOutputCost
		usageLog.CacheCreationCost = cost.CacheCreationCost
		usageLog.CacheReadCost = cost.CacheReadCost
		usageLog.TotalCost = cost.TotalCost
		usageLog.ActualCost = cost.ActualCost
		usageLog.BillingTier = optionalTrimmedStringPtr(cost.BillingTier)
	}
	usageLog.RateMultiplier = multiplier
	usageLog.AccountRateMultiplier = &accountRateMultiplier
	usageLog.BillingType = billingType
	usageLog.Stream = result.Stream
	usageLog.OpenAIWSMode = result.OpenAIWSMode
	usageLog.DurationMs = &durationMs
	usageLog.FirstTokenMs = result.FirstTokenMs
	usageLog.CreatedAt = time.Now()
	// 设置渠道信息
	usageLog.ChannelID = optionalInt64Ptr(input.ChannelID)
	usageLog.ModelMappingChain = optionalTrimmedStringPtr(input.ModelMappingChain)
	// 设置计费模式
	if cost != nil && cost.BillingMode != "" {
		billingMode := cost.BillingMode
		usageLog.BillingMode = &billingMode
	} else {
		billingMode := string(BillingModeToken)
		if result.VideoCount > 0 {
			billingMode = string(BillingModeVideo)
		} else if result.ImageCount > 0 {
			billingMode = string(BillingModeImage)
		}
		usageLog.BillingMode = &billingMode
	}
	// 添加 UserAgent
	if input.UserAgent != "" {
		usageLog.UserAgent = &input.UserAgent
	}

	// 添加 IPAddress
	if input.IPAddress != "" {
		usageLog.IPAddress = &input.IPAddress
	}

	if apiKey.GroupID != nil {
		usageLog.GroupID = apiKey.GroupID
	}
	if subscription != nil {
		usageLog.SubscriptionID = &subscription.ID
	}

	// 计算账号统计定价费用（使用最终上游模型匹配自定义规则）
	if apiKey.GroupID != nil {
		applyAccountStatsCost(ctx, usageLog, s.channelService, s.billingService,
			account.ID, *apiKey.GroupID, result.UpstreamModel, result.Model,
			tokens, cost.TotalCost,
		)
	}
	// Text supplier cost is derived from the same settled usage calculation as
	// account statistics. Image supplier cost remains excluded until its raw
	// upstream deduction reconciliation is authoritative.
	if result.ImageCount == 0 && result.VideoCount == 0 && apiKey.GroupID != nil {
		actualBaseCost := usageLog.TotalCost
		if usageLog.AccountStatsCost != nil {
			actualBaseCost = *usageLog.AccountStatsCost
		}
		routeObservationModel := strings.TrimSpace(input.RouteObservationModel)
		if routeObservationModel == "" {
			routeObservationModel = requestedModel
		}
		s.ReportOpenAIRouteActualCost(
			account,
			apiKey.GroupID,
			routeObservationModel,
			OpenAIRouteRequestClassText,
			input.UpstreamEndpoint,
			actualBaseCost,
			actualBaseCost*accountRateMultiplier,
		)
	}

	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		writeUsageLogBestEffort(ctx, s.usageLogRepo, usageLog, "service.openai_gateway")
		logger.LegacyPrintf("service.openai_gateway", "[SIMPLE MODE] Usage recorded (not billed): user=%d, tokens=%d", usageLog.UserID, usageLog.TotalTokens())
		s.deferredService.ScheduleLastUsedUpdate(account.ID)
		return nil
	}

	billingParams := &postUsageBillingParams{
		Cost:                  cost,
		User:                  user,
		APIKey:                apiKey,
		Account:               account,
		Subscription:          subscription,
		RequestPayloadHash:    resolveUsageBillingPayloadFingerprint(ctx, input.RequestPayloadHash),
		IsSubscriptionBill:    isSubscriptionBilling,
		AccountRateMultiplier: accountRateMultiplier,
		APIKeyService:         input.APIKeyService,
	}
	billingDeps := s.billingDeps()
	usageLog.AccountingCommand = buildUsageBillingCommand(requestID, usageLog, billingParams)
	if err := persistUsageLogForBilling(ctx, s.usageLogRepo, usageLog, "service.openai_gateway"); err != nil {
		return err
	}
	applied, billingErr := applyUsageBilling(ctx, requestID, usageLog, billingParams, billingDeps, s.usageBillingRepo)
	if billingErr != nil {
		return billingErr
	}
	triggerConsumptionCommission(applied, usageLog, billingParams, billingDeps)

	return nil
}

// ParseCodexRateLimitHeaders extracts Codex usage limits from response headers.
// Exported for use in ratelimit_service when handling OpenAI 429 responses.
func ParseCodexRateLimitHeaders(headers http.Header) *OpenAICodexUsageSnapshot {
	snapshot := &OpenAICodexUsageSnapshot{}
	hasData := false

	// Helper to parse float64 from header
	parseFloat := func(key string) *float64 {
		if v := headers.Get(key); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return &f
			}
		}
		return nil
	}

	// Helper to parse int from header
	parseInt := func(key string) *int {
		if v := headers.Get(key); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return &i
			}
		}
		return nil
	}

	// Primary (weekly) limits
	if v := parseFloat("x-codex-primary-used-percent"); v != nil {
		snapshot.PrimaryUsedPercent = v
		hasData = true
	}
	if v := parseInt("x-codex-primary-reset-after-seconds"); v != nil {
		snapshot.PrimaryResetAfterSeconds = v
		hasData = true
	}
	if v := parseInt("x-codex-primary-window-minutes"); v != nil {
		snapshot.PrimaryWindowMinutes = v
		hasData = true
	}

	// Secondary (5h) limits
	if v := parseFloat("x-codex-secondary-used-percent"); v != nil {
		snapshot.SecondaryUsedPercent = v
		hasData = true
	}
	if v := parseInt("x-codex-secondary-reset-after-seconds"); v != nil {
		snapshot.SecondaryResetAfterSeconds = v
		hasData = true
	}
	if v := parseInt("x-codex-secondary-window-minutes"); v != nil {
		snapshot.SecondaryWindowMinutes = v
		hasData = true
	}

	// Overflow ratio
	if v := parseFloat("x-codex-primary-over-secondary-limit-percent"); v != nil {
		snapshot.PrimaryOverSecondaryPercent = v
		hasData = true
	}

	if !hasData {
		return nil
	}

	snapshot.UpdatedAt = time.Now().Format(time.RFC3339)
	return snapshot
}

func codexSnapshotBaseTime(snapshot *OpenAICodexUsageSnapshot, fallback time.Time) time.Time {
	if snapshot == nil {
		return fallback
	}
	if snapshot.UpdatedAt == "" {
		return fallback
	}
	base, err := time.Parse(time.RFC3339, snapshot.UpdatedAt)
	if err != nil {
		return fallback
	}
	return base
}

func codexResetAtRFC3339(base time.Time, resetAfterSeconds *int) *string {
	if resetAfterSeconds == nil {
		return nil
	}
	sec := *resetAfterSeconds
	if sec < 0 {
		sec = 0
	}
	resetAt := base.Add(time.Duration(sec) * time.Second).Format(time.RFC3339)
	return &resetAt
}

func buildCodexUsageExtraUpdates(snapshot *OpenAICodexUsageSnapshot, fallbackNow time.Time) map[string]any {
	if snapshot == nil {
		return nil
	}

	baseTime := codexSnapshotBaseTime(snapshot, fallbackNow)
	updates := make(map[string]any)

	// 保存原始 primary/secondary 字段，便于排查问题
	if snapshot.PrimaryUsedPercent != nil {
		updates["codex_primary_used_percent"] = *snapshot.PrimaryUsedPercent
	}
	if snapshot.PrimaryResetAfterSeconds != nil {
		updates["codex_primary_reset_after_seconds"] = *snapshot.PrimaryResetAfterSeconds
	}
	if snapshot.PrimaryWindowMinutes != nil {
		updates["codex_primary_window_minutes"] = *snapshot.PrimaryWindowMinutes
	}
	if snapshot.SecondaryUsedPercent != nil {
		updates["codex_secondary_used_percent"] = *snapshot.SecondaryUsedPercent
	}
	if snapshot.SecondaryResetAfterSeconds != nil {
		updates["codex_secondary_reset_after_seconds"] = *snapshot.SecondaryResetAfterSeconds
	}
	if snapshot.SecondaryWindowMinutes != nil {
		updates["codex_secondary_window_minutes"] = *snapshot.SecondaryWindowMinutes
	}
	if snapshot.PrimaryOverSecondaryPercent != nil {
		updates["codex_primary_over_secondary_percent"] = *snapshot.PrimaryOverSecondaryPercent
	}
	updates["codex_usage_updated_at"] = baseTime.Format(time.RFC3339)

	// 归一化到 5h/7d 规范字段
	if normalized := snapshot.Normalize(); normalized != nil {
		if normalized.Used5hPercent != nil {
			updates["codex_5h_used_percent"] = *normalized.Used5hPercent
		}
		if normalized.Reset5hSeconds != nil {
			updates["codex_5h_reset_after_seconds"] = *normalized.Reset5hSeconds
		}
		if normalized.Window5hMinutes != nil {
			updates["codex_5h_window_minutes"] = *normalized.Window5hMinutes
		}
		if normalized.Used7dPercent != nil {
			updates["codex_7d_used_percent"] = *normalized.Used7dPercent
		}
		if normalized.Reset7dSeconds != nil {
			updates["codex_7d_reset_after_seconds"] = *normalized.Reset7dSeconds
		}
		if normalized.Window7dMinutes != nil {
			updates["codex_7d_window_minutes"] = *normalized.Window7dMinutes
		}
		if reset5hAt := codexResetAtRFC3339(baseTime, normalized.Reset5hSeconds); reset5hAt != nil {
			updates["codex_5h_reset_at"] = *reset5hAt
		}
		if reset7dAt := codexResetAtRFC3339(baseTime, normalized.Reset7dSeconds); reset7dAt != nil {
			updates["codex_7d_reset_at"] = *reset7dAt
		}
	}

	return updates
}

// updateCodexUsageSnapshot saves the Codex usage snapshot to account's Extra field
func (s *OpenAIGatewayService) updateCodexUsageSnapshot(ctx context.Context, accountID int64, snapshot *OpenAICodexUsageSnapshot) {
	if snapshot == nil {
		return
	}
	if s == nil || s.accountRepo == nil {
		return
	}

	now := time.Now()
	updates := buildCodexUsageExtraUpdates(snapshot, now)
	if len(updates) == 0 {
		return
	}
	if !s.getCodexSnapshotThrottle().Allow(accountID, now) {
		return
	}

	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.accountRepo.UpdateExtra(updateCtx, accountID, updates)
	}()
}

func (s *OpenAIGatewayService) UpdateCodexUsageSnapshotFromHeaders(ctx context.Context, accountID int64, headers http.Header) {
	if accountID <= 0 || headers == nil {
		return
	}
	if snapshot := ParseCodexRateLimitHeaders(headers); snapshot != nil {
		s.updateCodexUsageSnapshot(ctx, accountID, snapshot)
	}
}

func getOpenAIReasoningEffortFromReqBody(reqBody map[string]any) (value string, present bool) {
	if reqBody == nil {
		return "", false
	}

	// Primary: reasoning.effort
	if reasoning, ok := reqBody["reasoning"].(map[string]any); ok {
		if effort, ok := reasoning["effort"].(string); ok {
			return normalizeOpenAIReasoningEffort(effort), true
		}
	}

	// Fallback: some clients may use a flat field.
	if effort, ok := reqBody["reasoning_effort"].(string); ok {
		return normalizeOpenAIReasoningEffort(effort), true
	}

	return "", false
}

func deriveOpenAIReasoningEffortFromModel(model string) string {
	if strings.TrimSpace(model) == "" {
		return ""
	}

	modelID := strings.TrimSpace(model)
	if strings.Contains(modelID, "/") {
		parts := strings.Split(modelID, "/")
		modelID = parts[len(parts)-1]
	}

	parts := strings.FieldsFunc(strings.ToLower(modelID), func(r rune) bool {
		switch r {
		case '-', '_', ' ':
			return true
		default:
			return false
		}
	})
	if len(parts) == 0 {
		return ""
	}

	return normalizeOpenAIReasoningEffort(parts[len(parts)-1])
}

func extractOpenAIRequestMetaFromBody(body []byte) (model string, stream bool, promptCacheKey string) {
	if len(body) == 0 {
		return "", false, ""
	}

	model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
	stream = gjson.GetBytes(body, "stream").Bool()
	promptCacheKey = strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
	return model, stream, promptCacheKey
}

// normalizeOpenAIPassthroughOAuthBody 将透传 OAuth 请求体收敛为旧链路关键行为：
// 1) store=false 2) 非 compact 保持 stream=true；compact 强制 stream=false
func normalizeOpenAIPassthroughOAuthBody(body []byte, compact bool) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	normalized := body
	changed := false

	if compact {
		if store := gjson.GetBytes(normalized, "store"); store.Exists() {
			next, err := sjson.DeleteBytes(normalized, "store")
			if err != nil {
				return body, false, fmt.Errorf("normalize passthrough body delete store: %w", err)
			}
			normalized = next
			changed = true
		}
		if stream := gjson.GetBytes(normalized, "stream"); stream.Exists() {
			next, err := sjson.DeleteBytes(normalized, "stream")
			if err != nil {
				return body, false, fmt.Errorf("normalize passthrough body delete stream: %w", err)
			}
			normalized = next
			changed = true
		}
	} else {
		if store := gjson.GetBytes(normalized, "store"); !store.Exists() || store.Type != gjson.False {
			next, err := sjson.SetBytes(normalized, "store", false)
			if err != nil {
				return body, false, fmt.Errorf("normalize passthrough body store=false: %w", err)
			}
			normalized = next
			changed = true
		}
		if stream := gjson.GetBytes(normalized, "stream"); !stream.Exists() || stream.Type != gjson.True {
			next, err := sjson.SetBytes(normalized, "stream", true)
			if err != nil {
				return body, false, fmt.Errorf("normalize passthrough body stream=true: %w", err)
			}
			normalized = next
			changed = true
		}
	}

	return normalized, changed, nil
}

func detectOpenAIPassthroughInstructionsRejectReason(reqModel string, body []byte) string {
	model := strings.ToLower(strings.TrimSpace(reqModel))
	if !strings.Contains(model, "codex") {
		return ""
	}

	instructions := gjson.GetBytes(body, "instructions")
	if !instructions.Exists() {
		return "instructions_missing"
	}
	if instructions.Type != gjson.String {
		return "instructions_not_string"
	}
	if strings.TrimSpace(instructions.String()) == "" {
		return "instructions_empty"
	}
	return ""
}

func extractOpenAIReasoningEffortFromBody(body []byte, requestedModel string) *string {
	reasoningEffort := strings.TrimSpace(gjson.GetBytes(body, "reasoning.effort").String())
	if reasoningEffort == "" {
		reasoningEffort = strings.TrimSpace(gjson.GetBytes(body, "reasoning_effort").String())
	}
	if reasoningEffort != "" {
		normalized := normalizeOpenAIReasoningEffort(reasoningEffort)
		if normalized == "" {
			return nil
		}
		return &normalized
	}

	value := deriveOpenAIReasoningEffortFromModel(requestedModel)
	if value == "" {
		return nil
	}
	return &value
}

func extractOpenAIServiceTier(reqBody map[string]any) *string {
	if reqBody == nil {
		return nil
	}
	raw, ok := reqBody["service_tier"].(string)
	if !ok {
		return nil
	}
	return normalizeOpenAIServiceTier(raw)
}

func extractOpenAIServiceTierFromBody(body []byte) *string {
	if len(body) == 0 {
		return nil
	}
	return normalizeOpenAIServiceTier(gjson.GetBytes(body, "service_tier").String())
}

func normalizeOpenAIServiceTier(raw string) *string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return nil
	}
	if value == "fast" {
		value = "priority"
	}
	switch value {
	case "priority", "flex", "auto", "default", "scale":
		return &value
	default:
		return nil
	}
}

type OpenAIFastBlockedError struct {
	Message string
}

func (e *OpenAIFastBlockedError) Error() string { return e.Message }

func (s *OpenAIGatewayService) evaluateOpenAIFastPolicy(ctx context.Context, account *Account, model, serviceTier string) (action, errMsg string) {
	if s == nil {
		return BetaPolicyActionPass, ""
	}
	tier := strings.ToLower(strings.TrimSpace(serviceTier))
	if tier == "" {
		return BetaPolicyActionPass, ""
	}
	settings := openAIFastPolicySettingsFromContext(ctx)
	if settings == nil && s.settingService != nil {
		fetched, err := s.settingService.GetOpenAIFastPolicySettings(ctx)
		if err == nil && fetched != nil {
			settings = fetched
		}
	}
	if settings != nil {
		action, errMsg = evaluateOpenAIFastPolicyWithSettings(settings, account, model, tier)
		if action != BetaPolicyActionPass {
			return action, errMsg
		}
	}
	if s.serviceTierCapabilityGate && (tier == OpenAIFastTierPriority || tier == OpenAIFastTierFlex) && !s.hasVerifiedChannelServiceTier(ctx, model, tier) {
		return BetaPolicyActionFilter, ""
	}
	return BetaPolicyActionPass, ""
}

func (s *OpenAIGatewayService) hasVerifiedChannelServiceTier(ctx context.Context, model, tier string) bool {
	if s == nil || s.serviceTierPricing == nil || ctx == nil {
		return false
	}
	group, _ := ctx.Value(ctxkey.Group).(*Group)
	if !IsGroupContextValid(group) {
		return false
	}
	pricing := s.serviceTierPricing.GetChannelModelPricing(ctx, group.ID, strings.TrimSpace(model))
	return channelPricingSupportsServiceTier(pricing, tier)
}

func channelPricingSupportsServiceTier(pricing *ChannelModelPricing, tier string) bool {
	if pricing == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case OpenAIFastTierPriority:
		return pricing.FastSupported && pricing.FastVerifiedAt != nil && !pricing.FastVerifiedAt.IsZero() &&
			pricing.FastMultiplier != nil && *pricing.FastMultiplier > 0
	case OpenAIFastTierFlex:
		return pricing.FlexSupported && pricing.FlexVerifiedAt != nil && !pricing.FlexVerifiedAt.IsZero() &&
			pricing.FlexMultiplier != nil && *pricing.FlexMultiplier > 0
	default:
		return true
	}
}

func evaluateOpenAIFastPolicyWithSettings(settings *OpenAIFastPolicySettings, account *Account, model, tier string) (action, errMsg string) {
	if settings == nil {
		return BetaPolicyActionPass, ""
	}
	isOAuth := account != nil && account.IsOAuth()
	isBedrock := account != nil && account.IsBedrock()
	for _, rule := range settings.Rules {
		if !betaPolicyScopeMatches(rule.Scope, isOAuth, isBedrock) {
			continue
		}
		ruleTier := strings.ToLower(strings.TrimSpace(rule.ServiceTier))
		if ruleTier != "" && ruleTier != OpenAIFastTierAny && ruleTier != tier {
			continue
		}
		return resolveOpenAIFastPolicyRuleAction(rule, model)
	}
	return BetaPolicyActionPass, ""
}

func resolveOpenAIFastPolicyRuleAction(rule OpenAIFastPolicyRule, model string) (action, errorMessage string) {
	if len(rule.ModelWhitelist) == 0 {
		return rule.Action, rule.ErrorMessage
	}
	if matchOpenAIFastPolicyModelWhitelist(model, rule.ModelWhitelist) {
		return rule.Action, rule.ErrorMessage
	}
	if rule.FallbackAction != "" {
		return rule.FallbackAction, rule.FallbackErrorMessage
	}
	return BetaPolicyActionPass, ""
}

func matchOpenAIFastPolicyModelWhitelist(model string, whitelist []string) bool {
	for _, pattern := range whitelist {
		if matchModelPattern(pattern, model) {
			return true
		}
	}
	return false
}

type openAIFastPolicyCtxKeyType struct{}

var openAIFastPolicyCtxKey = openAIFastPolicyCtxKeyType{}

func withOpenAIFastPolicyContext(ctx context.Context, settings *OpenAIFastPolicySettings) context.Context {
	if ctx == nil || settings == nil {
		return ctx
	}
	return context.WithValue(ctx, openAIFastPolicyCtxKey, settings)
}

func openAIFastPolicySettingsFromContext(ctx context.Context) *OpenAIFastPolicySettings {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(openAIFastPolicyCtxKey).(*OpenAIFastPolicySettings); ok {
		return v
	}
	return nil
}

func (s *OpenAIGatewayService) applyOpenAIFastPolicyToBody(ctx context.Context, account *Account, model string, body []byte) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	rawTier := gjson.GetBytes(body, "service_tier")
	if !rawTier.Exists() || rawTier.Type != gjson.String {
		return body, nil
	}
	normTier := normalizedOpenAIServiceTierValue(rawTier.String())
	if normTier == "" {
		return body, nil
	}
	action, errMsg := s.evaluateOpenAIFastPolicy(ctx, account, model, normTier)
	switch action {
	case BetaPolicyActionBlock:
		msg := errMsg
		if msg == "" {
			msg = fmt.Sprintf("openai service_tier=%s is not allowed for model %s", normTier, model)
		}
		return body, &OpenAIFastBlockedError{Message: msg}
	case BetaPolicyActionFilter:
		trimmed, err := sjson.DeleteBytes(body, "service_tier")
		if err != nil {
			return body, fmt.Errorf("strip service_tier from body: %w", err)
		}
		return trimmed, nil
	default:
		if normTier == rawTier.String() {
			return body, nil
		}
		updated, err := sjson.SetBytes(body, "service_tier", normTier)
		if err != nil {
			return body, fmt.Errorf("normalize service_tier on pass: %w", err)
		}
		return updated, nil
	}
}

func writeOpenAIFastPolicyBlockedResponse(c *gin.Context, err *OpenAIFastBlockedError) {
	if c == nil || err == nil {
		return
	}
	c.JSON(http.StatusForbidden, OpenAIClientErrorEnvelope(c, "permission_error", err.Message))
}

func (s *OpenAIGatewayService) applyOpenAIFastPolicyToWSResponseCreate(
	ctx context.Context,
	account *Account,
	model string,
	frame []byte,
) ([]byte, *OpenAIFastBlockedError, error) {
	if len(frame) == 0 || !gjson.ValidBytes(frame) {
		return frame, nil, nil
	}
	if strings.TrimSpace(gjson.GetBytes(frame, "type").String()) != "response.create" {
		return frame, nil, nil
	}
	rawTier := gjson.GetBytes(frame, "service_tier")
	if !rawTier.Exists() || rawTier.Type != gjson.String {
		return frame, nil, nil
	}
	normTier := normalizedOpenAIServiceTierValue(rawTier.String())
	if normTier == "" {
		return frame, nil, nil
	}
	action, errMsg := s.evaluateOpenAIFastPolicy(ctx, account, model, normTier)
	switch action {
	case BetaPolicyActionBlock:
		msg := errMsg
		if msg == "" {
			msg = fmt.Sprintf("openai service_tier=%s is not allowed for model %s", normTier, model)
		}
		return frame, &OpenAIFastBlockedError{Message: msg}, nil
	case BetaPolicyActionFilter:
		trimmed, err := sjson.DeleteBytes(frame, "service_tier")
		if err != nil {
			return frame, nil, fmt.Errorf("strip service_tier from ws frame: %w", err)
		}
		return trimmed, nil, nil
	default:
		if normTier == rawTier.String() {
			return frame, nil, nil
		}
		updated, err := sjson.SetBytes(frame, "service_tier", normTier)
		if err != nil {
			return frame, nil, fmt.Errorf("normalize service_tier on ws frame: %w", err)
		}
		return updated, nil, nil
	}
}

func newOpenAIFastPolicyWSEventID() string {
	id, err := uuid.NewRandom()
	if err != nil {
		return "evt_openai_fast_policy"
	}
	return "evt_" + strings.ReplaceAll(id.String(), "-", "")
}

func buildOpenAIFastPolicyBlockedWSEvent(err *OpenAIFastBlockedError, requestID string) []byte {
	if err == nil {
		return nil
	}
	eventID := newOpenAIFastPolicyWSEventID()
	errorObj := map[string]any{
		"type":    "invalid_request_error",
		"code":    "policy_violation",
		"message": clientMessageWithRequestID(err.Message, requestID),
	}
	if requestID = strings.TrimSpace(requestID); requestID != "" {
		errorObj["request_id"] = requestID
	}
	payload, marshalErr := json.Marshal(map[string]any{
		"event_id": eventID,
		"type":     "error",
		"error":    errorObj,
	})
	if marshalErr != nil {
		if requestID != "" {
			return []byte(`{"event_id":"` + eventID + `","type":"error","error":{"type":"invalid_request_error","code":"policy_violation","message":"openai fast policy blocked this request","request_id":"` + requestID + `"}}`)
		}
		return []byte(`{"event_id":"` + eventID + `","type":"error","error":{"type":"invalid_request_error","code":"policy_violation","message":"openai fast policy blocked this request"}}`)
	}
	return payload
}

func sanitizeEmptyBase64InputImagesInOpenAIBody(body []byte) ([]byte, bool, error) {
	if len(body) == 0 || !bytes.Contains(body, []byte(`"image_url"`)) || !bytes.Contains(body, []byte(`base64,`)) {
		return body, false, nil
	}

	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return body, false, fmt.Errorf("sanitize request body: %w", err)
	}
	if !sanitizeEmptyBase64InputImagesInOpenAIRequestBodyMap(reqBody) {
		return body, false, nil
	}
	normalized, err := json.Marshal(reqBody)
	if err != nil {
		return body, false, fmt.Errorf("serialize sanitized request body: %w", err)
	}
	return normalized, true, nil
}

func sanitizeEmptyBase64InputImagesInOpenAIRequestBodyMap(reqBody map[string]any) bool {
	if reqBody == nil {
		return false
	}
	input, ok := reqBody["input"]
	if !ok {
		return false
	}
	normalizedInput, changed := sanitizeEmptyBase64InputImagesInOpenAIInput(input)
	if !changed {
		return false
	}
	reqBody["input"] = normalizedInput
	return true
}

func sanitizeEmptyBase64InputImagesInOpenAIInput(input any) (any, bool) {
	items, ok := input.([]any)
	if !ok {
		return input, false
	}

	normalizedItems := make([]any, 0, len(items))
	changed := false
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			normalizedItems = append(normalizedItems, item)
			continue
		}
		if shouldDropEmptyBase64InputImagePart(itemMap) {
			changed = true
			continue
		}
		content, ok := itemMap["content"]
		if !ok {
			normalizedItems = append(normalizedItems, itemMap)
			continue
		}
		parts, ok := content.([]any)
		if !ok {
			normalizedItems = append(normalizedItems, itemMap)
			continue
		}

		normalizedParts := make([]any, 0, len(parts))
		itemChanged := false
		for _, part := range parts {
			if shouldDropEmptyBase64InputImagePart(part) {
				changed = true
				itemChanged = true
				continue
			}
			normalizedParts = append(normalizedParts, part)
		}
		if itemChanged {
			if len(normalizedParts) == 0 {
				continue
			}
			itemMap["content"] = normalizedParts
		}
		normalizedItems = append(normalizedItems, itemMap)
	}
	if !changed {
		return input, false
	}
	return normalizedItems, true
}

func shouldDropEmptyBase64InputImagePart(part any) bool {
	partMap, ok := part.(map[string]any)
	if !ok {
		return false
	}
	typeValue, _ := partMap["type"].(string)
	if strings.TrimSpace(typeValue) != "input_image" {
		return false
	}
	imageURL, _ := partMap["image_url"].(string)
	return isEmptyBase64DataURI(imageURL)
}

func isEmptyBase64DataURI(raw string) bool {
	if !strings.HasPrefix(raw, "data:") {
		return false
	}
	rest := strings.TrimPrefix(raw, "data:")
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return false
	}
	rest = rest[semicolonIdx+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return false
	}
	return strings.TrimSpace(strings.TrimPrefix(rest, "base64,")) == ""
}

func getOpenAIRequestBodyMap(c *gin.Context, body []byte) (map[string]any, error) {
	if c != nil {
		if cached, ok := c.Get(OpenAIParsedRequestBodyKey); ok {
			if reqBody, ok := cached.(map[string]any); ok && reqBody != nil {
				return reqBody, nil
			}
		}
	}

	var reqBody map[string]any
	if err := decodeOpenAIJSONUseNumber(body, &reqBody); err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}
	if c != nil {
		c.Set(OpenAIParsedRequestBodyKey, reqBody)
	}
	return reqBody, nil
}

func extractOpenAIReasoningEffort(reqBody map[string]any, requestedModel string) *string {
	if value, present := getOpenAIReasoningEffortFromReqBody(reqBody); present {
		if value == "" {
			return nil
		}
		return &value
	}

	value := deriveOpenAIReasoningEffortFromModel(requestedModel)
	if value == "" {
		return nil
	}
	return &value
}

func normalizeOpenAIReasoningEffort(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}

	// Normalize separators for "x-high"/"x_high" variants.
	value = strings.NewReplacer("-", "", "_", "", " ", "").Replace(value)

	switch value {
	case "none", "minimal":
		return ""
	case "low", "medium", "high":
		return value
	case "xhigh", "extrahigh":
		return "xhigh"
	default:
		// Only store known effort levels for now to keep UI consistent.
		return ""
	}
}
