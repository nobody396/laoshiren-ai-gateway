package handler

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPricingHandler 公开模型价格处理器（无需认证）。
type ModelPricingHandler struct {
	modelPricingService *service.ModelPricingService
}

// NewModelPricingHandler 创建公开模型价格处理器。
func NewModelPricingHandler(modelPricingService *service.ModelPricingService) *ModelPricingHandler {
	return &ModelPricingHandler{modelPricingService: modelPricingService}
}

// GetModelPricing 返回全部 active 分组的模型价格目录。
// GET /api/v1/public/model-pricing
func (h *ModelPricingHandler) GetModelPricing(c *gin.Context) {
	catalog, err := h.modelPricingService.GetPublicModelPricing(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// The catalog shape and active-group contents change independently of the
	// frontend bundle. Do not let a browser or intermediary reuse an older
	// response that can make the current model directory appear empty.
	c.Header("Cache-Control", "no-store, max-age=0")
	c.Header("Pragma", "no-cache")
	response.Success(c, catalog)
}
