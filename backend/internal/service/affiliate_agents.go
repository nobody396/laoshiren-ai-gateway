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
	ErrAffiliatePartnerPerformanceNotFound = infraerrors.NotFound(
		"AFFILIATE_PARTNER_PERFORMANCE_NOT_FOUND",
		"affiliate partner not found",
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
	RequiredSelfMicros          int64      `json:"required_self_micros"`
	DirectRouteQualified        bool       `json:"direct_route_qualified"`
	SelfRouteQualified          bool       `json:"self_route_qualified"`
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
	SelfConsumptionMicros       int64      `json:"self_consumption_micros"`
	DirectTeamConsumptionMicros int64      `json:"direct_team_consumption_micros"`
	CombinedConsumptionMicros   int64      `json:"combined_consumption_micros"`
	ApplicationNote             string     `json:"application_note"`
	DecisionNote                string     `json:"decision_note"`
	SubmittedAt                 time.Time  `json:"submitted_at"`
	ReviewedAt                  *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy                  *int64     `json:"reviewed_by,omitempty"`
}

// AffiliateQualifiedCandidate is a user who currently satisfies at least one
// partner qualification route but has not submitted an application yet.
// Listing candidates is read-only: it must never create a principal,
// application, reward, or commission entry.
type AffiliateQualifiedCandidate struct {
	UserID                      int64  `json:"user_id"`
	Email                       string `json:"email,omitempty"`
	Username                    string `json:"username,omitempty"`
	QualificationRoute          string `json:"qualification_route"`
	ValidDirectUserCount        int32  `json:"valid_direct_user_count"`
	SelfConsumptionMicros       int64  `json:"self_consumption_micros"`
	DirectTeamConsumptionMicros int64  `json:"direct_team_consumption_micros"`
	CombinedConsumptionMicros   int64  `json:"combined_consumption_micros"`
}

// AffiliateOperationsSummary contains queue counts used by the admin console
// and sidebar. ActionableTotal deliberately excludes qualified users (follow-up
// only) and historical paid withdrawals.
type AffiliateOperationsSummary struct {
	QualifiedFollowup      int64 `json:"qualified_followup"`
	PendingApplications    int64 `json:"pending_applications"`
	PendingPaymentProfiles int64 `json:"pending_payment_profiles"`
	ProcessingWithdrawals  int64 `json:"processing_withdrawals"`
	OverdueWithdrawals     int64 `json:"overdue_withdrawals"`
	AbnormalPartners       int64 `json:"abnormal_partners"`
	ActionableTotal        int64 `json:"actionable_total"`
}

type AffiliatePartnerPerformance struct {
	AgentID                     int64     `json:"agent_id"`
	Email                       string    `json:"email"`
	Username                    string    `json:"username"`
	ActivatedAt                 time.Time `json:"activated_at"`
	DirectUserCount             int64     `json:"direct_user_count"`
	PaidDirectUserCount         int64     `json:"paid_direct_user_count"`
	SelfRechargeMicros          int64     `json:"self_recharge_micros"`
	DirectTeamRechargeMicros    int64     `json:"direct_team_recharge_micros"`
	SelfConsumptionMicros       int64     `json:"self_consumption_micros"`
	DirectTeamConsumptionMicros int64     `json:"direct_team_consumption_micros"`
	Recent30dConsumptionMicros  int64     `json:"recent_30d_consumption_micros"`
	LifetimeEarnedMicros        int64     `json:"lifetime_earned_micros"`
	AvailableCommissionMicros   int64     `json:"available_commission_micros"`
	ProcessingWithdrawalMicros  int64     `json:"processing_withdrawal_micros"`
	PaidCommissionMicros        int64     `json:"paid_commission_micros"`
}

type AffiliatePartnerUserPerformance struct {
	UserID                    int64     `json:"user_id"`
	Email                     string    `json:"email"`
	Username                  string    `json:"username"`
	JoinedAt                  time.Time `json:"joined_at"`
	RechargeMicros            int64     `json:"recharge_micros"`
	ConsumptionMicros         int64     `json:"consumption_micros"`
	GeneratedCommissionMicros int64     `json:"generated_commission_micros"`
}

type AffiliatePartnerCommissionEntry struct {
	ID                     int64     `json:"id"`
	ConsumerUserID         int64     `json:"consumer_user_id"`
	EntryType              string    `json:"entry_type"`
	PostingStatus          string    `json:"posting_status"`
	AmountMicros           int64     `json:"amount_micros"`
	SourceAmountMicros     int64     `json:"source_amount_micros"`
	CustomerRebateRateBPS  int32     `json:"customer_rebate_rate_bps"`
	AgentCommissionRateBPS int32     `json:"agent_commission_rate_bps"`
	SourceType             string    `json:"source_type"`
	OccurredAt             time.Time `json:"occurred_at"`
}

type AffiliatePartnerWithdrawal struct {
	ID               int64      `json:"id"`
	AmountMicros     int64      `json:"amount_micros"`
	Status           string     `json:"status"`
	RequestedAt      time.Time  `json:"requested_at"`
	PaidAt           *time.Time `json:"paid_at,omitempty"`
	PaymentReference string     `json:"payment_reference,omitempty"`
	FailureReason    string     `json:"failure_reason,omitempty"`
}

type AffiliatePartnerPerformanceDetail struct {
	Summary          AffiliatePartnerPerformance       `json:"summary"`
	PeriodStart      time.Time                         `json:"period_start"`
	PeriodEnd        time.Time                         `json:"period_end"`
	DirectUsers      []AffiliatePartnerUserPerformance `json:"direct_users"`
	CommissionLedger []AffiliatePartnerCommissionEntry `json:"commission_ledger"`
	Withdrawals      []AffiliatePartnerWithdrawal      `json:"withdrawals"`
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
	ListQualifiedCandidates(
		ctx context.Context,
		limit int,
	) ([]AffiliateQualifiedCandidate, error)
	GetOperationsSummary(ctx context.Context) (*AffiliateOperationsSummary, error)
	ListPartnerPerformance(ctx context.Context, limit int) ([]AffiliatePartnerPerformance, error)
	GetPartnerPerformance(
		ctx context.Context,
		agentID int64,
		start time.Time,
		end time.Time,
	) (*AffiliatePartnerPerformanceDetail, error)
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

func (s *AffiliateAgentService) GetOperationsSummary(ctx context.Context) (*AffiliateOperationsSummary, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	return s.repo.GetOperationsSummary(ctx)
}

func (s *AffiliateAgentService) ListPartnerPerformance(
	ctx context.Context,
	limit int,
) ([]AffiliatePartnerPerformance, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListPartnerPerformance(ctx, limit)
}

func (s *AffiliateAgentService) GetPartnerPerformance(
	ctx context.Context,
	agentID int64,
	start *time.Time,
	end *time.Time,
) (*AffiliatePartnerPerformanceDetail, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	if agentID <= 0 {
		return nil, ErrInvalidInput
	}
	periodEnd := time.Now().UTC()
	if end != nil {
		periodEnd = end.UTC()
	}
	periodStart := time.Time{}
	if start != nil {
		periodStart = start.UTC()
	}
	if !periodStart.IsZero() && !periodStart.Before(periodEnd) {
		return nil, ErrInvalidInput
	}
	return s.repo.GetPartnerPerformance(ctx, agentID, periodStart, periodEnd)
}

func (s *AffiliateAgentService) ListQualifiedCandidates(
	ctx context.Context,
	limit int,
) ([]AffiliateQualifiedCandidate, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate agent repository is not configured")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListQualifiedCandidates(ctx, limit)
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
