package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials      = infraerrors.Unauthorized("INVALID_CREDENTIALS", "invalid email or password")
	ErrUserNotActive           = infraerrors.Forbidden("USER_NOT_ACTIVE", "user is not active")
	ErrEmailExists             = infraerrors.Conflict("EMAIL_EXISTS", "email already exists")
	ErrEmailReserved           = infraerrors.BadRequest("EMAIL_RESERVED", "email is reserved")
	ErrInvalidToken            = infraerrors.Unauthorized("INVALID_TOKEN", "invalid token")
	ErrTokenExpired            = infraerrors.Unauthorized("TOKEN_EXPIRED", "token has expired")
	ErrAccessTokenExpired      = infraerrors.Unauthorized("ACCESS_TOKEN_EXPIRED", "access token has expired")
	ErrTokenTooLarge           = infraerrors.BadRequest("TOKEN_TOO_LARGE", "token too large")
	ErrTokenRevoked            = infraerrors.Unauthorized("TOKEN_REVOKED", "token has been revoked")
	ErrRefreshTokenInvalid     = infraerrors.Unauthorized("REFRESH_TOKEN_INVALID", "invalid refresh token")
	ErrRefreshTokenExpired     = infraerrors.Unauthorized("REFRESH_TOKEN_EXPIRED", "refresh token has expired")
	ErrRefreshTokenReused      = infraerrors.Unauthorized("REFRESH_TOKEN_REUSED", "refresh token has been reused")
	ErrEmailVerifyRequired     = infraerrors.BadRequest("EMAIL_VERIFY_REQUIRED", "email verification is required")
	ErrEmailSuffixNotAllowed   = infraerrors.BadRequest("EMAIL_SUFFIX_NOT_ALLOWED", "email suffix is not allowed")
	ErrRegDisabled             = infraerrors.Forbidden("REGISTRATION_DISABLED", "registration is currently disabled")
	ErrServiceUnavailable      = infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "service temporarily unavailable")
	ErrInvitationCodeRequired  = infraerrors.BadRequest("INVITATION_CODE_REQUIRED", "invitation code is required")
	ErrInvitationCodeInvalid   = infraerrors.BadRequest("INVITATION_CODE_INVALID", "invalid or used invitation code")
	ErrOAuthInvitationRequired = infraerrors.Forbidden("OAUTH_INVITATION_REQUIRED", "invitation code required to complete oauth registration")
	ErrOAuthEmailOwnership     = infraerrors.Forbidden("OAUTH_EMAIL_OWNERSHIP_REQUIRED", "existing account requires a verified provider email or authenticated identity binding")
	ErrInvalidSSOTicket        = infraerrors.Unauthorized("INVALID_SSO_TICKET", "invalid or expired sso ticket")
	ErrInvalidEmbedTicket      = infraerrors.Unauthorized("INVALID_EMBED_TICKET", "invalid or expired embed ticket")
	ErrEmbedTargetUnavailable  = infraerrors.Forbidden("EMBED_TARGET_UNAVAILABLE", "embed target is unavailable")
)

// maxTokenLength 限制 token 大小，避免超长 header 触发解析时的异常内存分配。
const maxTokenLength = 8192

// refreshTokenPrefix is the prefix for refresh tokens to distinguish them from access tokens.
const refreshTokenPrefix = "rt_"

const ssoTicketTTL = 60 * time.Second

const (
	ssoTicketPurpose          = "chatbot_sso"
	embedTicketPurpose        = "embed"
	embedSessionPurpose       = "embed_session"
	embedSessionTTL           = 5 * time.Minute
	EmbedTargetKindPurchase   = "purchase_subscription"
	EmbedTargetKindCustomMenu = "custom_menu"
	EmbedDeliveryIframe       = "iframe"
	EmbedDeliveryNewTab       = "new_tab"
)

// JWTClaims JWT载荷数据
type JWTClaims struct {
	UserID       int64  `json:"user_id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	TokenVersion int64  `json:"token_version"` // Used to invalidate tokens on password change
	jwt.RegisteredClaims
}

// AuthService 认证服务
type AuthService struct {
	entClient           *dbent.Client
	userRepo            UserRepository
	redeemRepo          RedeemCodeRepository
	refreshTokenCache   RefreshTokenCache
	ssoTicketCache      SSOTicketCache
	cfg                 *config.Config
	settingService      *SettingService
	emailService        *EmailService
	turnstileService    *TurnstileService
	emailQueueService   *EmailQueueService
	promoService        *PromoService
	defaultSubAssigner  DefaultSubscriptionAssigner
	commissionService   *CommissionService
	embedTargetResolver EmbedTargetResolver
	unitOfWork          UnitOfWork
}

// SetUnitOfWork injects the transaction boundary used by registration flows.
func (s *AuthService) SetUnitOfWork(unitOfWork UnitOfWork) {
	s.unitOfWork = unitOfWork
}

func (s *AuthService) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s.unitOfWork == nil {
		return fn(ctx)
	}
	return s.unitOfWork.WithinTx(ctx, fn)
}

type EmbedTargetResolver interface {
	ResolveEmbedTarget(ctx context.Context, kind, targetID, role string) (*EmbedTarget, error)
}

type DefaultSubscriptionAssigner interface {
	AssignOrExtendSubscription(ctx context.Context, input *AssignSubscriptionInput) (*UserSubscription, bool, error)
}

// NewAuthService 创建认证服务实例
func NewAuthService(
	entClient *dbent.Client,
	userRepo UserRepository,
	redeemRepo RedeemCodeRepository,
	refreshTokenCache RefreshTokenCache,
	ssoTicketCache SSOTicketCache,
	cfg *config.Config,
	settingService *SettingService,
	emailService *EmailService,
	turnstileService *TurnstileService,
	emailQueueService *EmailQueueService,
	promoService *PromoService,
	defaultSubAssigner DefaultSubscriptionAssigner,
	commissionService *CommissionService,
) *AuthService {
	return &AuthService{
		entClient:           entClient,
		userRepo:            userRepo,
		redeemRepo:          redeemRepo,
		refreshTokenCache:   refreshTokenCache,
		ssoTicketCache:      ssoTicketCache,
		cfg:                 cfg,
		settingService:      settingService,
		emailService:        emailService,
		turnstileService:    turnstileService,
		emailQueueService:   emailQueueService,
		promoService:        promoService,
		defaultSubAssigner:  defaultSubAssigner,
		commissionService:   commissionService,
		embedTargetResolver: settingService,
	}
}

// Register 用户注册，返回token和用户
func (s *AuthService) Register(ctx context.Context, email, password string) (string, *User, error) {
	return s.RegisterWithVerification(ctx, email, password, "", "", "", "")
}

// RegisterWithVerification 用户注册（支持邮件验证、优惠码、门控邀请码和分佣邀请码），返回token和用户
// referralCode: 分佣邀请码（可选），与 invitationCode（门控注册码）语义不同
func (s *AuthService) RegisterWithVerification(ctx context.Context, email, password, verifyCode, promoCode, invitationCode, referralCode string) (string, *User, error) {
	// 检查是否开放注册（默认关闭：settingService 未配置时不允许注册）
	if s.settingService == nil || !s.settingService.IsRegistrationEnabled(ctx) {
		return "", nil, ErrRegDisabled
	}

	// 防止用户注册 LinuxDo OAuth 合成邮箱，避免第三方登录与本地账号发生碰撞。
	if isReservedEmail(email) {
		return "", nil, ErrEmailReserved
	}
	if err := s.validateRegistrationEmailPolicy(ctx, email); err != nil {
		return "", nil, err
	}

	// 检查是否需要邀请码
	var invitationRedeemCode *RedeemCode
	if s.settingService != nil && s.settingService.IsInvitationCodeEnabled(ctx) {
		if invitationCode == "" {
			return "", nil, ErrInvitationCodeRequired
		}
		// 验证邀请码
		redeemCode, err := s.redeemRepo.GetByCode(ctx, invitationCode)
		if err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Invalid invitation code: %s, error: %v", invitationCode, err)
			return "", nil, ErrInvitationCodeInvalid
		}
		// 检查类型和状态
		if redeemCode.Type != RedeemTypeInvitation || redeemCode.Status != StatusUnused {
			logger.LegacyPrintf("service.auth", "[Auth] Invitation code invalid: type=%s, status=%s", redeemCode.Type, redeemCode.Status)
			return "", nil, ErrInvitationCodeInvalid
		}
		invitationRedeemCode = redeemCode
	}

	// 检查是否需要邮件验证
	if s.settingService != nil && s.settingService.IsEmailVerifyEnabled(ctx) {
		// 如果邮件验证已开启但邮件服务未配置，拒绝注册
		// 这是一个配置错误，不应该允许绕过验证
		if s.emailService == nil {
			logger.LegacyPrintf("service.auth", "%s", "[Auth] Email verification enabled but email service not configured, rejecting registration")
			return "", nil, ErrServiceUnavailable
		}
		if verifyCode == "" {
			return "", nil, ErrEmailVerifyRequired
		}
		// 验证邮箱验证码
		if err := s.emailService.VerifyCode(ctx, email, verifyCode); err != nil {
			return "", nil, fmt.Errorf("verify code: %w", err)
		}
	}

	// 检查邮箱是否已存在
	existsEmail, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Database error checking email exists: %v", err)
		return "", nil, ErrServiceUnavailable
	}
	if existsEmail {
		return "", nil, ErrEmailExists
	}
	aliasExists, err := emailAliasExistsForAnotherUser(ctx, s.userRepo, email, 0)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Database error checking email alias exists: %v", err)
		return "", nil, ErrServiceUnavailable
	}
	if aliasExists {
		return "", nil, ErrEmailExists
	}

	// 密码哈希
	hashedPassword, err := s.HashPassword(password)
	if err != nil {
		return "", nil, fmt.Errorf("hash password: %w", err)
	}

	// 获取默认配置
	defaultBalance := s.cfg.Default.UserBalance
	defaultConcurrency := s.cfg.Default.UserConcurrency
	if s.settingService != nil {
		defaultBalance = s.settingService.GetDefaultBalance(ctx)
		defaultConcurrency = s.settingService.GetDefaultConcurrency(ctx)
	}

	// 创建用户
	user := &User{
		Email:        email,
		PasswordHash: hashedPassword,
		Role:         RoleUser,
		Balance:      defaultBalance,
		Concurrency:  defaultConcurrency,
		Status:       StatusActive,
	}

	err = s.withinTx(ctx, func(txCtx context.Context) error {
		if createErr := s.userRepo.Create(txCtx, user); createErr != nil {
			return createErr
		}
		if invitationRedeemCode != nil {
			if useErr := s.redeemRepo.Use(txCtx, invitationRedeemCode.ID, user.ID); useErr != nil {
				return ErrInvitationCodeInvalid.WithCause(useErr)
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrEmailExists) {
			return "", nil, ErrEmailExists
		}
		if errors.Is(err, ErrInvitationCodeInvalid) {
			return "", nil, ErrInvitationCodeInvalid
		}
		logger.LegacyPrintf("service.auth", "[Auth] Database error creating user: %v", err)
		return "", nil, ErrServiceUnavailable
	}

	// These side effects are deliberately after commit: no cache invalidation,
	// subscription grant, referral, or notification may escape a rolled-back
	// registration transaction.
	s.assignDefaultSubscriptions(ctx, user.ID)
	// 应用优惠码（如果提供且功能已启用）
	if promoCode != "" && s.promoService != nil && s.settingService != nil && s.settingService.IsPromoCodeEnabled(ctx) {
		if err := s.promoService.ApplyPromoCode(ctx, user.ID, promoCode); err != nil {
			// 优惠码应用失败不影响注册，只记录日志
			logger.LegacyPrintf("service.auth", "[Auth] Failed to apply promo code for user %d: %v", user.ID, err)
		} else {
			// 重新获取用户信息以获取更新后的余额
			if updatedUser, err := s.userRepo.GetByID(ctx, user.ID); err == nil {
				user = updatedUser
			}
		}
	}

	s.bindReferralCode(ctx, user.ID, referralCode)
	s.applyInviteActivityRegistrationBonus(ctx, user.ID)

	// 生成token
	token, err := s.GenerateToken(user)
	if err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	_ = s.userRepo.TouchLastActive(ctx, user.ID, time.Now())

	return token, user, nil
}

// SendVerifyCodeResult 发送验证码返回结果
type SendVerifyCodeResult struct {
	Countdown int `json:"countdown"` // 倒计时秒数
}

// SendVerifyCode 发送邮箱验证码（同步方式）
func (s *AuthService) SendVerifyCode(ctx context.Context, email string) error {
	// 检查是否开放注册（默认关闭）
	if s.settingService == nil || !s.settingService.IsRegistrationEnabled(ctx) {
		return ErrRegDisabled
	}

	if isReservedEmail(email) {
		return ErrEmailReserved
	}
	if err := s.validateRegistrationEmailPolicy(ctx, email); err != nil {
		return err
	}

	// 检查邮箱是否已存在
	existsEmail, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Database error checking email exists: %v", err)
		return ErrServiceUnavailable
	}
	if existsEmail {
		return ErrEmailExists
	}
	aliasExists, err := emailAliasExistsForAnotherUser(ctx, s.userRepo, email, 0)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Database error checking email alias exists: %v", err)
		return ErrServiceUnavailable
	}
	if aliasExists {
		return ErrEmailExists
	}

	// 发送验证码
	if s.emailService == nil {
		return errors.New("email service not configured")
	}

	// 获取网站名称
	siteName := "Sub2API"
	if s.settingService != nil {
		siteName = s.settingService.GetSiteName(ctx)
	}

	return s.emailService.SendVerifyCode(ctx, email, siteName)
}

// SendVerifyCodeAsync 异步发送邮箱验证码并返回倒计时
func (s *AuthService) SendVerifyCodeAsync(ctx context.Context, email string) (*SendVerifyCodeResult, error) {
	logger.LegacyPrintf("service.auth", "[Auth] SendVerifyCodeAsync called for email: %s", email)

	// 检查是否开放注册（默认关闭）
	if s.settingService == nil || !s.settingService.IsRegistrationEnabled(ctx) {
		logger.LegacyPrintf("service.auth", "%s", "[Auth] Registration is disabled")
		return nil, ErrRegDisabled
	}

	if isReservedEmail(email) {
		return nil, ErrEmailReserved
	}
	if err := s.validateRegistrationEmailPolicy(ctx, email); err != nil {
		return nil, err
	}

	// 检查邮箱是否已存在
	existsEmail, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Database error checking email exists: %v", err)
		return nil, ErrServiceUnavailable
	}
	if existsEmail {
		logger.LegacyPrintf("service.auth", "[Auth] Email already exists: %s", email)
		return nil, ErrEmailExists
	}
	aliasExists, err := emailAliasExistsForAnotherUser(ctx, s.userRepo, email, 0)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Database error checking email alias exists: %v", err)
		return nil, ErrServiceUnavailable
	}
	if aliasExists {
		return nil, ErrEmailExists
	}

	// 检查邮件队列服务是否配置
	if s.emailQueueService == nil {
		logger.LegacyPrintf("service.auth", "%s", "[Auth] Email queue service not configured")
		return nil, errors.New("email queue service not configured")
	}

	// 获取网站名称
	siteName := "Sub2API"
	if s.settingService != nil {
		siteName = s.settingService.GetSiteName(ctx)
	}

	// 异步发送
	logger.LegacyPrintf("service.auth", "[Auth] Enqueueing verify code for: %s", email)
	if err := s.emailQueueService.EnqueueVerifyCode(email, siteName); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to enqueue: %v", err)
		return nil, fmt.Errorf("enqueue verify code: %w", err)
	}

	logger.LegacyPrintf("service.auth", "[Auth] Verify code enqueued successfully for: %s", email)
	return &SendVerifyCodeResult{
		Countdown: 60, // 60秒倒计时
	}, nil
}

// VerifyTurnstileForRegister 在注册场景下验证 Turnstile。
// 当邮箱验证开启且已提交验证码时，说明验证码发送阶段已完成 Turnstile 校验，
// 此处跳过二次校验，避免一次性 token 在注册提交时重复使用导致误报失败。
func (s *AuthService) VerifyTurnstileForRegister(ctx context.Context, token, remoteIP, verifyCode string) error {
	if s.IsEmailVerifyEnabled(ctx) && strings.TrimSpace(verifyCode) != "" {
		logger.LegacyPrintf("service.auth", "%s", "[Auth] Email verify flow detected, skip duplicate Turnstile check on register")
		return nil
	}
	return s.VerifyTurnstile(ctx, token, remoteIP)
}

// VerifyTurnstile 验证Turnstile token
func (s *AuthService) VerifyTurnstile(ctx context.Context, token string, remoteIP string) error {
	required := s.cfg != nil && s.cfg.Server.Mode == "release" && s.cfg.Turnstile.Required

	if required {
		if s.settingService == nil {
			logger.LegacyPrintf("service.auth", "%s", "[Auth] Turnstile required but settings service is not configured")
			return ErrTurnstileNotConfigured
		}
		enabled := s.settingService.IsTurnstileEnabled(ctx)
		secretConfigured := s.settingService.GetTurnstileSecretKey(ctx) != ""
		if !enabled || !secretConfigured {
			logger.LegacyPrintf("service.auth", "[Auth] Turnstile required but not configured (enabled=%v, secret_configured=%v)", enabled, secretConfigured)
			return ErrTurnstileNotConfigured
		}
	}

	if s.turnstileService == nil {
		if required {
			logger.LegacyPrintf("service.auth", "%s", "[Auth] Turnstile required but service not configured")
			return ErrTurnstileNotConfigured
		}
		return nil // 服务未配置则跳过验证
	}

	if !required && s.settingService != nil && s.settingService.IsTurnstileEnabled(ctx) && s.settingService.GetTurnstileSecretKey(ctx) == "" {
		logger.LegacyPrintf("service.auth", "%s", "[Auth] Turnstile enabled but secret key not configured")
	}

	return s.turnstileService.VerifyToken(ctx, token, remoteIP)
}

// IsTurnstileEnabled 检查是否启用Turnstile验证
func (s *AuthService) IsTurnstileEnabled(ctx context.Context) bool {
	if s.turnstileService == nil {
		return false
	}
	return s.turnstileService.IsEnabled(ctx)
}

// IsRegistrationEnabled 检查是否开放注册
func (s *AuthService) IsRegistrationEnabled(ctx context.Context) bool {
	if s.settingService == nil {
		return false // 安全默认：settingService 未配置时关闭注册
	}
	return s.settingService.IsRegistrationEnabled(ctx)
}

// IsEmailVerifyEnabled 检查是否开启邮件验证
func (s *AuthService) IsEmailVerifyEnabled(ctx context.Context) bool {
	if s.settingService == nil {
		return false
	}
	return s.settingService.IsEmailVerifyEnabled(ctx)
}

// Login 用户登录，返回JWT token
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *User, error) {
	// 查找用户
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", nil, ErrInvalidCredentials
		}
		// 记录数据库错误但不暴露给用户
		logger.LegacyPrintf("service.auth", "[Auth] Database error during login: %v", err)
		return "", nil, ErrServiceUnavailable
	}

	// 验证密码
	if !s.CheckPassword(password, user.PasswordHash) {
		return "", nil, ErrInvalidCredentials
	}

	// 检查用户状态
	if !user.IsActive() {
		return "", nil, ErrUserNotActive
	}

	// 生成JWT token
	token, err := s.GenerateToken(user)
	if err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}

	return token, user, nil
}

// LoginOrRegisterOAuth 用于第三方 OAuth/SSO 登录：
// - 如果邮箱已存在：直接登录（不需要本地密码）
// - 如果邮箱不存在：创建新用户并登录
//
// 注意：该函数用于 LinuxDo OAuth 登录场景（不同于上游账号的 OAuth，例如 Claude/OpenAI/Gemini）。
// 为了满足现有数据库约束（需要密码哈希），新用户会生成随机密码并进行哈希保存。
func (s *AuthService) LoginOrRegisterOAuth(ctx context.Context, email, username string) (string, *User, error) {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 255 {
		return "", nil, infraerrors.BadRequest("INVALID_EMAIL", "invalid email")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", nil, infraerrors.BadRequest("INVALID_EMAIL", "invalid email")
	}

	username = strings.TrimSpace(username)
	if len([]rune(username)) > 100 {
		username = string([]rune(username)[:100])
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// OAuth 首次登录视为注册（fail-close：settingService 未配置时不允许注册）
			if s.settingService == nil || !s.settingService.IsRegistrationEnabled(ctx) {
				return "", nil, ErrRegDisabled
			}
			if err := s.validateRegistrationEmailPolicy(ctx, email); err != nil {
				return "", nil, err
			}

			randomPassword, err := randomHexString(32)
			if err != nil {
				logger.LegacyPrintf("service.auth", "[Auth] Failed to generate random password for oauth signup: %v", err)
				return "", nil, ErrServiceUnavailable
			}
			hashedPassword, err := s.HashPassword(randomPassword)
			if err != nil {
				return "", nil, fmt.Errorf("hash password: %w", err)
			}

			// 新用户默认值。
			defaultBalance := s.cfg.Default.UserBalance
			defaultConcurrency := s.cfg.Default.UserConcurrency
			if s.settingService != nil {
				defaultBalance = s.settingService.GetDefaultBalance(ctx)
				defaultConcurrency = s.settingService.GetDefaultConcurrency(ctx)
			}

			newUser := &User{
				Email:        email,
				Username:     username,
				PasswordHash: hashedPassword,
				Role:         RoleUser,
				Balance:      defaultBalance,
				Concurrency:  defaultConcurrency,
				Status:       StatusActive,
			}

			if err := s.userRepo.Create(ctx, newUser); err != nil {
				if errors.Is(err, ErrEmailExists) {
					// 并发场景：GetByEmail 与 Create 之间用户被创建。
					user, err = s.userRepo.GetByEmail(ctx, email)
					if err != nil {
						logger.LegacyPrintf("service.auth", "[Auth] Database error getting user after conflict: %v", err)
						return "", nil, ErrServiceUnavailable
					}
				} else {
					logger.LegacyPrintf("service.auth", "[Auth] Database error creating oauth user: %v", err)
					return "", nil, ErrServiceUnavailable
				}
			} else {
				user = newUser
				s.assignDefaultSubscriptions(ctx, user.ID)
			}
		} else {
			logger.LegacyPrintf("service.auth", "[Auth] Database error during oauth login: %v", err)
			return "", nil, ErrServiceUnavailable
		}
	}

	if !user.IsActive() {
		return "", nil, ErrUserNotActive
	}

	// 尽力补全：当用户名为空时，使用第三方返回的用户名回填。
	if user.Username == "" && username != "" {
		user.Username = username
		if err := s.userRepo.Update(ctx, user); err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Failed to update username after oauth login: %v", err)
		}
	}

	token, err := s.GenerateToken(user)
	if err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	_ = s.userRepo.TouchLastActive(ctx, user.ID, time.Now())
	return token, user, nil
}

// LoginOrRegisterOAuthWithTokenPair 用于第三方 OAuth/SSO 登录，返回完整的 TokenPair。
// 与 LoginOrRegisterOAuth 功能相同，但返回 TokenPair 而非单个 token。
// invitationCode 仅在邀请码注册模式下新用户注册时使用；referralCode 仅在首次创建 OAuth 用户时绑定邀请关系。
func (s *AuthService) LoginOrRegisterOAuthWithTokenPair(ctx context.Context, email, username, invitationCode, referralCode string) (*TokenPair, *User, error) {
	return s.loginOrRegisterOAuthWithTokenPair(ctx, email, username, invitationCode, referralCode, true)
}

// LoginOrRegisterOAuthIdentityWithTokenPair preserves the provider's email
// verification boundary. An unverified provider claim may create a new account,
// but it must never take over an existing local account solely by matching its
// email address; existing accounts require a verified claim or an authenticated
// identity-binding flow.
func (s *AuthService) LoginOrRegisterOAuthIdentityWithTokenPair(ctx context.Context, email, username, invitationCode, referralCode string, emailVerified bool) (*TokenPair, *User, error) {
	return s.loginOrRegisterOAuthWithTokenPair(ctx, email, username, invitationCode, referralCode, emailVerified)
}

func (s *AuthService) loginOrRegisterOAuthWithTokenPair(ctx context.Context, email, username, invitationCode, referralCode string, emailVerified bool) (*TokenPair, *User, error) {
	// 检查 refreshTokenCache 是否可用
	if s.refreshTokenCache == nil {
		return nil, nil, errors.New("refresh token cache not configured")
	}

	email = strings.TrimSpace(email)
	if email == "" || len(email) > 255 {
		return nil, nil, infraerrors.BadRequest("INVALID_EMAIL", "invalid email")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, nil, infraerrors.BadRequest("INVALID_EMAIL", "invalid email")
	}

	username = strings.TrimSpace(username)
	if len([]rune(username)) > 100 {
		username = string([]rune(username)[:100])
	}
	referralCode = strings.TrimSpace(referralCode)
	createdNewUser := false

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil && !emailVerified {
		return nil, nil, ErrOAuthEmailOwnership
	}
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// OAuth 首次登录视为注册
			if s.settingService == nil || !s.settingService.IsRegistrationEnabled(ctx) {
				return nil, nil, ErrRegDisabled
			}
			if err := s.validateRegistrationEmailPolicy(ctx, email); err != nil {
				return nil, nil, err
			}

			// 检查是否需要邀请码
			var invitationRedeemCode *RedeemCode
			if s.settingService != nil && s.settingService.IsInvitationCodeEnabled(ctx) {
				if invitationCode == "" {
					return nil, nil, ErrOAuthInvitationRequired
				}
				redeemCode, err := s.redeemRepo.GetByCode(ctx, invitationCode)
				if err != nil {
					return nil, nil, ErrInvitationCodeInvalid
				}
				if redeemCode.Type != RedeemTypeInvitation || redeemCode.Status != StatusUnused {
					return nil, nil, ErrInvitationCodeInvalid
				}
				invitationRedeemCode = redeemCode
			}

			randomPassword, err := randomHexString(32)
			if err != nil {
				logger.LegacyPrintf("service.auth", "[Auth] Failed to generate random password for oauth signup: %v", err)
				return nil, nil, ErrServiceUnavailable
			}
			hashedPassword, err := s.HashPassword(randomPassword)
			if err != nil {
				return nil, nil, fmt.Errorf("hash password: %w", err)
			}

			defaultBalance := s.cfg.Default.UserBalance
			defaultConcurrency := s.cfg.Default.UserConcurrency
			if s.settingService != nil {
				defaultBalance = s.settingService.GetDefaultBalance(ctx)
				defaultConcurrency = s.settingService.GetDefaultConcurrency(ctx)
			}

			newUser := &User{
				Email:        email,
				Username:     username,
				PasswordHash: hashedPassword,
				Role:         RoleUser,
				Balance:      defaultBalance,
				Concurrency:  defaultConcurrency,
				Status:       StatusActive,
			}

			createErr := s.withinTx(ctx, func(txCtx context.Context) error {
				if err := s.userRepo.Create(txCtx, newUser); err != nil {
					return err
				}
				if invitationRedeemCode != nil {
					if err := s.redeemRepo.Use(txCtx, invitationRedeemCode.ID, newUser.ID); err != nil {
						return ErrInvitationCodeInvalid.WithCause(err)
					}
				}
				return nil
			})
			if createErr != nil {
				if errors.Is(createErr, ErrEmailExists) {
					if !emailVerified {
						return nil, nil, ErrOAuthEmailOwnership
					}
					// The UnitOfWork has rolled back before reading the winner.
					user, err = s.userRepo.GetByEmail(ctx, email)
					if err != nil {
						logger.LegacyPrintf("service.auth", "[Auth] Database error getting user after conflict: %v", err)
						return nil, nil, ErrServiceUnavailable
					}
				} else if errors.Is(createErr, ErrInvitationCodeInvalid) {
					return nil, nil, ErrInvitationCodeInvalid
				} else {
					logger.LegacyPrintf("service.auth", "[Auth] Database error creating oauth user: %v", createErr)
					return nil, nil, ErrServiceUnavailable
				}
			} else {
				user = newUser
				createdNewUser = true
			}
		} else {
			logger.LegacyPrintf("service.auth", "[Auth] Database error during oauth login: %v", err)
			return nil, nil, ErrServiceUnavailable
		}
	}

	if !user.IsActive() {
		return nil, nil, ErrUserNotActive
	}

	if user.Username == "" && username != "" {
		user.Username = username
		if err := s.userRepo.Update(ctx, user); err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Failed to update username after oauth login: %v", err)
		}
	}
	if createdNewUser {
		s.assignDefaultSubscriptions(ctx, user.ID)
		s.bindReferralCode(ctx, user.ID, referralCode)
		s.applyInviteActivityRegistrationBonus(ctx, user.ID)
	}

	tokenPair, err := s.GenerateTokenPair(ctx, user, "")
	if err != nil {
		return nil, nil, fmt.Errorf("generate token pair: %w", err)
	}
	_ = s.userRepo.TouchLastActive(ctx, user.ID, time.Now())
	return tokenPair, user, nil
}

// ShouldPromptOAuthReferral returns true when an OAuth login would create a new
// user and no referral code was already captured in the OAuth state.
func (s *AuthService) ShouldPromptOAuthReferral(ctx context.Context, email, referralCode string) (bool, error) {
	if strings.TrimSpace(referralCode) != "" {
		return false, nil
	}
	if s == nil || s.userRepo == nil {
		return false, ErrServiceUnavailable
	}
	if s.settingService == nil || !s.settingService.IsRegistrationEnabled(ctx) {
		return false, nil
	}

	email = strings.TrimSpace(email)
	if email == "" {
		return false, infraerrors.BadRequest("INVALID_EMAIL", "invalid email")
	}
	if err := s.validateRegistrationEmailPolicy(ctx, email); err != nil {
		return false, nil
	}
	_, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, ErrUserNotFound) {
		return true, nil
	}
	logger.LegacyPrintf("service.auth", "[Auth] Database error checking oauth referral prompt: %v", err)
	return false, ErrServiceUnavailable
}

// bindReferralCode 绑定分佣邀请关系（与门控邀请码 invitationCode 完全独立）。
func (s *AuthService) bindReferralCode(ctx context.Context, userID int64, referralCode string) {
	referralCode = strings.TrimSpace(referralCode)
	if referralCode == "" || s.commissionService == nil {
		return
	}
	if err := s.commissionService.BindReferralCode(ctx, userID, referralCode); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to bind referral code for user %d: %v", userID, err)
	}
}

func (s *AuthService) applyInviteActivityRegistrationBonus(ctx context.Context, userID int64) {
	if s.commissionService == nil {
		return
	}
	if err := s.commissionService.ProcessInviteActivityRegistrationBonus(ctx, userID); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to apply invite activity registration bonus for user %d: %v", userID, err)
	}
}

// pendingOAuthTokenTTL is the validity period for pending OAuth tokens.
const pendingOAuthTokenTTL = 10 * time.Minute

// pendingOAuthPurpose is the purpose claim value for pending OAuth registration tokens.
const pendingOAuthPurpose = "pending_oauth_registration"

type pendingOAuthClaims struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Purpose  string `json:"purpose"`
	jwt.RegisteredClaims
}

// CreatePendingOAuthToken generates a short-lived JWT that carries the OAuth identity
// while waiting for the user to supply an invitation code.
func (s *AuthService) CreatePendingOAuthToken(email, username string) (string, error) {
	now := time.Now()
	claims := &pendingOAuthClaims{
		Email:    email,
		Username: username,
		Purpose:  pendingOAuthPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(pendingOAuthTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWT.Secret))
}

// VerifyPendingOAuthToken validates a pending OAuth token and returns the embedded identity.
// Returns ErrInvalidToken when the token is invalid or expired.
func (s *AuthService) VerifyPendingOAuthToken(tokenStr string) (email, username string, err error) {
	if len(tokenStr) > maxTokenLength {
		return "", "", ErrInvalidToken
	}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	token, parseErr := parser.ParseWithClaims(tokenStr, &pendingOAuthClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWT.Secret), nil
	})
	if parseErr != nil {
		return "", "", ErrInvalidToken
	}
	claims, ok := token.Claims.(*pendingOAuthClaims)
	if !ok || !token.Valid {
		return "", "", ErrInvalidToken
	}
	if claims.Purpose != pendingOAuthPurpose {
		return "", "", ErrInvalidToken
	}
	return claims.Email, claims.Username, nil
}

func (s *AuthService) assignDefaultSubscriptions(ctx context.Context, userID int64) {
	if s.settingService == nil || s.defaultSubAssigner == nil || userID <= 0 {
		return
	}
	items := s.settingService.GetDefaultSubscriptions(ctx)
	for _, item := range items {
		if _, _, err := s.defaultSubAssigner.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{
			UserID:       userID,
			GroupID:      item.GroupID,
			ValidityDays: item.ValidityDays,
			Notes:        "auto assigned by default user subscriptions setting",
		}); err != nil {
			logger.LegacyPrintf("service.auth", "[Auth] Failed to assign default subscription: user_id=%d group_id=%d err=%v", userID, item.GroupID, err)
		}
	}
}

func (s *AuthService) validateRegistrationEmailPolicy(ctx context.Context, email string) error {
	if s.settingService == nil {
		return nil
	}
	whitelist, restricted := s.registrationEmailSuffixWhitelistForCurrentPolicy(ctx)
	if !restricted {
		return nil
	}
	if !IsRegistrationEmailSuffixAllowed(email, whitelist) {
		return buildEmailSuffixNotAllowedError(whitelist)
	}
	return nil
}

func (s *AuthService) registrationEmailSuffixWhitelistForCurrentPolicy(ctx context.Context) ([]string, bool) {
	whitelist := s.settingService.GetRegistrationEmailSuffixWhitelist(ctx)
	if s.commissionService == nil {
		return whitelist, len(whitelist) > 0
	}
	return s.commissionService.ActiveInviteActivityEmailSuffixWhitelist(ctx, whitelist)
}

func buildEmailSuffixNotAllowedError(whitelist []string) error {
	if len(whitelist) == 0 {
		return ErrEmailSuffixNotAllowed
	}

	allowed := strings.Join(whitelist, ", ")
	return infraerrors.BadRequest(
		"EMAIL_SUFFIX_NOT_ALLOWED",
		fmt.Sprintf("email suffix is not allowed, allowed suffixes: %s", allowed),
	).WithMetadata(map[string]string{
		"allowed_suffixes":     strings.Join(whitelist, ","),
		"allowed_suffix_count": strconv.Itoa(len(whitelist)),
	})
}

// ValidateToken 验证JWT token并返回用户声明
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	// 先做长度校验，尽早拒绝异常超长 token，降低 DoS 风险。
	if len(tokenString) > maxTokenLength {
		return nil, ErrTokenTooLarge
	}

	// 使用解析器并限制可接受的签名算法，防止算法混淆。
	parser := jwt.NewParser(jwt.WithValidMethods([]string{
		jwt.SigningMethodHS256.Name,
		jwt.SigningMethodHS384.Name,
		jwt.SigningMethodHS512.Name,
	}))

	// 保留默认 claims 校验（exp/nbf），避免放行过期或未生效的 token。
	token, err := parser.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWT.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			// token 过期但仍返回 claims（用于 RefreshToken 等场景）
			// jwt-go 在解析时即使遇到过期错误，token.Claims 仍会被填充
			if claims, ok := token.Claims.(*JWTClaims); ok {
				return claims, ErrTokenExpired
			}
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func randomHexString(byteLength int) (string, error) {
	if byteLength <= 0 {
		byteLength = 16
	}
	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isReservedEmail(email string) bool {
	normalized := strings.ToLower(strings.TrimSpace(email))
	return strings.HasSuffix(normalized, LinuxDoConnectSyntheticEmailDomain)
}

// GenerateToken 生成JWT access token
// 使用新的access_token_expire_minutes配置项（如果配置了），否则回退到expire_hour
func (s *AuthService) GenerateToken(user *User) (string, error) {
	now := time.Now()
	var expiresAt time.Time
	if s.cfg.JWT.AccessTokenExpireMinutes > 0 {
		expiresAt = now.Add(time.Duration(s.cfg.JWT.AccessTokenExpireMinutes) * time.Minute)
	} else {
		// 向后兼容：使用旧的expire_hour配置
		expiresAt = now.Add(time.Duration(s.cfg.JWT.ExpireHour) * time.Hour)
	}

	claims := &JWTClaims{
		UserID:       user.ID,
		Email:        user.Email,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}

// GetAccessTokenExpiresIn 返回Access Token的有效期（秒）
// 用于前端设置刷新定时器
func (s *AuthService) GetAccessTokenExpiresIn() int {
	if s.cfg.JWT.AccessTokenExpireMinutes > 0 {
		return s.cfg.JWT.AccessTokenExpireMinutes * 60
	}
	return s.cfg.JWT.ExpireHour * 3600
}

// HashPassword 使用bcrypt加密密码
func (s *AuthService) HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

// CheckPassword 验证密码是否匹配
func (s *AuthService) CheckPassword(password, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// RefreshToken 刷新token
func (s *AuthService) RefreshToken(ctx context.Context, oldTokenString string) (string, error) {
	// 验证旧token（即使过期也允许，用于刷新）
	claims, err := s.ValidateToken(oldTokenString)
	if err != nil && !errors.Is(err, ErrTokenExpired) {
		return "", err
	}

	// 获取最新的用户信息
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", ErrInvalidToken
		}
		logger.LegacyPrintf("service.auth", "[Auth] Database error refreshing token: %v", err)
		return "", ErrServiceUnavailable
	}

	// 检查用户状态
	if !user.IsActive() {
		return "", ErrUserNotActive
	}

	// Security: Check TokenVersion to prevent refreshing revoked tokens
	// This ensures tokens issued before a password change cannot be refreshed
	if claims.TokenVersion != user.TokenVersion {
		return "", ErrTokenRevoked
	}

	// 生成新token
	return s.GenerateToken(user)
}

// IsPasswordResetEnabled 检查是否启用密码重置功能
// 要求：必须同时开启邮件验证且 SMTP 配置正确
func (s *AuthService) IsPasswordResetEnabled(ctx context.Context) bool {
	if s.settingService == nil {
		return false
	}
	// Must have email verification enabled and SMTP configured
	if !s.settingService.IsEmailVerifyEnabled(ctx) {
		return false
	}
	return s.settingService.IsPasswordResetEnabled(ctx)
}

// preparePasswordReset validates the password reset request and returns necessary data
// Returns (siteName, resetURL, shouldProceed)
// shouldProceed is false when we should silently return success (to prevent enumeration)
func (s *AuthService) preparePasswordReset(ctx context.Context, email, frontendBaseURL string) (string, string, bool) {
	// Check if user exists (but don't reveal this to the caller)
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Security: Log but don't reveal that user doesn't exist
			logger.LegacyPrintf("service.auth", "[Auth] Password reset requested for non-existent email: %s", email)
			return "", "", false
		}
		logger.LegacyPrintf("service.auth", "[Auth] Database error checking email for password reset: %v", err)
		return "", "", false
	}

	// Check if user is active
	if !user.IsActive() {
		logger.LegacyPrintf("service.auth", "[Auth] Password reset requested for inactive user: %s", email)
		return "", "", false
	}

	// Get site name
	siteName := "Sub2API"
	if s.settingService != nil {
		siteName = s.settingService.GetSiteName(ctx)
	}

	// Build reset URL base
	resetURL := fmt.Sprintf("%s/reset-password", strings.TrimSuffix(frontendBaseURL, "/"))

	return siteName, resetURL, true
}

// RequestPasswordReset 请求密码重置（同步发送）
// Security: Returns the same response regardless of whether the email exists (prevent user enumeration)
func (s *AuthService) RequestPasswordReset(ctx context.Context, email, frontendBaseURL string) error {
	if !s.IsPasswordResetEnabled(ctx) {
		return infraerrors.Forbidden("PASSWORD_RESET_DISABLED", "password reset is not enabled")
	}
	if s.emailService == nil {
		return ErrServiceUnavailable
	}

	siteName, resetURL, shouldProceed := s.preparePasswordReset(ctx, email, frontendBaseURL)
	if !shouldProceed {
		return nil // Silent success to prevent enumeration
	}

	if err := s.emailService.SendPasswordResetEmail(ctx, email, siteName, resetURL); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to send password reset email to %s: %v", email, err)
		return nil // Silent success to prevent enumeration
	}

	logger.LegacyPrintf("service.auth", "[Auth] Password reset email sent to: %s", email)
	return nil
}

// RequestPasswordResetAsync 异步请求密码重置（队列发送）
// Security: Returns the same response regardless of whether the email exists (prevent user enumeration)
func (s *AuthService) RequestPasswordResetAsync(ctx context.Context, email, frontendBaseURL string) error {
	if !s.IsPasswordResetEnabled(ctx) {
		return infraerrors.Forbidden("PASSWORD_RESET_DISABLED", "password reset is not enabled")
	}
	if s.emailQueueService == nil {
		return ErrServiceUnavailable
	}

	siteName, resetURL, shouldProceed := s.preparePasswordReset(ctx, email, frontendBaseURL)
	if !shouldProceed {
		return nil // Silent success to prevent enumeration
	}

	if err := s.emailQueueService.EnqueuePasswordReset(email, siteName, resetURL); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to enqueue password reset email for %s: %v", email, err)
		return nil // Silent success to prevent enumeration
	}

	logger.LegacyPrintf("service.auth", "[Auth] Password reset email enqueued for: %s", email)
	return nil
}

// ResetPassword 重置密码
// Security: Increments TokenVersion to invalidate all existing JWT tokens
func (s *AuthService) ResetPassword(ctx context.Context, email, token, newPassword string) error {
	// Check if password reset is enabled
	if !s.IsPasswordResetEnabled(ctx) {
		return infraerrors.Forbidden("PASSWORD_RESET_DISABLED", "password reset is not enabled")
	}

	if s.emailService == nil {
		return ErrServiceUnavailable
	}

	// Verify and consume the reset token (one-time use)
	if err := s.emailService.ConsumePasswordResetToken(ctx, email, token); err != nil {
		return err
	}

	// Get user
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrInvalidResetToken // Token was valid but user was deleted
		}
		logger.LegacyPrintf("service.auth", "[Auth] Database error getting user for password reset: %v", err)
		return ErrServiceUnavailable
	}

	// Check if user is active
	if !user.IsActive() {
		return ErrUserNotActive
	}

	// Hash new password
	hashedPassword, err := s.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if _, err := updatePasswordAndIncrementTokenVersion(ctx, s.userRepo, user.ID, hashedPassword); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Database error updating password for user %d: %v", user.ID, err)
		return ErrServiceUnavailable
	}

	// Also revoke all refresh tokens for this user
	if err := s.deleteAllUserRefreshTokens(ctx, user.ID); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to revoke refresh tokens for user %d: %v", user.ID, err)
		// Don't return error - password was already changed successfully
	}

	logger.LegacyPrintf("service.auth", "[Auth] Password reset successful for user: %s", email)
	return nil
}

// IssueSSOTicket creates a short-lived, one-time ticket for logging into trusted external apps.
func (s *AuthService) IssueSSOTicket(ctx context.Context, userID int64, apiKeyID *int64) (string, time.Duration, error) {
	if s.ssoTicketCache == nil {
		return "", 0, ErrServiceUnavailable
	}
	if userID <= 0 {
		return "", 0, ErrInvalidSSOTicket
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", 0, fmt.Errorf("generate sso ticket: %w", err)
	}
	ticket := hex.EncodeToString(tokenBytes)

	data := SSOTicketData{
		Purpose:   ssoTicketPurpose,
		UserID:    userID,
		APIKeyID:  apiKeyID,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.ssoTicketCache.StoreSSOTicket(ctx, ticket, data, ssoTicketTTL); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to store SSO ticket for user %d: %v", userID, err)
		return "", 0, ErrServiceUnavailable
	}
	return ticket, ssoTicketTTL, nil
}

// ExchangeSSOTicket consumes a one-time SSO ticket and issues the standard token pair.
func (s *AuthService) ExchangeSSOTicket(ctx context.Context, ticket string) (*TokenPair, *User, *int64, error) {
	if s.ssoTicketCache == nil {
		return nil, nil, nil, ErrInvalidSSOTicket
	}

	ticket = strings.TrimSpace(ticket)
	if len(ticket) != 64 {
		return nil, nil, nil, ErrInvalidSSOTicket
	}
	if _, err := hex.DecodeString(ticket); err != nil {
		return nil, nil, nil, ErrInvalidSSOTicket
	}

	data, err := s.ssoTicketCache.ConsumeSSOTicket(ctx, ticket)
	if err != nil {
		if errors.Is(err, ErrSSOTicketNotFound) {
			return nil, nil, nil, ErrInvalidSSOTicket
		}
		logger.LegacyPrintf("service.auth", "[Auth] Failed to consume SSO ticket: %v", err)
		return nil, nil, nil, ErrServiceUnavailable
	}
	if data.Purpose != ssoTicketPurpose {
		return nil, nil, nil, ErrInvalidSSOTicket
	}

	user, err := s.userRepo.GetByID(ctx, data.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil, nil, ErrInvalidSSOTicket
		}
		logger.LegacyPrintf("service.auth", "[Auth] Database error getting SSO user %d: %v", data.UserID, err)
		return nil, nil, nil, ErrServiceUnavailable
	}
	if !user.IsActive() {
		return nil, nil, nil, ErrUserNotActive
	}

	pair, err := s.GenerateTokenPair(ctx, user, "")
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to generate SSO token pair for user %d: %v", user.ID, err)
		return nil, nil, nil, ErrServiceUnavailable
	}
	_ = s.userRepo.TouchLastActive(ctx, user.ID, time.Now())
	return pair, user, data.APIKeyID, nil
}

// EmbedTicket is the short-lived launch credential and its server-resolved
// consumer contract. The target URL is configuration, never browser input.
type EmbedTicket struct {
	Ticket    string
	ExpiresIn int
	TargetURL string
	Audience  string
}

type EmbedSession struct {
	SessionToken string
	ExpiresIn    int
	UserID       int64
	Role         string
}

type embedSessionClaims struct {
	UserID     int64  `json:"user_id"`
	Role       string `json:"role"`
	Purpose    string `json:"purpose"`
	TargetKind string `json:"target_kind"`
	TargetID   string `json:"target_id"`
	Delivery   string `json:"delivery"`
	jwt.RegisteredClaims
}

// IssueEmbedTicket creates a one-time ticket bound to one configured audience,
// target and delivery mode. It is deliberately separate from chatbot SSO.
func (s *AuthService) IssueEmbedTicket(ctx context.Context, userID int64, targetKind, targetID, delivery string) (*EmbedTicket, error) {
	if s.ssoTicketCache == nil || s.embedTargetResolver == nil {
		return nil, ErrServiceUnavailable
	}
	if delivery != EmbedDeliveryIframe && delivery != EmbedDeliveryNewTab {
		return nil, ErrEmbedTargetUnavailable
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidEmbedTicket
		}
		return nil, ErrServiceUnavailable
	}
	if !user.IsActive() {
		return nil, ErrUserNotActive
	}
	target, err := s.embedTargetResolver.ResolveEmbedTarget(ctx, targetKind, targetID, user.Role)
	if err != nil {
		return nil, ErrEmbedTargetUnavailable
	}

	ticket, err := randomHexString(32)
	if err != nil {
		return nil, fmt.Errorf("generate embed ticket: %w", err)
	}
	data := SSOTicketData{
		Purpose:    embedTicketPurpose,
		UserID:     user.ID,
		Audience:   target.Audience,
		TargetKind: target.Kind,
		TargetID:   target.ID,
		Delivery:   delivery,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.ssoTicketCache.StoreSSOTicket(ctx, ticket, data, ssoTicketTTL); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to store embed ticket for user %d: %v", user.ID, err)
		return nil, ErrServiceUnavailable
	}
	return &EmbedTicket{Ticket: ticket, ExpiresIn: int(ssoTicketTTL.Seconds()), TargetURL: target.URL, Audience: target.Audience}, nil
}

// ExchangeEmbedTicket consumes the ticket before validation so failed or
// replayed attempts cannot be upgraded into a general login session.
func (s *AuthService) ExchangeEmbedTicket(ctx context.Context, ticket, audience, targetKind, targetID, delivery string) (*EmbedSession, error) {
	if s.ssoTicketCache == nil {
		return nil, ErrInvalidEmbedTicket
	}
	ticket = strings.TrimSpace(ticket)
	if len(ticket) != 64 {
		return nil, ErrInvalidEmbedTicket
	}
	if _, err := hex.DecodeString(ticket); err != nil {
		return nil, ErrInvalidEmbedTicket
	}
	data, err := s.ssoTicketCache.ConsumeSSOTicket(ctx, ticket)
	if err != nil {
		if errors.Is(err, ErrSSOTicketNotFound) {
			return nil, ErrInvalidEmbedTicket
		}
		return nil, ErrServiceUnavailable
	}
	normalizedAudience, err := normalizeEmbedAudience(audience)
	if err != nil || data.Purpose != embedTicketPurpose || data.Audience != normalizedAudience ||
		data.TargetKind != strings.TrimSpace(targetKind) || data.TargetID != strings.TrimSpace(targetID) || data.Delivery != delivery {
		return nil, ErrInvalidEmbedTicket
	}
	now := time.Now().UTC()
	if data.CreatedAt.IsZero() || data.CreatedAt.After(now.Add(5*time.Second)) || now.Sub(data.CreatedAt) > ssoTicketTTL {
		return nil, ErrInvalidEmbedTicket
	}
	user, err := s.userRepo.GetByID(ctx, data.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidEmbedTicket
		}
		return nil, ErrServiceUnavailable
	}
	if !user.IsActive() {
		return nil, ErrUserNotActive
	}
	expiresAt := now.Add(embedSessionTTL)
	claims := &embedSessionClaims{
		UserID: user.ID, Role: user.Role, Purpose: embedSessionPurpose,
		TargetKind: data.TargetKind, TargetID: data.TargetID, Delivery: data.Delivery,
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{data.Audience},
			Subject:   strconv.FormatInt(user.ID, 10),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return nil, ErrServiceUnavailable
	}
	return &EmbedSession{SessionToken: signed, ExpiresIn: int(embedSessionTTL.Seconds()), UserID: user.ID, Role: user.Role}, nil
}

// ==================== Refresh Token Methods ====================

// TokenPair 包含Access Token和Refresh Token
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // Access Token有效期（秒）
}

// TokenPairWithUser extends TokenPair with user role for backend mode checks
type TokenPairWithUser struct {
	TokenPair
	UserRole string
}

// GenerateTokenPair 生成Access Token和Refresh Token对
// familyID: 可选的Token家族ID，用于Token轮转时保持家族关系
func (s *AuthService) GenerateTokenPair(ctx context.Context, user *User, familyID string) (*TokenPair, error) {
	// 检查 refreshTokenCache 是否可用
	if s.refreshTokenCache == nil {
		return nil, errors.New("refresh token cache not configured")
	}

	// 生成Access Token
	accessToken, err := s.GenerateToken(user)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	// 生成Refresh Token
	refreshToken, err := s.generateRefreshToken(ctx, user, familyID)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    s.GetAccessTokenExpiresIn(),
	}, nil
}

// generateRefreshToken 生成并存储Refresh Token
func (s *AuthService) generateRefreshToken(ctx context.Context, user *User, familyID string) (string, error) {
	// 生成随机Token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	rawToken := refreshTokenPrefix + hex.EncodeToString(tokenBytes)

	// 计算Token哈希（存储哈希而非原始Token）
	tokenHash := hashToken(rawToken)

	// 如果没有提供familyID，生成新的
	if familyID == "" {
		familyBytes := make([]byte, 16)
		if _, err := rand.Read(familyBytes); err != nil {
			return "", fmt.Errorf("generate family id: %w", err)
		}
		familyID = hex.EncodeToString(familyBytes)
	}

	now := time.Now()
	ttl := time.Duration(s.cfg.JWT.RefreshTokenExpireDays) * 24 * time.Hour

	data := &RefreshTokenData{
		UserID:       user.ID,
		TokenVersion: user.TokenVersion,
		FamilyID:     familyID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
	}

	// 存储Token数据
	if err := s.refreshTokenCache.StoreRefreshToken(ctx, tokenHash, data, ttl); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}

	// 添加到用户Token集合
	if err := s.refreshTokenCache.AddToUserTokenSet(ctx, user.ID, tokenHash, ttl); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to add token to user set: %v", err)
		_ = s.refreshTokenCache.DeleteRefreshToken(ctx, tokenHash)
		return "", fmt.Errorf("add refresh token to user set: %w", err)
	}

	// 添加到家族Token集合
	if err := s.refreshTokenCache.AddToFamilyTokenSet(ctx, familyID, tokenHash, ttl); err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Failed to add token to family set: %v", err)
		_ = s.refreshTokenCache.DeleteRefreshToken(ctx, tokenHash)
		return "", fmt.Errorf("add refresh token to family set: %w", err)
	}

	return rawToken, nil
}

// RefreshTokenPair 使用Refresh Token刷新Token对
// 实现Token轮转：每次刷新都会生成新的Refresh Token，旧Token立即失效
func (s *AuthService) RefreshTokenPair(ctx context.Context, refreshToken string) (*TokenPairWithUser, error) {
	// 检查 refreshTokenCache 是否可用
	if s.refreshTokenCache == nil {
		return nil, ErrRefreshTokenInvalid
	}

	// 验证Token格式
	if !strings.HasPrefix(refreshToken, refreshTokenPrefix) {
		return nil, ErrRefreshTokenInvalid
	}

	tokenHash := hashToken(refreshToken)

	// 原子消费Token，确保同一个Refresh Token最多只能成功刷新一次。
	data, err := s.refreshTokenCache.ConsumeRefreshToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, ErrRefreshTokenReused) {
			if data != nil && data.FamilyID != "" {
				_ = s.refreshTokenCache.DeleteTokenFamily(ctx, data.FamilyID)
			}
			logger.LegacyPrintf("service.auth", "[Auth] Refresh token reuse detected; revoked token family")
			return nil, ErrRefreshTokenReused
		}
		if errors.Is(err, ErrRefreshTokenNotFound) {
			// Token不存在，可能是已被使用（Token轮转）或已过期
			logger.LegacyPrintf("service.auth", "[Auth] Refresh token not found")
			return nil, ErrRefreshTokenInvalid
		}
		logger.LegacyPrintf("service.auth", "[Auth] Error consuming refresh token: %v", err)
		return nil, ErrServiceUnavailable
	}

	// 检查Token是否过期
	if time.Now().After(data.ExpiresAt) {
		// 删除过期Token
		_ = s.refreshTokenCache.DeleteRefreshToken(ctx, tokenHash)
		return nil, ErrRefreshTokenExpired
	}

	// 获取用户信息
	user, err := s.userRepo.GetByID(ctx, data.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// 用户已删除，撤销整个Token家族
			_ = s.refreshTokenCache.DeleteTokenFamily(ctx, data.FamilyID)
			return nil, ErrRefreshTokenInvalid
		}
		logger.LegacyPrintf("service.auth", "[Auth] Database error getting user for token refresh: %v", err)
		return nil, ErrServiceUnavailable
	}

	// 检查用户状态
	if !user.IsActive() {
		// 用户被禁用，撤销整个Token家族
		_ = s.refreshTokenCache.DeleteTokenFamily(ctx, data.FamilyID)
		return nil, ErrUserNotActive
	}

	// 检查TokenVersion（密码更改后所有Token失效）
	if data.TokenVersion != user.TokenVersion {
		// TokenVersion不匹配，撤销整个Token家族
		_ = s.refreshTokenCache.DeleteTokenFamily(ctx, data.FamilyID)
		return nil, ErrTokenRevoked
	}

	// 生成新的Token对，保持同一个家族ID
	pair, err := s.GenerateTokenPair(ctx, user, data.FamilyID)
	if err != nil {
		return nil, err
	}
	return &TokenPairWithUser{
		TokenPair: *pair,
		UserRole:  user.Role,
	}, nil
}

// RevokeRefreshToken 撤销单个Refresh Token
func (s *AuthService) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	if s.refreshTokenCache == nil {
		return nil // No-op if cache not configured
	}
	if !strings.HasPrefix(refreshToken, refreshTokenPrefix) {
		return ErrRefreshTokenInvalid
	}

	tokenHash := hashToken(refreshToken)
	return s.refreshTokenCache.DeleteRefreshToken(ctx, tokenHash)
}

// RevokeAllUserSessions 撤销用户的所有会话（所有Refresh Token）
// 用于密码更改或用户主动登出所有设备
func (s *AuthService) RevokeAllUserSessions(ctx context.Context, userID int64) error {
	if _, err := incrementTokenVersion(ctx, s.userRepo, userID); err != nil {
		return err
	}
	return s.deleteAllUserRefreshTokens(ctx, userID)
}

func (s *AuthService) deleteAllUserRefreshTokens(ctx context.Context, userID int64) error {
	if s.refreshTokenCache == nil {
		return nil // No-op if cache not configured
	}
	return s.refreshTokenCache.DeleteUserRefreshTokens(ctx, userID)
}

// hashToken 计算Token的SHA256哈希
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
