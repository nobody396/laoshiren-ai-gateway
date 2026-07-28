package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var ErrUsageBillingRequestIDRequired = errors.New("usage billing request_id is required")
var ErrUsageBillingRequestConflict = errors.New("usage billing request fingerprint conflict")

const SubscriptionSharedQuotaNoteKey = "shared_quota="

// UsageBillingCommand describes one billable request that must be applied at most once.
type UsageBillingCommand struct {
	RequestID          string
	APIKeyID           int64
	UsageLogID         int64
	RequestFingerprint string
	RequestPayloadHash string

	UserID              int64
	AccountID           int64
	SubscriptionID      *int64
	AccountType         string
	Model               string
	ServiceTier         string
	ReasoningEffort     string
	BillingType         int8
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	ImageCount          int
	MediaType           string

	BalanceCost         float64
	SubscriptionCost    float64
	APIKeyQuotaCost     float64
	APIKeyRateLimitCost float64
	AccountQuotaCost    float64
}

func (c *UsageBillingCommand) Normalize() {
	if c == nil {
		return
	}
	c.RequestID = strings.TrimSpace(c.RequestID)
	if strings.TrimSpace(c.RequestFingerprint) == "" {
		c.RequestFingerprint = buildUsageBillingFingerprint(c)
	}
}

func buildUsageBillingFingerprint(c *UsageBillingCommand) string {
	if c == nil {
		return ""
	}
	raw := fmt.Sprintf(
		"%d|%d|%d|%s|%s|%s|%s|%d|%d|%d|%d|%d|%d|%s|%d|%0.10f|%0.10f|%0.10f|%0.10f|%0.10f",
		c.UserID,
		c.AccountID,
		c.APIKeyID,
		strings.TrimSpace(c.AccountType),
		strings.TrimSpace(c.Model),
		strings.TrimSpace(c.ServiceTier),
		strings.TrimSpace(c.ReasoningEffort),
		c.BillingType,
		c.InputTokens,
		c.OutputTokens,
		c.CacheCreationTokens,
		c.CacheReadTokens,
		c.ImageCount,
		strings.TrimSpace(c.MediaType),
		valueOrZero(c.SubscriptionID),
		c.BalanceCost,
		c.SubscriptionCost,
		c.APIKeyQuotaCost,
		c.APIKeyRateLimitCost,
		c.AccountQuotaCost,
	)
	if payloadHash := strings.TrimSpace(c.RequestPayloadHash); payloadHash != "" {
		raw += "|" + payloadHash
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func HashUsageRequestPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func valueOrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// AccountQuotaState holds the post-increment quota state returned by the DB transaction.
// All values are post-update (i.e., already include the increment).
type AccountQuotaState struct {
	TotalUsed   float64
	TotalLimit  float64
	DailyUsed   float64
	DailyLimit  float64
	WeeklyUsed  float64
	WeeklyLimit float64
}

type UsageBillingApplyResult struct {
	Applied              bool
	APIKeyQuotaExhausted bool
	NewBalance           *float64
	QuotaState           *AccountQuotaState

	SubscriptionUsageUpdates       []SubscriptionUsageUpdate
	BalanceConfirmedMicros         int64
	MonthlyConfirmedMicros         int64
	ConfirmedConsumptionMicros     int64
	AffiliateProgramMode           string
	AffiliateCustomerRebateMicros  int64
	AffiliateAgentCommissionMicros int64
}

type SubscriptionUsageUpdate struct {
	UserID  int64
	GroupID int64
	CostUSD float64
}

type UsageBillingRepository interface {
	Apply(ctx context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error)
}

func SubscriptionRedeemSharedQuotaMarker(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	return "redeem:" + code
}

func SubscriptionSharedQuotaMarkerFromNotes(notes string) string {
	idx := strings.LastIndex(notes, SubscriptionSharedQuotaNoteKey)
	if idx < 0 {
		return subscriptionRedeemSharedQuotaMarkerFromLegacyNotes(notes)
	}
	marker := strings.TrimSpace(notes[idx+len(SubscriptionSharedQuotaNoteKey):])
	if marker == "" {
		return ""
	}
	for i, r := range marker {
		if unicode.IsSpace(r) || r == ';' || r == ',' {
			return strings.TrimSpace(marker[:i])
		}
	}
	return marker
}

func subscriptionRedeemSharedQuotaMarkerFromLegacyNotes(notes string) string {
	const (
		prefix = "通过兑换码 "
		suffix = " 兑换"
	)
	idx := strings.LastIndex(notes, prefix)
	if idx < 0 {
		return ""
	}
	codeStart := idx + len(prefix)
	codeEnd := strings.Index(notes[codeStart:], suffix)
	if codeEnd < 0 {
		return ""
	}
	return SubscriptionRedeemSharedQuotaMarker(notes[codeStart : codeStart+codeEnd])
}
