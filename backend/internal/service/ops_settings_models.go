package service

// Ops settings models stored in DB `settings` table (JSON blobs).

type OpsEmailNotificationConfig struct {
	Alert  OpsEmailAlertConfig  `json:"alert"`
	Report OpsEmailReportConfig `json:"report"`
}

type OpsEmailAlertConfig struct {
	Enabled               bool     `json:"enabled"`
	Recipients            []string `json:"recipients"`
	MinSeverity           string   `json:"min_severity"`
	RateLimitPerHour      int      `json:"rate_limit_per_hour"`
	BatchingWindowSeconds int      `json:"batching_window_seconds"`
	IncludeResolvedAlerts bool     `json:"include_resolved_alerts"`
}

type OpsEmailReportConfig struct {
	Enabled                         bool     `json:"enabled"`
	Recipients                      []string `json:"recipients"`
	DailySummaryEnabled             bool     `json:"daily_summary_enabled"`
	DailySummarySchedule            string   `json:"daily_summary_schedule"`
	WeeklySummaryEnabled            bool     `json:"weekly_summary_enabled"`
	WeeklySummarySchedule           string   `json:"weekly_summary_schedule"`
	ErrorDigestEnabled              bool     `json:"error_digest_enabled"`
	ErrorDigestSchedule             string   `json:"error_digest_schedule"`
	ErrorDigestMinCount             int      `json:"error_digest_min_count"`
	AccountHealthEnabled            bool     `json:"account_health_enabled"`
	AccountHealthSchedule           string   `json:"account_health_schedule"`
	AccountHealthErrorRateThreshold float64  `json:"account_health_error_rate_threshold"`
}

// OpsEmailNotificationConfigUpdateRequest allows partial updates, while the
// frontend can still send the full config shape.
type OpsEmailNotificationConfigUpdateRequest struct {
	Alert  *OpsEmailAlertConfig  `json:"alert"`
	Report *OpsEmailReportConfig `json:"report"`
}

type OpsWebhookNotificationConfig struct {
	Feishu   OpsFeishuNotificationConfig   `json:"feishu"`
	DingTalk OpsDingTalkNotificationConfig `json:"dingtalk"`
	Telegram OpsTelegramNotificationConfig `json:"telegram"`
}

type OpsFeishuNotificationConfig struct {
	Enabled              bool   `json:"enabled"`
	Name                 string `json:"name"`
	WebhookURL           string `json:"webhook_url,omitempty"`
	WebhookURLConfigured bool   `json:"webhook_url_configured"`
	Secret               string `json:"secret,omitempty"`
	SecretConfigured     bool   `json:"secret_configured"`
	MinSeverity          string `json:"min_severity"`
	RateLimitPerHour     int    `json:"rate_limit_per_hour"`
}

type OpsDingTalkNotificationConfig struct {
	Enabled              bool   `json:"enabled"`
	Name                 string `json:"name"`
	WebhookURL           string `json:"webhook_url,omitempty"`
	WebhookURLConfigured bool   `json:"webhook_url_configured"`
	Secret               string `json:"secret,omitempty"`
	SecretConfigured     bool   `json:"secret_configured"`
	MinSeverity          string `json:"min_severity"`
	RateLimitPerHour     int    `json:"rate_limit_per_hour"`
}

type OpsTelegramNotificationConfig struct {
	Enabled            bool   `json:"enabled"`
	Name               string `json:"name"`
	BotToken           string `json:"bot_token,omitempty"`
	BotTokenConfigured bool   `json:"bot_token_configured"`
	ChatID             string `json:"chat_id"`
	MinSeverity        string `json:"min_severity"`
	RateLimitPerHour   int    `json:"rate_limit_per_hour"`
}

type OpsWebhookNotificationConfigUpdateRequest struct {
	Feishu   *OpsFeishuNotificationConfig   `json:"feishu"`
	DingTalk *OpsDingTalkNotificationConfig `json:"dingtalk"`
	Telegram *OpsTelegramNotificationConfig `json:"telegram"`
}

type OpsWebhookNotificationTestRequest struct {
	Channel string `json:"channel"`
}

type OpsWebhookNotificationTestResponse struct {
	Channel string `json:"channel"`
	Sent    bool   `json:"sent"`
}

type OpsDistributedLockSettings struct {
	Enabled    bool   `json:"enabled"`
	Key        string `json:"key"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type OpsAlertSilenceEntry struct {
	RuleID     *int64   `json:"rule_id,omitempty"`
	Severities []string `json:"severities,omitempty"`

	UntilRFC3339 string `json:"until_rfc3339"`
	Reason       string `json:"reason"`
}

type OpsAlertSilencingSettings struct {
	Enabled bool `json:"enabled"`

	GlobalUntilRFC3339 string `json:"global_until_rfc3339"`
	GlobalReason       string `json:"global_reason"`

	Entries []OpsAlertSilenceEntry `json:"entries,omitempty"`
}

type OpsMetricThresholds struct {
	SLAPercentMin               *float64 `json:"sla_percent_min,omitempty"`                 // SLA低于此值变红
	TTFTp99MsMax                *float64 `json:"ttft_p99_ms_max,omitempty"`                 // TTFT P99高于此值变红
	RequestErrorRatePercentMax  *float64 `json:"request_error_rate_percent_max,omitempty"`  // 请求错误率高于此值变红
	UpstreamErrorRatePercentMax *float64 `json:"upstream_error_rate_percent_max,omitempty"` // 上游错误率高于此值变红
}

type OpsRuntimeLogConfig struct {
	Level           string         `json:"level"`
	EnableSampling  bool           `json:"enable_sampling"`
	SamplingInitial int            `json:"sampling_initial"`
	SamplingNext    int            `json:"sampling_thereafter"`
	Caller          bool           `json:"caller"`
	StacktraceLevel string         `json:"stacktrace_level"`
	RetentionDays   int            `json:"retention_days"`
	Source          string         `json:"source,omitempty"`
	UpdatedAt       string         `json:"updated_at,omitempty"`
	UpdatedByUserID int64          `json:"updated_by_user_id,omitempty"`
	Extra           map[string]any `json:"extra,omitempty"`
}

type OpsAlertRuntimeSettings struct {
	EvaluationIntervalSeconds int `json:"evaluation_interval_seconds"`

	DistributedLock OpsDistributedLockSettings `json:"distributed_lock"`
	Silencing       OpsAlertSilencingSettings  `json:"silencing"`
	Thresholds      OpsMetricThresholds        `json:"thresholds"` // 指标阈值配置
}

// OpsAdvancedSettings stores advanced ops configuration (data retention, aggregation).
type OpsAdvancedSettings struct {
	DataRetention                   OpsDataRetentionSettings `json:"data_retention"`
	Aggregation                     OpsAggregationSettings   `json:"aggregation"`
	IgnoreCountTokensErrors         bool                     `json:"ignore_count_tokens_errors"`
	IgnoreContextCanceled           bool                     `json:"ignore_context_canceled"`
	IgnoreNoAvailableAccounts       bool                     `json:"ignore_no_available_accounts"`
	IgnoreInvalidApiKeyErrors       bool                     `json:"ignore_invalid_api_key_errors"`
	IgnoreInsufficientBalanceErrors bool                     `json:"ignore_insufficient_balance_errors"`
	DisplayOpenAITokenStats         bool                     `json:"display_openai_token_stats"`
	DisplayAlertEvents              bool                     `json:"display_alert_events"`
	AutoRefreshEnabled              bool                     `json:"auto_refresh_enabled"`
	AutoRefreshIntervalSec          int                      `json:"auto_refresh_interval_seconds"`
}

type OpsDataRetentionSettings struct {
	CleanupEnabled             bool   `json:"cleanup_enabled"`
	CleanupSchedule            string `json:"cleanup_schedule"`
	ErrorLogRetentionDays      int    `json:"error_log_retention_days"`
	MinuteMetricsRetentionDays int    `json:"minute_metrics_retention_days"`
	HourlyMetricsRetentionDays int    `json:"hourly_metrics_retention_days"`
}

type OpsAggregationSettings struct {
	AggregationEnabled bool `json:"aggregation_enabled"`
}
