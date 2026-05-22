package service

import "github.com/bozhouDev/DragonCode-sub2api/internal/domain"

// Status constants
const (
	StatusActive   = domain.StatusActive
	StatusDisabled = domain.StatusDisabled
	StatusError    = domain.StatusError
	StatusUnused   = domain.StatusUnused
	StatusUsed     = domain.StatusUsed
	StatusExpired  = domain.StatusExpired
)

// Role constants
const (
	RoleAdmin = domain.RoleAdmin
	RoleUser  = domain.RoleUser
	RoleAgent = domain.RoleAgent
)

// Platform constants
const (
	PlatformAnthropic   = domain.PlatformAnthropic
	PlatformOpenAI      = domain.PlatformOpenAI
	PlatformGemini      = domain.PlatformGemini
	PlatformAntigravity = domain.PlatformAntigravity
	PlatformGPTImage    = domain.PlatformGPTImage
)

// Account type constants
const (
	AccountTypeOAuth      = domain.AccountTypeOAuth      // OAuth类型账号（full scope: profile + inference）
	AccountTypeSetupToken = domain.AccountTypeSetupToken // Setup Token类型账号（inference only scope）
	AccountTypeAPIKey     = domain.AccountTypeAPIKey     // API Key类型账号
	AccountTypeUpstream   = domain.AccountTypeUpstream   // 上游透传类型账号（通过 Base URL + API Key 连接上游）
	AccountTypeBedrock    = domain.AccountTypeBedrock    // AWS Bedrock 类型账号（通过 SigV4 签名或 API Key 连接 Bedrock，由 credentials.auth_mode 区分）
)

// Redeem type constants
const (
	RedeemTypeBalance      = domain.RedeemTypeBalance
	RedeemTypeConcurrency  = domain.RedeemTypeConcurrency
	RedeemTypeSubscription = domain.RedeemTypeSubscription
	RedeemTypeInvitation   = domain.RedeemTypeInvitation
)

// PromoCode status constants
const (
	PromoCodeStatusActive   = domain.PromoCodeStatusActive
	PromoCodeStatusDisabled = domain.PromoCodeStatusDisabled
)

// Admin adjustment type constants
const (
	AdjustmentTypeAdminBalance     = domain.AdjustmentTypeAdminBalance     // 管理员调整余额
	AdjustmentTypeAdminConcurrency = domain.AdjustmentTypeAdminConcurrency // 管理员调整并发数
)

// Commission type constants
const (
	CommissionTypeConsumption                     = domain.CommissionTypeConsumption
	CommissionTypeFirstRechargeInvitee            = domain.CommissionTypeFirstRechargeInvitee
	CommissionTypeFirstRechargeFriendInvitee      = domain.CommissionTypeFirstRechargeFriendInvitee
	CommissionTypeFirstRechargeReferral           = domain.CommissionTypeFirstRechargeReferral
	CommissionTypeInviteActivityRegistrationBonus = domain.CommissionTypeInviteActivityRegistrationBonus
)

// Group subscription type constants
const (
	SubscriptionTypeStandard     = domain.SubscriptionTypeStandard     // 标准计费模式（按余额扣费）
	SubscriptionTypeSubscription = domain.SubscriptionTypeSubscription // 订阅模式（按限额控制）
)

// Subscription status constants
const (
	SubscriptionStatusActive    = domain.SubscriptionStatusActive
	SubscriptionStatusExpired   = domain.SubscriptionStatusExpired
	SubscriptionStatusSuspended = domain.SubscriptionStatusSuspended
)

// LinuxDoConnectSyntheticEmailDomain 是 LinuxDo Connect 用户的合成邮箱后缀（RFC 保留域名）。
const LinuxDoConnectSyntheticEmailDomain = "@linuxdo-connect.invalid"

// OIDCConnectSyntheticEmailDomain 是 OIDC 用户的合成邮箱后缀（RFC 保留域名）。
const OIDCConnectSyntheticEmailDomain = "@oidc-connect.invalid"

// Setting keys
const (
	// 注册设置
	SettingKeyRegistrationEnabled              = "registration_enabled"                // 是否开放注册
	SettingKeyEmailVerifyEnabled               = "email_verify_enabled"                // 是否开启邮件验证
	SettingKeyRegistrationEmailSuffixWhitelist = "registration_email_suffix_whitelist" // 注册邮箱后缀白名单（JSON 数组）
	SettingKeyPromoCodeEnabled                 = "promo_code_enabled"                  // 是否启用优惠码功能
	SettingKeyPasswordResetEnabled             = "password_reset_enabled"              // 是否启用忘记密码功能（需要先开启邮件验证）
	SettingKeyFrontendURL                      = "frontend_url"                        // 前端基础URL，用于生成邮件中的重置密码链接
	SettingKeyInvitationCodeEnabled            = "invitation_code_enabled"             // 是否启用邀请码注册

	// 邮件服务设置
	SettingKeySMTPHost            = "smtp_host"             // SMTP服务器地址
	SettingKeySMTPPort            = "smtp_port"             // SMTP端口
	SettingKeySMTPUsername        = "smtp_username"         // SMTP用户名
	SettingKeySMTPPassword        = "smtp_password"         // SMTP密码（加密存储）
	SettingKeySMTPFrom            = "smtp_from"             // 发件人地址
	SettingKeySMTPFromName        = "smtp_from_name"        // 发件人名称
	SettingKeySMTPUseTLS          = "smtp_use_tls"          // 是否使用TLS
	SettingKeyFeedbackNotifyEmail = "feedback_notify_email" // 反馈通知邮箱

	// Cloudflare Turnstile 设置
	SettingKeyTurnstileEnabled   = "turnstile_enabled"    // 是否启用 Turnstile 验证
	SettingKeyTurnstileSiteKey   = "turnstile_site_key"   // Turnstile Site Key
	SettingKeyTurnstileSecretKey = "turnstile_secret_key" // Turnstile Secret Key

	// TOTP 双因素认证设置
	SettingKeyTotpEnabled = "totp_enabled" // 是否启用 TOTP 2FA 功能

	// LinuxDo Connect OAuth 登录设置
	SettingKeyLinuxDoConnectEnabled      = "linuxdo_connect_enabled"
	SettingKeyLinuxDoConnectClientID     = "linuxdo_connect_client_id"
	SettingKeyLinuxDoConnectClientSecret = "linuxdo_connect_client_secret"
	SettingKeyLinuxDoConnectRedirectURL  = "linuxdo_connect_redirect_url"

	// Generic OIDC OAuth 登录设置
	SettingKeyOIDCConnectEnabled              = "oidc_connect_enabled"
	SettingKeyOIDCConnectProviderName         = "oidc_connect_provider_name"
	SettingKeyOIDCConnectClientID             = "oidc_connect_client_id"
	SettingKeyOIDCConnectClientSecret         = "oidc_connect_client_secret"
	SettingKeyOIDCConnectIssuerURL            = "oidc_connect_issuer_url"
	SettingKeyOIDCConnectDiscoveryURL         = "oidc_connect_discovery_url"
	SettingKeyOIDCConnectAuthorizeURL         = "oidc_connect_authorize_url"
	SettingKeyOIDCConnectTokenURL             = "oidc_connect_token_url"
	SettingKeyOIDCConnectUserInfoURL          = "oidc_connect_userinfo_url"
	SettingKeyOIDCConnectJWKSURL              = "oidc_connect_jwks_url"
	SettingKeyOIDCConnectScopes               = "oidc_connect_scopes"
	SettingKeyOIDCConnectRedirectURL          = "oidc_connect_redirect_url"
	SettingKeyOIDCConnectFrontendRedirectURL  = "oidc_connect_frontend_redirect_url"
	SettingKeyOIDCConnectTokenAuthMethod      = "oidc_connect_token_auth_method"
	SettingKeyOIDCConnectUsePKCE              = "oidc_connect_use_pkce"
	SettingKeyOIDCConnectValidateIDToken      = "oidc_connect_validate_id_token"
	SettingKeyOIDCConnectAllowedSigningAlgs   = "oidc_connect_allowed_signing_algs"
	SettingKeyOIDCConnectClockSkewSeconds     = "oidc_connect_clock_skew_seconds"
	SettingKeyOIDCConnectRequireEmailVerified = "oidc_connect_require_email_verified"
	SettingKeyOIDCConnectUserInfoEmailPath    = "oidc_connect_userinfo_email_path"
	SettingKeyOIDCConnectUserInfoIDPath       = "oidc_connect_userinfo_id_path"
	SettingKeyOIDCConnectUserInfoUsernamePath = "oidc_connect_userinfo_username_path"

	// GitHub OAuth 登录设置
	SettingKeyGitHubOAuthEnabled             = "github_oauth_enabled"
	SettingKeyGitHubOAuthClientID            = "github_oauth_client_id"
	SettingKeyGitHubOAuthClientSecret        = "github_oauth_client_secret"
	SettingKeyGitHubOAuthRedirectURL         = "github_oauth_redirect_url"
	SettingKeyGitHubOAuthFrontendRedirectURL = "github_oauth_frontend_redirect_url"

	// OEM设置
	SettingKeySiteName                    = "site_name"                      // 网站名称
	SettingKeySiteLogo                    = "site_logo"                      // 网站Logo (base64)
	SettingKeySiteSubtitle                = "site_subtitle"                  // 网站副标题
	SettingKeyAPIBaseURL                  = "api_base_url"                   // API端点地址（用于客户端配置和导入）
	SettingKeyContactInfo                 = "contact_info"                   // 客服联系方式
	SettingKeyTechSupportQRCode           = "tech_support_qrcode"            // 技术客服二维码图片 URL（负责安装及环境配置问题）
	SettingKeyAfterSalesQRCode            = "after_sales_qrcode"             // 售后客服二维码图片 URL（负责账户充值、推广等售后事宜）
	SettingKeyDocURL                      = "doc_url"                        // 文档链接
	SettingKeyChatbotURL                  = "chatbot_url"                    // Chatbot 前端地址（用于 SSO 跳转）
	SettingKeyHomeContent                 = "home_content"                   // 首页内容（支持 Markdown/HTML，或 URL 作为 iframe src）
	SettingKeyLandingReportsEnabled       = "landing_reports_enabled"        // 官网首页是否展示模型检测报告
	SettingKeyLandingPricingProMultiplier = "landing_pricing_pro_multiplier" // 官网模型价格展示：Pro 分组展示倍率（¥ / $）
	SettingKeyLandingPricingMaxMultiplier = "landing_pricing_max_multiplier" // 官网模型价格展示：Max 分组展示倍率（¥ / $）
	SettingKeyLandingPricingExchangeRate  = "landing_pricing_exchange_rate"  // 官网模型价格展示：折扣参考汇率（¥ / $）
	SettingKeyHideCcsImportButton         = "hide_ccs_import_button"         // 是否隐藏 API Keys 页面的导入 CCS 按钮
	SettingKeyPurchaseSubscriptionEnabled = "purchase_subscription_enabled"  // 是否展示"购买订阅"页面入口
	SettingKeyPurchaseSubscriptionURL     = "purchase_subscription_url"      // "购买订阅"页面 URL（作为 iframe src）
	SettingKeyCardShopEnabled             = "card_shop_enabled"              // 是否启用外部卡密商城充值入口
	SettingKeyCardShopProducts            = "card_shop_products"             // 外部卡密商城固定面额商品列表（JSON）
	SettingKeyInvoiceManagementEnabled    = "invoice_management_enabled"     // 是否向用户展示发票管理入口
	SettingKeyFeedbackManagementEnabled   = "feedback_management_enabled"    // 是否向用户展示反馈入口
	SettingKeyGroupCacheHitRateEnabled    = "group_cache_hit_rate_enabled"   // 是否向用户展示分组 7 日缓存率
	SettingKeySoraClientEnabled           = "sora_client_enabled"            // 是否启用 Sora 客户端（管理员手动控制）
	SettingKeyTableDefaultPageSize        = "table_default_page_size"        // 表格默认每页条数
	SettingKeyTablePageSizeOptions        = "table_page_size_options"        // 表格可选每页条数（JSON 数组）
	SettingKeyCustomMenuItems             = "custom_menu_items"              // 自定义菜单项（JSON 数组）
	SettingKeyCustomEndpoints             = "custom_endpoints"               // 自定义端点列表（JSON 数组）

	// 默认配置
	SettingKeyDefaultConcurrency   = "default_concurrency"   // 新用户默认并发量
	SettingKeyDefaultBalance       = "default_balance"       // 新用户默认余额
	SettingKeyDefaultSubscriptions = "default_subscriptions" // 新用户默认订阅列表（JSON）

	// 管理员 API Key
	SettingKeyAdminAPIKey = "admin_api_key" // 全局管理员 API Key（用于外部系统集成）

	// Public payment switch
	SettingPaymentEnabled = "payment_enabled"

	// Stripe 支付设置
	SettingKeyStripeEnabled       = "stripe_enabled"
	SettingKeyStripeSecretKey     = "stripe_secret_key"
	SettingKeyStripeWebhookSecret = "stripe_webhook_secret"

	// 支付宝当面付设置
	SettingKeyAlipayEnabled    = "alipay_enabled"
	SettingKeyAlipayAppID      = "alipay_app_id"
	SettingKeyAlipayPrivateKey = "alipay_private_key" // RSA2 应用私钥
	SettingKeyAlipayPublicKey  = "alipay_public_key"  // 支付宝公钥
	SettingKeyAlipayNotifyURL  = "alipay_notify_url"  // 异步通知回调 URL
	SettingKeyAlipayPlans      = "alipay_plans"       // JSON array of SubscriptionPlan

	// 虎皮椒聚合支付（余额充值）
	SettingKeyXunhuAlipayEnabled = "xunhu_alipay_enabled" // 是否启用虎皮椒支付宝充值
	SettingKeyXunhuAlipayAppID   = "xunhu_alipay_appid"   // 虎皮椒支付宝 AppID
	SettingKeyXunhuAlipayKey     = "xunhu_alipay_key"     // 虎皮椒支付宝密钥
	SettingKeyXunhuWechatEnabled = "xunhu_wechat_enabled" // 是否启用虎皮椒微信充值
	SettingKeyXunhuWechatAppID   = "xunhu_wechat_appid"   // 虎皮椒微信 AppID
	SettingKeyXunhuWechatKey     = "xunhu_wechat_key"     // 虎皮椒微信密钥
	SettingKeyXunhuNotifyURL     = "xunhu_notify_url"     // 虎皮椒回调地址

	// Gemini 配额策略（JSON）
	SettingKeyGeminiQuotaPolicy = "gemini_quota_policy"

	// Model fallback settings
	SettingKeyEnableModelFallback      = "enable_model_fallback"
	SettingKeyFallbackModelAnthropic   = "fallback_model_anthropic"
	SettingKeyFallbackModelOpenAI      = "fallback_model_openai"
	SettingKeyFallbackModelGemini      = "fallback_model_gemini"
	SettingKeyFallbackModelAntigravity = "fallback_model_antigravity"

	// Request identity patch (Claude -> Gemini systemInstruction injection)
	SettingKeyEnableIdentityPatch = "enable_identity_patch"
	SettingKeyIdentityPatchPrompt = "identity_patch_prompt"

	// =========================
	// Ops Monitoring (vNext)
	// =========================

	// SettingKeyOpsMonitoringEnabled is a DB-backed soft switch to enable/disable ops module at runtime.
	SettingKeyOpsMonitoringEnabled = "ops_monitoring_enabled"

	// SettingKeyOpsRealtimeMonitoringEnabled controls realtime features (e.g. WS/QPS push).
	SettingKeyOpsRealtimeMonitoringEnabled = "ops_realtime_monitoring_enabled"

	// SettingKeyOpsQueryModeDefault controls the default query mode for ops dashboard (auto/raw/preagg).
	SettingKeyOpsQueryModeDefault = "ops_query_mode_default"

	// SettingKeyOpsEmailNotificationConfig stores JSON config for ops email notifications.
	SettingKeyOpsEmailNotificationConfig = "ops_email_notification_config"

	// SettingKeyOpsWebhookNotificationConfig stores JSON config for ops Feishu/Telegram webhook notifications.
	SettingKeyOpsWebhookNotificationConfig = "ops_webhook_notification_config"

	// SettingKeyOpsAlertRuntimeSettings stores JSON config for ops alert evaluator runtime settings.
	SettingKeyOpsAlertRuntimeSettings = "ops_alert_runtime_settings"

	// SettingKeyOpsMetricsIntervalSeconds controls the ops metrics collector interval (>=60).
	SettingKeyOpsMetricsIntervalSeconds = "ops_metrics_interval_seconds"

	// SettingKeyOpsAdvancedSettings stores JSON config for ops advanced settings (data retention, aggregation).
	SettingKeyOpsAdvancedSettings = "ops_advanced_settings"

	// SettingKeyOpsRuntimeLogConfig stores JSON config for runtime log settings.
	SettingKeyOpsRuntimeLogConfig = "ops_runtime_log_config"

	// =========================
	// Overload Cooldown (529)
	// =========================

	// SettingKeyOverloadCooldownSettings stores JSON config for 529 overload cooldown handling.
	SettingKeyOverloadCooldownSettings = "overload_cooldown_settings"

	// =========================
	// Stream Timeout Handling
	// =========================

	// SettingKeyStreamTimeoutSettings stores JSON config for stream timeout handling.
	SettingKeyStreamTimeoutSettings = "stream_timeout_settings"

	// =========================
	// Request Rectifier (请求整流器)
	// =========================

	// SettingKeyRectifierSettings stores JSON config for rectifier settings (thinking signature + budget).
	SettingKeyRectifierSettings = "rectifier_settings"

	// =========================
	// Beta Policy Settings
	// =========================

	// SettingKeyBetaPolicySettings stores JSON config for beta policy rules.
	SettingKeyBetaPolicySettings = "beta_policy_settings"

	// SettingKeyOpenAIFastPolicySettings stores JSON config for OpenAI service_tier policy rules.
	SettingKeyOpenAIFastPolicySettings = "openai_fast_policy_settings"

	// =========================
	// Claude Code Version Check
	// =========================

	// SettingKeyMinClaudeCodeVersion 最低 Claude Code 版本号要求 (semver, 如 "2.1.0"，空值=不检查)
	SettingKeyMinClaudeCodeVersion = "min_claude_code_version"

	// SettingKeyMaxClaudeCodeVersion 最高 Claude Code 版本号限制 (semver, 如 "3.0.0"，空值=不检查)
	SettingKeyMaxClaudeCodeVersion = "max_claude_code_version"

	// SettingKeyAllowUngroupedKeyScheduling 允许未分组 API Key 调度（默认 false：未分组 Key 返回 403）
	SettingKeyAllowUngroupedKeyScheduling = "allow_ungrouped_key_scheduling"

	// SettingKeyBackendModeEnabled Backend 模式：禁用用户注册和自助服务，仅管理员可登录
	SettingKeyBackendModeEnabled = "backend_mode_enabled"

	// Gateway Forwarding Behavior
	SettingKeyEnableFingerprintUnification       = "enable_fingerprint_unification"
	SettingKeyEnableMetadataPassthrough          = "enable_metadata_passthrough"
	SettingKeyEnableCCHSigning                   = "enable_cch_signing"
	SettingKeyEnableAnthropicCacheTTL1hInjection = "enable_anthropic_cache_ttl_1h_injection"
	SettingKeyAnthropicCachePolicyMode           = "anthropic_cache_policy_mode"
	SettingKeyAnthropicCachePolicyVersion        = "anthropic_cache_policy_version"
	SettingKeyAnthropicCachePolicyPhase          = "anthropic_cache_policy_phase"
	SettingKeyAnthropicCachePolicyTokenThreshold = "anthropic_cache_policy_token_threshold"
	SettingKeyAnthropicCachePolicyIntervalMins   = "anthropic_cache_policy_interval_threshold_minutes"

	// Balance Alert
	SettingKeyBalanceAlertEnabled          = "balance_alert_enabled"
	SettingKeyBalanceAlertDefaultThreshold = "balance_alert_default_threshold"

	// Account Quota Notification
	SettingKeyAccountQuotaNotifyEnabled = "account_quota_notify_enabled"
	SettingKeyAccountQuotaNotifyEmails  = "account_quota_notify_emails"

	// Web Search Emulation
	SettingKeyWebSearchEmulationConfig = "web_search_emulation_config"
)

// AdminAPIKeyPrefix is the prefix for admin API keys (distinct from user "sk-" keys).
const AdminAPIKeyPrefix = "admin-"
