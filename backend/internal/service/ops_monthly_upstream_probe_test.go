package service

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type monthlyStatusSettingRepoStub struct {
	values  map[string]string
	updates map[string]string
}

func (s *monthlyStatusSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *monthlyStatusSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *monthlyStatusSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	if s.updates == nil {
		s.updates = map[string]string{}
	}
	s.values[key] = value
	s.updates[key] = value
	return nil
}

func (s *monthlyStatusSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *monthlyStatusSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *monthlyStatusSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *monthlyStatusSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestMonthlyCardPublicStatusSnapshotHiddenByDefault(t *testing.T) {
	ctx := context.Background()
	called := 0
	svc := &OpsService{
		opsRepo: &opsRepoMock{
			ListMonthlyUpstreamProbeResultsFn: func(ctx context.Context, since time.Time) ([]MonthlyUpstreamProbePoint, error) {
				called++
				return []MonthlyUpstreamProbePoint{{
					AccountName: "pomoai-monthly-codex-0.12",
					Platform:    PlatformOpenAI,
					Model:       "gpt-5.4-mini",
					ProbePath:   MonthlyUpstreamProbePathGateway,
					Status:      "ok",
					CheckedAt:   time.Now(),
				}}, nil
			},
		},
		settingRepo: &monthlyStatusSettingRepoStub{
			values: map[string]string{
				SettingKeyMonthlyUpstreamProbeEnabled: "true",
			},
		},
	}

	snapshot, err := svc.GetMonthlyCardPublicStatusSnapshot(ctx, 60)

	require.NoError(t, err)
	require.False(t, snapshot.Enabled)
	require.False(t, snapshot.VisibleToUsers)
	require.Empty(t, snapshot.Accounts)
	require.Equal(t, 0, called)
}

func TestMonthlyCardPublicStatusSnapshotVisibleWhenEnabled(t *testing.T) {
	ctx := context.Background()
	checkedAt := time.Now().Add(-time.Minute)
	svc := &OpsService{
		opsRepo: &opsRepoMock{
			ListMonthlyUpstreamProbeResultsFn: func(ctx context.Context, since time.Time) ([]MonthlyUpstreamProbePoint, error) {
				return []MonthlyUpstreamProbePoint{{
					AccountID:   1,
					AccountName: "pomoai-monthly-codex-0.12",
					Platform:    PlatformOpenAI,
					Model:       "gpt-5.4-mini",
					ProbePath:   MonthlyUpstreamProbePathGateway,
					Status:      "ok",
					CheckedAt:   checkedAt,
				}}, nil
			},
		},
		settingRepo: &monthlyStatusSettingRepoStub{
			values: map[string]string{
				SettingKeyMonthlyUpstreamProbeEnabled:    "true",
				SettingKeyMonthlyCardPublicStatusEnabled: "true",
			},
		},
	}

	snapshot, err := svc.GetMonthlyCardPublicStatusSnapshot(ctx, 60)

	require.NoError(t, err)
	require.True(t, snapshot.Enabled)
	require.True(t, snapshot.VisibleToUsers)
	require.Len(t, snapshot.Accounts, 1)
	require.Equal(t, "Codex", snapshot.Accounts[0].Channel)
	require.Equal(t, "ok", string(snapshot.Accounts[0].Status))
}

func TestMonthlyUpstreamProbeSnapshotUsesGatewayPointsForStatus(t *testing.T) {
	ctx := context.Background()
	gatewayCheckedAt := time.Now().Add(-2 * time.Minute)
	directCheckedAt := time.Now().Add(-time.Minute)
	svc := &OpsService{
		opsRepo: &opsRepoMock{
			ListMonthlyUpstreamProbeResultsFn: func(ctx context.Context, since time.Time) ([]MonthlyUpstreamProbePoint, error) {
				return []MonthlyUpstreamProbePoint{
					{
						AccountID:   1,
						AccountName: "pomoai-monthly-codex-0.12",
						Platform:    PlatformOpenAI,
						Model:       "gpt-5.4-mini",
						ProbePath:   MonthlyUpstreamProbePathGateway,
						Status:      "ok",
						HTTPStatus:  monthlyStatusIntPtr(200),
						LatencyMs:   800,
						CheckedAt:   gatewayCheckedAt,
					},
					{
						AccountID:    1,
						AccountName:  "pomoai-monthly-codex-0.12",
						Platform:     PlatformOpenAI,
						Model:        "gpt-5.4-mini",
						ProbePath:    MonthlyUpstreamProbePathDirectUpstream,
						Status:       "failed",
						HTTPStatus:   monthlyStatusIntPtr(502),
						LatencyMs:    25000,
						ErrorCode:    "request_failed",
						ErrorMessage: "direct upstream timeout",
						CheckedAt:    directCheckedAt,
					},
				}, nil
			},
		},
		settingRepo: &monthlyStatusSettingRepoStub{
			values: map[string]string{
				SettingKeyMonthlyUpstreamProbeEnabled:    "true",
				SettingKeyMonthlyCardPublicStatusEnabled: "true",
			},
		},
	}

	snapshot, err := svc.GetMonthlyUpstreamProbeSnapshot(ctx, 60)

	require.NoError(t, err)
	require.Len(t, snapshot.Accounts, 1)
	account := snapshot.Accounts[0]
	require.Equal(t, "ok", account.LatestStatus)
	require.Equal(t, 1, account.SuccessCount)
	require.Equal(t, monthlyUpstreamProbeExpectedSlotCount(60), account.TotalCount)
	require.InDelta(t, 1.0/float64(monthlyUpstreamProbeExpectedSlotCount(60)), account.Uptime, 0.0001)
	require.Len(t, account.Points, 1)
	require.NotNil(t, account.DirectUpstream)
	require.Equal(t, "failed", account.DirectUpstream.Status)

	publicSnapshot, err := svc.GetMonthlyCardPublicStatusSnapshot(ctx, 60)
	require.NoError(t, err)
	require.Len(t, publicSnapshot.Accounts, 1)
	require.Equal(t, "ok", string(publicSnapshot.Accounts[0].Status))
	require.Len(t, publicSnapshot.Accounts[0].Points, 1)
}

func TestMonthlyUpstreamProbeSnapshotScoresExpectedSlots(t *testing.T) {
	ctx := context.Background()
	reference := time.Now().Truncate(time.Minute)
	svc := &OpsService{
		opsRepo: &opsRepoMock{
			ListMonthlyUpstreamProbeResultsFn: func(ctx context.Context, since time.Time) ([]MonthlyUpstreamProbePoint, error) {
				return []MonthlyUpstreamProbePoint{
					{
						AccountID:   1,
						AccountName: "pomoai-monthly-codex-0.12",
						Platform:    PlatformOpenAI,
						Model:       "gpt-5.4-mini",
						ProbePath:   MonthlyUpstreamProbePathGateway,
						Status:      "ok",
						CheckedAt:   reference.Add(-2 * time.Minute),
					},
					{
						AccountID:   1,
						AccountName: "pomoai-monthly-codex-0.12",
						Platform:    PlatformOpenAI,
						Model:       "gpt-5.4-mini",
						ProbePath:   MonthlyUpstreamProbePathGateway,
						Status:      "slow",
						CheckedAt:   reference.Add(-4 * time.Minute),
					},
					{
						AccountID:   1,
						AccountName: "pomoai-monthly-codex-0.12",
						Platform:    PlatformOpenAI,
						Model:       "gpt-5.4-mini",
						ProbePath:   MonthlyUpstreamProbePathGateway,
						Status:      "failed",
						CheckedAt:   reference.Add(-6 * time.Minute),
					},
				}, nil
			},
		},
		settingRepo: &monthlyStatusSettingRepoStub{
			values: map[string]string{
				SettingKeyMonthlyUpstreamProbeEnabled: "true",
			},
		},
	}

	snapshot, err := svc.GetMonthlyUpstreamProbeSnapshot(ctx, 60)

	require.NoError(t, err)
	require.Len(t, snapshot.Accounts, 1)
	account := snapshot.Accounts[0]
	expectedSlots := monthlyUpstreamProbeExpectedSlotCount(60)
	require.Equal(t, expectedSlots, account.TotalCount)
	require.Equal(t, 1, account.SuccessCount)
	require.InDelta(t, 1.5/float64(expectedSlots), account.Uptime, 0.0001)
	require.Equal(t, "ok", account.LatestStatus)
}

func TestUpdateMonthlyUpstreamProbeSettingsDoesNotOverwriteOmittedFields(t *testing.T) {
	ctx := context.Background()
	settings := &monthlyStatusSettingRepoStub{
		values: map[string]string{
			SettingKeyMonthlyUpstreamProbeEnabled:    "true",
			SettingKeyMonthlyCardPublicStatusEnabled: "true",
		},
	}
	svc := &OpsService{settingRepo: settings}

	updated, err := svc.UpdateMonthlyUpstreamProbeSettings(ctx, &MonthlyUpstreamProbeSettingsUpdate{
		Enabled: monthlyStatusBoolPtr(false),
	})

	require.NoError(t, err)
	require.False(t, updated.Enabled)
	require.True(t, updated.PublicStatusEnabled)
	require.Equal(t, "false", settings.values[SettingKeyMonthlyUpstreamProbeEnabled])
	require.Equal(t, "true", settings.values[SettingKeyMonthlyCardPublicStatusEnabled])
	require.NotContains(t, settings.updates, SettingKeyMonthlyCardPublicStatusEnabled)

	updated, err = svc.UpdateMonthlyUpstreamProbeSettings(ctx, &MonthlyUpstreamProbeSettingsUpdate{
		PublicStatusEnabled: monthlyStatusBoolPtr(false),
	})

	require.NoError(t, err)
	require.False(t, updated.Enabled)
	require.False(t, updated.PublicStatusEnabled)
	require.Equal(t, "false", settings.values[SettingKeyMonthlyUpstreamProbeEnabled])
	require.Equal(t, "false", settings.values[SettingKeyMonthlyCardPublicStatusEnabled])
}

func TestMonthlyOpenAIProbeCostEstimateMatchesObservedBilling(t *testing.T) {
	estimate := buildMonthlyUpstreamProbeCostEstimate(
		"pomoai-monthly-codex-0.12",
		PlatformOpenAI,
		"gpt-5.4-mini",
		nil,
	)

	require.NotNil(t, estimate)
	require.InDelta(t, 8e-7, estimate.InputCostPerToken, 1e-12)
	require.InDelta(t, 3.2e-6, estimate.OutputCostPerToken, 1e-12)

	observedCost := (21*estimate.InputCostPerToken + 5*estimate.OutputCostPerToken) * estimate.RateMultiplier
	require.InDelta(t, 0.00000400, observedCost, 0.0000001)
}

func TestMonthlyGatewayProbePointDoesNotStoreSuccessBodyAsError(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.WriteHeader(200)
	_, _ = recorder.WriteString(`{"id":"resp_probe","output_text":"OK"}`)

	point := monthlyGatewayProbePoint(&Account{
		ID:       1,
		Name:     "pomoai-monthly-codex-0.12",
		Platform: PlatformOpenAI,
	}, "gpt-5.4-mini", recorder, time.Now(), nil, nil)

	require.Equal(t, "ok", point.Status)
	require.Empty(t, point.ErrorCode)
	require.Empty(t, point.ErrorMessage)
}

func TestMonthlyGatewayProbePointClassifiesGenericGatewayFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.WriteHeader(502)
	_, _ = recorder.WriteString(`{"error":{"type":"api_error","message":"The service is temporarily unavailable. Please try again later."}}`)

	point := monthlyGatewayProbePoint(&Account{
		ID:       1,
		Name:     "pomoai-monthly-codex-0.12",
		Platform: PlatformOpenAI,
	}, "gpt-5.4-mini", recorder, time.Now(), context.DeadlineExceeded, nil)

	require.Equal(t, "failed", point.Status)
	require.Equal(t, "gateway_forward_failed", point.ErrorCode)
	require.Contains(t, point.ErrorMessage, "deadline exceeded")
}

func monthlyStatusBoolPtr(value bool) *bool {
	return &value
}

func monthlyStatusIntPtr(value int) *int {
	return &value
}
