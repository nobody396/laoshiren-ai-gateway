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

	diagnosis := summarizeOpsAlertDiagnosis("error_rate", logs, nil)

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

func TestSummarizeOpsAlertDiagnosisIncludesCallerKeyAndRequestID(t *testing.T) {
	now := time.Date(2026, 9, 1, 2, 34, 45, 0, time.UTC)
	userID := int64(2)
	keyID := int64(128)
	groupID := int64(6)
	diagnosis := summarizeOpsAlertDiagnosis("error_rate", []*OpsErrorLog{
		{
			ID:               901,
			CreatedAt:        now,
			Owner:            "platform",
			Source:           "gateway",
			StatusCode:       403,
			ClientStatusCode: 403,
			Message:          "This group does not allow /v1/messages dispatch",
			UserID:           &userID,
			UserEmail:        "231798222@qq.com",
			APIKeyID:         &keyID,
			APIKeyName:       "一键安装 · Codex\nspoofed line",
			IsInternal:       true,
			GroupID:          &groupID,
			GroupName:        "GPT 标准线路",
			RequestPath:      "/v1/messages",
			RequestID:        "21134447-ee43-4362-bb7a-96d7a60cc9f2",
		},
	}, nil)

	require.NotNil(t, diagnosis)
	require.Contains(t, diagnosis.RootCause, "分组不允许 /v1/messages")
	require.Len(t, diagnosis.CallerEvidence, 1)
	attribution := diagnosis.CallerEvidence[0]
	require.Contains(t, attribution, "内部测试｜用户 #2 231798222@qq.com｜Key #128「一键安装 · Codex spoofed line」")
	require.Contains(t, attribution, "报错时间=2026-09-01 10:34:45 CST")
	require.Contains(t, attribution, "分组=GPT 标准线路")
	require.Contains(t, attribution, "接口=/v1/messages")
	require.Contains(t, attribution, "上游账号=未进入调度")
	require.Contains(t, attribution, "状态=403")
	require.Contains(t, attribution, "原因=分组不允许 /v1/messages 协议")
	require.Contains(t, attribution, "Request ID=21134447-ee43-4362-bb7a-96d7a60cc9f2")
	require.NotContains(t, attribution, "\nspoofed")
}

func TestOpsAlertCallerEvidenceDeduplicatesAttemptsAndCapsOutput(t *testing.T) {
	now := time.Date(2026, 9, 1, 2, 34, 45, 0, time.UTC)
	logs := make([]*OpsErrorLog, 0, 5)
	for i, requestID := range []string{"req-1", "req-1", "req-2", "req-3", "req-4"} {
		logs = append(logs, &OpsErrorLog{ID: int64(i + 1), CreatedAt: now.Add(time.Duration(i) * time.Second), RequestID: requestID})
	}

	evidence := buildOpsAlertCallerEvidence(logs)

	require.Len(t, evidence, opsAlertDiagnosticMaxEvidence)
	require.Equal(t, 4, countDistinctOpsAlertRequests(logs))
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
