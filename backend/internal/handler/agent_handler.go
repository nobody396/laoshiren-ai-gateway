package handler

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AgentHandler 代理商相关请求处理
type AgentHandler struct {
	commissionService  *service.CommissionService
	affiliateLinks     *service.AffiliateLinkService
	affiliateAgents    *service.AffiliateAgentService
	affiliateCommunity *service.AffiliateCommunityService
	affiliateWallet    *service.AffiliateWalletService
}

// NewAgentHandler 创建 AgentHandler
func NewAgentHandler(
	commissionService *service.CommissionService,
	affiliateLinks *service.AffiliateLinkService,
	affiliateAgents *service.AffiliateAgentService,
	affiliateCommunity *service.AffiliateCommunityService,
	affiliateWallet *service.AffiliateWalletService,
) *AgentHandler {
	return &AgentHandler{
		commissionService:  commissionService,
		affiliateLinks:     affiliateLinks,
		affiliateAgents:    affiliateAgents,
		affiliateCommunity: affiliateCommunity,
		affiliateWallet:    affiliateWallet,
	}
}

// GetInviteCode 获取或生成当前用户的邀请码
// GET /api/v1/agent/invite-code
func (h *AgentHandler) GetInviteCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	code, err := h.commissionService.GetOrCreateInviteCode(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"invite_code": code})
}

// GetDashboard 获取代理商总览统计
// GET /api/v1/agent/dashboard?start=2026-01-01&end=2026-01-31
func (h *AgentHandler) GetDashboard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	start, end := parseDateRange(c)
	dashboard, err := h.commissionService.GetAgentDashboard(c.Request.Context(), subject.UserID, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dashboard)
}

// GetInvitedUsers 获取代理商邀请的用户列表及消费统计
// GET /api/v1/agent/users?page=1&page_size=20&start=2026-01-01&end=2026-01-31
func (h *AgentHandler) GetInvitedUsers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	params := pagination.PaginationParams{
		Page:     parseIntQuery(c, "page", 1),
		PageSize: parseIntQuery(c, "page_size", 20),
	}
	start, end := parseDateRange(c)

	users, paginationResult, err := h.commissionService.GetAgentInvitedUsers(c.Request.Context(), subject.UserID, params, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":      users,
		"pagination": paginationResult,
	})
}

// GetCommissions 获取代理商的分佣记录明细
// GET /api/v1/agent/commissions?type=consumption&start=2026-01-01&end=2026-01-31
func (h *AgentHandler) GetCommissions(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	params := pagination.PaginationParams{
		Page:     parseIntQuery(c, "page", 1),
		PageSize: parseIntQuery(c, "page_size", 20),
	}
	typeFilter := c.Query("type")
	start, end := parseDateRange(c)

	records, paginationResult, err := h.commissionService.GetAgentCommissions(c.Request.Context(), subject.UserID, params, typeFilter, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":      records,
		"pagination": paginationResult,
	})
}

type createAffiliateLinkRequest struct {
	Name                  string `json:"name" binding:"required"`
	Channel               string `json:"channel"`
	CustomerRebateRateBPS int32  `json:"customer_rebate_rate_bps"`
}

type updateAffiliateLinkRateRequest struct {
	CustomerRebateRateBPS int32 `json:"customer_rebate_rate_bps"`
}

type updateAffiliateLinkStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *AgentHandler) ListAffiliateLinks(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	links, err := h.affiliateLinks.List(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": links})
}

func (h *AgentHandler) CreateAffiliateLink(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createAffiliateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	link, err := h.affiliateLinks.Create(
		c.Request.Context(),
		subject.UserID,
		req.Name,
		req.Channel,
		req.CustomerRebateRateBPS,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, link)
}

func (h *AgentHandler) UpdateAffiliateLinkRate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	linkID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || linkID <= 0 {
		response.BadRequest(c, "Invalid affiliate link id")
		return
	}
	var req updateAffiliateLinkRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	link, err := h.affiliateLinks.UpdateRate(
		c.Request.Context(),
		subject.UserID,
		linkID,
		req.CustomerRebateRateBPS,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, link)
}

func (h *AgentHandler) UpdateAffiliateLinkStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	linkID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || linkID <= 0 {
		response.BadRequest(c, "Invalid affiliate link id")
		return
	}
	var req updateAffiliateLinkStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	link, err := h.affiliateLinks.SetStatus(
		c.Request.Context(),
		subject.UserID,
		linkID,
		req.Status,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, link)
}

func (h *AgentHandler) GetAffiliateQualification(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	qualification, err := h.affiliateAgents.GetQualification(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, qualification)
}

func (h *AgentHandler) ActivateAffiliateAgent(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	activation, err := h.affiliateAgents.Activate(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, activation)
}

func (h *AgentHandler) GetAffiliateCommunity(c *gin.Context) {
	settings, err := h.affiliateCommunity.Get(c.Request.Context(), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if settings.Enabled && settings.HasQRCode {
		settings.QRCodeURL = "/api/v1/agent/affiliate/community/qr"
	}
	response.Success(c, settings)
}

func (h *AgentHandler) GetAffiliateCommunityQRCode(c *gin.Context) {
	file, err := h.affiliateCommunity.GetQRCodeFile(c.Request.Context(), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, no-store")
	c.File(file.Path)
}

type affiliateWalletAmountRequest struct {
	AmountMicros int64 `json:"amount_micros"`
}

func (h *AgentHandler) GetAffiliateWallet(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	wallet, err := h.affiliateWallet.GetWallet(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, wallet)
}

func (h *AgentHandler) ListAffiliateWithdrawals(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.affiliateWallet.ListWithdrawals(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AgentHandler) RequestAffiliateWithdrawal(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req affiliateWalletAmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.affiliateWallet.RequestWithdrawal(
		c.Request.Context(),
		subject.UserID,
		req.AmountMicros,
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *AgentHandler) ConvertAffiliateCommission(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req affiliateWalletAmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.affiliateWallet.Convert(
		c.Request.Context(),
		subject.UserID,
		req.AmountMicros,
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *AgentHandler) ListAffiliateNotices(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.affiliateWallet.ListNotices(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AgentHandler) ReadAffiliateNotice(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	noticeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || noticeID <= 0 {
		response.BadRequest(c, "Invalid notice id")
		return
	}
	if err := h.affiliateWallet.MarkNoticeRead(c.Request.Context(), subject.UserID, noticeID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"read": true})
}

type updateAgentPaymentProfileRequest struct {
	AlipayRealName string `json:"alipay_real_name"`
	AlipayAccount  string `json:"alipay_account"`
	ContactPhone   string `json:"contact_phone"`
	PaymentNote    string `json:"payment_note"`
}

func (h *AgentHandler) GetPaymentProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	profile, err := h.commissionService.GetAgentPaymentProfile(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	profile.AlipayQRCodeURL = "/api/v1/agent/payment-profile/alipay-qr"
	response.Success(c, profile)
}

func (h *AgentHandler) UpdatePaymentProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req updateAgentPaymentProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.commissionService.UpdateAgentPaymentProfile(c.Request.Context(), &service.AgentPaymentProfile{
		AgentID:        subject.UserID,
		AlipayRealName: req.AlipayRealName,
		AlipayAccount:  req.AlipayAccount,
		ContactPhone:   req.ContactPhone,
		PaymentNote:    req.PaymentNote,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	profile.AlipayQRCodeURL = "/api/v1/agent/payment-profile/alipay-qr"
	response.Success(c, profile)
}

func (h *AgentHandler) UploadPaymentQRCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	profile, err := h.uploadPaymentQRCode(c, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	profile.AlipayQRCodeURL = "/api/v1/agent/payment-profile/alipay-qr"
	response.Created(c, profile)
}

func (h *AgentHandler) GetPaymentQRCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.servePaymentQRCode(c, subject.UserID)
}

// GetMyInviteCode 普通用户获取自己的邀请码
// GET /api/v1/user/invite-code
func (h *AgentHandler) GetMyInviteCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	code, err := h.commissionService.GetOrCreateInviteCode(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"invite_code": code})
}

func (h *AgentHandler) uploadPaymentQRCode(c *gin.Context, agentID int64) (*service.AgentPaymentProfile, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, infraerrors.BadRequest("PAYMENT_QR_REQUIRED", "file is required")
	}
	if fileHeader.Size <= 0 {
		return nil, infraerrors.BadRequest("PAYMENT_QR_EMPTY", "empty file is not allowed")
	}
	if fileHeader.Size > 5<<20 {
		return nil, infraerrors.BadRequest("PAYMENT_QR_TOO_LARGE", "file size must be at most 5MB")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	header := make([]byte, 512)
	n, readErr := file.Read(header)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, http.ErrBodyReadAfterClose) {
		return nil, readErr
	}
	contentType := http.DetectContentType(header[:n])
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return nil, infraerrors.BadRequest("PAYMENT_QR_UNSUPPORTED", "only jpg/png/webp images are supported")
	}
	if seeker, ok := file.(interface {
		Seek(offset int64, whence int) (int64, error)
	}); ok {
		if _, err := seeker.Seek(0, 0); err != nil {
			return nil, err
		}
	}
	return h.commissionService.UploadAgentPaymentQRCode(c.Request.Context(), agentID, service.AgentPaymentQRCodeUpload{
		Filename:    filepath.Base(fileHeader.Filename),
		ContentType: contentType,
		Size:        fileHeader.Size,
		Body:        file,
	})
}

func (h *AgentHandler) servePaymentQRCode(c *gin.Context, agentID int64) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	file, err := h.commissionService.GetAgentPaymentQRCodeFile(c.Request.Context(), agentID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.commissionService.RecordAgentPaymentQRCodeAccess(
		c.Request.Context(),
		agentID,
		subject.UserID,
		"agent_self",
	); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.File(file.Path)
}

// parseDateRange 从 query 参数解析 start/end 时间（格式：2006-01-02）
func parseDateRange(c *gin.Context) (*time.Time, *time.Time) {
	var start, end *time.Time
	if s := c.Query("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			start = &t
		}
	}
	if e := c.Query("end"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			// end of day
			t = t.Add(24*time.Hour - time.Second)
			end = &t
		}
	}
	return start, end
}

// parseIntQuery 从 query 参数解析整数，fallback 到 defaultVal
func parseIntQuery(c *gin.Context, key string, defaultVal int) int {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}

// ValidateReferralCode 验证邀请码是否有效（公开接口）
// GET /api/v1/validate-referral-code?code=XXXX
func (h *AgentHandler) ValidateReferralCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Success(c, gin.H{"valid": false})
		return
	}

	inviter, err := h.commissionService.ValidateAndGetInviter(c.Request.Context(), code)
	if err != nil {
		response.Success(c, gin.H{"valid": false})
		return
	}

	response.Success(c, gin.H{"valid": inviter != nil})
}
