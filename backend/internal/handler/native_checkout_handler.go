package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type NativeCheckoutHandler struct {
	service *service.NativeCheckoutService
}

func NewNativeCheckoutHandler(nativeCheckoutService *service.NativeCheckoutService) *NativeCheckoutHandler {
	return &NativeCheckoutHandler{service: nativeCheckoutService}
}

type createNativeCheckoutOrderRequest struct {
	OfferCode string `json:"offer_code" binding:"required,max=64"`
	// PayType is only meaningful for easypay offers (alipay/wechat, default
	// alipay); providers that choose their own channel ignore it.
	PayType string `json:"pay_type" binding:"omitempty,oneof=alipay wechat"`
}

type nativeCheckoutOrderResponse struct {
	OrderNo             string    `json:"order_no"`
	Status              string    `json:"status"`
	Provider            string    `json:"provider"`
	PayAmountCNYFen     int64     `json:"pay_amount_cny_fen"`
	BenefitAmountCNYFen int64     `json:"benefit_amount_cny_fen"`
	PaymentURL          string    `json:"payment_url,omitempty"`
	PaymentMethod       string    `json:"payment_method,omitempty"`
	DirectQRURL         string    `json:"direct_qr_url,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type nativeCheckoutOfferResponse struct {
	Code                string                       `json:"code"`
	Provider            string                       `json:"provider"`
	Name                string                       `json:"name"`
	Description         string                       `json:"description"`
	ProductKind         string                       `json:"product_kind"`
	PayAmountCNYFen     int64                        `json:"pay_amount_cny_fen"`
	BenefitAmountCNYFen int64                        `json:"benefit_amount_cny_fen"`
	OncePerUser         bool                         `json:"once_per_user"`
	Claimed             bool                         `json:"claimed"`
	Order               *nativeCheckoutOrderResponse `json:"order,omitempty"`
}

type nativeCheckoutManualOfferStatusResponse struct {
	Code        string `json:"code"`
	Claimed     bool   `json:"claimed"`
	PurchaseURL string `json:"purchase_url,omitempty"`
}

func (h *NativeCheckoutHandler) ListOffers(c *gin.Context) {
	userID, ok := nativeCheckoutUserID(c)
	if !ok {
		return
	}
	offers, err := h.service.ListOffers(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result := make([]nativeCheckoutOfferResponse, 0, len(offers))
	for i := range offers {
		view := offers[i]
		item := nativeCheckoutOfferResponse{
			Code:                view.Code,
			Provider:            view.Provider,
			Name:                view.Name,
			Description:         view.Description,
			ProductKind:         view.ProductKind,
			PayAmountCNYFen:     view.PayAmountCNYFen,
			BenefitAmountCNYFen: view.BenefitAmountCNYFen,
			OncePerUser:         view.OncePerUser,
			Claimed:             view.Claimed,
		}
		if view.Order != nil {
			item.Order = nativeCheckoutOrderDTO(view.Order)
		}
		result = append(result, item)
	}
	response.Success(c, result)
}

func (h *NativeCheckoutHandler) GetManualOfferStatus(c *gin.Context) {
	h.getManualOfferStatus(c, false)
}

func (h *NativeCheckoutHandler) GetManualOfferPurchase(c *gin.Context) {
	h.getManualOfferStatus(c, true)
}

func (h *NativeCheckoutHandler) getManualOfferStatus(c *gin.Context, includePurchaseURL bool) {
	userID, ok := nativeCheckoutUserID(c)
	if !ok {
		return
	}
	status, err := h.service.GetManualOfferStatus(c.Request.Context(), userID, c.Param("offerCode"), includePurchaseURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nativeCheckoutManualOfferStatusResponse{
		Code:        status.Code,
		Claimed:     status.Claimed,
		PurchaseURL: status.PurchaseURL,
	})
}

func (h *NativeCheckoutHandler) CreateOrder(c *gin.Context) {
	userID, ok := nativeCheckoutUserID(c)
	if !ok {
		return
	}
	var request createNativeCheckoutOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid checkout request")
		return
	}
	order, err := h.service.CreateOrder(c.Request.Context(), userID, request.OfferCode, request.PayType)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nativeCheckoutOrderDTO(order))
}

func (h *NativeCheckoutHandler) GetOrder(c *gin.Context) {
	userID, ok := nativeCheckoutUserID(c)
	if !ok {
		return
	}
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		response.BadRequest(c, "Missing order number")
		return
	}
	// Status reads must stay local and cheap. Only the server-side reconciler is
	// allowed to query LDXP, so many tabs or customers cannot multiply upstream
	// polling or race the same fulfillment.
	order, err := h.service.GetOrder(c.Request.Context(), userID, orderNo)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nativeCheckoutOrderDTO(order))
}

func (h *NativeCheckoutHandler) GetDirectPaymentQR(c *gin.Context) {
	userID, ok := nativeCheckoutUserID(c)
	if !ok {
		return
	}
	body, contentType, err := h.service.FetchDirectPaymentQR(c.Request.Context(), userID, c.Param("orderNo"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, contentType, body)
}

func nativeCheckoutUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func nativeCheckoutOrderDTO(order *service.NativeCheckoutOrder) *nativeCheckoutOrderResponse {
	if order == nil {
		return nil
	}
	result := &nativeCheckoutOrderResponse{
		OrderNo:             order.OrderNo,
		Status:              order.Status,
		Provider:            order.Provider,
		PayAmountCNYFen:     order.PayAmountCNYFen,
		BenefitAmountCNYFen: order.BenefitAmountCNYFen,
		PaymentMethod:       order.PaymentMethod,
		CreatedAt:           order.CreatedAt,
	}
	if order.Status == service.NativeCheckoutStatusPending && strings.TrimSpace(order.PaymentURL) != "" {
		result.PaymentURL = order.PaymentURL
		result.DirectQRURL = "/native-checkout/orders/" + order.OrderNo + "/qr"
	}
	return result
}
