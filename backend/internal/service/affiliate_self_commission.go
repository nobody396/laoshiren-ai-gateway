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
		"未找到该合伙人",
	)
	ErrAffiliateSelfCommissionRevisionConflict = infraerrors.Conflict(
		"AFFILIATE_SELF_COMMISSION_POLICY_REVISION_CONFLICT",
		"本人消费返佣设置已被其他操作更新，请刷新后重试",
	)
	ErrAffiliateSelfCommissionNotEligible = infraerrors.Conflict(
		"AFFILIATE_SELF_COMMISSION_NOT_ELIGIBLE",
		"该合伙人暂不符合本人消费返佣开通条件",
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
