package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	AffiliateProgramVersionV2 = "v2"

	AffiliateProgramModeOff    = "off"
	AffiliateProgramModeShadow = "shadow"
	AffiliateProgramModeLive   = "live"

	AffiliateAgentPoolRateBPS = int32(1000)
	AffiliateRateStepBPS      = int32(100)
)

var (
	ErrAffiliateProgramSettingsInvalid  = errors.New("invalid affiliate program settings")
	ErrAffiliateProgramRevisionConflict = errors.New("affiliate program settings revision conflict")
	ErrAffiliateProgramModeTransition   = errors.New("invalid affiliate program mode transition")
)

// AffiliateProgramSettings is the fixed-point, versioned configuration for
// affiliate V2. Monetary values are micro-units and rates are basis points.
type AffiliateProgramSettings struct {
	ID                                       int16      `json:"id"`
	ProgramVersion                           string     `json:"program_version"`
	Mode                                     string     `json:"mode"`
	StartedAt                                *time.Time `json:"started_at,omitempty"`
	OrdinaryReferralRateBPS                  int32      `json:"ordinary_referral_rate_bps"`
	FirstPaidBonusThresholdMicros            int64      `json:"first_paid_bonus_threshold_micros"`
	FirstPaidBonusMicros                     int64      `json:"first_paid_bonus_micros"`
	AgentPoolRateBPS                         int32      `json:"agent_pool_rate_bps"`
	QualificationDirectUserCount             int32      `json:"qualification_direct_user_count"`
	QualificationMinUserConsumptionMicros    int64      `json:"qualification_min_user_consumption_micros"`
	QualificationDirectTeamConsumptionMicros int64      `json:"qualification_direct_team_consumption_micros"`
	QualificationCombinedConsumptionMicros   int64      `json:"qualification_combined_consumption_micros"`
	MaxCampaignLinks                         int32      `json:"max_campaign_links"`
	CommissionConversionMultiplierMillis     int32      `json:"commission_conversion_multiplier_millis"`
	WithdrawalMinMicros                      int64      `json:"withdrawal_min_micros"`
	WithdrawalSLAHours                       int32      `json:"withdrawal_sla_hours"`
	MarginFloorBPS                           int32      `json:"margin_floor_bps"`
	Revision                                 int64      `json:"revision"`
	UpdatedBy                                *int64     `json:"updated_by,omitempty"`
	CreatedAt                                time.Time  `json:"created_at"`
	UpdatedAt                                time.Time  `json:"updated_at"`
}

func DefaultAffiliateProgramSettings() AffiliateProgramSettings {
	return AffiliateProgramSettings{
		ID:                                       1,
		ProgramVersion:                           AffiliateProgramVersionV2,
		Mode:                                     AffiliateProgramModeOff,
		OrdinaryReferralRateBPS:                  500,
		FirstPaidBonusThresholdMicros:            50_000_000,
		FirstPaidBonusMicros:                     5_000_000,
		AgentPoolRateBPS:                         AffiliateAgentPoolRateBPS,
		QualificationDirectUserCount:             10,
		QualificationMinUserConsumptionMicros:    20_000_000,
		QualificationDirectTeamConsumptionMicros: 1_000_000_000,
		QualificationCombinedConsumptionMicros:   2_000_000_000,
		MaxCampaignLinks:                         5,
		CommissionConversionMultiplierMillis:     1200,
		WithdrawalMinMicros:                      100_000_000,
		WithdrawalSLAHours:                       24,
		MarginFloorBPS:                           3500,
		Revision:                                 1,
	}
}

func (s AffiliateProgramSettings) Validate() error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrAffiliateProgramSettingsInvalid, fmt.Sprintf(format, args...))
	}

	if s.ID != 1 {
		return invalid("id must be 1")
	}
	if s.ProgramVersion != AffiliateProgramVersionV2 {
		return invalid("program_version must be %q", AffiliateProgramVersionV2)
	}
	if !isAffiliateProgramMode(s.Mode) {
		return invalid("unsupported mode %q", s.Mode)
	}
	if s.Mode == AffiliateProgramModeLive && s.StartedAt == nil {
		return invalid("live mode requires started_at")
	}
	if s.OrdinaryReferralRateBPS < 0 || s.OrdinaryReferralRateBPS > AffiliateAgentPoolRateBPS {
		return invalid("ordinary referral rate must be between 0 and %d bps", AffiliateAgentPoolRateBPS)
	}
	if s.AgentPoolRateBPS != AffiliateAgentPoolRateBPS {
		return invalid("agent pool rate must remain fixed at %d bps", AffiliateAgentPoolRateBPS)
	}
	if s.FirstPaidBonusThresholdMicros < 0 || s.FirstPaidBonusMicros < 0 {
		return invalid("first-paid bonus values cannot be negative")
	}
	if s.QualificationDirectUserCount <= 0 ||
		s.QualificationMinUserConsumptionMicros <= 0 ||
		s.QualificationDirectTeamConsumptionMicros <= 0 ||
		s.QualificationCombinedConsumptionMicros <= 0 {
		return invalid("qualification thresholds must be positive")
	}
	if s.MaxCampaignLinks < 0 || s.MaxCampaignLinks > 100 {
		return invalid("max campaign links must be between 0 and 100")
	}
	if s.CommissionConversionMultiplierMillis < 1000 {
		return invalid("commission conversion multiplier cannot be below 1.0")
	}
	if s.WithdrawalMinMicros <= 0 {
		return invalid("withdrawal minimum must be positive")
	}
	if s.WithdrawalSLAHours <= 0 || s.WithdrawalSLAHours > 24*7 {
		return invalid("withdrawal SLA must be between 1 and 168 hours")
	}
	if s.MarginFloorBPS < 3500 || s.MarginFloorBPS > 10000 {
		return invalid("margin floor must be between 3500 and 10000 bps")
	}
	if err := ValidateAffiliateCommercialMarginFloor(s.MarginFloorBPS); err != nil {
		return err
	}
	if s.Revision <= 0 {
		return invalid("revision must be positive")
	}
	return nil
}

func ValidateAffiliateCustomerRebateRate(rateBPS int32) error {
	if rateBPS < 0 || rateBPS > AffiliateAgentPoolRateBPS {
		return fmt.Errorf("%w: customer rebate rate must be between 0 and %d bps", ErrAffiliateProgramSettingsInvalid, AffiliateAgentPoolRateBPS)
	}
	if rateBPS%AffiliateRateStepBPS != 0 {
		return fmt.Errorf("%w: customer rebate rate must use %d bps steps", ErrAffiliateProgramSettingsInvalid, AffiliateRateStepBPS)
	}
	return nil
}

func AffiliateAgentCommissionRate(rateBPS int32) (int32, error) {
	if err := ValidateAffiliateCustomerRebateRate(rateBPS); err != nil {
		return 0, err
	}
	return AffiliateAgentPoolRateBPS - rateBPS, nil
}

func isAffiliateProgramMode(mode string) bool {
	switch mode {
	case AffiliateProgramModeOff, AffiliateProgramModeShadow, AffiliateProgramModeLive:
		return true
	default:
		return false
	}
}

func validateAffiliateModeTransition(current, next string) error {
	if !isAffiliateProgramMode(current) || !isAffiliateProgramMode(next) {
		return fmt.Errorf("%w: %q -> %q", ErrAffiliateProgramModeTransition, current, next)
	}
	if current == next {
		return nil
	}

	switch current {
	case AffiliateProgramModeOff:
		if next == AffiliateProgramModeShadow {
			return nil
		}
	case AffiliateProgramModeShadow:
		if next == AffiliateProgramModeOff || next == AffiliateProgramModeLive {
			return nil
		}
	case AffiliateProgramModeLive:
		if next == AffiliateProgramModeOff || next == AffiliateProgramModeShadow {
			return nil
		}
	}
	return fmt.Errorf("%w: %q -> %q", ErrAffiliateProgramModeTransition, current, next)
}

// AffiliateProgramDecision separates observation from monetary writes. Shadow
// mode may emit diagnostics, but can never create rewards or commission rows.
type AffiliateProgramDecision struct {
	Mode                string
	Observe             bool
	AllowMonetaryWrites bool
}

func DecideAffiliateProgram(settings AffiliateProgramSettings, eventAt time.Time) AffiliateProgramDecision {
	decision := AffiliateProgramDecision{Mode: settings.Mode}
	switch settings.Mode {
	case AffiliateProgramModeShadow:
		decision.Observe = true
	case AffiliateProgramModeLive:
		decision.Observe = true
		decision.AllowMonetaryWrites = settings.StartedAt != nil && !eventAt.Before(*settings.StartedAt)
	}
	return decision
}

type AffiliateProgramRepository interface {
	GetSettings(ctx context.Context) (*AffiliateProgramSettings, error)
	UpdateSettings(ctx context.Context, settings *AffiliateProgramSettings, expectedRevision int64) error
}

type AffiliateProgramService struct {
	repo AffiliateProgramRepository
}

func NewAffiliateProgramService(repo AffiliateProgramRepository) *AffiliateProgramService {
	return &AffiliateProgramService{repo: repo}
}

func (s *AffiliateProgramService) GetSettings(ctx context.Context) (*AffiliateProgramSettings, error) {
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	return settings, nil
}

// UpdateSettings uses optimistic concurrency and keeps started_at immutable once
// the program has ever entered live mode. Direct off -> live activation is
// intentionally forbidden; staging must validate shadow mode first.
func (s *AffiliateProgramService) UpdateSettings(
	ctx context.Context,
	next AffiliateProgramSettings,
	expectedRevision int64,
	actorID int64,
) (*AffiliateProgramSettings, error) {
	current, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if current.Revision != expectedRevision {
		return nil, ErrAffiliateProgramRevisionConflict
	}
	if err := validateAffiliateModeTransition(current.Mode, next.Mode); err != nil {
		return nil, err
	}
	if current.StartedAt != nil {
		if next.StartedAt == nil || !next.StartedAt.Equal(*current.StartedAt) {
			return nil, fmt.Errorf("%w: started_at is immutable after activation", ErrAffiliateProgramSettingsInvalid)
		}
	} else if next.Mode != AffiliateProgramModeLive && next.StartedAt != nil {
		return nil, fmt.Errorf("%w: started_at is assigned only when entering live mode", ErrAffiliateProgramSettingsInvalid)
	}

	next.ID = 1
	next.ProgramVersion = AffiliateProgramVersionV2
	next.Revision = expectedRevision
	next.CreatedAt = current.CreatedAt
	next.UpdatedBy = &actorID
	if err := next.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateSettings(ctx, &next, expectedRevision); err != nil {
		return nil, err
	}
	return &next, nil
}
