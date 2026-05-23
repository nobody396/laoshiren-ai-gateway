package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

func defaultInviteActivityConfig() *InviteActivityConfig {
	return &InviteActivityConfig{
		Enabled:                 false,
		Name:                    "公测邀请活动",
		RegistrationBonusAmount: 5,
	}
}

func (s *CommissionService) GetInviteActivityConfig(ctx context.Context) (*InviteActivityConfig, error) {
	activity, ok := s.getInviteActivityConfig(ctx)
	if !ok || activity == nil {
		return defaultInviteActivityConfig(), nil
	}
	return activity, nil
}

func (s *CommissionService) UpdateInviteActivityConfig(ctx context.Context, activity *InviteActivityConfig) (*InviteActivityConfig, error) {
	if s.activityRepo == nil {
		return nil, fmt.Errorf("invite activity repository is not configured")
	}
	if activity == nil {
		return nil, infraerrors.BadRequest("INVALID_INVITE_ACTIVITY", "invite activity config is required")
	}
	normalized := normalizeInviteActivityConfig(activity)
	if normalized.RegistrationBonusAmount < 0 {
		return nil, infraerrors.BadRequest("INVALID_INVITE_ACTIVITY", "registration bonus cannot be negative")
	}
	emailWhitelist, err := NormalizeRegistrationEmailSuffixWhitelist(normalized.EmailSuffixWhitelist)
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_INVITE_ACTIVITY_EMAIL_SUFFIX_WHITELIST", err.Error())
	}
	normalized.EmailSuffixWhitelist = emailWhitelist
	if normalized.StartAt != nil && normalized.EndAt != nil && !normalized.EndAt.After(*normalized.StartAt) {
		return nil, infraerrors.BadRequest("INVALID_INVITE_ACTIVITY_TIME_RANGE", "activity end time must be after start time")
	}
	if err := s.activityRepo.UpdateInviteActivityConfig(ctx, normalized); err != nil {
		return nil, fmt.Errorf("update invite activity config: %w", err)
	}
	return s.GetInviteActivityConfig(ctx)
}

// ProcessInviteActivityRegistrationBonus grants the invite campaign registration
// balance bonus to newly invited users while the campaign window is active.
func (s *CommissionService) ProcessInviteActivityRegistrationBonus(ctx context.Context, userID int64) error {
	activity, ok := s.getInviteActivityConfig(ctx)
	if !ok || activity == nil || !activity.Enabled || activity.RegistrationBonusAmount <= 0 {
		return nil
	}
	now := s.now()
	if !inviteActivityContains(activity, now) {
		return nil
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user for invite activity registration bonus: %w", err)
	}
	if user.InviterID == nil && user.AgentID == nil {
		return nil
	}

	if err := s.userRepo.UpdateBalance(ctx, userID, activity.RegistrationBonusAmount); err != nil {
		return fmt.Errorf("apply invite activity registration bonus: %w", err)
	}
	if s.commissionRepo == nil {
		return nil
	}

	note := fmt.Sprintf("邀请活动注册赠送：%s", activity.Name)
	record := &CommissionRecord{
		BeneficiaryID: userID,
		UserID:        userID,
		Amount:        activity.RegistrationBonusAmount,
		SourceAmount:  0,
		Type:          CommissionTypeInviteActivityRegistrationBonus,
		RateSource:    "invite_activity",
		Note:          &note,
	}
	if err := s.commissionRepo.Create(ctx, record); err != nil {
		return fmt.Errorf("create invite activity registration bonus record: %w", err)
	}
	return nil
}

func (s *CommissionService) getInviteActivityConfig(ctx context.Context) (*InviteActivityConfig, bool) {
	if s.activityRepo == nil {
		return defaultInviteActivityConfig(), false
	}
	activity, err := s.activityRepo.GetInviteActivityConfig(ctx)
	if err != nil || activity == nil {
		if err != nil {
			slog.Warn("load invite activity config failed, using defaults", "error", err)
		}
		return defaultInviteActivityConfig(), false
	}
	normalized := normalizeInviteActivityConfig(activity)
	return s.autoCloseExpiredInviteActivity(ctx, normalized), true
}

func (s *CommissionService) autoCloseExpiredInviteActivity(ctx context.Context, activity *InviteActivityConfig) *InviteActivityConfig {
	if activity == nil || !activity.Enabled || activity.EndAt == nil || s.now().Before(*activity.EndAt) {
		return activity
	}

	closed := *activity
	closed.Enabled = false
	closed.EmailRestrictionEnabled = false
	if s.activityRepo == nil {
		return &closed
	}
	if err := s.activityRepo.UpdateInviteActivityConfig(ctx, &closed); err != nil {
		slog.Warn("auto close expired invite activity failed", "name", activity.Name, "end_at", activity.EndAt, "error", err)
		return &closed
	}
	slog.Info("auto closed expired invite activity", "name", activity.Name, "end_at", activity.EndAt)
	return normalizeInviteActivityConfig(&closed)
}

func normalizeInviteActivityConfig(activity *InviteActivityConfig) *InviteActivityConfig {
	if activity == nil {
		return defaultInviteActivityConfig()
	}
	normalized := *activity
	normalized.Name = strings.TrimSpace(normalized.Name)
	if normalized.Name == "" {
		normalized.Name = "公测邀请活动"
	}
	normalized.EmailSuffixWhitelist = normalizeInviteActivityEmailSuffixWhitelist(normalized.EmailSuffixWhitelist)
	return &normalized
}

func (s *CommissionService) ActiveInviteActivityEmailSuffixWhitelist(ctx context.Context, fallbackWhitelist []string) ([]string, bool) {
	activity, ok := s.getInviteActivityConfig(ctx)
	if !ok || activity == nil || !activity.Enabled || !activity.EmailRestrictionEnabled {
		return nil, false
	}
	if !inviteActivityContains(activity, s.now()) {
		return nil, false
	}

	whitelist := normalizeInviteActivityEmailSuffixWhitelist(activity.EmailSuffixWhitelist)
	if len(whitelist) == 0 {
		whitelist = normalizeInviteActivityEmailSuffixWhitelist(fallbackWhitelist)
	}
	if len(whitelist) == 0 {
		return nil, false
	}
	return whitelist, true
}

func normalizeInviteActivityEmailSuffixWhitelist(raw []string) []string {
	normalized, err := NormalizeRegistrationEmailSuffixWhitelist(raw)
	if err != nil {
		return []string{}
	}
	return normalized
}

func inviteActivityContains(activity *InviteActivityConfig, ts time.Time) bool {
	if activity == nil {
		return false
	}
	if activity.StartAt != nil && ts.Before(*activity.StartAt) {
		return false
	}
	if activity.EndAt != nil && !ts.Before(*activity.EndAt) {
		return false
	}
	return true
}
