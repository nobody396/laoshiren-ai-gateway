package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const AffiliateDefaultCustomerRebateRateBPS int32 = 500

var (
	ErrAffiliateProgramNotLive = infraerrors.Conflict(
		"AFFILIATE_PROGRAM_NOT_LIVE",
		"affiliate program is not live",
	)
	ErrAffiliateQualificationNotMet = infraerrors.Conflict(
		"AFFILIATE_QUALIFICATION_NOT_MET",
		"affiliate agent qualification is not met",
	)
	ErrAffiliateAgentActivationBlocked = infraerrors.Forbidden(
		"AFFILIATE_AGENT_ACTIVATION_BLOCKED",
		"affiliate agent activation is blocked",
	)
)

type AffiliateAgentQualification struct {
	UserID                      int64      `json:"user_id"`
	ProgramMode                 string     `json:"program_mode"`
	ProgramStartedAt            *time.Time `json:"program_started_at,omitempty"`
	AgentStatus                 string     `json:"agent_status"`
	RiskStatus                  string     `json:"risk_status"`
	SelfConsumptionMicros       int64      `json:"self_consumption_micros"`
	DirectTeamConsumptionMicros int64      `json:"direct_team_consumption_micros"`
	CombinedConsumptionMicros   int64      `json:"combined_consumption_micros"`
	ValidDirectUserCount        int32      `json:"valid_direct_user_count"`
	RequiredDirectUserCount     int32      `json:"required_direct_user_count"`
	RequiredPerUserMicros       int64      `json:"required_per_user_micros"`
	RequiredDirectTeamMicros    int64      `json:"required_direct_team_micros"`
	RequiredCombinedMicros      int64      `json:"required_combined_micros"`
	DirectRouteQualified        bool       `json:"direct_route_qualified"`
	CombinedRouteQualified      bool       `json:"combined_route_qualified"`
	Qualified                   bool       `json:"qualified"`
	QualificationRoute          string     `json:"qualification_route,omitempty"`
	CanActivate                 bool       `json:"can_activate"`
	ActivatedAt                 *time.Time `json:"activated_at,omitempty"`
}

type AffiliateAgentActivation struct {
	Qualification AffiliateAgentQualification `json:"qualification"`
	DefaultLink   AffiliateLink               `json:"default_link"`
}

type AffiliateAgentRepository interface {
	GetAgentQualification(ctx context.Context, userID int64) (*AffiliateAgentQualification, error)
	ActivateQualifiedAgent(
		ctx context.Context,
		userID int64,
		defaultCode string,
		defaultCustomerRebateRateBPS int32,
	) (*AffiliateAgentActivation, error)
}

type AffiliateAgentService struct {
	repo AffiliateAgentRepository
}

func NewAffiliateAgentService(repo AffiliateAgentRepository) *AffiliateAgentService {
	return &AffiliateAgentService{repo: repo}
}

func (s *AffiliateAgentService) GetQualification(
	ctx context.Context,
	userID int64,
) (*AffiliateAgentQualification, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	if userID <= 0 {
		return nil, ErrInvalidInput
	}
	return s.repo.GetAgentQualification(ctx, userID)
}

func (s *AffiliateAgentService) Activate(
	ctx context.Context,
	userID int64,
) (*AffiliateAgentActivation, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	if userID <= 0 {
		return nil, ErrInvalidInput
	}
	for attempt := 0; attempt < 10; attempt++ {
		code, err := generateInviteCode()
		if err != nil {
			return nil, fmt.Errorf("generate default affiliate link: %w", err)
		}
		activation, err := s.repo.ActivateQualifiedAgent(
			ctx,
			userID,
			"A"+code,
			AffiliateDefaultCustomerRebateRateBPS,
		)
		if errors.Is(err, ErrAffiliateLinkConflict) {
			continue
		}
		return activation, err
	}
	return nil, ErrAffiliateLinkConflict
}
