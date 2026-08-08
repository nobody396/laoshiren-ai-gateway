package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	googleOAuthDefaultFrontendCB = "/auth/google/callback"
	gitHubOAuthDefaultFrontendCB = "/auth/github/callback"

	externalOAuthCookieMaxAgeSec = 10 * 60
	externalOAuthMaxSubjectLen   = 255
)

type externalOAuthConfig struct {
	Provider             string
	ClientID             string
	ClientSecret         string
	AuthorizeURL         string
	TokenURL             string
	UserInfoURL          string
	EmailsURL            string
	Scopes               string
	RedirectURL          string
	FrontendRedirectURL  string
	TokenAuthMethod      string
	UsePKCE              bool
	RequireEmailVerified bool
	UserInfoEmailPath    string
	UserInfoIDPath       string
	UserInfoUsernamePath string
}

type externalOAuthStartFlow struct {
	State        string
	RedirectTo   string
	CodeVerifier string
	AuthURL      string
}

type externalOAuthIdentity struct {
	Email         string
	Username      string
	Subject       string
	RawProfile    map[string]any
	AvatarURL     string
	EmailVerified bool
}

type completeExternalOAuthRequest struct {
	PendingOAuthToken string `json:"pending_oauth_token" binding:"required"`
	InvitationCode    string `json:"invitation_code"`
	ReferralCode      string `json:"referral_code"`
}

// GoogleOAuthStart starts the Google/OpenID Connect login flow.
func (h *AuthHandler) GoogleOAuthStart(c *gin.Context) {
	h.startExternalOAuth(c, service.AuthProviderGoogle)
}

// GitHubOAuthStart starts the GitHub OAuth login flow.
func (h *AuthHandler) GitHubOAuthStart(c *gin.Context) {
	h.startExternalOAuth(c, service.AuthProviderGitHub)
}

func (h *AuthHandler) startExternalOAuth(c *gin.Context, provider string) {
	cfg, err := h.getExternalOAuthConfig(c.Request.Context(), provider)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	flow, err := buildExternalOAuthStartFlow(c.Request.Context(), cfg, h.identitySvc, nil, service.PendingAuthActionLogin, c.Query("redirect"), oauthRegistrationContextFromQuery(c), c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if flow.CodeVerifier != "" {
		setCookieWithPath(c, externalOAuthVerifierCookie(provider), encodeCookieValue(flow.CodeVerifier), externalOAuthCookiePath(provider), externalOAuthCookieMaxAgeSec, isRequestHTTPS(c))
	}
	c.Redirect(http.StatusFound, flow.AuthURL)
}

// GoogleOAuthCallback handles the Google/OpenID Connect OAuth callback.
func (h *AuthHandler) GoogleOAuthCallback(c *gin.Context) {
	h.externalOAuthCallback(c, service.AuthProviderGoogle)
}

// GitHubOAuthCallback handles the GitHub OAuth callback.
func (h *AuthHandler) GitHubOAuthCallback(c *gin.Context) {
	h.externalOAuthCallback(c, service.AuthProviderGitHub)
}

func (h *AuthHandler) externalOAuthCallback(c *gin.Context, provider string) {
	cfg, cfgErr := h.getExternalOAuthConfig(c.Request.Context(), provider)
	if cfgErr != nil {
		response.ErrorFrom(c, cfgErr)
		return
	}
	frontendCallback := strings.TrimSpace(cfg.FrontendRedirectURL)
	if frontendCallback == "" {
		frontendCallback = defaultExternalFrontendCallback(provider)
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
	if session.Provider != provider {
		redirectOAuthError(c, frontendCallback, "invalid_state", "oauth state provider mismatch", "")
		return
	}

	redirectTo := sanitizeFrontendRedirectPath(session.RedirectURI)
	if redirectTo == "" {
		redirectTo = linuxDoOAuthDefaultRedirectTo
	}

	codeVerifier := ""
	if cfg.UsePKCE {
		secureCookie := isRequestHTTPS(c)
		cookieName := externalOAuthVerifierCookie(provider)
		defer clearCookieWithPath(c, cookieName, externalOAuthCookiePath(provider), secureCookie)
		codeVerifier, _ = readCookieDecoded(c, cookieName)
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

	identity, err := h.resolveExternalOAuthIdentity(c.Request.Context(), cfg, session, code, redirectURI, codeVerifier)
	if err != nil {
		var exchangeErr *linuxDoTokenExchangeError
		if errors.As(err, &exchangeErr) && exchangeErr != nil {
			log.Printf(
				"[%s OAuth] token exchange failed: status=%d provider_error=%q provider_description=%q body=%s",
				provider,
				exchangeErr.StatusCode,
				exchangeErr.ProviderError,
				exchangeErr.ProviderDescription,
				truncateLogValue(exchangeErr.Body, 2048),
			)
			redirectOAuthError(c, frontendCallback, "token_exchange_failed", "failed to exchange oauth code", singleLine(exchangeErr.Error()))
			return
		}
		log.Printf("[%s OAuth] identity resolution failed: %v", provider, err)
		redirectOAuthError(c, frontendCallback, "userinfo_failed", "failed to fetch user info", singleLine(err.Error()))
		return
	}

	h.completeExternalOAuthLogin(c, frontendCallback, redirectTo, session, provider, identity)
}

func (h *AuthHandler) completeExternalOAuthLogin(
	c *gin.Context,
	frontendCallback string,
	redirectTo string,
	session *service.PendingAuthSession,
	provider string,
	identity *externalOAuthIdentity,
) {
	if identity == nil || strings.TrimSpace(identity.Email) == "" || strings.TrimSpace(identity.Subject) == "" {
		redirectOAuthError(c, frontendCallback, "userinfo_failed", "provider identity is incomplete", "")
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
			Provider:       provider,
			ProviderUserID: identity.Subject,
			Email:          identity.Email,
			EmailVerified:  identity.EmailVerified,
			DisplayName:    identity.Username,
			AvatarURL:      identity.AvatarURL,
			RawProfile:     identity.RawProfile,
			LastLoginAt:    &now,
		}); err != nil {
			redirectOAuthError(c, frontendCallback, "binding_conflict", infraerrors.Reason(err), infraerrors.Message(err))
			return
		}
		_ = h.identitySvc.MarkPendingSessionConsumed(c.Request.Context(), session.State)
		fragment := url.Values{}
		fragment.Set("binding", "success")
		fragment.Set("provider", provider)
		fragment.Set("redirect", redirectTo)
		redirectWithFragment(c, frontendCallback, fragment)
		return
	}

	if bound, err := h.identitySvc.GetByProviderAccount(c.Request.Context(), provider, identity.Subject); err == nil && bound != nil {
		user, userErr := h.userService.GetByID(c.Request.Context(), bound.UserID)
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
			Provider:       provider,
			ProviderUserID: identity.Subject,
			Email:          identity.Email,
			EmailVerified:  identity.EmailVerified,
			DisplayName:    identity.Username,
			AvatarURL:      identity.AvatarURL,
			RawProfile:     identity.RawProfile,
			LastLoginAt:    &now,
		})
		_ = h.identitySvc.MarkPendingSessionConsumed(c.Request.Context(), session.State)
		redirectOAuthSuccess(c, frontendCallback, redirectTo, tokenPair)
		return
	}

	if prompt, err := h.shouldPromptOAuthReferral(c.Request.Context(), identity.Email, session); err != nil {
		redirectOAuthError(c, frontendCallback, "login_failed", infraerrors.Reason(err), infraerrors.Message(err))
		return
	} else if prompt {
		h.redirectOAuthReferralOptional(c, frontendCallback, provider, redirectTo, session)
		return
	}

	tokenPair, user, err := h.authService.LoginOrRegisterOAuthIdentityWithTokenPair(c.Request.Context(), identity.Email, identity.Username, oauthInvitationCodeFromSession(session), oauthReferralCodeFromSession(session), identity.EmailVerified)
	if err != nil {
		if errors.Is(err, service.ErrOAuthInvitationRequired) {
			fragment := url.Values{}
			fragment.Set("error", "invitation_required")
			fragment.Set("pending_oauth_token", session.State)
			fragment.Set("provider", provider)
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
		Provider:       provider,
		ProviderUserID: identity.Subject,
		Email:          identity.Email,
		EmailVerified:  identity.EmailVerified,
		DisplayName:    identity.Username,
		AvatarURL:      identity.AvatarURL,
		RawProfile:     identity.RawProfile,
		LastLoginAt:    &now,
	}); err != nil {
		redirectOAuthError(c, frontendCallback, "binding_conflict", infraerrors.Reason(err), infraerrors.Message(err))
		return
	}
	_ = h.identitySvc.MarkPendingSessionConsumed(c.Request.Context(), session.State)
	redirectOAuthSuccess(c, frontendCallback, redirectTo, tokenPair)
}

func (h *AuthHandler) CompleteGoogleOAuthRegistration(c *gin.Context) {
	h.completeExternalOAuthRegistration(c, service.AuthProviderGoogle)
}

func (h *AuthHandler) CompleteGitHubOAuthRegistration(c *gin.Context) {
	h.completeExternalOAuthRegistration(c, service.AuthProviderGitHub)
}

func (h *AuthHandler) completeExternalOAuthRegistration(c *gin.Context, provider string) {
	var req completeExternalOAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_REQUEST", "message": err.Error()})
		return
	}
	session, sessionErr := h.identitySvc.GetPendingSession(c.Request.Context(), req.PendingOAuthToken)
	if sessionErr != nil || session == nil || session.Provider != provider {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_TOKEN", "message": "invalid or expired registration token"})
		return
	}
	identity, err := externalIdentityFromSession(session)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "INVALID_TOKEN", "message": "invalid or expired registration token"})
		return
	}

	referralCode := strings.TrimSpace(req.ReferralCode)
	if referralCode == "" {
		referralCode = oauthReferralCodeFromSession(session)
	}
	tokenPair, user, err := h.authService.LoginOrRegisterOAuthIdentityWithTokenPair(c.Request.Context(), identity.Email, identity.Username, req.InvitationCode, referralCode, identity.EmailVerified)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	now := time.Now().UTC()
	if _, err := h.identitySvc.UpsertBinding(c.Request.Context(), service.AuthIdentityUpsertInput{
		UserID:         user.ID,
		Provider:       provider,
		ProviderUserID: identity.Subject,
		Email:          identity.Email,
		EmailVerified:  identity.EmailVerified,
		DisplayName:    identity.Username,
		AvatarURL:      identity.AvatarURL,
		RawProfile:     identity.RawProfile,
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
}

func (h *AuthHandler) shouldPromptOAuthReferral(ctx context.Context, email string, session *service.PendingAuthSession) (bool, error) {
	if h == nil || h.authService == nil {
		return false, nil
	}
	return h.authService.ShouldPromptOAuthReferral(ctx, email, oauthReferralCodeFromSession(session))
}

func (h *AuthHandler) redirectOAuthReferralOptional(c *gin.Context, frontendCallback, provider, redirectTo string, session *service.PendingAuthSession) {
	fragment := url.Values{}
	fragment.Set("error", "referral_optional")
	fragment.Set("pending_oauth_token", session.State)
	fragment.Set("provider", provider)
	fragment.Set("redirect", redirectTo)
	if h.oauthInvitationRequiredForSession(c.Request.Context(), session) {
		fragment.Set("invitation_required", "true")
	}
	redirectWithFragment(c, frontendCallback, fragment)
}

func (h *AuthHandler) oauthInvitationRequiredForSession(ctx context.Context, session *service.PendingAuthSession) bool {
	if h == nil || h.settingSvc == nil || session == nil || oauthInvitationCodeFromSession(session) != "" {
		return false
	}
	return h.settingSvc.IsInvitationCodeEnabled(ctx)
}

func (h *AuthHandler) getExternalOAuthConfig(ctx context.Context, provider string) (externalOAuthConfig, error) {
	switch provider {
	case service.AuthProviderGoogle:
		return getGoogleOAuthConfig(ctx, h.settingSvc, h.cfg)
	case service.AuthProviderGitHub:
		return getGitHubOAuthConfig(ctx, h.settingSvc, h.cfg)
	default:
		return externalOAuthConfig{}, infraerrors.BadRequest("OAUTH_PROVIDER_UNSUPPORTED", "unsupported oauth provider")
	}
}

func getGoogleOAuthConfig(ctx context.Context, settingSvc *service.SettingService, cfg *config.Config) (externalOAuthConfig, error) {
	var oidcCfg config.OIDCConnectConfig
	var err error
	if settingSvc != nil {
		oidcCfg, err = settingSvc.GetOIDCConnectOAuthConfig(ctx)
	} else if cfg != nil && cfg.OIDC.Enabled {
		oidcCfg = cfg.OIDC
	} else {
		err = infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}
	if err != nil {
		return externalOAuthConfig{}, err
	}
	frontendCallback := strings.TrimSpace(oidcCfg.FrontendRedirectURL)
	if frontendCallback == "" {
		frontendCallback = googleOAuthDefaultFrontendCB
	}
	return externalOAuthConfig{
		Provider:             service.AuthProviderGoogle,
		ClientID:             strings.TrimSpace(oidcCfg.ClientID),
		ClientSecret:         strings.TrimSpace(oidcCfg.ClientSecret),
		AuthorizeURL:         strings.TrimSpace(oidcCfg.AuthorizeURL),
		TokenURL:             strings.TrimSpace(oidcCfg.TokenURL),
		UserInfoURL:          strings.TrimSpace(oidcCfg.UserInfoURL),
		Scopes:               strings.TrimSpace(oidcCfg.Scopes),
		RedirectURL:          strings.TrimSpace(oidcCfg.RedirectURL),
		FrontendRedirectURL:  frontendCallback,
		TokenAuthMethod:      strings.ToLower(strings.TrimSpace(oidcCfg.TokenAuthMethod)),
		UsePKCE:              oidcCfg.UsePKCE,
		RequireEmailVerified: oidcCfg.RequireEmailVerified,
		UserInfoEmailPath:    strings.TrimSpace(oidcCfg.UserInfoEmailPath),
		UserInfoIDPath:       strings.TrimSpace(oidcCfg.UserInfoIDPath),
		UserInfoUsernamePath: strings.TrimSpace(oidcCfg.UserInfoUsernamePath),
	}, nil
}

func getGitHubOAuthConfig(ctx context.Context, settingSvc *service.SettingService, cfg *config.Config) (externalOAuthConfig, error) {
	var ghCfg config.GitHubOAuthConfig
	var err error
	if settingSvc != nil {
		ghCfg, err = settingSvc.GetGitHubOAuthConfig(ctx)
	} else if cfg != nil && cfg.GitHub.Enabled {
		ghCfg = cfg.GitHub
	} else {
		err = infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}
	if err != nil {
		return externalOAuthConfig{}, err
	}
	frontendCallback := strings.TrimSpace(ghCfg.FrontendRedirectURL)
	if frontendCallback == "" {
		frontendCallback = gitHubOAuthDefaultFrontendCB
	}
	return externalOAuthConfig{
		Provider:            service.AuthProviderGitHub,
		ClientID:            strings.TrimSpace(ghCfg.ClientID),
		ClientSecret:        strings.TrimSpace(ghCfg.ClientSecret),
		AuthorizeURL:        strings.TrimSpace(ghCfg.AuthorizeURL),
		TokenURL:            strings.TrimSpace(ghCfg.TokenURL),
		UserInfoURL:         strings.TrimSpace(ghCfg.UserInfoURL),
		EmailsURL:           strings.TrimSpace(ghCfg.EmailsURL),
		Scopes:              strings.TrimSpace(ghCfg.Scopes),
		RedirectURL:         strings.TrimSpace(ghCfg.RedirectURL),
		FrontendRedirectURL: frontendCallback,
		TokenAuthMethod:     "client_secret_post",
	}, nil
}

func buildExternalOAuthStartFlow(
	ctx context.Context,
	cfg externalOAuthConfig,
	identitySvc *service.IdentityService,
	targetUserID *int64,
	intendedAction string,
	redirect string,
	registrationContext map[string]any,
	userAgent string,
	clientIP string,
) (*externalOAuthStartFlow, error) {
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

	flow := &externalOAuthStartFlow{State: state, RedirectTo: redirectTo}
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
		Provider:       cfg.Provider,
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
	authURL, err := buildExternalAuthorizeURL(cfg, state, codeChallenge, redirectURI)
	if err != nil {
		return nil, infraerrors.InternalServer("OAUTH_BUILD_URL_FAILED", "failed to build oauth authorization url").WithCause(err)
	}
	flow.AuthURL = authURL
	return flow, nil
}

func (h *AuthHandler) resolveExternalOAuthIdentity(
	ctx context.Context,
	cfg externalOAuthConfig,
	session *service.PendingAuthSession,
	code string,
	redirectURI string,
	codeVerifier string,
) (*externalOAuthIdentity, error) {
	if session == nil {
		return nil, service.ErrPendingAuthSessionNotFound
	}
	if session.ProviderUserID != nil && strings.TrimSpace(*session.ProviderUserID) != "" && len(session.ClaimsSnapshot) > 0 {
		return externalIdentityFromSession(session)
	}

	tokenResp, err := exchangeExternalOAuthCode(ctx, cfg, code, redirectURI, codeVerifier)
	if err != nil {
		return nil, err
	}

	var identity *externalOAuthIdentity
	switch cfg.Provider {
	case service.AuthProviderGoogle:
		identity, err = fetchGoogleOAuthIdentity(ctx, cfg, tokenResp)
	case service.AuthProviderGitHub:
		identity, err = fetchGitHubOAuthIdentity(ctx, cfg, tokenResp)
	default:
		err = fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
	if err != nil {
		return nil, err
	}
	updatedSession, err := h.identitySvc.ResolvePendingSession(ctx, session.State, service.PendingAuthSessionResolveInput{
		ProviderUserID: identity.Subject,
		ClaimsSnapshot: externalIdentityClaims(identity),
	})
	if err != nil {
		return nil, err
	}
	return externalIdentityFromSession(updatedSession)
}

func buildExternalAuthorizeURL(cfg externalOAuthConfig, state string, codeChallenge string, redirectURI string) (string, error) {
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

func exchangeExternalOAuthCode(
	ctx context.Context,
	cfg externalOAuthConfig,
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
		return nil, &linuxDoTokenExchangeError{StatusCode: resp.StatusCode, Body: body}
	}
	if strings.TrimSpace(tokenResp.TokenType) == "" {
		tokenResp.TokenType = "Bearer"
	}
	return tokenResp, nil
}

func fetchGoogleOAuthIdentity(ctx context.Context, cfg externalOAuthConfig, token *linuxDoTokenResponse) (*externalOAuthIdentity, error) {
	authorization, err := buildBearerAuthorization(token.TokenType, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token for userinfo request: %w", err)
	}
	resp, err := req.C().SetTimeout(30*time.Second).
		R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", authorization).
		Get(cfg.UserInfoURL)
	if err != nil {
		return nil, fmt.Errorf("request userinfo: %w", err)
	}
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("userinfo status=%d", resp.StatusCode)
	}
	body := resp.String()
	subject := firstNonEmpty(getGJSON(body, cfg.UserInfoIDPath), getGJSON(body, "sub"), getGJSON(body, "id"))
	if !isSafeExternalSubject(subject) {
		return nil, errors.New("userinfo returned invalid subject")
	}
	email := strings.TrimSpace(firstNonEmpty(getGJSON(body, cfg.UserInfoEmailPath), getGJSON(body, "email")))
	if email == "" {
		return nil, errors.New("userinfo missing email")
	}
	emailVerified := gjson.Get(body, "email_verified").Bool()
	if cfg.RequireEmailVerified && !emailVerified {
		return nil, errors.New("provider email is not verified")
	}
	username := firstNonEmpty(
		getGJSON(body, cfg.UserInfoUsernamePath),
		getGJSON(body, "name"),
		getGJSON(body, "preferred_username"),
		getGJSON(body, "given_name"),
	)
	if username == "" {
		username = email
	}
	return &externalOAuthIdentity{
		Email:         email,
		Username:      username,
		Subject:       strings.TrimSpace(subject),
		RawProfile:    jsonObjectMap([]byte(body)),
		AvatarURL:     strings.TrimSpace(getGJSON(body, "picture")),
		EmailVerified: emailVerified,
	}, nil
}

func fetchGitHubOAuthIdentity(ctx context.Context, cfg externalOAuthConfig, token *linuxDoTokenResponse) (*externalOAuthIdentity, error) {
	authorization, err := buildBearerAuthorization(token.TokenType, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token for github request: %w", err)
	}

	client := req.C().SetTimeout(30 * time.Second)
	userResp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/vnd.github+json").
		SetHeader("Authorization", authorization).
		SetHeader("X-GitHub-Api-Version", "2022-11-28").
		Get(cfg.UserInfoURL)
	if err != nil {
		return nil, fmt.Errorf("request github user: %w", err)
	}
	if !userResp.IsSuccessState() {
		return nil, fmt.Errorf("github user status=%d", userResp.StatusCode)
	}

	var ghUser struct {
		ID        int64   `json:"id"`
		Login     string  `json:"login"`
		Name      string  `json:"name"`
		Email     *string `json:"email"`
		AvatarURL string  `json:"avatar_url"`
	}
	if err := json.Unmarshal(userResp.Bytes(), &ghUser); err != nil {
		return nil, fmt.Errorf("parse github user: %w", err)
	}
	subject := fmt.Sprintf("%d", ghUser.ID)
	if !isSafeExternalSubject(subject) {
		return nil, errors.New("github user missing id")
	}

	email, err := fetchGitHubPrimaryVerifiedEmail(ctx, client, cfg, authorization)
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(ghUser.Name)
	if username == "" {
		username = strings.TrimSpace(ghUser.Login)
	}
	if username == "" {
		username = email
	}

	raw := map[string]any{
		"user":   jsonObjectMap(userResp.Bytes()),
		"emails": nil,
	}
	return &externalOAuthIdentity{
		Email:         email,
		Username:      username,
		Subject:       subject,
		RawProfile:    raw,
		AvatarURL:     strings.TrimSpace(ghUser.AvatarURL),
		EmailVerified: true,
	}, nil
}

func fetchGitHubPrimaryVerifiedEmail(ctx context.Context, client *req.Client, cfg externalOAuthConfig, authorization string) (string, error) {
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/vnd.github+json").
		SetHeader("Authorization", authorization).
		SetHeader("X-GitHub-Api-Version", "2022-11-28").
		Get(cfg.EmailsURL)
	if err != nil {
		return "", fmt.Errorf("request github emails: %w", err)
	}
	if !resp.IsSuccessState() {
		return "", fmt.Errorf("github emails status=%d", resp.StatusCode)
	}
	var emails []struct {
		Email      string `json:"email"`
		Primary    bool   `json:"primary"`
		Verified   bool   `json:"verified"`
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(resp.Bytes(), &emails); err != nil {
		return "", fmt.Errorf("parse github emails: %w", err)
	}
	for _, item := range emails {
		if item.Primary && item.Verified && strings.TrimSpace(item.Email) != "" {
			return strings.TrimSpace(item.Email), nil
		}
	}
	for _, item := range emails {
		if item.Verified && strings.TrimSpace(item.Email) != "" {
			return strings.TrimSpace(item.Email), nil
		}
	}
	return "", errors.New("github account has no verified email")
}

func externalIdentityFromSession(session *service.PendingAuthSession) (*externalOAuthIdentity, error) {
	if session == nil || len(session.ClaimsSnapshot) == 0 {
		return nil, errors.New("missing pending auth claims")
	}
	identity := &externalOAuthIdentity{
		Email:         strings.TrimSpace(stringClaim(session.ClaimsSnapshot, "email")),
		Username:      strings.TrimSpace(stringClaim(session.ClaimsSnapshot, "username")),
		Subject:       strings.TrimSpace(stringClaim(session.ClaimsSnapshot, "subject")),
		AvatarURL:     strings.TrimSpace(stringClaim(session.ClaimsSnapshot, "avatar_url")),
		EmailVerified: boolClaim(session.ClaimsSnapshot, "email_verified"),
		RawProfile:    oauthIdentityClaimsOnly(session.ClaimsSnapshot),
	}
	if identity.Email == "" || identity.Username == "" || identity.Subject == "" {
		return nil, errors.New("incomplete pending auth claims")
	}
	return identity, nil
}

func externalIdentityClaims(identity *externalOAuthIdentity) map[string]any {
	if identity == nil {
		return map[string]any{}
	}
	claims := map[string]any{}
	for k, v := range identity.RawProfile {
		claims[k] = v
	}
	claims["email"] = strings.TrimSpace(identity.Email)
	claims["username"] = strings.TrimSpace(identity.Username)
	claims["subject"] = strings.TrimSpace(identity.Subject)
	claims["avatar_url"] = strings.TrimSpace(identity.AvatarURL)
	claims["email_verified"] = identity.EmailVerified
	return claims
}

func boolClaim(claims map[string]any, key string) bool {
	value, ok := claims[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return fmt.Sprint(v) == "true"
	}
}

func jsonObjectMap(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func isSafeExternalSubject(subject string) bool {
	subject = strings.TrimSpace(subject)
	if subject == "" || len(subject) > externalOAuthMaxSubjectLen {
		return false
	}
	return !strings.ContainsAny(subject, "\r\n\t")
}

func externalOAuthVerifierCookie(provider string) string {
	return strings.TrimSpace(provider) + "_oauth_verifier"
}

func externalOAuthCookiePath(provider string) string {
	return "/api/v1/auth/oauth/" + strings.TrimSpace(provider)
}

func defaultExternalFrontendCallback(provider string) string {
	switch provider {
	case service.AuthProviderGoogle:
		return googleOAuthDefaultFrontendCB
	case service.AuthProviderGitHub:
		return gitHubOAuthDefaultFrontendCB
	default:
		return linuxDoOAuthDefaultFrontendCB
	}
}
