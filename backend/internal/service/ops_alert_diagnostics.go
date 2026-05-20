package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	opsAlertDiagnosticPageSize    = 80
	opsAlertDiagnosticMaxEvidence = 3
)

type OpsAlertDiagnosis struct {
	RootCause        string
	Impact           string
	Evidence         []string
	SuggestedAction  string
	SampleWindowText string
}

type opsAlertCauseBucket struct {
	Cause     string
	Action    string
	Account   string
	Model     string
	Owner     string
	Source    string
	Status    int
	Count     int
	FirstSeen time.Time
	LastSeen  time.Time
}

func (s *OpsAlertEvaluatorService) buildOpsAlertDiagnosis(ctx context.Context, rule *OpsAlertRule, event *OpsAlertEvent) *OpsAlertDiagnosis {
	if s == nil || s.opsRepo == nil || rule == nil || event == nil {
		return nil
	}
	if !opsAlertMetricUsesErrorLogs(rule.MetricType) {
		return nil
	}

	end := event.FiredAt
	if end.IsZero() {
		end = time.Now().UTC()
	}
	windowMinutes := rule.WindowMinutes
	if windowMinutes <= 0 {
		windowMinutes = 5
	}
	start := end.Add(-time.Duration(windowMinutes) * time.Minute)

	platform, groupID, _ := parseOpsAlertRuleScope(rule.Filters)
	if event.Dimensions != nil {
		if p, ok := event.Dimensions["platform"].(string); ok && strings.TrimSpace(p) != "" {
			platform = strings.TrimSpace(p)
		}
		if parsed := parseOpsAlertDimensionGroupID(event.Dimensions["group_id"]); parsed != nil {
			groupID = parsed
		}
	}

	logs := s.listOpsAlertDiagnosisLogs(ctx, start, end, platform, groupID)
	if len(logs) == 0 {
		return nil
	}
	diagnosis := summarizeOpsAlertDiagnosisFromLogs(logs)
	if diagnosis == nil {
		return nil
	}
	diagnosis.SampleWindowText = fmt.Sprintf("%dm", windowMinutes)
	return diagnosis
}

func opsAlertMetricUsesErrorLogs(metricType string) bool {
	switch strings.TrimSpace(metricType) {
	case "success_rate",
		"error_rate",
		"upstream_error_rate",
		"group_available_accounts",
		"group_available_ratio",
		"account_rate_limited_count",
		"account_error_count",
		"group_rate_limit_ratio",
		"account_error_ratio",
		"overload_account_count":
		return true
	default:
		return false
	}
}

func (s *OpsAlertEvaluatorService) listOpsAlertDiagnosisLogs(ctx context.Context, start time.Time, end time.Time, platform string, groupID *int64) []*OpsErrorLog {
	if s == nil || s.opsRepo == nil {
		return nil
	}
	base := &OpsErrorLogFilter{
		StartTime: &start,
		EndTime:   &end,
		Platform:  strings.TrimSpace(platform),
		GroupID:   groupID,
		View:      "errors",
		Page:      1,
		PageSize:  opsAlertDiagnosticPageSize,
	}

	byID := map[int64]*OpsErrorLog{}
	addList := func(filter *OpsErrorLogFilter) {
		list, err := s.opsRepo.ListErrorLogs(ctx, filter)
		if err != nil || list == nil {
			return
		}
		for _, item := range list.Errors {
			if item == nil || item.ID <= 0 {
				continue
			}
			byID[item.ID] = item
		}
	}

	// Default list covers client-visible errors. Upstream phase list also includes
	// recovered upstream errors that were hidden from the client but still affect
	// availability and alerting.
	addList(base)
	upstream := *base
	upstream.Phase = "upstream"
	addList(&upstream)

	out := make([]*OpsErrorLog, 0, len(byID))
	for _, item := range byID {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func summarizeOpsAlertDiagnosisFromLogs(logs []*OpsErrorLog) *OpsAlertDiagnosis {
	buckets := map[string]*opsAlertCauseBucket{}
	total := 0
	for _, item := range logs {
		if item == nil {
			continue
		}
		total++
		cause, action := classifyOpsAlertErrorCause(item)
		account := strings.TrimSpace(item.AccountName)
		model := strings.TrimSpace(item.Model)
		owner := strings.TrimSpace(item.Owner)
		source := strings.TrimSpace(item.Source)
		status := item.StatusCode
		key := strings.Join([]string{cause, action, account, model, owner, source, fmt.Sprintf("%d", status)}, "\x00")
		bucket := buckets[key]
		if bucket == nil {
			bucket = &opsAlertCauseBucket{
				Cause:   cause,
				Action:  action,
				Account: account,
				Model:   model,
				Owner:   owner,
				Source:  source,
				Status:  status,
			}
			buckets[key] = bucket
		}
		bucket.Count++
		if bucket.FirstSeen.IsZero() || item.CreatedAt.Before(bucket.FirstSeen) {
			bucket.FirstSeen = item.CreatedAt
		}
		if bucket.LastSeen.IsZero() || item.CreatedAt.After(bucket.LastSeen) {
			bucket.LastSeen = item.CreatedAt
		}
	}
	if len(buckets) == 0 {
		return nil
	}

	ordered := make([]*opsAlertCauseBucket, 0, len(buckets))
	for _, bucket := range buckets {
		ordered = append(ordered, bucket)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Count != ordered[j].Count {
			return ordered[i].Count > ordered[j].Count
		}
		if !ordered[i].LastSeen.Equal(ordered[j].LastSeen) {
			return ordered[i].LastSeen.After(ordered[j].LastSeen)
		}
		return ordered[i].Cause < ordered[j].Cause
	})

	top := ordered[0]
	diagnosis := &OpsAlertDiagnosis{
		RootCause:       opsAlertRootCauseText(top),
		Impact:          opsAlertImpactText(total, top),
		SuggestedAction: top.Action,
		Evidence:        make([]string, 0, minInt(len(ordered), opsAlertDiagnosticMaxEvidence)),
	}
	for i, bucket := range ordered {
		if i >= opsAlertDiagnosticMaxEvidence {
			break
		}
		diagnosis.Evidence = append(diagnosis.Evidence, opsAlertEvidenceText(bucket))
	}
	return diagnosis
}

func classifyOpsAlertErrorCause(item *OpsErrorLog) (cause string, action string) {
	if item == nil {
		return "未知错误", "打开 /admin/ops 查看最近错误日志，按账号和状态码继续定位。"
	}
	message := strings.ToLower(strings.TrimSpace(item.Message))
	owner := strings.ToLower(strings.TrimSpace(item.Owner))
	source := strings.ToLower(strings.TrimSpace(item.Source))
	status := item.StatusCode

	switch {
	case strings.Contains(message, "no available accounts"):
		return "二级上游账号池无可用账号",
			"先暂停或降权对应二级中转账号；到对方平台补充/恢复它后面的官方账号池；恢复后再重新启用。"
	case strings.Contains(message, "web_search"):
		return "Claude Code 客户端版本过旧，仍在请求已废弃的 web_search 工具",
			"让受影响客户端执行 npm install -g @anthropic-ai/claude-code@latest，重启 Claude Code 后重试。"
	case strings.Contains(message, "context canceled"):
		return "请求或流式连接中途取消",
			"通常是客户端断开或二级上游流中断；若持续出现，检查客户端超时、代理链路和上游流式稳定性。"
	case owner == "client" || source == "client_request":
		if status == 401 {
			return "客户端鉴权失败，API Key 缺失、过期或填错",
				"让用户检查 Base URL、Authorization/x-api-key，以及后台 API Key 是否仍有效。"
		}
		return "客户端请求参数或鉴权问题，不属于上游供应商故障",
			"先按请求 ID 查看错误详情；修正客户端参数、模型名、鉴权或请求体后重试。"
	case status == 429:
		return "上游或二级中转触发限流",
			"降低该上游权重或并发，等待限流窗口恢复；必要时增加可用账号池容量。"
	case status == 502 || status == 503 || status == 504:
		return "二级上游服务暂不可用或链路超时",
			"先暂停/降权异常上游账号，切到备用上游；同时检查二级中转站状态和网络链路。"
	case status == 400:
		return "请求被上游拒绝，可能是客户端兼容性、模型名或参数不兼容",
			"查看错误详情里的上游 message；优先升级客户端、修正模型映射或删除不兼容参数。"
	case owner == "provider" || source == "upstream_http":
		return "上游供应商或二级中转返回错误",
			"按账号维度查看最近错误；先降权异常上游，再联系上游或切备用线路。"
	default:
		return "平台或上游请求失败增多",
			"打开 /admin/ops 查看最近错误日志，按账号、模型、状态码聚合后处理。"
	}
}

func opsAlertRootCauseText(bucket *opsAlertCauseBucket) string {
	if bucket == nil {
		return ""
	}
	prefix := ""
	if bucket.Account != "" {
		prefix = "账号/上游「" + bucket.Account + "」："
	}
	return prefix + bucket.Cause
}

func opsAlertImpactText(total int, bucket *opsAlertCauseBucket) string {
	if bucket == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("告警窗口内错误样本 %d 条", total)}
	if bucket.Count > 0 {
		parts = append(parts, fmt.Sprintf("主要根因 %d 条", bucket.Count))
	}
	if bucket.Account != "" {
		parts = append(parts, "集中账号/上游："+bucket.Account)
	}
	if bucket.Model != "" {
		parts = append(parts, "模型："+bucket.Model)
	}
	return strings.Join(parts, "，")
}

func opsAlertEvidenceText(bucket *opsAlertCauseBucket) string {
	if bucket == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("%d条", bucket.Count), bucket.Cause}
	if bucket.Account != "" {
		parts = append(parts, "账号/上游="+bucket.Account)
	}
	if bucket.Model != "" {
		parts = append(parts, "模型="+bucket.Model)
	}
	if bucket.Status > 0 {
		parts = append(parts, fmt.Sprintf("状态=%d", bucket.Status))
	}
	if bucket.Owner != "" {
		parts = append(parts, "责任="+opsAlertOwnerLabel(bucket.Owner))
	}
	return strings.Join(parts, "，")
}

func opsAlertOwnerLabel(owner string) string {
	switch strings.TrimSpace(strings.ToLower(owner)) {
	case "client":
		return "客户端"
	case "provider":
		return "上游/供应商"
	case "platform":
		return "本平台"
	default:
		return strings.TrimSpace(owner)
	}
}

func parseOpsAlertDimensionGroupID(value any) *int64 {
	switch v := value.(type) {
	case int64:
		if v > 0 {
			return &v
		}
	case int:
		if v > 0 {
			n := int64(v)
			return &n
		}
	case float64:
		if v > 0 {
			n := int64(v)
			return &n
		}
	case string:
		if n, ok := parsePositiveInt64(v); ok {
			return &n
		}
	}
	return nil
}

func parsePositiveInt64(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
