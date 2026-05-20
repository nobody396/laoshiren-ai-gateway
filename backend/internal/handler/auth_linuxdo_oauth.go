package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/oauth"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
)

const (
	linuxDoOAuthCookiePath        = "/api/v1/auth/oauth/linuxdo"
	linuxDoOAuthVerifierCookie    = "linuxdo_oauth_verifier"
	linuxDoOAuthCookieMaxAgeSec   = 10 * 60 // 10 minutes
	linuxDoOAuthDefaultRedirectTo = "/dashboard"
	linuxDoOAuthDefaultFrontendCB = "/auth/linuxdo/callback"

	linuxDoOAuthMaxRedirectLen      = 2048
	linuxDoOAuthMaxFragmentValueLen = 512
	linuxDoOAuthMaxSubjectLen       = 64 - len("linuxdo-")
)

type linuxDoTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type linuxDoTokenExchangeError struct {
	StatusCode          int
	ProviderError       string
	ProviderDescription string
	Body                string
}

func (e *linuxDoTokenExchangeError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("token exchange status=%d", e.StatusCode)}
	if strings.TrimSpace(e.ProviderError) != "" {
		parts = append(parts, "error="+strings.TrimSpace(e.ProviderError))
	}
	if strings.TrimSpace(e.ProviderDescription) != "" {
		parts = append(parts, "error_description="+strings.TrimSpace(e.ProviderDescription))
	}
	return strings.Join(parts, " ")
}

type linuxDoStartFlow struct {
	State        string
	RedirectTo   string
	CodeVerifier string
	AuthURL      string
}

// LinuxDoOAuthStart 启动 LinuxDo Connect OAuth 登录流程。
// GET /api/v1/auth/oauth/linuxdo/start?redirect=/dashboard
func (h *AuthHandler) LinuxDoOAuthStart(c *gin.Context) {
	cfg, err := getLinuxDoOAuthConfig(c.Request.Context(), h.settingSvc, h.cfg)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	flow, err := buildLinuxDoOAuthStartFlow(c.Request.Context(), cfg, h.identitySvc, nil, service.PendingAuthActionLogin, c.Query("redirect"), oauthRegistrationContextFromQuery(c), c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if flow.CodeVerifier != "" {
		setCookie(c, linuxDoOAuthVerifierCookie, encodeCookieValue(flow.CodeVerifier), linuxDoOAuthCookieMaxAgeSec, isRequestHTTPS(c))
	}

	c.Redirect(http.StatusFound, flow.AuthURL)
}

// LinuxDoOAuthCallback 处理 OAuth 回调：创建/登录用户，然后重定向到前端。
// GET /api/v1/auth/oauth/linuxdo/callback?code=...&state=...
func (h *AuthHandler) LinuxDoOAuthCallback(c *gin.Context) {
	cfg, cfgErr := getLinuxDoOAuthConfig(c.Request.Context(), h.settingSvc, h.cfg)
	if cfgErr != nil {
		response.ErrorFrom(c, cfgErr)
		return
	}

	frontendCallback := strings.TrimSpace(cfg.FrontendRedirectURL)
	if frontendCallback == "" {
		frontendCallback = linuxDoOAuthDefaultFrontendCB
	}

	if providerErr := strings.TrimSpace(c.Query("error")); providerErr != "" {
		redirectOAuthError(c, frontendCallback, "provider_error", providerErr, c.Query("error_description"))
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		redirectOAuthError(c, frontendCallback, "missing_params", "missing code/state", "")
		return
	}

	session, err := h.identitySvc.GetPendingSession(c.Request.Context(), state)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPendingAuthSessionExpired):
			redirectOAuthError(c, frontendCallback, "state_expired", "oauth state has expired", "")
		case errors.Is(err, service.ErrPendingAuthSessionConsumed):
			redirectOAuthError(c, frontendCallback, "state_consumed", "oauth state has already been used", "")
		default:
			redirectOAuthError(c, frontendCallback, "invalid_state", "invalid oauth state", "")
		}
		return
	}

	redirectTo := sanitizeFrontendRedirectPath(session.RedirectURI)
	if redirectTo == "" {
		redirectTo = linuxDoOAuthDefaultRedirectTo
	}

	codeVerifier := ""
	if cfg.UsePKCE {
		secureCookie := isRequestHTTPS(c)
		defer clearCookie(c, linuxDoOAuthVerifierCookie, secureCookie)
		codeVerifier, _ = readCookieDecoded(c, linuxDoOAuthVerifierCookie)
		if codeVerifier == "" {
			redirectOAuthError(c, frontendCallback, "missing_verifier", "missing pkce verifier", "")
			return
		}
	}

	redirectURI := strings.TrimSpace(cfg.RedirectURL)
	if redirectURI == "" {
		redirectOAuthError(c, frontendCallback, "config_error", "oauth redirect url not configured", "")
		return
	}

	email, username, subject, rawProfile, avatarURL, err := h.resolveLinuxDoSessionIdentity(c.Request.Context(), cfg, session, code, redirectURI, codeVerifier)
	if err != nil {
		var exchangeErr *linuxDoTokenExchangeError
		if errors.As(err, &exchangeErr) && exchangeErr != nil {
			log.Printf(
				"[LinuxDo OAuth] token exchange failed: status=%d provider_error=%q provider_description=%q body=%s",
				exchangeErr.StatusCode,
				exchangeErr.ProviderError,
				exchangeErr.ProviderDescription,
				truncateLogValue(exchangeErr.Body, 2048),
			)
			redirectOAuthError(c, frontendCallback, "token_exchange_failed", "failed to exchange oauth code", singleLine(exchangeErr.Error()))
			return
		}
		log.Printf("[LinuxDo OAuth] identity resolution failed: %v", err)
		redirectOAuthError(c, frontendCallback, "userinfo_failed", "failed to fetch user info", singleLine(err.Error()))
		return
	}

	if session.IntendedAction == service.PendingAuthActionBind {
		if session.UserID == nil || *session.UserID <= 0 {
			redirectOAuthError(c, frontendCallback, "binding_failed", "missing binding user context", "")
			return
		}
		now := time.Now().UTC()
		if _, err := h.identitySvc.UpsertBinding(c.Request.Context(), service.AuthIdentityUpsertInput{
			UserID:         *session.UserID,
			Provider:       service.AuthProviderLinuxDo,
			ProviderUserID: subject,
			Email:          email,
			EmailVerified:  false,
			DisplayName:    username,
			AvatarURL:      avatarURL,
			RawProfile:     rawProfile,
			LastLoginAt:    &now,
		}); err != nil {
			redirectOAuthError(c, frontendCallback, "binding_conflict", infraerrors.Reason(err), infraerrors.Message(err))
			return
		}
		_ = h.identitySvc.MarkPendingSessionConsumed(c.Request.Context(), session.State)
		fragment := url.Values{}
		fragment.Set("binding", "success")
		fragment.Set("provider", service.AuthProviderLinuxDo)
		fragment.Set("redirect", redirectTo)
		redirectWithFragment(c, frontendCallback, fragment)
		return
	}

	if identity, err := h.identitySvc.GetByProviderAccount(c.Request.Context(), service.AuthProviderLinuxDo, subject); err == nil && identity != nil {
		user, userErr := h.userService.GetByID(c.Request.Context(), identity.UserID)
		if userErr != nil {
			redirectOAuthError(c, frontendCallback, "login_failed", infraerrors.Reason(userErr), infraerrors.Message(userErr))
			return
		}
		tokenPair, tokenErr := h.authService.GenerateTokenPair(c.Request.Context(), user, "")
		if tokenErr != nil {
			redirectOAuthError(c, frontendCallback, "login_failed", "service_error", "")
			return
		}
		_ = h.userService.TouchLastActive(c.Request.Context(), user.ID)
		now := time.Now().UTC()
		_, _ = h.identitySvc.UpsertBinding(c.Request.Context(), service.AuthIdentityUpsertInput{
			UserID:         user.ID,
			Provider:       service.AuthProviderLinuxDo,
			ProviderUserID: subject,
			Email:          email,
			EmailVerified:  false,
			DisplayName:    username,
			AvatarURL:      avatarURL,
			RawProfile:     rawProfile,
			LastLoginAt:    &now,
		})
		_ = h.identitySvc.MarkPendingSessionConsumed(c.Request.Context(), session.State)
		redirectOAuthSuccess(c, frontendCallback, redirectTo, tokenPair)
		return
	}

	if prompt, err := h.shouldPromptOAuthReferral(c.Request.Context(), email, session); err != nil {
		redirectOAuthError(c, frontendCallback, "login_failed", infraerrors.Reason(err), infraerrors.Message(err))
		return
	} else if prompt {
		h.redirectOAuthReferralOptional(c, frontendCallback, service.AuthProviderLinuxDo, redirectTo, session)
		return
	}

	tokenPair, user, err := h.authService.LoginOrRegisterOAuthWithTokenPair(c.Request.Context(), email, username, oauthInvitationCodeFromSession(session), oauthReferralCodeFromSession(session))
	if err != nil {
		if errors.Is(err, service.ErrOAuthInvitationRequired) {
			fragment := url.Values{}
			fragment.Set("error", "invitation_required")
			fragment.Set("pending_oauth_token", session.State)
			fragment.Set("provider", service.AuthProviderLinuxDo)
			fragment.Set("redirect", redirectTo)
			if oauthReferralCodeFromSession(session) == "" {
				fragment.Set("referral_optional", "true")
			}
			redirectWithFragment(c, frontendCallback, fragment)
			return
		}
		redirectOAuthError(c, frontendCallback, "login_failed", infraerrors.Reason(err), infraerrors.Message(err))
		return
	}

	now := time.Now().UTC()
	if _, err := h.identitySvc.UpsertBinding(c.Request.Context(), service.AuthIdentityUpsertInput{
		UserID:         user.ID,
		Provider:       service.AuthProviderLinuxDo,
		ProviderUserID: subject,
		Email:          email,
		EmailVerified:  false,
		DisplayName:    username,
		AvatarURL:      avatarURL,
		RawProfile:     rawProfile,
		LastLoginAt:    &now,
	}); err != nil {
		redirectOAuthError(c, frontendCallback, "binding_conflict", infraerrors.Reason(err), infraerrors.Message(err))
		return
	}
	_ = h.identitySvc.MarkPendingSessionConsumed(c.Request.Context(), session.State)
	redirectOAuthSuccess(c, frontendCallback, redirectTo, tokenPair)
}

type completeLinuxDoOAuthRequest struct {
	PendingOAuthToken string `json:"pending_oauth_token" binding:"required"`
	InvitationCode    string `json:"invitation_code"`
	ReferralCode      string `json:"referral_code"`
}

// CompleteLinuxDoOAuthRegistration completes a pending OAuth registration by validating
// the invitation code and creating the user account.
// POST /api/v1/auth/oauth/linuxdo/complete-registration
func (h *AuthHandler) CompleteLinuxDoOAuthRegistration(c *gin.Context) {
	var req completeLinuxDoOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_REQUEST", "message": err.Error()})
		return
	}

	session, sessionErr := h.identitySvc.GetPendingSession(c.Request.Context(), req.PendingOAuthToken)
	if sessionErr == nil && session != nil && session.Provider == service.AuthProviderLinuxDo {
		email, username, subject, rawProfile, avatarURL, identityErr := linuxDoIdentityFromSession(session)
		if identityErr != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_TOKEN", "message": "invalid or expired registration token"})
			return
		}
		referralCode := strings.TrimSpace(req.ReferralCode)
		if referralCode == "" {
			referralCode = oauthReferralCodeFromSession(session)
		}
		tokenPair, user, err := h.authService.LoginOrRegisterOAuthWithTokenPair(c.Request.Context(), email, username, req.InvitationCode, referralCode)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		now := time.Now().UTC()
		if _, err := h.identitySvc.UpsertBinding(c.Request.Context(), service.AuthIdentityUpsertInput{
			UserID:         user.ID,
			Provider:       service.AuthProviderLinuxDo,
			ProviderUserID: subject,
			Email:          email,
			EmailVerified:  false,
			DisplayName:    username,
			AvatarURL:      avatarURL,
			RawProfile:     rawProfile,
			LastLoginAt:    &now,
		}); err != nil {
			response.ErrorFrom(c, err)
			return
		}
		_ = h.identitySvc.MarkPendingSessionConsumed(c.Request.Context(), session.State)
		c.JSON(http.StatusOK, gin.H{
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"expires_in":    tokenPair.ExpiresIn,
			"token_type":    "Bearer",
		})
		return
	}

	email, username, err := h.authService.VerifyPendingOAuthToken(req.PendingOAuthToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_TOKEN", "message": "invalid or expired registration token"})
		return
	}

	tokenPair, _, err := h.authService.LoginOrRegisterOAuthWithTokenPair(c.Request.Context(), email, username, req.InvitationCode, req.ReferralCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenPair.AccessToken,
		"refresh_token": tokenPair.RefreshToken,
		"expires_in":    tokenPair.ExpiresIn,
		"token_type":    "Bearer",
	})
}

func getLinuxDoOAuthConfig(ctx context.Context, settingSvc *service.SettingService, cfg *config.Config) (config.LinuxDoConnectConfig, error) {
	if settingSvc != nil {
		return settingSvc.GetLinuxDoConnectOAuthConfig(ctx)
	}
	if cfg == nil {
		return config.LinuxDoConnectConfig{}, infraerrors.ServiceUnavailable("CONFIG_NOT_READY", "config not loaded")
	}
	if !cfg.LinuxDo.Enabled {
		return config.LinuxDoConnectConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}
	return cfg.LinuxDo, nil
}

func buildLinuxDoOAuthStartFlow(
	ctx context.Context,
	cfg config.LinuxDoConnectConfig,
	identitySvc *service.IdentityService,
	targetUserID *int64,
	intendedAction string,
	redirect string,
	registrationContext map[string]any,
	userAgent string,
	clientIP string,
) (*linuxDoStartFlow, error) {
	if identitySvc == nil {
		return nil, infraerrors.ServiceUnavailable("IDENTITY_SERVICE_NOT_READY", "identity service is not configured")
	}

	state, err := oauth.GenerateState()
	if err != nil {
		return nil, infraerrors.InternalServer("OAUTH_STATE_GEN_FAILED", "failed to generate oauth state").WithCause(err)
	}

	redirectTo := sanitizeFrontendRedirectPath(redirect)
	if redirectTo == "" {
		redirectTo = linuxDoOAuthDefaultRedirectTo
	}
	redirectURI := strings.TrimSpace(cfg.RedirectURL)
	if redirectURI == "" {
		return nil, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth redirect url not configured")
	}

	flow := &linuxDoStartFlow{
		State:      state,
		RedirectTo: redirectTo,
	}
	codeChallenge := ""
	if cfg.UsePKCE {
		verifier, err := oauth.GenerateCodeVerifier()
		if err != nil {
			return nil, infraerrors.InternalServer("OAUTH_PKCE_GEN_FAILED", "failed to generate pkce verifier").WithCause(err)
		}
		flow.CodeVerifier = verifier
		codeChallenge = oauth.GenerateCodeChallenge(verifier)
	}
	if _, err := identitySvc.CreatePendingSession(ctx, service.PendingAuthSessionCreateInput{
		State:          state,
		Provider:       service.AuthProviderLinuxDo,
		IntendedAction: strings.TrimSpace(intendedAction),
		ClaimsSnapshot: registrationContext,
		RedirectURI:    redirectTo,
		UserID:         targetUserID,
		ExpiresAt:      time.Now().UTC().Add(10 * time.Minute),
		IPAddress:      clientIP,
		UserAgent:      userAgent,
	}); err != nil {
		return nil, err
	}

	authURL, err := buildLinuxDoAuthorizeURL(cfg, state, codeChallenge, redirectURI)
	if err != nil {
		return nil, infraerrors.InternalServer("OAUTH_BUILD_URL_FAILED", "failed to build oauth authorization url").WithCause(err)
	}
	flow.AuthURL = authURL
	return flow, nil
}

func redirectOAuthSuccess(c *gin.Context, frontendCallback string, redirectTo string, tokenPair *service.TokenPair) {
	fragment := url.Values{}
	if tokenPair != nil {
		fragment.Set("access_token", tokenPair.AccessToken)
		fragment.Set("refresh_token", tokenPair.RefreshToken)
		fragment.Set("expires_in", fmt.Sprintf("%d", tokenPair.ExpiresIn))
		fragment.Set("token_type", "Bearer")
	}
	fragment.Set("redirect", redirectTo)
	redirectWithFragment(c, frontendCallback, fragment)
}

func linuxDoClaimsSnapshot(email, username, subject, providerEmail string) map[string]any {
	return map[string]any{
		"email":          strings.TrimSpace(email),
		"username":       strings.TrimSpace(username),
		"subject":        strings.TrimSpace(subject),
		"provider_email": strings.TrimSpace(providerEmail),
	}
}

func linuxDoIdentityFromSession(session *service.PendingAuthSession) (email string, username string, subject string, rawProfile map[string]any, avatarURL string, err error) {
	if session == nil || len(session.ClaimsSnapshot) == 0 {
		return "", "", "", nil, "", errors.New("missing pending auth claims")
	}
	rawProfile = oauthIdentityClaimsOnly(session.ClaimsSnapshot)
	email = strings.TrimSpace(stringClaim(session.ClaimsSnapshot, "email"))
	username = strings.TrimSpace(stringClaim(session.ClaimsSnapshot, "username"))
	subject = strings.TrimSpace(stringClaim(session.ClaimsSnapshot, "subject"))
	avatarURL = strings.TrimSpace(stringClaim(session.ClaimsSnapshot, "avatar_url"))
	if email == "" || username == "" || subject == "" {
		return "", "", "", nil, "", errors.New("incomplete pending auth claims")
	}
	return email, username, subject, rawProfile, avatarURL, nil
}

func (h *AuthHandler) resolveLinuxDoSessionIdentity(
	ctx context.Context,
	cfg config.LinuxDoConnectConfig,
	session *service.PendingAuthSession,
	code string,
	redirectURI string,
	codeVerifier string,
) (email string, username string, subject string, rawProfile map[string]any, avatarURL string, err error) {
	if session == nil {
		return "", "", "", nil, "", service.ErrPendingAuthSessionNotFound
	}
	if session.ProviderUserID != nil && strings.TrimSpace(*session.ProviderUserID) != "" && len(session.ClaimsSnapshot) > 0 {
		return linuxDoIdentityFromSession(session)
	}

	tokenResp, err := linuxDoExchangeCode(ctx, cfg, code, redirectURI, codeVerifier)
	if err != nil {
		return "", "", "", nil, "", err
	}

	providerEmail, username, subject, err := linuxDoFetchUserInfo(ctx, cfg, tokenResp)
	if err != nil {
		return "", "", "", nil, "", err
	}
	email = providerEmail
	if subject != "" {
		email = linuxDoSyntheticEmail(subject)
	}
	rawProfile = linuxDoClaimsSnapshot(email, username, subject, providerEmail)

	updatedSession, err := h.identitySvc.ResolvePendingSession(ctx, session.State, service.PendingAuthSessionResolveInput{
		ProviderUserID: subject,
		ClaimsSnapshot: rawProfile,
	})
	if err != nil {
		return "", "", "", nil, "", err
	}
	return linuxDoIdentityFromSession(updatedSession)
}

func linuxDoExchangeCode(
	ctx context.Context,
	cfg config.LinuxDoConnectConfig,
	code string,
	redirectURI string,
	codeVerifier string,
) (*linuxDoTokenResponse, error) {
	client := req.C().SetTimeout(30 * time.Second)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", cfg.ClientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	if cfg.UsePKCE {
		form.Set("code_verifier", codeVerifier)
	}

	r := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json")

	switch strings.ToLower(strings.TrimSpace(cfg.TokenAuthMethod)) {
	case "", "client_secret_post":
		form.Set("client_secret", cfg.ClientSecret)
	case "client_secret_basic":
		r.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	case "none":
	default:
		return nil, fmt.Errorf("unsupported token_auth_method: %s", cfg.TokenAuthMethod)
	}

	resp, err := r.SetFormDataFromValues(form).Post(cfg.TokenURL)
	if err != nil {
		return nil, fmt.Errorf("request token: %w", err)
	}
	body := strings.TrimSpace(resp.String())
	if !resp.IsSuccessState() {
		providerErr, providerDesc := parseOAuthProviderError(body)
		return nil, &linuxDoTokenExchangeError{
			StatusCode:          resp.StatusCode,
			ProviderError:       providerErr,
			ProviderDescription: providerDesc,
			Body:                body,
		}
	}

	tokenResp, ok := parseLinuxDoTokenResponse(body)
	if !ok || strings.TrimSpace(tokenResp.AccessToken) == "" {
		return nil, &linuxDoTokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       body,
		}
	}
	if strings.TrimSpace(tokenResp.TokenType) == "" {
		tokenResp.TokenType = "Bearer"
	}
	return tokenResp, nil
}

func linuxDoFetchUserInfo(
	ctx context.Context,
	cfg config.LinuxDoConnectConfig,
	token *linuxDoTokenResponse,
) (email string, username string, subject string, err error) {
	client := req.C().SetTimeout(30 * time.Second)
	authorization, err := buildBearerAuthorization(token.TokenType, token.AccessToken)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid token for userinfo request: %w", err)
	}

	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", authorization).
		Get(cfg.UserInfoURL)
	if err != nil {
		return "", "", "", fmt.Errorf("request userinfo: %w", err)
	}
	if !resp.IsSuccessState() {
		return "", "", "", fmt.Errorf("userinfo status=%d", resp.StatusCode)
	}

	return linuxDoParseUserInfo(resp.String(), cfg)
}

func linuxDoParseUserInfo(body string, cfg config.LinuxDoConnectConfig) (email string, username string, subject string, err error) {
	email = firstNonEmpty(
		getGJSON(body, cfg.UserInfoEmailPath),
		getGJSON(body, "email"),
		getGJSON(body, "user.email"),
		getGJSON(body, "data.email"),
		getGJSON(body, "attributes.email"),
	)
	username = firstNonEmpty(
		getGJSON(body, cfg.UserInfoUsernamePath),
		getGJSON(body, "username"),
		getGJSON(body, "preferred_username"),
		getGJSON(body, "name"),
		getGJSON(body, "user.username"),
		getGJSON(body, "user.name"),
	)
	subject = firstNonEmpty(
		getGJSON(body, cfg.UserInfoIDPath),
		getGJSON(body, "sub"),
		getGJSON(body, "id"),
		getGJSON(body, "user_id"),
		getGJSON(body, "uid"),
		getGJSON(body, "user.id"),
	)

	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", "", "", errors.New("userinfo missing id field")
	}
	if !isSafeLinuxDoSubject(subject) {
		return "", "", "", errors.New("userinfo returned invalid id field")
	}

	email = strings.TrimSpace(email)
	if email == "" {
		// LinuxDo Connect 的 userinfo 可能不提供 email。为兼容现有用户模型（email 必填且唯一），使用稳定的合成邮箱。
		email = linuxDoSyntheticEmail(subject)
	}

	username = strings.TrimSpace(username)
	if username == "" {
		username = "linuxdo_" + subject
	}

	return email, username, subject, nil
}

func buildLinuxDoAuthorizeURL(cfg config.LinuxDoConnectConfig, state string, codeChallenge string, redirectURI string) (string, error) {
	u, err := url.Parse(cfg.AuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("parse authorize_url: %w", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	if strings.TrimSpace(cfg.Scopes) != "" {
		q.Set("scope", cfg.Scopes)
	}
	q.Set("state", state)
	if cfg.UsePKCE {
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", "S256")
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func redirectOAuthError(c *gin.Context, frontendCallback string, code string, message string, description string) {
	fragment := url.Values{}
	fragment.Set("error", truncateFragmentValue(code))
	if strings.TrimSpace(message) != "" {
		fragment.Set("error_message", truncateFragmentValue(message))
	}
	if strings.TrimSpace(description) != "" {
		fragment.Set("error_description", truncateFragmentValue(description))
	}
	redirectWithFragment(c, frontendCallback, fragment)
}

func redirectWithFragment(c *gin.Context, frontendCallback string, fragment url.Values) {
	u, err := url.Parse(frontendCallback)
	if err != nil {
		// 兜底：尽力跳转到默认页面，避免卡死在回调页。
		c.Redirect(http.StatusFound, linuxDoOAuthDefaultRedirectTo)
		return
	}
	if u.Scheme != "" && !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		c.Redirect(http.StatusFound, linuxDoOAuthDefaultRedirectTo)
		return
	}
	u.Fragment = fragment.Encode()
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Redirect(http.StatusFound, u.String())
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func parseOAuthProviderError(body string) (providerErr string, providerDesc string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", ""
	}

	providerErr = firstNonEmpty(
		getGJSON(body, "error"),
		getGJSON(body, "code"),
		getGJSON(body, "error.code"),
	)
	providerDesc = firstNonEmpty(
		getGJSON(body, "error_description"),
		getGJSON(body, "error.message"),
		getGJSON(body, "message"),
		getGJSON(body, "detail"),
	)

	if providerErr != "" || providerDesc != "" {
		return providerErr, providerDesc
	}

	values, err := url.ParseQuery(body)
	if err != nil {
		return "", ""
	}
	providerErr = firstNonEmpty(values.Get("error"), values.Get("code"))
	providerDesc = firstNonEmpty(values.Get("error_description"), values.Get("error_message"), values.Get("message"))
	return providerErr, providerDesc
}

func parseLinuxDoTokenResponse(body string) (*linuxDoTokenResponse, bool) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, false
	}

	accessToken := strings.TrimSpace(getGJSON(body, "access_token"))
	if accessToken != "" {
		tokenType := strings.TrimSpace(getGJSON(body, "token_type"))
		refreshToken := strings.TrimSpace(getGJSON(body, "refresh_token"))
		scope := strings.TrimSpace(getGJSON(body, "scope"))
		expiresIn := gjson.Get(body, "expires_in").Int()
		return &linuxDoTokenResponse{
			AccessToken:  accessToken,
			TokenType:    tokenType,
			ExpiresIn:    expiresIn,
			RefreshToken: refreshToken,
			Scope:        scope,
		}, true
	}

	values, err := url.ParseQuery(body)
	if err != nil {
		return nil, false
	}
	accessToken = strings.TrimSpace(values.Get("access_token"))
	if accessToken == "" {
		return nil, false
	}
	expiresIn := int64(0)
	if raw := strings.TrimSpace(values.Get("expires_in")); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			expiresIn = v
		}
	}
	return &linuxDoTokenResponse{
		AccessToken:  accessToken,
		TokenType:    strings.TrimSpace(values.Get("token_type")),
		ExpiresIn:    expiresIn,
		RefreshToken: strings.TrimSpace(values.Get("refresh_token")),
		Scope:        strings.TrimSpace(values.Get("scope")),
	}, true
}

func getGJSON(body string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	res := gjson.Get(body, path)
	if !res.Exists() {
		return ""
	}
	return res.String()
}

func stringClaim(claims map[string]any, key string) string {
	if len(claims) == 0 {
		return ""
	}
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func truncateLogValue(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxLen <= 0 {
		return ""
	}
	if len(value) <= maxLen {
		return value
	}
	value = value[:maxLen]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func singleLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func sanitizeFrontendRedirectPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if len(path) > linuxDoOAuthMaxRedirectLen {
		return ""
	}
	// 只允许同源相对路径（避免开放重定向）。
	if !strings.HasPrefix(path, "/") {
		return ""
	}
	if strings.HasPrefix(path, "//") {
		return ""
	}
	if strings.Contains(path, "://") {
		return ""
	}
	if strings.ContainsAny(path, "\r\n") {
		return ""
	}
	return path
}

func oauthRegistrationContextFromQuery(c *gin.Context) map[string]any {
	if c == nil {
		return nil
	}
	context := map[string]any{}
	if invitationCode := sanitizeOAuthRegistrationCode(c.Query("invitation_code")); invitationCode != "" {
		context["invitation_code"] = invitationCode
	}
	if referralCode := sanitizeOAuthRegistrationCode(firstNonEmptyQuery(c, "referral_code", "ref", "aff")); referralCode != "" {
		context["referral_code"] = referralCode
	}
	if len(context) == 0 {
		return nil
	}
	return context
}

func oauthInvitationCodeFromSession(session *service.PendingAuthSession) string {
	if session == nil {
		return ""
	}
	return sanitizeOAuthRegistrationCode(stringClaim(session.ClaimsSnapshot, "invitation_code"))
}

func oauthReferralCodeFromSession(session *service.PendingAuthSession) string {
	if session == nil {
		return ""
	}
	return sanitizeOAuthRegistrationCode(stringClaim(session.ClaimsSnapshot, "referral_code"))
}

func oauthIdentityClaimsOnly(claims map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range claims {
		if key == "invitation_code" || key == "referral_code" {
			continue
		}
		out[key] = value
	}
	return out
}

func firstNonEmptyQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(c.Query(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func sanitizeOAuthRegistrationCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 128 || strings.ContainsAny(code, "\r\n") {
		return ""
	}
	return code
}

func isRequestHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	proto := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")))
	return proto == "https"
}

func encodeCookieValue(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCookieValue(value string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func readCookieDecoded(c *gin.Context, name string) (string, error) {
	ck, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return decodeCookieValue(ck.Value)
}

func setCookie(c *gin.Context, name string, value string, maxAgeSec int, secure bool) {
	setCookieWithPath(c, name, value, linuxDoOAuthCookiePath, maxAgeSec, secure)
}

func setCookieWithPath(c *gin.Context, name string, value string, path string, maxAgeSec int, secure bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAgeSec,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(c *gin.Context, name string, secure bool) {
	clearCookieWithPath(c, name, linuxDoOAuthCookiePath, secure)
}

func clearCookieWithPath(c *gin.Context, name string, path string, secure bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func truncateFragmentValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > linuxDoOAuthMaxFragmentValueLen {
		value = value[:linuxDoOAuthMaxFragmentValueLen]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func buildBearerAuthorization(tokenType, accessToken string) (string, error) {
	tokenType = strings.TrimSpace(tokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	if !strings.EqualFold(tokenType, "Bearer") {
		return "", fmt.Errorf("unsupported token_type: %s", tokenType)
	}

	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return "", errors.New("missing access_token")
	}
	if strings.ContainsAny(accessToken, " \t\r\n") {
		return "", errors.New("access_token contains whitespace")
	}
	return "Bearer " + accessToken, nil
}

func isSafeLinuxDoSubject(subject string) bool {
	subject = strings.TrimSpace(subject)
	if subject == "" || len(subject) > linuxDoOAuthMaxSubjectLen {
		return false
	}
	for _, r := range subject {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func linuxDoSyntheticEmail(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ""
	}
	return "linuxdo-" + subject + service.LinuxDoConnectSyntheticEmailDomain
}
