package handler

import (
	"net/http"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// TopupHandler handles balance top-up via Xunhupay
type TopupHandler struct {
	topupService *service.TopupService
}

// NewTopupHandler creates a new TopupHandler
func NewTopupHandler(topupService *service.TopupService) *TopupHandler {
	return &TopupHandler{topupService: topupService}
}

// CreateTopupOrderRequest represents the request body for creating a topup order
type CreateTopupOrderRequest struct {
	// 充值金额，单位：分（CNY）。例如 2000 = ¥20
	AmountCNYFen int    `json:"amount_cny_fen" binding:"required,min=2000"`
	PayType      string `json:"pay_type" binding:"required,oneof=alipay wechat"`
}

// CreateTopupOrder 创建充值订单
// POST /api/v1/topup/order
func (h *TopupHandler) CreateTopupOrder(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req CreateTopupOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	orderNo, qrCodeURL, err := h.topupService.CreateTopupOrder(
		c.Request.Context(),
		subject.UserID,
		req.AmountCNYFen,
		req.PayType,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"order_no":       orderNo,
		"qr_code_url":    qrCodeURL,
		"amount_cny_fen": req.AmountCNYFen,
		"pay_type":       req.PayType,
	})
}

// QueryTopupOrderStatus 查询充值订单状态
// GET /api/v1/topup/order/:orderNo/status
func (h *TopupHandler) QueryTopupOrderStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	orderNo := c.Param("orderNo")
	if orderNo == "" {
		response.BadRequest(c, "Missing order number")
		return
	}

	order, err := h.topupService.QueryOrderStatus(c.Request.Context(), orderNo, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"order_no":       order.OrderNo,
		"status":         order.Status,
		"amount_cny_fen": order.AmountCNYFen,
		"pay_type":       order.PayType,
		"qr_code_url":    order.QRCodeURL,
	})
}

// HandleTopupNotify 处理虎皮椒异步回调
// POST /api/v1/topup/notify
// 无 JWT 鉴权，靠签名验证安全性
func (h *TopupHandler) HandleTopupNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	if err := h.topupService.HandleNotify(c.Request.Context(), c.Request.Form); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}
