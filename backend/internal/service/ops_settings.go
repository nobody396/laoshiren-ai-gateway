package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	opsAlertEvaluatorLeaderLockKeyDefault = "ops:alert:evaluator:leader"
	opsAlertEvaluatorLeaderLockTTLDefault = 30 * time.Second
)

// =========================
// Email notification config
// =========================

func (s *OpsService) GetEmailNotificationConfig(ctx context.Context) (*OpsEmailNotificationConfig, error) {
	defaultCfg := defaultOpsEmailNotificationConfig()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsEmailNotificationConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			// Initialize defaults on first read (best-effort).
			if b, mErr := json.Marshal(defaultCfg); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyOpsEmailNotificationConfig, string(b))
			}
			return defaultCfg, nil
		}
		return nil, err
	}

	cfg := &OpsEmailNotificationConfig{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		// Corrupted JSON should not break ops UI; fall back to defaults.
		return defaultCfg, nil
	}
	normalizeOpsEmailNotificationConfig(cfg)
	return cfg, nil
}

func (s *OpsService) UpdateEmailNotificationConfig(ctx context.Context, req *OpsEmailNotificationConfigUpdateRequest) (*OpsEmailNotificationConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		return nil, errors.New("invalid request")
	}

	cfg, err := s.GetEmailNotificationConfig(ctx)
	if err != nil {
		return nil, err
	}

	if req.Alert != nil {
		cfg.Alert.Enabled = req.Alert.Enabled
		if req.Alert.Recipients != nil {
			cfg.Alert.Recipients = req.Alert.Recipients
		}
		cfg.Alert.MinSeverity = strings.TrimSpace(req.Alert.MinSeverity)
		cfg.Alert.RateLimitPerHour = req.Alert.RateLimitPerHour
		cfg.Alert.BatchingWindowSeconds = req.Alert.BatchingWindowSeconds
		cfg.Alert.IncludeResolvedAlerts = req.Alert.IncludeResolvedAlerts
	}

	if req.Report != nil {
		cfg.Report.Enabled = req.Report.Enabled
		if req.Report.Recipients != nil {
			cfg.Report.Recipients = req.Report.Recipients
		}
		cfg.Report.DailySummaryEnabled = req.Report.DailySummaryEnabled
		cfg.Report.DailySummarySchedule = strings.TrimSpace(req.Report.DailySummarySchedule)
		cfg.Report.WeeklySummaryEnabled = req.Report.WeeklySummaryEnabled
		cfg.Report.WeeklySummarySchedule = strings.TrimSpace(req.Report.WeeklySummarySchedule)
		cfg.Report.ErrorDigestEnabled = req.Report.ErrorDigestEnabled
		cfg.Report.ErrorDigestSchedule = strings.TrimSpace(req.Report.ErrorDigestSchedule)
		cfg.Report.ErrorDigestMinCount = req.Report.ErrorDigestMinCount
		cfg.Report.AccountHealthEnabled = req.Report.AccountHealthEnabled
		cfg.Report.AccountHealthSchedule = strings.TrimSpace(req.Report.AccountHealthSchedule)
		cfg.Report.AccountHealthErrorRateThreshold = req.Report.AccountHealthErrorRateThreshold
	}

	if err := validateOpsEmailNotificationConfig(cfg); err != nil {
		return nil, err
	}

	normalizeOpsEmailNotificationConfig(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpsEmailNotificationConfig, string(raw)); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultOpsEmailNotificationConfig() *OpsEmailNotificationConfig {
	return &OpsEmailNotificationConfig{
		Alert: OpsEmailAlertConfig{
			Enabled:               true,
			Recipients:            []string{},
			MinSeverity:           "",
			RateLimitPerHour:      0,
			BatchingWindowSeconds: 0,
			IncludeResolvedAlerts: false,
		},
		Report: OpsEmailReportConfig{
			Enabled:                         false,
			Recipients:                      []string{},
			DailySummaryEnabled:             false,
			DailySummarySchedule:            "0 9 * * *",
			WeeklySummaryEnabled:            false,
			WeeklySummarySchedule:           "0 9 * * 1",
			ErrorDigestEnabled:              false,
			ErrorDigestSchedule:             "0 9 * * *",
			ErrorDigestMinCount:             10,
			AccountHealthEnabled:            false,
			AccountHealthSchedule:           "0 9 * * *",
			AccountHealthErrorRateThreshold: 10.0,
		},
	}
}

func normalizeOpsEmailNotificationConfig(cfg *OpsEmailNotificationConfig) {
	if cfg == nil {
		return
	}
	if cfg.Alert.Recipients == nil {
		cfg.Alert.Recipients = []string{}
	}
	if cfg.Report.Recipients == nil {
		cfg.Report.Recipients = []string{}
	}

	cfg.Alert.MinSeverity = strings.TrimSpace(cfg.Alert.MinSeverity)
	cfg.Report.DailySummarySchedule = strings.TrimSpace(cfg.Report.DailySummarySchedule)
	cfg.Report.WeeklySummarySchedule = strings.TrimSpace(cfg.Report.WeeklySummarySchedule)
	cfg.Report.ErrorDigestSchedule = strings.TrimSpace(cfg.Report.ErrorDigestSchedule)
	cfg.Report.AccountHealthSchedule = strings.TrimSpace(cfg.Report.AccountHealthSchedule)

	// Fill missing schedules with defaults to avoid breaking cron logic if clients send empty strings.
	if cfg.Report.DailySummarySchedule == "" {
		cfg.Report.DailySummarySchedule = "0 9 * * *"
	}
	if cfg.Report.WeeklySummarySchedule == "" {
		cfg.Report.WeeklySummarySchedule = "0 9 * * 1"
	}
	if cfg.Report.ErrorDigestSchedule == "" {
		cfg.Report.ErrorDigestSchedule = "0 9 * * *"
	}
	if cfg.Report.AccountHealthSchedule == "" {
		cfg.Report.AccountHealthSchedule = "0 9 * * *"
	}
}

func validateOpsEmailNotificationConfig(cfg *OpsEmailNotificationConfig) error {
	if cfg == nil {
		return errors.New("invalid config")
	}

	if cfg.Alert.RateLimitPerHour < 0 {
		return errors.New("alert.rate_limit_per_hour must be >= 0")
	}
	if cfg.Alert.BatchingWindowSeconds < 0 {
		return errors.New("alert.batching_window_seconds must be >= 0")
	}
	switch strings.TrimSpace(cfg.Alert.MinSeverity) {
	case "", "critical", "warning", "info":
	default:
		return errors.New("alert.min_severity must be one of: critical, warning, info, or empty")
	}

	if cfg.Report.ErrorDigestMinCount < 0 {
		return errors.New("report.error_digest_min_count must be >= 0")
	}
	if cfg.Report.AccountHealthErrorRateThreshold < 0 || cfg.Report.AccountHealthErrorRateThreshold > 100 {
		return errors.New("report.account_health_error_rate_threshold must be between 0 and 100")
	}
	return nil
}

// =========================
// Webhook notification config
// =========================

func (s *OpsService) GetWebhookNotificationConfig(ctx context.Context) (*OpsWebhookNotificationConfig, error) {
	cfg, err := s.getWebhookNotificationConfigRaw(ctx)
	if err != nil {
		return nil, err
	}
	return redactOpsWebhookNotificationConfig(cfg), nil
}

func (s *OpsService) UpdateWebhookNotificationConfig(ctx context.Context, req *OpsWebhookNotificationConfigUpdateRequest) (*OpsWebhookNotificationConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req == nil {
		return nil, errors.New("invalid request")
	}

	cfg, err := s.getWebhookNotificationConfigRaw(ctx)
	if err != nil {
		return nil, err
	}

	if req.Feishu != nil {
		cfg.Feishu.Enabled = req.Feishu.Enabled
		cfg.Feishu.Name = strings.TrimSpace(req.Feishu.Name)
		if strings.TrimSpace(req.Feishu.WebhookURL) != "" {
			cfg.Feishu.WebhookURL = strings.TrimSpace(req.Feishu.WebhookURL)
		}
		if strings.TrimSpace(req.Feishu.Secret) != "" {
			cfg.Feishu.Secret = strings.TrimSpace(req.Feishu.Secret)
		}
		cfg.Feishu.MinSeverity = strings.TrimSpace(req.Feishu.MinSeverity)
		cfg.Feishu.RateLimitPerHour = req.Feishu.RateLimitPerHour
	}

	if req.DingTalk != nil {
		cfg.DingTalk.Enabled = req.DingTalk.Enabled
		cfg.DingTalk.Name = strings.TrimSpace(req.DingTalk.Name)
		if strings.TrimSpace(req.DingTalk.WebhookURL) != "" {
			cfg.DingTalk.WebhookURL = strings.TrimSpace(req.DingTalk.WebhookURL)
		}
		if strings.TrimSpace(req.DingTalk.Secret) != "" {
			cfg.DingTalk.Secret = strings.TrimSpace(req.DingTalk.Secret)
		}
		cfg.DingTalk.MinSeverity = strings.TrimSpace(req.DingTalk.MinSeverity)
		cfg.DingTalk.RateLimitPerHour = req.DingTalk.RateLimitPerHour
	}

	if req.Telegram != nil {
		cfg.Telegram.Enabled = req.Telegram.Enabled
		cfg.Telegram.Name = strings.TrimSpace(req.Telegram.Name)
		if strings.TrimSpace(req.Telegram.BotToken) != "" {
			cfg.Telegram.BotToken = strings.TrimSpace(req.Telegram.BotToken)
		}
		cfg.Telegram.ChatID = strings.TrimSpace(req.Telegram.ChatID)
		cfg.Telegram.MinSeverity = strings.TrimSpace(req.Telegram.MinSeverity)
		cfg.Telegram.RateLimitPerHour = req.Telegram.RateLimitPerHour
	}

	normalizeOpsWebhookNotificationConfig(cfg)
	if err := validateOpsWebhookNotificationConfig(cfg); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpsWebhookNotificationConfig, string(raw)); err != nil {
		return nil, err
	}
	return redactOpsWebhookNotificationConfig(cfg), nil
}

func (s *OpsService) TestWebhookNotification(ctx context.Context, channel string) (*OpsWebhookNotificationTestResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := s.getWebhookNotificationConfigRaw(ctx)
	if err != nil {
		return nil, err
	}

	text := buildOpsWebhookTestMessage()
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "feishu":
		if !cfg.Feishu.Enabled {
			return nil, errors.New("feishu notification is disabled")
		}
		if err := sendOpsFeishuText(ctx, opsNotificationHTTPClient, cfg.Feishu, text); err != nil {
			return nil, err
		}
		return &OpsWebhookNotificationTestResponse{Channel: "feishu", Sent: true}, nil
	case "dingtalk":
		if !cfg.DingTalk.Enabled {
			return nil, errors.New("dingtalk notification is disabled")
		}
		if err := sendOpsDingTalkText(ctx, opsNotificationHTTPClient, cfg.DingTalk, text); err != nil {
			return nil, err
		}
		return &OpsWebhookNotificationTestResponse{Channel: "dingtalk", Sent: true}, nil
	case "telegram":
		if !cfg.Telegram.Enabled {
			return nil, errors.New("telegram notification is disabled")
		}
		if err := sendOpsTelegramText(ctx, opsNotificationHTTPClient, cfg.Telegram, text); err != nil {
			return nil, err
		}
		return &OpsWebhookNotificationTestResponse{Channel: "telegram", Sent: true}, nil
	default:
		return nil, errors.New("channel must be feishu, dingtalk, or telegram")
	}
}

func (s *OpsService) getWebhookNotificationConfigRaw(ctx context.Context) (*OpsWebhookNotificationConfig, error) {
	defaultCfg := defaultOpsWebhookNotificationConfig()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsWebhookNotificationConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			if b, mErr := json.Marshal(defaultCfg); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyOpsWebhookNotificationConfig, string(b))
			}
			return defaultCfg, nil
		}
		return nil, err
	}

	cfg := &OpsWebhookNotificationConfig{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return defaultCfg, nil
	}
	normalizeOpsWebhookNotificationConfig(cfg)
	return cfg, nil
}

func defaultOpsWebhookNotificationConfig() *OpsWebhookNotificationConfig {
	return &OpsWebhookNotificationConfig{
		Feishu: OpsFeishuNotificationConfig{
			Enabled:          false,
			Name:             "飞书告警群",
			MinSeverity:      "warning",
			RateLimitPerHour: 20,
		},
		DingTalk: OpsDingTalkNotificationConfig{
			Enabled:          false,
			Name:             "钉钉告警群",
			MinSeverity:      "warning",
			RateLimitPerHour: 20,
		},
		Telegram: OpsTelegramNotificationConfig{
			Enabled:          false,
			Name:             "Telegram 告警群",
			MinSeverity:      "critical",
			RateLimitPerHour: 10,
		},
	}
}

func normalizeOpsWebhookNotificationConfig(cfg *OpsWebhookNotificationConfig) {
	if cfg == nil {
		return
	}
	cfg.Feishu.Name = strings.TrimSpace(cfg.Feishu.Name)
	if cfg.Feishu.Name == "" {
		cfg.Feishu.Name = "飞书告警群"
	}
	cfg.Feishu.WebhookURL = strings.TrimSpace(cfg.Feishu.WebhookURL)
	cfg.Feishu.Secret = strings.TrimSpace(cfg.Feishu.Secret)
	cfg.Feishu.MinSeverity = strings.TrimSpace(cfg.Feishu.MinSeverity)
	if cfg.Feishu.RateLimitPerHour < 0 {
		cfg.Feishu.RateLimitPerHour = 0
	}

	cfg.DingTalk.Name = strings.TrimSpace(cfg.DingTalk.Name)
	if cfg.DingTalk.Name == "" {
		cfg.DingTalk.Name = "钉钉告警群"
	}
	cfg.DingTalk.WebhookURL = strings.TrimSpace(cfg.DingTalk.WebhookURL)
	cfg.DingTalk.Secret = strings.TrimSpace(cfg.DingTalk.Secret)
	cfg.DingTalk.MinSeverity = strings.TrimSpace(cfg.DingTalk.MinSeverity)
	if cfg.DingTalk.RateLimitPerHour < 0 {
		cfg.DingTalk.RateLimitPerHour = 0
	}

	cfg.Telegram.Name = strings.TrimSpace(cfg.Telegram.Name)
	if cfg.Telegram.Name == "" {
		cfg.Telegram.Name = "Telegram 告警群"
	}
	cfg.Telegram.BotToken = strings.TrimSpace(cfg.Telegram.BotToken)
	cfg.Telegram.ChatID = strings.TrimSpace(cfg.Telegram.ChatID)
	cfg.Telegram.MinSeverity = strings.TrimSpace(cfg.Telegram.MinSeverity)
	if cfg.Telegram.RateLimitPerHour < 0 {
		cfg.Telegram.RateLimitPerHour = 0
	}
}

func validateOpsWebhookNotificationConfig(cfg *OpsWebhookNotificationConfig) error {
	if cfg == nil {
		return errors.New("invalid config")
	}
	if cfg.Feishu.RateLimitPerHour < 0 {
		return errors.New("feishu.rate_limit_per_hour must be >= 0")
	}
	if cfg.DingTalk.RateLimitPerHour < 0 {
		return errors.New("dingtalk.rate_limit_per_hour must be >= 0")
	}
	if cfg.Telegram.RateLimitPerHour < 0 {
		return errors.New("telegram.rate_limit_per_hour must be >= 0")
	}
	if err := validateOpsNotificationSeverity(cfg.Feishu.MinSeverity, "feishu.min_severity"); err != nil {
		return err
	}
	if err := validateOpsNotificationSeverity(cfg.DingTalk.MinSeverity, "dingtalk.min_severity"); err != nil {
		return err
	}
	if err := validateOpsNotificationSeverity(cfg.Telegram.MinSeverity, "telegram.min_severity"); err != nil {
		return err
	}
	if cfg.Feishu.Enabled && cfg.Feishu.WebhookURL == "" {
		return errors.New("feishu.webhook_url is required when enabled")
	}
	if cfg.DingTalk.Enabled && cfg.DingTalk.WebhookURL == "" {
		return errors.New("dingtalk.webhook_url is required when enabled")
	}
	if cfg.Telegram.Enabled {
		if cfg.Telegram.BotToken == "" {
			return errors.New("telegram.bot_token is required when enabled")
		}
		if cfg.Telegram.ChatID == "" {
			return errors.New("telegram.chat_id is required when enabled")
		}
		if strings.ContainsAny(cfg.Telegram.BotToken, "/ \t\r\n") {
			return errors.New("telegram.bot_token is invalid")
		}
	}
	return nil
}

func validateOpsNotificationSeverity(raw string, field string) error {
	switch strings.TrimSpace(raw) {
	case "", "critical", "warning", "info":
		return nil
	default:
		return errors.New(field + " must be one of: critical, warning, info, or empty")
	}
}

func redactOpsWebhookNotificationConfig(cfg *OpsWebhookNotificationConfig) *OpsWebhookNotificationConfig {
	if cfg == nil {
		return defaultOpsWebhookNotificationConfig()
	}
	clone := *cfg
	clone.Feishu.WebhookURLConfigured = strings.TrimSpace(cfg.Feishu.WebhookURL) != ""
	clone.Feishu.SecretConfigured = strings.TrimSpace(cfg.Feishu.Secret) != ""
	clone.Feishu.WebhookURL = ""
	clone.Feishu.Secret = ""
	clone.DingTalk.WebhookURLConfigured = strings.TrimSpace(cfg.DingTalk.WebhookURL) != ""
	clone.DingTalk.SecretConfigured = strings.TrimSpace(cfg.DingTalk.Secret) != ""
	clone.DingTalk.WebhookURL = ""
	clone.DingTalk.Secret = ""
	clone.Telegram.BotTokenConfigured = strings.TrimSpace(cfg.Telegram.BotToken) != ""
	clone.Telegram.BotToken = ""
	return &clone
}

// =========================
// Alert runtime settings
// =========================

func defaultOpsAlertRuntimeSettings() *OpsAlertRuntimeSettings {
	return &OpsAlertRuntimeSettings{
		EvaluationIntervalSeconds: 60,
		DistributedLock: OpsDistributedLockSettings{
			Enabled:    true,
			Key:        opsAlertEvaluatorLeaderLockKeyDefault,
			TTLSeconds: int(opsAlertEvaluatorLeaderLockTTLDefault.Seconds()),
		},
		Silencing: OpsAlertSilencingSettings{
			Enabled:            false,
			GlobalUntilRFC3339: "",
			GlobalReason:       "",
			Entries:            []OpsAlertSilenceEntry{},
		},
	}
}

func normalizeOpsDistributedLockSettings(s *OpsDistributedLockSettings, defaultKey string, defaultTTLSeconds int) {
	if s == nil {
		return
	}
	s.Key = strings.TrimSpace(s.Key)
	if s.Key == "" {
		s.Key = defaultKey
	}
	if s.TTLSeconds <= 0 {
		s.TTLSeconds = defaultTTLSeconds
	}
}

func normalizeOpsAlertSilencingSettings(s *OpsAlertSilencingSettings) {
	if s == nil {
		return
	}
	s.GlobalUntilRFC3339 = strings.TrimSpace(s.GlobalUntilRFC3339)
	s.GlobalReason = strings.TrimSpace(s.GlobalReason)
	if s.Entries == nil {
		s.Entries = []OpsAlertSilenceEntry{}
	}
	for i := range s.Entries {
		s.Entries[i].UntilRFC3339 = strings.TrimSpace(s.Entries[i].UntilRFC3339)
		s.Entries[i].Reason = strings.TrimSpace(s.Entries[i].Reason)
	}
}

func validateOpsDistributedLockSettings(s OpsDistributedLockSettings) error {
	if strings.TrimSpace(s.Key) == "" {
		return errors.New("distributed_lock.key is required")
	}
	if s.TTLSeconds <= 0 || s.TTLSeconds > int((24*time.Hour).Seconds()) {
		return errors.New("distributed_lock.ttl_seconds must be between 1 and 86400")
	}
	return nil
}

func validateOpsAlertSilencingSettings(s OpsAlertSilencingSettings) error {
	parse := func(raw string) error {
		if strings.TrimSpace(raw) == "" {
			return nil
		}
		if _, err := time.Parse(time.RFC3339, raw); err != nil {
			return errors.New("silencing time must be RFC3339")
		}
		return nil
	}

	if err := parse(s.GlobalUntilRFC3339); err != nil {
		return err
	}
	for _, entry := range s.Entries {
		if strings.TrimSpace(entry.UntilRFC3339) == "" {
			return errors.New("silencing.entries.until_rfc3339 is required")
		}
		if _, err := time.Parse(time.RFC3339, entry.UntilRFC3339); err != nil {
			return errors.New("silencing.entries.until_rfc3339 must be RFC3339")
		}
	}
	return nil
}

func (s *OpsService) GetOpsAlertRuntimeSettings(ctx context.Context) (*OpsAlertRuntimeSettings, error) {
	defaultCfg := defaultOpsAlertRuntimeSettings()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsAlertRuntimeSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			if b, mErr := json.Marshal(defaultCfg); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyOpsAlertRuntimeSettings, string(b))
			}
			return defaultCfg, nil
		}
		return nil, err
	}

	cfg := &OpsAlertRuntimeSettings{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return defaultCfg, nil
	}

	if cfg.EvaluationIntervalSeconds <= 0 {
		cfg.EvaluationIntervalSeconds = defaultCfg.EvaluationIntervalSeconds
	}
	normalizeOpsDistributedLockSettings(&cfg.DistributedLock, opsAlertEvaluatorLeaderLockKeyDefault, defaultCfg.DistributedLock.TTLSeconds)
	normalizeOpsAlertSilencingSettings(&cfg.Silencing)

	return cfg, nil
}

func (s *OpsService) UpdateOpsAlertRuntimeSettings(ctx context.Context, cfg *OpsAlertRuntimeSettings) (*OpsAlertRuntimeSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("invalid config")
	}

	if cfg.EvaluationIntervalSeconds < 1 || cfg.EvaluationIntervalSeconds > int((24*time.Hour).Seconds()) {
		return nil, errors.New("evaluation_interval_seconds must be between 1 and 86400")
	}
	if cfg.DistributedLock.Enabled {
		if err := validateOpsDistributedLockSettings(cfg.DistributedLock); err != nil {
			return nil, err
		}
	}
	if cfg.Silencing.Enabled {
		if err := validateOpsAlertSilencingSettings(cfg.Silencing); err != nil {
			return nil, err
		}
	}

	defaultCfg := defaultOpsAlertRuntimeSettings()
	normalizeOpsDistributedLockSettings(&cfg.DistributedLock, opsAlertEvaluatorLeaderLockKeyDefault, defaultCfg.DistributedLock.TTLSeconds)
	normalizeOpsAlertSilencingSettings(&cfg.Silencing)

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpsAlertRuntimeSettings, string(raw)); err != nil {
		return nil, err
	}

	// Return a fresh copy (avoid callers holding pointers into internal slices that may be mutated).
	updated := &OpsAlertRuntimeSettings{}
	_ = json.Unmarshal(raw, updated)
	return updated, nil
}

// =========================
// Advanced settings
// =========================

func defaultOpsAdvancedSettings() *OpsAdvancedSettings {
	return &OpsAdvancedSettings{
		DataRetention: OpsDataRetentionSettings{
			CleanupEnabled:             false,
			CleanupSchedule:            "0 2 * * *",
			ErrorLogRetentionDays:      30,
			MinuteMetricsRetentionDays: 30,
			HourlyMetricsRetentionDays: 30,
		},
		Aggregation: OpsAggregationSettings{
			AggregationEnabled: false,
		},
		IgnoreCountTokensErrors:         true,  // count_tokens 404 是预期行为，默认忽略
		IgnoreContextCanceled:           true,  // Default to true - client disconnects are not errors
		IgnoreNoAvailableAccounts:       false, // Default to false - this is a real routing issue
		IgnoreInsufficientBalanceErrors: false, // 默认不忽略，余额不足可能需要关注
		DisplayOpenAITokenStats:         false,
		DisplayAlertEvents:              true,
		AutoRefreshEnabled:              false,
		AutoRefreshIntervalSec:          30,
	}
}

func normalizeOpsAdvancedSettings(cfg *OpsAdvancedSettings) {
	if cfg == nil {
		return
	}
	cfg.DataRetention.CleanupSchedule = strings.TrimSpace(cfg.DataRetention.CleanupSchedule)
	if cfg.DataRetention.CleanupSchedule == "" {
		cfg.DataRetention.CleanupSchedule = "0 2 * * *"
	}
	if cfg.DataRetention.ErrorLogRetentionDays < 0 {
		cfg.DataRetention.ErrorLogRetentionDays = 30
	}
	if cfg.DataRetention.MinuteMetricsRetentionDays < 0 {
		cfg.DataRetention.MinuteMetricsRetentionDays = 30
	}
	if cfg.DataRetention.HourlyMetricsRetentionDays < 0 {
		cfg.DataRetention.HourlyMetricsRetentionDays = 30
	}
	// Normalize auto refresh interval (default 30 seconds)
	if cfg.AutoRefreshIntervalSec <= 0 {
		cfg.AutoRefreshIntervalSec = 30
	}
}

func validateOpsAdvancedSettings(cfg *OpsAdvancedSettings) error {
	if cfg == nil {
		return errors.New("invalid config")
	}
	if cfg.DataRetention.ErrorLogRetentionDays < 0 || cfg.DataRetention.ErrorLogRetentionDays > 365 {
		return errors.New("error_log_retention_days must be between 0 and 365")
	}
	if cfg.DataRetention.MinuteMetricsRetentionDays < 0 || cfg.DataRetention.MinuteMetricsRetentionDays > 365 {
		return errors.New("minute_metrics_retention_days must be between 0 and 365")
	}
	if cfg.DataRetention.HourlyMetricsRetentionDays < 0 || cfg.DataRetention.HourlyMetricsRetentionDays > 365 {
		return errors.New("hourly_metrics_retention_days must be between 0 and 365")
	}
	if cfg.AutoRefreshIntervalSec < 15 || cfg.AutoRefreshIntervalSec > 300 {
		return errors.New("auto_refresh_interval_seconds must be between 15 and 300")
	}
	return nil
}

func (s *OpsService) GetOpsAdvancedSettings(ctx context.Context) (*OpsAdvancedSettings, error) {
	defaultCfg := defaultOpsAdvancedSettings()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsAdvancedSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			if b, mErr := json.Marshal(defaultCfg); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyOpsAdvancedSettings, string(b))
			}
			return defaultCfg, nil
		}
		return nil, err
	}

	cfg := defaultOpsAdvancedSettings()
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return defaultCfg, nil
	}

	normalizeOpsAdvancedSettings(cfg)
	return cfg, nil
}

func (s *OpsService) UpdateOpsAdvancedSettings(ctx context.Context, cfg *OpsAdvancedSettings) (*OpsAdvancedSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("invalid config")
	}

	if err := validateOpsAdvancedSettings(cfg); err != nil {
		return nil, err
	}

	normalizeOpsAdvancedSettings(cfg)
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpsAdvancedSettings, string(raw)); err != nil {
		return nil, err
	}

	updated := &OpsAdvancedSettings{}
	_ = json.Unmarshal(raw, updated)
	return updated, nil
}

// =========================
// Metric thresholds
// =========================

const SettingKeyOpsMetricThresholds = "ops_metric_thresholds"

func defaultOpsMetricThresholds() *OpsMetricThresholds {
	slaMin := 99.5
	ttftMax := 500.0
	reqErrMax := 5.0
	upstreamErrMax := 5.0
	return &OpsMetricThresholds{
		SLAPercentMin:               &slaMin,
		TTFTp99MsMax:                &ttftMax,
		RequestErrorRatePercentMax:  &reqErrMax,
		UpstreamErrorRatePercentMax: &upstreamErrMax,
	}
}

func (s *OpsService) GetMetricThresholds(ctx context.Context) (*OpsMetricThresholds, error) {
	defaultCfg := defaultOpsMetricThresholds()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsMetricThresholds)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			if b, mErr := json.Marshal(defaultCfg); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyOpsMetricThresholds, string(b))
			}
			return defaultCfg, nil
		}
		return nil, err
	}

	cfg := &OpsMetricThresholds{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return defaultCfg, nil
	}

	return cfg, nil
}

func (s *OpsService) UpdateMetricThresholds(ctx context.Context, cfg *OpsMetricThresholds) (*OpsMetricThresholds, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("invalid config")
	}

	// Validate thresholds
	if cfg.SLAPercentMin != nil && (*cfg.SLAPercentMin < 0 || *cfg.SLAPercentMin > 100) {
		return nil, errors.New("sla_percent_min must be between 0 and 100")
	}
	if cfg.TTFTp99MsMax != nil && *cfg.TTFTp99MsMax < 0 {
		return nil, errors.New("ttft_p99_ms_max must be >= 0")
	}
	if cfg.RequestErrorRatePercentMax != nil && (*cfg.RequestErrorRatePercentMax < 0 || *cfg.RequestErrorRatePercentMax > 100) {
		return nil, errors.New("request_error_rate_percent_max must be between 0 and 100")
	}
	if cfg.UpstreamErrorRatePercentMax != nil && (*cfg.UpstreamErrorRatePercentMax < 0 || *cfg.UpstreamErrorRatePercentMax > 100) {
		return nil, errors.New("upstream_error_rate_percent_max must be between 0 and 100")
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpsMetricThresholds, string(raw)); err != nil {
		return nil, err
	}

	updated := &OpsMetricThresholds{}
	_ = json.Unmarshal(raw, updated)
	return updated, nil
}
