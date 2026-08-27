package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/imroc/req/v3"
	"golang.org/x/sync/singleflight"
)

var (
	ErrRegistrationDisabled   = infraerrors.Forbidden("REGISTRATION_DISABLED", "registration is currently disabled")
	ErrSettingNotFound        = infraerrors.NotFound("SETTING_NOT_FOUND", "setting not found")
	ErrDefaultSubGroupInvalid = infraerrors.BadRequest(
		"DEFAULT_SUBSCRIPTION_GROUP_INVALID",
		"default subscription group must exist and be subscription type",
	)
	ErrDefaultSubGroupDuplicate = infraerrors.BadRequest(
		"DEFAULT_SUBSCRIPTION_GROUP_DUPLICATE",
		"default subscription group cannot be duplicated",
	)
)

const (
	defaultLandingPricingProMultiplier = 1.2
	defaultLandingPricingMaxMultiplier = 4.0
	defaultLandingPricingExchangeRate  = 7.0
)

type SettingRepository interface {
	Get(ctx context.Context, key string) (*Setting, error)
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetMultiple(ctx context.Context, keys []string) (map[string]string, error)
	SetMultiple(ctx context.Context, settings map[string]string) error
	GetAll(ctx context.Context) (map[string]string, error)
	Delete(ctx context.Context, key string) error
}

// cachedVersionBounds 缓存 Claude Code 版本号上下限（进程内缓存，60s TTL）
type cachedVersionBounds struct {
	min       string // 空字符串 = 不检查
	max       string // 空字符串 = 不检查
	expiresAt int64  // unix nano
}

// versionBoundsCache 版本号上下限进程内缓存
var versionBoundsCache atomic.Value // *cachedVersionBounds

// versionBoundsSF 防止缓存过期时 thundering herd
var versionBoundsSF singleflight.Group

// versionBoundsCacheTTL 缓存有效期
const versionBoundsCacheTTL = 60 * time.Second

// versionBoundsErrorTTL DB 错误时的短缓存，快速重试
const versionBoundsErrorTTL = 5 * time.Second

// versionBoundsDBTimeout singleflight 内 DB 查询超时，独立于请求 context
const versionBoundsDBTimeout = 5 * time.Second

// cachedBackendMode Backend Mode cache (in-process, 60s TTL)
type cachedBackendMode struct {
	value     bool
	expiresAt int64 // unix nano
}

var backendModeCache atomic.Value // *cachedBackendMode
var backendModeSF singleflight.Group

const backendModeCacheTTL = 60 * time.Second
const backendModeErrorTTL = 5 * time.Second
const backendModeDBTimeout = 5 * time.Second

// cachedGatewayForwardingSettings 缓存网关转发行为设置（进程内缓存，60s TTL）
type cachedGatewayForwardingSettings struct {
	fingerprintUnification       bool
	metadataPassthrough          bool
	cchSigning                   bool
	anthropicCacheTTL1hInjection bool
	expiresAt                    int64 // unix nano
}

var gatewayForwardingCache atomic.Value // *cachedGatewayForwardingSettings
var gatewayForwardingSF singleflight.Group

const gatewayForwardingCacheTTL = 60 * time.Second
const gatewayForwardingErrorTTL = 5 * time.Second
const gatewayForwardingDBTimeout = 5 * time.Second

// DefaultSubscriptionGroupReader validates group references used by default subscriptions.
type DefaultSubscriptionGroupReader interface {
	GetByID(ctx context.Context, id int64) (*Group, error)
}

// WebSearchManagerBuilder creates a websearch.Manager from config (injected by infra layer).
// proxyURLs maps proxy ID to resolved URL for provider-level proxy support.
type WebSearchManagerBuilder func(cfg *WebSearchEmulationConfig, proxyURLs map[int64]string)

// SettingService 系统设置服务
type SettingService struct {
	settingRepo             SettingRepository
	defaultSubGroupReader   DefaultSubscriptionGroupReader
	proxyRepo               ProxyRepository // for resolving websearch provider proxy URLs
	cfg                     *config.Config
	onUpdate                func() // Callback when settings are updated (for cache invalidation)
	version                 string // Application version
	webSearchManagerBuilder WebSearchManagerBuilder
}

// NewSettingService 创建系统设置服务实例
func NewSettingService(settingRepo SettingRepository, cfg *config.Config) *SettingService {
	return &SettingService{
		settingRepo: settingRepo,
		cfg:         cfg,
	}
}

// SetDefaultSubscriptionGroupReader injects an optional group reader for default subscription validation.
func (s *SettingService) SetDefaultSubscriptionGroupReader(reader DefaultSubscriptionGroupReader) {
	s.defaultSubGroupReader = reader
}

// SetProxyRepository injects a proxy repo for resolving websearch provider proxy URLs.
func (s *SettingService) SetProxyRepository(repo ProxyRepository) {
	s.proxyRepo = repo
}

// GetAllSettings 获取所有系统设置
func (s *SettingService) GetAllSettings(ctx context.Context) (*SystemSettings, error) {
	settings, err := s.settingRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}

	return s.parseSettings(settings), nil
}

// GetFrontendURL 获取前端基础URL（数据库优先，fallback 到配置文件）
func (s *SettingService) GetFrontendURL(ctx context.Context) string {
	val, err := s.settingRepo.GetValue(ctx, SettingKeyFrontendURL)
	if err == nil && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return s.cfg.Server.FrontendURL
}

// GetPublicSettings 获取公开设置（无需登录）
func (s *SettingService) GetPublicSettings(ctx context.Context) (*PublicSettings, error) {
	keys := []string{
		SettingKeyRegistrationEnabled,
		SettingKeyEmailVerifyEnabled,
		SettingKeyRegistrationEmailSuffixWhitelist,
		SettingKeyPromoCodeEnabled,
		SettingKeyPasswordResetEnabled,
		SettingKeyInvitationCodeEnabled,
		SettingKeyTotpEnabled,
		SettingKeyTurnstileEnabled,
		SettingKeyTurnstileSiteKey,
		SettingKeySiteName,
		SettingKeySiteLogo,
		SettingKeySiteSubtitle,
		SettingKeyAPIBaseURL,
		SettingKeyContactInfo,
		SettingKeyTechSupportQRCode,
		SettingKeyAfterSalesQRCode,
		SettingKeyDocURL,
		SettingKeyChatbotURL,
		SettingKeyHomeContent,
		SettingKeyLandingReportsEnabled,
		SettingKeyLandingPricingProMultiplier,
		SettingKeyLandingPricingMaxMultiplier,
		SettingKeyLandingPricingExchangeRate,
		SettingKeyHideCcsImportButton,
		SettingKeyPurchaseSubscriptionEnabled,
		SettingKeyPurchaseSubscriptionURL,
		SettingKeyCardShopEnabled,
		SettingKeyCardShopProducts,
		SettingKeyInvoiceManagementEnabled,
		SettingKeyFeedbackManagementEnabled,
		SettingKeyGroupCacheHitRateEnabled,
		SettingKeyTableDefaultPageSize,
		SettingKeyTablePageSizeOptions,
		SettingKeyCustomMenuItems,
		SettingKeyCustomEndpoints,
		SettingKeyLinuxDoConnectEnabled,
		SettingKeyBackendModeEnabled,
		SettingPaymentEnabled,
		SettingKeyXunhuAlipayEnabled,
		SettingKeyXunhuWechatEnabled,
		SettingKeyXunhuAlipayAppID,
		SettingKeyXunhuAlipayKey,
		SettingKeyXunhuWechatAppID,
		SettingKeyXunhuWechatKey,
		SettingKeyEasyPayEnabled,
		SettingKeyEasyPayPID,
		SettingKeyEasyPayKey,
		SettingKeyTopupAlipayProvider,
		SettingKeyTopupWechatProvider,
		SettingKeyOIDCConnectEnabled,
		SettingKeyOIDCConnectProviderName,
		SettingKeyGitHubOAuthEnabled,
		SettingKeyAccountQuotaNotifyEnabled,
	}

	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get public settings: %w", err)
	}

	linuxDoEnabled := false
	if raw, ok := settings[SettingKeyLinuxDoConnectEnabled]; ok {
		linuxDoEnabled = raw == "true"
	} else {
		linuxDoEnabled = s.cfg != nil && s.cfg.LinuxDo.Enabled
	}
	oidcEnabled := false
	if raw, ok := settings[SettingKeyOIDCConnectEnabled]; ok {
		oidcEnabled = raw == "true"
	} else {
		oidcEnabled = s.cfg != nil && s.cfg.OIDC.Enabled
	}
	oidcProviderName := strings.TrimSpace(settings[SettingKeyOIDCConnectProviderName])
	if oidcProviderName == "" && s.cfg != nil {
		oidcProviderName = strings.TrimSpace(s.cfg.OIDC.ProviderName)
	}
	if oidcProviderName == "" {
		oidcProviderName = "Google"
	}
	gitHubEnabled := false
	if raw, ok := settings[SettingKeyGitHubOAuthEnabled]; ok {
		gitHubEnabled = raw == "true"
	} else {
		gitHubEnabled = s.cfg != nil && s.cfg.GitHub.Enabled
	}

	// Password reset requires email verification to be enabled
	emailVerifyEnabled := settings[SettingKeyEmailVerifyEnabled] == "true"
	passwordResetEnabled := emailVerifyEnabled && settings[SettingKeyPasswordResetEnabled] == "true"

	// 充值渠道是否可用取决于当前选用的网关：虎皮椒要求渠道启用且 appid/key 齐全；
	// EasyPay 要求网关启用且 pid/key 齐全。只暴露布尔值，不暴露任何密钥。
	xunhuAlipayUsable := settings[SettingKeyXunhuAlipayEnabled] == "true" &&
		settings[SettingKeyXunhuAlipayAppID] != "" && settings[SettingKeyXunhuAlipayKey] != ""
	xunhuWechatUsable := settings[SettingKeyXunhuWechatEnabled] == "true" &&
		settings[SettingKeyXunhuWechatAppID] != "" && settings[SettingKeyXunhuWechatKey] != ""
	easyPayUsable := settings[SettingKeyEasyPayEnabled] == "true" &&
		settings[SettingKeyEasyPayPID] != "" && settings[SettingKeyEasyPayKey] != ""
	topupAlipayEnabled := topupChannelUsable(settings[SettingKeyTopupAlipayProvider], xunhuAlipayUsable, easyPayUsable)
	topupWechatEnabled := topupChannelUsable(settings[SettingKeyTopupWechatProvider], xunhuWechatUsable, easyPayUsable)
	registrationEmailSuffixWhitelist := ParseRegistrationEmailSuffixWhitelist(
		settings[SettingKeyRegistrationEmailSuffixWhitelist],
	)
	tableDefaultPageSize, tablePageSizeOptions := parseTablePreferences(
		settings[SettingKeyTableDefaultPageSize],
		settings[SettingKeyTablePageSizeOptions],
	)

	return &PublicSettings{
		TeamEnabled:                      s.cfg == nil || s.cfg.Team.Enabled,
		TeamSelfServiceEnabled:           s.cfg == nil || s.cfg.Team.SelfServiceEnabled,
		RegistrationEnabled:              settings[SettingKeyRegistrationEnabled] == "true",
		EmailVerifyEnabled:               emailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: registrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 settings[SettingKeyPromoCodeEnabled] != "false", // 默认启用
		PasswordResetEnabled:             passwordResetEnabled,
		InvitationCodeEnabled:            settings[SettingKeyInvitationCodeEnabled] == "true",
		TotpEnabled:                      settings[SettingKeyTotpEnabled] == "true",
		TurnstileEnabled:                 settings[SettingKeyTurnstileEnabled] == "true",
		TurnstileSiteKey:                 settings[SettingKeyTurnstileSiteKey],
		SiteName:                         s.getStringOrDefault(settings, SettingKeySiteName, "Sub2API"),
		SiteLogo:                         settings[SettingKeySiteLogo],
		SiteSubtitle:                     s.getStringOrDefault(settings, SettingKeySiteSubtitle, "Subscription to API Conversion Platform"),
		APIBaseURL:                       settings[SettingKeyAPIBaseURL],
		ContactInfo:                      settings[SettingKeyContactInfo],
		TechSupportQRCode:                settings[SettingKeyTechSupportQRCode],
		AfterSalesQRCode:                 settings[SettingKeyAfterSalesQRCode],
		DocURL:                           settings[SettingKeyDocURL],
		ChatbotURL:                       strings.TrimSpace(settings[SettingKeyChatbotURL]),
		HomeContent:                      settings[SettingKeyHomeContent],
		LandingReportsEnabled:            settings[SettingKeyLandingReportsEnabled] != "false",
		LandingPricingProMultiplier:      parsePositiveFloatOrDefault(settings[SettingKeyLandingPricingProMultiplier], defaultLandingPricingProMultiplier),
		LandingPricingMaxMultiplier:      parsePositiveFloatOrDefault(settings[SettingKeyLandingPricingMaxMultiplier], defaultLandingPricingMaxMultiplier),
		LandingPricingExchangeRate:       parsePositiveFloatOrDefault(settings[SettingKeyLandingPricingExchangeRate], defaultLandingPricingExchangeRate),
		HideCcsImportButton:              settings[SettingKeyHideCcsImportButton] == "true",
		PurchaseSubscriptionEnabled:      settings[SettingKeyPurchaseSubscriptionEnabled] == "true",
		PurchaseSubscriptionURL:          strings.TrimSpace(settings[SettingKeyPurchaseSubscriptionURL]),
		CardShopEnabled:                  settings[SettingKeyCardShopEnabled] == "true",
		CardShopProducts:                 publicCardShopProducts(parseCardShopProducts(settings[SettingKeyCardShopProducts])),
		InvoiceManagementEnabled:         settings[SettingKeyInvoiceManagementEnabled] == "true",
		FeedbackManagementEnabled:        settings[SettingKeyFeedbackManagementEnabled] != "false",
		GroupCacheHitRateEnabled:         settings[SettingKeyGroupCacheHitRateEnabled] == "true",
		SoraClientEnabled:                settings[SettingKeySoraClientEnabled] == "true",
		TableDefaultPageSize:             tableDefaultPageSize,
		TablePageSizeOptions:             tablePageSizeOptions,
		CustomMenuItems:                  settings[SettingKeyCustomMenuItems],
		CustomEndpoints:                  settings[SettingKeyCustomEndpoints],
		LinuxDoOAuthEnabled:              linuxDoEnabled,
		BackendModeEnabled:               settings[SettingKeyBackendModeEnabled] == "true",
		PaymentEnabled:                   settings[SettingPaymentEnabled] == "true",
		StripeEnabled:                    settings[SettingKeyStripeEnabled] == "true",
		AlipayEnabled:                    settings[SettingKeyAlipayEnabled] == "true",
		XunhuAlipayEnabled:               settings[SettingKeyXunhuAlipayEnabled] == "true",
		XunhuWechatEnabled:               settings[SettingKeyXunhuWechatEnabled] == "true",
		TopupAlipayEnabled:               topupAlipayEnabled,
		TopupWechatEnabled:               topupWechatEnabled,
		OIDCOAuthEnabled:                 oidcEnabled,
		OIDCOAuthProviderName:            oidcProviderName,
		GitHubOAuthEnabled:               gitHubEnabled,
		AccountQuotaNotifyEnabled:        settings[SettingKeyAccountQuotaNotifyEnabled] == "true",
	}, nil
}

// SetOnUpdateCallback sets a callback function to be called when settings are updated
// This is used for cache invalidation (e.g., HTML cache in frontend server)
func (s *SettingService) SetOnUpdateCallback(callback func()) {
	s.AddOnUpdateCallback(callback)
}

// AddOnUpdateCallback appends a settings-update callback without clobbering existing ones.
func (s *SettingService) AddOnUpdateCallback(callback func()) {
	if callback == nil {
		return
	}
	if s.onUpdate == nil {
		s.onUpdate = callback
		return
	}
	prev := s.onUpdate
	s.onUpdate = func() {
		prev()
		callback()
	}
}

// SetVersion sets the application version for injection into public settings
func (s *SettingService) SetVersion(version string) {
	s.version = version
}

func (s *SettingService) GetUserMenuVisibilitySettings(ctx context.Context) (*UserMenuVisibilitySettings, error) {
	settings, err := s.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &UserMenuVisibilitySettings{
		InvoiceManagementEnabled:  settings.InvoiceManagementEnabled,
		FeedbackManagementEnabled: settings.FeedbackManagementEnabled,
		GroupCacheHitRateEnabled:  settings.GroupCacheHitRateEnabled,
		LandingReportsEnabled:     settings.LandingReportsEnabled,
	}, nil
}

func (s *SettingService) UpdateUserMenuVisibilitySettings(ctx context.Context, settings UserMenuVisibilitySettings) error {
	updates := map[string]string{
		SettingKeyInvoiceManagementEnabled:  strconv.FormatBool(settings.InvoiceManagementEnabled),
		SettingKeyFeedbackManagementEnabled: strconv.FormatBool(settings.FeedbackManagementEnabled),
		SettingKeyGroupCacheHitRateEnabled:  strconv.FormatBool(settings.GroupCacheHitRateEnabled),
		SettingKeyLandingReportsEnabled:     strconv.FormatBool(settings.LandingReportsEnabled),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}

// GetPublicSettingsForInjection returns public settings in a format suitable for HTML injection
// This implements the web.PublicSettingsProvider interface
func (s *SettingService) GetPublicSettingsForInjection(ctx context.Context) (any, error) {
	settings, err := s.GetPublicSettings(ctx)
	if err != nil {
		return nil, err
	}

	// Return a struct that matches the frontend's expected format
	return &struct {
		RegistrationEnabled              bool              `json:"registration_enabled"`
		EmailVerifyEnabled               bool              `json:"email_verify_enabled"`
		RegistrationEmailSuffixWhitelist []string          `json:"registration_email_suffix_whitelist"`
		PromoCodeEnabled                 bool              `json:"promo_code_enabled"`
		PasswordResetEnabled             bool              `json:"password_reset_enabled"`
		InvitationCodeEnabled            bool              `json:"invitation_code_enabled"`
		TotpEnabled                      bool              `json:"totp_enabled"`
		TurnstileEnabled                 bool              `json:"turnstile_enabled"`
		TurnstileSiteKey                 string            `json:"turnstile_site_key,omitempty"`
		SiteName                         string            `json:"site_name"`
		SiteLogo                         string            `json:"site_logo,omitempty"`
		SiteSubtitle                     string            `json:"site_subtitle,omitempty"`
		APIBaseURL                       string            `json:"api_base_url,omitempty"`
		ContactInfo                      string            `json:"contact_info,omitempty"`
		TechSupportQRCode                string            `json:"tech_support_qrcode,omitempty"`
		AfterSalesQRCode                 string            `json:"after_sales_qrcode,omitempty"`
		DocURL                           string            `json:"doc_url,omitempty"`
		ChatbotURL                       string            `json:"chatbot_url,omitempty"`
		HomeContent                      string            `json:"home_content,omitempty"`
		LandingReportsEnabled            bool              `json:"landing_reports_enabled"`
		LandingPricingProMultiplier      float64           `json:"landing_pricing_pro_multiplier"`
		LandingPricingMaxMultiplier      float64           `json:"landing_pricing_max_multiplier"`
		LandingPricingExchangeRate       float64           `json:"landing_pricing_exchange_rate"`
		HideCcsImportButton              bool              `json:"hide_ccs_import_button"`
		PurchaseSubscriptionEnabled      bool              `json:"purchase_subscription_enabled"`
		PurchaseSubscriptionURL          string            `json:"purchase_subscription_url,omitempty"`
		CardShopEnabled                  bool              `json:"card_shop_enabled"`
		CardShopProducts                 []CardShopProduct `json:"card_shop_products"`
		InvoiceManagementEnabled         bool              `json:"invoice_management_enabled"`
		FeedbackManagementEnabled        bool              `json:"feedback_management_enabled"`
		GroupCacheHitRateEnabled         bool              `json:"group_cache_hit_rate_enabled"`
		SoraClientEnabled                bool              `json:"sora_client_enabled"`
		TableDefaultPageSize             int               `json:"table_default_page_size"`
		TablePageSizeOptions             []int             `json:"table_page_size_options"`
		CustomMenuItems                  json.RawMessage   `json:"custom_menu_items"`
		CustomEndpoints                  json.RawMessage   `json:"custom_endpoints"`
		LinuxDoOAuthEnabled              bool              `json:"linuxdo_oauth_enabled"`
		BackendModeEnabled               bool              `json:"backend_mode_enabled"`
		PaymentEnabled                   bool              `json:"payment_enabled"`
		StripeEnabled                    bool              `json:"stripe_enabled"`
		AlipayEnabled                    bool              `json:"alipay_enabled"`
		XunhuAlipayEnabled               bool              `json:"xunhu_alipay_enabled"`
		XunhuWechatEnabled               bool              `json:"xunhu_wechat_enabled"`
		TopupAlipayEnabled               bool              `json:"topup_alipay_enabled"`
		TopupWechatEnabled               bool              `json:"topup_wechat_enabled"`
		OIDCOAuthEnabled                 bool              `json:"oidc_oauth_enabled"`
		OIDCOAuthProviderName            string            `json:"oidc_oauth_provider_name"`
		GitHubOAuthEnabled               bool              `json:"github_oauth_enabled"`
		Version                          string            `json:"version,omitempty"`
		AccountQuotaNotifyEnabled        bool              `json:"account_quota_notify_enabled"`
	}{
		RegistrationEnabled:              settings.RegistrationEnabled,
		EmailVerifyEnabled:               settings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 settings.PromoCodeEnabled,
		PasswordResetEnabled:             settings.PasswordResetEnabled,
		InvitationCodeEnabled:            settings.InvitationCodeEnabled,
		TotpEnabled:                      settings.TotpEnabled,
		TurnstileEnabled:                 settings.TurnstileEnabled,
		TurnstileSiteKey:                 settings.TurnstileSiteKey,
		SiteName:                         settings.SiteName,
		SiteLogo:                         settings.SiteLogo,
		SiteSubtitle:                     settings.SiteSubtitle,
		APIBaseURL:                       settings.APIBaseURL,
		ContactInfo:                      settings.ContactInfo,
		TechSupportQRCode:                settings.TechSupportQRCode,
		AfterSalesQRCode:                 settings.AfterSalesQRCode,
		DocURL:                           settings.DocURL,
		ChatbotURL:                       settings.ChatbotURL,
		HomeContent:                      settings.HomeContent,
		LandingReportsEnabled:            settings.LandingReportsEnabled,
		LandingPricingProMultiplier:      settings.LandingPricingProMultiplier,
		LandingPricingMaxMultiplier:      settings.LandingPricingMaxMultiplier,
		LandingPricingExchangeRate:       settings.LandingPricingExchangeRate,
		HideCcsImportButton:              settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:      settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:          settings.PurchaseSubscriptionURL,
		CardShopEnabled:                  settings.CardShopEnabled,
		CardShopProducts:                 settings.CardShopProducts,
		InvoiceManagementEnabled:         settings.InvoiceManagementEnabled,
		FeedbackManagementEnabled:        settings.FeedbackManagementEnabled,
		GroupCacheHitRateEnabled:         settings.GroupCacheHitRateEnabled,
		SoraClientEnabled:                settings.SoraClientEnabled,
		TableDefaultPageSize:             settings.TableDefaultPageSize,
		TablePageSizeOptions:             settings.TablePageSizeOptions,
		CustomMenuItems:                  filterUserVisibleMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                  safeRawJSONArray(settings.CustomEndpoints),
		LinuxDoOAuthEnabled:              settings.LinuxDoOAuthEnabled,
		BackendModeEnabled:               settings.BackendModeEnabled,
		PaymentEnabled:                   settings.PaymentEnabled,
		StripeEnabled:                    settings.StripeEnabled,
		AlipayEnabled:                    settings.AlipayEnabled,
		XunhuAlipayEnabled:               settings.XunhuAlipayEnabled,
		XunhuWechatEnabled:               settings.XunhuWechatEnabled,
		TopupAlipayEnabled:               settings.TopupAlipayEnabled,
		TopupWechatEnabled:               settings.TopupWechatEnabled,
		OIDCOAuthEnabled:                 settings.OIDCOAuthEnabled,
		OIDCOAuthProviderName:            settings.OIDCOAuthProviderName,
		GitHubOAuthEnabled:               settings.GitHubOAuthEnabled,
		Version:                          s.version,
		AccountQuotaNotifyEnabled:        settings.AccountQuotaNotifyEnabled,
	}, nil
}

// filterUserVisibleMenuItems filters out admin-only menu items from a raw JSON
// array string, returning only items with visibility != "admin".
func filterUserVisibleMenuItems(raw string) json.RawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return json.RawMessage("[]")
	}
	var items []struct {
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return json.RawMessage("[]")
	}

	// Parse full items to preserve all fields
	var fullItems []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fullItems); err != nil {
		return json.RawMessage("[]")
	}

	var filtered []json.RawMessage
	for i, item := range items {
		if item.Visibility != "admin" {
			filtered = append(filtered, fullItems[i])
		}
	}
	if len(filtered) == 0 {
		return json.RawMessage("[]")
	}
	result, err := json.Marshal(filtered)
	if err != nil {
		return json.RawMessage("[]")
	}
	return result
}

// safeRawJSONArray returns raw as json.RawMessage if it's valid JSON, otherwise "[]".
func safeRawJSONArray(raw string) json.RawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return json.RawMessage("[]")
	}
	if json.Valid([]byte(raw)) {
		return json.RawMessage(raw)
	}
	return json.RawMessage("[]")
}

// GetFrameSrcOrigins returns deduplicated http(s) origins from home_content URL,
// purchase_subscription_url, and all custom_menu_items URLs. Used by the router layer for CSP frame-src injection.
func (s *SettingService) GetFrameSrcOrigins(ctx context.Context) ([]string, error) {
	settings, err := s.GetPublicSettings(ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var origins []string

	addOrigin := func(rawURL string) {
		if origin := extractOriginFromURL(rawURL); origin != "" {
			if _, ok := seen[origin]; !ok {
				seen[origin] = struct{}{}
				origins = append(origins, origin)
			}
		}
	}

	// home content URL (when home_content is set to a URL for iframe embedding)
	addOrigin(settings.HomeContent)

	// purchase subscription URL
	if settings.PurchaseSubscriptionEnabled {
		addOrigin(settings.PurchaseSubscriptionURL)
	}

	// all custom menu items (including admin-only, since CSP must allow all iframes)
	for _, item := range parseCustomMenuItemURLs(settings.CustomMenuItems) {
		addOrigin(item)
	}

	return origins, nil
}

// extractOriginFromURL returns the scheme+host origin from rawURL.
// Only http and https schemes are accepted.
func extractOriginFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// EmbedTarget is a configured, audience-bound external consumer. Callers use
// the stable kind/id pair; browser-provided URLs are never trusted.
type EmbedTarget struct {
	Kind     string
	ID       string
	URL      string
	Audience string
}

// ResolveEmbedTarget resolves a configured embed target and enforces its
// visibility before any one-time credential is issued.
func (s *SettingService) ResolveEmbedTarget(ctx context.Context, kind, targetID, role string) (*EmbedTarget, error) {
	settings, err := s.GetPublicSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get embed settings: %w", err)
	}
	kind = strings.TrimSpace(kind)
	targetID = strings.TrimSpace(targetID)
	if targetID == "" || len(targetID) > 128 {
		return nil, ErrEmbedTargetUnavailable
	}

	var rawURL string
	switch kind {
	case EmbedTargetKindPurchase:
		if targetID != "purchase" || !settings.PurchaseSubscriptionEnabled {
			return nil, ErrEmbedTargetUnavailable
		}
		rawURL = settings.PurchaseSubscriptionURL
	case EmbedTargetKindCustomMenu:
		var items []struct {
			ID         string `json:"id"`
			URL        string `json:"url"`
			Visibility string `json:"visibility"`
		}
		if err := json.Unmarshal([]byte(settings.CustomMenuItems), &items); err != nil {
			return nil, ErrEmbedTargetUnavailable
		}
		for _, item := range items {
			if strings.TrimSpace(item.ID) != targetID {
				continue
			}
			if item.Visibility == "admin" && role != RoleAdmin {
				return nil, ErrEmbedTargetUnavailable
			}
			rawURL = item.URL
			break
		}
	default:
		return nil, ErrEmbedTargetUnavailable
	}

	allowLocalhost := s.cfg != nil && s.cfg.Server.Mode != "release"
	normalizedURL, audience, err := normalizeConfiguredEmbedURL(rawURL, allowLocalhost)
	if err != nil {
		return nil, ErrEmbedTargetUnavailable
	}
	return &EmbedTarget{Kind: kind, ID: targetID, URL: normalizedURL, Audience: audience}, nil
}

func normalizeConfiguredEmbedURL(rawURL string, allowLocalhost bool) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" || u.User != nil {
		return "", "", errors.New("invalid embed URL")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	hostname := strings.ToLower(u.Hostname())
	isLocalhost := hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
	if u.Scheme != "https" && (!allowLocalhost || !isLocalhost || u.Scheme != "http") {
		return "", "", errors.New("embed URL must use https")
	}
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if strings.Contains(hostname, ":") {
		hostname = "[" + hostname + "]"
	}
	u.Host = hostname
	if port != "" {
		u.Host += ":" + port
	}
	query := u.Query()
	for key := range query {
		switch strings.ToLower(key) {
		case "token", "access_token", "refresh_token", "api_key", "apikey", "user_id", "src_url":
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()
	return u.String(), u.Scheme + "://" + u.Host, nil
}

func normalizeEmbedAudience(raw string) (string, error) {
	_, audience, err := normalizeConfiguredEmbedURL(raw, true)
	return audience, err
}

// parseCustomMenuItemURLs extracts URLs from a raw JSON array of custom menu items.
func parseCustomMenuItemURLs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var items []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	urls := make([]string, 0, len(items))
	for _, item := range items {
		if item.URL != "" {
			urls = append(urls, item.URL)
		}
	}
	return urls
}

func parseCardShopProducts(raw string) []CardShopProduct {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return []CardShopProduct{}
	}
	var items []CardShopProduct
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []CardShopProduct{}
	}
	return normalizeCardShopProducts(items)
}

func normalizeCardShopProducts(items []CardShopProduct) []CardShopProduct {
	if len(items) == 0 {
		return []CardShopProduct{}
	}
	normalized := make([]CardShopProduct, 0, len(items))
	for index, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Label = strings.TrimSpace(item.Label)
		item.URL = strings.TrimSpace(item.URL)
		if item.SortOrder == 0 {
			item.SortOrder = index
		}
		if item.ID == "" && item.Label == "" && item.URL == "" && item.AmountCNY <= 0 {
			continue
		}
		normalized = append(normalized, item)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SortOrder == normalized[j].SortOrder {
			return normalized[i].AmountCNY < normalized[j].AmountCNY
		}
		return normalized[i].SortOrder < normalized[j].SortOrder
	})
	return normalized
}

func publicCardShopProducts(items []CardShopProduct) []CardShopProduct {
	if len(items) == 0 {
		return []CardShopProduct{}
	}
	out := make([]CardShopProduct, 0, len(items))
	for _, item := range items {
		if !item.Enabled || item.AmountCNY <= 0 || strings.TrimSpace(item.URL) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

// UpdateSettings replaces all system settings represented by settings.
func (s *SettingService) UpdateSettings(ctx context.Context, settings *SystemSettings) error {
	return s.updateSettings(ctx, settings, nil)
}

// UpdateSettingsPartial persists only the supplied setting keys. The settings
// value must contain the merged final state so validation and cache refreshes
// see a coherent configuration.
func (s *SettingService) UpdateSettingsPartial(ctx context.Context, settings *SystemSettings, keys map[string]struct{}) error {
	if keys == nil {
		keys = map[string]struct{}{}
	}
	return s.updateSettings(ctx, settings, keys)
}

func (s *SettingService) updateSettings(ctx context.Context, settings *SystemSettings, keys map[string]struct{}) error {
	if err := s.validateDefaultSubscriptionGroups(ctx, settings.DefaultSubscriptions); err != nil {
		return err
	}
	normalizedWhitelist, err := NormalizeRegistrationEmailSuffixWhitelist(settings.RegistrationEmailSuffixWhitelist)
	if err != nil {
		return infraerrors.BadRequest("INVALID_REGISTRATION_EMAIL_SUFFIX_WHITELIST", err.Error())
	}
	if normalizedWhitelist == nil {
		normalizedWhitelist = []string{}
	}
	settings.RegistrationEmailSuffixWhitelist = normalizedWhitelist

	updates := make(map[string]string)

	// 注册设置
	updates[SettingKeyRegistrationEnabled] = strconv.FormatBool(settings.RegistrationEnabled)
	updates[SettingKeyEmailVerifyEnabled] = strconv.FormatBool(settings.EmailVerifyEnabled)
	registrationEmailSuffixWhitelistJSON, err := json.Marshal(settings.RegistrationEmailSuffixWhitelist)
	if err != nil {
		return fmt.Errorf("marshal registration email suffix whitelist: %w", err)
	}
	updates[SettingKeyRegistrationEmailSuffixWhitelist] = string(registrationEmailSuffixWhitelistJSON)
	updates[SettingKeyPromoCodeEnabled] = strconv.FormatBool(settings.PromoCodeEnabled)
	updates[SettingKeyPasswordResetEnabled] = strconv.FormatBool(settings.PasswordResetEnabled)
	updates[SettingKeyFrontendURL] = settings.FrontendURL
	updates[SettingKeyInvitationCodeEnabled] = strconv.FormatBool(settings.InvitationCodeEnabled)
	updates[SettingKeyTotpEnabled] = strconv.FormatBool(settings.TotpEnabled)

	// 邮件服务设置（只有非空才更新密码）
	updates[SettingKeySMTPHost] = settings.SMTPHost
	updates[SettingKeySMTPPort] = strconv.Itoa(settings.SMTPPort)
	updates[SettingKeySMTPUsername] = settings.SMTPUsername
	if settings.SMTPPassword != "" {
		updates[SettingKeySMTPPassword] = settings.SMTPPassword
	}
	updates[SettingKeySMTPFrom] = settings.SMTPFrom
	updates[SettingKeySMTPFromName] = settings.SMTPFromName
	updates[SettingKeySMTPUseTLS] = strconv.FormatBool(settings.SMTPUseTLS)
	updates[SettingKeyFeedbackNotifyEmail] = strings.TrimSpace(settings.FeedbackNotifyEmail)

	// Cloudflare Turnstile 设置（只有非空才更新密钥）
	updates[SettingKeyTurnstileEnabled] = strconv.FormatBool(settings.TurnstileEnabled)
	updates[SettingKeyTurnstileSiteKey] = settings.TurnstileSiteKey
	if settings.TurnstileSecretKey != "" {
		updates[SettingKeyTurnstileSecretKey] = settings.TurnstileSecretKey
	}

	// LinuxDo Connect OAuth 登录
	updates[SettingKeyLinuxDoConnectEnabled] = strconv.FormatBool(settings.LinuxDoConnectEnabled)
	updates[SettingKeyLinuxDoConnectClientID] = settings.LinuxDoConnectClientID
	updates[SettingKeyLinuxDoConnectRedirectURL] = settings.LinuxDoConnectRedirectURL
	if settings.LinuxDoConnectClientSecret != "" {
		updates[SettingKeyLinuxDoConnectClientSecret] = settings.LinuxDoConnectClientSecret
	}

	// Generic OIDC OAuth 登录
	updates[SettingKeyOIDCConnectEnabled] = strconv.FormatBool(settings.OIDCConnectEnabled)
	updates[SettingKeyOIDCConnectProviderName] = settings.OIDCConnectProviderName
	updates[SettingKeyOIDCConnectClientID] = settings.OIDCConnectClientID
	updates[SettingKeyOIDCConnectIssuerURL] = settings.OIDCConnectIssuerURL
	updates[SettingKeyOIDCConnectDiscoveryURL] = settings.OIDCConnectDiscoveryURL
	updates[SettingKeyOIDCConnectAuthorizeURL] = settings.OIDCConnectAuthorizeURL
	updates[SettingKeyOIDCConnectTokenURL] = settings.OIDCConnectTokenURL
	updates[SettingKeyOIDCConnectUserInfoURL] = settings.OIDCConnectUserInfoURL
	updates[SettingKeyOIDCConnectJWKSURL] = settings.OIDCConnectJWKSURL
	updates[SettingKeyOIDCConnectScopes] = settings.OIDCConnectScopes
	updates[SettingKeyOIDCConnectRedirectURL] = settings.OIDCConnectRedirectURL
	updates[SettingKeyOIDCConnectFrontendRedirectURL] = settings.OIDCConnectFrontendRedirectURL
	updates[SettingKeyOIDCConnectTokenAuthMethod] = settings.OIDCConnectTokenAuthMethod
	updates[SettingKeyOIDCConnectUsePKCE] = strconv.FormatBool(settings.OIDCConnectUsePKCE)
	updates[SettingKeyOIDCConnectValidateIDToken] = strconv.FormatBool(settings.OIDCConnectValidateIDToken)
	updates[SettingKeyOIDCConnectAllowedSigningAlgs] = settings.OIDCConnectAllowedSigningAlgs
	updates[SettingKeyOIDCConnectClockSkewSeconds] = strconv.Itoa(settings.OIDCConnectClockSkewSeconds)
	updates[SettingKeyOIDCConnectRequireEmailVerified] = strconv.FormatBool(settings.OIDCConnectRequireEmailVerified)
	updates[SettingKeyOIDCConnectUserInfoEmailPath] = settings.OIDCConnectUserInfoEmailPath
	updates[SettingKeyOIDCConnectUserInfoIDPath] = settings.OIDCConnectUserInfoIDPath
	updates[SettingKeyOIDCConnectUserInfoUsernamePath] = settings.OIDCConnectUserInfoUsernamePath
	if settings.OIDCConnectClientSecret != "" {
		updates[SettingKeyOIDCConnectClientSecret] = settings.OIDCConnectClientSecret
	}

	// GitHub OAuth 登录
	updates[SettingKeyGitHubOAuthEnabled] = strconv.FormatBool(settings.GitHubOAuthEnabled)
	updates[SettingKeyGitHubOAuthClientID] = settings.GitHubOAuthClientID
	updates[SettingKeyGitHubOAuthRedirectURL] = settings.GitHubOAuthRedirectURL
	updates[SettingKeyGitHubOAuthFrontendRedirectURL] = settings.GitHubOAuthFrontendRedirectURL
	if settings.GitHubOAuthClientSecret != "" {
		updates[SettingKeyGitHubOAuthClientSecret] = settings.GitHubOAuthClientSecret
	}

	// OEM设置
	updates[SettingKeySiteName] = settings.SiteName
	updates[SettingKeySiteLogo] = settings.SiteLogo
	updates[SettingKeySiteSubtitle] = settings.SiteSubtitle
	updates[SettingKeyAPIBaseURL] = settings.APIBaseURL
	updates[SettingKeyContactInfo] = settings.ContactInfo
	updates[SettingKeyTechSupportQRCode] = settings.TechSupportQRCode
	updates[SettingKeyAfterSalesQRCode] = settings.AfterSalesQRCode
	updates[SettingKeyDocURL] = settings.DocURL
	updates[SettingKeyChatbotURL] = strings.TrimSpace(settings.ChatbotURL)
	updates[SettingKeyHomeContent] = settings.HomeContent
	updates[SettingKeyLandingReportsEnabled] = strconv.FormatBool(settings.LandingReportsEnabled)
	updates[SettingKeyLandingPricingProMultiplier] = strconv.FormatFloat(
		normalizePositiveFloat(settings.LandingPricingProMultiplier, defaultLandingPricingProMultiplier),
		'f', -1, 64,
	)
	updates[SettingKeyLandingPricingMaxMultiplier] = strconv.FormatFloat(
		normalizePositiveFloat(settings.LandingPricingMaxMultiplier, defaultLandingPricingMaxMultiplier),
		'f', -1, 64,
	)
	updates[SettingKeyLandingPricingExchangeRate] = strconv.FormatFloat(
		normalizePositiveFloat(settings.LandingPricingExchangeRate, defaultLandingPricingExchangeRate),
		'f', -1, 64,
	)
	updates[SettingKeyHideCcsImportButton] = strconv.FormatBool(settings.HideCcsImportButton)
	updates[SettingKeyPurchaseSubscriptionEnabled] = strconv.FormatBool(settings.PurchaseSubscriptionEnabled)
	updates[SettingKeyPurchaseSubscriptionURL] = strings.TrimSpace(settings.PurchaseSubscriptionURL)
	updates[SettingKeyCardShopEnabled] = strconv.FormatBool(settings.CardShopEnabled)
	cardShopProductsJSON, err := json.Marshal(normalizeCardShopProducts(settings.CardShopProducts))
	if err != nil {
		return fmt.Errorf("marshal card shop products: %w", err)
	}
	updates[SettingKeyCardShopProducts] = string(cardShopProductsJSON)
	updates[SettingKeyInvoiceManagementEnabled] = strconv.FormatBool(settings.InvoiceManagementEnabled)
	updates[SettingKeyFeedbackManagementEnabled] = strconv.FormatBool(settings.FeedbackManagementEnabled)
	updates[SettingKeyGroupCacheHitRateEnabled] = strconv.FormatBool(settings.GroupCacheHitRateEnabled)
	updates[SettingKeySoraClientEnabled] = strconv.FormatBool(settings.SoraClientEnabled)
	tableDefaultPageSize, tablePageSizeOptions := normalizeTablePreferences(
		settings.TableDefaultPageSize,
		settings.TablePageSizeOptions,
	)
	updates[SettingKeyTableDefaultPageSize] = strconv.Itoa(tableDefaultPageSize)
	tablePageSizeOptionsJSON, err := json.Marshal(tablePageSizeOptions)
	if err != nil {
		return fmt.Errorf("marshal table page size options: %w", err)
	}
	updates[SettingKeyTablePageSizeOptions] = string(tablePageSizeOptionsJSON)
	updates[SettingKeyCustomMenuItems] = settings.CustomMenuItems
	updates[SettingKeyCustomEndpoints] = settings.CustomEndpoints

	// 默认配置
	updates[SettingKeyDefaultConcurrency] = strconv.Itoa(settings.DefaultConcurrency)
	updates[SettingKeyDefaultBalance] = strconv.FormatFloat(settings.DefaultBalance, 'f', 8, 64)
	defaultSubsJSON, err := json.Marshal(settings.DefaultSubscriptions)
	if err != nil {
		return fmt.Errorf("marshal default subscriptions: %w", err)
	}
	updates[SettingKeyDefaultSubscriptions] = string(defaultSubsJSON)

	// Model fallback configuration
	updates[SettingKeyEnableModelFallback] = strconv.FormatBool(settings.EnableModelFallback)
	updates[SettingKeyFallbackModelAnthropic] = settings.FallbackModelAnthropic
	updates[SettingKeyFallbackModelOpenAI] = settings.FallbackModelOpenAI
	updates[SettingKeyFallbackModelGemini] = settings.FallbackModelGemini
	updates[SettingKeyFallbackModelAntigravity] = settings.FallbackModelAntigravity

	// Identity patch configuration (Claude -> Gemini)
	updates[SettingKeyEnableIdentityPatch] = strconv.FormatBool(settings.EnableIdentityPatch)
	updates[SettingKeyIdentityPatchPrompt] = settings.IdentityPatchPrompt

	// Ops monitoring (vNext)
	updates[SettingKeyOpsMonitoringEnabled] = strconv.FormatBool(settings.OpsMonitoringEnabled)
	updates[SettingKeyOpsRealtimeMonitoringEnabled] = strconv.FormatBool(settings.OpsRealtimeMonitoringEnabled)
	updates[SettingKeyOpsQueryModeDefault] = string(ParseOpsQueryMode(settings.OpsQueryModeDefault))
	if settings.OpsMetricsIntervalSeconds > 0 {
		updates[SettingKeyOpsMetricsIntervalSeconds] = strconv.Itoa(settings.OpsMetricsIntervalSeconds)
	}

	// Claude Code version check
	updates[SettingKeyMinClaudeCodeVersion] = settings.MinClaudeCodeVersion
	updates[SettingKeyMaxClaudeCodeVersion] = settings.MaxClaudeCodeVersion

	// 分组隔离
	updates[SettingKeyAllowUngroupedKeyScheduling] = strconv.FormatBool(settings.AllowUngroupedKeyScheduling)

	// Backend Mode
	updates[SettingKeyBackendModeEnabled] = strconv.FormatBool(settings.BackendModeEnabled)

	// Stripe 支付设置
	updates[SettingKeyStripeEnabled] = strconv.FormatBool(settings.StripeEnabled)
	if settings.StripeSecretKey != "" {
		updates[SettingKeyStripeSecretKey] = settings.StripeSecretKey
	}
	if settings.StripeWebhookSecret != "" {
		updates[SettingKeyStripeWebhookSecret] = settings.StripeWebhookSecret
	}

	// 支付宝支付设置
	updates[SettingKeyAlipayEnabled] = strconv.FormatBool(settings.AlipayEnabled)
	updates[SettingKeyAlipayAppID] = settings.AlipayAppID
	if settings.AlipayPrivateKey != "" {
		updates[SettingKeyAlipayPrivateKey] = settings.AlipayPrivateKey
	}
	if settings.AlipayPublicKey != "" {
		updates[SettingKeyAlipayPublicKey] = settings.AlipayPublicKey
	}
	updates[SettingKeyAlipayNotifyURL] = settings.AlipayNotifyURL

	// 虎皮椒聚合支付设置
	updates[SettingKeyXunhuAlipayEnabled] = strconv.FormatBool(settings.XunhuAlipayEnabled)
	updates[SettingKeyXunhuAlipayAppID] = settings.XunhuAlipayAppID
	if settings.XunhuAlipayKey != "" {
		updates[SettingKeyXunhuAlipayKey] = settings.XunhuAlipayKey
	}
	updates[SettingKeyXunhuWechatEnabled] = strconv.FormatBool(settings.XunhuWechatEnabled)
	updates[SettingKeyXunhuWechatAppID] = settings.XunhuWechatAppID
	if settings.XunhuWechatKey != "" {
		updates[SettingKeyXunhuWechatKey] = settings.XunhuWechatKey
	}
	updates[SettingKeyXunhuNotifyURL] = settings.XunhuNotifyURL

	// EasyPay 聚合支付设置（密钥为空时保留已存值）
	updates[SettingKeyEasyPayEnabled] = strconv.FormatBool(settings.EasyPayEnabled)
	updates[SettingKeyEasyPayPID] = strings.TrimSpace(settings.EasyPayPID)
	if settings.EasyPayKey != "" {
		updates[SettingKeyEasyPayKey] = settings.EasyPayKey
	}
	updates[SettingKeyEasyPayAPIBase] = strings.TrimSpace(settings.EasyPayAPIBase)

	// 充值网关选择
	topupAlipayProvider, err := normalizeTopupProvider(settings.TopupAlipayProvider)
	if err != nil {
		return err
	}
	updates[SettingKeyTopupAlipayProvider] = topupAlipayProvider
	topupWechatProvider, err := normalizeTopupProvider(settings.TopupWechatProvider)
	if err != nil {
		return err
	}
	updates[SettingKeyTopupWechatProvider] = topupWechatProvider

	// Gateway forwarding behavior
	updates[SettingKeyEnableFingerprintUnification] = strconv.FormatBool(settings.EnableFingerprintUnification)
	updates[SettingKeyEnableMetadataPassthrough] = strconv.FormatBool(settings.EnableMetadataPassthrough)
	updates[SettingKeyEnableCCHSigning] = strconv.FormatBool(settings.EnableCCHSigning)
	updates[SettingKeyEnableAnthropicCacheTTL1hInjection] = strconv.FormatBool(settings.EnableAnthropicCacheTTL1hInjection)

	// Balance alert
	updates[SettingKeyBalanceAlertEnabled] = strconv.FormatBool(settings.BalanceAlertEnabled)
	updates[SettingKeyBalanceAlertDefaultThreshold] = strconv.FormatFloat(settings.BalanceAlertDefaultThreshold, 'f', 8, 64)
	updates[SettingKeyAccountQuotaNotifyEnabled] = strconv.FormatBool(settings.AccountQuotaNotifyEnabled)
	updates[SettingKeyAccountQuotaNotifyEmails] = MarshalNotifyEmails(settings.AccountQuotaNotifyEmails)

	if keys != nil {
		for key := range updates {
			if _, ok := keys[key]; !ok {
				delete(updates, key)
			}
		}
	}
	if len(updates) == 0 {
		return nil
	}

	err = s.settingRepo.SetMultiple(ctx, updates)
	if err == nil {
		// 先使 inflight singleflight 失效，再刷新缓存，缩小旧值覆盖新值的竞态窗口
		_, minVersionUpdated := updates[SettingKeyMinClaudeCodeVersion]
		_, maxVersionUpdated := updates[SettingKeyMaxClaudeCodeVersion]
		if minVersionUpdated || maxVersionUpdated {
			versionBoundsSF.Forget("version_bounds")
			versionBoundsCache.Store(&cachedVersionBounds{
				min:       settings.MinClaudeCodeVersion,
				max:       settings.MaxClaudeCodeVersion,
				expiresAt: time.Now().Add(versionBoundsCacheTTL).UnixNano(),
			})
		}
		if _, backendModeUpdated := updates[SettingKeyBackendModeEnabled]; backendModeUpdated {
			backendModeSF.Forget("backend_mode")
			backendModeCache.Store(&cachedBackendMode{
				value:     settings.BackendModeEnabled,
				expiresAt: time.Now().Add(backendModeCacheTTL).UnixNano(),
			})
		}
		_, fingerprintUpdated := updates[SettingKeyEnableFingerprintUnification]
		_, metadataUpdated := updates[SettingKeyEnableMetadataPassthrough]
		_, cchUpdated := updates[SettingKeyEnableCCHSigning]
		_, cacheTTLUpdated := updates[SettingKeyEnableAnthropicCacheTTL1hInjection]
		if fingerprintUpdated || metadataUpdated || cchUpdated || cacheTTLUpdated {
			gatewayForwardingSF.Forget("gateway_forwarding")
			gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
				fingerprintUnification:       settings.EnableFingerprintUnification,
				metadataPassthrough:          settings.EnableMetadataPassthrough,
				cchSigning:                   settings.EnableCCHSigning,
				anthropicCacheTTL1hInjection: settings.EnableAnthropicCacheTTL1hInjection,
				expiresAt:                    time.Now().Add(gatewayForwardingCacheTTL).UnixNano(),
			})
		}
		if s.onUpdate != nil {
			s.onUpdate() // Invalidate cache after settings update
		}
	}
	return err
}

func (s *SettingService) validateDefaultSubscriptionGroups(ctx context.Context, items []DefaultSubscriptionSetting) error {
	if len(items) == 0 {
		return nil
	}

	checked := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.GroupID <= 0 {
			continue
		}
		if _, ok := checked[item.GroupID]; ok {
			return ErrDefaultSubGroupDuplicate.WithMetadata(map[string]string{
				"group_id": strconv.FormatInt(item.GroupID, 10),
			})
		}
		checked[item.GroupID] = struct{}{}
		if s.defaultSubGroupReader == nil {
			continue
		}

		group, err := s.defaultSubGroupReader.GetByID(ctx, item.GroupID)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				return ErrDefaultSubGroupInvalid.WithMetadata(map[string]string{
					"group_id": strconv.FormatInt(item.GroupID, 10),
				})
			}
			return fmt.Errorf("get default subscription group %d: %w", item.GroupID, err)
		}
		if !group.IsSubscriptionType() {
			return ErrDefaultSubGroupInvalid.WithMetadata(map[string]string{
				"group_id": strconv.FormatInt(item.GroupID, 10),
			})
		}
	}

	return nil
}

// IsRegistrationEnabled 检查是否开放注册
func (s *SettingService) IsRegistrationEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationEnabled)
	if err != nil {
		// 安全默认：如果设置不存在或查询出错，默认关闭注册
		return false
	}
	return value == "true"
}

// IsBackendModeEnabled checks if backend mode is enabled
// Uses in-process atomic.Value cache with 60s TTL, zero-lock hot path
func (s *SettingService) IsBackendModeEnabled(ctx context.Context) bool {
	if cached, ok := backendModeCache.Load().(*cachedBackendMode); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.value
		}
	}
	result, _, _ := backendModeSF.Do("backend_mode", func() (any, error) {
		if cached, ok := backendModeCache.Load().(*cachedBackendMode); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.value, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), backendModeDBTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyBackendModeEnabled)
		if err != nil {
			if errors.Is(err, ErrSettingNotFound) {
				// Setting not yet created (fresh install) - default to disabled with full TTL
				backendModeCache.Store(&cachedBackendMode{
					value:     false,
					expiresAt: time.Now().Add(backendModeCacheTTL).UnixNano(),
				})
				return false, nil
			}
			slog.Warn("failed to get backend_mode_enabled setting", "error", err)
			backendModeCache.Store(&cachedBackendMode{
				value:     false,
				expiresAt: time.Now().Add(backendModeErrorTTL).UnixNano(),
			})
			return false, nil
		}
		enabled := value == "true"
		backendModeCache.Store(&cachedBackendMode{
			value:     enabled,
			expiresAt: time.Now().Add(backendModeCacheTTL).UnixNano(),
		})
		return enabled, nil
	})
	if val, ok := result.(bool); ok {
		return val
	}
	return false
}

type gatewayForwardingSettingsResult struct {
	fp, mp, cch, cacheTTL1h bool
}

func (s *SettingService) getGatewayForwardingSettingsCached(ctx context.Context) gatewayForwardingSettingsResult {
	if cached, ok := gatewayForwardingCache.Load().(*cachedGatewayForwardingSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return gatewayForwardingSettingsResult{
				fp:         cached.fingerprintUnification,
				mp:         cached.metadataPassthrough,
				cch:        cached.cchSigning,
				cacheTTL1h: cached.anthropicCacheTTL1hInjection,
			}
		}
	}
	val, _, _ := gatewayForwardingSF.Do("gateway_forwarding", func() (any, error) {
		if cached, ok := gatewayForwardingCache.Load().(*cachedGatewayForwardingSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return gatewayForwardingSettingsResult{
					fp:         cached.fingerprintUnification,
					mp:         cached.metadataPassthrough,
					cch:        cached.cchSigning,
					cacheTTL1h: cached.anthropicCacheTTL1hInjection,
				}, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayForwardingDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeyEnableFingerprintUnification,
			SettingKeyEnableMetadataPassthrough,
			SettingKeyEnableCCHSigning,
			SettingKeyEnableAnthropicCacheTTL1hInjection,
		})
		if err != nil {
			slog.Warn("failed to get gateway forwarding settings", "error", err)
			gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
				fingerprintUnification:       true,
				metadataPassthrough:          false,
				cchSigning:                   false,
				anthropicCacheTTL1hInjection: false,
				expiresAt:                    time.Now().Add(gatewayForwardingErrorTTL).UnixNano(),
			})
			return gatewayForwardingSettingsResult{fp: true}, nil
		}
		fp := true
		if v, ok := values[SettingKeyEnableFingerprintUnification]; ok && v != "" {
			fp = v == "true"
		}
		mp := values[SettingKeyEnableMetadataPassthrough] == "true"
		cch := values[SettingKeyEnableCCHSigning] == "true"
		cacheTTL1h := values[SettingKeyEnableAnthropicCacheTTL1hInjection] == "true"
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
			fingerprintUnification:       fp,
			metadataPassthrough:          mp,
			cchSigning:                   cch,
			anthropicCacheTTL1hInjection: cacheTTL1h,
			expiresAt:                    time.Now().Add(gatewayForwardingCacheTTL).UnixNano(),
		})
		return gatewayForwardingSettingsResult{fp: fp, mp: mp, cch: cch, cacheTTL1h: cacheTTL1h}, nil
	})
	if r, ok := val.(gatewayForwardingSettingsResult); ok {
		return r
	}
	return gatewayForwardingSettingsResult{fp: true}
}

// GetGatewayForwardingSettings returns cached gateway forwarding settings.
// Uses in-process atomic.Value cache with 60s TTL, zero-lock hot path.
// Returns (fingerprintUnification, metadataPassthrough, cchSigning).
func (s *SettingService) GetGatewayForwardingSettings(ctx context.Context) (fingerprintUnification, metadataPassthrough, cchSigning bool) {
	result := s.getGatewayForwardingSettingsCached(ctx)
	return result.fp, result.mp, result.cch
}

// IsAnthropicCacheTTL1hInjectionEnabled 检查是否对 Anthropic OAuth/SetupToken 请求体注入 1h cache_control ttl。
func (s *SettingService) IsAnthropicCacheTTL1hInjectionEnabled(ctx context.Context) bool {
	return s.getGatewayForwardingSettingsCached(ctx).cacheTTL1h
}

// IsEmailVerifyEnabled 检查是否开启邮件验证
func (s *SettingService) IsEmailVerifyEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyEmailVerifyEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// GetRegistrationEmailSuffixWhitelist returns normalized registration email suffix whitelist.
func (s *SettingService) GetRegistrationEmailSuffixWhitelist(ctx context.Context) []string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationEmailSuffixWhitelist)
	if err != nil {
		return []string{}
	}
	return ParseRegistrationEmailSuffixWhitelist(value)
}

// IsPromoCodeEnabled 检查是否启用优惠码功能
func (s *SettingService) IsPromoCodeEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyPromoCodeEnabled)
	if err != nil {
		return true // 默认启用
	}
	return value != "false"
}

// IsInvitationCodeEnabled 检查是否启用邀请码注册功能
func (s *SettingService) IsInvitationCodeEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyInvitationCodeEnabled)
	if err != nil {
		return false // 默认关闭
	}
	return value == "true"
}

// IsPasswordResetEnabled 检查是否启用密码重置功能
// 要求：必须同时开启邮件验证
func (s *SettingService) IsPasswordResetEnabled(ctx context.Context) bool {
	// Password reset requires email verification to be enabled
	if !s.IsEmailVerifyEnabled(ctx) {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyPasswordResetEnabled)
	if err != nil {
		return false // 默认关闭
	}
	return value == "true"
}

// IsTotpEnabled 检查是否启用 TOTP 双因素认证功能
func (s *SettingService) IsTotpEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyTotpEnabled)
	if err != nil {
		return false // 默认关闭
	}
	return value == "true"
}

// IsTotpEncryptionKeyConfigured 检查 TOTP 加密密钥是否已手动配置
// 只有手动配置了密钥才允许在管理后台启用 TOTP 功能
func (s *SettingService) IsTotpEncryptionKeyConfigured() bool {
	return s.cfg.Totp.EncryptionKeyConfigured
}

// GetSiteName 获取网站名称
func (s *SettingService) GetSiteName(ctx context.Context) string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeySiteName)
	if err != nil || value == "" {
		return "Sub2API"
	}
	return value
}

// GetFeedbackNotifyEmail 获取反馈通知邮箱。
func (s *SettingService) GetFeedbackNotifyEmail(ctx context.Context) string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyFeedbackNotifyEmail)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// GetDefaultConcurrency 获取默认并发量
func (s *SettingService) GetDefaultConcurrency(ctx context.Context) int {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyDefaultConcurrency)
	if err != nil {
		return s.cfg.Default.UserConcurrency
	}
	if v, err := strconv.Atoi(value); err == nil && v > 0 {
		return v
	}
	return s.cfg.Default.UserConcurrency
}

// GetDefaultBalance 获取默认余额
func (s *SettingService) GetDefaultBalance(ctx context.Context) float64 {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyDefaultBalance)
	if err != nil {
		return s.cfg.Default.UserBalance
	}
	if v, err := strconv.ParseFloat(value, 64); err == nil && v >= 0 {
		return v
	}
	return s.cfg.Default.UserBalance
}

// GetDefaultSubscriptions 获取新用户默认订阅配置列表。
func (s *SettingService) GetDefaultSubscriptions(ctx context.Context) []DefaultSubscriptionSetting {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyDefaultSubscriptions)
	if err != nil {
		return nil
	}
	return parseDefaultSubscriptions(value)
}

// InitializeDefaultSettings 初始化默认设置
func (s *SettingService) InitializeDefaultSettings(ctx context.Context) error {
	// 检查是否已有设置
	_, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationEnabled)
	if err == nil {
		// 已有设置，不需要初始化
		return nil
	}
	if !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("check existing settings: %w", err)
	}

	// 初始化默认设置
	defaults := map[string]string{
		SettingKeyRegistrationEnabled:              "true",
		SettingKeyEmailVerifyEnabled:               "false",
		SettingKeyRegistrationEmailSuffixWhitelist: "[]",
		SettingKeyPromoCodeEnabled:                 "true", // 默认启用优惠码功能
		SettingKeySiteName:                         "Sub2API",
		SettingKeySiteLogo:                         "",
		SettingKeyPurchaseSubscriptionEnabled:      "false",
		SettingKeyPurchaseSubscriptionURL:          "",
		SettingKeyCardShopEnabled:                  "false",
		SettingKeyCardShopProducts:                 "[]",
		SettingKeyInvoiceManagementEnabled:         "false",
		SettingKeyFeedbackManagementEnabled:        "true",
		SettingKeyGroupCacheHitRateEnabled:         "false",
		SettingKeyMonthlyCardPublicStatusEnabled:   "false",
		SettingKeyMonthlyCardPublicStatusChannels:  `["codex","claude","grok"]`,
		SettingKeyReliabilityObservationEnabled:    "false",
		SettingKeyServiceStatusEnabled:             "false",
		SettingKeyServiceStatusPublicEnabled:       "false",
		SettingKeyChatbotURL:                       "",
		SettingKeyLandingReportsEnabled:            "true",
		SettingKeyLandingPricingProMultiplier:      strconv.FormatFloat(defaultLandingPricingProMultiplier, 'f', -1, 64),
		SettingKeyLandingPricingMaxMultiplier:      strconv.FormatFloat(defaultLandingPricingMaxMultiplier, 'f', -1, 64),
		SettingKeyLandingPricingExchangeRate:       strconv.FormatFloat(defaultLandingPricingExchangeRate, 'f', -1, 64),
		SettingKeyTableDefaultPageSize:             "20",
		SettingKeyTablePageSizeOptions:             "[10,20,50,100]",
		SettingKeyCustomMenuItems:                  "[]",
		SettingKeyCustomEndpoints:                  "[]",
		SettingKeyOIDCConnectEnabled:               "false",
		SettingKeyOIDCConnectProviderName:          "Google",
		SettingKeyOIDCConnectIssuerURL:             "https://accounts.google.com",
		SettingKeyOIDCConnectDiscoveryURL:          "https://accounts.google.com/.well-known/openid-configuration",
		SettingKeyOIDCConnectAuthorizeURL:          "https://accounts.google.com/o/oauth2/v2/auth",
		SettingKeyOIDCConnectTokenURL:              "https://oauth2.googleapis.com/token",
		SettingKeyOIDCConnectUserInfoURL:           "https://openidconnect.googleapis.com/v1/userinfo",
		SettingKeyOIDCConnectJWKSURL:               "https://www.googleapis.com/oauth2/v3/certs",
		SettingKeyOIDCConnectScopes:                "openid profile email",
		SettingKeyOIDCConnectFrontendRedirectURL:   "/auth/google/callback",
		SettingKeyOIDCConnectTokenAuthMethod:       "client_secret_post",
		SettingKeyOIDCConnectUsePKCE:               "false",
		SettingKeyOIDCConnectValidateIDToken:       "false",
		SettingKeyOIDCConnectAllowedSigningAlgs:    "RS256",
		SettingKeyOIDCConnectClockSkewSeconds:      "120",
		SettingKeyOIDCConnectRequireEmailVerified:  "true",
		SettingKeyOIDCConnectUserInfoEmailPath:     "email",
		SettingKeyOIDCConnectUserInfoIDPath:        "sub",
		SettingKeyOIDCConnectUserInfoUsernamePath:  "name",
		SettingKeyGitHubOAuthEnabled:               "false",
		SettingKeyGitHubOAuthFrontendRedirectURL:   "/auth/github/callback",
		SettingKeyDefaultConcurrency:               strconv.Itoa(s.cfg.Default.UserConcurrency),
		SettingKeyDefaultBalance:                   strconv.FormatFloat(s.cfg.Default.UserBalance, 'f', 8, 64),
		SettingKeyDefaultSubscriptions:             "[]",
		SettingKeySMTPPort:                         "587",
		SettingKeySMTPUseTLS:                       "false",
		SettingKeyFeedbackNotifyEmail:              "",
		// Model fallback defaults
		SettingKeyEnableModelFallback:      "false",
		SettingKeyFallbackModelAnthropic:   "claude-3-5-sonnet-20241022",
		SettingKeyFallbackModelOpenAI:      "gpt-4o",
		SettingKeyFallbackModelGemini:      "gemini-2.5-pro",
		SettingKeyFallbackModelAntigravity: "gemini-2.5-pro",
		// Identity patch defaults
		SettingKeyEnableIdentityPatch: "true",
		SettingKeyIdentityPatchPrompt: "",

		// Ops monitoring defaults (vNext)
		SettingKeyOpsMonitoringEnabled:         "true",
		SettingKeyOpsRealtimeMonitoringEnabled: "true",
		SettingKeyOpsQueryModeDefault:          "auto",
		SettingKeyOpsMetricsIntervalSeconds:    "60",

		// Claude Code version check (default: empty = disabled)
		SettingKeyMinClaudeCodeVersion: "",
		SettingKeyMaxClaudeCodeVersion: "",

		// 分组隔离（默认不允许未分组 Key 调度）
		SettingKeyAllowUngroupedKeyScheduling:        "false",
		SettingKeyEnableAnthropicCacheTTL1hInjection: "false",
	}

	return s.settingRepo.SetMultiple(ctx, defaults)
}

// parseSettings 解析设置到结构体
func (s *SettingService) parseSettings(settings map[string]string) *SystemSettings {
	emailVerifyEnabled := settings[SettingKeyEmailVerifyEnabled] == "true"
	result := &SystemSettings{
		RegistrationEnabled:              settings[SettingKeyRegistrationEnabled] == "true",
		EmailVerifyEnabled:               emailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: ParseRegistrationEmailSuffixWhitelist(settings[SettingKeyRegistrationEmailSuffixWhitelist]),
		PromoCodeEnabled:                 settings[SettingKeyPromoCodeEnabled] != "false", // 默认启用
		PasswordResetEnabled:             emailVerifyEnabled && settings[SettingKeyPasswordResetEnabled] == "true",
		FrontendURL:                      settings[SettingKeyFrontendURL],
		InvitationCodeEnabled:            settings[SettingKeyInvitationCodeEnabled] == "true",
		TotpEnabled:                      settings[SettingKeyTotpEnabled] == "true",
		SMTPHost:                         settings[SettingKeySMTPHost],
		SMTPUsername:                     settings[SettingKeySMTPUsername],
		SMTPFrom:                         settings[SettingKeySMTPFrom],
		SMTPFromName:                     settings[SettingKeySMTPFromName],
		SMTPUseTLS:                       settings[SettingKeySMTPUseTLS] == "true",
		SMTPPasswordConfigured:           settings[SettingKeySMTPPassword] != "",
		FeedbackNotifyEmail:              strings.TrimSpace(settings[SettingKeyFeedbackNotifyEmail]),
		TurnstileEnabled:                 settings[SettingKeyTurnstileEnabled] == "true",
		TurnstileSiteKey:                 settings[SettingKeyTurnstileSiteKey],
		TurnstileSecretKeyConfigured:     settings[SettingKeyTurnstileSecretKey] != "",
		SiteName:                         s.getStringOrDefault(settings, SettingKeySiteName, "Sub2API"),
		SiteLogo:                         settings[SettingKeySiteLogo],
		SiteSubtitle:                     s.getStringOrDefault(settings, SettingKeySiteSubtitle, "Subscription to API Conversion Platform"),
		APIBaseURL:                       settings[SettingKeyAPIBaseURL],
		ContactInfo:                      settings[SettingKeyContactInfo],
		TechSupportQRCode:                settings[SettingKeyTechSupportQRCode],
		AfterSalesQRCode:                 settings[SettingKeyAfterSalesQRCode],
		DocURL:                           settings[SettingKeyDocURL],
		ChatbotURL:                       strings.TrimSpace(settings[SettingKeyChatbotURL]),
		HomeContent:                      settings[SettingKeyHomeContent],
		LandingReportsEnabled:            settings[SettingKeyLandingReportsEnabled] != "false",
		LandingPricingProMultiplier:      parsePositiveFloatOrDefault(settings[SettingKeyLandingPricingProMultiplier], defaultLandingPricingProMultiplier),
		LandingPricingMaxMultiplier:      parsePositiveFloatOrDefault(settings[SettingKeyLandingPricingMaxMultiplier], defaultLandingPricingMaxMultiplier),
		LandingPricingExchangeRate:       parsePositiveFloatOrDefault(settings[SettingKeyLandingPricingExchangeRate], defaultLandingPricingExchangeRate),
		HideCcsImportButton:              settings[SettingKeyHideCcsImportButton] == "true",
		PurchaseSubscriptionEnabled:      settings[SettingKeyPurchaseSubscriptionEnabled] == "true",
		PurchaseSubscriptionURL:          strings.TrimSpace(settings[SettingKeyPurchaseSubscriptionURL]),
		CardShopEnabled:                  settings[SettingKeyCardShopEnabled] == "true",
		CardShopProducts:                 parseCardShopProducts(settings[SettingKeyCardShopProducts]),
		InvoiceManagementEnabled:         settings[SettingKeyInvoiceManagementEnabled] == "true",
		FeedbackManagementEnabled:        settings[SettingKeyFeedbackManagementEnabled] != "false",
		GroupCacheHitRateEnabled:         settings[SettingKeyGroupCacheHitRateEnabled] == "true",
		SoraClientEnabled:                settings[SettingKeySoraClientEnabled] == "true",
		CustomMenuItems:                  settings[SettingKeyCustomMenuItems],
		CustomEndpoints:                  settings[SettingKeyCustomEndpoints],
		BackendModeEnabled:               settings[SettingKeyBackendModeEnabled] == "true",
	}
	result.TableDefaultPageSize, result.TablePageSizeOptions = parseTablePreferences(
		settings[SettingKeyTableDefaultPageSize],
		settings[SettingKeyTablePageSizeOptions],
	)

	// 解析整数类型
	if port, err := strconv.Atoi(settings[SettingKeySMTPPort]); err == nil {
		result.SMTPPort = port
	} else {
		result.SMTPPort = 587
	}

	if concurrency, err := strconv.Atoi(settings[SettingKeyDefaultConcurrency]); err == nil {
		result.DefaultConcurrency = concurrency
	} else {
		result.DefaultConcurrency = s.cfg.Default.UserConcurrency
	}

	// 解析浮点数类型
	if balance, err := strconv.ParseFloat(settings[SettingKeyDefaultBalance], 64); err == nil {
		result.DefaultBalance = balance
	} else {
		result.DefaultBalance = s.cfg.Default.UserBalance
	}
	result.DefaultSubscriptions = parseDefaultSubscriptions(settings[SettingKeyDefaultSubscriptions])

	// 敏感信息直接返回，方便测试连接时使用
	result.SMTPPassword = settings[SettingKeySMTPPassword]
	result.TurnstileSecretKey = settings[SettingKeyTurnstileSecretKey]

	// LinuxDo Connect 设置：
	// - 兼容 config.yaml/env（避免老部署因为未迁移到数据库设置而被意外关闭）
	// - 支持在后台“系统设置”中覆盖并持久化（存储于 DB）
	linuxDoBase := config.LinuxDoConnectConfig{}
	if s.cfg != nil {
		linuxDoBase = s.cfg.LinuxDo
	}

	if raw, ok := settings[SettingKeyLinuxDoConnectEnabled]; ok {
		result.LinuxDoConnectEnabled = raw == "true"
	} else {
		result.LinuxDoConnectEnabled = linuxDoBase.Enabled
	}

	if v, ok := settings[SettingKeyLinuxDoConnectClientID]; ok && strings.TrimSpace(v) != "" {
		result.LinuxDoConnectClientID = strings.TrimSpace(v)
	} else {
		result.LinuxDoConnectClientID = linuxDoBase.ClientID
	}

	if v, ok := settings[SettingKeyLinuxDoConnectRedirectURL]; ok && strings.TrimSpace(v) != "" {
		result.LinuxDoConnectRedirectURL = strings.TrimSpace(v)
	} else {
		result.LinuxDoConnectRedirectURL = linuxDoBase.RedirectURL
	}

	result.LinuxDoConnectClientSecret = strings.TrimSpace(settings[SettingKeyLinuxDoConnectClientSecret])
	if result.LinuxDoConnectClientSecret == "" {
		result.LinuxDoConnectClientSecret = strings.TrimSpace(linuxDoBase.ClientSecret)
	}
	result.LinuxDoConnectClientSecretConfigured = result.LinuxDoConnectClientSecret != ""

	// Generic OIDC 设置：
	// - 兼容 config.yaml/env
	// - 支持后台系统设置覆盖并持久化（存储于 DB）
	oidcBase := config.OIDCConnectConfig{}
	if s.cfg != nil {
		oidcBase = s.cfg.OIDC
	}

	if raw, ok := settings[SettingKeyOIDCConnectEnabled]; ok {
		result.OIDCConnectEnabled = raw == "true"
	} else {
		result.OIDCConnectEnabled = oidcBase.Enabled
	}

	if v, ok := settings[SettingKeyOIDCConnectProviderName]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectProviderName = strings.TrimSpace(v)
	} else {
		result.OIDCConnectProviderName = strings.TrimSpace(oidcBase.ProviderName)
	}
	if result.OIDCConnectProviderName == "" {
		result.OIDCConnectProviderName = "OIDC"
	}

	if v, ok := settings[SettingKeyOIDCConnectClientID]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectClientID = strings.TrimSpace(v)
	} else {
		result.OIDCConnectClientID = strings.TrimSpace(oidcBase.ClientID)
	}
	if v, ok := settings[SettingKeyOIDCConnectIssuerURL]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectIssuerURL = strings.TrimSpace(v)
	} else {
		result.OIDCConnectIssuerURL = strings.TrimSpace(oidcBase.IssuerURL)
	}
	if v, ok := settings[SettingKeyOIDCConnectDiscoveryURL]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectDiscoveryURL = strings.TrimSpace(v)
	} else {
		result.OIDCConnectDiscoveryURL = strings.TrimSpace(oidcBase.DiscoveryURL)
	}
	if v, ok := settings[SettingKeyOIDCConnectAuthorizeURL]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectAuthorizeURL = strings.TrimSpace(v)
	} else {
		result.OIDCConnectAuthorizeURL = strings.TrimSpace(oidcBase.AuthorizeURL)
	}
	if v, ok := settings[SettingKeyOIDCConnectTokenURL]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectTokenURL = strings.TrimSpace(v)
	} else {
		result.OIDCConnectTokenURL = strings.TrimSpace(oidcBase.TokenURL)
	}
	if v, ok := settings[SettingKeyOIDCConnectUserInfoURL]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectUserInfoURL = strings.TrimSpace(v)
	} else {
		result.OIDCConnectUserInfoURL = strings.TrimSpace(oidcBase.UserInfoURL)
	}
	if v, ok := settings[SettingKeyOIDCConnectJWKSURL]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectJWKSURL = strings.TrimSpace(v)
	} else {
		result.OIDCConnectJWKSURL = strings.TrimSpace(oidcBase.JWKSURL)
	}
	if v, ok := settings[SettingKeyOIDCConnectScopes]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectScopes = strings.TrimSpace(v)
	} else {
		result.OIDCConnectScopes = strings.TrimSpace(oidcBase.Scopes)
	}
	if v, ok := settings[SettingKeyOIDCConnectRedirectURL]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectRedirectURL = strings.TrimSpace(v)
	} else {
		result.OIDCConnectRedirectURL = strings.TrimSpace(oidcBase.RedirectURL)
	}
	if v, ok := settings[SettingKeyOIDCConnectFrontendRedirectURL]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectFrontendRedirectURL = strings.TrimSpace(v)
	} else {
		result.OIDCConnectFrontendRedirectURL = strings.TrimSpace(oidcBase.FrontendRedirectURL)
	}
	if v, ok := settings[SettingKeyOIDCConnectTokenAuthMethod]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectTokenAuthMethod = strings.ToLower(strings.TrimSpace(v))
	} else {
		result.OIDCConnectTokenAuthMethod = strings.ToLower(strings.TrimSpace(oidcBase.TokenAuthMethod))
	}
	if raw, ok := settings[SettingKeyOIDCConnectUsePKCE]; ok {
		result.OIDCConnectUsePKCE = raw == "true"
	} else {
		result.OIDCConnectUsePKCE = oidcBase.UsePKCE
	}
	if raw, ok := settings[SettingKeyOIDCConnectValidateIDToken]; ok {
		result.OIDCConnectValidateIDToken = raw == "true"
	} else {
		result.OIDCConnectValidateIDToken = oidcBase.ValidateIDToken
	}
	if v, ok := settings[SettingKeyOIDCConnectAllowedSigningAlgs]; ok && strings.TrimSpace(v) != "" {
		result.OIDCConnectAllowedSigningAlgs = strings.TrimSpace(v)
	} else {
		result.OIDCConnectAllowedSigningAlgs = strings.TrimSpace(oidcBase.AllowedSigningAlgs)
	}
	clockSkewSet := false
	if raw, ok := settings[SettingKeyOIDCConnectClockSkewSeconds]; ok && strings.TrimSpace(raw) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			result.OIDCConnectClockSkewSeconds = parsed
			clockSkewSet = true
		}
	}
	if !clockSkewSet {
		result.OIDCConnectClockSkewSeconds = oidcBase.ClockSkewSeconds
	}
	if !clockSkewSet && result.OIDCConnectClockSkewSeconds == 0 {
		result.OIDCConnectClockSkewSeconds = 120
	}
	if raw, ok := settings[SettingKeyOIDCConnectRequireEmailVerified]; ok {
		result.OIDCConnectRequireEmailVerified = raw == "true"
	} else {
		result.OIDCConnectRequireEmailVerified = oidcBase.RequireEmailVerified
	}
	if v, ok := settings[SettingKeyOIDCConnectUserInfoEmailPath]; ok {
		result.OIDCConnectUserInfoEmailPath = strings.TrimSpace(v)
	} else {
		result.OIDCConnectUserInfoEmailPath = strings.TrimSpace(oidcBase.UserInfoEmailPath)
	}
	if v, ok := settings[SettingKeyOIDCConnectUserInfoIDPath]; ok {
		result.OIDCConnectUserInfoIDPath = strings.TrimSpace(v)
	} else {
		result.OIDCConnectUserInfoIDPath = strings.TrimSpace(oidcBase.UserInfoIDPath)
	}
	if v, ok := settings[SettingKeyOIDCConnectUserInfoUsernamePath]; ok {
		result.OIDCConnectUserInfoUsernamePath = strings.TrimSpace(v)
	} else {
		result.OIDCConnectUserInfoUsernamePath = strings.TrimSpace(oidcBase.UserInfoUsernamePath)
	}
	result.OIDCConnectClientSecret = strings.TrimSpace(settings[SettingKeyOIDCConnectClientSecret])
	if result.OIDCConnectClientSecret == "" {
		result.OIDCConnectClientSecret = strings.TrimSpace(oidcBase.ClientSecret)
	}
	result.OIDCConnectClientSecretConfigured = result.OIDCConnectClientSecret != ""

	gitHubBase := config.GitHubOAuthConfig{}
	if s.cfg != nil {
		gitHubBase = s.cfg.GitHub
	}
	if raw, ok := settings[SettingKeyGitHubOAuthEnabled]; ok {
		result.GitHubOAuthEnabled = raw == "true"
	} else {
		result.GitHubOAuthEnabled = gitHubBase.Enabled
	}
	if v, ok := settings[SettingKeyGitHubOAuthClientID]; ok && strings.TrimSpace(v) != "" {
		result.GitHubOAuthClientID = strings.TrimSpace(v)
	} else {
		result.GitHubOAuthClientID = strings.TrimSpace(gitHubBase.ClientID)
	}
	result.GitHubOAuthClientSecret = strings.TrimSpace(settings[SettingKeyGitHubOAuthClientSecret])
	if result.GitHubOAuthClientSecret == "" {
		result.GitHubOAuthClientSecret = strings.TrimSpace(gitHubBase.ClientSecret)
	}
	result.GitHubOAuthClientSecretConfigured = result.GitHubOAuthClientSecret != ""
	if v, ok := settings[SettingKeyGitHubOAuthRedirectURL]; ok && strings.TrimSpace(v) != "" {
		result.GitHubOAuthRedirectURL = strings.TrimSpace(v)
	} else {
		result.GitHubOAuthRedirectURL = strings.TrimSpace(gitHubBase.RedirectURL)
	}
	if v, ok := settings[SettingKeyGitHubOAuthFrontendRedirectURL]; ok && strings.TrimSpace(v) != "" {
		result.GitHubOAuthFrontendRedirectURL = strings.TrimSpace(v)
	} else {
		result.GitHubOAuthFrontendRedirectURL = strings.TrimSpace(gitHubBase.FrontendRedirectURL)
	}
	if result.GitHubOAuthFrontendRedirectURL == "" {
		result.GitHubOAuthFrontendRedirectURL = "/auth/github/callback"
	}

	// Model fallback settings
	result.EnableModelFallback = settings[SettingKeyEnableModelFallback] == "true"
	result.FallbackModelAnthropic = s.getStringOrDefault(settings, SettingKeyFallbackModelAnthropic, "claude-3-5-sonnet-20241022")
	result.FallbackModelOpenAI = s.getStringOrDefault(settings, SettingKeyFallbackModelOpenAI, "gpt-4o")
	result.FallbackModelGemini = s.getStringOrDefault(settings, SettingKeyFallbackModelGemini, "gemini-2.5-pro")
	result.FallbackModelAntigravity = s.getStringOrDefault(settings, SettingKeyFallbackModelAntigravity, "gemini-2.5-pro")

	// Identity patch settings (default: enabled, to preserve existing behavior)
	if v, ok := settings[SettingKeyEnableIdentityPatch]; ok && v != "" {
		result.EnableIdentityPatch = v == "true"
	} else {
		result.EnableIdentityPatch = true
	}
	result.IdentityPatchPrompt = settings[SettingKeyIdentityPatchPrompt]

	// Ops monitoring settings (default: enabled, fail-open)
	result.OpsMonitoringEnabled = !isFalseSettingValue(settings[SettingKeyOpsMonitoringEnabled])
	result.OpsRealtimeMonitoringEnabled = !isFalseSettingValue(settings[SettingKeyOpsRealtimeMonitoringEnabled])
	result.OpsQueryModeDefault = string(ParseOpsQueryMode(settings[SettingKeyOpsQueryModeDefault]))
	result.OpsMetricsIntervalSeconds = 60
	if raw := strings.TrimSpace(settings[SettingKeyOpsMetricsIntervalSeconds]); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			if v < 60 {
				v = 60
			}
			if v > 3600 {
				v = 3600
			}
			result.OpsMetricsIntervalSeconds = v
		}
	}

	// Claude Code version check
	result.MinClaudeCodeVersion = settings[SettingKeyMinClaudeCodeVersion]
	result.MaxClaudeCodeVersion = settings[SettingKeyMaxClaudeCodeVersion]

	// 分组隔离
	result.AllowUngroupedKeyScheduling = settings[SettingKeyAllowUngroupedKeyScheduling] == "true"

	// Gateway forwarding behavior (defaults: fingerprint=true, metadata_passthrough=false, cch_signing=false)
	if v, ok := settings[SettingKeyEnableFingerprintUnification]; ok && v != "" {
		result.EnableFingerprintUnification = v == "true"
	} else {
		result.EnableFingerprintUnification = true // default: enabled (current behavior)
	}
	result.EnableMetadataPassthrough = settings[SettingKeyEnableMetadataPassthrough] == "true"
	result.EnableCCHSigning = settings[SettingKeyEnableCCHSigning] == "true"
	result.EnableAnthropicCacheTTL1hInjection = settings[SettingKeyEnableAnthropicCacheTTL1hInjection] == "true"

	// Web search emulation: quick enabled check from the JSON config
	if raw := settings[SettingKeyWebSearchEmulationConfig]; raw != "" {
		var wsCfg WebSearchEmulationConfig
		if err := json.Unmarshal([]byte(raw), &wsCfg); err == nil {
			result.WebSearchEmulationEnabled = wsCfg.Enabled && len(wsCfg.Providers) > 0
		}
	}

	// Stripe 支付设置
	result.StripeEnabled = settings[SettingKeyStripeEnabled] == "true"
	result.StripeSecretKey = settings[SettingKeyStripeSecretKey]
	result.StripeSecretKeyConfigured = result.StripeSecretKey != ""
	result.StripeWebhookSecret = settings[SettingKeyStripeWebhookSecret]
	result.StripeWebhookSecretConfigured = result.StripeWebhookSecret != ""

	// 支付宝支付设置
	result.AlipayEnabled = settings[SettingKeyAlipayEnabled] == "true"
	result.AlipayAppID = settings[SettingKeyAlipayAppID]
	result.AlipayPrivateKey = settings[SettingKeyAlipayPrivateKey]
	result.AlipayPrivateKeyConfigured = result.AlipayPrivateKey != ""
	result.AlipayPublicKey = settings[SettingKeyAlipayPublicKey]
	result.AlipayPublicKeyConfigured = result.AlipayPublicKey != ""
	result.AlipayNotifyURL = settings[SettingKeyAlipayNotifyURL]

	// 虎皮椒聚合支付设置
	result.XunhuAlipayEnabled = settings[SettingKeyXunhuAlipayEnabled] == "true"
	result.XunhuAlipayAppID = settings[SettingKeyXunhuAlipayAppID]
	result.XunhuAlipayKey = ""
	result.XunhuAlipayKeyConfigured = settings[SettingKeyXunhuAlipayKey] != ""
	result.XunhuWechatEnabled = settings[SettingKeyXunhuWechatEnabled] == "true"
	result.XunhuWechatAppID = settings[SettingKeyXunhuWechatAppID]
	result.XunhuWechatKey = ""
	result.XunhuWechatKeyConfigured = settings[SettingKeyXunhuWechatKey] != ""
	result.XunhuNotifyURL = settings[SettingKeyXunhuNotifyURL]

	// EasyPay 聚合支付设置（密钥永不回传，仅暴露是否已配置）
	result.EasyPayEnabled = settings[SettingKeyEasyPayEnabled] == "true"
	result.EasyPayPID = settings[SettingKeyEasyPayPID]
	result.EasyPayKey = ""
	result.EasyPayKeyConfigured = settings[SettingKeyEasyPayKey] != ""
	result.EasyPayAPIBase = settings[SettingKeyEasyPayAPIBase]
	result.TopupAlipayProvider = topupProviderOrDefault(settings[SettingKeyTopupAlipayProvider])
	result.TopupWechatProvider = topupProviderOrDefault(settings[SettingKeyTopupWechatProvider])

	// Balance alert
	result.BalanceAlertEnabled = settings[SettingKeyBalanceAlertEnabled] == "true"
	result.BalanceAlertDefaultThreshold = 5
	if v, err := strconv.ParseFloat(settings[SettingKeyBalanceAlertDefaultThreshold], 64); err == nil && v >= 0 {
		result.BalanceAlertDefaultThreshold = v
	}
	if result.BalanceAlertDefaultThreshold <= 0 {
		result.BalanceAlertDefaultThreshold = 5
	}

	// Account quota notification
	result.AccountQuotaNotifyEnabled = settings[SettingKeyAccountQuotaNotifyEnabled] == "true"
	if raw := strings.TrimSpace(settings[SettingKeyAccountQuotaNotifyEmails]); raw != "" {
		result.AccountQuotaNotifyEmails = ParseNotifyEmails(raw)
	}
	if result.AccountQuotaNotifyEmails == nil {
		result.AccountQuotaNotifyEmails = []NotifyEmailEntry{}
	}

	return result
}

func isFalseSettingValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "disabled":
		return true
	default:
		return false
	}
}

func parseDefaultSubscriptions(raw string) []DefaultSubscriptionSetting {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var items []DefaultSubscriptionSetting
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}

	normalized := make([]DefaultSubscriptionSetting, 0, len(items))
	for _, item := range items {
		if item.GroupID <= 0 || item.ValidityDays <= 0 {
			continue
		}
		if item.ValidityDays > MaxValidityDays {
			item.ValidityDays = MaxValidityDays
		}
		normalized = append(normalized, item)
	}

	return normalized
}

func parseTablePreferences(defaultPageSizeRaw, optionsRaw string) (int, []int) {
	defaultPageSize := 20
	if v, err := strconv.Atoi(strings.TrimSpace(defaultPageSizeRaw)); err == nil {
		defaultPageSize = v
	}

	var options []int
	if strings.TrimSpace(optionsRaw) != "" {
		_ = json.Unmarshal([]byte(optionsRaw), &options)
	}

	return normalizeTablePreferences(defaultPageSize, options)
}

func normalizeTablePreferences(defaultPageSize int, options []int) (int, []int) {
	const minPageSize = 5
	const maxPageSize = 1000
	const fallbackPageSize = 20

	seen := make(map[int]struct{}, len(options))
	normalizedOptions := make([]int, 0, len(options))
	for _, option := range options {
		if option < minPageSize || option > maxPageSize {
			continue
		}
		if _, ok := seen[option]; ok {
			continue
		}
		seen[option] = struct{}{}
		normalizedOptions = append(normalizedOptions, option)
	}
	sort.Ints(normalizedOptions)

	if defaultPageSize < minPageSize || defaultPageSize > maxPageSize {
		defaultPageSize = fallbackPageSize
	}

	if len(normalizedOptions) == 0 {
		normalizedOptions = []int{10, 20, 50}
	}

	return defaultPageSize, normalizedOptions
}

func parsePositiveFloatOrDefault(raw string, defaultValue float64) float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil && v > 0 {
		return v
	}
	return defaultValue
}

func normalizePositiveFloat(value, defaultValue float64) float64 {
	if value > 0 {
		return value
	}
	return defaultValue
}

// getStringOrDefault 获取字符串值或默认值
func (s *SettingService) getStringOrDefault(settings map[string]string, key, defaultValue string) string {
	if value, ok := settings[key]; ok && value != "" {
		return value
	}
	return defaultValue
}

// IsTurnstileEnabled 检查是否启用 Turnstile 验证
func (s *SettingService) IsTurnstileEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyTurnstileEnabled)
	if err != nil {
		return false
	}
	return value == "true"
}

// GetTurnstileSecretKey 获取 Turnstile Secret Key
func (s *SettingService) GetTurnstileSecretKey(ctx context.Context) string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyTurnstileSecretKey)
	if err != nil {
		return ""
	}
	return value
}

// IsIdentityPatchEnabled 检查是否启用身份补丁（Claude -> Gemini systemInstruction 注入）
func (s *SettingService) IsIdentityPatchEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyEnableIdentityPatch)
	if err != nil {
		// 默认开启，保持兼容
		return true
	}
	return value == "true"
}

// GetIdentityPatchPrompt 获取自定义身份补丁提示词（为空表示使用内置默认模板）
func (s *SettingService) GetIdentityPatchPrompt(ctx context.Context) string {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyIdentityPatchPrompt)
	if err != nil {
		return ""
	}
	return value
}

// GenerateAdminAPIKey 生成新的管理员 API Key
func (s *SettingService) GenerateAdminAPIKey(ctx context.Context) (string, error) {
	// 生成 32 字节随机数 = 64 位十六进制字符
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	key := AdminAPIKeyPrefix + hex.EncodeToString(bytes)

	// 存储到 settings 表
	if err := s.settingRepo.Set(ctx, SettingKeyAdminAPIKey, key); err != nil {
		return "", fmt.Errorf("save admin api key: %w", err)
	}

	return key, nil
}

// GetAdminAPIKeyStatus 获取管理员 API Key 状态
// 返回脱敏的 key、是否存在、错误
func (s *SettingService) GetAdminAPIKeyStatus(ctx context.Context) (maskedKey string, exists bool, err error) {
	key, err := s.settingRepo.GetValue(ctx, SettingKeyAdminAPIKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	if key == "" {
		return "", false, nil
	}

	// 脱敏：显示前 10 位和后 4 位
	if len(key) > 14 {
		maskedKey = key[:10] + "..." + key[len(key)-4:]
	} else {
		maskedKey = key
	}

	return maskedKey, true, nil
}

// GetAdminAPIKey 获取完整的管理员 API Key（仅供内部验证使用）
// 如果未配置返回空字符串和 nil 错误，只有数据库错误时才返回 error
func (s *SettingService) GetAdminAPIKey(ctx context.Context) (string, error) {
	key, err := s.settingRepo.GetValue(ctx, SettingKeyAdminAPIKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return "", nil // 未配置，返回空字符串
		}
		return "", err // 数据库错误
	}
	return key, nil
}

// DeleteAdminAPIKey 删除管理员 API Key
func (s *SettingService) DeleteAdminAPIKey(ctx context.Context) error {
	return s.settingRepo.Delete(ctx, SettingKeyAdminAPIKey)
}

// GetAlipayConfig returns Alipay configuration from settings.
func (s *SettingService) GetAlipayConfig(ctx context.Context) (appID, privateKey, alipayPublicKey, notifyURL string, enabled bool, err error) {
	keys := []string{SettingKeyAlipayEnabled, SettingKeyAlipayAppID, SettingKeyAlipayPrivateKey, SettingKeyAlipayPublicKey, SettingKeyAlipayNotifyURL}
	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return "", "", "", "", false, fmt.Errorf("read alipay settings: %w", err)
	}
	enabled = settings[SettingKeyAlipayEnabled] == "true"
	return settings[SettingKeyAlipayAppID], settings[SettingKeyAlipayPrivateKey], settings[SettingKeyAlipayPublicKey], settings[SettingKeyAlipayNotifyURL], enabled, nil
}

// GetAlipayPlansRaw returns the raw JSON string for alipay subscription plans.
func (s *SettingService) GetAlipayPlansRaw(ctx context.Context) (string, error) {
	return s.settingRepo.GetValue(ctx, SettingKeyAlipayPlans)
}

// GetXunhuConfig 返回虎皮椒指定渠道的配置（payType: "alipay" 或 "wechat"）。
func (s *SettingService) GetXunhuConfig(ctx context.Context, payType string) (appID, key, notifyURL string, enabled bool, err error) {
	var enabledKey, appIDKey, keyKey string
	if payType == "wechat" {
		enabledKey = SettingKeyXunhuWechatEnabled
		appIDKey = SettingKeyXunhuWechatAppID
		keyKey = SettingKeyXunhuWechatKey
	} else {
		enabledKey = SettingKeyXunhuAlipayEnabled
		appIDKey = SettingKeyXunhuAlipayAppID
		keyKey = SettingKeyXunhuAlipayKey
	}
	keys := []string{enabledKey, appIDKey, keyKey, SettingKeyXunhuNotifyURL}
	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return "", "", "", false, fmt.Errorf("read xunhu settings: %w", err)
	}
	return settings[appIDKey], settings[keyKey], settings[SettingKeyXunhuNotifyURL], settings[enabledKey] == "true", nil
}

// GetEasyPayConfig 返回 EasyPay（彩虹易支付兼容）网关配置。
func (s *SettingService) GetEasyPayConfig(ctx context.Context) (pid, key, apiBase string, enabled bool, err error) {
	keys := []string{SettingKeyEasyPayEnabled, SettingKeyEasyPayPID, SettingKeyEasyPayKey, SettingKeyEasyPayAPIBase}
	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return "", "", "", false, fmt.Errorf("read easypay settings: %w", err)
	}
	return settings[SettingKeyEasyPayPID], settings[SettingKeyEasyPayKey], settings[SettingKeyEasyPayAPIBase], settings[SettingKeyEasyPayEnabled] == "true", nil
}

// GetTopupProvider 返回指定充值渠道（payType: "alipay" 或 "wechat"）当前选用的支付网关。
// 默认 "xunhu"；仅当设置显式为 "easypay" 时返回 "easypay"。
func (s *SettingService) GetTopupProvider(ctx context.Context, payType string) (string, error) {
	var settingKey string
	switch payType {
	case "alipay":
		settingKey = SettingKeyTopupAlipayProvider
	case "wechat":
		settingKey = SettingKeyTopupWechatProvider
	default:
		return "", infraerrors.BadRequest("INVALID_TOPUP_PAY_TYPE", "payType must be alipay or wechat")
	}
	settings, err := s.settingRepo.GetMultiple(ctx, []string{settingKey})
	if err != nil {
		return "", fmt.Errorf("read topup provider setting: %w", err)
	}
	return topupProviderOrDefault(settings[settingKey]), nil
}

// topupProviderOrDefault 返回充值网关的有效值：空或未知值一律回落到 "xunhu"。
func topupProviderOrDefault(value string) string {
	if strings.TrimSpace(value) == "easypay" {
		return "easypay"
	}
	return "xunhu"
}

// normalizeTopupProvider 校验并归一化管理员提交的充值网关取值。
func normalizeTopupProvider(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "xunhu":
		return "xunhu", nil
	case "easypay":
		return "easypay", nil
	default:
		return "", infraerrors.BadRequest("INVALID_TOPUP_PROVIDER", "topup provider must be one of: xunhu, easypay")
	}
}

// topupChannelUsable 判断当前选用网关下该充值渠道是否真实可用。
func topupChannelUsable(provider string, xunhuUsable, easyPayUsable bool) bool {
	if topupProviderOrDefault(provider) == "easypay" {
		return easyPayUsable
	}
	return xunhuUsable
}

// IsModelFallbackEnabled 检查是否启用模型兜底机制
func (s *SettingService) IsModelFallbackEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyEnableModelFallback)
	if err != nil {
		return false // Default: disabled
	}
	return value == "true"
}

// GetFallbackModel 获取指定平台的兜底模型
func (s *SettingService) GetFallbackModel(ctx context.Context, platform string) string {
	var key string
	var defaultModel string

	switch platform {
	case PlatformAnthropic:
		key = SettingKeyFallbackModelAnthropic
		defaultModel = "claude-3-5-sonnet-20241022"
	case PlatformOpenAI:
		key = SettingKeyFallbackModelOpenAI
		defaultModel = "gpt-4o"
	case PlatformGemini:
		key = SettingKeyFallbackModelGemini
		defaultModel = "gemini-2.5-pro"
	case PlatformAntigravity:
		key = SettingKeyFallbackModelAntigravity
		defaultModel = "gemini-2.5-pro"
	default:
		return ""
	}

	value, err := s.settingRepo.GetValue(ctx, key)
	if err != nil || value == "" {
		return defaultModel
	}
	return value
}

// GetLinuxDoConnectOAuthConfig 返回用于登录的"最终生效" LinuxDo Connect 配置。
//
// 优先级：
// - 若对应系统设置键存在，则覆盖 config.yaml/env 的值
// - 否则回退到 config.yaml/env 的值
func (s *SettingService) GetLinuxDoConnectOAuthConfig(ctx context.Context) (config.LinuxDoConnectConfig, error) {
	if s == nil || s.cfg == nil {
		return config.LinuxDoConnectConfig{}, infraerrors.ServiceUnavailable("CONFIG_NOT_READY", "config not loaded")
	}

	effective := s.cfg.LinuxDo

	keys := []string{
		SettingKeyLinuxDoConnectEnabled,
		SettingKeyLinuxDoConnectClientID,
		SettingKeyLinuxDoConnectClientSecret,
		SettingKeyLinuxDoConnectRedirectURL,
	}
	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return config.LinuxDoConnectConfig{}, fmt.Errorf("get linuxdo connect settings: %w", err)
	}

	if raw, ok := settings[SettingKeyLinuxDoConnectEnabled]; ok {
		effective.Enabled = raw == "true"
	}
	if v, ok := settings[SettingKeyLinuxDoConnectClientID]; ok && strings.TrimSpace(v) != "" {
		effective.ClientID = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyLinuxDoConnectClientSecret]; ok && strings.TrimSpace(v) != "" {
		effective.ClientSecret = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyLinuxDoConnectRedirectURL]; ok && strings.TrimSpace(v) != "" {
		effective.RedirectURL = strings.TrimSpace(v)
	}

	if !effective.Enabled {
		return config.LinuxDoConnectConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}

	// 基础健壮性校验（避免把用户重定向到一个必然失败或不安全的 OAuth 流程里）。
	if strings.TrimSpace(effective.ClientID) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth client id not configured")
	}
	if strings.TrimSpace(effective.AuthorizeURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth authorize url not configured")
	}
	if strings.TrimSpace(effective.TokenURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token url not configured")
	}
	if strings.TrimSpace(effective.UserInfoURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth userinfo url not configured")
	}
	if strings.TrimSpace(effective.RedirectURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth redirect url not configured")
	}
	if strings.TrimSpace(effective.FrontendRedirectURL) == "" {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth frontend redirect url not configured")
	}

	if err := config.ValidateAbsoluteHTTPURL(effective.AuthorizeURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth authorize url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.TokenURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.UserInfoURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth userinfo url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.RedirectURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth redirect url invalid")
	}
	if err := config.ValidateFrontendRedirectURL(effective.FrontendRedirectURL); err != nil {
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth frontend redirect url invalid")
	}

	method := strings.ToLower(strings.TrimSpace(effective.TokenAuthMethod))
	switch method {
	case "", "client_secret_post", "client_secret_basic":
		if strings.TrimSpace(effective.ClientSecret) == "" {
			return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth client secret not configured")
		}
	case "none":
		if !effective.UsePKCE {
			return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth pkce must be enabled when token_auth_method=none")
		}
	default:
		return config.LinuxDoConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token_auth_method invalid")
	}

	return effective, nil
}

// GetOverloadCooldownSettings 获取529过载冷却配置
func (s *SettingService) GetOverloadCooldownSettings(ctx context.Context) (*OverloadCooldownSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOverloadCooldownSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultOverloadCooldownSettings(), nil
		}
		return nil, fmt.Errorf("get overload cooldown settings: %w", err)
	}
	if value == "" {
		return DefaultOverloadCooldownSettings(), nil
	}

	var settings OverloadCooldownSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultOverloadCooldownSettings(), nil
	}

	// 修正配置值范围
	if settings.CooldownMinutes < 1 {
		settings.CooldownMinutes = 1
	}
	if settings.CooldownMinutes > 120 {
		settings.CooldownMinutes = 120
	}

	return &settings, nil
}

// SetOverloadCooldownSettings 设置529过载冷却配置
func (s *SettingService) SetOverloadCooldownSettings(ctx context.Context, settings *OverloadCooldownSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	// 禁用时修正为合法值即可，不拒绝请求
	if settings.CooldownMinutes < 1 || settings.CooldownMinutes > 120 {
		if settings.Enabled {
			return fmt.Errorf("cooldown_minutes must be between 1-120")
		}
		settings.CooldownMinutes = 10 // 禁用状态下归一化为默认值
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal overload cooldown settings: %w", err)
	}

	return s.settingRepo.Set(ctx, SettingKeyOverloadCooldownSettings, string(data))
}

// GetOIDCConnectOAuthConfig 返回用于登录的“最终生效” OIDC 配置。
//
// 优先级：
// - 若对应系统设置键存在，则覆盖 config.yaml/env 的值
// - 否则回退到 config.yaml/env 的值
func (s *SettingService) GetOIDCConnectOAuthConfig(ctx context.Context) (config.OIDCConnectConfig, error) {
	if s == nil || s.cfg == nil {
		return config.OIDCConnectConfig{}, infraerrors.ServiceUnavailable("CONFIG_NOT_READY", "config not loaded")
	}

	effective := s.cfg.OIDC

	keys := []string{
		SettingKeyOIDCConnectEnabled,
		SettingKeyOIDCConnectProviderName,
		SettingKeyOIDCConnectClientID,
		SettingKeyOIDCConnectClientSecret,
		SettingKeyOIDCConnectIssuerURL,
		SettingKeyOIDCConnectDiscoveryURL,
		SettingKeyOIDCConnectAuthorizeURL,
		SettingKeyOIDCConnectTokenURL,
		SettingKeyOIDCConnectUserInfoURL,
		SettingKeyOIDCConnectJWKSURL,
		SettingKeyOIDCConnectScopes,
		SettingKeyOIDCConnectRedirectURL,
		SettingKeyOIDCConnectFrontendRedirectURL,
		SettingKeyOIDCConnectTokenAuthMethod,
		SettingKeyOIDCConnectUsePKCE,
		SettingKeyOIDCConnectValidateIDToken,
		SettingKeyOIDCConnectAllowedSigningAlgs,
		SettingKeyOIDCConnectClockSkewSeconds,
		SettingKeyOIDCConnectRequireEmailVerified,
		SettingKeyOIDCConnectUserInfoEmailPath,
		SettingKeyOIDCConnectUserInfoIDPath,
		SettingKeyOIDCConnectUserInfoUsernamePath,
	}
	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return config.OIDCConnectConfig{}, fmt.Errorf("get oidc connect settings: %w", err)
	}

	if raw, ok := settings[SettingKeyOIDCConnectEnabled]; ok {
		effective.Enabled = raw == "true"
	}
	if v, ok := settings[SettingKeyOIDCConnectProviderName]; ok && strings.TrimSpace(v) != "" {
		effective.ProviderName = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectClientID]; ok && strings.TrimSpace(v) != "" {
		effective.ClientID = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectClientSecret]; ok && strings.TrimSpace(v) != "" {
		effective.ClientSecret = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectIssuerURL]; ok && strings.TrimSpace(v) != "" {
		effective.IssuerURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectDiscoveryURL]; ok && strings.TrimSpace(v) != "" {
		effective.DiscoveryURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectAuthorizeURL]; ok && strings.TrimSpace(v) != "" {
		effective.AuthorizeURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectTokenURL]; ok && strings.TrimSpace(v) != "" {
		effective.TokenURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectUserInfoURL]; ok && strings.TrimSpace(v) != "" {
		effective.UserInfoURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectJWKSURL]; ok && strings.TrimSpace(v) != "" {
		effective.JWKSURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectScopes]; ok && strings.TrimSpace(v) != "" {
		effective.Scopes = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectRedirectURL]; ok && strings.TrimSpace(v) != "" {
		effective.RedirectURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectFrontendRedirectURL]; ok && strings.TrimSpace(v) != "" {
		effective.FrontendRedirectURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectTokenAuthMethod]; ok && strings.TrimSpace(v) != "" {
		effective.TokenAuthMethod = strings.ToLower(strings.TrimSpace(v))
	}
	if raw, ok := settings[SettingKeyOIDCConnectUsePKCE]; ok {
		effective.UsePKCE = raw == "true"
	}
	if raw, ok := settings[SettingKeyOIDCConnectValidateIDToken]; ok {
		effective.ValidateIDToken = raw == "true"
	}
	if v, ok := settings[SettingKeyOIDCConnectAllowedSigningAlgs]; ok && strings.TrimSpace(v) != "" {
		effective.AllowedSigningAlgs = strings.TrimSpace(v)
	}
	if raw, ok := settings[SettingKeyOIDCConnectClockSkewSeconds]; ok && strings.TrimSpace(raw) != "" {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil {
			effective.ClockSkewSeconds = parsed
		}
	}
	if raw, ok := settings[SettingKeyOIDCConnectRequireEmailVerified]; ok {
		effective.RequireEmailVerified = raw == "true"
	}
	if v, ok := settings[SettingKeyOIDCConnectUserInfoEmailPath]; ok {
		effective.UserInfoEmailPath = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectUserInfoIDPath]; ok {
		effective.UserInfoIDPath = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyOIDCConnectUserInfoUsernamePath]; ok {
		effective.UserInfoUsernamePath = strings.TrimSpace(v)
	}

	if !effective.Enabled {
		return config.OIDCConnectConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}
	if strings.TrimSpace(effective.ProviderName) == "" {
		effective.ProviderName = "OIDC"
	}
	if strings.TrimSpace(effective.ClientID) == "" {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth client id not configured")
	}
	if strings.TrimSpace(effective.IssuerURL) == "" {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth issuer url not configured")
	}
	if strings.TrimSpace(effective.RedirectURL) == "" {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth redirect url not configured")
	}
	if strings.TrimSpace(effective.FrontendRedirectURL) == "" {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth frontend redirect url not configured")
	}
	if !scopesContainOpenID(effective.Scopes) {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth scopes must contain openid")
	}
	if effective.ClockSkewSeconds < 0 || effective.ClockSkewSeconds > 600 {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth clock skew must be between 0 and 600")
	}

	if err := config.ValidateAbsoluteHTTPURL(effective.IssuerURL); err != nil {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth issuer url invalid")
	}

	discoveryURL := strings.TrimSpace(effective.DiscoveryURL)
	if discoveryURL == "" {
		discoveryURL = oidcDefaultDiscoveryURL(effective.IssuerURL)
		effective.DiscoveryURL = discoveryURL
	}
	if discoveryURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(discoveryURL); err != nil {
			return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth discovery url invalid")
		}
	}

	needsDiscovery := strings.TrimSpace(effective.AuthorizeURL) == "" ||
		strings.TrimSpace(effective.TokenURL) == "" ||
		(effective.ValidateIDToken && strings.TrimSpace(effective.JWKSURL) == "")
	if needsDiscovery && discoveryURL != "" {
		metadata, resolveErr := oidcResolveProviderMetadata(ctx, discoveryURL)
		if resolveErr != nil {
			return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth discovery resolve failed").WithCause(resolveErr)
		}
		if strings.TrimSpace(effective.AuthorizeURL) == "" {
			effective.AuthorizeURL = strings.TrimSpace(metadata.AuthorizationEndpoint)
		}
		if strings.TrimSpace(effective.TokenURL) == "" {
			effective.TokenURL = strings.TrimSpace(metadata.TokenEndpoint)
		}
		if strings.TrimSpace(effective.UserInfoURL) == "" {
			effective.UserInfoURL = strings.TrimSpace(metadata.UserInfoEndpoint)
		}
		if strings.TrimSpace(effective.JWKSURL) == "" {
			effective.JWKSURL = strings.TrimSpace(metadata.JWKSURI)
		}
	}

	if strings.TrimSpace(effective.AuthorizeURL) == "" {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth authorize url not configured")
	}
	if strings.TrimSpace(effective.TokenURL) == "" {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token url not configured")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.AuthorizeURL); err != nil {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth authorize url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.TokenURL); err != nil {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token url invalid")
	}
	if v := strings.TrimSpace(effective.UserInfoURL); v != "" {
		if err := config.ValidateAbsoluteHTTPURL(v); err != nil {
			return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth userinfo url invalid")
		}
	}
	if effective.ValidateIDToken {
		if strings.TrimSpace(effective.JWKSURL) == "" {
			return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth jwks url not configured")
		}
		if strings.TrimSpace(effective.AllowedSigningAlgs) == "" {
			return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth signing algs not configured")
		}
	}
	if v := strings.TrimSpace(effective.JWKSURL); v != "" {
		if err := config.ValidateAbsoluteHTTPURL(v); err != nil {
			return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth jwks url invalid")
		}
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.RedirectURL); err != nil {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth redirect url invalid")
	}
	if err := config.ValidateFrontendRedirectURL(effective.FrontendRedirectURL); err != nil {
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth frontend redirect url invalid")
	}

	method := strings.ToLower(strings.TrimSpace(effective.TokenAuthMethod))
	switch method {
	case "", "client_secret_post", "client_secret_basic":
		if strings.TrimSpace(effective.ClientSecret) == "" {
			return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth client secret not configured")
		}
	case "none":
		if !effective.UsePKCE {
			return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth pkce must be enabled when token_auth_method=none")
		}
	default:
		return config.OIDCConnectConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth token_auth_method invalid")
	}

	return effective, nil
}

// GetGitHubOAuthConfig 返回用于登录的最终生效 GitHub OAuth 配置。
func (s *SettingService) GetGitHubOAuthConfig(ctx context.Context) (config.GitHubOAuthConfig, error) {
	if s == nil || s.cfg == nil {
		return config.GitHubOAuthConfig{}, infraerrors.ServiceUnavailable("CONFIG_NOT_READY", "config not loaded")
	}

	effective := s.cfg.GitHub
	if strings.TrimSpace(effective.AuthorizeURL) == "" {
		effective.AuthorizeURL = "https://github.com/login/oauth/authorize"
	}
	if strings.TrimSpace(effective.TokenURL) == "" {
		effective.TokenURL = "https://github.com/login/oauth/access_token"
	}
	if strings.TrimSpace(effective.UserInfoURL) == "" {
		effective.UserInfoURL = "https://api.github.com/user"
	}
	if strings.TrimSpace(effective.EmailsURL) == "" {
		effective.EmailsURL = "https://api.github.com/user/emails"
	}
	if strings.TrimSpace(effective.Scopes) == "" {
		effective.Scopes = "read:user user:email"
	}
	if strings.TrimSpace(effective.FrontendRedirectURL) == "" {
		effective.FrontendRedirectURL = "/auth/github/callback"
	}

	keys := []string{
		SettingKeyGitHubOAuthEnabled,
		SettingKeyGitHubOAuthClientID,
		SettingKeyGitHubOAuthClientSecret,
		SettingKeyGitHubOAuthRedirectURL,
		SettingKeyGitHubOAuthFrontendRedirectURL,
	}
	settings, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return config.GitHubOAuthConfig{}, fmt.Errorf("get github oauth settings: %w", err)
	}
	if raw, ok := settings[SettingKeyGitHubOAuthEnabled]; ok {
		effective.Enabled = raw == "true"
	}
	if v, ok := settings[SettingKeyGitHubOAuthClientID]; ok && strings.TrimSpace(v) != "" {
		effective.ClientID = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyGitHubOAuthClientSecret]; ok && strings.TrimSpace(v) != "" {
		effective.ClientSecret = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyGitHubOAuthRedirectURL]; ok && strings.TrimSpace(v) != "" {
		effective.RedirectURL = strings.TrimSpace(v)
	}
	if v, ok := settings[SettingKeyGitHubOAuthFrontendRedirectURL]; ok && strings.TrimSpace(v) != "" {
		effective.FrontendRedirectURL = strings.TrimSpace(v)
	}

	if !effective.Enabled {
		return config.GitHubOAuthConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}
	if strings.TrimSpace(effective.ClientID) == "" {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github client id not configured")
	}
	if strings.TrimSpace(effective.ClientSecret) == "" {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github client secret not configured")
	}
	if strings.TrimSpace(effective.RedirectURL) == "" {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github redirect url not configured")
	}
	if strings.TrimSpace(effective.FrontendRedirectURL) == "" {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github frontend redirect url not configured")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.AuthorizeURL); err != nil {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github authorize url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.TokenURL); err != nil {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github token url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.UserInfoURL); err != nil {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github userinfo url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.EmailsURL); err != nil {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github emails url invalid")
	}
	if err := config.ValidateAbsoluteHTTPURL(effective.RedirectURL); err != nil {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github redirect url invalid")
	}
	if err := config.ValidateFrontendRedirectURL(effective.FrontendRedirectURL); err != nil {
		return config.GitHubOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "github frontend redirect url invalid")
	}

	return effective, nil
}

func scopesContainOpenID(scopes string) bool {
	for _, scope := range strings.Fields(strings.ToLower(strings.TrimSpace(scopes))) {
		if scope == "openid" {
			return true
		}
	}
	return false
}

type oidcProviderMetadata struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

func oidcDefaultDiscoveryURL(issuerURL string) string {
	issuerURL = strings.TrimSpace(issuerURL)
	if issuerURL == "" {
		return ""
	}
	return strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"
}

func oidcResolveProviderMetadata(ctx context.Context, discoveryURL string) (*oidcProviderMetadata, error) {
	discoveryURL = strings.TrimSpace(discoveryURL)
	if discoveryURL == "" {
		return nil, fmt.Errorf("discovery url is empty")
	}

	resp, err := req.C().
		SetTimeout(15*time.Second).
		R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		Get(discoveryURL)
	if err != nil {
		return nil, fmt.Errorf("request discovery document: %w", err)
	}
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("discovery request failed: status=%d", resp.StatusCode)
	}

	metadata := &oidcProviderMetadata{}
	if err := json.Unmarshal(resp.Bytes(), metadata); err != nil {
		return nil, fmt.Errorf("parse discovery document: %w", err)
	}
	return metadata, nil
}

// GetStreamTimeoutSettings 获取流超时处理配置
func (s *SettingService) GetStreamTimeoutSettings(ctx context.Context) (*StreamTimeoutSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyStreamTimeoutSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultStreamTimeoutSettings(), nil
		}
		return nil, fmt.Errorf("get stream timeout settings: %w", err)
	}
	if value == "" {
		return DefaultStreamTimeoutSettings(), nil
	}

	var settings StreamTimeoutSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultStreamTimeoutSettings(), nil
	}

	// 验证并修正配置值
	if settings.TempUnschedMinutes < 1 {
		settings.TempUnschedMinutes = 1
	}
	if settings.TempUnschedMinutes > 60 {
		settings.TempUnschedMinutes = 60
	}
	if settings.ThresholdCount < 1 {
		settings.ThresholdCount = 1
	}
	if settings.ThresholdCount > 10 {
		settings.ThresholdCount = 10
	}
	if settings.ThresholdWindowMinutes < 1 {
		settings.ThresholdWindowMinutes = 1
	}
	if settings.ThresholdWindowMinutes > 60 {
		settings.ThresholdWindowMinutes = 60
	}

	// 验证 action
	switch settings.Action {
	case StreamTimeoutActionTempUnsched, StreamTimeoutActionError, StreamTimeoutActionNone:
		// valid
	default:
		settings.Action = StreamTimeoutActionTempUnsched
	}

	return &settings, nil
}

// IsUngroupedKeySchedulingAllowed 查询是否允许未分组 Key 调度
func (s *SettingService) IsUngroupedKeySchedulingAllowed(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAllowUngroupedKeyScheduling)
	if err != nil {
		return false // fail-closed: 查询失败时默认不允许
	}
	return value == "true"
}

// GetClaudeCodeVersionBounds 获取 Claude Code 版本号上下限要求
// 使用进程内 atomic.Value 缓存，60 秒 TTL，热路径零锁开销
// singleflight 防止缓存过期时 thundering herd
// 返回空字符串表示不做对应方向的版本检查
func (s *SettingService) GetClaudeCodeVersionBounds(ctx context.Context) (min, max string) {
	if cached, ok := versionBoundsCache.Load().(*cachedVersionBounds); ok {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.min, cached.max
		}
	}
	// singleflight: 同一时刻只有一个 goroutine 查询 DB，其余复用结果
	type bounds struct{ min, max string }
	result, err, _ := versionBoundsSF.Do("version_bounds", func() (any, error) {
		// 二次检查，避免排队的 goroutine 重复查询
		if cached, ok := versionBoundsCache.Load().(*cachedVersionBounds); ok {
			if time.Now().UnixNano() < cached.expiresAt {
				return bounds{cached.min, cached.max}, nil
			}
		}
		// 使用独立 context：断开请求取消链，避免客户端断连导致空值被长期缓存
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), versionBoundsDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeyMinClaudeCodeVersion,
			SettingKeyMaxClaudeCodeVersion,
		})
		if err != nil {
			// fail-open: DB 错误时不阻塞请求，但记录日志并使用短 TTL 快速重试
			slog.Warn("failed to get claude code version bounds setting, skipping version check", "error", err)
			versionBoundsCache.Store(&cachedVersionBounds{
				min:       "",
				max:       "",
				expiresAt: time.Now().Add(versionBoundsErrorTTL).UnixNano(),
			})
			return bounds{"", ""}, nil
		}
		b := bounds{
			min: values[SettingKeyMinClaudeCodeVersion],
			max: values[SettingKeyMaxClaudeCodeVersion],
		}
		versionBoundsCache.Store(&cachedVersionBounds{
			min:       b.min,
			max:       b.max,
			expiresAt: time.Now().Add(versionBoundsCacheTTL).UnixNano(),
		})
		return b, nil
	})
	if err != nil {
		return "", ""
	}
	b, ok := result.(bounds)
	if !ok {
		return "", ""
	}
	return b.min, b.max
}

// GetRectifierSettings 获取请求整流器配置
func (s *SettingService) GetRectifierSettings(ctx context.Context) (*RectifierSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRectifierSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultRectifierSettings(), nil
		}
		return nil, fmt.Errorf("get rectifier settings: %w", err)
	}
	if value == "" {
		return DefaultRectifierSettings(), nil
	}

	var settings RectifierSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultRectifierSettings(), nil
	}

	return &settings, nil
}

// SetRectifierSettings 设置请求整流器配置
func (s *SettingService) SetRectifierSettings(ctx context.Context, settings *RectifierSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal rectifier settings: %w", err)
	}

	return s.settingRepo.Set(ctx, SettingKeyRectifierSettings, string(data))
}

// IsSignatureRectifierEnabled 判断签名整流是否启用（总开关 && 签名子开关）
func (s *SettingService) IsSignatureRectifierEnabled(ctx context.Context) bool {
	settings, err := s.GetRectifierSettings(ctx)
	if err != nil {
		return true // fail-open: 查询失败时默认启用
	}
	return settings.Enabled && settings.ThinkingSignatureEnabled
}

// IsBudgetRectifierEnabled 判断 Budget 整流是否启用（总开关 && Budget 子开关）
func (s *SettingService) IsBudgetRectifierEnabled(ctx context.Context) bool {
	settings, err := s.GetRectifierSettings(ctx)
	if err != nil {
		return true // fail-open: 查询失败时默认启用
	}
	return settings.Enabled && settings.ThinkingBudgetEnabled
}

// GetBetaPolicySettings 获取 Beta 策略配置
func (s *SettingService) GetBetaPolicySettings(ctx context.Context) (*BetaPolicySettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyBetaPolicySettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultBetaPolicySettings(), nil
		}
		return nil, fmt.Errorf("get beta policy settings: %w", err)
	}
	if value == "" {
		return DefaultBetaPolicySettings(), nil
	}

	var settings BetaPolicySettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultBetaPolicySettings(), nil
	}

	return &settings, nil
}

// SetBetaPolicySettings 设置 Beta 策略配置
func (s *SettingService) SetBetaPolicySettings(ctx context.Context, settings *BetaPolicySettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	validActions := map[string]bool{
		BetaPolicyActionPass: true, BetaPolicyActionFilter: true, BetaPolicyActionBlock: true,
	}
	validScopes := map[string]bool{
		BetaPolicyScopeAll: true, BetaPolicyScopeOAuth: true, BetaPolicyScopeAPIKey: true, BetaPolicyScopeBedrock: true,
	}

	for i, rule := range settings.Rules {
		if rule.BetaToken == "" {
			return fmt.Errorf("rule[%d]: beta_token cannot be empty", i)
		}
		if !validActions[rule.Action] {
			return fmt.Errorf("rule[%d]: invalid action %q", i, rule.Action)
		}
		if !validScopes[rule.Scope] {
			return fmt.Errorf("rule[%d]: invalid scope %q", i, rule.Scope)
		}
		// Validate model_whitelist patterns
		for j, pattern := range rule.ModelWhitelist {
			trimmed := strings.TrimSpace(pattern)
			if trimmed == "" {
				return fmt.Errorf("rule[%d]: model_whitelist[%d] cannot be empty", i, j)
			}
			settings.Rules[i].ModelWhitelist[j] = trimmed
		}
		// Validate fallback_action
		if rule.FallbackAction != "" && !validActions[rule.FallbackAction] {
			return fmt.Errorf("rule[%d]: invalid fallback_action %q", i, rule.FallbackAction)
		}
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal beta policy settings: %w", err)
	}

	return s.settingRepo.Set(ctx, SettingKeyBetaPolicySettings, string(data))
}

// GetOpenAIFastPolicySettings 获取 OpenAI fast/flex 策略配置
func (s *SettingService) GetOpenAIFastPolicySettings(ctx context.Context) (*OpenAIFastPolicySettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOpenAIFastPolicySettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultOpenAIFastPolicySettings(), nil
		}
		return nil, fmt.Errorf("get openai fast policy settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultOpenAIFastPolicySettings(), nil
	}

	var settings OpenAIFastPolicySettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		slog.Warn("failed to unmarshal openai fast policy settings, falling back to defaults",
			"error", err,
			"key", SettingKeyOpenAIFastPolicySettings)
		return DefaultOpenAIFastPolicySettings(), nil
	}

	return &settings, nil
}

// SetOpenAIFastPolicySettings 设置 OpenAI fast/flex 策略配置
func (s *SettingService) SetOpenAIFastPolicySettings(ctx context.Context, settings *OpenAIFastPolicySettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	validActions := map[string]bool{
		BetaPolicyActionPass: true, BetaPolicyActionFilter: true, BetaPolicyActionBlock: true,
	}
	validScopes := map[string]bool{
		BetaPolicyScopeAll: true, BetaPolicyScopeOAuth: true, BetaPolicyScopeAPIKey: true, BetaPolicyScopeBedrock: true,
	}
	validTiers := map[string]bool{
		OpenAIFastTierAny: true, OpenAIFastTierPriority: true, OpenAIFastTierFlex: true,
	}

	for i, rule := range settings.Rules {
		tier := strings.ToLower(strings.TrimSpace(rule.ServiceTier))
		if tier == "" {
			tier = OpenAIFastTierAny
		}
		if !validTiers[tier] {
			return fmt.Errorf("rule[%d]: invalid service_tier %q", i, rule.ServiceTier)
		}
		settings.Rules[i].ServiceTier = tier
		if !validActions[rule.Action] {
			return fmt.Errorf("rule[%d]: invalid action %q", i, rule.Action)
		}
		if !validScopes[rule.Scope] {
			return fmt.Errorf("rule[%d]: invalid scope %q", i, rule.Scope)
		}
		for j, pattern := range rule.ModelWhitelist {
			trimmed := strings.TrimSpace(pattern)
			if trimmed == "" {
				return fmt.Errorf("rule[%d]: model_whitelist[%d] cannot be empty", i, j)
			}
			settings.Rules[i].ModelWhitelist[j] = trimmed
		}
		if rule.FallbackAction != "" && !validActions[rule.FallbackAction] {
			return fmt.Errorf("rule[%d]: invalid fallback_action %q", i, rule.FallbackAction)
		}
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal openai fast policy settings: %w", err)
	}

	return s.settingRepo.Set(ctx, SettingKeyOpenAIFastPolicySettings, string(data))
}

// SetStreamTimeoutSettings 设置流超时处理配置
func (s *SettingService) SetStreamTimeoutSettings(ctx context.Context, settings *StreamTimeoutSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	// 验证配置值
	if settings.TempUnschedMinutes < 1 || settings.TempUnschedMinutes > 60 {
		return fmt.Errorf("temp_unsched_minutes must be between 1-60")
	}
	if settings.ThresholdCount < 1 || settings.ThresholdCount > 10 {
		return fmt.Errorf("threshold_count must be between 1-10")
	}
	if settings.ThresholdWindowMinutes < 1 || settings.ThresholdWindowMinutes > 60 {
		return fmt.Errorf("threshold_window_minutes must be between 1-60")
	}

	switch settings.Action {
	case StreamTimeoutActionTempUnsched, StreamTimeoutActionError, StreamTimeoutActionNone:
		// valid
	default:
		return fmt.Errorf("invalid action: %s", settings.Action)
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal stream timeout settings: %w", err)
	}

	return s.settingRepo.Set(ctx, SettingKeyStreamTimeoutSettings, string(data))
}
