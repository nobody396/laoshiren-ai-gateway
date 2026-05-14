package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
)

const (
	AuthProviderLinuxDo = "linuxdo"
	AuthProviderGoogle  = "google"
	AuthProviderGitHub  = "github"

	PendingAuthActionLogin = "login"
	PendingAuthActionBind  = "bind"
)

var (
	ErrAuthIdentityNotFound     = infraerrors.NotFound("AUTH_IDENTITY_NOT_FOUND", "auth identity not found")
	ErrAuthIdentityConflict     = infraerrors.Conflict("AUTH_IDENTITY_CONFLICT", "auth identity already belongs to another user")
	ErrAuthIdentityOrphanUnbind = infraerrors.BadRequest("AUTH_IDENTITY_ORPHAN_UNBIND", "cannot unbind the last available login identity")

	ErrPendingAuthSessionNotFound = infraerrors.NotFound("PENDING_AUTH_SESSION_NOT_FOUND", "pending auth session not found")
	ErrPendingAuthSessionExpired  = infraerrors.Unauthorized("PENDING_AUTH_SESSION_EXPIRED", "pending auth session has expired")
	ErrPendingAuthSessionConsumed = infraerrors.Unauthorized("PENDING_AUTH_SESSION_CONSUMED", "pending auth session has already been used")

	userAgentVersionRegex = regexp.MustCompile(`/(\d+)\.(\d+)\.(\d+)`)
)

var defaultFingerprint = Fingerprint{
	UserAgent:               "claude-cli/2.1.92 (external, cli)",
	StainlessLang:           "js",
	StainlessPackageVersion: "0.70.0",
	StainlessOS:             "Linux",
	StainlessArch:           "arm64",
	StainlessRuntime:        "node",
	StainlessRuntimeVersion: "v24.13.0",
}

// Fingerprint represents account fingerprint data.
type Fingerprint struct {
	ClientID                string
	UserAgent               string
	StainlessLang           string
	StainlessPackageVersion string
	StainlessOS             string
	StainlessArch           string
	StainlessRuntime        string
	StainlessRuntimeVersion string
	UpdatedAt               int64 `json:",omitempty"`
}

// IdentityCache defines cache operations for identity service.
type IdentityCache interface {
	GetFingerprint(ctx context.Context, accountID int64) (*Fingerprint, error)
	SetFingerprint(ctx context.Context, accountID int64, fp *Fingerprint) error
	GetMaskedSessionID(ctx context.Context, accountID int64) (string, error)
	SetMaskedSessionID(ctx context.Context, accountID int64, sessionID string) error
}

type AuthIdentity struct {
	ID             int64          `json:"id"`
	UserID         int64          `json:"user_id"`
	Provider       string         `json:"provider"`
	ProviderUserID string         `json:"provider_user_id"`
	Email          string         `json:"email"`
	EmailVerified  bool           `json:"email_verified"`
	DisplayName    string         `json:"display_name"`
	AvatarURL      string         `json:"avatar_url"`
	RawProfile     map[string]any `json:"raw_profile"`
	LastLoginAt    *time.Time     `json:"last_login_at,omitempty"`
	BoundAt        time.Time      `json:"bound_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type AuthIdentityUpsertInput struct {
	UserID         int64
	Provider       string
	ProviderUserID string
	Email          string
	EmailVerified  bool
	DisplayName    string
	AvatarURL      string
	RawProfile     map[string]any
	LastLoginAt    *time.Time
}

type PendingAuthSession struct {
	ID             int64
	State          string
	Provider       string
	ProviderUserID *string
	IntendedAction string
	ClaimsSnapshot map[string]any
	RedirectURI    string
	UserID         *int64
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
	CreatedAt      time.Time
	IPAddress      string
	UserAgent      string
}

func (s *PendingAuthSession) IsExpired(now time.Time) bool {
	if s == nil {
		return true
	}
	return !s.ExpiresAt.IsZero() && s.ExpiresAt.Before(now)
}

func (s *PendingAuthSession) IsConsumed() bool {
	return s != nil && s.ConsumedAt != nil
}

type PendingAuthSessionCreateInput struct {
	State          string
	Provider       string
	IntendedAction string
	ClaimsSnapshot map[string]any
	RedirectURI    string
	UserID         *int64
	ExpiresAt      time.Time
	IPAddress      string
	UserAgent      string
}

type PendingAuthSessionResolveInput struct {
	ProviderUserID string
	ClaimsSnapshot map[string]any
}

type AuthIdentityRepository interface {
	ListByUserID(ctx context.Context, userID int64) ([]*AuthIdentity, error)
	GetByProviderAccount(ctx context.Context, provider, providerUserID string) (*AuthIdentity, error)
	Upsert(ctx context.Context, input AuthIdentityUpsertInput) (*AuthIdentity, error)
	DeleteByUserProvider(ctx context.Context, userID int64, provider string) error
}

type PendingAuthSessionRepository interface {
	Create(ctx context.Context, input PendingAuthSessionCreateInput) (*PendingAuthSession, error)
	GetByState(ctx context.Context, state string) (*PendingAuthSession, error)
	Resolve(ctx context.Context, state string, input PendingAuthSessionResolveInput) (*PendingAuthSession, error)
	MarkConsumed(ctx context.Context, state string, consumedAt time.Time) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// IdentityService combines OAuth fingerprint handling and user-bound auth identities.
type IdentityService struct {
	cache              IdentityCache
	identityRepo       AuthIdentityRepository
	pendingSessionRepo PendingAuthSessionRepository
	userRepo           UserRepository
}

func NewIdentityService(
	cache IdentityCache,
	identityRepo AuthIdentityRepository,
	pendingSessionRepo PendingAuthSessionRepository,
	userRepo UserRepository,
) *IdentityService {
	return &IdentityService{
		cache:              cache,
		identityRepo:       identityRepo,
		pendingSessionRepo: pendingSessionRepo,
		userRepo:           userRepo,
	}
}

func (s *IdentityService) ListBindings(ctx context.Context, userID int64) ([]*AuthIdentity, error) {
	return s.identityRepo.ListByUserID(ctx, userID)
}

func (s *IdentityService) GetByProviderAccount(ctx context.Context, provider, providerUserID string) (*AuthIdentity, error) {
	return s.identityRepo.GetByProviderAccount(ctx, provider, providerUserID)
}

func (s *IdentityService) UpsertBinding(ctx context.Context, input AuthIdentityUpsertInput) (*AuthIdentity, error) {
	return s.identityRepo.Upsert(ctx, input)
}

func (s *IdentityService) CreatePendingSession(ctx context.Context, input PendingAuthSessionCreateInput) (*PendingAuthSession, error) {
	return s.pendingSessionRepo.Create(ctx, input)
}

func (s *IdentityService) GetPendingSession(ctx context.Context, state string) (*PendingAuthSession, error) {
	session, err := s.pendingSessionRepo.GetByState(ctx, strings.TrimSpace(state))
	if err != nil {
		return nil, err
	}
	if session.IsConsumed() {
		return nil, ErrPendingAuthSessionConsumed
	}
	if session.IsExpired(time.Now()) {
		return nil, ErrPendingAuthSessionExpired
	}
	return session, nil
}

func (s *IdentityService) ResolvePendingSession(ctx context.Context, state string, input PendingAuthSessionResolveInput) (*PendingAuthSession, error) {
	session, err := s.GetPendingSession(ctx, state)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ProviderUserID) == "" {
		return nil, infraerrors.BadRequest("PENDING_AUTH_PROVIDER_SUBJECT_REQUIRED", "provider user id is required")
	}
	if len(input.ClaimsSnapshot) == 0 {
		return nil, infraerrors.BadRequest("PENDING_AUTH_CLAIMS_REQUIRED", "claims snapshot is required")
	}
	if session.ProviderUserID != nil && strings.TrimSpace(*session.ProviderUserID) != "" {
		return session, nil
	}
	claims := map[string]any{}
	for key, value := range session.ClaimsSnapshot {
		claims[key] = value
	}
	for key, value := range input.ClaimsSnapshot {
		claims[key] = value
	}
	input.ClaimsSnapshot = claims
	return s.pendingSessionRepo.Resolve(ctx, state, input)
}

func (s *IdentityService) MarkPendingSessionConsumed(ctx context.Context, state string) error {
	return s.pendingSessionRepo.MarkConsumed(ctx, state, time.Now().UTC())
}

func (s *IdentityService) CleanupExpiredPendingSessions(ctx context.Context, before time.Time) (int64, error) {
	return s.pendingSessionRepo.DeleteExpired(ctx, before)
}

func (s *IdentityService) Unbind(ctx context.Context, userID int64, provider string) error {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return infraerrors.BadRequest("AUTH_PROVIDER_REQUIRED", "provider is required")
	}
	identities, err := s.identityRepo.ListByUserID(ctx, userID)
	if err != nil {
		return err
	}

	var found *AuthIdentity
	remaining := 0
	for _, identity := range identities {
		if identity == nil {
			continue
		}
		if identity.Provider == provider {
			found = identity
			continue
		}
		remaining++
	}
	if found == nil {
		return ErrAuthIdentityNotFound
	}
	if remaining == 0 {
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			return err
		}
		if isReservedEmail(user.Email) {
			return ErrAuthIdentityOrphanUnbind
		}
	}
	return s.identityRepo.DeleteByUserProvider(ctx, userID, provider)
}

// GetOrCreateFingerprint gets or creates a stable OAuth fingerprint for an account.
func (s *IdentityService) GetOrCreateFingerprint(ctx context.Context, accountID int64, headers http.Header) (*Fingerprint, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("identity cache is not configured")
	}

	cached, err := s.cache.GetFingerprint(ctx, accountID)
	if err == nil && cached != nil {
		needWrite := false

		clientUA := headers.Get("User-Agent")
		if clientUA != "" && isNewerVersion(clientUA, cached.UserAgent) {
			mergeHeadersIntoFingerprint(cached, headers)
			needWrite = true
			logger.LegacyPrintf("service.identity", "Updated fingerprint for account %d: %s (merge update)", accountID, clientUA)
		} else if time.Since(time.Unix(cached.UpdatedAt, 0)) > 24*time.Hour {
			needWrite = true
		}

		if needWrite {
			cached.UpdatedAt = time.Now().Unix()
			if err := s.cache.SetFingerprint(ctx, accountID, cached); err != nil {
				logger.LegacyPrintf("service.identity", "Warning: failed to refresh fingerprint for account %d: %v", accountID, err)
			}
		}
		return cached, nil
	}

	fp := s.createFingerprintFromHeaders(headers)
	fp.ClientID = generateClientID()
	fp.UpdatedAt = time.Now().Unix()

	if err := s.cache.SetFingerprint(ctx, accountID, fp); err != nil {
		logger.LegacyPrintf("service.identity", "Warning: failed to cache fingerprint for account %d: %v", accountID, err)
	}

	logger.LegacyPrintf("service.identity", "Created new fingerprint for account %d with client_id: %s", accountID, fp.ClientID)
	return fp, nil
}

func (s *IdentityService) createFingerprintFromHeaders(headers http.Header) *Fingerprint {
	fp := &Fingerprint{}

	if ua := headers.Get("User-Agent"); ua != "" {
		fp.UserAgent = ua
	} else {
		fp.UserAgent = defaultFingerprint.UserAgent
	}

	fp.StainlessLang = getHeaderOrDefault(headers, "X-Stainless-Lang", defaultFingerprint.StainlessLang)
	fp.StainlessPackageVersion = getHeaderOrDefault(headers, "X-Stainless-Package-Version", defaultFingerprint.StainlessPackageVersion)
	fp.StainlessOS = getHeaderOrDefault(headers, "X-Stainless-OS", defaultFingerprint.StainlessOS)
	fp.StainlessArch = getHeaderOrDefault(headers, "X-Stainless-Arch", defaultFingerprint.StainlessArch)
	fp.StainlessRuntime = getHeaderOrDefault(headers, "X-Stainless-Runtime", defaultFingerprint.StainlessRuntime)
	fp.StainlessRuntimeVersion = getHeaderOrDefault(headers, "X-Stainless-Runtime-Version", defaultFingerprint.StainlessRuntimeVersion)

	return fp
}

func mergeHeadersIntoFingerprint(fp *Fingerprint, headers http.Header) {
	if ua := headers.Get("User-Agent"); ua != "" {
		fp.UserAgent = ua
	}
	mergeHeader(headers, "X-Stainless-Lang", &fp.StainlessLang)
	mergeHeader(headers, "X-Stainless-Package-Version", &fp.StainlessPackageVersion)
	mergeHeader(headers, "X-Stainless-OS", &fp.StainlessOS)
	mergeHeader(headers, "X-Stainless-Arch", &fp.StainlessArch)
	mergeHeader(headers, "X-Stainless-Runtime", &fp.StainlessRuntime)
	mergeHeader(headers, "X-Stainless-Runtime-Version", &fp.StainlessRuntimeVersion)
}

func mergeHeader(headers http.Header, key string, target *string) {
	if v := headers.Get(key); v != "" {
		*target = v
	}
}

func getHeaderOrDefault(headers http.Header, key, defaultValue string) string {
	if v := headers.Get(key); v != "" {
		return v
	}
	return defaultValue
}

func (s *IdentityService) ApplyFingerprint(req *http.Request, fp *Fingerprint) {
	if fp == nil {
		return
	}

	if fp.UserAgent != "" {
		req.Header.Set("user-agent", fp.UserAgent)
	}
	if fp.StainlessLang != "" {
		req.Header.Set("X-Stainless-Lang", fp.StainlessLang)
	}
	if fp.StainlessPackageVersion != "" {
		req.Header.Set("X-Stainless-Package-Version", fp.StainlessPackageVersion)
	}
	if fp.StainlessOS != "" {
		req.Header.Set("X-Stainless-OS", fp.StainlessOS)
	}
	if fp.StainlessArch != "" {
		req.Header.Set("X-Stainless-Arch", fp.StainlessArch)
	}
	if fp.StainlessRuntime != "" {
		req.Header.Set("X-Stainless-Runtime", fp.StainlessRuntime)
	}
	if fp.StainlessRuntimeVersion != "" {
		req.Header.Set("X-Stainless-Runtime-Version", fp.StainlessRuntimeVersion)
	}
}

// RewriteUserID rewrites metadata.user_id while preserving the rest of the body.
func (s *IdentityService) RewriteUserID(body []byte, accountID int64, accountUUID, cachedClientID, fingerprintUA string) ([]byte, error) {
	if len(body) == 0 || accountUUID == "" || cachedClientID == "" {
		return body, nil
	}

	var reqMap map[string]json.RawMessage
	if err := json.Unmarshal(body, &reqMap); err != nil {
		return body, nil
	}

	metadataRaw, ok := reqMap["metadata"]
	if !ok {
		return body, nil
	}

	var metadata map[string]any
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		return body, nil
	}

	userID, ok := metadata["user_id"].(string)
	if !ok || userID == "" {
		return body, nil
	}

	parsed := ParseMetadataUserID(userID)
	if parsed == nil {
		return body, nil
	}

	seed := fmt.Sprintf("%d::%s", accountID, parsed.SessionID)
	newSessionHash := generateUUIDFromSeed(seed)
	version := ExtractCLIVersion(fingerprintUA)
	newUserID := FormatMetadataUserID(cachedClientID, accountUUID, newSessionHash, version)
	metadata["user_id"] = newUserID

	newMetadataRaw, err := json.Marshal(metadata)
	if err != nil {
		return body, nil
	}
	reqMap["metadata"] = newMetadataRaw

	return json.Marshal(reqMap)
}

// RewriteUserIDWithMasking applies session masking when enabled for the account.
func (s *IdentityService) RewriteUserIDWithMasking(ctx context.Context, body []byte, account *Account, accountUUID, cachedClientID, fingerprintUA string) ([]byte, error) {
	newBody, err := s.RewriteUserID(body, account.ID, accountUUID, cachedClientID, fingerprintUA)
	if err != nil {
		return newBody, err
	}
	if !account.IsSessionIDMaskingEnabled() || s.cache == nil {
		return newBody, nil
	}

	var reqMap map[string]json.RawMessage
	if err := json.Unmarshal(newBody, &reqMap); err != nil {
		return newBody, nil
	}

	metadataRaw, ok := reqMap["metadata"]
	if !ok {
		return newBody, nil
	}

	var metadata map[string]any
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		return newBody, nil
	}

	userID, ok := metadata["user_id"].(string)
	if !ok || userID == "" {
		return newBody, nil
	}

	uidParsed := ParseMetadataUserID(userID)
	if uidParsed == nil {
		return newBody, nil
	}

	maskedSessionID, err := s.cache.GetMaskedSessionID(ctx, account.ID)
	if err != nil {
		logger.LegacyPrintf("service.identity", "Warning: failed to get masked session ID for account %d: %v", account.ID, err)
		return newBody, nil
	}
	if maskedSessionID == "" {
		maskedSessionID = generateRandomUUID()
		logger.LegacyPrintf("service.identity", "Generated new masked session ID for account %d: %s", account.ID, maskedSessionID)
	}
	if err := s.cache.SetMaskedSessionID(ctx, account.ID, maskedSessionID); err != nil {
		logger.LegacyPrintf("service.identity", "Warning: failed to set masked session ID for account %d: %v", account.ID, err)
	}

	version := ExtractCLIVersion(fingerprintUA)
	newUserID := FormatMetadataUserID(uidParsed.DeviceID, uidParsed.AccountUUID, maskedSessionID, version)
	slog.Debug("session_id_masking_applied", "account_id", account.ID, "before", userID, "after", newUserID)
	metadata["user_id"] = newUserID

	newMetadataRaw, marshalErr := json.Marshal(metadata)
	if marshalErr != nil {
		return newBody, nil
	}
	reqMap["metadata"] = newMetadataRaw

	return json.Marshal(reqMap)
}

func generateRandomUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		h := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		b = h[:16]
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func generateClientID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		logger.LegacyPrintf("service.identity", "Warning: crypto/rand.Read failed: %v, using fallback", err)
		h := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		return hex.EncodeToString(h[:])
	}
	return hex.EncodeToString(b)
}

func generateUUIDFromSeed(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	bytes := hash[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func parseUserAgentVersion(ua string) (major, minor, patch int, ok bool) {
	matches := userAgentVersionRegex.FindStringSubmatch(ua)
	if len(matches) != 4 {
		return 0, 0, 0, false
	}
	major, _ = strconv.Atoi(matches[1])
	minor, _ = strconv.Atoi(matches[2])
	patch, _ = strconv.Atoi(matches[3])
	return major, minor, patch, true
}

func extractProduct(ua string) string {
	if idx := strings.Index(ua, "/"); idx > 0 {
		return strings.ToLower(ua[:idx])
	}
	return ""
}

func isNewerVersion(newUA, cachedUA string) bool {
	newProduct := extractProduct(newUA)
	cachedProduct := extractProduct(cachedUA)
	if newProduct == "" || cachedProduct == "" || newProduct != cachedProduct {
		return false
	}

	newMajor, newMinor, newPatch, newOK := parseUserAgentVersion(newUA)
	cachedMajor, cachedMinor, cachedPatch, cachedOK := parseUserAgentVersion(cachedUA)
	if !newOK || !cachedOK {
		return false
	}

	if newMajor != cachedMajor {
		return newMajor > cachedMajor
	}
	if newMinor != cachedMinor {
		return newMinor > cachedMinor
	}
	return newPatch > cachedPatch
}
