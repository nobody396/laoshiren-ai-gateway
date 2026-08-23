package service

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestLoadChannelMonitoringEvidenceUsesBoundedReadOnlyAggregate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SET LOCAL statement_timeout = '5s'")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT o.fact_type").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{
		"fact_type", "group_id", "group_name", "account_id", "account_name", "route_fingerprint", "platform", "model", "request_class", "protocol",
		"sample_count", "success_count", "failure_count", "recovered_count", "customer_impact_count", "average_latency_ms", "p95_latency_ms", "last_observed_at", "last_failure_at", "last_success_at", "last_recovery_at",
		"total_buckets", "total_samples", "total_failures", "total_customer_requests", "total_customer_successes",
	}).AddRow("active_probe", nil, "", 53, "Pomo JP", "0123456789abcdef0123456789abcdef", "openai", "gpt-5.6", "text", "http", 10, 8, 2, 0, 0, 320.0, 600.0, now, now.Add(-time.Minute), now, nil, 1, 10, 2, 0, 0))
	mock.ExpectCommit()
	svc := &StatusControlService{db: db}
	items, totals, bucketTotal, err := svc.loadChannelMonitoringEvidence(context.Background(), now.Add(-15*time.Minute), now)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(2), items[0].FailureCount)
	require.Equal(t, 0.8, *items[0].Availability)
	require.InDelta(t, 10.0/15.0, items[0].SamplesPerMinute, 0.0001)
	require.Nil(t, items[0].LastRecoveryAt, "ordinary success must not be mislabeled as recovery")
	require.Equal(t, int64(10), totals.SampleCount)
	require.Equal(t, int64(1), bucketTotal)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProjectChannelMonitoringEvidenceUsesGroupPrecedenceAndProbeFanout(t *testing.T) {
	groupID := int64(40)
	accountID := int64(53)
	products := []statusProductDefinition{
		{ID: 1, Code: "openai-api", Components: []statusComponentDefinition{{ID: 11, Code: "openai-http", AccessMode: "http", Bindings: []statusBinding{{Platform: PlatformOpenAI}}}}},
		{ID: 2, Code: "builder-pass-gpt", Components: []statusComponentDefinition{{ID: 22, Code: "builder-gpt-http", AccessMode: "http", Bindings: []statusBinding{{GroupID: &groupID, AccountIDs: map[int64]struct{}{accountID: {}}}}}}},
	}
	now := time.Now()
	evidence := []ChannelMonitoringEvidence{
		{FactType: ReliabilityFactCustomerRequest, GroupID: &groupID, AccountID: &accountID, Platform: PlatformOpenAI, Model: "gpt-5.6", Protocol: "responses", LastObservedAt: now},
		{FactType: ReliabilityFactActiveProbe, AccountID: &accountID, Platform: PlatformOpenAI, Model: "gpt-5.6", Protocol: "http", LastObservedAt: now},
		{FactType: ReliabilityFactUpstreamAttempt, GroupID: &groupID, AccountID: &accountID, Platform: PlatformOpenAI, Model: "gpt-5.6", Protocol: "responses", LastObservedAt: now},
	}
	projectChannelMonitoringEvidence(products, evidence)
	require.Equal(t, []string{"builder-pass-gpt"}, evidence[0].ProductCodes)
	require.Equal(t, []string{"builder-pass-gpt", "openai-api"}, evidence[1].ProductCodes)
	require.Equal(t, []string{"builder-pass-gpt"}, evidence[2].ProductCodes)
}

func TestChannelMonitoringDTOContainsNoSecretsOrRawBaseURL(t *testing.T) {
	payload, err := json.Marshal(ChannelMonitoringEvidence{
		FactType: ReliabilityFactActiveProbe, AccountID: int64PointerForMonitoring(53), AccountName: "Pomo JP",
		RouteFingerprint: "0123456789abcdef0123456789abcdef", Platform: "openai", ProductCodes: []string{"openai-api"}, ComponentCodes: []string{"openai-http"},
	})
	require.NoError(t, err)
	for _, forbidden := range []string{"credentials", "api_key", "base_url", "prompt", "response_body"} {
		require.NotContains(t, string(payload), forbidden)
	}
}

func TestChannelMonitoringSnapshotHonorsRequestDeadlineAndWindowBound(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	svc := &StatusControlService{db: db}
	_, err = svc.MonitoringSnapshot(context.Background(), 121*time.Minute)
	require.ErrorContains(t, err, "two hours")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = svc.MonitoringSnapshot(ctx, 15*time.Minute)
	require.Error(t, err)
}

func int64PointerForMonitoring(value int64) *int64 { return &value }
