package main

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReliabilityBackfillAuditSQLIsReadOnlyAndBounded(t *testing.T) {
	sql := strings.ToLower(reliabilityBackfillAuditSQL())
	require.Contains(t, sql, "with source_rows as")
	require.Contains(t, sql, "limit $3")
	for _, forbidden := range []string{" insert ", " update ", " delete ", " alter ", " drop ", " truncate ", " copy "} {
		require.NotContains(t, " "+strings.Join(strings.Fields(sql), " ")+" ", forbidden)
	}
}

func TestParseAuditWindowRejectsMoreThanSevenDays(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	_, _, err := parseAuditWindow("2026-08-01", "2026-08-09", location, time.Time{})
	require.ErrorContains(t, err, "seven days")

	start, end, err := parseAuditWindow("2026-08-01", "2026-08-08", location, time.Time{})
	require.NoError(t, err)
	require.Equal(t, 7*24*time.Hour, end.Sub(start))
}

func TestClassifyAuditCandidatesEmitsRecoveredRequestOnceAndKeepsAttempt(t *testing.T) {
	now := time.Date(2026, 8, 22, 4, 0, 0, 0, time.UTC)
	candidates := classifyAuditCandidates([]auditSourceRow{
		{Source: "usage_logs", SourceID: 10, RequestID: "req-1", ObservedAt: now.Add(time.Second)},
		{Source: "ops_error_logs", SourceID: 20, RequestID: "req-1", ClientRequestID: "client-1", ErrorOwner: "provider", ObservedAt: now},
	})

	require.Len(t, candidates, 2)
	counts := map[string]int{}
	for _, candidate := range candidates {
		counts[candidate.FactType+":"+candidate.Outcome]++
	}
	require.Equal(t, 1, counts["customer_request:recovered"])
	require.Equal(t, 1, counts["upstream_attempt:failure"])
	require.Zero(t, counts["customer_request:failure"])
}
