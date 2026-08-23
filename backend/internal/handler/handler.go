package handler

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/admin"
)

// AdminHandlers contains all admin-related HTTP handlers
type AdminHandlers struct {
	Dashboard             *admin.DashboardHandler
	Agent                 *admin.AgentHandler
	User                  *admin.UserHandler
	Group                 *admin.GroupHandler
	Account               *admin.AccountHandler
	Announcement          *admin.AnnouncementHandler
	Changelog             *admin.ChangelogHandler
	Feedback              *admin.FeedbackHandler
	FinanceTransaction    *admin.FinanceTransactionHandler
	DataManagement        *admin.DataManagementHandler
	Backup                *admin.BackupHandler
	OAuth                 *admin.OAuthHandler
	OpenAIOAuth           *admin.OpenAIOAuthHandler
	GeminiOAuth           *admin.GeminiOAuthHandler
	AntigravityOAuth      *admin.AntigravityOAuthHandler
	GrokOAuth             *admin.GrokOAuthHandler
	Proxy                 *admin.ProxyHandler
	Redeem                *admin.RedeemHandler
	Promo                 *admin.PromoHandler
	Setting               *admin.SettingHandler
	Ops                   *admin.OpsHandler
	System                *admin.SystemHandler
	Subscription          *admin.SubscriptionHandler
	Usage                 *admin.UsageHandler
	Invoice               *admin.InvoiceHandler
	UserAttribute         *admin.UserAttributeHandler
	ErrorPassthrough      *admin.ErrorPassthroughHandler
	TLSFingerprintProfile *admin.TLSFingerprintProfileHandler
	APIKey                *admin.AdminAPIKeyHandler
	ScheduledTest         *admin.ScheduledTestHandler
	Channel               *admin.ChannelHandler
	Supplier              *admin.SupplierHandler
	RBAC                  *admin.RBACHandler
}

// Handlers contains all HTTP handlers
type Handlers struct {
	Auth           *AuthHandler
	User           *UserHandler
	Agent          *AgentHandler
	APIKey         *APIKeyHandler
	Usage          *UsageHandler
	Redeem         *RedeemHandler
	Subscription   *SubscriptionHandler
	Announcement   *AnnouncementHandler
	Changelog      *ChangelogHandler
	Invoice        *InvoiceHandler
	Feedback       *FeedbackHandler
	Admin          *AdminHandlers
	Gateway        *GatewayHandler
	OpenAIGateway  *OpenAIGatewayHandler
	Setting        *SettingHandler
	ModelPricing   *ModelPricingHandler
	Totp           *TotpHandler
	Payment        *PaymentHandler
	Topup          *TopupHandler
	PaymentGateway *PaymentGatewayHandler
	NativeCheckout *NativeCheckoutHandler
	BalanceAlert   *BalanceAlertHandler
	Resource       *ResourceHandler
	Status         *StatusHandler
	CustomerTier   *CustomerTierHandler
}

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	BuildType string // "source" for manual builds, "release" for CI builds
}
