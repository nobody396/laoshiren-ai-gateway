package handler

import (
	"net/http"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PaymentHandler handles payment-related HTTP requests
type PaymentHandler struct {
	paymentService *service.PaymentService
}

// NewPaymentHandler creates a new PaymentHandler
func NewPaymentHandler(paymentService *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
	}
}

// CreateOrderRequest represents the order creation request
type CreateOrderRequest struct {
	PlanID string `json:"plan_id" binding:"required"`
}

// GetPlans returns available subscription plans
// GET /api/v1/payment/plans
func (h *PaymentHandler) GetPlans(c *gin.Context) {
	plans, err := h.paymentService.GetPlans(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}

// CreateOrder creates a payment order and returns Alipay QR code URL
// POST /api/v1/payment/order
func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	orderNo, qrCodeURL, err := h.paymentService.CreateOrder(
		c.Request.Context(),
		subject.UserID,
		req.PlanID,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"order_no":    orderNo,
		"qr_code_url": qrCodeURL,
	})
}

// QueryOrderStatus polls the status of a payment order
// GET /api/v1/payment/order/:orderNo/status
func (h *PaymentHandler) QueryOrderStatus(c *gin.Context) {
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

	order, err := h.paymentService.QueryOrderStatus(c.Request.Context(), orderNo, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"plan_id":  order.PlanID,
		"group_id": order.GroupID,
	})
}

// HandleNotify handles Alipay async notification callback
// POST /api/v1/payment/notify
func (h *PaymentHandler) HandleNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	if err := h.paymentService.HandleNotify(c.Request.Context(), c.Request.Form); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	// Alipay requires plain text "success" response
	c.String(http.StatusOK, "success")
}

// GetPaymentHistory returns the user's payment history
// GET /api/v1/payment/history
func (h *PaymentHandler) GetPaymentHistory(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	orders, err := h.paymentService.GetUserPaymentHistory(c.Request.Context(), subject.UserID, 50)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, orders)
}
