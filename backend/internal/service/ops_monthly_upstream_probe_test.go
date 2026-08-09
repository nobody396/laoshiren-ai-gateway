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

type monthlyProbeAccountRepoStub struct {
	AccountRepository
	accountsByGroup map[int64][]Account
}

func (s *monthlyProbeAccountRepoStub) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	return s.accountsByGroup[groupID], nil
}

type monthlyProbeGroupRepoStub struct {
	groupRepoNoop
	groups map[int64]*Group
}

func (s *monthlyProbeGroupRepoStub) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	if group := s.groups[id]; group != nil {
		return group, nil
	}
	return nil, ErrGroupNotFound
}

func (s *monthlyProbeGroupRepoStub) ListActive(ctx context.Context) ([]Group, error) {
	groups := make([]Group, 0, len(s.groups))
	for _, group := range s.groups {
		if group != nil && group.Status == StatusActive {
			groups = append(groups, *group)
		}
	}
	return groups, nil
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

func TestMonthlyProbeGrokCredentialUsesAccountTypeContract(t *testing.T) {
	require.Equal(t, "native-key", monthlyProbeGrokCredential(&Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":      "native-key",
			"access_token": "wrong-field",
		},
	}))
	require.Equal(t, "oauth-token", monthlyProbeGrokCredential(&Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"api_key":      "wrong-field",
			"access_token": "oauth-token",
		},
	}))
}

func TestGrokMonthlyProbePayloadUsesNativeStreamingResponses(t *testing.T) {
	payload := createGrokMonthlyProbePayload("grok-4.5")

	require.Equal(t, "grok-4.5", payload["model"])
	require.Equal(t, true, payload["stream"])
	require.NotEmpty(t, payload["input"])
}

func TestMonthlyGatewayProbePointSurfacesUpstreamFailureAfterStreamingHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.WriteHeader(200)
	point := monthlyGatewayProbePoint(
		&Account{ID: 29, Name: "grok-native", Platform: PlatformGrok},
		"grok-4.5",
		recorder,
		time.Now(),
		&UpstreamFailoverError{
			StatusCode:   500,
			ResponseBody: []byte(`{"error":{"code":"upstream_overload","message":"provider overloaded"}}`),
		},
		nil,
	)

	require.Equal(t, "failed", point.Status)
	require.NotNil(t, point.HTTPStatus)
	require.Equal(t, 500, *point.HTTPStatus)
	require.Equal(t, "upstream_overload", point.ErrorCode)
	require.Equal(t, "provider overloaded", point.ErrorMessage)
}

func TestMonthlyGatewayProbePointDoesNotTreatSuccessfulSSEAsError(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "text/event-stream")
	recorder.WriteHeader(200)
	_, _ = recorder.WriteString("event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n")

	point := monthlyGatewayProbePoint(
		&Account{ID: 29, Name: "grok-native", Platform: PlatformGrok},
		"grok-4.5",
		recorder,
		time.Now(),
		nil,
		nil,
	)

	require.Equal(t, "ok", point.Status)
	require.Empty(t, point.ErrorCode)
	require.Empty(t, point.ErrorMessage)
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
	require.Equal(t, 1, account.TotalCount)
	require.InDelta(t, 1.0, account.Uptime, 0.0001)
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
	require.Equal(t, 3, account.TotalCount)
	require.Equal(t, 1, account.SuccessCount)
	require.InDelta(t, 0.5, account.Uptime, 0.0001)
	require.Equal(t, "ok", account.LatestStatus)
}

func TestMonthlyUpstreamProbeTargetsFollowMonthlyGroupBindings(t *testing.T) {
	ctx := context.Background()
	rate := 0.2
	svc := &OpsService{
		accountRepo: &monthlyProbeAccountRepoStub{
			accountsByGroup: map[int64][]Account{
				7: {
					{
						ID:             12,
						Name:           "pomoai-monthly-codex-0.2",
						Platform:       PlatformOpenAI,
						Status:         StatusActive,
						Schedulable:    true,
						RateMultiplier: &rate,
					},
					{
						ID:             15,
						Name:           "pomoai-codexplus-0.12",
						Platform:       PlatformOpenAI,
						Status:         StatusActive,
						Schedulable:    true,
						RateMultiplier: &rate,
					},
				},
				11: {
					{
						ID:             20,
						Name:           "claude-monthly",
						Platform:       PlatformAnthropic,
						Status:         StatusActive,
						Schedulable:    true,
						RateMultiplier: &rate,
					},
				},
				35: {
					{
						ID:             26,
						Name:           "pomoai-grok",
						Platform:       PlatformGrok,
						Status:         StatusActive,
						Schedulable:    true,
						RateMultiplier: &rate,
					},
				},
			},
		},
		groupRepo: &monthlyProbeGroupRepoStub{
			groups: map[int64]*Group{
				7: {
					ID:               7,
					Name:             "GPT Plus 月卡组",
					Platform:         PlatformOpenAI,
					Status:           StatusActive,
					SubscriptionType: SubscriptionTypeCredit,
				},
				11: {
					ID:               11,
					Name:             "Claude Plus 月卡组",
					Platform:         PlatformAnthropic,
					Status:           StatusActive,
					SubscriptionType: SubscriptionTypeCredit,
				},
				35: {
					ID:               35,
					Name:             "Grok Lite 月卡组",
					Platform:         PlatformGrok,
					Status:           StatusActive,
					SubscriptionType: SubscriptionTypeCredit,
				},
			},
		},
	}

	targets, err := svc.loadMonthlyUpstreamProbeTargets(ctx)

	require.NoError(t, err)
	require.Len(t, targets, 3)
	require.Equal(t, "monthly-codex-gateway", targets[0].AccountName)
	require.Equal(t, PlatformOpenAI, targets[0].Platform)
	require.Equal(t, "gpt-5.4-mini", targets[0].Model)
	require.Equal(t, int64(7), targets[0].GroupID)
	require.NotNil(t, targets[0].Account)
	require.InDelta(t, 0.2, targets[0].Account.BillingRateMultiplier(), 0.0001)
	require.Equal(t, "monthly-claude-gateway", targets[1].AccountName)
	require.Equal(t, PlatformAnthropic, targets[1].Platform)
	require.Equal(t, "claude-haiku-4-5", targets[1].Model)
	require.Equal(t, int64(11), targets[1].GroupID)
	require.Equal(t, "monthly-grok-gateway", targets[2].AccountName)
	require.Equal(t, PlatformGrok, targets[2].Platform)
	require.Equal(t, "grok-4.5", targets[2].Model)
	require.Equal(t, int64(35), targets[2].GroupID)

	plans := svc.loadMonthlyCardPublicPlans(ctx)
	require.Len(t, plans, 3)
	require.Nil(t, plans[0].GrokGroup)

	svc.opsRepo = &opsRepoMock{
		ListMonthlyUpstreamProbeResultsFn: func(ctx context.Context, since time.Time) ([]MonthlyUpstreamProbePoint, error) {
			return []MonthlyUpstreamProbePoint{}, nil
		},
	}
	svc.settingRepo = &monthlyStatusSettingRepoStub{
		values: map[string]string{
			SettingKeyMonthlyUpstreamProbeEnabled:    "true",
			SettingKeyMonthlyCardPublicStatusEnabled: "true",
		},
	}
	publicSnapshot, err := svc.GetMonthlyCardPublicStatusSnapshot(ctx, 60)
	require.NoError(t, err)
	require.Len(t, publicSnapshot.Accounts, 3)
	require.Equal(t, "Codex", publicSnapshot.Accounts[0].Channel)
	require.Equal(t, "Claude", publicSnapshot.Accounts[1].Channel)
	require.Equal(t, "Grok", publicSnapshot.Accounts[2].Channel)
}

func TestMonthlyCardPublicStatusLabelsNativeGrokProtocol(t *testing.T) {
	require.Equal(t, "Grok", monthlyCardPublicChannelName("monthly-grok-gateway", "grok-4.5", PlatformGrok))
	require.Equal(t, "Grok 月卡", monthlyCardPublicDisplayName("monthly-grok-gateway", "grok-4.5", PlatformGrok))
	require.Equal(t, "Claude", monthlyCardPublicChannelName("monthly-claude-gateway", "claude-haiku-4-5", PlatformAnthropic))
}

func TestMonthlyUpstreamProbeSnapshotFiltersObsoleteRenamedAccountPoints(t *testing.T) {
	ctx := context.Background()
	reference := time.Now().Truncate(time.Minute)
	svc := &OpsService{
		opsRepo: &opsRepoMock{
			ListMonthlyUpstreamProbeResultsFn: func(ctx context.Context, since time.Time) ([]MonthlyUpstreamProbePoint, error) {
				return []MonthlyUpstreamProbePoint{
					{
						AccountName:  "pomoai-monthly-codex-0.12",
						Platform:     PlatformOpenAI,
						Model:        "gpt-5.4-mini",
						ProbePath:    MonthlyUpstreamProbePathGateway,
						Status:       "failed",
						ErrorCode:    "missing_account",
						ErrorMessage: "monthly upstream account was not found",
						CheckedAt:    reference.Add(-2 * time.Minute),
					},
					{
						AccountID:   12,
						AccountName: "monthly-codex-gateway",
						Platform:    PlatformOpenAI,
						Model:       "gpt-5.4-mini",
						ProbePath:   MonthlyUpstreamProbePathGateway,
						Status:      "ok",
						CheckedAt:   reference.Add(-time.Minute),
					},
				}, nil
			},
		},
		settingRepo: &monthlyStatusSettingRepoStub{
			values: map[string]string{
				SettingKeyMonthlyUpstreamProbeEnabled: "true",
			},
		},
		accountRepo: &monthlyProbeAccountRepoStub{
			accountsByGroup: map[int64][]Account{
				7: {
					{
						ID:          12,
						Name:        "pomoai-monthly-codex-0.2",
						Platform:    PlatformOpenAI,
						Status:      StatusActive,
						Schedulable: true,
					},
				},
			},
		},
		groupRepo: &monthlyProbeGroupRepoStub{
			groups: map[int64]*Group{
				7: {
					ID:               7,
					Name:             "GPT Plus 月卡组",
					Platform:         PlatformOpenAI,
					Status:           StatusActive,
					SubscriptionType: SubscriptionTypeCredit,
				},
			},
		},
	}

	snapshot, err := svc.GetMonthlyUpstreamProbeSnapshot(ctx, 60)

	require.NoError(t, err)
	require.Len(t, snapshot.Accounts, 1)
	require.Equal(t, "monthly-codex-gateway", snapshot.Accounts[0].AccountName)
	require.Equal(t, "ok", snapshot.Accounts[0].LatestStatus)
	require.Len(t, snapshot.Accounts[0].Points, 1)
	require.Equal(t, "monthly-codex-gateway", snapshot.Accounts[0].Points[0].AccountName)
	require.Equal(t, int64(12), snapshot.Accounts[0].Points[0].AccountID)
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

func TestMonthlyUpstreamProbeTimeoutForModelWidensOnlyGrok(t *testing.T) {
	require.Equal(t, 45*time.Second, monthlyUpstreamProbeTimeoutForModel("grok-4.5"))
	require.Equal(t, 45*time.Second, monthlyUpstreamProbeTimeoutForModel(" GROK-4.5 "))
	require.Equal(t, 25*time.Second, monthlyUpstreamProbeTimeoutForModel("claude-haiku-4-5"))
	require.Equal(t, 25*time.Second, monthlyUpstreamProbeTimeoutForModel("gpt-5.4-mini"))
}

func monthlyStatusBoolPtr(value bool) *bool {
	return &value
}

func monthlyStatusIntPtr(value int) *int {
	return &value
}
