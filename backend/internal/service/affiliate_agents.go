package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	ErrAffiliateApplicationPending = infraerrors.Conflict(
		"AFFILIATE_APPLICATION_PENDING",
		"partner application is already under review",
	)
	ErrAffiliateApplicationNotFound = infraerrors.NotFound(
		"AFFILIATE_APPLICATION_NOT_FOUND",
		"partner application not found",
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
	CanApply                    bool       `json:"can_apply"`
	ActivatedAt                 *time.Time `json:"activated_at,omitempty"`
}

type AffiliateAgentActivation struct {
	Qualification AffiliateAgentQualification `json:"qualification"`
	DefaultLink   AffiliateLink               `json:"default_link"`
}

type AffiliateAgentApplication struct {
	ID                          int64      `json:"id"`
	UserID                      int64      `json:"user_id"`
	Email                       string     `json:"email,omitempty"`
	Username                    string     `json:"username,omitempty"`
	Status                      string     `json:"status"`
	QualifyingRoute             string     `json:"qualifying_route"`
	ValidDirectUserCount        int32      `json:"valid_direct_user_count"`
	DirectTeamConsumptionMicros int64      `json:"direct_team_consumption_micros"`
	ApplicationNote             string     `json:"application_note"`
	DecisionNote                string     `json:"decision_note"`
	SubmittedAt                 time.Time  `json:"submitted_at"`
	ReviewedAt                  *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy                  *int64     `json:"reviewed_by,omitempty"`
}

type AffiliateAgentReviewResult struct {
	Application AffiliateAgentApplication `json:"application"`
	Activation  *AffiliateAgentActivation `json:"activation,omitempty"`
}

type AffiliateAgentRepository interface {
	GetAgentQualification(ctx context.Context, userID int64) (*AffiliateAgentQualification, error)
	SubmitAgentApplication(
		ctx context.Context,
		userID int64,
		note string,
	) (*AffiliateAgentApplication, error)
	ListAgentApplications(
		ctx context.Context,
		status string,
		limit int,
	) ([]AffiliateAgentApplication, error)
	ReviewAgentApplication(
		ctx context.Context,
		applicationID int64,
		approve bool,
		decisionNote string,
		operatorID int64,
		defaultCode string,
		defaultCustomerRebateRateBPS int32,
	) (*AffiliateAgentReviewResult, error)
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

func (s *AffiliateAgentService) Apply(
	ctx context.Context,
	userID int64,
	note string,
) (*AffiliateAgentApplication, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	if userID <= 0 {
		return nil, ErrInvalidInput
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > 500 {
		return nil, ErrInvalidInput
	}
	return s.repo.SubmitAgentApplication(ctx, userID, note)
}

func (s *AffiliateAgentService) ListApplications(
	ctx context.Context,
	status string,
	limit int,
) ([]AffiliateAgentApplication, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "pending_review"
	}
	switch status {
	case "pending_review", "approved", "rejected", "cancelled", "all":
	default:
		return nil, ErrInvalidInput
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListAgentApplications(ctx, status, limit)
}

func (s *AffiliateAgentService) ReviewApplication(
	ctx context.Context,
	applicationID int64,
	approve bool,
	decisionNote string,
	operatorID int64,
) (*AffiliateAgentReviewResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	decisionNote = strings.TrimSpace(decisionNote)
	if applicationID <= 0 || operatorID <= 0 || len([]rune(decisionNote)) > 500 {
		return nil, ErrInvalidInput
	}
	if !approve {
		return s.repo.ReviewAgentApplication(
			ctx,
			applicationID,
			false,
			decisionNote,
			operatorID,
			"",
			AffiliateDefaultCustomerRebateRateBPS,
		)
	}
	for attempt := 0; attempt < 10; attempt++ {
		code, err := generateInviteCode()
		if err != nil {
			return nil, fmt.Errorf("generate default affiliate link: %w", err)
		}
		result, err := s.repo.ReviewAgentApplication(
			ctx,
			applicationID,
			true,
			decisionNote,
			operatorID,
			"A"+code,
			AffiliateDefaultCustomerRebateRateBPS,
		)
		if errors.Is(err, ErrAffiliateLinkConflict) {
			continue
		}
		return result, err
	}
	return nil, ErrAffiliateLinkConflict
}
