//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSummarizeOpsAlertDiagnosisFromLogsUsesTopCause(t *testing.T) {
	now := time.Date(2026, 5, 20, 4, 30, 0, 0, time.UTC)
	logs := []*OpsErrorLog{
		{
			ID:          1,
			CreatedAt:   now.Add(-1 * time.Minute),
			Owner:       "provider",
			Source:      "upstream_http",
			StatusCode:  503,
			AccountName: "KNA. 成本1.05r/1usd",
			Model:       "claude-sonnet-4",
			Message:     "No available accounts for model claude-sonnet-4",
		},
		{
			ID:          2,
			CreatedAt:   now.Add(-2 * time.Minute),
			Owner:       "provider",
			Source:      "upstream_http",
			StatusCode:  503,
			AccountName: "KNA. 成本1.05r/1usd",
			Model:       "claude-sonnet-4",
			Message:     "No available accounts for model claude-sonnet-4",
		},
		{
			ID:          3,
			CreatedAt:   now.Add(-30 * time.Second),
			Owner:       "provider",
			Source:      "upstream_http",
			StatusCode:  400,
			AccountName: "dragoncode",
			Model:       "claude-sonnet-4",
			Message:     "unknown tool web_search requested by client",
		},
	}

	diagnosis := summarizeOpsAlertDiagnosisFromLogs(logs)

	require.NotNil(t, diagnosis)
	require.Contains(t, diagnosis.RootCause, "KNA. 成本1.05r/1usd")
	require.Contains(t, diagnosis.RootCause, "二级上游账号池无可用账号")
	require.Contains(t, diagnosis.Impact, "告警窗口内错误样本 3 条")
	require.Contains(t, diagnosis.Impact, "主要根因 2 条")
	require.Len(t, diagnosis.Evidence, 2)
	require.Contains(t, diagnosis.Evidence[0], "2条")
	require.Contains(t, diagnosis.Evidence[0], "状态=503")
	require.Contains(t, diagnosis.SuggestedAction, "暂停或降权")
}

func TestParsePositiveInt64RejectsPartialNumbers(t *testing.T) {
	_, ok := parsePositiveInt64("12abc")
	require.False(t, ok)

	n, ok := parsePositiveInt64("12")
	require.True(t, ok)
	require.Equal(t, int64(12), n)
}
