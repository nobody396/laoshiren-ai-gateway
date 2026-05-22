package service

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"golang.org/x/sync/singleflight"
)

const (
	cachePolicySettingsTTL      = 60 * time.Second
	cachePolicySettingsErrorTTL = 5 * time.Second
	cachePolicySettingsTimeout  = 2 * time.Second
)

type anthropicCacheControlPath struct {
	Path string
	TTL  string
}

type anthropicCachePolicySettings struct {
	mode                     string
	version                  string
	phase                    string
	tokenThreshold           int
	intervalThresholdMinutes int
	expiresAt                int64
}

var anthropicCachePolicySettingsCache atomic.Value // *anthropicCachePolicySettings
var anthropicCachePolicySettingsSF singleflight.Group

func (s *GatewayService) applyAnthropicCachePolicy(ctx context.Context, c *gin.Context, account *Account, model string, body []byte) ([]byte, *CachePolicyDecision) {
	if account == nil || account.Platform != PlatformAnthropic {
		return body, nil
	}

	paths := collectAnthropicCacheControlPaths(body)
	settings := s.getAnthropicCachePolicySettings(ctx)
	mode, modeSource := resolveAnthropicCachePolicyMode(account, groupFromCachePolicyContext(ctx), settings.mode)
	clientType := inferCachePolicyClientType(ctx, c)
	targetTTL := cacheTTLTarget5m
	shadowTTL := ""
	reasons := []string{fmt.Sprintf("mode=%s", mode), "mode_source=" + modeSource, "phase=" + settings.phase}

	unknownTTL := hasUnknownAnthropicCacheTTL(paths)
	illegalOrder := hasIllegalAnthropicCacheTTLOrder(paths)
	estimatedTokens := estimateAnthropicCacheableTokens(body)
	isCostSensitive := isCachePolicyCostSensitiveGroup(groupFromCachePolicyContext(ctx))

	switch mode {
	case CachePolicyModeForce1h:
		targetTTL = cacheTTLTarget1h
		reasons = append(reasons, "force_1h")
	case CachePolicyModeAdaptive:
		targetTTL, reasons = evaluateAnthropicAdaptiveTTL(clientType, estimatedTokens, settings.tokenThreshold, isCostSensitive, reasons)
	case CachePolicyModeShadowAdaptive:
		shadowTTL, reasons = evaluateAnthropicAdaptiveTTL(clientType, estimatedTokens, settings.tokenThreshold, isCostSensitive, reasons)
		targetTTL = cacheTTLTarget5m
		reasons = append(reasons, "shadow_actual_safe_5m")
	case CachePolicyModeForce5m:
		targetTTL = cacheTTLTarget5m
		reasons = append(reasons, "force_5m")
	default:
		targetTTL = cacheTTLTarget5m
		reasons = append(reasons, "safe_5m_default")
	}

	downgraded := false
	if illegalOrder || unknownTTL {
		if targetTTL != cacheTTLTarget5m {
			reasons = append(reasons, "unsafe_ttl_state_downgrade_5m")
		}
		targetTTL = cacheTTLTarget5m
		downgraded = true
	}
	if targetTTL == cacheTTLTarget5m && containsAnthropicCacheTTL(paths, cacheTTLTarget1h) {
		downgraded = true
	}
	if illegalOrder {
		reasons = append(reasons, "illegal_order_detected")
	}
	if unknownTTL {
		reasons = append(reasons, "unknown_ttl_detected")
	}
	if len(paths) == 0 {
		reasons = append(reasons, "no_cache_control")
	}

	out := body
	normalized := false
	if len(paths) > 0 {
		out = forceEphemeralCacheControlTTL(body, targetTTL)
		normalized = !bytes.Equal(out, body)
		if targetTTL == cacheTTLTarget5m {
			reasons = append(reasons, "normalized_all_cache_blocks_5m")
		} else {
			reasons = append(reasons, "normalized_all_cache_blocks_1h")
		}
	}

	decision := &CachePolicyDecision{
		ClientType:             clientType,
		Model:                  strings.TrimSpace(model),
		PolicyMode:             mode,
		PolicyVersion:          settings.version,
		ActualTTL:              targetTTL,
		ShadowTTL:              shadowTTL,
		DecisionReason:         strings.Join(reasons, ";"),
		CacheControlPathsCount: len(paths),
		Normalized:             normalized || illegalOrder || unknownTTL,
		Downgraded:             downgraded,
		CreatedAt:              time.Now(),
	}
	if ctx != nil {
		if clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
			decision.ClientRequestID = strings.TrimSpace(clientRequestID)
		}
	}
	return out, decision
}

func collectAnthropicCacheControlPaths(body []byte) []anthropicCacheControlPath {
	var paths []anthropicCacheControlPath
	addPath := func(path string, value gjson.Result) {
		cc := value.Get("cache_control")
		if !cc.Exists() || cc.Get("type").String() != "ephemeral" {
			return
		}
		paths = append(paths, anthropicCacheControlPath{
			Path: path + ".cache_control",
			TTL:  strings.TrimSpace(cc.Get("ttl").String()),
		})
	}

	tools := gjson.GetBytes(body, "tools")
	if tools.IsArray() {
		idx := -1
		tools.ForEach(func(_, tool gjson.Result) bool {
			idx++
			addPath(fmt.Sprintf("tools.%d", idx), tool)
			return true
		})
	}

	system := gjson.GetBytes(body, "system")
	if system.IsArray() {
		idx := -1
		system.ForEach(func(_, block gjson.Result) bool {
			idx++
			addPath(fmt.Sprintf("system.%d", idx), block)
			return true
		})
	}

	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		msgIdx := -1
		messages.ForEach(func(_, msg gjson.Result) bool {
			msgIdx++
			content := msg.Get("content")
			if !content.IsArray() {
				return true
			}
			contentIdx := -1
			content.ForEach(func(_, block gjson.Result) bool {
				contentIdx++
				addPath(fmt.Sprintf("messages.%d.content.%d", msgIdx, contentIdx), block)
				return true
			})
			return true
		})
	}

	return paths
}

func hasIllegalAnthropicCacheTTLOrder(paths []anthropicCacheControlPath) bool {
	seen5m := false
	for _, path := range paths {
		ttl := strings.TrimSpace(path.TTL)
		if ttl == "" || ttl == cacheTTLTarget5m {
			seen5m = true
			continue
		}
		if ttl == cacheTTLTarget1h && seen5m {
			return true
		}
	}
	return false
}

func hasUnknownAnthropicCacheTTL(paths []anthropicCacheControlPath) bool {
	for _, path := range paths {
		ttl := strings.TrimSpace(path.TTL)
		if ttl != "" && ttl != cacheTTLTarget5m && ttl != cacheTTLTarget1h {
			return true
		}
	}
	return false
}

func containsAnthropicCacheTTL(paths []anthropicCacheControlPath, ttl string) bool {
	for _, path := range paths {
		if strings.TrimSpace(path.TTL) == ttl {
			return true
		}
	}
	return false
}

func isAnthropicCacheTTLOrder400(body []byte) bool {
	msg := strings.ToLower(string(body))
	return strings.Contains(msg, "cache_control.ttl") &&
		strings.Contains(msg, "ttl='1h'") &&
		strings.Contains(msg, "ttl='5m'") &&
		strings.Contains(msg, "must not come after")
}

func markAnthropicCacheTTLRetry(decision *CachePolicyDecision, reason string) {
	if decision == nil {
		return
	}
	decision.ActualTTL = cacheTTLTarget5m
	decision.Normalized = true
	decision.Downgraded = true
	decision.Retried = true
	if strings.TrimSpace(reason) != "" {
		if decision.DecisionReason != "" {
			decision.DecisionReason += ";"
		}
		decision.DecisionReason += reason
	}
}

func (s *GatewayService) getAnthropicCachePolicySettings(ctx context.Context) anthropicCachePolicySettings {
	defaults := defaultAnthropicCachePolicySettings()
	if s == nil || s.settingService == nil || s.settingService.settingRepo == nil {
		return defaults
	}

	now := time.Now()
	if cached, ok := anthropicCachePolicySettingsCache.Load().(*anthropicCachePolicySettings); ok && cached != nil && now.UnixNano() < cached.expiresAt {
		return *cached
	}

	value, err, _ := anthropicCachePolicySettingsSF.Do("settings", func() (any, error) {
		loadCtx := ctx
		if loadCtx == nil {
			loadCtx = context.Background()
		}
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(loadCtx), cachePolicySettingsTimeout)
		defer cancel()

		keys := []string{
			SettingKeyAnthropicCachePolicyMode,
			SettingKeyAnthropicCachePolicyVersion,
			SettingKeyAnthropicCachePolicyPhase,
			SettingKeyAnthropicCachePolicyTokenThreshold,
			SettingKeyAnthropicCachePolicyIntervalMins,
		}
		values, err := s.settingService.settingRepo.GetMultiple(loadCtx, keys)
		if err != nil {
			defaults.expiresAt = time.Now().Add(cachePolicySettingsErrorTTL).UnixNano()
			anthropicCachePolicySettingsCache.Store(&defaults)
			return defaults, err
		}

		loaded := defaultAnthropicCachePolicySettings()
		if raw := strings.TrimSpace(values[SettingKeyAnthropicCachePolicyMode]); raw != "" {
			loaded.mode = normalizeCachePolicyMode(raw)
		}
		if raw := strings.TrimSpace(values[SettingKeyAnthropicCachePolicyVersion]); raw != "" {
			loaded.version = raw
		}
		if raw := strings.TrimSpace(values[SettingKeyAnthropicCachePolicyPhase]); raw != "" {
			loaded.phase = raw
		}
		if raw := strings.TrimSpace(values[SettingKeyAnthropicCachePolicyTokenThreshold]); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil {
				loaded.tokenThreshold = clampInt(parsed, 30000, 80000)
			}
		}
		if raw := strings.TrimSpace(values[SettingKeyAnthropicCachePolicyIntervalMins]); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil {
				loaded.intervalThresholdMinutes = clampInt(parsed, 4, 12)
			}
		}
		loaded.expiresAt = time.Now().Add(cachePolicySettingsTTL).UnixNano()
		anthropicCachePolicySettingsCache.Store(&loaded)
		return loaded, nil
	})
	if err != nil {
		if settings, ok := value.(anthropicCachePolicySettings); ok {
			return settings
		}
		return defaults
	}
	settings, ok := value.(anthropicCachePolicySettings)
	if !ok {
		return defaults
	}
	return settings
}

func defaultAnthropicCachePolicySettings() anthropicCachePolicySettings {
	return anthropicCachePolicySettings{
		mode:                     CachePolicyModeSafe5m,
		version:                  "v1",
		phase:                    "phase0_protection",
		tokenThreshold:           40000,
		intervalThresholdMinutes: 5,
		expiresAt:                time.Now().Add(cachePolicySettingsTTL).UnixNano(),
	}
}

func normalizeCachePolicyMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case CachePolicyModeSafe5m, CachePolicyModeShadowAdaptive, CachePolicyModeAdaptive, CachePolicyModeForce5m, CachePolicyModeForce1h:
		return strings.TrimSpace(mode)
	default:
		return CachePolicyModeSafe5m
	}
}

func resolveAnthropicCachePolicyMode(account *Account, group *Group, globalMode string) (string, string) {
	if mode, ok := accountCachePolicyMode(account); ok {
		return mode, "account"
	}
	if group != nil && group.ClaudeCodeOnly && globalMode == CachePolicyModeAdaptive {
		return CachePolicyModeAdaptive, "group_claude_code"
	}
	if mode := normalizeCachePolicyMode(globalMode); mode != "" {
		return mode, "global"
	}
	return CachePolicyModeSafe5m, "fallback"
}

func accountCachePolicyMode(account *Account) (string, bool) {
	if account == nil || account.Extra == nil {
		return "", false
	}
	for _, key := range []string{"anthropic_cache_policy_mode", "cache_policy_mode"} {
		raw, ok := account.Extra[key]
		if !ok {
			continue
		}
		if value, ok := raw.(string); ok {
			mode := normalizeCachePolicyMode(value)
			if mode != CachePolicyModeSafe5m || strings.TrimSpace(value) == CachePolicyModeSafe5m {
				return mode, true
			}
		}
	}
	return "", false
}

func groupFromCachePolicyContext(ctx context.Context) *Group {
	if ctx == nil {
		return nil
	}
	group, _ := ctx.Value(ctxkey.Group).(*Group)
	if IsGroupContextValid(group) {
		return group
	}
	return nil
}

func inferCachePolicyClientType(ctx context.Context, c *gin.Context) string {
	ua := ""
	if c != nil {
		ua = strings.TrimSpace(c.GetHeader("User-Agent"))
	}
	lower := strings.ToLower(ua)
	if strings.Contains(lower, "claude-code") || strings.Contains(lower, "claude code") {
		return "claude_code"
	}
	if strings.Contains(lower, "claude") && (strings.Contains(lower, "electron") || strings.Contains(lower, "desktop")) {
		return "claude_desktop"
	}
	if strings.Contains(lower, "codex") {
		return "codex"
	}
	if strings.Contains(lower, "cursor") {
		return "cursor"
	}
	if strings.Contains(lower, "agent") {
		return "agent"
	}
	if ctx != nil {
		if v, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.Contains(strings.ToLower(v), "claude") {
			return "claude_desktop"
		}
	}
	return "api"
}

func evaluateAnthropicAdaptiveTTL(clientType string, estimatedTokens int, tokenThreshold int, costSensitive bool, reasons []string) (string, []string) {
	if costSensitive {
		return cacheTTLTarget5m, append(reasons, "cost_sensitive_group")
	}
	if estimatedTokens < 30000 {
		return cacheTTLTarget5m, append(reasons, fmt.Sprintf("cacheable_tokens_lt_30000:%d", estimatedTokens))
	}
	threshold := clampInt(tokenThreshold, 30000, 80000)
	if estimatedTokens < threshold {
		return cacheTTLTarget5m, append(reasons, fmt.Sprintf("cacheable_tokens_lt_threshold:%d/%d", estimatedTokens, threshold))
	}
	switch clientType {
	case "claude_desktop", "claude_code", "agent":
		return cacheTTLTarget1h, append(reasons, fmt.Sprintf("agent_large_context_candidate:%d", estimatedTokens))
	default:
		return cacheTTLTarget5m, append(reasons, "client_not_long_task_class")
	}
}

func estimateAnthropicCacheableTokens(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	return len(body) / 4
}

func isCachePolicyCostSensitiveGroup(group *Group) bool {
	if group == nil {
		return false
	}
	name := strings.ToLower(group.Name + " " + group.Description)
	if strings.Contains(name, "成本") ||
		strings.Contains(name, "cost") ||
		strings.Contains(name, "c端") ||
		strings.Contains(name, "consumer") ||
		strings.Contains(name, "普通") {
		return true
	}
	return group.IsFreeSubscription() || group.RateMultiplier <= 0
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
