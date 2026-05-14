package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/httputil"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ip"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GPTImageGenerate handles GPT-Image API requests.
// POST /gpt-image/v1/images/generations
func (h *OpenAIGatewayHandler) GPTImageGenerate(c *gin.Context) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	requestStart := time.Now()

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if apiKey.Group == nil || apiKey.Group.Platform != service.PlatformGPTImage {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "This API key is not assigned to a GPT-Image group")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}

	reqLog := requestLogger(
		c,
		"handler.openai_gateway.gpt_image",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	parsed, err := h.gatewayService.ParseGPTImageRequest(body)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	reqLog = reqLog.With(zap.String("model", parsed.Model))

	setOpsRequestContext(c, parsed.Model, false, parsed.Body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(false, false)))
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		reqLog.Info("gpt_image.billing_eligibility_check_failed", zap.Error(err))
		status, code, message := billingErrorDetails(err)
		h.handleStreamingAwareError(c, status, code, message, streamStarted)
		return
	}

	sessionHash := h.gatewayService.GenerateExplicitSessionHash(c, parsed.Body)
	failedAccountIDs := make(map[int64]struct{})
	var lastFailoverErr *service.UpstreamFailoverError

	for switchCount := 0; ; switchCount++ {
		selection, err := h.gatewayService.SelectAccountWithSchedulerForGPTImage(
			c.Request.Context(),
			apiKey.GroupID,
			sessionHash,
			parsed.Model,
			failedAccountIDs,
		)
		if err != nil {
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, streamStarted)
			} else {
				h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available compatible accounts", streamStarted)
			}
			return
		}
		if selection == nil || selection.Account == nil {
			h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available compatible accounts", streamStarted)
			return
		}

		account := selection.Account
		sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc, acquired := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, false, &streamStarted, reqLog)
		if !acquired {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		result, err := h.gatewayService.ForwardGPTImage(c.Request.Context(), c, account, parsed)
		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}

		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)

		if err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= h.maxAccountSwitches {
					h.handleFailoverExhausted(c, failoverErr, streamStarted)
					return
				}
				continue
			}
			_ = h.ensureForwardErrorResponse(c, streamStarted)
			reqLog.Error("gpt_image.forward_failed", zap.Error(err), zap.Int64("account_id", account.ID))
			return
		}

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(parsed.Body)

		for _, taskID := range result.GPTImageTaskIDs {
			h.gatewayService.RegisterGPTImagePendingTask(&service.GPTImagePendingTaskUsage{
				TaskID:             taskID,
				APIKeyID:           apiKey.ID,
				UserID:             subject.UserID,
				Account:            *account,
				Model:              result.Model,
				UpstreamModel:      result.UpstreamModel,
				Resolution:         result.ImageSize,
				Size:               parsed.Size,
				ImageCount:         1,
				InboundEndpoint:    GetInboundEndpoint(c),
				UpstreamEndpoint:   GetUpstreamEndpoint(c, service.PlatformGPTImage),
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				RequestPayloadHash: requestPayloadHash,
			})
		}
		if result.ImageCount <= 0 {
			return
		}

		h.submitUsageRecordTask(func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:             result,
				APIKey:             apiKey,
				User:               apiKey.User,
				Account:            account,
				Subscription:       subscription,
				InboundEndpoint:    GetInboundEndpoint(c),
				UpstreamEndpoint:   GetUpstreamEndpoint(c, service.PlatformGPTImage),
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				RequestPayloadHash: requestPayloadHash,
				APIKeyService:      h.apiKeyService,
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.gpt_image"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.String("model", parsed.Model),
					zap.Int64("account_id", account.ID),
				).Error("gpt_image.record_usage_failed", zap.Error(err))
			}
		})
		return
	}
}

// GPTImageTask proxies APIMart-style async task status and bills only after completion.
// GET /gpt-image/v1/tasks/:task_id
func (h *OpenAIGatewayHandler) GPTImageTask(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if apiKey.Group == nil || apiKey.Group.Platform != service.PlatformGPTImage {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "This API key is not assigned to a GPT-Image group")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}

	taskID := c.Param("task_id")
	pending, ok := h.gatewayService.GetGPTImagePendingTask(taskID)
	if !ok || pending.APIKeyID != apiKey.ID || pending.UserID != subject.UserID {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "GPT-Image task not found")
		return
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)

	result, err := h.gatewayService.ForwardGPTImageTask(c.Request.Context(), c, &pending.Account, taskID)
	if err != nil {
		_ = h.ensureForwardErrorResponse(c, false)
		return
	}
	result.Model = pending.Model
	result.UpstreamModel = pending.UpstreamModel
	result.ImageSize = pending.Resolution
	if result.ImageCount <= 0 {
		c.Data(result.ResponseStatus, result.ResponseType, result.ResponseBody)
		return
	}

	rewrittenBody, imageCount, err := h.gatewayService.StoreGPTImageTaskResult(
		c.Request.Context(),
		taskID,
		result.ResponseBody,
		resolveGPTImageMediaBaseURL(c),
	)
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "storage_error", err.Error())
		return
	}
	c.Data(result.ResponseStatus, result.ResponseType, rewrittenBody)
	result.ImageCount = imageCount

	claimed, ok := h.gatewayService.ClaimGPTImageTaskBilling(taskID)
	if !ok {
		return
	}
	result.Model = claimed.Model
	result.UpstreamModel = claimed.UpstreamModel
	result.ImageSize = claimed.Resolution
	result.RequestID = "gpt-image-task:" + taskID

	h.submitUsageRecordTask(func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:             result,
			APIKey:             apiKey,
			User:               apiKey.User,
			Account:            &claimed.Account,
			Subscription:       subscription,
			InboundEndpoint:    claimed.InboundEndpoint,
			UpstreamEndpoint:   claimed.UpstreamEndpoint,
			UserAgent:          claimed.UserAgent,
			IPAddress:          claimed.IPAddress,
			RequestPayloadHash: claimed.RequestPayloadHash,
			APIKeyService:      h.apiKeyService,
		}); err != nil {
			h.gatewayService.ReleaseGPTImageTaskBillingClaim(taskID)
			logger.L().With(
				zap.String("component", "handler.openai_gateway.gpt_image"),
				zap.Int64("user_id", subject.UserID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.String("task_id", taskID),
			).Error("gpt_image.task_record_usage_failed", zap.Error(err))
			return
		}
		h.gatewayService.MarkGPTImageTaskBilled(taskID)
	})
}

// GPTImageMedia proxies generated images from private S3 without exposing upstream URLs.
// GET /gpt-image/media/:task_id/:index?token=...
func (h *OpenAIGatewayHandler) GPTImageMedia(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	index, err := strconv.Atoi(strings.TrimSpace(c.Param("index")))
	if err != nil || index < 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Invalid media index")
		return
	}
	token := strings.TrimSpace(c.Query("token"))
	body, contentType, size, err := h.gatewayService.OpenGPTImageMedia(c.Request.Context(), taskID, index, token)
	if err != nil {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "GPT-Image media not found")
		return
	}
	defer func() { _ = body.Close() }()

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, max-age=3600")
	c.Header("X-Content-Type-Options", "nosniff")
	if size > 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, body)
}

func resolveGPTImageMediaBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request != nil && c.Request.TLS != nil {
		scheme = "https"
	}
	if xfProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xfProto != "" {
		scheme = strings.TrimSpace(strings.Split(xfProto, ",")[0])
	}
	host := ""
	if c.Request != nil {
		host = strings.TrimSpace(c.Request.Host)
	}
	if xfHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); xfHost != "" {
		host = strings.TrimSpace(strings.Split(xfHost, ",")[0])
	}
	if host == "" {
		return "/gpt-image/media"
	}
	return scheme + "://" + host + "/gpt-image/media"
}

func (h *OpenAIGatewayHandler) GPTImageModels(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if apiKey.Group == nil || apiKey.Group.Platform != service.PlatformGPTImage {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "This API key is not assigned to a GPT-Image group")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data": []gin.H{
			{
				"id":     "gpt-image-2",
				"object": "model",
			},
		},
	})
}
