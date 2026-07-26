package handler

import (
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ChangelogHandler serves the public Build in Public feed.
type ChangelogHandler struct {
	service *service.ChangelogService
}

func NewChangelogHandler(changelogService *service.ChangelogService) *ChangelogHandler {
	return &ChangelogHandler{service: changelogService}
}

func (h *ChangelogHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    "published_at",
		SortOrder: "desc",
	}
	items, result, err := h.service.ListPublished(c.Request.Context(), params, service.ChangelogListFilters{
		Category: strings.TrimSpace(c.Query("category")),
		Search:   search,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.PublicChangelogEntry, 0, len(items))
	for i := range items {
		out = append(out, *dto.PublicChangelogFromService(&items[i]))
	}
	c.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	response.Paginated(c, out, result.Total, page, pageSize)
}

func (h *ChangelogHandler) GetBySlug(c *gin.Context) {
	entry, err := h.service.GetPublishedBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	response.Success(c, dto.PublicChangelogFromService(entry))
}

func (h *ChangelogHandler) Latest(c *gin.Context) {
	entry, err := h.service.LatestPublished(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	response.Success(c, dto.PublicChangelogFromService(entry))
}
