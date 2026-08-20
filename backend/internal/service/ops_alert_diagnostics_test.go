//go:build unit

package service

import (
	"strings"
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
	require.Contains(t, diagnosis.Impact, "真实失败证据 3 条")
	require.Len(t, diagnosis.Evidence, 2)
	require.Contains(t, diagnosis.Evidence[0], "2条")
	require.Contains(t, diagnosis.Evidence[0], "状态=503")
	require.Contains(t, diagnosis.SuggestedAction, "暂停或降权")
}

func TestSummarizeOpsAlertDiagnosisAlignsWithSLAMetricAndExplainsCompensation(t *testing.T) {
	now := time.Date(2026, 8, 20, 7, 49, 0, 0, time.UTC)
	userID := int64(91)
	groupID := int64(41)
	logs := []*OpsErrorLog{
		{
			ID: 1, CreatedAt: now.Add(-time.Minute), Owner: "ops", Source: "monthly_upstream_probe",
			StatusCode: 502, ClientStatusCode: 502, AccountName: "probe-account",
		},
		{
			ID: 2, CreatedAt: now.Add(-2 * time.Minute), Owner: "provider", Source: "upstream_http",
			StatusCode: 502, ClientStatusCode: 200, AccountName: "recovered-account", UserID: &userID,
		},
		{
			ID: 3, CreatedAt: now.Add(-3 * time.Minute), Owner: "client", Source: "client_request",
			StatusCode: 400, ClientStatusCode: 400, UserID: &userID,
		},
		{
			ID: 4, CreatedAt: now.Add(-4 * time.Minute), Owner: "provider", Source: "upstream_http",
			StatusCode: 502, ClientStatusCode: 502, AccountName: "claude-upstream", Model: "claude-opus-5", UserID: &userID, GroupID: &groupID,
		},
		{
			ID: 5, CreatedAt: now.Add(-4*time.Minute - time.Second), Owner: "provider", Source: "upstream_http",
			StatusCode: 502, ClientStatusCode: 502, AccountName: "claude-upstream", Model: "claude-opus-5", UserID: &userID, GroupID: &groupID,
		},
	}
	overview := &OpsDashboardOverview{SuccessCount: 13, ErrorCountSLA: 2, RequestCountSLA: 15}

	diagnosis := summarizeOpsAlertDiagnosis("error_rate", logs, overview)

	require.NotNil(t, diagnosis)
	require.Contains(t, diagnosis.Impact, "SLA 样本 15 次：成功 13 / 真实失败 2")
	require.Contains(t, diagnosis.Impact, "影响 1 个用户 / 1 个分组")
	require.Contains(t, diagnosis.Impact, "探针 1")
	require.Contains(t, diagnosis.Impact, "客户端 1")
	require.Contains(t, diagnosis.Impact, "已兜底恢复 1")
	require.NotContains(t, strings.Join(diagnosis.Evidence, "\n"), "probe-account")
	require.Contains(t, diagnosis.CompensationAssessment, "符合规则 2 次")
	require.Contains(t, diagnosis.CompensationAssessment, "未达到 3 次门槛")
}

func TestOpsAlertCompensationAssessmentMarksQualifiedUserAsCandidate(t *testing.T) {
	userID := int64(99)
	logs := make([]*OpsErrorLog, 0, 3)
	for id := int64(1); id <= 3; id++ {
		logs = append(logs, &OpsErrorLog{
			ID: id, Owner: "provider", Source: "upstream_http", StatusCode: 503,
			ClientStatusCode: 503, UserID: &userID,
		})
	}

	assessment := opsAlertCompensationAssessment(logs)

	require.Contains(t, assessment, "候选 1 人 / 3 次失败")
	require.Contains(t, assessment, "仍需人工审批")
}

func TestParsePositiveInt64RejectsPartialNumbers(t *testing.T) {
	_, ok := parsePositiveInt64("12abc")
	require.False(t, ok)

	n, ok := parsePositiveInt64("12")
	require.True(t, ok)
	require.Equal(t, int64(12), n)
}
