package handler

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler 公开设置处理器（无需认证）
type SettingHandler struct {
	settingService    *service.SettingService
	commissionService *service.CommissionService
	version           string
}

// NewSettingHandler 创建公开设置处理器
func NewSettingHandler(settingService *service.SettingService, commissionService *service.CommissionService, version string) *SettingHandler {
	return &SettingHandler{
		settingService:    settingService,
		commissionService: commissionService,
		version:           version,
	}
}

// GetPublicSettings 获取公开设置
// GET /api/v1/settings/public
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingService.GetPublicSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	registrationEmailSuffixWhitelist := settings.RegistrationEmailSuffixWhitelist
	if h.commissionService != nil {
		if activeWhitelist, active := h.commissionService.ActiveInviteActivityEmailSuffixWhitelist(c.Request.Context(), settings.RegistrationEmailSuffixWhitelist); active {
			registrationEmailSuffixWhitelist = activeWhitelist
		} else {
			registrationEmailSuffixWhitelist = []string{}
		}
	}

	response.Success(c, dto.PublicSettings{
		TeamEnabled:                      settings.TeamEnabled,
		TeamSelfServiceEnabled:           settings.TeamSelfServiceEnabled,
		RegistrationEnabled:              settings.RegistrationEnabled,
		EmailVerifyEnabled:               settings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: registrationEmailSuffixWhitelist,
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
		CardShopProducts:                 dto.CardShopProductsFromService(settings.CardShopProducts),
		InvoiceManagementEnabled:         settings.InvoiceManagementEnabled,
		FeedbackManagementEnabled:        settings.FeedbackManagementEnabled,
		GroupCacheHitRateEnabled:         settings.GroupCacheHitRateEnabled,
		CustomMenuItems:                  dto.ParseUserVisibleMenuItems(settings.CustomMenuItems),
		LinuxDoOAuthEnabled:              settings.LinuxDoOAuthEnabled,
		OIDCOAuthEnabled:                 settings.OIDCOAuthEnabled,
		OIDCOAuthProviderName:            settings.OIDCOAuthProviderName,
		GitHubOAuthEnabled:               settings.GitHubOAuthEnabled,
		SoraClientEnabled:                settings.SoraClientEnabled,
		BackendModeEnabled:               settings.BackendModeEnabled,
		PaymentEnabled:                   settings.PaymentEnabled,
		StripeEnabled:                    settings.StripeEnabled,
		AlipayEnabled:                    settings.AlipayEnabled,
		XunhuAlipayEnabled:               settings.XunhuAlipayEnabled,
		XunhuWechatEnabled:               settings.XunhuWechatEnabled,
		TopupAlipayEnabled:               settings.TopupAlipayEnabled,
		TopupWechatEnabled:               settings.TopupWechatEnabled,
		Version:                          h.version,
		AccountQuotaNotifyEnabled:        settings.AccountQuotaNotifyEnabled,
	})
}
