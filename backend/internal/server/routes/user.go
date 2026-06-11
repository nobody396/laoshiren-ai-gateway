package routes

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler"
	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes 注册用户相关路由（需要认证）
func RegisterUserRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	settingService *service.SettingService,
) {
	v1.GET("/resource-downloads/:token", h.Resource.DownloadWithToken)

	authenticated := v1.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	authenticated.Use(middleware.BackendModeUserGuard(settingService))
	{
		// 用户接口
		user := authenticated.Group("/user")
		{
			user.GET("/profile", h.User.GetProfile)
			user.PUT("/password", h.User.ChangePassword)
			user.PUT("", h.User.UpdateProfile)
			user.GET("/identities", h.User.ListIdentityBindings)
			user.POST("/identities/linuxdo/start-bind", h.User.StartLinuxDoBind)
			user.POST("/identities/google/start-bind", h.User.StartGoogleBind)
			user.POST("/identities/github/start-bind", h.User.StartGitHubBind)
			user.DELETE("/identities/:provider", h.User.UnbindIdentity)

			balanceAlert := user.Group("/balance-alert")
			{
				balanceAlert.GET("", h.BalanceAlert.GetConfig)
				balanceAlert.PUT("", h.BalanceAlert.UpdateConfig)
			}

			// TOTP 双因素认证
			totp := user.Group("/totp")
			{
				totp.GET("/status", h.Totp.GetStatus)
				totp.GET("/verification-method", h.Totp.GetVerificationMethod)
				totp.POST("/send-code", h.Totp.SendVerifyCode)
				totp.POST("/setup", h.Totp.InitiateSetup)
				totp.POST("/enable", h.Totp.Enable)
				totp.POST("/disable", h.Totp.Disable)
			}
		}

		// API Key管理
		keys := authenticated.Group("/keys")
		{
			keys.GET("", h.APIKey.List)
			keys.GET("/:id", h.APIKey.GetByID)
			keys.POST("", h.APIKey.Create)
			keys.PUT("/:id", h.APIKey.Update)
			keys.DELETE("/:id", h.APIKey.Delete)
		}

		// 用户可用分组（非管理员接口）
		groups := authenticated.Group("/groups")
		{
			groups.GET("/available", h.APIKey.GetAvailableGroups)
			groups.GET("/cache-stats", h.APIKey.GetAvailableGroupCacheStats)
			groups.GET("/rates", h.APIKey.GetUserGroupRates)
		}

		// 使用记录
		usage := authenticated.Group("/usage")
		{
			usage.GET("", h.Usage.List)
			usage.GET("/:id", h.Usage.GetByID)
			usage.GET("/stats", h.Usage.Stats)
			// User dashboard endpoints
			usage.GET("/dashboard/stats", h.Usage.DashboardStats)
			usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
			usage.GET("/dashboard/models", h.Usage.DashboardModels)
			usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		}

		// 公告（用户可见）
		announcements := authenticated.Group("/announcements")
		{
			announcements.GET("", h.Announcement.List)
			announcements.POST("/:id/read", h.Announcement.MarkRead)
		}

		feedbacks := authenticated.Group("/feedbacks")
		{
			feedbacks.POST("", h.Feedback.Create)
			feedbacks.GET("", h.Feedback.List)
			feedbacks.GET("/:id", h.Feedback.GetByID)
			feedbacks.PUT("/:id", h.Feedback.Update)
			feedbacks.POST("/:id/replies", h.Feedback.CreateReply)
			feedbacks.POST("/upload-image", h.Feedback.UploadImage)
		}

		// 卡密兑换
		redeem := authenticated.Group("/redeem")
		{
			redeem.POST("", h.Redeem.Redeem)
			redeem.GET("/history", h.Redeem.GetHistory)
		}

		resources := authenticated.Group("/resources")
		{
			resources.GET("/:tool", h.Resource.ListTool)
			resources.POST("/:tool/download-url/:assetID", h.Resource.CreateDownloadURL)
			resources.GET("/:tool/download/:assetID", h.Resource.DownloadTool)
		}

		// 用户邀请码（所有认证用户可用）
		user.GET("/invite-code", h.Agent.GetMyInviteCode)
		// 用户邀请看板��邀请统计、佣金）
		user.GET("/referral/dashboard", h.User.GetReferralDashboard)

		// 代理商路由（需要 agent 或 admin 角色）
		agent := authenticated.Group("/agent")
		agent.Use(middleware.AgentOrAdmin())
		{
			agent.GET("/invite-code", h.Agent.GetInviteCode)
			agent.GET("/dashboard", h.Agent.GetDashboard)
			agent.GET("/users", h.Agent.GetInvitedUsers)
			agent.GET("/commissions", h.Agent.GetCommissions)
			agent.GET("/payment-profile", h.Agent.GetPaymentProfile)
			agent.PUT("/payment-profile", h.Agent.UpdatePaymentProfile)
			agent.POST("/payment-profile/alipay-qr", h.Agent.UploadPaymentQRCode)
			agent.GET("/payment-profile/alipay-qr", h.Agent.GetPaymentQRCode)
		}

		// 用户订阅
		subscriptions := authenticated.Group("/subscriptions")
		{
			subscriptions.GET("", h.Subscription.List)
			subscriptions.GET("/active", h.Subscription.GetActive)
			subscriptions.GET("/progress", h.Subscription.GetProgress)
			subscriptions.GET("/summary", h.Subscription.GetSummary)
		}

		// 支付
		payment := authenticated.Group("/payment")
		{
			payment.GET("/plans", h.Payment.GetPlans)
			payment.POST("/order", h.Payment.CreateOrder)
			payment.GET("/order/:orderNo/status", h.Payment.QueryOrderStatus)
			payment.GET("/history", h.Payment.GetPaymentHistory)
		}

		// 虎皮椒余额充值
		topup := authenticated.Group("/topup")
		{
			topup.POST("/order", h.Topup.CreateTopupOrder)
			topup.GET("/order/:orderNo/status", h.Topup.QueryTopupOrderStatus)
			topup.GET("/orders", h.Invoice.ListUserTopupOrders)
		}

		invoice := authenticated.Group("/invoice")
		{
			invoice.GET("/profiles", h.Invoice.ListProfiles)
			invoice.POST("/profiles", h.Invoice.CreateProfile)
			invoice.PUT("/profiles/:id", h.Invoice.UpdateProfile)
			invoice.DELETE("/profiles/:id", h.Invoice.DeleteProfile)
			invoice.PUT("/profiles/:id/default", h.Invoice.SetDefaultProfile)
			invoice.POST("/requests", h.Invoice.CreateRequest)
			invoice.GET("/requests", h.Invoice.ListRequests)
		}
	}
}
