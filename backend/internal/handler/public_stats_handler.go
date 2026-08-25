package handler

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PublicStatsHandler 公开平台统计处理器（无需认证）。
type PublicStatsHandler struct {
	publicStatsService *service.PublicStatsService
}

// NewPublicStatsHandler 创建公开平台统计处理器。
func NewPublicStatsHandler(publicStatsService *service.PublicStatsService) *PublicStatsHandler {
	return &PublicStatsHandler{publicStatsService: publicStatsService}
}

// GetPublicStats 返回平台累计统计（tokens/请求数/累计赔付，落地页计数器使用）。
// GET /api/v1/public/stats
func (h *PublicStatsHandler) GetPublicStats(c *gin.Context) {
	stats, err := h.publicStatsService.GetPublicStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}
