package routes

import (
	"net/http"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler"
	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterGatewayRoutes 注册 API 网关路由（Claude/OpenAI/Gemini 兼容）
func RegisterGatewayRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	cfg *config.Config,
) {
	bodyLimit := middleware.RequestBodyLimit(cfg.Gateway.MaxBodySize)
	clientRequestID := middleware.ClientRequestID()
	opsErrorLogger := handler.OpsErrorLoggerMiddleware(opsService)
	endpointNorm := handler.InboundEndpointMiddleware()

	// 未分组 Key 拦截中间件（按协议格式区分错误响应）
	requireGroupAnthropic := middleware.RequireGroupAssignment(settingService, middleware.AnthropicErrorWriter)
	requireGroupOpenAI := middleware.RequireGroupAssignment(settingService, middleware.OpenAIErrorWriter)
	requireGroupGoogle := middleware.RequireGroupAssignment(settingService, middleware.GoogleErrorWriter)

	// Responses subpaths are appended to an upstream URL. Reject malformed
	// segments after authentication but before account selection or forwarding.
	guardResponsesSubpath := func(next gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !service.IsForwardableOpenAIResponsesRequestPath(c) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"type":    "not_found_error",
						"message": "Unsupported responses subpath",
					},
				})
				return
			}
			if service.IsOpenAIResponsesInputTokensRequestPath(c) {
				h.OpenAIGateway.ResponsesInputTokens(c)
				return
			}
			next(c)
		}
	}
	imagesHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokImages(c)
			return
		}
		h.OpenAIGateway.Images(c)
	}
	videoGenerationHandler := func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokVideoGeneration(c)
	}
	videoEditHandler := func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokVideoEdit(c)
	}
	videoExtensionHandler := func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokVideoExtension(c)
	}
	videoStatusHandler := func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokVideoStatus(c)
	}
	videoContentHandler := func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokVideoContent(c)
	}

	// API网关（Claude API兼容）
	gateway := r.Group("/v1")
	gateway.Use(bodyLimit)
	gateway.Use(clientRequestID)
	gateway.Use(opsErrorLogger)
	gateway.Use(endpointNorm)
	gateway.Use(gin.HandlerFunc(apiKeyAuth))
	gateway.Use(requireGroupAnthropic)
	{
		// /v1/messages: auto-route based on group platform
		gateway.POST("/messages", func(c *gin.Context) {
			if getGroupPlatform(c) == service.PlatformOpenAI || getGroupPlatform(c) == service.PlatformGrok {
				h.OpenAIGateway.Messages(c)
				return
			}
			h.Gateway.Messages(c)
		})
		// /v1/messages/count_tokens: Grok estimates locally because xAI has no
		// Anthropic-compatible token-counting endpoint. OpenAI remains unsupported.
		gateway.POST("/messages/count_tokens", func(c *gin.Context) {
			switch getGroupPlatform(c) {
			case service.PlatformGrok:
				h.OpenAIGateway.GrokCountTokens(c)
				return
			case service.PlatformOpenAI:
				c.JSON(http.StatusNotFound, gin.H{
					"type": "error",
					"error": gin.H{
						"type":    "not_found_error",
						"message": "Token counting is not supported for this platform",
					},
				})
				return
			}
			h.Gateway.CountTokens(c)
		})
		gateway.GET("/models", h.Gateway.Models)
		gateway.GET("/usage", h.Gateway.Usage)
		gateway.POST("/live", h.OpenAIGateway.Live)
		gateway.GET("/live/:call_id", h.OpenAIGateway.LiveSideband)
		// OpenAI Responses API
		gateway.POST("/responses", h.OpenAIGateway.Responses)
		gateway.POST("/responses/*subpath", guardResponsesSubpath(h.OpenAIGateway.Responses))
		gateway.GET("/responses", h.OpenAIGateway.ResponsesWebSocket)
		// OpenAI Chat Completions API
		gateway.POST("/chat/completions", h.OpenAIGateway.ChatCompletions)
		// OpenAI Images API
		gateway.POST("/images/generations", imagesHandler)
		gateway.POST("/images/edits", imagesHandler)
		gateway.POST("/videos/generations", videoGenerationHandler)
		gateway.POST("/videos/edits", videoEditHandler)
		gateway.POST("/videos/extensions", videoExtensionHandler)
		gateway.GET("/videos/:request_id", videoStatusHandler)
		gateway.GET("/videos/:request_id/content", videoContentHandler)
	}

	// Gemini 原生 API 兼容层（Gemini SDK/CLI 直连）
	gemini := r.Group("/v1beta")
	gemini.Use(bodyLimit)
	gemini.Use(clientRequestID)
	gemini.Use(opsErrorLogger)
	gemini.Use(endpointNorm)
	gemini.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	gemini.Use(requireGroupGoogle)
	{
		gemini.GET("/models", h.Gateway.GeminiV1BetaListModels)
		gemini.GET("/models/:model", h.Gateway.GeminiV1BetaGetModel)
		// Gin treats ":" as a param marker, but Gemini uses "{model}:{action}" in the same segment.
		gemini.POST("/models/*modelAction", h.Gateway.GeminiV1BetaModels)
	}

	// OpenAI Responses API（不带v1前缀的别名）
	r.POST("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, h.OpenAIGateway.Responses)
	r.POST("/responses/*subpath", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, guardResponsesSubpath(h.OpenAIGateway.Responses))
	r.GET("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, h.OpenAIGateway.ResponsesWebSocket)
	codexDirect := r.Group("/backend-api/codex")
	codexDirect.Use(bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic)
	{
		codexDirect.POST("/realtime/calls", h.OpenAIGateway.Live)
		codexDirect.POST("/responses/input_tokens", h.OpenAIGateway.ResponsesInputTokens)
		codexDirect.GET("/:call_id", h.OpenAIGateway.LiveSideband)
	}
	r.POST("/messages/count_tokens", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokCountTokens(c)
			return
		}
		if getGroupPlatform(c) == service.PlatformOpenAI {
			c.JSON(http.StatusNotFound, gin.H{"type": "error", "error": gin.H{"type": "not_found_error", "message": "Token counting is not supported for this platform"}})
			return
		}
		h.Gateway.CountTokens(c)
	})
	// OpenAI Chat Completions API（不带v1前缀的别名）
	r.POST("/chat/completions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, h.OpenAIGateway.ChatCompletions)
	// OpenAI Images API（不带v1前缀的别名）
	r.POST("/images/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, imagesHandler)
	r.POST("/images/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, imagesHandler)
	r.POST("/videos/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, videoGenerationHandler)
	r.POST("/videos/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, videoEditHandler)
	r.POST("/videos/extensions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, videoExtensionHandler)
	r.GET("/videos/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, videoStatusHandler)
	r.GET("/videos/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, videoContentHandler)

	// Antigravity 模型列表
	r.GET("/antigravity/models", gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, h.Gateway.AntigravityModels)

	gptImage := r.Group("/gpt-image/v1")
	gptImage.Use(bodyLimit)
	gptImage.Use(clientRequestID)
	gptImage.Use(opsErrorLogger)
	gptImage.Use(endpointNorm)
	gptImage.Use(middleware.ForcePlatform(service.PlatformGPTImage))
	gptImage.Use(gin.HandlerFunc(apiKeyAuth))
	gptImage.Use(requireGroupOpenAI)
	{
		gptImage.POST("/images/generations", h.OpenAIGateway.GPTImageGenerate)
		gptImage.GET("/tasks/:task_id", h.OpenAIGateway.GPTImageTask)
		gptImage.GET("/models", h.OpenAIGateway.GPTImageModels)
	}
	r.GET("/gpt-image/media/:task_id/:index", h.OpenAIGateway.GPTImageMedia)
	// Legacy Codex preview URLs remain temporarily readable until their strict
	// 24-hour TTL expires; new responses use local generatedImage delivery and
	// never write this store. Preview URLs are intentionally unauthenticated:
	// Desktop image
	// renderers do not attach the caller's API key to Markdown fetches. Access is
	// bounded by a 256-bit random token and a server-enforced short TTL.
	// Keep the capability token out of URL.Path because the global HTTP access
	// logger records paths. Query strings are deliberately excluded there.
	r.GET("/v1/codex-image/preview", h.OpenAIGateway.CodexImagePreview)
	r.HEAD("/v1/codex-image/preview", h.OpenAIGateway.CodexImagePreview)

	// Antigravity 专用路由（仅使用 antigravity 账户，不混合调度）
	antigravityV1 := r.Group("/antigravity/v1")
	antigravityV1.Use(bodyLimit)
	antigravityV1.Use(clientRequestID)
	antigravityV1.Use(opsErrorLogger)
	antigravityV1.Use(endpointNorm)
	antigravityV1.Use(middleware.ForcePlatform(service.PlatformAntigravity))
	antigravityV1.Use(gin.HandlerFunc(apiKeyAuth))
	antigravityV1.Use(requireGroupAnthropic)
	{
		antigravityV1.POST("/messages", h.Gateway.Messages)
		antigravityV1.POST("/messages/count_tokens", h.Gateway.CountTokens)
		antigravityV1.GET("/models", h.Gateway.AntigravityModels)
		antigravityV1.GET("/usage", h.Gateway.Usage)
	}

	antigravityV1Beta := r.Group("/antigravity/v1beta")
	antigravityV1Beta.Use(bodyLimit)
	antigravityV1Beta.Use(clientRequestID)
	antigravityV1Beta.Use(opsErrorLogger)
	antigravityV1Beta.Use(endpointNorm)
	antigravityV1Beta.Use(middleware.ForcePlatform(service.PlatformAntigravity))
	antigravityV1Beta.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	antigravityV1Beta.Use(requireGroupGoogle)
	{
		antigravityV1Beta.GET("/models", h.Gateway.GeminiV1BetaListModels)
		antigravityV1Beta.GET("/models/:model", h.Gateway.GeminiV1BetaGetModel)
		antigravityV1Beta.POST("/models/*modelAction", h.Gateway.GeminiV1BetaModels)
	}
}

// getGroupPlatform extracts the group platform from the API Key stored in context.
func getGroupPlatform(c *gin.Context) string {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		return ""
	}
	return apiKey.Group.Platform
}
