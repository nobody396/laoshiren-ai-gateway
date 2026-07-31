package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const (
	AffiliateRiskStatusClear   = "clear"
	AffiliateRiskStatusReview  = "review"
	AffiliateRiskStatusBlocked = "blocked"
)

var (
	ErrAffiliateRiskAgentNotFound = infraerrors.NotFound(
		"AFFILIATE_RISK_AGENT_NOT_FOUND",
		"affiliate agent principal not found",
	)
	ErrAffiliatePerformanceEventNotFound = infraerrors.NotFound(
		"AFFILIATE_PERFORMANCE_EVENT_NOT_FOUND",
		"affiliate performance event not found",
	)
	ErrAffiliatePerformanceEventNotReversible = infraerrors.Conflict(
		"AFFILIATE_PERFORMANCE_EVENT_NOT_REVERSIBLE",
		"affiliate performance event cannot be reversed",
	)
)

type AffiliateRiskPrincipal struct {
	AgentID                         int64      `json:"agent_id"`
	Email                           string     `json:"email"`
	Username                        string     `json:"username"`
	AgentStatus                     string     `json:"agent_status"`
	RiskStatus                      string     `json:"risk_status"`
	RiskNote                        string     `json:"risk_note"`
	SelfCommissionEnabled           bool       `json:"self_commission_enabled"`
	SelfCommissionRateBPS           int        `json:"self_commission_rate_bps"`
	SelfCommissionEffectiveAt       *time.Time `json:"self_commission_effective_at,omitempty"`
	SelfCommissionRevision          int64      `json:"self_commission_revision"`
	SelfCommissionReason            string     `json:"self_commission_reason,omitempty"`
	SelfCommissionUpdatedBy         *int64     `json:"self_commission_updated_by,omitempty"`
	SelfCommissionUpdatedByEmail    string     `json:"self_commission_updated_by_email,omitempty"`
	SelfCommissionUpdatedByUsername string     `json:"self_commission_updated_by_username,omitempty"`
	SelfCommissionUpdatedAt         *time.Time `json:"self_commission_updated_at,omitempty"`
	HasUpstream                     bool       `json:"has_upstream"`
	SelfCommissionEligible          bool       `json:"self_commission_eligible"`
	SelfCommissionBlockReason       string     `json:"self_commission_block_reason,omitempty"`
	HeldRewardCount                 int32      `json:"held_reward_count"`
	HeldRewardMicros                int64      `json:"held_reward_micros"`
	HeldCashCount                   int32      `json:"held_cash_count"`
	HeldCashMicros                  int64      `json:"held_cash_micros"`
	UpdatedAt                       time.Time  `json:"updated_at"`
}

type AffiliateRiskActionResult struct {
	ID                   int64     `json:"id"`
	AgentID              int64     `json:"agent_id"`
	PreviousRiskStatus   string    `json:"previous_risk_status"`
	NextRiskStatus       string    `json:"next_risk_status"`
	Reason               string    `json:"reason"`
	ReleasedRewardCount  int32     `json:"released_reward_count"`
	ReleasedRewardMicros int64     `json:"released_reward_micros"`
	ReleasedCashCount    int32     `json:"released_cash_count"`
	ReleasedCashMicros   int64     `json:"released_cash_micros"`
	AffectedCreditUsers  []int64   `json:"-"`
	CreatedAt            time.Time `json:"created_at"`
}

type AffiliatePerformanceReversal struct {
	ID                   int64     `json:"id"`
	OriginalEventID      int64     `json:"original_event_id"`
	ReversalEventID      int64     `json:"reversal_event_id"`
	ConsumerUserID       int64     `json:"consumer_user_id"`
	DirectAgentID        *int64    `json:"direct_agent_id,omitempty"`
	AmountMicros         int64     `json:"amount_micros"`
	ReversedRewardMicros int64     `json:"reversed_reward_micros"`
	ReversedCashMicros   int64     `json:"reversed_cash_micros"`
	Reason               string    `json:"reason"`
	OperatorID           int64     `json:"operator_id"`
	AffectedCreditUsers  []int64   `json:"-"`
	CreatedAt            time.Time `json:"created_at"`
}

type AffiliateRiskRepository interface {
	ListAffiliateRiskPrincipals(ctx context.Context, limit int) ([]AffiliateRiskPrincipal, error)
	SetAffiliateAgentRisk(
		ctx context.Context,
		agentID int64,
		nextStatus string,
		reason string,
		operatorID int64,
	) (*AffiliateRiskActionResult, error)
	ReverseAffiliatePerformanceEvent(
		ctx context.Context,
		eventID int64,
		reason string,
		operatorID int64,
	) (*AffiliatePerformanceReversal, error)
}

type AffiliateRiskService struct {
	repo         AffiliateRiskRepository
	balanceCache interface {
		InvalidateUserBalance(ctx context.Context, userID int64) error
	}
}

func NewAffiliateRiskService(repo AffiliateRiskRepository) *AffiliateRiskService {
	return &AffiliateRiskService{repo: repo}
}

func (s *AffiliateRiskService) SetBalanceCache(cache *BillingCacheService) {
	if s != nil {
		s.balanceCache = cache
	}
}

func (s *AffiliateRiskService) List(
	ctx context.Context,
	limit int,
) ([]AffiliateRiskPrincipal, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate risk repository is not configured")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListAffiliateRiskPrincipals(ctx, limit)
}

func (s *AffiliateRiskService) SetAgentRisk(
	ctx context.Context,
	agentID int64,
	nextStatus string,
	reason string,
	operatorID int64,
) (*AffiliateRiskActionResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate risk repository is not configured")
	}
	nextStatus = strings.TrimSpace(strings.ToLower(nextStatus))
	reason = strings.TrimSpace(reason)
	if agentID <= 0 || operatorID <= 0 || !isAffiliateRiskStatus(nextStatus) {
		return nil, ErrInvalidInput
	}
	if reason == "" || len([]rune(reason)) > 500 {
		return nil, fmt.Errorf("%w: risk reason must be 1 to 500 characters", ErrInvalidInput)
	}
	result, err := s.repo.SetAffiliateAgentRisk(ctx, agentID, nextStatus, reason, operatorID)
	if err != nil {
		return nil, err
	}
	s.invalidateCreditUsers(ctx, result.AffectedCreditUsers)
	return result, nil
}

func (s *AffiliateRiskService) ReversePerformanceEvent(
	ctx context.Context,
	eventID int64,
	reason string,
	operatorID int64,
) (*AffiliatePerformanceReversal, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate risk repository is not configured")
	}
	reason = strings.TrimSpace(reason)
	if eventID <= 0 || operatorID <= 0 {
		return nil, ErrInvalidInput
	}
	if reason == "" || len([]rune(reason)) > 500 {
		return nil, fmt.Errorf("%w: reversal reason must be 1 to 500 characters", ErrInvalidInput)
	}
	result, err := s.repo.ReverseAffiliatePerformanceEvent(ctx, eventID, reason, operatorID)
	if err != nil {
		return nil, err
	}
	s.invalidateCreditUsers(ctx, result.AffectedCreditUsers)
	return result, nil
}

func (s *AffiliateRiskService) invalidateCreditUsers(ctx context.Context, userIDs []int64) {
	if s == nil || s.balanceCache == nil {
		return
	}
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		if err := s.balanceCache.InvalidateUserBalance(ctx, userID); err != nil {
			slog.Error("invalidate affiliate risk credit balance failed", "user_id", userID, "error", err)
		}
	}
}

func isAffiliateRiskStatus(status string) bool {
	switch status {
	case AffiliateRiskStatusClear, AffiliateRiskStatusReview, AffiliateRiskStatusBlocked:
		return true
	default:
		return false
	}
}
