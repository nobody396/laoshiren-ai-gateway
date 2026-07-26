package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMergeOmittedSettings_CardShopProductsOnlyPreservesOtherSettings(t *testing.T) {
	current := &service.SystemSettings{
		RegistrationEnabled:              true,
		EmailVerifyEnabled:               true,
		RegistrationEmailSuffixWhitelist: []string{"example.com"},
		PromoCodeEnabled:                 false,
		PasswordResetEnabled:             true,
		FrontendURL:                      "https://console.example.com",
		InvitationCodeEnabled:            true,
		TotpEnabled:                      true,
		SMTPHost:                         "smtp.example.com",
		SMTPPort:                         465,
		SMTPUsername:                     "mailer",
		SMTPFrom:                         "noreply@example.com",
		SMTPFromName:                     "Example",
		SMTPUseTLS:                       true,
		FeedbackNotifyEmail:              "feedback@example.com",
		TurnstileEnabled:                 true,
		TurnstileSiteKey:                 "site-key",
		LinuxDoConnectEnabled:            true,
		LinuxDoConnectClientID:           "linuxdo-client",
		LinuxDoConnectRedirectURL:        "https://console.example.com/oauth/linuxdo/callback",
		OIDCConnectEnabled:               true,
		OIDCConnectProviderName:          "Example ID",
		OIDCConnectClientID:              "oidc-client",
		OIDCConnectIssuerURL:             "https://id.example.com",
		OIDCConnectDiscoveryURL:          "https://id.example.com/.well-known/openid-configuration",
		OIDCConnectAuthorizeURL:          "https://id.example.com/authorize",
		OIDCConnectTokenURL:              "https://id.example.com/token",
		OIDCConnectUserInfoURL:           "https://id.example.com/userinfo",
		OIDCConnectJWKSURL:               "https://id.example.com/jwks",
		OIDCConnectScopes:                "openid email profile",
		OIDCConnectRedirectURL:           "https://api.example.com/oauth/oidc/callback",
		OIDCConnectFrontendRedirectURL:   "/auth/oidc/callback",
		OIDCConnectTokenAuthMethod:       "client_secret_post",
		OIDCConnectUsePKCE:               true,
		OIDCConnectValidateIDToken:       true,
		OIDCConnectAllowedSigningAlgs:    "RS256",
		OIDCConnectClockSkewSeconds:      120,
		OIDCConnectRequireEmailVerified:  true,
		OIDCConnectUserInfoEmailPath:     "email",
		OIDCConnectUserInfoIDPath:        "sub",
		OIDCConnectUserInfoUsernamePath:  "name",
		GitHubOAuthEnabled:               true,
		GitHubOAuthClientID:              "github-client",
		GitHubOAuthRedirectURL:           "https://api.example.com/oauth/github/callback",
		GitHubOAuthFrontendRedirectURL:   "/auth/github/callback",
		SiteName:                         "Example",
		SiteLogo:                         "https://cdn.example.com/logo.png",
		SiteSubtitle:                     "Example subtitle",
		APIBaseURL:                       "https://api.example.com",
		ContactInfo:                      "support",
		TechSupportQRCode:                "https://cdn.example.com/tech.png",
		AfterSalesQRCode:                 "https://cdn.example.com/sales.png",
		DocURL:                           "https://docs.example.com",
		ChatbotURL:                       "https://chat.example.com",
		HomeContent:                      "Welcome",
		HideCcsImportButton:              true,
		SoraClientEnabled:                true,
		TableDefaultPageSize:             50,
		TablePageSizeOptions:             []int{20, 50, 100},
		DefaultConcurrency:               9,
		DefaultBalance:                   12.5,
		DefaultSubscriptions: []service.DefaultSubscriptionSetting{
			{GroupID: 7, ValidityDays: 30},
		},
		EnableModelFallback:         true,
		FallbackModelAnthropic:      "claude-fallback",
		FallbackModelOpenAI:         "openai-fallback",
		FallbackModelGemini:         "gemini-fallback",
		FallbackModelAntigravity:    "antigravity-fallback",
		EnableIdentityPatch:         false,
		IdentityPatchPrompt:         "identity",
		MinClaudeCodeVersion:        "2.1.0",
		MaxClaudeCodeVersion:        "3.0.0",
		AllowUngroupedKeyScheduling: true,
		BackendModeEnabled:          true,
		StripeEnabled:               true,
		AlipayEnabled:               true,
		AlipayAppID:                 "alipay-app",
		AlipayNotifyURL:             "https://api.example.com/alipay/notify",
		XunhuAlipayEnabled:          true,
		XunhuAlipayAppID:            "xunhu-alipay-app",
		XunhuWechatEnabled:          true,
		XunhuWechatAppID:            "xunhu-wechat-app",
		XunhuNotifyURL:              "https://api.example.com/xunhu/notify",
	}

	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"card_shop_products":[]}`), &req))
	mergeOmittedSettings(&req, current)

	require.NotNil(t, req.CardShopProducts)
	require.Empty(t, *req.CardShopProducts)
	require.True(t, req.RegistrationEnabled)
	require.True(t, req.EmailVerifyEnabled)
	require.Equal(t, []string{"example.com"}, req.RegistrationEmailSuffixWhitelist)
	require.False(t, req.PromoCodeEnabled)
	require.Equal(t, "smtp.example.com", req.SMTPHost)
	require.True(t, req.TurnstileEnabled)
	require.True(t, req.LinuxDoConnectEnabled)
	require.True(t, req.OIDCConnectEnabled)
	require.True(t, req.GitHubOAuthEnabled)
	require.Equal(t, "https://api.example.com", req.APIBaseURL)
	require.Equal(t, "Example", req.SiteName)
	require.Equal(t, "https://cdn.example.com/logo.png", req.SiteLogo)
	require.Equal(t, 9, req.DefaultConcurrency)
	require.Equal(t, 12.5, req.DefaultBalance)
	require.Equal(t, []dto.DefaultSubscriptionSetting{{GroupID: 7, ValidityDays: 30}}, req.DefaultSubscriptions)
	require.True(t, req.EnableModelFallback)
	require.Equal(t, "claude-fallback", req.FallbackModelAnthropic)
	require.Equal(t, "openai-fallback", req.FallbackModelOpenAI)
	require.Equal(t, "gemini-fallback", req.FallbackModelGemini)
	require.Equal(t, "antigravity-fallback", req.FallbackModelAntigravity)
	require.True(t, req.AllowUngroupedKeyScheduling)
	require.True(t, req.BackendModeEnabled)
	require.True(t, req.StripeEnabled)
	require.Equal(t, "alipay-app", req.AlipayAppID)
	require.Equal(t, "xunhu-wechat-app", req.XunhuWechatAppID)

	// Sensitive values are never copied into the request. Empty keeps the
	// persisted secret in SettingService.UpdateSettings.
	require.Empty(t, req.SMTPPassword)
	require.Empty(t, req.TurnstileSecretKey)
	require.Empty(t, req.LinuxDoConnectClientSecret)
	require.Empty(t, req.OIDCConnectClientSecret)
	require.Empty(t, req.GitHubOAuthClientSecret)
	require.Empty(t, req.StripeSecretKey)
	require.Empty(t, req.AlipayPrivateKey)
	require.Empty(t, req.XunhuWechatKey)
}

func TestMergeOmittedSettings_ExplicitZeroValuesAreNotReplaced(t *testing.T) {
	current := &service.SystemSettings{
		RegistrationEnabled:              true,
		RegistrationEmailSuffixWhitelist: []string{"example.com"},
		APIBaseURL:                       "https://api.example.com",
		SiteName:                         "Example",
		DefaultConcurrency:               9,
		DefaultBalance:                   12.5,
		EnableModelFallback:              true,
		FallbackModelOpenAI:              "gpt-current",
		AlipayAppID:                      "alipay-current",
	}

	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"registration_enabled": false,
		"registration_email_suffix_whitelist": [],
		"api_base_url": "",
		"site_name": "",
		"default_balance": 0,
		"enable_model_fallback": false,
		"fallback_model_openai": "",
		"alipay_app_id": ""
	}`), &req))
	mergeOmittedSettings(&req, current)

	require.False(t, req.RegistrationEnabled)
	require.Empty(t, req.RegistrationEmailSuffixWhitelist)
	require.Empty(t, req.APIBaseURL)
	require.Empty(t, req.SiteName)
	require.Zero(t, req.DefaultBalance)
	require.False(t, req.EnableModelFallback)
	require.Empty(t, req.FallbackModelOpenAI)
	require.Empty(t, req.AlipayAppID)
	require.Equal(t, 9, req.DefaultConcurrency, "omitted value must still be preserved")
}

func TestSettingHandler_UpdateSettings_CardShopProductsOnlyPreservesExistingSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newTestSettingRepo()
	repo.values = map[string]string{
		service.SettingKeyRegistrationEnabled:              "true",
		service.SettingKeyEmailVerifyEnabled:               "true",
		service.SettingKeyRegistrationEmailSuffixWhitelist: `["@example.com"]`,
		service.SettingKeyPromoCodeEnabled:                 "false",
		service.SettingKeyPasswordResetEnabled:             "true",
		service.SettingKeyFrontendURL:                      "https://console.example.com",
		service.SettingKeyInvitationCodeEnabled:            "true",
		service.SettingKeySMTPHost:                         "smtp.example.com",
		service.SettingKeySMTPPort:                         "465",
		service.SettingKeySMTPUsername:                     "mailer",
		service.SettingKeySMTPPassword:                     "smtp-secret",
		service.SettingKeySMTPFrom:                         "noreply@example.com",
		service.SettingKeySMTPFromName:                     "Example",
		service.SettingKeySMTPUseTLS:                       "true",
		service.SettingKeyTurnstileEnabled:                 "false",
		service.SettingKeyTurnstileSecretKey:               "turnstile-secret",
		service.SettingKeyLinuxDoConnectEnabled:            "true",
		service.SettingKeyLinuxDoConnectClientID:           "linuxdo-client",
		service.SettingKeyLinuxDoConnectClientSecret:       "linuxdo-secret",
		service.SettingKeyLinuxDoConnectRedirectURL:        "https://console.example.com/oauth/linuxdo/callback",
		service.SettingKeyOIDCConnectEnabled:               "true",
		service.SettingKeyOIDCConnectProviderName:          "Example ID",
		service.SettingKeyOIDCConnectClientID:              "oidc-client",
		service.SettingKeyOIDCConnectClientSecret:          "oidc-secret",
		service.SettingKeyOIDCConnectIssuerURL:             "https://id.example.com",
		service.SettingKeyOIDCConnectScopes:                "openid email profile",
		service.SettingKeyOIDCConnectRedirectURL:           "https://api.example.com/oauth/oidc/callback",
		service.SettingKeyOIDCConnectFrontendRedirectURL:   "/auth/oidc/callback",
		service.SettingKeyOIDCConnectTokenAuthMethod:       "client_secret_post",
		service.SettingKeyOIDCConnectClockSkewSeconds:      "120",
		service.SettingKeyGitHubOAuthEnabled:               "true",
		service.SettingKeyGitHubOAuthClientID:              "github-client",
		service.SettingKeyGitHubOAuthClientSecret:          "github-secret",
		service.SettingKeyGitHubOAuthRedirectURL:           "https://api.example.com/oauth/github/callback",
		service.SettingKeyGitHubOAuthFrontendRedirectURL:   "/auth/github/callback",
		service.SettingKeySiteName:                         "Example",
		service.SettingKeySiteLogo:                         "https://cdn.example.com/logo.png",
		service.SettingKeySiteSubtitle:                     "Example subtitle",
		service.SettingKeyAPIBaseURL:                       "https://api.example.com",
		service.SettingKeyContactInfo:                      "support",
		service.SettingKeyTechSupportQRCode:                "https://cdn.example.com/tech.png",
		service.SettingKeyAfterSalesQRCode:                 "https://cdn.example.com/sales.png",
		service.SettingKeyDocURL:                           "https://docs.example.com",
		service.SettingKeyChatbotURL:                       "https://chat.example.com",
		service.SettingKeyHomeContent:                      "Welcome",
		service.SettingKeyDefaultConcurrency:               "9",
		service.SettingKeyDefaultBalance:                   "12.50000000",
		service.SettingKeyDefaultSubscriptions:             "[]",
		service.SettingKeyEnableModelFallback:              "true",
		service.SettingKeyFallbackModelAnthropic:           "claude-fallback",
		service.SettingKeyFallbackModelOpenAI:              "openai-fallback",
		service.SettingKeyFallbackModelGemini:              "gemini-fallback",
		service.SettingKeyFallbackModelAntigravity:         "antigravity-fallback",
		service.SettingKeyCardShopProducts:                 `[{"id":"old","label":"Old","amount_cny":10,"url":"https://shop.example.com/old","enabled":true,"sort_order":0}]`,
		service.SettingKeyStripeSecretKey:                  "stripe-secret",
		service.SettingKeyStripeWebhookSecret:              "stripe-webhook-secret",
		service.SettingKeyAlipayPrivateKey:                 "alipay-private",
		service.SettingKeyAlipayPublicKey:                  "alipay-public",
		service.SettingKeyXunhuAlipayKey:                   "xunhu-alipay-secret",
		service.SettingKeyXunhuWechatKey:                   "xunhu-wechat-secret",
	}
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     5,
		},
	}
	settingService := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(settingService, nil, nil, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/settings", handler.UpdateSettings)

	keysThatMustNotChange := []string{
		service.SettingKeyRegistrationEnabled,
		service.SettingKeyEmailVerifyEnabled,
		service.SettingKeyRegistrationEmailSuffixWhitelist,
		service.SettingKeyPromoCodeEnabled,
		service.SettingKeyPasswordResetEnabled,
		service.SettingKeyFrontendURL,
		service.SettingKeyInvitationCodeEnabled,
		service.SettingKeySMTPHost,
		service.SettingKeySMTPPort,
		service.SettingKeySMTPUsername,
		service.SettingKeySMTPPassword,
		service.SettingKeySMTPFrom,
		service.SettingKeySMTPFromName,
		service.SettingKeySMTPUseTLS,
		service.SettingKeyTurnstileSecretKey,
		service.SettingKeyLinuxDoConnectEnabled,
		service.SettingKeyLinuxDoConnectClientID,
		service.SettingKeyLinuxDoConnectClientSecret,
		service.SettingKeyLinuxDoConnectRedirectURL,
		service.SettingKeyOIDCConnectEnabled,
		service.SettingKeyOIDCConnectProviderName,
		service.SettingKeyOIDCConnectClientID,
		service.SettingKeyOIDCConnectClientSecret,
		service.SettingKeyOIDCConnectIssuerURL,
		service.SettingKeyOIDCConnectScopes,
		service.SettingKeyOIDCConnectRedirectURL,
		service.SettingKeyOIDCConnectFrontendRedirectURL,
		service.SettingKeyOIDCConnectTokenAuthMethod,
		service.SettingKeyOIDCConnectClockSkewSeconds,
		service.SettingKeyGitHubOAuthEnabled,
		service.SettingKeyGitHubOAuthClientID,
		service.SettingKeyGitHubOAuthClientSecret,
		service.SettingKeyGitHubOAuthRedirectURL,
		service.SettingKeyGitHubOAuthFrontendRedirectURL,
		service.SettingKeySiteName,
		service.SettingKeySiteLogo,
		service.SettingKeySiteSubtitle,
		service.SettingKeyAPIBaseURL,
		service.SettingKeyContactInfo,
		service.SettingKeyTechSupportQRCode,
		service.SettingKeyAfterSalesQRCode,
		service.SettingKeyDocURL,
		service.SettingKeyChatbotURL,
		service.SettingKeyHomeContent,
		service.SettingKeyDefaultConcurrency,
		service.SettingKeyDefaultBalance,
		service.SettingKeyDefaultSubscriptions,
		service.SettingKeyEnableModelFallback,
		service.SettingKeyFallbackModelAnthropic,
		service.SettingKeyFallbackModelOpenAI,
		service.SettingKeyFallbackModelGemini,
		service.SettingKeyFallbackModelAntigravity,
		service.SettingKeyStripeSecretKey,
		service.SettingKeyStripeWebhookSecret,
		service.SettingKeyAlipayPrivateKey,
		service.SettingKeyAlipayPublicKey,
		service.SettingKeyXunhuAlipayKey,
		service.SettingKeyXunhuWechatKey,
	}
	before := make(map[string]string, len(keysThatMustNotChange))
	for _, key := range keysThatMustNotChange {
		before[key] = repo.values[key]
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/settings",
		bytes.NewBufferString(`{"card_shop_products":[]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.JSONEq(t, "[]", repo.values[service.SettingKeyCardShopProducts])
	require.Equal(
		t,
		map[string]string{service.SettingKeyCardShopProducts: "[]"},
		repo.lastSetMultiple,
		"partial request must persist only its target setting",
	)
	for _, key := range keysThatMustNotChange {
		require.Equalf(t, before[key], repo.values[key], "setting %s changed", key)
	}
}

func TestSettingHandler_UpdateSettings_ExplicitZeroAndEmptyValuesAreApplied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newTestSettingRepo()
	repo.values = map[string]string{
		service.SettingKeyRegistrationEnabled:         "true",
		service.SettingKeyAPIBaseURL:                  "https://api.example.com",
		service.SettingKeyDefaultConcurrency:          "9",
		service.SettingKeyDefaultBalance:              "12.50000000",
		service.SettingKeyDefaultSubscriptions:        "[]",
		service.SettingKeySMTPHost:                    "smtp.example.com",
		service.SettingKeySMTPPort:                    "587",
		service.SettingKeySMTPPassword:                "smtp-secret",
		service.SettingKeyStripeSecretKey:             "stripe-secret",
		service.SettingKeyPurchaseSubscriptionEnabled: "true",
		service.SettingKeyPurchaseSubscriptionURL:     "https://shop.example.com",
		service.SettingKeyCardShopEnabled:             "true",
		service.SettingKeyCardShopProducts:            `[{"id":"old","label":"Old","amount_cny":10,"url":"https://shop.example.com/old","enabled":true,"sort_order":0}]`,
		service.SettingKeyLandingReportsEnabled:       "true",
		service.SettingKeyCustomMenuItems:             `[{"id":"docs","label":"Docs","url":"https://docs.example.com","visibility":"user","icon_svg":""}]`,
		service.SettingKeyAlipayAppID:                 "alipay-app",
	}
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     5,
		},
	}
	settingService := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(settingService, nil, nil, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/settings", handler.UpdateSettings)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/settings",
		bytes.NewBufferString(`{
			"registration_enabled": false,
			"api_base_url": "",
			"default_balance": 0,
			"smtp_host": "",
			"smtp_password": "",
			"stripe_secret_key": "",
			"purchase_subscription_enabled": false,
			"purchase_subscription_url": "",
			"card_shop_enabled": false,
			"card_shop_products": [],
			"landing_reports_enabled": false,
			"custom_menu_items": [],
			"alipay_app_id": ""
		}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "false", repo.values[service.SettingKeyRegistrationEnabled])
	require.Empty(t, repo.values[service.SettingKeyAPIBaseURL])
	require.Equal(t, "0.00000000", repo.values[service.SettingKeyDefaultBalance])
	require.Empty(t, repo.values[service.SettingKeySMTPHost])
	require.Equal(t, "smtp-secret", repo.values[service.SettingKeySMTPPassword])
	require.Equal(t, "stripe-secret", repo.values[service.SettingKeyStripeSecretKey])
	require.Equal(t, "false", repo.values[service.SettingKeyPurchaseSubscriptionEnabled])
	require.Empty(t, repo.values[service.SettingKeyPurchaseSubscriptionURL])
	require.Equal(t, "false", repo.values[service.SettingKeyCardShopEnabled])
	require.JSONEq(t, "[]", repo.values[service.SettingKeyCardShopProducts])
	require.Equal(t, "false", repo.values[service.SettingKeyLandingReportsEnabled])
	require.JSONEq(t, "[]", repo.values[service.SettingKeyCustomMenuItems])
	require.Empty(t, repo.values[service.SettingKeyAlipayAppID])
	require.Equal(t, "9", repo.values[service.SettingKeyDefaultConcurrency], "omitted field changed")

	require.Equal(t, map[string]string{
		service.SettingKeyRegistrationEnabled:         "false",
		service.SettingKeyAPIBaseURL:                  "",
		service.SettingKeyDefaultBalance:              "0.00000000",
		service.SettingKeySMTPHost:                    "",
		service.SettingKeyPurchaseSubscriptionEnabled: "false",
		service.SettingKeyPurchaseSubscriptionURL:     "",
		service.SettingKeyCardShopEnabled:             "false",
		service.SettingKeyCardShopProducts:            "[]",
		service.SettingKeyLandingReportsEnabled:       "false",
		service.SettingKeyCustomMenuItems:             "[]",
		service.SettingKeyAlipayAppID:                 "",
	}, repo.lastSetMultiple)
}

func TestMergeOmittedSettings_NullIsTreatedAsOmitted(t *testing.T) {
	current := &service.SystemSettings{
		RegistrationEnabled: true,
		APIBaseURL:          "https://api.example.com",
		DefaultBalance:      12.5,
	}

	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"registration_enabled": null,
		"api_base_url": null,
		"default_balance": null
	}`), &req))
	mergeOmittedSettings(&req, current)

	require.True(t, req.RegistrationEnabled)
	require.Equal(t, "https://api.example.com", req.APIBaseURL)
	require.Equal(t, 12.5, req.DefaultBalance)
	require.Empty(t, req.providedSettingKeys())
}

func TestUpdateSettingsRequest_ProvidedSettingKeysPreservesBlankSecrets(t *testing.T) {
	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"smtp_password": "",
		"stripe_secret_key": "  ",
		"oidc_connect_client_secret": "new-secret",
		"smtp_from_email": "noreply@example.com",
		"api_base_url": ""
	}`), &req))

	require.Equal(t, map[string]struct{}{
		service.SettingKeyOIDCConnectClientSecret: {},
		service.SettingKeySMTPFrom:                {},
		service.SettingKeyAPIBaseURL:              {},
	}, req.providedSettingKeys())
	require.False(t, req.nonEmptySecretProvided("smtp_password"))
	require.True(t, req.nonEmptySecretProvided("oidc_connect_client_secret"))
}
