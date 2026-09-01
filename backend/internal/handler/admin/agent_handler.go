package admin

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	apptimezone "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/timezone"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	commissionService  *service.CommissionService
	affiliateProgram   *service.AffiliateProgramService
	affiliateCommunity *service.AffiliateCommunityService
	affiliateWallet    *service.AffiliateWalletService
	affiliateRisk      *service.AffiliateRiskService
	selfCommission     *service.AffiliateSelfCommissionPolicyService
	affiliateAgents    *service.AffiliateAgentService
}

func NewAgentHandler(
	commissionService *service.CommissionService,
	affiliateProgram *service.AffiliateProgramService,
	affiliateCommunity *service.AffiliateCommunityService,
	affiliateWallet *service.AffiliateWalletService,
	affiliateRisk *service.AffiliateRiskService,
	selfCommission *service.AffiliateSelfCommissionPolicyService,
	affiliateAgents *service.AffiliateAgentService,
) *AgentHandler {
	return &AgentHandler{
		commissionService:  commissionService,
		affiliateProgram:   affiliateProgram,
		affiliateCommunity: affiliateCommunity,
		affiliateWallet:    affiliateWallet,
		affiliateRisk:      affiliateRisk,
		selfCommission:     selfCommission,
		affiliateAgents:    affiliateAgents,
	}
}

type updateCommissionRatesRequest struct {
	ConsumptionRate           float64                       `json:"consumption_rate"`
	FirstRechargeInviteeRate  float64                       `json:"first_recharge_invitee_rate"`
	FirstRechargeReferralRate float64                       `json:"first_recharge_referral_rate"`
	InviteActivity            *service.InviteActivityConfig `json:"invite_activity"`
}

type updateAgentRateRequest struct {
	ConsumptionRate float64 `json:"consumption_rate"`
	Enabled         bool    `json:"enabled"`
}

type updateAgentLevelRulesRequest struct {
	Rules []service.AgentLevelRule `json:"rules"`
}

type createSettlementRequest struct {
	Amount           float64 `json:"amount"`
	Note             string  `json:"note"`
	PaymentReference string  `json:"payment_reference"`
}

type updateAgentSettlementSettingsRequest struct {
	MinimumAmount float64 `json:"minimum_amount"`
}

type reviewAgentPaymentProfileRequest struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note"`
}

type updateAffiliateCommunityRequest struct {
	Enabled  bool   `json:"enabled"`
	Title    string `json:"title" binding:"required"`
	Message  string `json:"message"`
	Revision int64  `json:"revision" binding:"required"`
}

type completeAffiliateWithdrawalRequest struct {
	PaymentReference string `json:"payment_reference"`
}

type failAffiliateWithdrawalRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type updateAffiliateRiskRequest struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

type updateAffiliateSelfCommissionPolicyRequest struct {
	Enabled          *bool  `json:"enabled" binding:"required"`
	ExpectedRevision *int64 `json:"expected_revision" binding:"required"`
	Reason           string `json:"reason" binding:"required"`
}

type reverseAffiliatePerformanceRequest struct {
	EventID int64  `json:"event_id" binding:"required"`
	Reason  string `json:"reason" binding:"required"`
}

type purchaseWithAffiliateCommissionRequest struct {
	AmountMicros      int64  `json:"amount_micros" binding:"required,gt=0"`
	PurchaseKind      string `json:"purchase_kind" binding:"required,oneof=monthly_card"`
	ProductCode       string `json:"product_code" binding:"required"`
	ExternalReference string `json:"external_reference" binding:"required"`
	Note              string `json:"note"`
}

type refundAffiliateCommissionPurchaseRequest struct {
	RateBPS                      int32  `json:"rate_bps" binding:"required,gt=0,lt=10000"`
	ExpectedOriginalAmountMicros int64  `json:"expected_original_amount_micros" binding:"required,gt=0"`
	ExpectedRefundAmountMicros   int64  `json:"expected_refund_amount_micros" binding:"required,gt=0"`
	Note                         string `json:"note"`
}

type reviewAffiliateApplicationRequest struct {
	Approve bool   `json:"approve"`
	Note    string `json:"note"`
}

type bindAgentUserRequest struct {
	Email          string `json:"email" binding:"required,email"`
	OverwriteAgent bool   `json:"overwrite_agent"`
}

func (h *AgentHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	start, end := parseAgentDateRange(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}
	items, result, err := h.commissionService.ListAdminAgents(c.Request.Context(), params, service.AdminAgentListFilters{
		Search:           c.Query("search"),
		SettlementStatus: c.Query("settlement_status"),
		Start:            start,
		End:              end,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) ListSettlementCandidates(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	start, end := parseAgentDateRange(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}
	items, result, err := h.commissionService.ListAdminAgentSettlementCandidates(c.Request.Context(), params, service.AdminAgentListFilters{
		Search:           c.Query("search"),
		SettlementStatus: c.Query("settlement_status"),
		Start:            start,
		End:              end,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) Get(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	start, end := parseAgentDateRange(c)
	item, err := h.commissionService.GetAdminAgent(c.Request.Context(), agentID, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AgentHandler) ListUsers(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	start, end := parseAgentDateRange(c)
	items, result, err := h.commissionService.ListAdminAgentUsers(c.Request.Context(), agentID, pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) BindUser(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	var req bindAgentUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.commissionService.BindUserToAgent(c.Request.Context(), agentID, service.BindAgentUserInput{
		Email:          req.Email,
		OverwriteAgent: req.OverwriteAgent,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) ListCommissions(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	start, end := parseAgentDateRange(c)
	items, result, err := h.commissionService.ListAdminAgentCommissions(c.Request.Context(), agentID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, c.Query("type"), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) ListSettlements(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.commissionService.ListAgentSettlements(c.Request.Context(), agentID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) CreateSettlement(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createSettlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	settlement, err := h.commissionService.CreateAgentSettlement(c.Request.Context(), agentID, subject.UserID, req.Amount, req.Note, req.PaymentReference)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settlement)
}

func (h *AgentHandler) GetPaymentProfile(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	profile, err := h.commissionService.GetAgentPaymentProfile(c.Request.Context(), agentID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	profile.AlipayQRCodeURL = "/api/v1/admin/agents/" + strconv.FormatInt(agentID, 10) + "/payment-profile/alipay-qr"
	response.Success(c, profile)
}

func (h *AgentHandler) GetPaymentQRCode(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
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
		"admin_profile",
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

func (h *AgentHandler) ReviewPaymentProfile(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req reviewAgentPaymentProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.commissionService.ReviewAgentPaymentProfile(
		c.Request.Context(),
		agentID,
		subject.UserID,
		req.Status,
		req.Note,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	profile.AlipayQRCodeURL = "/api/v1/admin/agents/" + strconv.FormatInt(agentID, 10) + "/payment-profile/alipay-qr"
	response.Success(c, profile)
}

func (h *AgentHandler) ListPendingPaymentProfiles(c *gin.Context) {
	items, err := h.commissionService.ListPendingAgentPaymentProfiles(
		c.Request.Context(),
		parsePositiveInt(c.Query("limit"), 100),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	for index := range items {
		items[index].AlipayQRCodeURL = "/api/v1/admin/agents/" +
			strconv.FormatInt(items[index].AgentID, 10) +
			"/payment-profile/alipay-qr"
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AgentHandler) GetAffiliateProgram(c *gin.Context) {
	settings, err := h.affiliateProgram.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AgentHandler) GetAffiliateCommercialPolicy(c *gin.Context) {
	settings, err := h.affiliateProgram.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.BuildAffiliateCommercialPolicyFromSettings(*settings))
}

func (h *AgentHandler) ListAffiliateRiskPrincipals(c *gin.Context) {
	items, err := h.affiliateRisk.List(
		c.Request.Context(),
		parsePositiveInt(c.Query("limit"), 100),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AgentHandler) ListAffiliateApplications(c *gin.Context) {
	items, err := h.affiliateAgents.ListApplications(
		c.Request.Context(),
		c.DefaultQuery("status", "pending_review"),
		parsePositiveInt(c.Query("limit"), 100),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AgentHandler) ListAffiliateQualifiedCandidates(c *gin.Context) {
	items, err := h.affiliateAgents.ListQualifiedCandidates(
		c.Request.Context(),
		parsePositiveInt(c.Query("limit"), 100),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AgentHandler) GetAffiliateOperationsSummary(c *gin.Context) {
	item, err := h.affiliateAgents.GetOperationsSummary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AgentHandler) ListAffiliatePartnerPerformance(c *gin.Context) {
	items, err := h.affiliateAgents.ListPartnerPerformance(
		c.Request.Context(),
		parsePositiveInt(c.Query("limit"), 100),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AgentHandler) GetAffiliatePartnerPerformance(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	start, end := parseAgentDateRange(c)
	item, err := h.affiliateAgents.GetPartnerPerformance(c.Request.Context(), agentID, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AgentHandler) ReviewAffiliateApplication(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	applicationID, err := strconv.ParseInt(c.Param("application_id"), 10, 64)
	if err != nil || applicationID <= 0 {
		response.BadRequest(c, "Invalid application id")
		return
	}
	var req reviewAffiliateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.affiliateAgents.ReviewApplication(
		c.Request.Context(),
		applicationID,
		req.Approve,
		req.Note,
		subject.UserID,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) UpdateAffiliateRisk(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req updateAffiliateRiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.affiliateRisk.SetAgentRisk(
		c.Request.Context(),
		agentID,
		req.Status,
		req.Reason,
		subject.UserID,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) UpdateAffiliateSelfCommissionPolicy(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req updateAffiliateSelfCommissionPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.selfCommission.Update(
		c.Request.Context(),
		agentID,
		*req.Enabled,
		*req.ExpectedRevision,
		req.Reason,
		subject.UserID,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) ReverseAffiliatePerformance(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req reverseAffiliatePerformanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.affiliateRisk.ReversePerformanceEvent(
		c.Request.Context(),
		req.EventID,
		req.Reason,
		subject.UserID,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) UpdateAffiliateProgram(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var next service.AffiliateProgramSettings
	if err := c.ShouldBindJSON(&next); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	current, err := h.affiliateProgram.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if next.Mode == service.AffiliateProgramModeLive && current.StartedAt == nil {
		startedAt := time.Now().UTC()
		next.StartedAt = &startedAt
	}
	settings, err := h.affiliateProgram.UpdateSettings(
		c.Request.Context(),
		next,
		next.Revision,
		subject.UserID,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AgentHandler) GetAffiliateCommunity(c *gin.Context) {
	settings, err := h.affiliateCommunity.Get(c.Request.Context(), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if settings.HasQRCode {
		settings.QRCodeURL = "/api/v1/admin/agents/affiliate-community/qr"
	}
	response.Success(c, settings)
}

func (h *AgentHandler) UpdateAffiliateCommunity(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req updateAffiliateCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	settings, err := h.affiliateCommunity.Update(
		c.Request.Context(),
		req.Title,
		req.Message,
		req.Enabled,
		subject.UserID,
		req.Revision,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if settings.HasQRCode {
		settings.QRCodeURL = "/api/v1/admin/agents/affiliate-community/qr"
	}
	response.Success(c, settings)
}

func (h *AgentHandler) UploadAffiliateCommunityQRCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	if header.Size <= 0 || header.Size > 5<<20 {
		response.BadRequest(c, "file size must be between 1 byte and 5MB")
		return
	}
	file, err := header.Open()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = file.Close() }()
	sniff := make([]byte, 512)
	n, readErr := file.Read(sniff)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		response.ErrorFrom(c, readErr)
		return
	}
	contentType := http.DetectContentType(sniff[:n])
	if seeker, ok := file.(interface {
		Seek(offset int64, whence int) (int64, error)
	}); ok {
		if _, err := seeker.Seek(0, 0); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	settings, err := h.affiliateCommunity.UploadQRCode(
		c.Request.Context(),
		subject.UserID,
		service.AffiliateCommunityQRCodeUpload{
			Filename:    filepath.Base(header.Filename),
			ContentType: contentType,
			Size:        header.Size,
			Body:        file,
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	settings.QRCodeURL = "/api/v1/admin/agents/affiliate-community/qr"
	response.Created(c, settings)
}

func (h *AgentHandler) GetAffiliateCommunityQRCode(c *gin.Context) {
	file, err := h.affiliateCommunity.GetQRCodeFile(c.Request.Context(), false)
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

func (h *AgentHandler) ListAffiliateWithdrawals(c *gin.Context) {
	items, err := h.affiliateWallet.ListAdminWithdrawals(
		c.Request.Context(),
		c.DefaultQuery("status", "processing"),
		parsePositiveInt(c.Query("limit"), 100),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AgentHandler) CompleteAffiliateWithdrawal(c *gin.Context) {
	withdrawalID, ok := parseAffiliateWithdrawalID(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req completeAffiliateWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.affiliateWallet.CompleteWithdrawal(
		c.Request.Context(),
		withdrawalID,
		subject.UserID,
		req.PaymentReference,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AgentHandler) PurchaseWithAffiliateCommission(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req purchaseWithAffiliateCommissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.affiliateWallet.Purchase(
		c.Request.Context(),
		agentID,
		req.AmountMicros,
		subject.UserID,
		req.PurchaseKind,
		req.ProductCode,
		req.ExternalReference,
		req.Note,
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *AgentHandler) RefundAffiliateCommissionPurchase(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	purchaseEntryID, err := strconv.ParseInt(c.Param("purchase_id"), 10, 64)
	if err != nil || purchaseEntryID <= 0 {
		response.BadRequest(c, "Invalid purchase ID")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req refundAffiliateCommissionPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.affiliateWallet.RefundPurchase(
		c.Request.Context(),
		agentID,
		purchaseEntryID,
		req.ExpectedOriginalAmountMicros,
		req.ExpectedRefundAmountMicros,
		subject.UserID,
		req.RateBPS,
		req.Note,
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *AgentHandler) FailAffiliateWithdrawal(c *gin.Context) {
	withdrawalID, ok := parseAffiliateWithdrawalID(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req failAffiliateWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.affiliateWallet.FailWithdrawal(
		c.Request.Context(),
		withdrawalID,
		subject.UserID,
		req.Reason,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AgentHandler) GetAffiliateWithdrawalQRCode(c *gin.Context) {
	withdrawalID, ok := parseAffiliateWithdrawalID(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	file, err := h.affiliateWallet.GetWithdrawalQRCodeFile(
		c.Request.Context(),
		withdrawalID,
		subject.UserID,
	)
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

func parseAffiliateWithdrawalID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("withdrawal_id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid withdrawal id")
		return 0, false
	}
	return id, true
}

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func (h *AgentHandler) GetSettlementSettings(c *gin.Context) {
	settings, err := h.commissionService.GetAgentSettlementSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AgentHandler) UpdateSettlementSettings(c *gin.Context) {
	var req updateAgentSettlementSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	settings, err := h.commissionService.UpdateAgentSettlementSettings(c.Request.Context(), &service.AgentSettlementSettings{
		MinimumAmount: req.MinimumAmount,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AgentHandler) GetRates(c *gin.Context) {
	rates, err := h.commissionService.GetCommissionRates(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rates)
}

func (h *AgentHandler) UpdateRates(c *gin.Context) {
	var req updateCommissionRatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rates, err := h.commissionService.UpdateCommissionRates(c.Request.Context(), &service.CommissionRates{
		ConsumptionRate:           req.ConsumptionRate,
		FirstRechargeInviteeRate:  req.FirstRechargeInviteeRate,
		FirstRechargeReferralRate: req.FirstRechargeReferralRate,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.InviteActivity != nil {
		activity, activityErr := h.commissionService.UpdateInviteActivityConfig(c.Request.Context(), req.InviteActivity)
		if activityErr != nil {
			response.ErrorFrom(c, activityErr)
			return
		}
		rates.InviteActivity = activity
	}
	response.Success(c, rates)
}

func (h *AgentHandler) GetLevelRules(c *gin.Context) {
	rules, err := h.commissionService.GetAgentLevelRules(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

func (h *AgentHandler) UpdateLevelRules(c *gin.Context) {
	var req updateAgentLevelRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rules, err := h.commissionService.UpdateAgentLevelRules(c.Request.Context(), req.Rules)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

func (h *AgentHandler) RunLevelEvaluations(c *gin.Context) {
	result, err := h.commissionService.RunAllAgentLevelEvaluations(c.Request.Context(), apptimezone.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) RunAgentLevelEvaluation(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	result, err := h.commissionService.RunAgentLevelEvaluation(c.Request.Context(), agentID, apptimezone.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) GetAgentRate(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	config, err := h.commissionService.GetAgentRateConfig(c.Request.Context(), agentID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *AgentHandler) UpdateAgentRate(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	var req updateAgentRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	config, err := h.commissionService.UpdateAgentRateConfig(c.Request.Context(), &service.AgentRateConfig{
		AgentID:         agentID,
		ConsumptionRate: req.ConsumptionRate,
		Enabled:         req.Enabled,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func parseAgentIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid agent ID")
		return 0, false
	}
	return id, true
}

func parseAgentDateRange(c *gin.Context) (*time.Time, *time.Time) {
	var start, end *time.Time
	if raw := c.Query("start"); raw != "" {
		if parsed, err := time.Parse("2006-01-02", raw); err == nil {
			start = &parsed
		}
	}
	if raw := c.Query("end"); raw != "" {
		if parsed, err := time.Parse("2006-01-02", raw); err == nil {
			parsed = parsed.Add(24*time.Hour - time.Second)
			end = &parsed
		}
	}
	return start, end
}
