package handler

import (
	"net/http"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UserHandler handles user-related requests
type UserHandler struct {
	userService       *service.UserService
	commissionService *service.CommissionService
	identityService   *service.IdentityService
	settingService    *service.SettingService
	cfg               *config.Config
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(userService *service.UserService, commissionService *service.CommissionService, identityService *service.IdentityService, settingService *service.SettingService, cfg *config.Config) *UserHandler {
	return &UserHandler{
		userService:       userService,
		commissionService: commissionService,
		identityService:   identityService,
		settingService:    settingService,
		cfg:               cfg,
	}
}

// ChangePasswordRequest represents the change password request payload
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// UpdateProfileRequest represents the update profile request payload
type UpdateProfileRequest struct {
	Username *string `json:"username"`
}

// GetProfile handles getting user profile
// GET /api/v1/user/profile
func (h *UserHandler) GetProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	userData, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(userData))
}

// GetReferralDashboard 获取当前用户的邀请看板统计
// GET /api/v1/user/referral/dashboard
func (h *UserHandler) GetReferralDashboard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	dashboard, err := h.commissionService.GetUserReferralDashboard(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dashboard)
}

// ChangePassword handles changing user password
// PUT /api/v1/user/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.ChangePasswordRequest{
		CurrentPassword: req.OldPassword,
		NewPassword:     req.NewPassword,
	}
	err := h.userService.ChangePassword(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Password changed successfully"})
}

// UpdateProfile handles updating user profile
// PUT /api/v1/user
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.UpdateProfileRequest{
		Username: req.Username,
	}
	updatedUser, err := h.userService.UpdateProfile(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(updatedUser))
}

func (h *UserHandler) ListIdentityBindings(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.identityService.ListBindings(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *UserHandler) StartLinuxDoBind(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	cfg, err := getLinuxDoOAuthConfig(c.Request.Context(), h.settingService, h.cfg)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	flow, err := buildLinuxDoOAuthStartFlow(c.Request.Context(), cfg, h.identityService, &subject.UserID, service.PendingAuthActionBind, c.Query("redirect"), nil, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if flow.CodeVerifier != "" {
		setCookie(c, linuxDoOAuthVerifierCookie, encodeCookieValue(flow.CodeVerifier), linuxDoOAuthCookieMaxAgeSec, isRequestHTTPS(c))
	}
	response.Success(c, gin.H{"auth_url": flow.AuthURL})
}

func (h *UserHandler) StartGoogleBind(c *gin.Context) {
	h.startExternalIdentityBind(c, service.AuthProviderGoogle)
}

func (h *UserHandler) StartGitHubBind(c *gin.Context) {
	h.startExternalIdentityBind(c, service.AuthProviderGitHub)
}

func (h *UserHandler) startExternalIdentityBind(c *gin.Context, provider string) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var cfg externalOAuthConfig
	var err error
	switch provider {
	case service.AuthProviderGoogle:
		cfg, err = getGoogleOAuthConfig(c.Request.Context(), h.settingService, h.cfg)
	case service.AuthProviderGitHub:
		cfg, err = getGitHubOAuthConfig(c.Request.Context(), h.settingService, h.cfg)
	default:
		err = infraerrors.BadRequest("OAUTH_PROVIDER_UNSUPPORTED", "unsupported oauth provider")
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	flow, err := buildExternalOAuthStartFlow(c.Request.Context(), cfg, h.identityService, &subject.UserID, service.PendingAuthActionBind, c.Query("redirect"), nil, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if flow.CodeVerifier != "" {
		setCookieWithPath(c, externalOAuthVerifierCookie(provider), encodeCookieValue(flow.CodeVerifier), externalOAuthCookiePath(provider), externalOAuthCookieMaxAgeSec, isRequestHTTPS(c))
	}
	response.Success(c, gin.H{"auth_url": flow.AuthURL})
}

func (h *UserHandler) UnbindIdentity(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	provider := c.Param("provider")
	if err := h.identityService.Unbind(c.Request.Context(), subject.UserID, provider); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Identity unbound successfully"})
}
