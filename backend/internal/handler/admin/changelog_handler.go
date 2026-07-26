package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	servermiddleware "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ChangelogHandler struct {
	service *service.ChangelogService
}

func NewChangelogHandler(changelogService *service.ChangelogService) *ChangelogHandler {
	return &ChangelogHandler{service: changelogService}
}

type CreateChangelogRequest struct {
	Slug            string     `json:"slug"`
	Title           string     `json:"title" binding:"required"`
	Summary         string     `json:"summary" binding:"required"`
	Rationale       string     `json:"rationale" binding:"required"`
	Content         string     `json:"content" binding:"required"`
	Category        string     `json:"category" binding:"required,oneof=feature model_config improvement fix"`
	RelatedProducts []string   `json:"related_products"`
	Status          string     `json:"status" binding:"omitempty,oneof=draft published archived"`
	PublishedAt     *time.Time `json:"published_at"`
	CommitSHA       *string    `json:"commit_sha"`
	PullRequestURL  *string    `json:"pull_request_url"`
}

type UpdateChangelogRequest struct {
	Slug            *string    `json:"slug"`
	Title           *string    `json:"title"`
	Summary         *string    `json:"summary"`
	Rationale       *string    `json:"rationale"`
	Content         *string    `json:"content"`
	Category        *string    `json:"category" binding:"omitempty,oneof=feature model_config improvement fix"`
	RelatedProducts *[]string  `json:"related_products"`
	Status          *string    `json:"status" binding:"omitempty,oneof=draft published archived"`
	PublishedAt     *time.Time `json:"published_at"`
	CommitSHA       *string    `json:"commit_sha"`
	PullRequestURL  *string    `json:"pull_request_url"`
}

func (h *ChangelogHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    strings.TrimSpace(c.DefaultQuery("sort_by", "published_at")),
		SortOrder: strings.TrimSpace(c.DefaultQuery("sort_order", "desc")),
	}
	items, result, err := h.service.List(c.Request.Context(), params, service.ChangelogListFilters{
		Status:   strings.TrimSpace(c.Query("status")),
		Category: strings.TrimSpace(c.Query("category")),
		Search:   search,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.AdminChangelogEntry, 0, len(items))
	for i := range items {
		out = append(out, *dto.AdminChangelogFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

func (h *ChangelogHandler) GetByID(c *gin.Context) {
	id, ok := parseChangelogID(c)
	if !ok {
		return
	}
	entry, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminChangelogFromService(entry))
}

func (h *ChangelogHandler) Create(c *gin.Context) {
	var request CreateChangelogRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	entry, err := h.service.Create(c.Request.Context(), &service.CreateChangelogInput{
		Slug:            request.Slug,
		Title:           request.Title,
		Summary:         request.Summary,
		Rationale:       request.Rationale,
		Content:         request.Content,
		Category:        request.Category,
		RelatedProducts: request.RelatedProducts,
		Status:          request.Status,
		PublishedAt:     request.PublishedAt,
		CommitSHA:       request.CommitSHA,
		PullRequestURL:  request.PullRequestURL,
		ActorID:         &subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminChangelogFromService(entry))
}

func (h *ChangelogHandler) Update(c *gin.Context) {
	id, ok := parseChangelogID(c)
	if !ok {
		return
	}
	var request UpdateChangelogRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	input := &service.UpdateChangelogInput{
		Slug:            request.Slug,
		Title:           request.Title,
		Summary:         request.Summary,
		Rationale:       request.Rationale,
		Content:         request.Content,
		Category:        request.Category,
		RelatedProducts: request.RelatedProducts,
		Status:          request.Status,
		PublishedAt:     request.PublishedAt,
		ActorID:         &subject.UserID,
	}
	if request.CommitSHA != nil {
		value := request.CommitSHA
		input.CommitSHA = &value
	}
	if request.PullRequestURL != nil {
		value := request.PullRequestURL
		input.PullRequestURL = &value
	}

	entry, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminChangelogFromService(entry))
}

func (h *ChangelogHandler) Delete(c *gin.Context) {
	id, ok := parseChangelogID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Changelog entry deleted successfully"})
}

func parseChangelogID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid changelog ID")
		return 0, false
	}
	return id, true
}
