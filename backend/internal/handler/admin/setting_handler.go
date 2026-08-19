package admin

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// semverPattern 预编译 semver 格式校验正则
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// menuItemIDPattern validates custom menu item IDs: alphanumeric, hyphens, underscores only.
var menuItemIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// generateMenuItemID generates a short random hex ID for a custom menu item.
func generateMenuItemID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate menu item ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func scopesContainOpenID(scopes string) bool {
	for _, scope := range strings.Fields(strings.ToLower(strings.TrimSpace(scopes))) {
		if scope == "openid" {
			return true
		}
	}
	return false
}

// SettingHandler 系统设置处理器
type SettingHandler struct {
	settingService   *service.SettingService
	emailService     *service.EmailService
	turnstileService *service.TurnstileService
	opsService       *service.OpsService
	updateMu         sync.Mutex
}

// NewSettingHandler 创建系统设置处理器
func NewSettingHandler(settingService *service.SettingService, emailService *service.EmailService, turnstileService *service.TurnstileService, opsService *service.OpsService) *SettingHandler {
	return &SettingHandler{
		settingService:   settingService,
		emailService:     emailService,
		turnstileService: turnstileService,
		opsService:       opsService,
	}
}

// GetSettings 获取所有系统设置
// GET /api/v1/admin/settings
func (h *SettingHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Check if ops monitoring is enabled (respects config.ops.enabled)
	opsEnabled := h.opsService != nil && h.opsService.IsMonitoringEnabled(c.Request.Context())
	defaultSubscriptions := make([]dto.DefaultSubscriptionSetting, 0, len(settings.DefaultSubscriptions))
	for _, sub := range settings.DefaultSubscriptions {
		defaultSubscriptions = append(defaultSubscriptions, dto.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	response.Success(c, dto.SystemSettings{
		RegistrationEnabled:                  settings.RegistrationEnabled,
		EmailVerifyEnabled:                   settings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist:     settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                     settings.PromoCodeEnabled,
		PasswordResetEnabled:                 settings.PasswordResetEnabled,
		FrontendURL:                          settings.FrontendURL,
		InvitationCodeEnabled:                settings.InvitationCodeEnabled,
		TotpEnabled:                          settings.TotpEnabled,
		TotpEncryptionKeyConfigured:          h.settingService.IsTotpEncryptionKeyConfigured(),
		SMTPHost:                             settings.SMTPHost,
		SMTPPort:                             settings.SMTPPort,
		SMTPUsername:                         settings.SMTPUsername,
		SMTPPasswordConfigured:               settings.SMTPPasswordConfigured,
		SMTPFrom:                             settings.SMTPFrom,
		SMTPFromName:                         settings.SMTPFromName,
		SMTPUseTLS:                           settings.SMTPUseTLS,
		FeedbackNotifyEmail:                  settings.FeedbackNotifyEmail,
		TurnstileEnabled:                     settings.TurnstileEnabled,
		TurnstileSiteKey:                     settings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:         settings.TurnstileSecretKeyConfigured,
		LinuxDoConnectEnabled:                settings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:               settings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured: settings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:            settings.LinuxDoConnectRedirectURL,
		OIDCConnectEnabled:                   settings.OIDCConnectEnabled,
		OIDCConnectProviderName:              settings.OIDCConnectProviderName,
		OIDCConnectClientID:                  settings.OIDCConnectClientID,
		OIDCConnectClientSecretConfigured:    settings.OIDCConnectClientSecretConfigured,
		OIDCConnectIssuerURL:                 settings.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:              settings.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:              settings.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:                  settings.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:               settings.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:                   settings.OIDCConnectJWKSURL,
		OIDCConnectScopes:                    settings.OIDCConnectScopes,
		OIDCConnectRedirectURL:               settings.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:       settings.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:           settings.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                   settings.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:           settings.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:        settings.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:          settings.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:      settings.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:         settings.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:            settings.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:      settings.OIDCConnectUserInfoUsernamePath,
		GitHubOAuthEnabled:                   settings.GitHubOAuthEnabled,
		GitHubOAuthClientID:                  settings.GitHubOAuthClientID,
		GitHubOAuthClientSecretConfigured:    settings.GitHubOAuthClientSecretConfigured,
		GitHubOAuthRedirectURL:               settings.GitHubOAuthRedirectURL,
		GitHubOAuthFrontendRedirectURL:       settings.GitHubOAuthFrontendRedirectURL,
		SiteName:                             settings.SiteName,
		SiteLogo:                             settings.SiteLogo,
		SiteSubtitle:                         settings.SiteSubtitle,
		APIBaseURL:                           settings.APIBaseURL,
		ContactInfo:                          settings.ContactInfo,
		TechSupportQRCode:                    settings.TechSupportQRCode,
		AfterSalesQRCode:                     settings.AfterSalesQRCode,
		DocURL:                               settings.DocURL,
		ChatbotURL:                           settings.ChatbotURL,
		HomeContent:                          settings.HomeContent,
		LandingReportsEnabled:                settings.LandingReportsEnabled,
		LandingPricingProMultiplier:          settings.LandingPricingProMultiplier,
		LandingPricingMaxMultiplier:          settings.LandingPricingMaxMultiplier,
		LandingPricingExchangeRate:           settings.LandingPricingExchangeRate,
		HideCcsImportButton:                  settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:          settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:              settings.PurchaseSubscriptionURL,
		CardShopEnabled:                      settings.CardShopEnabled,
		CardShopProducts:                     dto.CardShopProductsFromService(settings.CardShopProducts),
		InvoiceManagementEnabled:             settings.InvoiceManagementEnabled,
		FeedbackManagementEnabled:            settings.FeedbackManagementEnabled,
		GroupCacheHitRateEnabled:             settings.GroupCacheHitRateEnabled,
		SoraClientEnabled:                    settings.SoraClientEnabled,
		TableDefaultPageSize:                 settings.TableDefaultPageSize,
		TablePageSizeOptions:                 settings.TablePageSizeOptions,
		CustomMenuItems:                      dto.ParseCustomMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                      dto.ParseCustomEndpoints(settings.CustomEndpoints),
		DefaultConcurrency:                   settings.DefaultConcurrency,
		DefaultBalance:                       settings.DefaultBalance,
		DefaultSubscriptions:                 defaultSubscriptions,
		EnableModelFallback:                  settings.EnableModelFallback,
		FallbackModelAnthropic:               settings.FallbackModelAnthropic,
		FallbackModelOpenAI:                  settings.FallbackModelOpenAI,
		FallbackModelGemini:                  settings.FallbackModelGemini,
		FallbackModelAntigravity:             settings.FallbackModelAntigravity,
		EnableIdentityPatch:                  settings.EnableIdentityPatch,
		IdentityPatchPrompt:                  settings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                 opsEnabled && settings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:         settings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                  settings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:            settings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                 settings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                 settings.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:          settings.AllowUngroupedKeyScheduling,
		BackendModeEnabled:                   settings.BackendModeEnabled,
		EnableFingerprintUnification:         settings.EnableFingerprintUnification,
		EnableMetadataPassthrough:            settings.EnableMetadataPassthrough,
		EnableCCHSigning:                     settings.EnableCCHSigning,
		EnableAnthropicCacheTTL1hInjection:   settings.EnableAnthropicCacheTTL1hInjection,
		WebSearchEmulationEnabled:            settings.WebSearchEmulationEnabled,
		StripeEnabled:                        settings.StripeEnabled,
		StripeSecretKeyConfigured:            settings.StripeSecretKeyConfigured,
		StripeWebhookSecretConfigured:        settings.StripeWebhookSecretConfigured,
		AlipayEnabled:                        settings.AlipayEnabled,
		AlipayAppID:                          settings.AlipayAppID,
		AlipayPrivateKeyConfigured:           settings.AlipayPrivateKeyConfigured,
		AlipayPublicKeyConfigured:            settings.AlipayPublicKeyConfigured,
		AlipayNotifyURL:                      settings.AlipayNotifyURL,
		XunhuAlipayEnabled:                   settings.XunhuAlipayEnabled,
		XunhuAlipayAppID:                     settings.XunhuAlipayAppID,
		XunhuAlipayKeyConfigured:             settings.XunhuAlipayKeyConfigured,
		XunhuWechatEnabled:                   settings.XunhuWechatEnabled,
		XunhuWechatAppID:                     settings.XunhuWechatAppID,
		XunhuWechatKeyConfigured:             settings.XunhuWechatKeyConfigured,
		XunhuNotifyURL:                       settings.XunhuNotifyURL,
		EasyPayEnabled:                       settings.EasyPayEnabled,
		EasyPayPID:                           settings.EasyPayPID,
		EasyPayKeyConfigured:                 settings.EasyPayKeyConfigured,
		EasyPayAPIBase:                       settings.EasyPayAPIBase,
		TopupAlipayProvider:                  settings.TopupAlipayProvider,
		TopupWechatProvider:                  settings.TopupWechatProvider,
		BalanceAlertEnabled:                  settings.BalanceAlertEnabled,
		BalanceAlertDefaultThreshold:         settings.BalanceAlertDefaultThreshold,
		AccountQuotaNotifyEnabled:            settings.AccountQuotaNotifyEnabled,
		AccountQuotaNotifyEmails:             dto.NotifyEmailEntriesFromService(settings.AccountQuotaNotifyEmails),
	})
}

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {
	// 注册设置
	RegistrationEnabled              bool     `json:"registration_enabled"`
	EmailVerifyEnabled               bool     `json:"email_verify_enabled"`
	RegistrationEmailSuffixWhitelist []string `json:"registration_email_suffix_whitelist"`
	PromoCodeEnabled                 bool     `json:"promo_code_enabled"`
	PasswordResetEnabled             bool     `json:"password_reset_enabled"`
	FrontendURL                      string   `json:"frontend_url"`
	InvitationCodeEnabled            bool     `json:"invitation_code_enabled"`
	TotpEnabled                      bool     `json:"totp_enabled"` // TOTP 双因素认证

	// 邮件服务设置
	SMTPHost            string `json:"smtp_host"`
	SMTPPort            int    `json:"smtp_port"`
	SMTPUsername        string `json:"smtp_username"`
	SMTPPassword        string `json:"smtp_password"`
	SMTPFrom            string `json:"smtp_from_email"`
	SMTPFromName        string `json:"smtp_from_name"`
	SMTPUseTLS          bool   `json:"smtp_use_tls"`
	FeedbackNotifyEmail string `json:"feedback_notify_email"`

	// Cloudflare Turnstile 设置
	TurnstileEnabled   bool   `json:"turnstile_enabled"`
	TurnstileSiteKey   string `json:"turnstile_site_key"`
	TurnstileSecretKey string `json:"turnstile_secret_key"`

	// LinuxDo Connect OAuth 登录
	LinuxDoConnectEnabled      bool   `json:"linuxdo_connect_enabled"`
	LinuxDoConnectClientID     string `json:"linuxdo_connect_client_id"`
	LinuxDoConnectClientSecret string `json:"linuxdo_connect_client_secret"`
	LinuxDoConnectRedirectURL  string `json:"linuxdo_connect_redirect_url"`

	// Generic OIDC OAuth 登录
	OIDCConnectEnabled              bool   `json:"oidc_connect_enabled"`
	OIDCConnectProviderName         string `json:"oidc_connect_provider_name"`
	OIDCConnectClientID             string `json:"oidc_connect_client_id"`
	OIDCConnectClientSecret         string `json:"oidc_connect_client_secret"`
	OIDCConnectIssuerURL            string `json:"oidc_connect_issuer_url"`
	OIDCConnectDiscoveryURL         string `json:"oidc_connect_discovery_url"`
	OIDCConnectAuthorizeURL         string `json:"oidc_connect_authorize_url"`
	OIDCConnectTokenURL             string `json:"oidc_connect_token_url"`
	OIDCConnectUserInfoURL          string `json:"oidc_connect_userinfo_url"`
	OIDCConnectJWKSURL              string `json:"oidc_connect_jwks_url"`
	OIDCConnectScopes               string `json:"oidc_connect_scopes"`
	OIDCConnectRedirectURL          string `json:"oidc_connect_redirect_url"`
	OIDCConnectFrontendRedirectURL  string `json:"oidc_connect_frontend_redirect_url"`
	OIDCConnectTokenAuthMethod      string `json:"oidc_connect_token_auth_method"`
	OIDCConnectUsePKCE              bool   `json:"oidc_connect_use_pkce"`
	OIDCConnectValidateIDToken      bool   `json:"oidc_connect_validate_id_token"`
	OIDCConnectAllowedSigningAlgs   string `json:"oidc_connect_allowed_signing_algs"`
	OIDCConnectClockSkewSeconds     int    `json:"oidc_connect_clock_skew_seconds"`
	OIDCConnectRequireEmailVerified bool   `json:"oidc_connect_require_email_verified"`
	OIDCConnectUserInfoEmailPath    string `json:"oidc_connect_userinfo_email_path"`
	OIDCConnectUserInfoIDPath       string `json:"oidc_connect_userinfo_id_path"`
	OIDCConnectUserInfoUsernamePath string `json:"oidc_connect_userinfo_username_path"`

	// GitHub OAuth 登录
	GitHubOAuthEnabled             bool   `json:"github_oauth_enabled"`
	GitHubOAuthClientID            string `json:"github_oauth_client_id"`
	GitHubOAuthClientSecret        string `json:"github_oauth_client_secret"`
	GitHubOAuthRedirectURL         string `json:"github_oauth_redirect_url"`
	GitHubOAuthFrontendRedirectURL string `json:"github_oauth_frontend_redirect_url"`

	// OEM设置
	SiteName                    string                 `json:"site_name"`
	SiteLogo                    string                 `json:"site_logo"`
	SiteSubtitle                string                 `json:"site_subtitle"`
	APIBaseURL                  string                 `json:"api_base_url"`
	ContactInfo                 string                 `json:"contact_info"`
	TechSupportQRCode           string                 `json:"tech_support_qrcode"`
	AfterSalesQRCode            string                 `json:"after_sales_qrcode"`
	DocURL                      string                 `json:"doc_url"`
	ChatbotURL                  string                 `json:"chatbot_url"`
	HomeContent                 string                 `json:"home_content"`
	LandingReportsEnabled       *bool                  `json:"landing_reports_enabled"`
	LandingPricingProMultiplier *float64               `json:"landing_pricing_pro_multiplier"`
	LandingPricingMaxMultiplier *float64               `json:"landing_pricing_max_multiplier"`
	LandingPricingExchangeRate  *float64               `json:"landing_pricing_exchange_rate"`
	HideCcsImportButton         bool                   `json:"hide_ccs_import_button"`
	PurchaseSubscriptionEnabled *bool                  `json:"purchase_subscription_enabled"`
	PurchaseSubscriptionURL     *string                `json:"purchase_subscription_url"`
	CardShopEnabled             *bool                  `json:"card_shop_enabled"`
	CardShopProducts            *[]dto.CardShopProduct `json:"card_shop_products"`
	InvoiceManagementEnabled    *bool                  `json:"invoice_management_enabled"`
	FeedbackManagementEnabled   *bool                  `json:"feedback_management_enabled"`
	GroupCacheHitRateEnabled    *bool                  `json:"group_cache_hit_rate_enabled"`
	SoraClientEnabled           bool                   `json:"sora_client_enabled"`
	TableDefaultPageSize        int                    `json:"table_default_page_size"`
	TablePageSizeOptions        []int                  `json:"table_page_size_options"`
	CustomMenuItems             *[]dto.CustomMenuItem  `json:"custom_menu_items"`
	CustomEndpoints             *[]dto.CustomEndpoint  `json:"custom_endpoints"`

	// 默认配置
	DefaultConcurrency   int                              `json:"default_concurrency"`
	DefaultBalance       float64                          `json:"default_balance"`
	DefaultSubscriptions []dto.DefaultSubscriptionSetting `json:"default_subscriptions"`

	// Model fallback configuration
	EnableModelFallback      bool   `json:"enable_model_fallback"`
	FallbackModelAnthropic   string `json:"fallback_model_anthropic"`
	FallbackModelOpenAI      string `json:"fallback_model_openai"`
	FallbackModelGemini      string `json:"fallback_model_gemini"`
	FallbackModelAntigravity string `json:"fallback_model_antigravity"`

	// Identity patch configuration (Claude -> Gemini)
	EnableIdentityPatch bool   `json:"enable_identity_patch"`
	IdentityPatchPrompt string `json:"identity_patch_prompt"`

	// Ops monitoring (vNext)
	OpsMonitoringEnabled         *bool   `json:"ops_monitoring_enabled"`
	OpsRealtimeMonitoringEnabled *bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          *string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    *int    `json:"ops_metrics_interval_seconds"`

	MinClaudeCodeVersion string `json:"min_claude_code_version"`
	MaxClaudeCodeVersion string `json:"max_claude_code_version"`

	// 分组隔离
	AllowUngroupedKeyScheduling bool `json:"allow_ungrouped_key_scheduling"`

	// Backend Mode
	BackendModeEnabled bool `json:"backend_mode_enabled"`

	// Gateway forwarding behavior
	EnableFingerprintUnification       *bool `json:"enable_fingerprint_unification"`
	EnableMetadataPassthrough          *bool `json:"enable_metadata_passthrough"`
	EnableCCHSigning                   *bool `json:"enable_cch_signing"`
	EnableAnthropicCacheTTL1hInjection *bool `json:"enable_anthropic_cache_ttl_1h_injection"`

	// Balance low notification
	BalanceAlertEnabled          *bool                   `json:"balance_alert_enabled"`
	BalanceAlertDefaultThreshold *float64                `json:"balance_alert_default_threshold"`
	AccountQuotaNotifyEnabled    *bool                   `json:"account_quota_notify_enabled"`
	AccountQuotaNotifyEmails     *[]dto.NotifyEmailEntry `json:"account_quota_notify_emails"`

	// Stripe 支付设置
	StripeEnabled       bool   `json:"stripe_enabled"`
	StripeSecretKey     string `json:"stripe_secret_key"`
	StripeWebhookSecret string `json:"stripe_webhook_secret"`

	// 支付宝支付设置
	AlipayEnabled    bool   `json:"alipay_enabled"`
	AlipayAppID      string `json:"alipay_app_id"`
	AlipayPrivateKey string `json:"alipay_private_key"`
	AlipayPublicKey  string `json:"alipay_public_key"`
	AlipayNotifyURL  string `json:"alipay_notify_url"`

	// 虎皮椒聚合支付设置
	XunhuAlipayEnabled bool   `json:"xunhu_alipay_enabled"`
	XunhuAlipayAppID   string `json:"xunhu_alipay_appid"`
	XunhuAlipayKey     string `json:"xunhu_alipay_key"`
	XunhuWechatEnabled bool   `json:"xunhu_wechat_enabled"`
	XunhuWechatAppID   string `json:"xunhu_wechat_appid"`
	XunhuWechatKey     string `json:"xunhu_wechat_key"`
	XunhuNotifyURL     string `json:"xunhu_notify_url"`

	// EasyPay 聚合支付设置
	EasyPayEnabled      bool   `json:"easypay_enabled"`
	EasyPayPID          string `json:"easypay_pid"`
	EasyPayKey          string `json:"easypay_key"`
	EasyPayAPIBase      string `json:"easypay_api_base"`
	TopupAlipayProvider string `json:"topup_alipay_provider"`
	TopupWechatProvider string `json:"topup_wechat_provider"`

	// Payment configuration (integrated into settings, full replace)
	PaymentEnabled                   *bool    `json:"payment_enabled"`
	PaymentMinAmount                 *float64 `json:"payment_min_amount"`
	PaymentMaxAmount                 *float64 `json:"payment_max_amount"`
	PaymentDailyLimit                *float64 `json:"payment_daily_limit"`
	PaymentOrderTimeoutMin           *int     `json:"payment_order_timeout_minutes"`
	PaymentMaxPendingOrders          *int     `json:"payment_max_pending_orders"`
	PaymentEnabledTypes              []string `json:"payment_enabled_types"`
	PaymentBalanceDisabled           *bool    `json:"payment_balance_disabled"`
	PaymentBalanceRechargeMultiplier *float64 `json:"payment_balance_recharge_multiplier"`
	PaymentRechargeFeeRate           *float64 `json:"payment_recharge_fee_rate"`
	PaymentLoadBalanceStrat          *string  `json:"payment_load_balance_strategy"`
	PaymentProductNamePrefix         *string  `json:"payment_product_name_prefix"`
	PaymentProductNameSuffix         *string  `json:"payment_product_name_suffix"`
	PaymentHelpImageURL              *string  `json:"payment_help_image_url"`
	PaymentHelpText                  *string  `json:"payment_help_text"`

	// Cancel rate limit
	PaymentCancelRateLimitEnabled *bool   `json:"payment_cancel_rate_limit_enabled"`
	PaymentCancelRateLimitMax     *int    `json:"payment_cancel_rate_limit_max"`
	PaymentCancelRateLimitWindow  *int    `json:"payment_cancel_rate_limit_window"`
	PaymentCancelRateLimitUnit    *string `json:"payment_cancel_rate_limit_unit"`
	PaymentCancelRateLimitMode    *string `json:"payment_cancel_rate_limit_window_mode"`

	providedFields map[string]json.RawMessage
}

// UnmarshalJSON records which fields were present so PUT can safely support
// partial payloads without confusing an omitted value with a zero value.
func (r *UpdateSettingsRequest) UnmarshalJSON(data []byte) error {
	type requestAlias UpdateSettingsRequest

	var decoded requestAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var provided map[string]json.RawMessage
	if err := json.Unmarshal(data, &provided); err != nil {
		return err
	}

	*r = UpdateSettingsRequest(decoded)
	r.providedFields = provided
	return nil
}

func (r *UpdateSettingsRequest) fieldProvided(name string) bool {
	if r == nil {
		return false
	}
	raw, ok := r.providedFields[name]
	return ok && string(bytes.TrimSpace(raw)) != "null"
}

func (r *UpdateSettingsRequest) providedSettingKeys() map[string]struct{} {
	keys := make(map[string]struct{}, len(r.providedFields))
	for name, raw := range r.providedFields {
		raw = bytes.TrimSpace(raw)
		if string(raw) == "null" {
			continue
		}

		// Blank secrets intentionally mean "keep the persisted secret". Some
		// validation paths hydrate the request with that persisted value, so
		// exclude blank secret inputs from the persistence mask up front.
		switch name {
		case "smtp_password",
			"turnstile_secret_key",
			"linuxdo_connect_client_secret",
			"oidc_connect_client_secret",
			"github_oauth_client_secret",
			"stripe_secret_key",
			"stripe_webhook_secret",
			"alipay_private_key",
			"alipay_public_key",
			"xunhu_alipay_key",
			"xunhu_wechat_key",
			"easypay_key":
			var value string
			if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
				continue
			}
		}

		// The external DTO uses smtp_from_email while the persisted setting key
		// is smtp_from. All other settings handled by SettingService currently
		// share their JSON and persistence key names.
		if name == "smtp_from_email" {
			name = service.SettingKeySMTPFrom
		}
		keys[name] = struct{}{}
	}
	return keys
}

func (r *UpdateSettingsRequest) nonEmptySecretProvided(name string) bool {
	if r == nil {
		return false
	}
	raw, ok := r.providedFields[name]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return false
	}
	var value string
	return json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != ""
}

// mergeOmittedSettings keeps the current value for every non-pointer request
// field that was omitted. Pointer fields already carry presence information and
// are merged at their validation sites. Sensitive values are intentionally not
// copied here: an empty sensitive field keeps its existing persisted value in
// SettingService.UpdateSettings.
func mergeOmittedSettings(req *UpdateSettingsRequest, current *service.SystemSettings) {
	if req == nil || current == nil {
		return
	}

	if !req.fieldProvided("registration_enabled") {
		req.RegistrationEnabled = current.RegistrationEnabled
	}
	if !req.fieldProvided("email_verify_enabled") {
		req.EmailVerifyEnabled = current.EmailVerifyEnabled
	}
	if !req.fieldProvided("registration_email_suffix_whitelist") {
		req.RegistrationEmailSuffixWhitelist = append([]string(nil), current.RegistrationEmailSuffixWhitelist...)
	}
	if !req.fieldProvided("promo_code_enabled") {
		req.PromoCodeEnabled = current.PromoCodeEnabled
	}
	if !req.fieldProvided("password_reset_enabled") {
		req.PasswordResetEnabled = current.PasswordResetEnabled
	}
	if !req.fieldProvided("frontend_url") {
		req.FrontendURL = current.FrontendURL
	}
	if !req.fieldProvided("invitation_code_enabled") {
		req.InvitationCodeEnabled = current.InvitationCodeEnabled
	}
	if !req.fieldProvided("totp_enabled") {
		req.TotpEnabled = current.TotpEnabled
	}

	if !req.fieldProvided("smtp_host") {
		req.SMTPHost = current.SMTPHost
	}
	if !req.fieldProvided("smtp_port") {
		req.SMTPPort = current.SMTPPort
	}
	if !req.fieldProvided("smtp_username") {
		req.SMTPUsername = current.SMTPUsername
	}
	if !req.fieldProvided("smtp_from_email") {
		req.SMTPFrom = current.SMTPFrom
	}
	if !req.fieldProvided("smtp_from_name") {
		req.SMTPFromName = current.SMTPFromName
	}
	if !req.fieldProvided("smtp_use_tls") {
		req.SMTPUseTLS = current.SMTPUseTLS
	}
	if !req.fieldProvided("feedback_notify_email") {
		req.FeedbackNotifyEmail = current.FeedbackNotifyEmail
	}

	if !req.fieldProvided("turnstile_enabled") {
		req.TurnstileEnabled = current.TurnstileEnabled
	}
	if !req.fieldProvided("turnstile_site_key") {
		req.TurnstileSiteKey = current.TurnstileSiteKey
	}

	if !req.fieldProvided("linuxdo_connect_enabled") {
		req.LinuxDoConnectEnabled = current.LinuxDoConnectEnabled
	}
	if !req.fieldProvided("linuxdo_connect_client_id") {
		req.LinuxDoConnectClientID = current.LinuxDoConnectClientID
	}
	if !req.fieldProvided("linuxdo_connect_redirect_url") {
		req.LinuxDoConnectRedirectURL = current.LinuxDoConnectRedirectURL
	}

	if !req.fieldProvided("oidc_connect_enabled") {
		req.OIDCConnectEnabled = current.OIDCConnectEnabled
	}
	if !req.fieldProvided("oidc_connect_provider_name") {
		req.OIDCConnectProviderName = current.OIDCConnectProviderName
	}
	if !req.fieldProvided("oidc_connect_client_id") {
		req.OIDCConnectClientID = current.OIDCConnectClientID
	}
	if !req.fieldProvided("oidc_connect_issuer_url") {
		req.OIDCConnectIssuerURL = current.OIDCConnectIssuerURL
	}
	if !req.fieldProvided("oidc_connect_discovery_url") {
		req.OIDCConnectDiscoveryURL = current.OIDCConnectDiscoveryURL
	}
	if !req.fieldProvided("oidc_connect_authorize_url") {
		req.OIDCConnectAuthorizeURL = current.OIDCConnectAuthorizeURL
	}
	if !req.fieldProvided("oidc_connect_token_url") {
		req.OIDCConnectTokenURL = current.OIDCConnectTokenURL
	}
	if !req.fieldProvided("oidc_connect_userinfo_url") {
		req.OIDCConnectUserInfoURL = current.OIDCConnectUserInfoURL
	}
	if !req.fieldProvided("oidc_connect_jwks_url") {
		req.OIDCConnectJWKSURL = current.OIDCConnectJWKSURL
	}
	if !req.fieldProvided("oidc_connect_scopes") {
		req.OIDCConnectScopes = current.OIDCConnectScopes
	}
	if !req.fieldProvided("oidc_connect_redirect_url") {
		req.OIDCConnectRedirectURL = current.OIDCConnectRedirectURL
	}
	if !req.fieldProvided("oidc_connect_frontend_redirect_url") {
		req.OIDCConnectFrontendRedirectURL = current.OIDCConnectFrontendRedirectURL
	}
	if !req.fieldProvided("oidc_connect_token_auth_method") {
		req.OIDCConnectTokenAuthMethod = current.OIDCConnectTokenAuthMethod
	}
	if !req.fieldProvided("oidc_connect_use_pkce") {
		req.OIDCConnectUsePKCE = current.OIDCConnectUsePKCE
	}
	if !req.fieldProvided("oidc_connect_validate_id_token") {
		req.OIDCConnectValidateIDToken = current.OIDCConnectValidateIDToken
	}
	if !req.fieldProvided("oidc_connect_allowed_signing_algs") {
		req.OIDCConnectAllowedSigningAlgs = current.OIDCConnectAllowedSigningAlgs
	}
	if !req.fieldProvided("oidc_connect_clock_skew_seconds") {
		req.OIDCConnectClockSkewSeconds = current.OIDCConnectClockSkewSeconds
	}
	if !req.fieldProvided("oidc_connect_require_email_verified") {
		req.OIDCConnectRequireEmailVerified = current.OIDCConnectRequireEmailVerified
	}
	if !req.fieldProvided("oidc_connect_userinfo_email_path") {
		req.OIDCConnectUserInfoEmailPath = current.OIDCConnectUserInfoEmailPath
	}
	if !req.fieldProvided("oidc_connect_userinfo_id_path") {
		req.OIDCConnectUserInfoIDPath = current.OIDCConnectUserInfoIDPath
	}
	if !req.fieldProvided("oidc_connect_userinfo_username_path") {
		req.OIDCConnectUserInfoUsernamePath = current.OIDCConnectUserInfoUsernamePath
	}

	if !req.fieldProvided("github_oauth_enabled") {
		req.GitHubOAuthEnabled = current.GitHubOAuthEnabled
	}
	if !req.fieldProvided("github_oauth_client_id") {
		req.GitHubOAuthClientID = current.GitHubOAuthClientID
	}
	if !req.fieldProvided("github_oauth_redirect_url") {
		req.GitHubOAuthRedirectURL = current.GitHubOAuthRedirectURL
	}
	if !req.fieldProvided("github_oauth_frontend_redirect_url") {
		req.GitHubOAuthFrontendRedirectURL = current.GitHubOAuthFrontendRedirectURL
	}

	if !req.fieldProvided("site_name") {
		req.SiteName = current.SiteName
	}
	if !req.fieldProvided("site_logo") {
		req.SiteLogo = current.SiteLogo
	}
	if !req.fieldProvided("site_subtitle") {
		req.SiteSubtitle = current.SiteSubtitle
	}
	if !req.fieldProvided("api_base_url") {
		req.APIBaseURL = current.APIBaseURL
	}
	if !req.fieldProvided("contact_info") {
		req.ContactInfo = current.ContactInfo
	}
	if !req.fieldProvided("tech_support_qrcode") {
		req.TechSupportQRCode = current.TechSupportQRCode
	}
	if !req.fieldProvided("after_sales_qrcode") {
		req.AfterSalesQRCode = current.AfterSalesQRCode
	}
	if !req.fieldProvided("doc_url") {
		req.DocURL = current.DocURL
	}
	if !req.fieldProvided("chatbot_url") {
		req.ChatbotURL = current.ChatbotURL
	}
	if !req.fieldProvided("home_content") {
		req.HomeContent = current.HomeContent
	}
	if !req.fieldProvided("hide_ccs_import_button") {
		req.HideCcsImportButton = current.HideCcsImportButton
	}
	if !req.fieldProvided("sora_client_enabled") {
		req.SoraClientEnabled = current.SoraClientEnabled
	}
	if !req.fieldProvided("table_default_page_size") {
		req.TableDefaultPageSize = current.TableDefaultPageSize
	}
	if !req.fieldProvided("table_page_size_options") {
		req.TablePageSizeOptions = append([]int(nil), current.TablePageSizeOptions...)
	}

	if !req.fieldProvided("default_concurrency") {
		req.DefaultConcurrency = current.DefaultConcurrency
	}
	if !req.fieldProvided("default_balance") {
		req.DefaultBalance = current.DefaultBalance
	}
	if !req.fieldProvided("default_subscriptions") {
		req.DefaultSubscriptions = make([]dto.DefaultSubscriptionSetting, 0, len(current.DefaultSubscriptions))
		for _, sub := range current.DefaultSubscriptions {
			req.DefaultSubscriptions = append(req.DefaultSubscriptions, dto.DefaultSubscriptionSetting{
				GroupID:      sub.GroupID,
				ValidityDays: sub.ValidityDays,
			})
		}
	}

	if !req.fieldProvided("enable_model_fallback") {
		req.EnableModelFallback = current.EnableModelFallback
	}
	if !req.fieldProvided("fallback_model_anthropic") {
		req.FallbackModelAnthropic = current.FallbackModelAnthropic
	}
	if !req.fieldProvided("fallback_model_openai") {
		req.FallbackModelOpenAI = current.FallbackModelOpenAI
	}
	if !req.fieldProvided("fallback_model_gemini") {
		req.FallbackModelGemini = current.FallbackModelGemini
	}
	if !req.fieldProvided("fallback_model_antigravity") {
		req.FallbackModelAntigravity = current.FallbackModelAntigravity
	}
	if !req.fieldProvided("enable_identity_patch") {
		req.EnableIdentityPatch = current.EnableIdentityPatch
	}
	if !req.fieldProvided("identity_patch_prompt") {
		req.IdentityPatchPrompt = current.IdentityPatchPrompt
	}
	if !req.fieldProvided("min_claude_code_version") {
		req.MinClaudeCodeVersion = current.MinClaudeCodeVersion
	}
	if !req.fieldProvided("max_claude_code_version") {
		req.MaxClaudeCodeVersion = current.MaxClaudeCodeVersion
	}
	if !req.fieldProvided("allow_ungrouped_key_scheduling") {
		req.AllowUngroupedKeyScheduling = current.AllowUngroupedKeyScheduling
	}
	if !req.fieldProvided("backend_mode_enabled") {
		req.BackendModeEnabled = current.BackendModeEnabled
	}

	if !req.fieldProvided("stripe_enabled") {
		req.StripeEnabled = current.StripeEnabled
	}
	if !req.fieldProvided("alipay_enabled") {
		req.AlipayEnabled = current.AlipayEnabled
	}
	if !req.fieldProvided("alipay_app_id") {
		req.AlipayAppID = current.AlipayAppID
	}
	if !req.fieldProvided("alipay_notify_url") {
		req.AlipayNotifyURL = current.AlipayNotifyURL
	}
	if !req.fieldProvided("xunhu_alipay_enabled") {
		req.XunhuAlipayEnabled = current.XunhuAlipayEnabled
	}
	if !req.fieldProvided("xunhu_alipay_appid") {
		req.XunhuAlipayAppID = current.XunhuAlipayAppID
	}
	if !req.fieldProvided("xunhu_wechat_enabled") {
		req.XunhuWechatEnabled = current.XunhuWechatEnabled
	}
	if !req.fieldProvided("xunhu_wechat_appid") {
		req.XunhuWechatAppID = current.XunhuWechatAppID
	}
	if !req.fieldProvided("xunhu_notify_url") {
		req.XunhuNotifyURL = current.XunhuNotifyURL
	}
	if !req.fieldProvided("easypay_enabled") {
		req.EasyPayEnabled = current.EasyPayEnabled
	}
	if !req.fieldProvided("easypay_pid") {
		req.EasyPayPID = current.EasyPayPID
	}
	if !req.fieldProvided("easypay_api_base") {
		req.EasyPayAPIBase = current.EasyPayAPIBase
	}
	if !req.fieldProvided("topup_alipay_provider") {
		req.TopupAlipayProvider = current.TopupAlipayProvider
	}
	if !req.fieldProvided("topup_wechat_provider") {
		req.TopupWechatProvider = current.TopupWechatProvider
	}
}

// UpdateSettings 更新系统设置
// PUT /api/v1/admin/settings
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Keep read/merge/validate/write atomic within this process so concurrent
	// partial updates cannot violate cross-field invariants (for example,
	// enabling a shop while another request clears its products).
	h.updateMu.Lock()
	defer h.updateMu.Unlock()

	previousSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	mergeOmittedSettings(&req, previousSettings)
	providedSettingKeys := req.providedSettingKeys()

	// 验证参数
	if req.DefaultConcurrency < 1 {
		req.DefaultConcurrency = 1
	}
	if req.DefaultBalance < 0 {
		req.DefaultBalance = 0
	}
	// 通用表格配置：兼容旧客户端未传字段时保留当前值。
	if req.TableDefaultPageSize <= 0 {
		req.TableDefaultPageSize = previousSettings.TableDefaultPageSize
	}
	if req.TablePageSizeOptions == nil {
		req.TablePageSizeOptions = previousSettings.TablePageSizeOptions
	}
	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SMTPPassword = strings.TrimSpace(req.SMTPPassword)
	req.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
	req.SMTPFromName = strings.TrimSpace(req.SMTPFromName)
	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}
	req.DefaultSubscriptions = normalizeDefaultSubscriptions(req.DefaultSubscriptions)

	req.APIBaseURL = strings.TrimSpace(req.APIBaseURL)
	if req.APIBaseURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(req.APIBaseURL); err != nil {
			response.BadRequest(c, "API Base URL must be an absolute http(s) URL")
			return
		}
	}

	// Turnstile 参数验证
	if req.TurnstileEnabled {
		// 检查必填字段
		if req.TurnstileSiteKey == "" {
			response.BadRequest(c, "Turnstile Site Key is required when enabled")
			return
		}
		// 如果未提供 secret key，使用已保存的值（留空保留当前值）
		if req.TurnstileSecretKey == "" {
			if previousSettings.TurnstileSecretKey == "" {
				response.BadRequest(c, "Turnstile Secret Key is required when enabled")
				return
			}
			req.TurnstileSecretKey = previousSettings.TurnstileSecretKey
		}

		// 当 site_key 或 secret_key 任一变化时验证（避免配置错误导致无法登录）
		siteKeyChanged := previousSettings.TurnstileSiteKey != req.TurnstileSiteKey
		secretKeyChanged := previousSettings.TurnstileSecretKey != req.TurnstileSecretKey
		if siteKeyChanged || secretKeyChanged {
			if err := h.turnstileService.ValidateSecretKey(c.Request.Context(), req.TurnstileSecretKey); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
	}

	// TOTP 双因素认证参数验证
	// 只有手动配置了加密密钥才允许启用 TOTP 功能
	if req.TotpEnabled && !previousSettings.TotpEnabled {
		// 尝试启用 TOTP，检查加密密钥是否已手动配置
		if !h.settingService.IsTotpEncryptionKeyConfigured() {
			response.BadRequest(c, "Cannot enable TOTP: TOTP_ENCRYPTION_KEY environment variable must be configured first. Generate a key with 'openssl rand -hex 32' and set it in your environment.")
			return
		}
	}

	// LinuxDo Connect 参数验证
	if req.LinuxDoConnectEnabled {
		req.LinuxDoConnectClientID = strings.TrimSpace(req.LinuxDoConnectClientID)
		req.LinuxDoConnectClientSecret = strings.TrimSpace(req.LinuxDoConnectClientSecret)
		req.LinuxDoConnectRedirectURL = strings.TrimSpace(req.LinuxDoConnectRedirectURL)

		if req.LinuxDoConnectClientID == "" {
			response.BadRequest(c, "LinuxDo Client ID is required when enabled")
			return
		}
		if req.LinuxDoConnectRedirectURL == "" {
			response.BadRequest(c, "LinuxDo Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.LinuxDoConnectRedirectURL); err != nil {
			response.BadRequest(c, "LinuxDo Redirect URL must be an absolute http(s) URL")
			return
		}

		// 如果未提供 client_secret，则保留现有值（如有）。
		if req.LinuxDoConnectClientSecret == "" {
			if previousSettings.LinuxDoConnectClientSecret == "" {
				response.BadRequest(c, "LinuxDo Client Secret is required when enabled")
				return
			}
			req.LinuxDoConnectClientSecret = previousSettings.LinuxDoConnectClientSecret
		}
	}

	// Generic OIDC 参数验证
	if req.OIDCConnectEnabled {
		req.OIDCConnectProviderName = strings.TrimSpace(req.OIDCConnectProviderName)
		req.OIDCConnectClientID = strings.TrimSpace(req.OIDCConnectClientID)
		req.OIDCConnectClientSecret = strings.TrimSpace(req.OIDCConnectClientSecret)
		req.OIDCConnectIssuerURL = strings.TrimSpace(req.OIDCConnectIssuerURL)
		req.OIDCConnectDiscoveryURL = strings.TrimSpace(req.OIDCConnectDiscoveryURL)
		req.OIDCConnectAuthorizeURL = strings.TrimSpace(req.OIDCConnectAuthorizeURL)
		req.OIDCConnectTokenURL = strings.TrimSpace(req.OIDCConnectTokenURL)
		req.OIDCConnectUserInfoURL = strings.TrimSpace(req.OIDCConnectUserInfoURL)
		req.OIDCConnectJWKSURL = strings.TrimSpace(req.OIDCConnectJWKSURL)
		req.OIDCConnectScopes = strings.TrimSpace(req.OIDCConnectScopes)
		req.OIDCConnectRedirectURL = strings.TrimSpace(req.OIDCConnectRedirectURL)
		req.OIDCConnectFrontendRedirectURL = strings.TrimSpace(req.OIDCConnectFrontendRedirectURL)
		req.OIDCConnectTokenAuthMethod = strings.ToLower(strings.TrimSpace(req.OIDCConnectTokenAuthMethod))
		req.OIDCConnectAllowedSigningAlgs = strings.TrimSpace(req.OIDCConnectAllowedSigningAlgs)
		req.OIDCConnectUserInfoEmailPath = strings.TrimSpace(req.OIDCConnectUserInfoEmailPath)
		req.OIDCConnectUserInfoIDPath = strings.TrimSpace(req.OIDCConnectUserInfoIDPath)
		req.OIDCConnectUserInfoUsernamePath = strings.TrimSpace(req.OIDCConnectUserInfoUsernamePath)

		if req.OIDCConnectProviderName == "" {
			req.OIDCConnectProviderName = "OIDC"
		}
		if req.OIDCConnectClientID == "" {
			response.BadRequest(c, "OIDC Client ID is required when enabled")
			return
		}
		if req.OIDCConnectIssuerURL == "" {
			response.BadRequest(c, "OIDC Issuer URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectIssuerURL); err != nil {
			response.BadRequest(c, "OIDC Issuer URL must be an absolute http(s) URL")
			return
		}
		if req.OIDCConnectDiscoveryURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectDiscoveryURL); err != nil {
				response.BadRequest(c, "OIDC Discovery URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectAuthorizeURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectAuthorizeURL); err != nil {
				response.BadRequest(c, "OIDC Authorize URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectTokenURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectTokenURL); err != nil {
				response.BadRequest(c, "OIDC Token URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectUserInfoURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectUserInfoURL); err != nil {
				response.BadRequest(c, "OIDC UserInfo URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectRedirectURL == "" {
			response.BadRequest(c, "OIDC Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectRedirectURL); err != nil {
			response.BadRequest(c, "OIDC Redirect URL must be an absolute http(s) URL")
			return
		}
		if req.OIDCConnectFrontendRedirectURL == "" {
			response.BadRequest(c, "OIDC Frontend Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateFrontendRedirectURL(req.OIDCConnectFrontendRedirectURL); err != nil {
			response.BadRequest(c, "OIDC Frontend Redirect URL is invalid")
			return
		}
		if !scopesContainOpenID(req.OIDCConnectScopes) {
			response.BadRequest(c, "OIDC scopes must contain openid")
			return
		}
		switch req.OIDCConnectTokenAuthMethod {
		case "", "client_secret_post", "client_secret_basic", "none":
		default:
			response.BadRequest(c, "OIDC Token Auth Method must be one of client_secret_post/client_secret_basic/none")
			return
		}
		if req.OIDCConnectTokenAuthMethod == "none" && !req.OIDCConnectUsePKCE {
			response.BadRequest(c, "OIDC PKCE must be enabled when token_auth_method=none")
			return
		}
		if req.OIDCConnectClockSkewSeconds < 0 || req.OIDCConnectClockSkewSeconds > 600 {
			response.BadRequest(c, "OIDC clock skew seconds must be between 0 and 600")
			return
		}
		if req.OIDCConnectValidateIDToken {
			if req.OIDCConnectAllowedSigningAlgs == "" {
				response.BadRequest(c, "OIDC Allowed Signing Algs is required when validate_id_token=true")
				return
			}
		}
		if req.OIDCConnectJWKSURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectJWKSURL); err != nil {
				response.BadRequest(c, "OIDC JWKS URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectTokenAuthMethod == "" || req.OIDCConnectTokenAuthMethod == "client_secret_post" || req.OIDCConnectTokenAuthMethod == "client_secret_basic" {
			if req.OIDCConnectClientSecret == "" {
				if previousSettings.OIDCConnectClientSecret == "" {
					response.BadRequest(c, "OIDC Client Secret is required when enabled")
					return
				}
				req.OIDCConnectClientSecret = previousSettings.OIDCConnectClientSecret
			}
		}
	}

	// GitHub OAuth 参数验证
	if req.GitHubOAuthEnabled {
		req.GitHubOAuthClientID = strings.TrimSpace(req.GitHubOAuthClientID)
		req.GitHubOAuthClientSecret = strings.TrimSpace(req.GitHubOAuthClientSecret)
		req.GitHubOAuthRedirectURL = strings.TrimSpace(req.GitHubOAuthRedirectURL)
		req.GitHubOAuthFrontendRedirectURL = strings.TrimSpace(req.GitHubOAuthFrontendRedirectURL)

		if req.GitHubOAuthClientID == "" {
			response.BadRequest(c, "GitHub Client ID is required when enabled")
			return
		}
		if req.GitHubOAuthRedirectURL == "" {
			response.BadRequest(c, "GitHub Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.GitHubOAuthRedirectURL); err != nil {
			response.BadRequest(c, "GitHub Redirect URL must be an absolute http(s) URL")
			return
		}
		if req.GitHubOAuthFrontendRedirectURL == "" {
			req.GitHubOAuthFrontendRedirectURL = "/auth/github/callback"
		}
		if err := config.ValidateFrontendRedirectURL(req.GitHubOAuthFrontendRedirectURL); err != nil {
			response.BadRequest(c, "GitHub Frontend Redirect URL is invalid")
			return
		}
		if req.GitHubOAuthClientSecret == "" {
			if previousSettings.GitHubOAuthClientSecret == "" {
				response.BadRequest(c, "GitHub Client Secret is required when enabled")
				return
			}
			req.GitHubOAuthClientSecret = previousSettings.GitHubOAuthClientSecret
		}
	}

	// “购买订阅”页面配置验证
	purchaseEnabled := previousSettings.PurchaseSubscriptionEnabled
	if req.PurchaseSubscriptionEnabled != nil {
		purchaseEnabled = *req.PurchaseSubscriptionEnabled
	}
	purchaseURL := previousSettings.PurchaseSubscriptionURL
	if req.PurchaseSubscriptionURL != nil {
		purchaseURL = strings.TrimSpace(*req.PurchaseSubscriptionURL)
	}

	// - 启用时要求 URL 合法且非空
	// - 禁用时允许为空；若提供了 URL 也做基本校验，避免误配置
	if purchaseEnabled {
		if purchaseURL == "" {
			response.BadRequest(c, "Purchase Subscription URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	} else if purchaseURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	}

	cardShopEnabled := previousSettings.CardShopEnabled
	if req.CardShopEnabled != nil {
		cardShopEnabled = *req.CardShopEnabled
	}
	cardShopProducts := previousSettings.CardShopProducts
	if req.CardShopProducts != nil {
		const (
			maxCardShopProducts = 20
			maxProductLabelLen  = 50
			maxProductURLLen    = 2048
			maxProductIDLen     = 32
		)
		items := dto.CardShopProductsToService(*req.CardShopProducts)
		if len(items) > maxCardShopProducts {
			response.BadRequest(c, "Too many card shop products (max 20)")
			return
		}
		seen := make(map[string]struct{}, len(items))
		for i := range items {
			items[i].ID = strings.TrimSpace(items[i].ID)
			items[i].Label = strings.TrimSpace(items[i].Label)
			items[i].URL = strings.TrimSpace(items[i].URL)
			items[i].SortOrder = i
			if items[i].ID == "" {
				id, err := generateMenuItemID()
				if err != nil {
					response.Error(c, http.StatusInternalServerError, "Failed to generate card shop product ID")
					return
				}
				items[i].ID = id
			} else if len(items[i].ID) > maxProductIDLen {
				response.BadRequest(c, "Card shop product ID is too long (max 32 characters)")
				return
			} else if !menuItemIDPattern.MatchString(items[i].ID) {
				response.BadRequest(c, "Card shop product ID contains invalid characters")
				return
			}
			if _, exists := seen[items[i].ID]; exists {
				response.BadRequest(c, "Duplicate card shop product ID: "+items[i].ID)
				return
			}
			seen[items[i].ID] = struct{}{}
			if items[i].Label == "" {
				response.BadRequest(c, "Card shop product label is required")
				return
			}
			if len(items[i].Label) > maxProductLabelLen {
				response.BadRequest(c, "Card shop product label is too long (max 50 characters)")
				return
			}
			if items[i].AmountCNY <= 0 {
				response.BadRequest(c, "Card shop product amount must be greater than 0")
				return
			}
			if len(items[i].URL) > maxProductURLLen {
				response.BadRequest(c, "Card shop product URL is too long (max 2048 characters)")
				return
			}
			if items[i].URL != "" {
				if err := config.ValidateAbsoluteHTTPURL(items[i].URL); err != nil {
					response.BadRequest(c, "Card shop product URL must be an absolute http(s) URL")
					return
				}
			}
			if items[i].Enabled && items[i].URL == "" {
				response.BadRequest(c, "Enabled card shop products require a URL")
				return
			}
		}
		cardShopProducts = items
	}
	if cardShopEnabled {
		hasEnabledProduct := false
		for _, product := range cardShopProducts {
			if product.Enabled && product.AmountCNY > 0 && strings.TrimSpace(product.URL) != "" {
				hasEnabledProduct = true
				break
			}
		}
		if !hasEnabledProduct {
			response.BadRequest(c, "At least one enabled card shop product is required when card shop is enabled")
			return
		}
	}
	invoiceManagementEnabled := previousSettings.InvoiceManagementEnabled
	if req.InvoiceManagementEnabled != nil {
		invoiceManagementEnabled = *req.InvoiceManagementEnabled
	}
	feedbackManagementEnabled := previousSettings.FeedbackManagementEnabled
	if req.FeedbackManagementEnabled != nil {
		feedbackManagementEnabled = *req.FeedbackManagementEnabled
	}
	groupCacheHitRateEnabled := previousSettings.GroupCacheHitRateEnabled
	if req.GroupCacheHitRateEnabled != nil {
		groupCacheHitRateEnabled = *req.GroupCacheHitRateEnabled
	}

	// Frontend URL 验证
	req.FrontendURL = strings.TrimSpace(req.FrontendURL)
	if req.FrontendURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(req.FrontendURL); err != nil {
			response.BadRequest(c, "Frontend URL must be an absolute http(s) URL")
			return
		}
	}

	// 自定义菜单项验证
	const (
		maxCustomMenuItems    = 20
		maxMenuItemLabelLen   = 50
		maxMenuItemURLLen     = 2048
		maxMenuItemIconSVGLen = 10 * 1024 // 10KB
		maxMenuItemIDLen      = 32
	)

	customMenuJSON := previousSettings.CustomMenuItems
	if req.CustomMenuItems != nil {
		items := *req.CustomMenuItems
		if len(items) > maxCustomMenuItems {
			response.BadRequest(c, "Too many custom menu items (max 20)")
			return
		}
		for i, item := range items {
			if strings.TrimSpace(item.Label) == "" {
				response.BadRequest(c, "Custom menu item label is required")
				return
			}
			if len(item.Label) > maxMenuItemLabelLen {
				response.BadRequest(c, "Custom menu item label is too long (max 50 characters)")
				return
			}
			if strings.TrimSpace(item.URL) == "" {
				response.BadRequest(c, "Custom menu item URL is required")
				return
			}
			if len(item.URL) > maxMenuItemURLLen {
				response.BadRequest(c, "Custom menu item URL is too long (max 2048 characters)")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(strings.TrimSpace(item.URL)); err != nil {
				response.BadRequest(c, "Custom menu item URL must be an absolute http(s) URL")
				return
			}
			if item.Visibility != "user" && item.Visibility != "admin" {
				response.BadRequest(c, "Custom menu item visibility must be 'user' or 'admin'")
				return
			}
			if len(item.IconSVG) > maxMenuItemIconSVGLen {
				response.BadRequest(c, "Custom menu item icon SVG is too large (max 10KB)")
				return
			}
			// Auto-generate ID if missing
			if strings.TrimSpace(item.ID) == "" {
				id, err := generateMenuItemID()
				if err != nil {
					response.Error(c, http.StatusInternalServerError, "Failed to generate menu item ID")
					return
				}
				items[i].ID = id
			} else if len(item.ID) > maxMenuItemIDLen {
				response.BadRequest(c, "Custom menu item ID is too long (max 32 characters)")
				return
			} else if !menuItemIDPattern.MatchString(item.ID) {
				response.BadRequest(c, "Custom menu item ID contains invalid characters (only a-z, A-Z, 0-9, - and _ are allowed)")
				return
			}
		}
		// ID uniqueness check
		seen := make(map[string]struct{}, len(items))
		for _, item := range items {
			if _, exists := seen[item.ID]; exists {
				response.BadRequest(c, "Duplicate custom menu item ID: "+item.ID)
				return
			}
			seen[item.ID] = struct{}{}
		}
		menuBytes, err := json.Marshal(items)
		if err != nil {
			response.BadRequest(c, "Failed to serialize custom menu items")
			return
		}
		customMenuJSON = string(menuBytes)
	}

	// 自定义端点验证
	const (
		maxCustomEndpoints        = 10
		maxEndpointNameLen        = 50
		maxEndpointURLLen         = 2048
		maxEndpointDescriptionLen = 200
	)

	customEndpointsJSON := previousSettings.CustomEndpoints
	if req.CustomEndpoints != nil {
		endpoints := *req.CustomEndpoints
		if len(endpoints) > maxCustomEndpoints {
			response.BadRequest(c, "Too many custom endpoints (max 10)")
			return
		}
		for _, ep := range endpoints {
			if strings.TrimSpace(ep.Name) == "" {
				response.BadRequest(c, "Custom endpoint name is required")
				return
			}
			if len(ep.Name) > maxEndpointNameLen {
				response.BadRequest(c, "Custom endpoint name is too long (max 50 characters)")
				return
			}
			if strings.TrimSpace(ep.Endpoint) == "" {
				response.BadRequest(c, "Custom endpoint URL is required")
				return
			}
			if len(ep.Endpoint) > maxEndpointURLLen {
				response.BadRequest(c, "Custom endpoint URL is too long (max 2048 characters)")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(strings.TrimSpace(ep.Endpoint)); err != nil {
				response.BadRequest(c, "Custom endpoint URL must be an absolute http(s) URL")
				return
			}
			if len(ep.Description) > maxEndpointDescriptionLen {
				response.BadRequest(c, "Custom endpoint description is too long (max 200 characters)")
				return
			}
		}
		endpointBytes, err := json.Marshal(endpoints)
		if err != nil {
			response.BadRequest(c, "Failed to serialize custom endpoints")
			return
		}
		customEndpointsJSON = string(endpointBytes)
	}

	// Ops metrics collector interval validation (seconds).
	if req.OpsMetricsIntervalSeconds != nil {
		v := *req.OpsMetricsIntervalSeconds
		if v < 60 {
			v = 60
		}
		if v > 3600 {
			v = 3600
		}
		req.OpsMetricsIntervalSeconds = &v
	}
	defaultSubscriptions := make([]service.DefaultSubscriptionSetting, 0, len(req.DefaultSubscriptions))
	for _, sub := range req.DefaultSubscriptions {
		defaultSubscriptions = append(defaultSubscriptions, service.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// 验证最低版本号格式（空字符串=禁用，或合法 semver）
	if req.MinClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MinClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "min_claude_code_version must be empty or a valid semver (e.g. 2.1.63)")
			return
		}
	}

	// 验证最高版本号格式（空字符串=禁用，或合法 semver）
	if req.MaxClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MaxClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be empty or a valid semver (e.g. 3.0.0)")
			return
		}
	}

	// 交叉验证：如果同时设置了最低和最高版本号，最高版本号必须 >= 最低版本号
	if req.MinClaudeCodeVersion != "" && req.MaxClaudeCodeVersion != "" {
		if service.CompareVersions(req.MaxClaudeCodeVersion, req.MinClaudeCodeVersion) < 0 {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be greater than or equal to min_claude_code_version")
			return
		}
	}

	landingReportsEnabled := previousSettings.LandingReportsEnabled
	if req.LandingReportsEnabled != nil {
		landingReportsEnabled = *req.LandingReportsEnabled
	}
	landingPricingProMultiplier := previousSettings.LandingPricingProMultiplier
	if req.LandingPricingProMultiplier != nil {
		if *req.LandingPricingProMultiplier <= 0 {
			response.BadRequest(c, "landing_pricing_pro_multiplier must be greater than 0")
			return
		}
		landingPricingProMultiplier = *req.LandingPricingProMultiplier
	}
	landingPricingMaxMultiplier := previousSettings.LandingPricingMaxMultiplier
	if req.LandingPricingMaxMultiplier != nil {
		if *req.LandingPricingMaxMultiplier <= 0 {
			response.BadRequest(c, "landing_pricing_max_multiplier must be greater than 0")
			return
		}
		landingPricingMaxMultiplier = *req.LandingPricingMaxMultiplier
	}
	landingPricingExchangeRate := previousSettings.LandingPricingExchangeRate
	if req.LandingPricingExchangeRate != nil {
		if *req.LandingPricingExchangeRate <= 0 {
			response.BadRequest(c, "landing_pricing_exchange_rate must be greater than 0")
			return
		}
		landingPricingExchangeRate = *req.LandingPricingExchangeRate
	}

	settings := &service.SystemSettings{
		RegistrationEnabled:              req.RegistrationEnabled,
		EmailVerifyEnabled:               req.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: req.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 req.PromoCodeEnabled,
		PasswordResetEnabled:             req.PasswordResetEnabled,
		FrontendURL:                      req.FrontendURL,
		InvitationCodeEnabled:            req.InvitationCodeEnabled,
		TotpEnabled:                      req.TotpEnabled,
		SMTPHost:                         req.SMTPHost,
		SMTPPort:                         req.SMTPPort,
		SMTPUsername:                     req.SMTPUsername,
		SMTPPassword:                     req.SMTPPassword,
		SMTPFrom:                         req.SMTPFrom,
		SMTPFromName:                     req.SMTPFromName,
		SMTPUseTLS:                       req.SMTPUseTLS,
		FeedbackNotifyEmail:              req.FeedbackNotifyEmail,
		TurnstileEnabled:                 req.TurnstileEnabled,
		TurnstileSiteKey:                 req.TurnstileSiteKey,
		TurnstileSecretKey:               req.TurnstileSecretKey,
		LinuxDoConnectEnabled:            req.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:           req.LinuxDoConnectClientID,
		LinuxDoConnectClientSecret:       req.LinuxDoConnectClientSecret,
		LinuxDoConnectRedirectURL:        req.LinuxDoConnectRedirectURL,
		OIDCConnectEnabled:               req.OIDCConnectEnabled,
		OIDCConnectProviderName:          req.OIDCConnectProviderName,
		OIDCConnectClientID:              req.OIDCConnectClientID,
		OIDCConnectClientSecret:          req.OIDCConnectClientSecret,
		OIDCConnectIssuerURL:             req.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:          req.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:          req.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:              req.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:           req.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:               req.OIDCConnectJWKSURL,
		OIDCConnectScopes:                req.OIDCConnectScopes,
		OIDCConnectRedirectURL:           req.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:   req.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:       req.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:               req.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:       req.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:    req.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:      req.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:  req.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:     req.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:        req.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:  req.OIDCConnectUserInfoUsernamePath,
		GitHubOAuthEnabled:               req.GitHubOAuthEnabled,
		GitHubOAuthClientID:              req.GitHubOAuthClientID,
		GitHubOAuthClientSecret:          req.GitHubOAuthClientSecret,
		GitHubOAuthRedirectURL:           req.GitHubOAuthRedirectURL,
		GitHubOAuthFrontendRedirectURL:   req.GitHubOAuthFrontendRedirectURL,
		SiteName:                         req.SiteName,
		SiteLogo:                         req.SiteLogo,
		SiteSubtitle:                     req.SiteSubtitle,
		APIBaseURL:                       req.APIBaseURL,
		ContactInfo:                      req.ContactInfo,
		TechSupportQRCode:                req.TechSupportQRCode,
		AfterSalesQRCode:                 req.AfterSalesQRCode,
		DocURL:                           req.DocURL,
		ChatbotURL:                       req.ChatbotURL,
		HomeContent:                      req.HomeContent,
		LandingReportsEnabled:            landingReportsEnabled,
		LandingPricingProMultiplier:      landingPricingProMultiplier,
		LandingPricingMaxMultiplier:      landingPricingMaxMultiplier,
		LandingPricingExchangeRate:       landingPricingExchangeRate,
		HideCcsImportButton:              req.HideCcsImportButton,
		PurchaseSubscriptionEnabled:      purchaseEnabled,
		PurchaseSubscriptionURL:          purchaseURL,
		CardShopEnabled:                  cardShopEnabled,
		CardShopProducts:                 cardShopProducts,
		InvoiceManagementEnabled:         invoiceManagementEnabled,
		FeedbackManagementEnabled:        feedbackManagementEnabled,
		GroupCacheHitRateEnabled:         groupCacheHitRateEnabled,
		SoraClientEnabled:                req.SoraClientEnabled,
		TableDefaultPageSize:             req.TableDefaultPageSize,
		TablePageSizeOptions:             req.TablePageSizeOptions,
		CustomMenuItems:                  customMenuJSON,
		CustomEndpoints:                  customEndpointsJSON,
		DefaultConcurrency:               req.DefaultConcurrency,
		DefaultBalance:                   req.DefaultBalance,
		DefaultSubscriptions:             defaultSubscriptions,
		EnableModelFallback:              req.EnableModelFallback,
		FallbackModelAnthropic:           req.FallbackModelAnthropic,
		FallbackModelOpenAI:              req.FallbackModelOpenAI,
		FallbackModelGemini:              req.FallbackModelGemini,
		FallbackModelAntigravity:         req.FallbackModelAntigravity,
		EnableIdentityPatch:              req.EnableIdentityPatch,
		IdentityPatchPrompt:              req.IdentityPatchPrompt,
		MinClaudeCodeVersion:             req.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:             req.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:      req.AllowUngroupedKeyScheduling,
		BackendModeEnabled:               req.BackendModeEnabled,
		OpsMonitoringEnabled: func() bool {
			if req.OpsMonitoringEnabled != nil {
				return *req.OpsMonitoringEnabled
			}
			return previousSettings.OpsMonitoringEnabled
		}(),
		OpsRealtimeMonitoringEnabled: func() bool {
			if req.OpsRealtimeMonitoringEnabled != nil {
				return *req.OpsRealtimeMonitoringEnabled
			}
			return previousSettings.OpsRealtimeMonitoringEnabled
		}(),
		OpsQueryModeDefault: func() string {
			if req.OpsQueryModeDefault != nil {
				return *req.OpsQueryModeDefault
			}
			return previousSettings.OpsQueryModeDefault
		}(),
		OpsMetricsIntervalSeconds: func() int {
			if req.OpsMetricsIntervalSeconds != nil {
				return *req.OpsMetricsIntervalSeconds
			}
			return previousSettings.OpsMetricsIntervalSeconds
		}(),
		EnableFingerprintUnification: func() bool {
			if req.EnableFingerprintUnification != nil {
				return *req.EnableFingerprintUnification
			}
			return previousSettings.EnableFingerprintUnification
		}(),
		EnableMetadataPassthrough: func() bool {
			if req.EnableMetadataPassthrough != nil {
				return *req.EnableMetadataPassthrough
			}
			return previousSettings.EnableMetadataPassthrough
		}(),
		EnableCCHSigning: func() bool {
			if req.EnableCCHSigning != nil {
				return *req.EnableCCHSigning
			}
			return previousSettings.EnableCCHSigning
		}(),
		EnableAnthropicCacheTTL1hInjection: func() bool {
			if req.EnableAnthropicCacheTTL1hInjection != nil {
				return *req.EnableAnthropicCacheTTL1hInjection
			}
			return previousSettings.EnableAnthropicCacheTTL1hInjection
		}(),
		BalanceAlertEnabled: func() bool {
			if req.BalanceAlertEnabled != nil {
				return *req.BalanceAlertEnabled
			}
			return previousSettings.BalanceAlertEnabled
		}(),
		BalanceAlertDefaultThreshold: func() float64 {
			if req.BalanceAlertDefaultThreshold != nil {
				return *req.BalanceAlertDefaultThreshold
			}
			return previousSettings.BalanceAlertDefaultThreshold
		}(),
		AccountQuotaNotifyEnabled: func() bool {
			if req.AccountQuotaNotifyEnabled != nil {
				return *req.AccountQuotaNotifyEnabled
			}
			return previousSettings.AccountQuotaNotifyEnabled
		}(),
		AccountQuotaNotifyEmails: func() []service.NotifyEmailEntry {
			if req.AccountQuotaNotifyEmails != nil {
				return dto.NotifyEmailEntriesToService(*req.AccountQuotaNotifyEmails)
			}
			return previousSettings.AccountQuotaNotifyEmails
		}(),
		StripeEnabled:       req.StripeEnabled,
		StripeSecretKey:     req.StripeSecretKey,
		StripeWebhookSecret: req.StripeWebhookSecret,
		AlipayEnabled:       req.AlipayEnabled,
		AlipayAppID:         req.AlipayAppID,
		AlipayPrivateKey:    req.AlipayPrivateKey,
		AlipayPublicKey:     req.AlipayPublicKey,
		AlipayNotifyURL:     req.AlipayNotifyURL,
		XunhuAlipayEnabled:  req.XunhuAlipayEnabled,
		XunhuAlipayAppID:    req.XunhuAlipayAppID,
		XunhuAlipayKey:      req.XunhuAlipayKey,
		XunhuWechatEnabled:  req.XunhuWechatEnabled,
		XunhuWechatAppID:    req.XunhuWechatAppID,
		XunhuWechatKey:      req.XunhuWechatKey,
		XunhuNotifyURL:      req.XunhuNotifyURL,
		EasyPayEnabled:      req.EasyPayEnabled,
		EasyPayPID:          req.EasyPayPID,
		EasyPayKey:          req.EasyPayKey,
		EasyPayAPIBase:      req.EasyPayAPIBase,
		TopupAlipayProvider: req.TopupAlipayProvider,
		TopupWechatProvider: req.TopupWechatProvider,
	}

	if err := h.settingService.UpdateSettingsPartial(c.Request.Context(), settings, providedSettingKeys); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	h.auditSettingsUpdate(c, previousSettings, settings, req)

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updatedDefaultSubscriptions := make([]dto.DefaultSubscriptionSetting, 0, len(updatedSettings.DefaultSubscriptions))
	for _, sub := range updatedSettings.DefaultSubscriptions {
		updatedDefaultSubscriptions = append(updatedDefaultSubscriptions, dto.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	response.Success(c, dto.SystemSettings{
		RegistrationEnabled:                  updatedSettings.RegistrationEnabled,
		EmailVerifyEnabled:                   updatedSettings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist:     updatedSettings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                     updatedSettings.PromoCodeEnabled,
		PasswordResetEnabled:                 updatedSettings.PasswordResetEnabled,
		FrontendURL:                          updatedSettings.FrontendURL,
		InvitationCodeEnabled:                updatedSettings.InvitationCodeEnabled,
		TotpEnabled:                          updatedSettings.TotpEnabled,
		TotpEncryptionKeyConfigured:          h.settingService.IsTotpEncryptionKeyConfigured(),
		SMTPHost:                             updatedSettings.SMTPHost,
		SMTPPort:                             updatedSettings.SMTPPort,
		SMTPUsername:                         updatedSettings.SMTPUsername,
		SMTPPasswordConfigured:               updatedSettings.SMTPPasswordConfigured,
		SMTPFrom:                             updatedSettings.SMTPFrom,
		SMTPFromName:                         updatedSettings.SMTPFromName,
		SMTPUseTLS:                           updatedSettings.SMTPUseTLS,
		FeedbackNotifyEmail:                  updatedSettings.FeedbackNotifyEmail,
		TurnstileEnabled:                     updatedSettings.TurnstileEnabled,
		TurnstileSiteKey:                     updatedSettings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:         updatedSettings.TurnstileSecretKeyConfigured,
		LinuxDoConnectEnabled:                updatedSettings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:               updatedSettings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured: updatedSettings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:            updatedSettings.LinuxDoConnectRedirectURL,
		OIDCConnectEnabled:                   updatedSettings.OIDCConnectEnabled,
		OIDCConnectProviderName:              updatedSettings.OIDCConnectProviderName,
		OIDCConnectClientID:                  updatedSettings.OIDCConnectClientID,
		OIDCConnectClientSecretConfigured:    updatedSettings.OIDCConnectClientSecretConfigured,
		OIDCConnectIssuerURL:                 updatedSettings.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:              updatedSettings.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:              updatedSettings.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:                  updatedSettings.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:               updatedSettings.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:                   updatedSettings.OIDCConnectJWKSURL,
		OIDCConnectScopes:                    updatedSettings.OIDCConnectScopes,
		OIDCConnectRedirectURL:               updatedSettings.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:       updatedSettings.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:           updatedSettings.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                   updatedSettings.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:           updatedSettings.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:        updatedSettings.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:          updatedSettings.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:      updatedSettings.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:         updatedSettings.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:            updatedSettings.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:      updatedSettings.OIDCConnectUserInfoUsernamePath,
		GitHubOAuthEnabled:                   updatedSettings.GitHubOAuthEnabled,
		GitHubOAuthClientID:                  updatedSettings.GitHubOAuthClientID,
		GitHubOAuthClientSecretConfigured:    updatedSettings.GitHubOAuthClientSecretConfigured,
		GitHubOAuthRedirectURL:               updatedSettings.GitHubOAuthRedirectURL,
		GitHubOAuthFrontendRedirectURL:       updatedSettings.GitHubOAuthFrontendRedirectURL,
		SiteName:                             updatedSettings.SiteName,
		SiteLogo:                             updatedSettings.SiteLogo,
		SiteSubtitle:                         updatedSettings.SiteSubtitle,
		APIBaseURL:                           updatedSettings.APIBaseURL,
		ContactInfo:                          updatedSettings.ContactInfo,
		TechSupportQRCode:                    updatedSettings.TechSupportQRCode,
		AfterSalesQRCode:                     updatedSettings.AfterSalesQRCode,
		DocURL:                               updatedSettings.DocURL,
		ChatbotURL:                           updatedSettings.ChatbotURL,
		HomeContent:                          updatedSettings.HomeContent,
		LandingReportsEnabled:                updatedSettings.LandingReportsEnabled,
		LandingPricingProMultiplier:          updatedSettings.LandingPricingProMultiplier,
		LandingPricingMaxMultiplier:          updatedSettings.LandingPricingMaxMultiplier,
		LandingPricingExchangeRate:           updatedSettings.LandingPricingExchangeRate,
		HideCcsImportButton:                  updatedSettings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:          updatedSettings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:              updatedSettings.PurchaseSubscriptionURL,
		CardShopEnabled:                      updatedSettings.CardShopEnabled,
		CardShopProducts:                     dto.CardShopProductsFromService(updatedSettings.CardShopProducts),
		InvoiceManagementEnabled:             updatedSettings.InvoiceManagementEnabled,
		FeedbackManagementEnabled:            updatedSettings.FeedbackManagementEnabled,
		GroupCacheHitRateEnabled:             updatedSettings.GroupCacheHitRateEnabled,
		SoraClientEnabled:                    updatedSettings.SoraClientEnabled,
		TableDefaultPageSize:                 updatedSettings.TableDefaultPageSize,
		TablePageSizeOptions:                 updatedSettings.TablePageSizeOptions,
		CustomMenuItems:                      dto.ParseCustomMenuItems(updatedSettings.CustomMenuItems),
		CustomEndpoints:                      dto.ParseCustomEndpoints(updatedSettings.CustomEndpoints),
		DefaultConcurrency:                   updatedSettings.DefaultConcurrency,
		DefaultBalance:                       updatedSettings.DefaultBalance,
		DefaultSubscriptions:                 updatedDefaultSubscriptions,
		EnableModelFallback:                  updatedSettings.EnableModelFallback,
		FallbackModelAnthropic:               updatedSettings.FallbackModelAnthropic,
		FallbackModelOpenAI:                  updatedSettings.FallbackModelOpenAI,
		FallbackModelGemini:                  updatedSettings.FallbackModelGemini,
		FallbackModelAntigravity:             updatedSettings.FallbackModelAntigravity,
		EnableIdentityPatch:                  updatedSettings.EnableIdentityPatch,
		IdentityPatchPrompt:                  updatedSettings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                 updatedSettings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:         updatedSettings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                  updatedSettings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:            updatedSettings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                 updatedSettings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                 updatedSettings.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:          updatedSettings.AllowUngroupedKeyScheduling,
		BackendModeEnabled:                   updatedSettings.BackendModeEnabled,
		EnableFingerprintUnification:         updatedSettings.EnableFingerprintUnification,
		EnableMetadataPassthrough:            updatedSettings.EnableMetadataPassthrough,
		EnableCCHSigning:                     updatedSettings.EnableCCHSigning,
		EnableAnthropicCacheTTL1hInjection:   updatedSettings.EnableAnthropicCacheTTL1hInjection,
		StripeEnabled:                        updatedSettings.StripeEnabled,
		StripeSecretKeyConfigured:            updatedSettings.StripeSecretKeyConfigured,
		StripeWebhookSecretConfigured:        updatedSettings.StripeWebhookSecretConfigured,
		AlipayEnabled:                        updatedSettings.AlipayEnabled,
		AlipayAppID:                          updatedSettings.AlipayAppID,
		AlipayPrivateKeyConfigured:           updatedSettings.AlipayPrivateKeyConfigured,
		AlipayPublicKeyConfigured:            updatedSettings.AlipayPublicKeyConfigured,
		AlipayNotifyURL:                      updatedSettings.AlipayNotifyURL,
		XunhuAlipayEnabled:                   updatedSettings.XunhuAlipayEnabled,
		XunhuAlipayAppID:                     updatedSettings.XunhuAlipayAppID,
		XunhuAlipayKeyConfigured:             updatedSettings.XunhuAlipayKeyConfigured,
		XunhuWechatEnabled:                   updatedSettings.XunhuWechatEnabled,
		XunhuWechatAppID:                     updatedSettings.XunhuWechatAppID,
		XunhuWechatKeyConfigured:             updatedSettings.XunhuWechatKeyConfigured,
		XunhuNotifyURL:                       updatedSettings.XunhuNotifyURL,
		EasyPayEnabled:                       updatedSettings.EasyPayEnabled,
		EasyPayPID:                           updatedSettings.EasyPayPID,
		EasyPayKeyConfigured:                 updatedSettings.EasyPayKeyConfigured,
		EasyPayAPIBase:                       updatedSettings.EasyPayAPIBase,
		TopupAlipayProvider:                  updatedSettings.TopupAlipayProvider,
		TopupWechatProvider:                  updatedSettings.TopupWechatProvider,
		BalanceAlertEnabled:                  updatedSettings.BalanceAlertEnabled,
		BalanceAlertDefaultThreshold:         updatedSettings.BalanceAlertDefaultThreshold,
		AccountQuotaNotifyEnabled:            updatedSettings.AccountQuotaNotifyEnabled,
		AccountQuotaNotifyEmails:             dto.NotifyEmailEntriesFromService(updatedSettings.AccountQuotaNotifyEmails),
	})
}

type UserMenuVisibilityRequest struct {
	InvoiceManagementEnabled  bool  `json:"invoice_management_enabled"`
	FeedbackManagementEnabled bool  `json:"feedback_management_enabled"`
	GroupCacheHitRateEnabled  bool  `json:"group_cache_hit_rate_enabled"`
	LandingReportsEnabled     *bool `json:"landing_reports_enabled"`
}

func (h *SettingHandler) GetUserMenuVisibilitySettings(c *gin.Context) {
	settings, err := h.settingService.GetUserMenuVisibilitySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"invoice_management_enabled":   settings.InvoiceManagementEnabled,
		"feedback_management_enabled":  settings.FeedbackManagementEnabled,
		"group_cache_hit_rate_enabled": settings.GroupCacheHitRateEnabled,
		"landing_reports_enabled":      settings.LandingReportsEnabled,
	})
}

func (h *SettingHandler) UpdateUserMenuVisibilitySettings(c *gin.Context) {
	var req UserMenuVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	current, err := h.settingService.GetUserMenuVisibilitySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	landingReportsEnabled := current.LandingReportsEnabled
	if req.LandingReportsEnabled != nil {
		landingReportsEnabled = *req.LandingReportsEnabled
	}

	settings := service.UserMenuVisibilitySettings{
		InvoiceManagementEnabled:  req.InvoiceManagementEnabled,
		FeedbackManagementEnabled: req.FeedbackManagementEnabled,
		GroupCacheHitRateEnabled:  req.GroupCacheHitRateEnabled,
		LandingReportsEnabled:     landingReportsEnabled,
	}
	if err := h.settingService.UpdateUserMenuVisibilitySettings(c.Request.Context(), settings); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"invoice_management_enabled":   settings.InvoiceManagementEnabled,
		"feedback_management_enabled":  settings.FeedbackManagementEnabled,
		"group_cache_hit_rate_enabled": settings.GroupCacheHitRateEnabled,
		"landing_reports_enabled":      settings.LandingReportsEnabled,
	})
}

func (h *SettingHandler) auditSettingsUpdate(c *gin.Context, before *service.SystemSettings, after *service.SystemSettings, req UpdateSettingsRequest) {
	if before == nil || after == nil {
		return
	}

	changed := diffSettings(before, after, req)
	if len(changed) == 0 {
		return
	}

	subject, _ := middleware.GetAuthSubjectFromContext(c)
	role, _ := middleware.GetUserRoleFromContext(c)
	slog.Info("settings updated",
		"audit", true,
		"user_id", subject.UserID,
		"role", role,
		"changed", changed,
	)
}

func diffSettings(before *service.SystemSettings, after *service.SystemSettings, req UpdateSettingsRequest) []string {
	changed := make([]string, 0, 20)
	if before.RegistrationEnabled != after.RegistrationEnabled {
		changed = append(changed, "registration_enabled")
	}
	if before.EmailVerifyEnabled != after.EmailVerifyEnabled {
		changed = append(changed, "email_verify_enabled")
	}
	if !equalStringSlice(before.RegistrationEmailSuffixWhitelist, after.RegistrationEmailSuffixWhitelist) {
		changed = append(changed, "registration_email_suffix_whitelist")
	}
	if before.PromoCodeEnabled != after.PromoCodeEnabled {
		changed = append(changed, "promo_code_enabled")
	}
	if before.InvitationCodeEnabled != after.InvitationCodeEnabled {
		changed = append(changed, "invitation_code_enabled")
	}
	if before.PasswordResetEnabled != after.PasswordResetEnabled {
		changed = append(changed, "password_reset_enabled")
	}
	if before.FrontendURL != after.FrontendURL {
		changed = append(changed, "frontend_url")
	}
	if before.TotpEnabled != after.TotpEnabled {
		changed = append(changed, "totp_enabled")
	}
	if before.SMTPHost != after.SMTPHost {
		changed = append(changed, "smtp_host")
	}
	if before.SMTPPort != after.SMTPPort {
		changed = append(changed, "smtp_port")
	}
	if before.SMTPUsername != after.SMTPUsername {
		changed = append(changed, "smtp_username")
	}
	if req.nonEmptySecretProvided("smtp_password") {
		changed = append(changed, "smtp_password")
	}
	if before.SMTPFrom != after.SMTPFrom {
		changed = append(changed, "smtp_from_email")
	}
	if before.SMTPFromName != after.SMTPFromName {
		changed = append(changed, "smtp_from_name")
	}
	if before.SMTPUseTLS != after.SMTPUseTLS {
		changed = append(changed, "smtp_use_tls")
	}
	if before.TurnstileEnabled != after.TurnstileEnabled {
		changed = append(changed, "turnstile_enabled")
	}
	if before.TurnstileSiteKey != after.TurnstileSiteKey {
		changed = append(changed, "turnstile_site_key")
	}
	if req.nonEmptySecretProvided("turnstile_secret_key") {
		changed = append(changed, "turnstile_secret_key")
	}
	if before.LinuxDoConnectEnabled != after.LinuxDoConnectEnabled {
		changed = append(changed, "linuxdo_connect_enabled")
	}
	if before.LinuxDoConnectClientID != after.LinuxDoConnectClientID {
		changed = append(changed, "linuxdo_connect_client_id")
	}
	if req.nonEmptySecretProvided("linuxdo_connect_client_secret") {
		changed = append(changed, "linuxdo_connect_client_secret")
	}
	if before.LinuxDoConnectRedirectURL != after.LinuxDoConnectRedirectURL {
		changed = append(changed, "linuxdo_connect_redirect_url")
	}
	if before.OIDCConnectEnabled != after.OIDCConnectEnabled {
		changed = append(changed, "oidc_connect_enabled")
	}
	if before.OIDCConnectProviderName != after.OIDCConnectProviderName {
		changed = append(changed, "oidc_connect_provider_name")
	}
	if before.OIDCConnectClientID != after.OIDCConnectClientID {
		changed = append(changed, "oidc_connect_client_id")
	}
	if req.nonEmptySecretProvided("oidc_connect_client_secret") {
		changed = append(changed, "oidc_connect_client_secret")
	}
	if before.OIDCConnectIssuerURL != after.OIDCConnectIssuerURL {
		changed = append(changed, "oidc_connect_issuer_url")
	}
	if before.OIDCConnectDiscoveryURL != after.OIDCConnectDiscoveryURL {
		changed = append(changed, "oidc_connect_discovery_url")
	}
	if before.OIDCConnectAuthorizeURL != after.OIDCConnectAuthorizeURL {
		changed = append(changed, "oidc_connect_authorize_url")
	}
	if before.OIDCConnectTokenURL != after.OIDCConnectTokenURL {
		changed = append(changed, "oidc_connect_token_url")
	}
	if before.OIDCConnectUserInfoURL != after.OIDCConnectUserInfoURL {
		changed = append(changed, "oidc_connect_userinfo_url")
	}
	if before.OIDCConnectJWKSURL != after.OIDCConnectJWKSURL {
		changed = append(changed, "oidc_connect_jwks_url")
	}
	if before.OIDCConnectScopes != after.OIDCConnectScopes {
		changed = append(changed, "oidc_connect_scopes")
	}
	if before.OIDCConnectRedirectURL != after.OIDCConnectRedirectURL {
		changed = append(changed, "oidc_connect_redirect_url")
	}
	if before.OIDCConnectFrontendRedirectURL != after.OIDCConnectFrontendRedirectURL {
		changed = append(changed, "oidc_connect_frontend_redirect_url")
	}
	if before.OIDCConnectTokenAuthMethod != after.OIDCConnectTokenAuthMethod {
		changed = append(changed, "oidc_connect_token_auth_method")
	}
	if before.OIDCConnectUsePKCE != after.OIDCConnectUsePKCE {
		changed = append(changed, "oidc_connect_use_pkce")
	}
	if before.OIDCConnectValidateIDToken != after.OIDCConnectValidateIDToken {
		changed = append(changed, "oidc_connect_validate_id_token")
	}
	if before.OIDCConnectAllowedSigningAlgs != after.OIDCConnectAllowedSigningAlgs {
		changed = append(changed, "oidc_connect_allowed_signing_algs")
	}
	if before.OIDCConnectClockSkewSeconds != after.OIDCConnectClockSkewSeconds {
		changed = append(changed, "oidc_connect_clock_skew_seconds")
	}
	if before.OIDCConnectRequireEmailVerified != after.OIDCConnectRequireEmailVerified {
		changed = append(changed, "oidc_connect_require_email_verified")
	}
	if before.OIDCConnectUserInfoEmailPath != after.OIDCConnectUserInfoEmailPath {
		changed = append(changed, "oidc_connect_userinfo_email_path")
	}
	if before.OIDCConnectUserInfoIDPath != after.OIDCConnectUserInfoIDPath {
		changed = append(changed, "oidc_connect_userinfo_id_path")
	}
	if before.OIDCConnectUserInfoUsernamePath != after.OIDCConnectUserInfoUsernamePath {
		changed = append(changed, "oidc_connect_userinfo_username_path")
	}
	if before.GitHubOAuthEnabled != after.GitHubOAuthEnabled {
		changed = append(changed, "github_oauth_enabled")
	}
	if before.GitHubOAuthClientID != after.GitHubOAuthClientID {
		changed = append(changed, "github_oauth_client_id")
	}
	if req.nonEmptySecretProvided("github_oauth_client_secret") {
		changed = append(changed, "github_oauth_client_secret")
	}
	if before.GitHubOAuthRedirectURL != after.GitHubOAuthRedirectURL {
		changed = append(changed, "github_oauth_redirect_url")
	}
	if before.GitHubOAuthFrontendRedirectURL != after.GitHubOAuthFrontendRedirectURL {
		changed = append(changed, "github_oauth_frontend_redirect_url")
	}
	if before.SiteName != after.SiteName {
		changed = append(changed, "site_name")
	}
	if before.SiteLogo != after.SiteLogo {
		changed = append(changed, "site_logo")
	}
	if before.SiteSubtitle != after.SiteSubtitle {
		changed = append(changed, "site_subtitle")
	}
	if before.APIBaseURL != after.APIBaseURL {
		changed = append(changed, "api_base_url")
	}
	if before.ContactInfo != after.ContactInfo {
		changed = append(changed, "contact_info")
	}
	if before.TechSupportQRCode != after.TechSupportQRCode {
		changed = append(changed, "tech_support_qrcode")
	}
	if before.AfterSalesQRCode != after.AfterSalesQRCode {
		changed = append(changed, "after_sales_qrcode")
	}
	if before.DocURL != after.DocURL {
		changed = append(changed, "doc_url")
	}
	if before.ChatbotURL != after.ChatbotURL {
		changed = append(changed, "chatbot_url")
	}
	if before.HomeContent != after.HomeContent {
		changed = append(changed, "home_content")
	}
	if before.LandingReportsEnabled != after.LandingReportsEnabled {
		changed = append(changed, "landing_reports_enabled")
	}
	if before.LandingPricingProMultiplier != after.LandingPricingProMultiplier {
		changed = append(changed, "landing_pricing_pro_multiplier")
	}
	if before.LandingPricingMaxMultiplier != after.LandingPricingMaxMultiplier {
		changed = append(changed, "landing_pricing_max_multiplier")
	}
	if before.LandingPricingExchangeRate != after.LandingPricingExchangeRate {
		changed = append(changed, "landing_pricing_exchange_rate")
	}
	if before.HideCcsImportButton != after.HideCcsImportButton {
		changed = append(changed, "hide_ccs_import_button")
	}
	if before.DefaultConcurrency != after.DefaultConcurrency {
		changed = append(changed, "default_concurrency")
	}
	if before.DefaultBalance != after.DefaultBalance {
		changed = append(changed, "default_balance")
	}
	if !equalDefaultSubscriptions(before.DefaultSubscriptions, after.DefaultSubscriptions) {
		changed = append(changed, "default_subscriptions")
	}
	if before.EnableModelFallback != after.EnableModelFallback {
		changed = append(changed, "enable_model_fallback")
	}
	if before.FallbackModelAnthropic != after.FallbackModelAnthropic {
		changed = append(changed, "fallback_model_anthropic")
	}
	if before.FallbackModelOpenAI != after.FallbackModelOpenAI {
		changed = append(changed, "fallback_model_openai")
	}
	if before.FallbackModelGemini != after.FallbackModelGemini {
		changed = append(changed, "fallback_model_gemini")
	}
	if before.FallbackModelAntigravity != after.FallbackModelAntigravity {
		changed = append(changed, "fallback_model_antigravity")
	}
	if before.EnableIdentityPatch != after.EnableIdentityPatch {
		changed = append(changed, "enable_identity_patch")
	}
	if before.IdentityPatchPrompt != after.IdentityPatchPrompt {
		changed = append(changed, "identity_patch_prompt")
	}
	if before.OpsMonitoringEnabled != after.OpsMonitoringEnabled {
		changed = append(changed, "ops_monitoring_enabled")
	}
	if before.OpsRealtimeMonitoringEnabled != after.OpsRealtimeMonitoringEnabled {
		changed = append(changed, "ops_realtime_monitoring_enabled")
	}
	if before.OpsQueryModeDefault != after.OpsQueryModeDefault {
		changed = append(changed, "ops_query_mode_default")
	}
	if before.OpsMetricsIntervalSeconds != after.OpsMetricsIntervalSeconds {
		changed = append(changed, "ops_metrics_interval_seconds")
	}
	if before.MinClaudeCodeVersion != after.MinClaudeCodeVersion {
		changed = append(changed, "min_claude_code_version")
	}
	if before.MaxClaudeCodeVersion != after.MaxClaudeCodeVersion {
		changed = append(changed, "max_claude_code_version")
	}
	if before.AllowUngroupedKeyScheduling != after.AllowUngroupedKeyScheduling {
		changed = append(changed, "allow_ungrouped_key_scheduling")
	}
	if before.BackendModeEnabled != after.BackendModeEnabled {
		changed = append(changed, "backend_mode_enabled")
	}
	if before.PurchaseSubscriptionEnabled != after.PurchaseSubscriptionEnabled {
		changed = append(changed, "purchase_subscription_enabled")
	}
	if before.PurchaseSubscriptionURL != after.PurchaseSubscriptionURL {
		changed = append(changed, "purchase_subscription_url")
	}
	if before.CardShopEnabled != after.CardShopEnabled {
		changed = append(changed, "card_shop_enabled")
	}
	if !equalCardShopProducts(before.CardShopProducts, after.CardShopProducts) {
		changed = append(changed, "card_shop_products")
	}
	if before.InvoiceManagementEnabled != after.InvoiceManagementEnabled {
		changed = append(changed, "invoice_management_enabled")
	}
	if before.FeedbackManagementEnabled != after.FeedbackManagementEnabled {
		changed = append(changed, "feedback_management_enabled")
	}
	if before.GroupCacheHitRateEnabled != after.GroupCacheHitRateEnabled {
		changed = append(changed, "group_cache_hit_rate_enabled")
	}
	if before.TableDefaultPageSize != after.TableDefaultPageSize {
		changed = append(changed, "table_default_page_size")
	}
	if !equalIntSlice(before.TablePageSizeOptions, after.TablePageSizeOptions) {
		changed = append(changed, "table_page_size_options")
	}
	if before.CustomMenuItems != after.CustomMenuItems {
		changed = append(changed, "custom_menu_items")
	}
	if before.CustomEndpoints != after.CustomEndpoints {
		changed = append(changed, "custom_endpoints")
	}
	if before.EnableFingerprintUnification != after.EnableFingerprintUnification {
		changed = append(changed, "enable_fingerprint_unification")
	}
	if before.EnableMetadataPassthrough != after.EnableMetadataPassthrough {
		changed = append(changed, "enable_metadata_passthrough")
	}
	if before.EnableCCHSigning != after.EnableCCHSigning {
		changed = append(changed, "enable_cch_signing")
	}
	if before.EnableAnthropicCacheTTL1hInjection != after.EnableAnthropicCacheTTL1hInjection {
		changed = append(changed, "enable_anthropic_cache_ttl_1h_injection")
	}
	// Balance & quota notification
	if before.BalanceAlertEnabled != after.BalanceAlertEnabled {
		changed = append(changed, "balance_alert_enabled")
	}
	if before.BalanceAlertDefaultThreshold != after.BalanceAlertDefaultThreshold {
		changed = append(changed, "balance_alert_default_threshold")
	}
	if before.AccountQuotaNotifyEnabled != after.AccountQuotaNotifyEnabled {
		changed = append(changed, "account_quota_notify_enabled")
	}
	if !equalNotifyEmailEntries(before.AccountQuotaNotifyEmails, after.AccountQuotaNotifyEmails) {
		changed = append(changed, "account_quota_notify_emails")
	}
	return changed
}

func normalizeDefaultSubscriptions(input []dto.DefaultSubscriptionSetting) []dto.DefaultSubscriptionSetting {
	if len(input) == 0 {
		return nil
	}
	normalized := make([]dto.DefaultSubscriptionSetting, 0, len(input))
	for _, item := range input {
		if item.GroupID <= 0 || item.ValidityDays <= 0 {
			continue
		}
		if item.ValidityDays > service.MaxValidityDays {
			item.ValidityDays = service.MaxValidityDays
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalDefaultSubscriptions(a, b []service.DefaultSubscriptionSetting) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].GroupID != b[i].GroupID || a[i].ValidityDays != b[i].ValidityDays {
			return false
		}
	}
	return true
}

func equalCardShopProducts(a, b []service.CardShopProduct) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID ||
			a[i].Label != b[i].Label ||
			a[i].AmountCNY != b[i].AmountCNY ||
			a[i].URL != b[i].URL ||
			a[i].Enabled != b[i].Enabled ||
			a[i].SortOrder != b[i].SortOrder {
			return false
		}
	}
	return true
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalNotifyEmailEntries(a, b []service.NotifyEmailEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Email != b[i].Email || a[i].Verified != b[i].Verified || a[i].Disabled != b[i].Disabled {
			return false
		}
	}
	return true
}

// TestSMTPRequest 测试SMTP连接请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPUseTLS   *bool  `json:"smtp_use_tls"`
}

// TestSMTPConnection 测试SMTP连接
// POST /api/v1/admin/settings/test-smtp
func (h *SettingHandler) TestSMTPConnection(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)

	var savedConfig *service.SMTPConfig
	if cfg, err := h.emailService.GetSMTPConfig(c.Request.Context()); err == nil && cfg != nil {
		savedConfig = cfg
	}

	if req.SMTPHost == "" && savedConfig != nil {
		req.SMTPHost = savedConfig.Host
	}
	if req.SMTPPort <= 0 {
		if savedConfig != nil && savedConfig.Port > 0 {
			req.SMTPPort = savedConfig.Port
		} else {
			req.SMTPPort = 587
		}
	}
	if req.SMTPUsername == "" && savedConfig != nil {
		req.SMTPUsername = savedConfig.Username
	}
	password := strings.TrimSpace(req.SMTPPassword)
	if password == "" && savedConfig != nil {
		password = savedConfig.Password
	}
	useTLS := false
	if req.SMTPUseTLS != nil {
		useTLS = *req.SMTPUseTLS
	} else if savedConfig != nil {
		useTLS = savedConfig.UseTLS
	}
	if req.SMTPHost == "" {
		response.BadRequest(c, "SMTP host is required")
		return
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		UseTLS:   useTLS,
	}

	err := h.emailService.TestSMTPConnectionWithConfig(config)
	if err != nil {
		response.BadRequest(c, "SMTP connection test failed: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "SMTP connection successful"})
}

// SendTestEmailRequest 发送测试邮件请求
type SendTestEmailRequest struct {
	Email        string `json:"email" binding:"required,email"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   *bool  `json:"smtp_use_tls"`
}

// SendTestEmail 发送测试邮件
// POST /api/v1/admin/settings/send-test-email
func (h *SettingHandler) SendTestEmail(c *gin.Context) {
	var req SendTestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
	req.SMTPFromName = strings.TrimSpace(req.SMTPFromName)

	var savedConfig *service.SMTPConfig
	if cfg, err := h.emailService.GetSMTPConfig(c.Request.Context()); err == nil && cfg != nil {
		savedConfig = cfg
	}

	if req.SMTPHost == "" && savedConfig != nil {
		req.SMTPHost = savedConfig.Host
	}
	if req.SMTPPort <= 0 {
		if savedConfig != nil && savedConfig.Port > 0 {
			req.SMTPPort = savedConfig.Port
		} else {
			req.SMTPPort = 587
		}
	}
	if req.SMTPUsername == "" && savedConfig != nil {
		req.SMTPUsername = savedConfig.Username
	}
	password := strings.TrimSpace(req.SMTPPassword)
	if password == "" && savedConfig != nil {
		password = savedConfig.Password
	}
	if req.SMTPFrom == "" && savedConfig != nil {
		req.SMTPFrom = savedConfig.From
	}
	if req.SMTPFromName == "" && savedConfig != nil {
		req.SMTPFromName = savedConfig.FromName
	}
	useTLS := false
	if req.SMTPUseTLS != nil {
		useTLS = *req.SMTPUseTLS
	} else if savedConfig != nil {
		useTLS = savedConfig.UseTLS
	}
	if req.SMTPHost == "" {
		response.BadRequest(c, "SMTP host is required")
		return
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		From:     req.SMTPFrom,
		FromName: req.SMTPFromName,
		UseTLS:   useTLS,
	}

	siteName := h.settingService.GetSiteName(c.Request.Context())
	subject := "[" + siteName + "] Test Email"
	body := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; }
        .content { padding: 40px 30px; text-align: center; }
        .success { color: #10b981; font-size: 48px; margin-bottom: 20px; }
        .footer { background-color: #f8f9fa; padding: 20px; text-align: center; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + siteName + `</h1>
        </div>
        <div class="content">
            <div class="success">✓</div>
            <h2>Email Configuration Successful!</h2>
            <p>This is a test email to verify your SMTP settings are working correctly.</p>
        </div>
        <div class="footer">
            <p>This is an automated test message.</p>
        </div>
    </div>
</body>
</html>
`

	if err := h.emailService.SendEmailWithConfig(config, req.Email, subject, body); err != nil {
		response.BadRequest(c, "Failed to send test email: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Test email sent successfully"})
}

// GetAdminAPIKey 获取管理员 API Key 状态
// GET /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) GetAdminAPIKey(c *gin.Context) {
	maskedKey, exists, err := h.settingService.GetAdminAPIKeyStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"exists":     exists,
		"masked_key": maskedKey,
	})
}

// RegenerateAdminAPIKey 生成/重新生成管理员 API Key
// POST /api/v1/admin/settings/admin-api-key/regenerate
func (h *SettingHandler) RegenerateAdminAPIKey(c *gin.Context) {
	key, err := h.settingService.GenerateAdminAPIKey(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"key": key, // 完整 key 只在生成时返回一次
	})
}

// DeleteAdminAPIKey 删除管理员 API Key
// DELETE /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) DeleteAdminAPIKey(c *gin.Context) {
	if err := h.settingService.DeleteAdminAPIKey(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Admin API key deleted"})
}

// GetOverloadCooldownSettings 获取529过载冷却配置
// GET /api/v1/admin/settings/overload-cooldown
func (h *SettingHandler) GetOverloadCooldownSettings(c *gin.Context) {
	settings, err := h.settingService.GetOverloadCooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OverloadCooldownSettings{
		Enabled:         settings.Enabled,
		CooldownMinutes: settings.CooldownMinutes,
	})
}

// UpdateOverloadCooldownSettingsRequest 更新529过载冷却配置请求
type UpdateOverloadCooldownSettingsRequest struct {
	Enabled         bool `json:"enabled"`
	CooldownMinutes int  `json:"cooldown_minutes"`
}

// UpdateOverloadCooldownSettings 更新529过载冷却配置
// PUT /api/v1/admin/settings/overload-cooldown
func (h *SettingHandler) UpdateOverloadCooldownSettings(c *gin.Context) {
	var req UpdateOverloadCooldownSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.OverloadCooldownSettings{
		Enabled:         req.Enabled,
		CooldownMinutes: req.CooldownMinutes,
	}

	if err := h.settingService.SetOverloadCooldownSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updatedSettings, err := h.settingService.GetOverloadCooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OverloadCooldownSettings{
		Enabled:         updatedSettings.Enabled,
		CooldownMinutes: updatedSettings.CooldownMinutes,
	})
}

// GetStreamTimeoutSettings 获取流超时处理配置
// GET /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) GetStreamTimeoutSettings(c *gin.Context) {
	settings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                settings.Enabled,
		Action:                 settings.Action,
		TempUnschedMinutes:     settings.TempUnschedMinutes,
		ThresholdCount:         settings.ThresholdCount,
		ThresholdWindowMinutes: settings.ThresholdWindowMinutes,
	})
}

// GetRectifierSettings 获取请求整流器配置
// GET /api/v1/admin/settings/rectifier
func (h *SettingHandler) GetRectifierSettings(c *gin.Context) {
	settings, err := h.settingService.GetRectifierSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	patterns := settings.APIKeySignaturePatterns
	if patterns == nil {
		patterns = []string{}
	}
	response.Success(c, dto.RectifierSettings{
		Enabled:                  settings.Enabled,
		ThinkingSignatureEnabled: settings.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    settings.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   settings.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  patterns,
	})
}

// UpdateRectifierSettingsRequest 更新整流器配置请求
type UpdateRectifierSettingsRequest struct {
	Enabled                  bool     `json:"enabled"`
	ThinkingSignatureEnabled bool     `json:"thinking_signature_enabled"`
	ThinkingBudgetEnabled    bool     `json:"thinking_budget_enabled"`
	APIKeySignatureEnabled   bool     `json:"apikey_signature_enabled"`
	APIKeySignaturePatterns  []string `json:"apikey_signature_patterns"`
}

// UpdateRectifierSettings 更新请求整流器配置
// PUT /api/v1/admin/settings/rectifier
func (h *SettingHandler) UpdateRectifierSettings(c *gin.Context) {
	var req UpdateRectifierSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 校验并清理自定义匹配关键词
	const maxPatterns = 50
	const maxPatternLen = 500
	if len(req.APIKeySignaturePatterns) > maxPatterns {
		response.BadRequest(c, "Too many signature patterns (max 50)")
		return
	}
	var cleanedPatterns []string
	for _, p := range req.APIKeySignaturePatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) > maxPatternLen {
			response.BadRequest(c, "Signature pattern too long (max 500 characters)")
			return
		}
		cleanedPatterns = append(cleanedPatterns, p)
	}

	settings := &service.RectifierSettings{
		Enabled:                  req.Enabled,
		ThinkingSignatureEnabled: req.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    req.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   req.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  cleanedPatterns,
	}

	if err := h.settingService.SetRectifierSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetRectifierSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedPatterns := updatedSettings.APIKeySignaturePatterns
	if updatedPatterns == nil {
		updatedPatterns = []string{}
	}
	response.Success(c, dto.RectifierSettings{
		Enabled:                  updatedSettings.Enabled,
		ThinkingSignatureEnabled: updatedSettings.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    updatedSettings.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   updatedSettings.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  updatedPatterns,
	})
}

// GetBetaPolicySettings 获取 Beta 策略配置
// GET /api/v1/admin/settings/beta-policy
func (h *SettingHandler) GetBetaPolicySettings(c *gin.Context) {
	settings, err := h.settingService.GetBetaPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	rules := make([]dto.BetaPolicyRule, len(settings.Rules))
	for i, r := range settings.Rules {
		rules[i] = dto.BetaPolicyRule(r)
	}
	response.Success(c, dto.BetaPolicySettings{Rules: rules})
}

// UpdateBetaPolicySettingsRequest 更新 Beta 策略配置请求
type UpdateBetaPolicySettingsRequest struct {
	Rules []dto.BetaPolicyRule `json:"rules"`
}

// UpdateBetaPolicySettings 更新 Beta 策略配置
// PUT /api/v1/admin/settings/beta-policy
func (h *SettingHandler) UpdateBetaPolicySettings(c *gin.Context) {
	var req UpdateBetaPolicySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	rules := make([]service.BetaPolicyRule, len(req.Rules))
	for i, r := range req.Rules {
		rules[i] = service.BetaPolicyRule(r)
	}

	settings := &service.BetaPolicySettings{Rules: rules}
	if err := h.settingService.SetBetaPolicySettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Re-fetch to return updated settings
	updated, err := h.settingService.GetBetaPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outRules := make([]dto.BetaPolicyRule, len(updated.Rules))
	for i, r := range updated.Rules {
		outRules[i] = dto.BetaPolicyRule(r)
	}
	response.Success(c, dto.BetaPolicySettings{Rules: outRules})
}

// GetOpenAIFastPolicySettings 获取 OpenAI fast/flex 策略配置
// GET /api/v1/admin/settings/openai-fast-policy
func (h *SettingHandler) GetOpenAIFastPolicySettings(c *gin.Context) {
	settings, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	rules := make([]dto.OpenAIFastPolicyRule, len(settings.Rules))
	for i, r := range settings.Rules {
		rules[i] = dto.OpenAIFastPolicyRule(r)
	}
	response.Success(c, dto.OpenAIFastPolicySettings{Rules: rules})
}

// UpdateOpenAIFastPolicySettingsRequest 更新 OpenAI fast/flex 策略配置请求
type UpdateOpenAIFastPolicySettingsRequest struct {
	Rules []dto.OpenAIFastPolicyRule `json:"rules"`
}

// UpdateOpenAIFastPolicySettings 更新 OpenAI fast/flex 策略配置
// PUT /api/v1/admin/settings/openai-fast-policy
func (h *SettingHandler) UpdateOpenAIFastPolicySettings(c *gin.Context) {
	var req UpdateOpenAIFastPolicySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	rules := make([]service.OpenAIFastPolicyRule, len(req.Rules))
	for i, r := range req.Rules {
		tier := strings.ToLower(strings.TrimSpace(r.ServiceTier))
		if tier == "" {
			tier = service.OpenAIFastTierAny
		}
		r.ServiceTier = tier
		rules[i] = service.OpenAIFastPolicyRule(r)
	}

	settings := &service.OpenAIFastPolicySettings{Rules: rules}
	if err := h.settingService.SetOpenAIFastPolicySettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updated, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outRules := make([]dto.OpenAIFastPolicyRule, len(updated.Rules))
	for i, r := range updated.Rules {
		outRules[i] = dto.OpenAIFastPolicyRule(r)
	}
	response.Success(c, dto.OpenAIFastPolicySettings{Rules: outRules})
}

// UpdateStreamTimeoutSettingsRequest 更新流超时配置请求
type UpdateStreamTimeoutSettingsRequest struct {
	Enabled                bool   `json:"enabled"`
	Action                 string `json:"action"`
	TempUnschedMinutes     int    `json:"temp_unsched_minutes"`
	ThresholdCount         int    `json:"threshold_count"`
	ThresholdWindowMinutes int    `json:"threshold_window_minutes"`
}

// UpdateStreamTimeoutSettings 更新流超时处理配置
// PUT /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) UpdateStreamTimeoutSettings(c *gin.Context) {
	var req UpdateStreamTimeoutSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.StreamTimeoutSettings{
		Enabled:                req.Enabled,
		Action:                 req.Action,
		TempUnschedMinutes:     req.TempUnschedMinutes,
		ThresholdCount:         req.ThresholdCount,
		ThresholdWindowMinutes: req.ThresholdWindowMinutes,
	}

	if err := h.settingService.SetStreamTimeoutSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                updatedSettings.Enabled,
		Action:                 updatedSettings.Action,
		TempUnschedMinutes:     updatedSettings.TempUnschedMinutes,
		ThresholdCount:         updatedSettings.ThresholdCount,
		ThresholdWindowMinutes: updatedSettings.ThresholdWindowMinutes,
	})
}

// GetWebSearchEmulationConfig 获取 Web Search 模拟配置
// GET /api/v1/admin/settings/web-search-emulation
func (h *SettingHandler) GetWebSearchEmulationConfig(c *gin.Context) {
	cfg, err := h.settingService.GetWebSearchEmulationConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.PopulateWebSearchUsage(c.Request.Context(), cfg))
}

// UpdateWebSearchEmulationConfig 更新 Web Search 模拟配置
// PUT /api/v1/admin/settings/web-search-emulation
func (h *SettingHandler) UpdateWebSearchEmulationConfig(c *gin.Context) {
	var cfg service.WebSearchEmulationConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.settingService.SaveWebSearchEmulationConfig(c.Request.Context(), &cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Re-read (with sanitized api keys) to return current state
	updated, err := h.settingService.GetWebSearchEmulationConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.PopulateWebSearchUsage(c.Request.Context(), updated))
}

// ResetWebSearchUsage 重置指定 provider 的配额用量
// POST /api/v1/admin/settings/web-search-emulation/reset-usage
func (h *SettingHandler) ResetWebSearchUsage(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.ProviderType == "" {
		response.BadRequest(c, "provider_type is required")
		return
	}
	if err := service.ResetWebSearchUsage(c.Request.Context(), req.ProviderType); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// TestWebSearchEmulation 测试 Web Search 搜索
// POST /api/v1/admin/settings/web-search-emulation/test
func (h *SettingHandler) TestWebSearchEmulation(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		req.Query = "搜索今年世界大事件"
	}

	result, err := service.TestWebSearch(c.Request.Context(), req.Query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
