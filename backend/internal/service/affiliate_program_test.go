package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type affiliateProgramRepoStub struct {
	settings AffiliateProgramSettings
	updateFn func(*AffiliateProgramSettings, int64) error
}

func (r *affiliateProgramRepoStub) GetSettings(context.Context) (*AffiliateProgramSettings, error) {
	copy := r.settings
	return &copy, nil
}

func (r *affiliateProgramRepoStub) UpdateSettings(_ context.Context, settings *AffiliateProgramSettings, expected int64) error {
	if r.updateFn != nil {
		if err := r.updateFn(settings, expected); err != nil {
			return err
		}
	}
	settings.Revision = expected + 1
	r.settings = *settings
	return nil
}

func TestDefaultAffiliateProgramSettingsValidate(t *testing.T) {
	settings := DefaultAffiliateProgramSettings()
	require.NoError(t, settings.Validate())
	require.Equal(t, int32(1000), settings.AgentPoolRateBPS)
	require.Equal(t, int32(3500), settings.MarginFloorBPS)
	require.Equal(t, int64(100_000_000), settings.WithdrawalMinMicros)
	require.Equal(t, int32(24), settings.WithdrawalSLAHours)
}

func TestAffiliateProgramSettingsRejectUnsafeValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*AffiliateProgramSettings)
	}{
		{name: "agent pool is not fixed", mutate: func(s *AffiliateProgramSettings) { s.AgentPoolRateBPS = 900 }},
		{name: "ordinary inviter rate is not fixed", mutate: func(s *AffiliateProgramSettings) { s.OrdinaryReferralRateBPS = 600 }},
		{name: "ordinary invitee rate is not fixed", mutate: func(s *AffiliateProgramSettings) { s.OrdinaryInviteeRateBPS = 400 }},
		{name: "margin below 35 percent", mutate: func(s *AffiliateProgramSettings) { s.MarginFloorBPS = 3499 }},
		{name: "conversion multiplier below one", mutate: func(s *AffiliateProgramSettings) { s.CommissionConversionMultiplierMillis = 999 }},
		{name: "conversion multiplier above approved value", mutate: func(s *AffiliateProgramSettings) { s.CommissionConversionMultiplierMillis = 1300 }},
		{name: "reserve below approved minimum", mutate: func(s *AffiliateProgramSettings) { s.OperationalReserveBPS = 199 }},
		{name: "withdrawal sla too long", mutate: func(s *AffiliateProgramSettings) { s.WithdrawalSLAHours = 169 }},
		{name: "live without start", mutate: func(s *AffiliateProgramSettings) { s.Mode = AffiliateProgramModeLive }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := DefaultAffiliateProgramSettings()
			tt.mutate(&settings)
			err := settings.Validate()
			require.Error(t, err)
			require.ErrorIs(t, err, ErrAffiliateProgramSettingsInvalid)
		})
	}
}

func TestAffiliateLinkRateSplit(t *testing.T) {
	for rebate := int32(0); rebate <= 1000; rebate += 100 {
		commission, err := AffiliateAgentCommissionRate(rebate)
		require.NoError(t, err)
		require.Equal(t, int32(1000), rebate+commission)
	}

	_, err := AffiliateAgentCommissionRate(250)
	require.Error(t, err)
	_, err = AffiliateAgentCommissionRate(1100)
	require.Error(t, err)
}

func TestDecideAffiliateProgram(t *testing.T) {
	start := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)

	off := DefaultAffiliateProgramSettings()
	require.Equal(t, AffiliateProgramDecision{Mode: AffiliateProgramModeOff}, DecideAffiliateProgram(off, start))

	shadow := DefaultAffiliateProgramSettings()
	shadow.Mode = AffiliateProgramModeShadow
	require.Equal(t, AffiliateProgramDecision{
		Mode:    AffiliateProgramModeShadow,
		Observe: true,
	}, DecideAffiliateProgram(shadow, start))

	live := DefaultAffiliateProgramSettings()
	live.Mode = AffiliateProgramModeLive
	live.StartedAt = &start
	require.False(t, DecideAffiliateProgram(live, start.Add(-time.Nanosecond)).AllowMonetaryWrites)
	require.True(t, DecideAffiliateProgram(live, start).AllowMonetaryWrites)
	require.True(t, DecideAffiliateProgram(live, start.Add(time.Hour)).AllowMonetaryWrites)
}

func TestAffiliateProgramUpdateRequiresShadowBeforeLive(t *testing.T) {
	repo := &affiliateProgramRepoStub{settings: DefaultAffiliateProgramSettings()}
	svc := NewAffiliateProgramService(repo)
	start := time.Now().UTC().Add(time.Hour)
	next := repo.settings
	next.Mode = AffiliateProgramModeLive
	next.StartedAt = &start

	_, err := svc.UpdateSettings(context.Background(), next, 1, 99)
	require.ErrorIs(t, err, ErrAffiliateProgramModeTransition)
}

func TestAffiliateProgramUpdateShadowThenLiveAndFreezesStart(t *testing.T) {
	repo := &affiliateProgramRepoStub{settings: DefaultAffiliateProgramSettings()}
	svc := NewAffiliateProgramService(repo)

	shadow := repo.settings
	shadow.Mode = AffiliateProgramModeShadow
	updated, err := svc.UpdateSettings(context.Background(), shadow, 1, 99)
	require.NoError(t, err)
	require.Equal(t, int64(2), updated.Revision)
	require.Equal(t, AffiliateProgramModeShadow, updated.Mode)

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	live := *updated
	live.Mode = AffiliateProgramModeLive
	live.StartedAt = &start
	updated, err = svc.UpdateSettings(context.Background(), live, 2, 99)
	require.NoError(t, err)
	require.Equal(t, int64(3), updated.Revision)
	require.Equal(t, start, *updated.StartedAt)

	changed := *updated
	other := start.Add(time.Hour)
	changed.StartedAt = &other
	_, err = svc.UpdateSettings(context.Background(), changed, 3, 99)
	require.ErrorIs(t, err, ErrAffiliateProgramSettingsInvalid)
}

func TestAffiliateProgramUpdateRejectsStaleRevision(t *testing.T) {
	settings := DefaultAffiliateProgramSettings()
	settings.Revision = 4
	repo := &affiliateProgramRepoStub{settings: settings}
	svc := NewAffiliateProgramService(repo)

	_, err := svc.UpdateSettings(context.Background(), settings, 3, 99)
	require.True(t, errors.Is(err, ErrAffiliateProgramRevisionConflict))
}
