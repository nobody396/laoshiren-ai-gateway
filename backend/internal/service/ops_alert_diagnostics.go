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
	opsAlertDiagnosticPageSize    = 500
	opsAlertDiagnosticMaxEvidence = 3
	opsAlertCompensationMinErrors = 3
)

type OpsAlertDiagnosis struct {
	RootCause              string
	Impact                 string
	Evidence               []string
	CallerEvidence         []string
	CallerEvidenceOmitted  int
	SuggestedAction        string
	SampleWindowText       string
	CompensationAssessment string
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

	end := event.FiredAt.UTC().Truncate(time.Minute)
	if end.IsZero() {
		end = time.Now().UTC().Truncate(time.Minute)
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
	overview, _ := s.opsRepo.GetDashboardOverview(ctx, &OpsDashboardFilter{
		StartTime: start,
		EndTime:   end,
		Platform:  platform,
		GroupID:   groupID,
		QueryMode: OpsQueryModeRaw,
	})
	diagnosis := summarizeOpsAlertDiagnosis(rule.MetricType, logs, overview)
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
		View:      "all",
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

func summarizeOpsAlertDiagnosis(metricType string, logs []*OpsErrorLog, overview *OpsDashboardOverview) *OpsAlertDiagnosis {
	buckets := map[string]*opsAlertCauseBucket{}
	included := make([]*OpsErrorLog, 0, len(logs))
	excludedProbe := 0
	excludedClient := 0
	excludedBusiness := 0
	excludedCountTokens := 0
	excludedRecovered := 0
	for _, item := range logs {
		if item == nil {
			continue
		}
		switch opsAlertLogExclusion(item, metricType) {
		case "":
			included = append(included, item)
		case "probe":
			excludedProbe++
		case "client":
			excludedClient++
		case "business":
			excludedBusiness++
		case "count_tokens":
			excludedCountTokens++
		case "recovered":
			excludedRecovered++
		}
	}

	for _, item := range included {
		cause, action := classifyOpsAlertErrorCause(item)
		account := strings.TrimSpace(item.AccountName)
		model := strings.TrimSpace(item.Model)
		if model == "" {
			model = strings.TrimSpace(item.RequestedModel)
		}
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
	diagnosis := &OpsAlertDiagnosis{
		Impact:                 opsAlertImpactText(overview, included, excludedProbe, excludedClient, excludedBusiness, excludedCountTokens, excludedRecovered),
		CallerEvidence:         buildOpsAlertCallerEvidence(included),
		CompensationAssessment: opsAlertCompensationAssessment(logs),
	}
	diagnosis.CallerEvidenceOmitted = countDistinctOpsAlertRequests(included) - len(diagnosis.CallerEvidence)
	if diagnosis.CallerEvidenceOmitted < 0 {
		diagnosis.CallerEvidenceOmitted = 0
	}
	if len(buckets) == 0 {
		diagnosis.RootCause = "未找到与告警指标同口径的真实失败；监控探针、客户端错误和已恢复的上游重试均已排除"
		diagnosis.SuggestedAction = "先核对指标聚合窗口；不要根据被排除的探针噪声重启服务或发起赔付。"
		return diagnosis
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
	diagnosis.RootCause = opsAlertRootCauseText(top)
	diagnosis.SuggestedAction = top.Action
	diagnosis.Evidence = make([]string, 0, minInt(len(ordered), opsAlertDiagnosticMaxEvidence))
	for i, bucket := range ordered {
		if i >= opsAlertDiagnosticMaxEvidence {
			break
		}
		diagnosis.Evidence = append(diagnosis.Evidence, opsAlertEvidenceText(bucket))
	}
	return diagnosis
}

func buildOpsAlertCallerEvidence(logs []*OpsErrorLog) []string {
	ordered := append([]*OpsErrorLog(nil), logs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i] == nil {
			return false
		}
		if ordered[j] == nil {
			return true
		}
		return ordered[i].CreatedAt.After(ordered[j].CreatedAt)
	})

	out := make([]string, 0, minInt(len(ordered), opsAlertDiagnosticMaxEvidence))
	seen := map[string]struct{}{}
	for _, item := range ordered {
		if item == nil {
			continue
		}
		key := opsAlertRequestEvidenceKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, opsAlertCallerEvidenceText(item))
		if len(out) >= opsAlertDiagnosticMaxEvidence {
			break
		}
	}
	return out
}

func countDistinctOpsAlertRequests(logs []*OpsErrorLog) int {
	seen := map[string]struct{}{}
	for _, item := range logs {
		if item == nil {
			continue
		}
		seen[opsAlertRequestEvidenceKey(item)] = struct{}{}
	}
	return len(seen)
}

func opsAlertRequestEvidenceKey(item *OpsErrorLog) string {
	if item == nil {
		return "nil"
	}
	if value := strings.TrimSpace(item.RequestID); value != "" {
		return "request:" + value
	}
	if value := strings.TrimSpace(item.ClientRequestID); value != "" {
		return "client-request:" + value
	}
	return fmt.Sprintf("error:%d", item.ID)
}

func opsAlertCallerEvidenceText(item *OpsErrorLog) string {
	if item == nil {
		return ""
	}
	callerType := "真实客户"
	if item.IsInternal {
		callerType = "内部测试"
	}
	identity := []string{callerType}
	if item.UserID != nil && *item.UserID > 0 {
		user := fmt.Sprintf("用户 #%d", *item.UserID)
		if email := compactOpsAlertField(item.UserEmail, 96); email != "" {
			user += " " + email
		}
		identity = append(identity, user)
	} else if email := compactOpsAlertField(item.UserEmail, 96); email != "" {
		identity = append(identity, "用户 "+email)
	} else {
		identity[0] = "调用人未识别"
	}
	if item.APIKeyID != nil && *item.APIKeyID > 0 {
		key := fmt.Sprintf("Key #%d", *item.APIKeyID)
		if name := compactOpsAlertField(item.APIKeyName, 80); name != "" {
			key += "「" + name + "」"
		}
		identity = append(identity, key)
	}

	detail := []string{"报错时间=" + formatOpsAlertLocalTime(item.CreatedAt)}
	if group := compactOpsAlertField(item.GroupName, 80); group != "" {
		detail = append(detail, "分组="+group)
	} else if item.GroupID != nil && *item.GroupID > 0 {
		detail = append(detail, fmt.Sprintf("分组=#%d", *item.GroupID))
	}
	if path := compactOpsAlertField(item.RequestPath, 96); path != "" {
		detail = append(detail, "接口="+path)
	}
	if account := compactOpsAlertField(item.AccountName, 80); account != "" {
		detail = append(detail, "上游账号="+account)
	} else {
		detail = append(detail, "上游账号=未进入调度")
	}
	if model := compactOpsAlertField(firstNonEmptyOpsAlert(item.Model, item.RequestedModel), 80); model != "" {
		detail = append(detail, "模型="+model)
	}
	if item.StatusCode > 0 {
		detail = append(detail, fmt.Sprintf("状态=%d", item.StatusCode))
	}
	cause, _ := classifyOpsAlertErrorCause(item)
	if cause = compactOpsAlertField(cause, 120); cause != "" {
		detail = append(detail, "原因="+cause)
	}
	requestID := strings.TrimSpace(item.RequestID)
	if requestID == "" {
		requestID = strings.TrimSpace(item.ClientRequestID)
	}
	if requestID != "" {
		detail = append(detail, "Request ID="+compactOpsAlertField(requestID, 128))
	}
	return strings.Join(identity, "｜") + "\n  " + strings.Join(detail, "｜")
}

func compactOpsAlertField(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}

func firstNonEmptyOpsAlert(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func opsAlertLogExclusion(item *OpsErrorLog, metricType string) string {
	if item == nil {
		return "client"
	}
	owner := strings.ToLower(strings.TrimSpace(item.Owner))
	source := strings.ToLower(strings.TrimSpace(item.Source))
	if source == "monthly_upstream_probe" || owner == "ops" {
		return "probe"
	}
	if item.IsCountTokens {
		return "count_tokens"
	}
	if item.IsBusinessLimited {
		return "business"
	}

	clientStatus := item.ClientStatusCode
	if clientStatus == 0 {
		// Compatibility for old callers/tests that only populated StatusCode.
		clientStatus = item.StatusCode
	}
	switch strings.TrimSpace(metricType) {
	case "success_rate", "error_rate":
		if clientStatus < 400 {
			return "recovered"
		}
		if owner == "client" || source == "client_request" {
			return "client"
		}
	case "upstream_error_rate":
		if owner != "provider" || item.StatusCode == 429 || item.StatusCode == 529 {
			return "client"
		}
	default:
		if clientStatus < 400 {
			return "recovered"
		}
		if owner == "client" || source == "client_request" {
			return "client"
		}
	}
	return ""
}

func opsAlertImpactText(
	overview *OpsDashboardOverview,
	included []*OpsErrorLog,
	excludedProbe int,
	excludedClient int,
	excludedBusiness int,
	excludedCountTokens int,
	excludedRecovered int,
) string {
	parts := make([]string, 0, 4)
	if overview != nil {
		parts = append(parts, fmt.Sprintf("SLA 样本 %d 次：成功 %d / 真实失败 %d", overview.RequestCountSLA, overview.SuccessCount, overview.ErrorCountSLA))
	} else {
		parts = append(parts, fmt.Sprintf("真实失败证据 %d 条", len(included)))
	}

	users := map[int64]struct{}{}
	groups := map[int64]struct{}{}
	for _, item := range included {
		if item == nil {
			continue
		}
		if item.UserID != nil && *item.UserID > 0 {
			users[*item.UserID] = struct{}{}
		}
		if item.GroupID != nil && *item.GroupID > 0 {
			groups[*item.GroupID] = struct{}{}
		}
	}
	if len(users) > 0 || len(groups) > 0 {
		parts = append(parts, fmt.Sprintf("影响 %d 个用户 / %d 个分组", len(users), len(groups)))
	}

	excluded := make([]string, 0, 5)
	if excludedProbe > 0 {
		excluded = append(excluded, fmt.Sprintf("探针 %d", excludedProbe))
	}
	if excludedClient > 0 {
		excluded = append(excluded, fmt.Sprintf("客户端 %d", excludedClient))
	}
	if excludedBusiness > 0 {
		excluded = append(excluded, fmt.Sprintf("业务限制 %d", excludedBusiness))
	}
	if excludedCountTokens > 0 {
		excluded = append(excluded, fmt.Sprintf("count_tokens %d", excludedCountTokens))
	}
	if excludedRecovered > 0 {
		excluded = append(excluded, fmt.Sprintf("已兜底恢复 %d", excludedRecovered))
	}
	if len(excluded) > 0 {
		parts = append(parts, "已排除噪声："+strings.Join(excluded, "、"))
	}
	return strings.Join(parts, "；")
}

func opsAlertCompensationAssessment(logs []*OpsErrorLog) string {
	eligibleByUser := map[int64]int{}
	totalEligible := 0
	for _, item := range logs {
		if item == nil || item.UserID == nil || *item.UserID <= 0 || *item.UserID == 2 {
			continue
		}
		clientStatus := item.ClientStatusCode
		if clientStatus == 0 {
			clientStatus = item.StatusCode
		}
		if clientStatus < 400 || item.IsBusinessLimited || item.IsCountTokens {
			continue
		}
		if strings.TrimSpace(strings.ToLower(item.Owner)) != "provider" || strings.TrimSpace(strings.ToLower(item.Source)) == "monthly_upstream_probe" {
			continue
		}
		switch item.StatusCode {
		case 500, 502, 503, 504, 520, 524:
		default:
			continue
		}
		totalEligible++
		eligibleByUser[*item.UserID]++
	}

	qualifiedUsers := 0
	qualifiedFailures := 0
	maxFailures := 0
	for _, count := range eligibleByUser {
		if count > maxFailures {
			maxFailures = count
		}
		if count >= opsAlertCompensationMinErrors {
			qualifiedUsers++
			qualifiedFailures += count
		}
	}
	if qualifiedUsers > 0 {
		return fmt.Sprintf("候选 %d 人 / %d 次失败；小时批次会生成草案，仍需人工审批（失败请求本身不扣费）", qualifiedUsers, qualifiedFailures)
	}
	if totalEligible > 0 {
		return fmt.Sprintf("暂不生成：符合规则 %d 次，单个用户最多 %d 次，未达到 %d 次门槛（失败请求本身不扣费；最终以小时批次为准）", totalEligible, maxFailures, opsAlertCompensationMinErrors)
	}
	return "不生成：没有符合规则的真实上游 5xx（失败请求本身不扣费；最终以小时批次为准）"
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
	case strings.Contains(message, "does not allow /v1/messages dispatch"):
		return "分组不允许 /v1/messages 协议，请求在进入上游前被网关拒绝",
			"如果这是内部测试，改用该分组支持的 OpenAI 接口；如果确实要支持 Messages，再单独评估并配置协议能力。"
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
