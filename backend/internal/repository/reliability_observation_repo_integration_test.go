//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestReliabilityEvidenceServiceIsIdempotentAndSanitized(t *testing.T) {
	ctx := context.Background()
	evidence := integrationReliabilityEvidence(t)
	identity := "integration:reliability:customer:req-1"
	key := "customer:" + identity
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_observations WHERE idempotency_key = $1`, key)
	})
	status := 503
	userID, groupID, accountID := int64(900001), int64(900002), int64(900003)
	observation := &service.ReliabilityFinalOutcome{
		RequestIdentity: identity, RequestID: "req-1",
		UserID: &userID, GroupID: &groupID, AccountID: &accountID,
		Platform: service.PlatformOpenAI, Model: "gpt-5.6-sol",
		RequestClass: service.ReliabilityRequestClassText, Protocol: "responses",
		Outcome: service.ReliabilityOutcomeFailure, StatusCode: &status,
		ErrorOwner: "provider", LatencyMs: 1250,
		ObservedAt: time.Date(2026, 8, 22, 4, 0, 0, 0, time.UTC),
	}

	require.True(t, evidence.SubmitFinalOutcome(observation))
	require.Eventually(t, func() bool { return evidence.Completeness().Written == 1 }, 2*time.Second, 10*time.Millisecond)
	require.True(t, evidence.SubmitFinalOutcome(observation))
	require.Eventually(t, func() bool { return evidence.Completeness().Processed == 2 }, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, int64(1), evidence.Completeness().Written)

	var factType, source, platform, model, protocol, outcome, owner string
	var impact bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT fact_type, source, platform, model, protocol, outcome, error_owner, customer_impact
FROM reliability_observations WHERE idempotency_key = $1
`, key).Scan(&factType, &source, &platform, &model, &protocol, &outcome, &owner, &impact))
	require.Equal(t, "customer_request", factType)
	require.Equal(t, "gateway_final", source)
	require.Equal(t, "openai", platform)
	require.Equal(t, "gpt-5.6-sol", model)
	require.Equal(t, "responses", protocol)
	require.Equal(t, "failure", outcome)
	require.Equal(t, "provider", owner)
	require.True(t, impact)

	snapshot, err := evidence.Snapshot(ctx, &service.ReliabilityEvidenceQuery{
		Start: observation.ObservedAt.Add(-time.Minute), End: observation.ObservedAt.Add(time.Minute),
		FactTypes: []service.ReliabilityFactType{service.ReliabilityFactCustomerRequest},
		GroupID:   &groupID, CustomerImpact: boolPointer(true), Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, snapshot.Observations, 1)
	require.Equal(t, key, snapshot.Observations[0].IdempotencyKey)
}

func TestReliabilityObservationServiceFailsClosedBeforeInvalidWrite(t *testing.T) {
	ctx := context.Background()
	evidence := integrationReliabilityEvidence(t)
	prefix := "integration:reliability:rollback:"
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_observations WHERE idempotency_key LIKE $1`, prefix+"%")
	})
	invalidStatus := 700
	base := service.ReliabilityFinalOutcome{
		Platform: service.PlatformOpenAI, Model: "gpt-5.6-sol",
		RequestClass: service.ReliabilityRequestClassText, Protocol: "responses",
		Outcome: service.ReliabilityOutcomeSuccess, ObservedAt: time.Now(),
	}
	invalid := base
	invalid.RequestIdentity, invalid.StatusCode = prefix+"invalid", &invalidStatus

	require.True(t, evidence.SubmitFinalOutcome(&invalid))
	require.Eventually(t, func() bool { return evidence.Completeness().Failed == 1 }, 2*time.Second, 10*time.Millisecond)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_observations WHERE idempotency_key LIKE $1`, prefix+"%").Scan(&count))
	require.Zero(t, count)
}

func TestReliabilityEvidenceServiceClaimsOneProbePerRouteInterval(t *testing.T) {
	ctx := context.Background()
	evidence := integrationReliabilityEvidence(t)
	claim := &service.ReliabilityProbeClaim{
		ClaimIdentity:    "integration:probe:claim:1",
		RouteFingerprint: "0123456789abcdef0123456789abcdef",
		IntervalStart:    time.Date(2026, 8, 22, 4, 0, 0, 0, time.UTC),
		ExpiresAt:        time.Date(2026, 8, 22, 4, 2, 0, 0, time.UTC),
	}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_probe_claims WHERE claim_key = $1`, claim.ClaimIdentity)
	})

	claimed, err := evidence.ClaimProbe(ctx, claim)
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = evidence.ClaimProbe(ctx, claim)
	require.NoError(t, err)
	require.False(t, claimed)
}

func integrationReliabilityEvidence(t *testing.T) *service.ReliabilityEvidenceService {
	t.Helper()
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.Set(context.Background(), service.SettingKeyReliabilityObservationEnabled, "true"))
	evidence := service.NewReliabilityEvidenceService(integrationDB, settings)
	require.NoError(t, evidence.Start(context.Background()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = evidence.Stop(ctx)
		_ = settings.Set(context.Background(), service.SettingKeyReliabilityObservationEnabled, "false")
	})
	return evidence
}

func boolPointer(value bool) *bool { return &value }
