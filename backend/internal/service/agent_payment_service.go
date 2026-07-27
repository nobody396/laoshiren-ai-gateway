package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	defaultAgentSettlementMinimumAmount = 50.0
	agentSettlementAmountEpsilon        = 0.00000001
	agentPaymentQRCodeMaxSize           = 5 << 20
)

var (
	ErrAgentPaymentProfileIncomplete = infraerrors.Conflict(
		"AGENT_PAYMENT_PROFILE_INCOMPLETE",
		"agent payment profile is incomplete",
	)
	ErrAgentPaymentIdentityConflict = infraerrors.Conflict(
		"AGENT_PAYMENT_IDENTITY_CONFLICT",
		"this verified payment identity is already bound to another agent",
	)
)

type AgentPaymentQRCodeUpload struct {
	Filename    string
	ContentType string
	Size        int64
	Body        io.Reader
}

type AgentPaymentQRCodeFile struct {
	Path        string
	ContentType string
	Filename    string
}

func (s *CommissionService) GetAgentSettlementSettings(ctx context.Context) (*AgentSettlementSettings, error) {
	if s.paymentRepo == nil {
		return defaultAgentSettlementSettings(), nil
	}
	settings, err := s.paymentRepo.GetAgentSettlementSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get agent settlement settings: %w", err)
	}
	if settings == nil || settings.MinimumAmount <= 0 {
		return defaultAgentSettlementSettings(), nil
	}
	return settings, nil
}

func (s *CommissionService) UpdateAgentSettlementSettings(ctx context.Context, settings *AgentSettlementSettings) (*AgentSettlementSettings, error) {
	if s.paymentRepo == nil {
		return nil, fmt.Errorf("agent payment repository is not configured")
	}
	if settings == nil || settings.MinimumAmount <= 0 {
		return nil, infraerrors.BadRequest("INVALID_SETTLEMENT_SETTINGS", "settlement minimum amount must be greater than 0")
	}
	if err := s.paymentRepo.UpdateAgentSettlementSettings(ctx, settings); err != nil {
		return nil, fmt.Errorf("update agent settlement settings: %w", err)
	}
	return s.GetAgentSettlementSettings(ctx)
}

func (s *CommissionService) GetAgentPaymentProfile(ctx context.Context, agentID int64) (*AgentPaymentProfile, error) {
	if s.paymentRepo == nil {
		return emptyAgentPaymentProfile(agentID), nil
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}
	profile, err := s.paymentRepo.GetAgentPaymentProfile(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("get agent payment profile: %w", err)
	}
	if profile == nil {
		profile = emptyAgentPaymentProfile(agentID)
	}
	normalizeAgentPaymentProfile(profile)
	return profile, nil
}

func (s *CommissionService) UpdateAgentPaymentProfile(ctx context.Context, profile *AgentPaymentProfile) (*AgentPaymentProfile, error) {
	if s.paymentRepo == nil {
		return nil, fmt.Errorf("agent payment repository is not configured")
	}
	if profile == nil || profile.AgentID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_PAYMENT_PROFILE", "payment profile is required")
	}
	if err := s.ensureAgent(ctx, profile.AgentID); err != nil {
		return nil, err
	}
	normalizeAgentPaymentProfile(profile)
	if len([]rune(profile.AlipayRealName)) > 80 {
		return nil, infraerrors.BadRequest("INVALID_ALIPAY_REAL_NAME", "alipay real name is too long")
	}
	if len([]rune(profile.AlipayAccount)) > 120 {
		return nil, infraerrors.BadRequest("INVALID_ALIPAY_ACCOUNT", "alipay account is too long")
	}
	if len([]rune(profile.ContactPhone)) > 40 {
		return nil, infraerrors.BadRequest("INVALID_CONTACT_PHONE", "contact phone is too long")
	}
	if len([]rune(profile.PaymentNote)) > 500 {
		return nil, infraerrors.BadRequest("INVALID_PAYMENT_NOTE", "payment note is too long")
	}
	profile.IdentityFingerprintHash = agentPaymentIdentityFingerprint(
		profile.AlipayRealName,
		profile.AlipayAccount,
	)
	if err := s.paymentRepo.UpsertAgentPaymentProfile(ctx, profile); err != nil {
		return nil, fmt.Errorf("update agent payment profile: %w", err)
	}
	return s.GetAgentPaymentProfile(ctx, profile.AgentID)
}

func (s *CommissionService) ReviewAgentPaymentProfile(
	ctx context.Context,
	agentID int64,
	reviewerID int64,
	status string,
	note string,
) (*AgentPaymentProfile, error) {
	if s.paymentReview == nil {
		return nil, errors.New("agent payment review repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}
	if reviewerID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_REVIEWER", "invalid payment-profile reviewer")
	}
	status = strings.TrimSpace(strings.ToLower(status))
	if status != "verified" && status != "rejected" {
		return nil, infraerrors.BadRequest(
			"INVALID_PAYMENT_VERIFICATION_STATUS",
			"verification status must be verified or rejected",
		)
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > 500 {
		return nil, infraerrors.BadRequest("INVALID_VERIFICATION_NOTE", "verification note is too long")
	}
	profile, err := s.paymentReview.ReviewAgentPaymentProfile(
		ctx,
		agentID,
		reviewerID,
		status,
		note,
	)
	if err != nil {
		return nil, err
	}
	normalizeAgentPaymentProfile(profile)
	return profile, nil
}

func (s *CommissionService) ListPendingAgentPaymentProfiles(
	ctx context.Context,
	limit int,
) ([]AgentPaymentProfile, error) {
	if s.paymentReview == nil {
		return nil, errors.New("agent payment review repository is not configured")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	items, err := s.paymentReview.ListPendingAgentPaymentProfiles(ctx, limit)
	if err != nil {
		return nil, err
	}
	for index := range items {
		normalizeAgentPaymentProfile(&items[index])
	}
	return items, nil
}

func (s *CommissionService) UploadAgentPaymentQRCode(ctx context.Context, agentID int64, upload AgentPaymentQRCodeUpload) (*AgentPaymentProfile, error) {
	if s.paymentRepo == nil {
		return nil, fmt.Errorf("agent payment repository is not configured")
	}
	if err := s.ensureAgent(ctx, agentID); err != nil {
		return nil, err
	}
	if upload.Body == nil {
		return nil, infraerrors.BadRequest("PAYMENT_QR_REQUIRED", "payment QR file is required")
	}
	if upload.Size <= 0 {
		return nil, infraerrors.BadRequest("PAYMENT_QR_EMPTY", "payment QR file is empty")
	}
	if upload.Size > agentPaymentQRCodeMaxSize {
		return nil, infraerrors.BadRequest("PAYMENT_QR_TOO_LARGE", "payment QR file must be at most 5MB")
	}
	ext, ok := agentPaymentQRExt(upload.Filename, upload.ContentType)
	if !ok {
		return nil, infraerrors.BadRequest("PAYMENT_QR_UNSUPPORTED", "only jpg/png/webp images are supported")
	}

	token, err := randomPaymentHex(12)
	if err != nil {
		return nil, fmt.Errorf("generate payment QR filename: %w", err)
	}
	objectKey := filepath.ToSlash(filepath.Join(
		"agent-payment-qrcodes",
		strconv.FormatInt(agentID, 10),
		time.Now().UTC().Format("20060102T150405Z")+"_"+token+ext,
	))
	path, err := agentPaymentObjectPath(objectKey)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create payment QR directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create payment QR file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(upload.Body, upload.Size+1))
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("save payment QR file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close payment QR file: %w", closeErr)
	}
	if written != upload.Size {
		_ = os.Remove(path)
		return nil, infraerrors.BadRequest("PAYMENT_QR_INVALID_SIZE", "payment QR file size does not match upload size")
	}

	profile, err := s.paymentRepo.UpdateAgentPaymentQRCode(ctx, agentID, objectKey, upload.ContentType, filepath.Base(upload.Filename), upload.Size)
	if err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("update payment QR profile: %w", err)
	}
	if profile == nil {
		profile = emptyAgentPaymentProfile(agentID)
	}
	normalizeAgentPaymentProfile(profile)
	return profile, nil
}

func (s *CommissionService) GetAgentPaymentQRCodeFile(ctx context.Context, agentID int64) (*AgentPaymentQRCodeFile, error) {
	profile, err := s.GetAgentPaymentProfile(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(profile.AlipayQRCodeObjectKey) == "" {
		return nil, infraerrors.NotFound("PAYMENT_QR_NOT_FOUND", "payment QR file not found")
	}
	path, err := agentPaymentObjectPath(profile.AlipayQRCodeObjectKey)
	if err != nil {
		return nil, err
	}
	return &AgentPaymentQRCodeFile{
		Path:        path,
		ContentType: profile.AlipayQRCodeContentType,
		Filename:    profile.AlipayQRCodeOriginalName,
	}, nil
}

func (s *CommissionService) ListAdminAgentSettlementCandidates(ctx context.Context, params pagination.PaginationParams, filters AdminAgentListFilters) ([]AdminAgentSummary, *pagination.PaginationResult, error) {
	return s.ListAdminAgents(ctx, params, filters)
}

func (s *CommissionService) getAgentSettlementSettings(ctx context.Context) *AgentSettlementSettings {
	settings, err := s.GetAgentSettlementSettings(ctx)
	if err != nil || settings == nil || settings.MinimumAmount <= 0 {
		return defaultAgentSettlementSettings()
	}
	return settings
}

func defaultAgentSettlementSettings() *AgentSettlementSettings {
	return &AgentSettlementSettings{MinimumAmount: defaultAgentSettlementMinimumAmount}
}

func emptyAgentPaymentProfile(agentID int64) *AgentPaymentProfile {
	profile := &AgentPaymentProfile{AgentID: agentID}
	normalizeAgentPaymentProfile(profile)
	return profile
}

func normalizeAgentPaymentProfile(profile *AgentPaymentProfile) {
	if profile == nil {
		return
	}
	profile.AlipayRealName = strings.TrimSpace(profile.AlipayRealName)
	profile.AlipayAccount = strings.TrimSpace(profile.AlipayAccount)
	profile.ContactPhone = strings.TrimSpace(profile.ContactPhone)
	profile.PaymentNote = strings.TrimSpace(profile.PaymentNote)
	profile.AlipayQRCodeObjectKey = strings.TrimSpace(profile.AlipayQRCodeObjectKey)
	profile.AlipayQRCodeContentType = strings.TrimSpace(profile.AlipayQRCodeContentType)
	profile.AlipayQRCodeOriginalName = strings.TrimSpace(profile.AlipayQRCodeOriginalName)
	profile.IdentityFingerprintHash = strings.TrimSpace(profile.IdentityFingerprintHash)
	profile.VerificationStatus = strings.TrimSpace(profile.VerificationStatus)
	profile.VerificationNote = strings.TrimSpace(profile.VerificationNote)
	if profile.VerificationStatus == "" {
		profile.VerificationStatus = "incomplete"
	}
	profile.HasAlipayQRCode = profile.AlipayQRCodeObjectKey != ""
	profile.Complete = profile.AlipayRealName != "" && profile.AlipayAccount != "" && profile.HasAlipayQRCode
	profile.Verified = profile.Complete && profile.VerificationStatus == "verified"
}

func agentPaymentIdentityFingerprint(realName, account string) string {
	realName = strings.ToLower(strings.TrimSpace(realName))
	account = strings.ToLower(strings.TrimSpace(account))
	if realName == "" || account == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(realName + "\x00" + account))
	return hex.EncodeToString(sum[:])
}

func settlementGap(unsettled, minimum float64) float64 {
	gap := minimum - unsettled
	if gap < 0 {
		return 0
	}
	return gap
}

func agentPaymentObjectPath(objectKey string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(objectKey))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", infraerrors.BadRequest("INVALID_PAYMENT_QR_PATH", "invalid payment QR path")
	}
	return filepath.Join(agentPaymentDataDir(), "uploads", clean), nil
}

func agentPaymentDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("DATA_DIR")); dir != "" {
		return dir
	}
	if info, err := os.Stat("/app/data"); err == nil && info.IsDir() {
		return "/app/data"
	}
	return "."
}

func agentPaymentQRExt(filename, contentType string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return ext, true
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

func randomPaymentHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
