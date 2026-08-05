package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
)

const AffiliateDefaultCustomerRebateRateBPS int32 = 500

// AffiliateAgentAutoActivationNote marks applications approved by the
// qualify-and-activate flow so they stay distinguishable from manual reviews.
const AffiliateAgentAutoActivationNote = "auto"

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
	// ErrAffiliateActivationConflict marks a serialization/deadlock race
	// between concurrent activation attempts. Every activation write is
	// idempotent, so callers should simply retry the whole transaction.
	ErrAffiliateActivationConflict = infraerrors.Conflict(
		"AFFILIATE_ACTIVATION_CONFLICT",
		"affiliate agent activation raced with a concurrent update",
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
	// NewlyActivated reports whether this call performed a fresh activation
	// (as opposed to the idempotent repair path). Internal only: it drives
	// one-time side effects such as the activation email.
	NewlyActivated bool `json:"-"`
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
	AutoActivateAgent(
		ctx context.Context,
		userID int64,
		applicationNote string,
		defaultCode string,
		defaultCustomerRebateRateBPS int32,
	) (*AffiliateAgentReviewResult, error)
}

type AffiliateAgentService struct {
	repo AffiliateAgentRepository

	userRepo   affiliateActivationUserLookup
	settings   affiliateActivationSettings
	emailQueue affiliateActivationEmailQueue
}

// affiliateActivationSettings narrows SettingService to what the activation
// email needs, keeping the dependency fakeable in tests.
type affiliateActivationSettings interface {
	GetFrontendURL(ctx context.Context) string
	GetSiteName(ctx context.Context) string
}

// affiliateActivationUserLookup narrows UserRepository to the single read the
// activation email needs, keeping the dependency fakeable in tests.
type affiliateActivationUserLookup interface {
	GetByID(ctx context.Context, id int64) (*User, error)
}

// affiliateActivationEmailQueue narrows EmailQueueService to the activation
// email task, keeping the dependency fakeable in tests.
type affiliateActivationEmailQueue interface {
	EnqueueAffiliateAgentActivated(email, siteName, partnerCenterURL string) error
}

func NewAffiliateAgentService(repo AffiliateAgentRepository) *AffiliateAgentService {
	return &AffiliateAgentService{repo: repo}
}

// SetActivationNotificationDeps wires the optional activation email
// dependencies. Without them activation still succeeds; only the email is
// skipped (the in-app notice is written inside the activation transaction).
func (s *AffiliateAgentService) SetActivationNotificationDeps(
	userRepo affiliateActivationUserLookup,
	settings affiliateActivationSettings,
	emailQueue affiliateActivationEmailQueue,
) {
	s.userRepo = userRepo
	s.settings = settings
	s.emailQueue = emailQueue
}

// enqueueActivationEmail sends the "partner activated" email after the
// activation transaction committed. Failures are logged-and-forget, matching
// the feedback reply email path.
func (s *AffiliateAgentService) enqueueActivationEmail(ctx context.Context, userID int64) {
	if s == nil || s.userRepo == nil || s.settings == nil || s.emailQueue == nil {
		return
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil || strings.TrimSpace(user.Email) == "" {
		return
	}
	partnerCenterURL := strings.TrimRight(s.settings.GetFrontendURL(ctx), "/") + "/affiliate"
	_ = s.emailQueue.EnqueueAffiliateAgentActivated(
		user.Email,
		s.settings.GetSiteName(ctx),
		partnerCenterURL,
	)
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
	qualification, err := s.repo.GetAgentQualification(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !affiliateAutoActivationEligible(qualification) {
		return qualification, nil
	}
	if _, err := s.activateQualifiedAgent(ctx, userID, ""); err != nil {
		if isAffiliateActivationStateError(err) {
			// State changed between the read and the activation transaction
			// (e.g. a concurrent activation won). Return the fresh snapshot.
			return s.repo.GetAgentQualification(ctx, userID)
		}
		return nil, err
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
	qualification, err := s.repo.GetAgentQualification(ctx, userID)
	if err != nil {
		return nil, err
	}
	if affiliateAutoActivationEligible(qualification) {
		result, err := s.activateQualifiedAgent(ctx, userID, note)
		if err != nil {
			if isAffiliateActivationStateError(err) {
				// State changed between the read and the activation
				// transaction; fall back to the legacy submit path so the
				// caller still gets the canonical error for the new state.
				return s.repo.SubmitAgentApplication(ctx, userID, note)
			}
			return nil, err
		}
		if result != nil && result.Application.ID > 0 {
			return &result.Application, nil
		}
		// Already activated by a concurrent request without any application
		// row on record; there is nothing left to submit.
		return nil, ErrAffiliateAgentActivationBlocked
	}
	return s.repo.SubmitAgentApplication(ctx, userID, note)
}

// ActivateQualifiedCandidates activates every qualified-but-unactivated user
// (the same set ListQualifiedCandidates reports) through the exact same path
// as the lazy GET/apply triggers, so notices and emails stay idempotent. Each
// user runs in an independent transaction; one failure is logged and skipped
// without blocking the rest of the batch.
func (s *AffiliateAgentService) ActivateQualifiedCandidates(
	ctx context.Context,
	limit int,
) (scanned, activated, failed int, err error) {
	if s == nil || s.repo == nil {
		return 0, 0, 0, errors.New("affiliate agent repository is not configured")
	}
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	candidates, err := s.repo.ListQualifiedCandidates(ctx, limit)
	if err != nil {
		return 0, 0, 0, err
	}
	scanned = len(candidates)
	for _, candidate := range candidates {
		result, activateErr := s.activateQualifiedAgent(ctx, candidate.UserID, "")
		switch {
		case activateErr == nil && result != nil && result.NewlyActivated:
			activated++
		case activateErr == nil || isAffiliateActivationStateError(activateErr):
			// Already activated by a concurrent trigger, or the state flipped
			// between listing and activation: both are harmless no-ops.
		default:
			failed++
			logger.LegacyPrintf(
				"service.affiliate_activation",
				"[AffiliateActivation] Failed to activate user %d: %v",
				candidate.UserID, activateErr,
			)
		}
	}
	return scanned, activated, failed, nil
}

// affiliateAutoActivationEligible reports whether viewing the qualification
// page or calling the apply endpoint should activate the user immediately.
// Pending-review applications are eligible on purpose: auto activation
// converges them to approved instead of leaving them queued for a manual
// review that no longer exists.
func affiliateAutoActivationEligible(q *AffiliateAgentQualification) bool {
	if q == nil {
		return false
	}
	if q.ProgramMode != AffiliateProgramModeLive ||
		q.ProgramStartedAt == nil ||
		!q.Qualified ||
		q.RiskStatus != "clear" {
		return false
	}
	switch q.AgentStatus {
	case "active", "suspended", "terminated", "rejected":
		return false
	}
	return true
}

// isAffiliateActivationStateError marks errors caused by legitimate state
// changes between the eligibility read and the activation transaction, as
// opposed to real failures (conflicts, db errors) that callers should see.
func isAffiliateActivationStateError(err error) bool {
	return errors.Is(err, ErrAffiliateQualificationNotMet) ||
		errors.Is(err, ErrAffiliateAgentActivationBlocked) ||
		errors.Is(err, ErrAffiliateProgramNotLive) ||
		errors.Is(err, ErrUserNotFound)
}

// activateQualifiedAgent runs the shared auto-activation path for both user
// entry points, minting the default link exactly like an admin approval:
// "A" + random invite code, default customer rebate rate, retry on collision.
func (s *AffiliateAgentService) activateQualifiedAgent(
	ctx context.Context,
	userID int64,
	applicationNote string,
) (*AffiliateAgentReviewResult, error) {
	for attempt := 0; attempt < 10; attempt++ {
		code, err := generateInviteCode()
		if err != nil {
			return nil, fmt.Errorf("generate default affiliate link: %w", err)
		}
		result, err := s.repo.AutoActivateAgent(
			ctx,
			userID,
			applicationNote,
			"A"+code,
			AffiliateDefaultCustomerRebateRateBPS,
		)
		if errors.Is(err, ErrAffiliateLinkConflict) || errors.Is(err, ErrAffiliateActivationConflict) {
			continue
		}
		if err == nil && result != nil && result.NewlyActivated {
			s.enqueueActivationEmail(ctx, userID)
		}
		return result, err
	}
	return nil, ErrAffiliateLinkConflict
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
		if err == nil && result != nil && result.NewlyActivated {
			s.enqueueActivationEmail(ctx, result.Application.UserID)
		}
		return result, err
	}
	return nil, ErrAffiliateLinkConflict
}
