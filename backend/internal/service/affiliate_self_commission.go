package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const AffiliateSelfCommissionRateBPS = 1000

const (
	AffiliateSelfCommissionBlockNone           = ""
	AffiliateSelfCommissionBlockHasUpstream    = "has_upstream"
	AffiliateSelfCommissionBlockAgentNotActive = "agent_not_active"
	AffiliateSelfCommissionBlockRiskNotClear   = "risk_not_clear"
)

var (
	ErrAffiliateSelfCommissionAgentNotFound = infraerrors.NotFound(
		"AFFILIATE_SELF_COMMISSION_AGENT_NOT_FOUND",
		"affiliate partner not found",
	)
	ErrAffiliateSelfCommissionRevisionConflict = infraerrors.Conflict(
		"AFFILIATE_SELF_COMMISSION_POLICY_REVISION_CONFLICT",
		"self-commission policy changed; reload and retry",
	)
	ErrAffiliateSelfCommissionNotEligible = infraerrors.Conflict(
		"AFFILIATE_SELF_COMMISSION_NOT_ELIGIBLE",
		"partner is not eligible for self-consumption commission",
	)
)

// AffiliateSelfCommissionPolicy is the current per-partner control state.
// A missing database row is represented as disabled with revision zero.
type AffiliateSelfCommissionPolicy struct {
	AgentID         int64      `json:"agent_id"`
	Enabled         bool       `json:"enabled"`
	RateBPS         int        `json:"rate_bps"`
	EffectiveAt     *time.Time `json:"effective_at,omitempty"`
	Revision        int64      `json:"revision"`
	UpdatedBy       *int64     `json:"updated_by,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	HasUpstream     bool       `json:"has_upstream"`
	Eligible        bool       `json:"eligible"`
	BlockReasonCode string     `json:"block_reason_code,omitempty"`
}

type AffiliateSelfCommissionPolicyRepository interface {
	SetAffiliateSelfCommissionPolicy(
		ctx context.Context,
		agentID int64,
		enabled bool,
		expectedRevision int64,
		reason string,
		operatorID int64,
	) (*AffiliateSelfCommissionPolicy, error)
}

type AffiliateSelfCommissionPolicyService struct {
	repo AffiliateSelfCommissionPolicyRepository
}

func NewAffiliateSelfCommissionPolicyService(
	repo AffiliateSelfCommissionPolicyRepository,
) *AffiliateSelfCommissionPolicyService {
	return &AffiliateSelfCommissionPolicyService{repo: repo}
}

func (s *AffiliateSelfCommissionPolicyService) Update(
	ctx context.Context,
	agentID int64,
	enabled bool,
	expectedRevision int64,
	reason string,
	operatorID int64,
) (*AffiliateSelfCommissionPolicy, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate self-commission policy repository is not configured")
	}
	reason = strings.TrimSpace(reason)
	if agentID <= 0 || operatorID <= 0 || expectedRevision < 0 {
		return nil, ErrInvalidInput
	}
	if reason == "" || len([]rune(reason)) > 500 {
		return nil, fmt.Errorf("%w: policy reason must be 1 to 500 characters", ErrInvalidInput)
	}
	return s.repo.SetAffiliateSelfCommissionPolicy(
		ctx,
		agentID,
		enabled,
		expectedRevision,
		reason,
		operatorID,
	)
}
