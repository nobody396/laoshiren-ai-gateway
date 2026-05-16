//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var _ OpsRepository = (*stubOpsRepo)(nil)

type stubOpsRepo struct {
	OpsRepository
	overview     *OpsDashboardOverview
	err          error
	rules        []*OpsAlertRule
	activeEvents map[int64]*OpsAlertEvent

	updatedEventID int64
	updatedStatus  string
	resolvedAt     *time.Time
	heartbeat      *OpsUpsertJobHeartbeatInput
}

func (s *stubOpsRepo) GetDashboardOverview(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.overview != nil {
		return s.overview, nil
	}
	return &OpsDashboardOverview{}, nil
}

func (s *stubOpsRepo) ListAlertRules(ctx context.Context) ([]*OpsAlertRule, error) {
	return s.rules, nil
}

func (s *stubOpsRepo) GetLatestSystemMetrics(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
	return nil, nil
}

func (s *stubOpsRepo) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error) {
	if s.activeEvents == nil {
		return nil, nil
	}
	ev := s.activeEvents[ruleID]
	if ev == nil || ev.Status != OpsAlertStatusFiring {
		return nil, nil
	}
	return ev, nil
}

func (s *stubOpsRepo) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	s.updatedEventID = eventID
	s.updatedStatus = status
	s.resolvedAt = resolvedAt
	for _, ev := range s.activeEvents {
		if ev != nil && ev.ID == eventID {
			ev.Status = status
			ev.ResolvedAt = resolvedAt
		}
	}
	return nil
}

func (s *stubOpsRepo) UpsertJobHeartbeat(ctx context.Context, input *OpsUpsertJobHeartbeatInput) error {
	s.heartbeat = input
	return nil
}

func TestComputeGroupAvailableRatio(t *testing.T) {
	t.Parallel()

	t.Run("正常情况: 10个账号, 8个可用 = 80%", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  10,
			AvailableCount: 8,
		})
		require.InDelta(t, 80.0, got, 0.0001)
	})

	t.Run("边界情况: TotalAccounts = 0 应返回 0", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  0,
			AvailableCount: 8,
		})
		require.Equal(t, 0.0, got)
	})

	t.Run("边界情况: AvailableCount = 0 应返回 0%", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  10,
			AvailableCount: 0,
		})
		require.Equal(t, 0.0, got)
	})
}

func TestCountAccountsByCondition(t *testing.T) {
	t.Parallel()

	t.Run("测试限流账号统计: acc.IsRateLimited", func(t *testing.T) {
		t.Parallel()

		accounts := map[int64]*AccountAvailability{
			1: {IsRateLimited: true},
			2: {IsRateLimited: false},
			3: {IsRateLimited: true},
		}

		got := countAccountsByCondition(accounts, func(acc *AccountAvailability) bool {
			return acc.IsRateLimited
		})
		require.Equal(t, int64(2), got)
	})

	t.Run("测试错误账号统计（排除临时不可调度）: acc.HasError && acc.TempUnschedulableUntil == nil", func(t *testing.T) {
		t.Parallel()

		until := time.Now().UTC().Add(5 * time.Minute)
		accounts := map[int64]*AccountAvailability{
			1: {HasError: true},
			2: {HasError: true, TempUnschedulableUntil: &until},
			3: {HasError: false},
		}

		got := countAccountsByCondition(accounts, func(acc *AccountAvailability) bool {
			return acc.HasError && acc.TempUnschedulableUntil == nil
		})
		require.Equal(t, int64(1), got)
	})

	t.Run("边界情况: 空 map 应返回 0", func(t *testing.T) {
		t.Parallel()

		got := countAccountsByCondition(map[int64]*AccountAvailability{}, func(acc *AccountAvailability) bool {
			return acc.IsRateLimited
		})
		require.Equal(t, int64(0), got)
	})
}

func TestComputeRuleMetricNewIndicators(t *testing.T) {
	t.Parallel()

	groupID := int64(101)
	platform := "openai"

	availability := &OpsAccountAvailability{
		Group: &GroupAvailability{
			GroupID:        groupID,
			TotalAccounts:  10,
			AvailableCount: 8,
		},
		Accounts: map[int64]*AccountAvailability{
			1: {IsRateLimited: true},
			2: {IsRateLimited: true},
			3: {HasError: true},
			4: {HasError: true, TempUnschedulableUntil: timePtr(time.Now().UTC().Add(2 * time.Minute))},
			5: {HasError: false, IsRateLimited: false},
		},
	}

	opsService := &OpsService{
		getAccountAvailability: func(_ context.Context, _ string, _ *int64) (*OpsAccountAvailability, error) {
			return availability, nil
		},
	}

	svc := &OpsAlertEvaluatorService{
		opsService: opsService,
		opsRepo:    &stubOpsRepo{overview: &OpsDashboardOverview{}},
	}

	start := time.Now().UTC().Add(-5 * time.Minute)
	end := time.Now().UTC()
	ctx := context.Background()

	tests := []struct {
		name       string
		metricType string
		groupID    *int64
		wantValue  float64
		wantOK     bool
	}{
		{
			name:       "group_available_accounts",
			metricType: "group_available_accounts",
			groupID:    &groupID,
			wantValue:  8,
			wantOK:     true,
		},
		{
			name:       "group_available_ratio",
			metricType: "group_available_ratio",
			groupID:    &groupID,
			wantValue:  80.0,
			wantOK:     true,
		},
		{
			name:       "account_rate_limited_count",
			metricType: "account_rate_limited_count",
			groupID:    nil,
			wantValue:  2,
			wantOK:     true,
		},
		{
			name:       "account_error_count",
			metricType: "account_error_count",
			groupID:    nil,
			wantValue:  1,
			wantOK:     true,
		},
		{
			name:       "group_available_accounts without group_id returns false",
			metricType: "group_available_accounts",
			groupID:    nil,
			wantValue:  0,
			wantOK:     false,
		},
		{
			name:       "group_available_ratio without group_id returns false",
			metricType: "group_available_ratio",
			groupID:    nil,
			wantValue:  0,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule := &OpsAlertRule{
				MetricType: tt.metricType,
			}
			gotValue, gotOK := svc.computeRuleMetric(ctx, rule, nil, start, end, platform, tt.groupID)
			require.Equal(t, tt.wantOK, gotOK)
			if !tt.wantOK {
				return
			}
			require.InDelta(t, tt.wantValue, gotValue, 0.0001)
		})
	}
}

func TestAlertEvaluatorResolvesTrafficRateAlertWhenWindowHasNoSamples(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubOpsRepo{
		overview: &OpsDashboardOverview{RequestCountSLA: 0},
		rules: []*OpsAlertRule{
			{
				ID:               1,
				Name:             "错误率过高",
				Enabled:          true,
				Severity:         "P1",
				MetricType:       "error_rate",
				Operator:         ">",
				Threshold:        5,
				WindowMinutes:    5,
				SustainedMinutes: 1,
			},
		},
		activeEvents: map[int64]*OpsAlertEvent{
			1: {
				ID:      25,
				RuleID:  1,
				Status:  OpsAlertStatusFiring,
				FiredAt: now.Add(-10 * time.Minute),
			},
		},
	}
	svc := &OpsAlertEvaluatorService{
		opsRepo:    repo,
		instanceID: "test-instance",
		ruleStates: map[int64]*opsAlertRuleState{},
	}

	svc.evaluateOnce(time.Minute)

	require.Equal(t, int64(25), repo.updatedEventID)
	require.Equal(t, OpsAlertStatusResolved, repo.updatedStatus)
	require.NotNil(t, repo.resolvedAt)
	require.Equal(t, OpsAlertStatusResolved, repo.activeEvents[1].Status)
	require.NotNil(t, repo.heartbeat)
	require.NotNil(t, repo.heartbeat.LastResult)
	require.Contains(t, *repo.heartbeat.LastResult, "resolved=1")
}

func TestAlertEvaluatorDoesNotResolveTrafficRateAlertWhenMetricReadFails(t *testing.T) {
	t.Parallel()

	repo := &stubOpsRepo{
		err: errors.New("dashboard query failed"),
		rules: []*OpsAlertRule{
			{
				ID:               1,
				Name:             "错误率过高",
				Enabled:          true,
				Severity:         "P1",
				MetricType:       "error_rate",
				Operator:         ">",
				Threshold:        5,
				WindowMinutes:    5,
				SustainedMinutes: 1,
			},
		},
		activeEvents: map[int64]*OpsAlertEvent{
			1: {
				ID:     25,
				RuleID: 1,
				Status: OpsAlertStatusFiring,
			},
		},
	}
	svc := &OpsAlertEvaluatorService{
		opsRepo:    repo,
		instanceID: "test-instance",
		ruleStates: map[int64]*opsAlertRuleState{},
	}

	svc.evaluateOnce(time.Minute)

	require.Zero(t, repo.updatedEventID)
	require.Empty(t, repo.updatedStatus)
	require.Equal(t, OpsAlertStatusFiring, repo.activeEvents[1].Status)
}
