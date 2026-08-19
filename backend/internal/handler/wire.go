package handler

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/admin"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/google/wire"
)

// ProvideAdminHandlers creates the AdminHandlers struct
func ProvideAdminHandlers(
	dashboardHandler *admin.DashboardHandler,
	agentHandler *admin.AgentHandler,
	userHandler *admin.UserHandler,
	groupHandler *admin.GroupHandler,
	accountHandler *admin.AccountHandler,
	announcementHandler *admin.AnnouncementHandler,
	changelogHandler *admin.ChangelogHandler,
	feedbackHandler *admin.FeedbackHandler,
	financeTransactionHandler *admin.FinanceTransactionHandler,
	dataManagementHandler *admin.DataManagementHandler,
	backupHandler *admin.BackupHandler,
	oauthHandler *admin.OAuthHandler,
	openaiOAuthHandler *admin.OpenAIOAuthHandler,
	geminiOAuthHandler *admin.GeminiOAuthHandler,
	antigravityOAuthHandler *admin.AntigravityOAuthHandler,
	grokOAuthHandler *admin.GrokOAuthHandler,
	proxyHandler *admin.ProxyHandler,
	redeemHandler *admin.RedeemHandler,
	promoHandler *admin.PromoHandler,
	settingHandler *admin.SettingHandler,
	opsHandler *admin.OpsHandler,
	systemHandler *admin.SystemHandler,
	subscriptionHandler *admin.SubscriptionHandler,
	usageHandler *admin.UsageHandler,
	invoiceHandler *admin.InvoiceHandler,
	userAttributeHandler *admin.UserAttributeHandler,
	errorPassthroughHandler *admin.ErrorPassthroughHandler,
	tlsFingerprintProfileHandler *admin.TLSFingerprintProfileHandler,
	apiKeyHandler *admin.AdminAPIKeyHandler,
	scheduledTestHandler *admin.ScheduledTestHandler,
	channelHandler *admin.ChannelHandler,
	supplierHandler *admin.SupplierHandler,
	rbacHandler *admin.RBACHandler,
) *AdminHandlers {
	return &AdminHandlers{
		Dashboard:             dashboardHandler,
		Agent:                 agentHandler,
		User:                  userHandler,
		Group:                 groupHandler,
		Account:               accountHandler,
		Announcement:          announcementHandler,
		Changelog:             changelogHandler,
		Feedback:              feedbackHandler,
		FinanceTransaction:    financeTransactionHandler,
		DataManagement:        dataManagementHandler,
		Backup:                backupHandler,
		OAuth:                 oauthHandler,
		OpenAIOAuth:           openaiOAuthHandler,
		GeminiOAuth:           geminiOAuthHandler,
		AntigravityOAuth:      antigravityOAuthHandler,
		GrokOAuth:             grokOAuthHandler,
		Proxy:                 proxyHandler,
		Redeem:                redeemHandler,
		Promo:                 promoHandler,
		Setting:               settingHandler,
		Ops:                   opsHandler,
		System:                systemHandler,
		Subscription:          subscriptionHandler,
		Usage:                 usageHandler,
		Invoice:               invoiceHandler,
		UserAttribute:         userAttributeHandler,
		ErrorPassthrough:      errorPassthroughHandler,
		TLSFingerprintProfile: tlsFingerprintProfileHandler,
		APIKey:                apiKeyHandler,
		ScheduledTest:         scheduledTestHandler,
		Channel:               channelHandler,
		Supplier:              supplierHandler,
		RBAC:                  rbacHandler,
	}
}

// ProvideSystemHandler creates admin.SystemHandler with UpdateService
func ProvideSystemHandler(updateService *service.UpdateService, lockService *service.SystemOperationLockService) *admin.SystemHandler {
	return admin.NewSystemHandler(updateService, lockService)
}

// ProvideSettingHandler creates SettingHandler with version from BuildInfo
func ProvideSettingHandler(settingService *service.SettingService, commissionService *service.CommissionService, buildInfo BuildInfo) *SettingHandler {
	return NewSettingHandler(settingService, commissionService, buildInfo.Version)
}

func ProvideUserHandler(userService *service.UserService, commissionService *service.CommissionService, identityService *service.IdentityService, settingService *service.SettingService, cfg *config.Config) *UserHandler {
	return NewUserHandler(userService, commissionService, identityService, settingService, cfg)
}

func ProvideOpenAIGatewayHandler(
	gatewayService *service.OpenAIGatewayService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	grokQuotaService *service.GrokQuotaService,
	cfg *config.Config,
) *OpenAIGatewayHandler {
	h := NewOpenAIGatewayHandler(
		gatewayService, concurrencyService, billingCacheService, apiKeyService,
		usageRecordWorkerPool, errorPassthroughService, cfg,
	)
	h.SetGrokMediaEligibilityProber(grokQuotaService)
	return h
}

// ProvideTopupEasyPayNotifier adapts *service.TopupService to the handler-local
// TopupEasyPayNotifier interface (a cross-package wire.Bind cannot see the
// service.ProviderSet provider, so the adaptation is explicit).
func ProvideTopupEasyPayNotifier(topupService *service.TopupService) TopupEasyPayNotifier {
	return topupService
}

// ProvideNativeCheckoutEasyPayNotifier adapts *service.NativeCheckoutService to the
// handler-local NativeCheckoutEasyPayNotifier interface (same cross-package
// wire.Bind limitation as the topup adapter above).
func ProvideNativeCheckoutEasyPayNotifier(nativeCheckoutService *service.NativeCheckoutService) NativeCheckoutEasyPayNotifier {
	return nativeCheckoutService
}

// ProvideHandlers creates the Handlers struct
func ProvideHandlers(
	authHandler *AuthHandler,
	userHandler *UserHandler,
	agentHandler *AgentHandler,
	apiKeyHandler *APIKeyHandler,
	usageHandler *UsageHandler,
	redeemHandler *RedeemHandler,
	subscriptionHandler *SubscriptionHandler,
	announcementHandler *AnnouncementHandler,
	changelogHandler *ChangelogHandler,
	invoiceHandler *InvoiceHandler,
	feedbackHandler *FeedbackHandler,
	adminHandlers *AdminHandlers,
	gatewayHandler *GatewayHandler,
	openaiGatewayHandler *OpenAIGatewayHandler,
	settingHandler *SettingHandler,
	modelPricingHandler *ModelPricingHandler,
	totpHandler *TotpHandler,
	paymentHandler *PaymentHandler,
	topupHandler *TopupHandler,
	paymentGatewayHandler *PaymentGatewayHandler,
	nativeCheckoutHandler *NativeCheckoutHandler,
	balanceAlertHandler *BalanceAlertHandler,
	resourceHandler *ResourceHandler,
	_ *service.IdempotencyCoordinator,
	_ *service.IdempotencyCleanupService,
	_ *service.PendingAuthSessionCleanupService,
) *Handlers {
	return &Handlers{
		Auth:           authHandler,
		User:           userHandler,
		Agent:          agentHandler,
		APIKey:         apiKeyHandler,
		Usage:          usageHandler,
		Redeem:         redeemHandler,
		Subscription:   subscriptionHandler,
		Announcement:   announcementHandler,
		Changelog:      changelogHandler,
		Invoice:        invoiceHandler,
		Feedback:       feedbackHandler,
		Admin:          adminHandlers,
		Gateway:        gatewayHandler,
		OpenAIGateway:  openaiGatewayHandler,
		Setting:        settingHandler,
		ModelPricing:   modelPricingHandler,
		Totp:           totpHandler,
		Payment:        paymentHandler,
		Topup:          topupHandler,
		PaymentGateway: paymentGatewayHandler,
		NativeCheckout: nativeCheckoutHandler,
		BalanceAlert:   balanceAlertHandler,
		Resource:       resourceHandler,
	}
}

// ProviderSet is the Wire provider set for all handlers
var ProviderSet = wire.NewSet(
	// Top-level handlers
	NewAuthHandler,
	ProvideUserHandler,
	NewAgentHandler,
	NewAPIKeyHandler,
	NewUsageHandler,
	NewRedeemHandler,
	NewSubscriptionHandler,
	NewAnnouncementHandler,
	NewChangelogHandler,
	NewInvoiceHandler,
	NewFeedbackHandler,
	NewGatewayHandler,
	ProvideOpenAIGatewayHandler,
	NewTotpHandler,
	NewPaymentHandler,
	NewTopupHandler,
	NewPaymentGatewayHandler,
	ProvideTopupEasyPayNotifier,
	ProvideNativeCheckoutEasyPayNotifier,
	NewNativeCheckoutHandler,
	NewBalanceAlertHandler,
	NewResourceHandler,
	NewModelPricingHandler,
	ProvideSettingHandler,

	// Admin handlers
	admin.NewDashboardHandler,
	admin.NewAgentHandler,
	admin.NewUserHandler,
	admin.NewGroupHandler,
	admin.ProvideAccountHandler,
	admin.NewAnnouncementHandler,
	admin.NewChangelogHandler,
	admin.NewFeedbackHandler,
	admin.NewFinanceTransactionHandler,
	admin.NewDataManagementHandler,
	admin.NewBackupHandler,
	admin.NewOAuthHandler,
	admin.NewOpenAIOAuthHandler,
	admin.NewGeminiOAuthHandler,
	admin.NewAntigravityOAuthHandler,
	admin.NewGrokOAuthHandler,
	admin.NewProxyHandler,
	admin.NewRedeemHandler,
	admin.NewPromoHandler,
	admin.NewSettingHandler,
	admin.NewOpsHandler,
	ProvideSystemHandler,
	admin.NewSubscriptionHandler,
	admin.NewUsageHandler,
	admin.NewInvoiceHandler,
	admin.NewUserAttributeHandler,
	admin.NewErrorPassthroughHandler,
	admin.NewTLSFingerprintProfileHandler,
	admin.NewAdminAPIKeyHandler,
	admin.NewScheduledTestHandler,
	admin.NewChannelHandler,
	admin.NewSupplierHandler,
	admin.NewRBACHandler,

	// AdminHandlers and Handlers constructors
	ProvideAdminHandlers,
	ProvideHandlers,
)
