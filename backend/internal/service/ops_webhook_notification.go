package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type opsHTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var (
	opsNotificationHTTPClient = &http.Client{Timeout: 8 * time.Second}
	opsFeishuAPIBaseURL       = "https://open.feishu.cn"
	opsTelegramAPIBaseURL     = "https://api.telegram.org"
)

func buildOpsWebhookTestMessage() string {
	return fmt.Sprintf("老实人AI 运维通知测试\n时间：%s\n结果：群通知通道已连通", time.Now().UTC().Format(time.RFC3339))
}

func buildOpsAlertWebhookText(rule *OpsAlertRule, event *OpsAlertEvent) string {
	return buildOpsAlertWebhookTextWithDiagnosis(rule, event, nil)
}

func buildOpsAlertWebhookTextWithDiagnosis(rule *OpsAlertRule, event *OpsAlertEvent, diagnosis *OpsAlertDiagnosis) string {
	if rule == nil || event == nil {
		return ""
	}
	value := formatOpsAlertMetricValue(rule.MetricType, event.MetricValue)
	threshold := formatOpsAlertMetricValue(rule.MetricType, float64Ptr(rule.Threshold))
	if event.ThresholdValue != nil {
		threshold = formatOpsAlertMetricValue(rule.MetricType, event.ThresholdValue)
	}
	metricLabel := opsAlertMetricLabel(rule.MetricType)
	severityLabel := opsAlertSeverityLabel(rule.Severity)
	rootCause := buildOpsAlertLikelyCause(rule)
	suggestedAction := buildOpsAlertSuggestedAction(rule)
	if diagnosis != nil {
		if value := strings.TrimSpace(diagnosis.RootCause); value != "" {
			rootCause = value
		}
		if value := strings.TrimSpace(diagnosis.SuggestedAction); value != "" {
			suggestedAction = value
		}
	}

	lines := []string{
		"老实人AI 运维告警",
		"结论：" + buildOpsAlertPlainConclusion(rule, event),
		"级别：" + severityLabel,
		"规则：" + strings.TrimSpace(rule.Name),
		"状态：" + strings.TrimSpace(event.Status),
		"根因判断：" + rootCause,
	}
	if diagnosis != nil {
		if impact := strings.TrimSpace(diagnosis.Impact); impact != "" {
			lines = append(lines, "影响范围："+impact)
		}
		if window := strings.TrimSpace(diagnosis.SampleWindowText); window != "" {
			lines = append(lines, "样本窗口：最近 "+window)
		}
		if len(diagnosis.Evidence) > 0 {
			lines = append(lines, "根因证据：")
			for _, evidence := range diagnosis.Evidence {
				evidence = strings.TrimSpace(evidence)
				if evidence == "" {
					continue
				}
				lines = append(lines, "- "+evidence)
			}
		}
	}
	lines = append(lines,
		"处理建议："+suggestedAction,
		"指标："+metricLabel+" "+strings.TrimSpace(rule.Operator)+" "+threshold+"，当前 "+value,
		"触发时间："+formatOpsAlertLocalTime(event.FiredAt),
	)
	if desc := strings.TrimSpace(event.Description); desc != "" {
		lines = append(lines, "技术细节："+desc)
	}
	return strings.Join(lines, "\n")
}

func formatOpsAlertMetricValue(metricType string, value *float64) string {
	if value == nil {
		return "-"
	}
	switch strings.TrimSpace(metricType) {
	case "success_rate", "error_rate", "upstream_error_rate", "cpu_usage_percent", "memory_usage_percent", "group_available_ratio", "group_rate_limit_ratio", "account_error_ratio":
		return fmt.Sprintf("%.2f%%", *value)
	case "p95_latency_ms", "p99_latency_ms":
		return fmt.Sprintf("%.0fms", *value)
	default:
		return fmt.Sprintf("%.2f", *value)
	}
}

func opsAlertMetricLabel(metricType string) string {
	switch strings.TrimSpace(metricType) {
	case "success_rate":
		return "成功率"
	case "error_rate":
		return "错误率"
	case "upstream_error_rate":
		return "上游错误率"
	case "p95_latency_ms":
		return "P95 延迟"
	case "p99_latency_ms":
		return "P99 延迟"
	case "cpu_usage_percent":
		return "CPU 使用率"
	case "memory_usage_percent":
		return "内存使用率"
	case "concurrency_queue_depth":
		return "并发队列"
	case "group_available_accounts":
		return "可用账号数"
	case "group_available_ratio":
		return "可用账号比例"
	case "account_rate_limited_count":
		return "被限流账号数"
	case "account_error_count":
		return "异常账号数"
	case "group_rate_limit_ratio":
		return "限流账号比例"
	case "account_error_ratio":
		return "异常账号比例"
	case "overload_account_count":
		return "过载账号数"
	default:
		return strings.TrimSpace(metricType)
	}
}

func opsAlertSeverityLabel(severity string) string {
	switch strings.TrimSpace(severity) {
	case "P0":
		return "P0（最高优先级，可能影响可用性）"
	case "P1":
		return "P1（需要尽快处理）"
	case "P2":
		return "P2（需要关注）"
	case "P3":
		return "P3（低优先级）"
	default:
		return strings.TrimSpace(severity)
	}
}

func buildOpsAlertPlainConclusion(rule *OpsAlertRule, event *OpsAlertEvent) string {
	metric := opsAlertMetricLabel(rule.MetricType)
	value := formatOpsAlertMetricValue(rule.MetricType, event.MetricValue)
	switch strings.TrimSpace(rule.MetricType) {
	case "success_rate":
		return "系统成功率低于阈值，当前 " + value
	case "error_rate":
		return "系统错误率高于阈值，当前 " + value
	case "upstream_error_rate":
		return "上游供应商错误率高于阈值，当前 " + value
	case "p95_latency_ms", "p99_latency_ms":
		return "请求延迟高于阈值，当前 " + value
	case "cpu_usage_percent", "memory_usage_percent":
		return metric + "高于阈值，当前 " + value
	default:
		if name := strings.TrimSpace(rule.Name); name != "" {
			return name + " 已触发"
		}
		return "运维规则已触发"
	}
}

func buildOpsAlertLikelyCause(rule *OpsAlertRule) string {
	switch strings.TrimSpace(rule.MetricType) {
	case "success_rate", "error_rate":
		return "平台或上游请求失败增多；客户端 401/400 这类用户请求错误不会再计入系统错误率。"
	case "upstream_error_rate":
		return "上游模型供应商返回 5xx/异常响应增多，优先检查账号池和供应商状态。"
	case "p95_latency_ms", "p99_latency_ms":
		return "上游响应慢、网络抖动或队列积压导致请求变慢。"
	case "cpu_usage_percent":
		return "服务器 CPU 压力升高，可能是请求量升高或后台任务集中运行。"
	case "memory_usage_percent":
		return "服务器内存压力升高，可能存在缓存增长、请求峰值或后台任务占用。"
	case "concurrency_queue_depth":
		return "并发请求排队过多，当前处理能力跟不上瞬时请求量。"
	case "group_available_accounts", "group_available_ratio", "account_rate_limited_count", "account_error_count", "group_rate_limit_ratio", "account_error_ratio", "overload_account_count":
		return "账号池可用性下降，可能是账号限流、余额/权限异常或上游返回异常。"
	default:
		return "需要结合运维面板的错误日志、账号状态和资源监控进一步确认。"
	}
}

func buildOpsAlertSuggestedAction(rule *OpsAlertRule) string {
	switch strings.TrimSpace(rule.MetricType) {
	case "success_rate", "error_rate":
		return "先看运维面板的错误日志。如果主要是 provider/platform 错误再处理；如果都是 client/auth，可忽略。"
	case "upstream_error_rate":
		return "先看账号健康和上游状态，必要时切换可用账号或临时降低异常供应商权重。"
	case "p95_latency_ms", "p99_latency_ms":
		return "先看队列、CPU/内存和上游延迟趋势，判断是服务器压力还是供应商变慢。"
	case "cpu_usage_percent", "memory_usage_percent":
		return "先看服务器资源曲线和最近部署/后台任务；持续不恢复再扩容或限流。"
	case "concurrency_queue_depth":
		return "先看并发队列和请求峰值，必要时提升并发处理能力或做流量控制。"
	default:
		return "打开 /admin/ops 查看对应详情，优先处理持续触发且影响用户请求的告警。"
	}
}

func formatOpsAlertLocalTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	loc := time.FixedZone("CST", 8*60*60)
	return t.In(loc).Format("2006-01-02 15:04:05 CST")
}

func sendOpsFeishuText(ctx context.Context, client opsHTTPDoer, cfg OpsFeishuNotificationConfig, text string) error {
	if client == nil {
		client = opsNotificationHTTPClient
	}
	webhookURL := strings.TrimSpace(cfg.WebhookURL)
	if webhookURL == "" {
		return sendOpsFeishuAppBotText(ctx, client, cfg, text)
	}
	payload := map[string]any{
		"msg_type": "text",
		"content": map[string]string{
			"text": text,
		},
	}
	if secret := strings.TrimSpace(cfg.Secret); secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		payload["timestamp"] = timestamp
		payload["sign"] = signFeishuWebhook(timestamp, secret)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return doOpsWebhookRequest(client, req, "feishu")
}

func sendOpsFeishuAppBotText(ctx context.Context, client opsHTTPDoer, cfg OpsFeishuNotificationConfig, text string) error {
	appID := strings.TrimSpace(cfg.AppID)
	appSecret := strings.TrimSpace(cfg.AppSecret)
	chatID := strings.TrimSpace(cfg.ChatID)
	if appID == "" || appSecret == "" || chatID == "" {
		return errors.New("feishu webhook url or app bot credentials are required")
	}

	token, err := getOpsFeishuTenantAccessToken(ctx, client, appID, appSecret)
	if err != nil {
		return err
	}

	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	payload := map[string]string{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(content),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	apiBase := strings.TrimRight(opsFeishuAPIBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/open-apis/im/v1/messages?receive_id_type=chat_id", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return doOpsWebhookRequest(client, req, "feishu")
}

func getOpsFeishuTenantAccessToken(ctx context.Context, client opsHTTPDoer, appID string, appSecret string) (string, error) {
	payload := map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	apiBase := strings.TrimRight(opsFeishuAPIBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("feishu token api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var payloadResp struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(data, &payloadResp); err != nil {
		return "", err
	}
	if payloadResp.Code != 0 {
		return "", fmt.Errorf("feishu token api returned code %d: %s", payloadResp.Code, payloadResp.Msg)
	}
	token := strings.TrimSpace(payloadResp.TenantAccessToken)
	if token == "" {
		return "", errors.New("feishu token api returned empty tenant_access_token")
	}
	return token, nil
}

func signFeishuWebhook(timestamp string, secret string) string {
	stringToSign := timestamp + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func sendOpsTelegramText(ctx context.Context, client opsHTTPDoer, cfg OpsTelegramNotificationConfig, text string) error {
	if client == nil {
		client = opsNotificationHTTPClient
	}
	token := strings.TrimSpace(cfg.BotToken)
	chatID := strings.TrimSpace(cfg.ChatID)
	if token == "" {
		return errors.New("telegram bot token is required")
	}
	if chatID == "" {
		return errors.New("telegram chat id is required")
	}

	form := url.Values{}
	form.Set("chat_id", chatID)
	form.Set("text", text)
	apiBase := strings.TrimRight(opsTelegramAPIBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/bot"+token+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doOpsWebhookRequest(client, req, "telegram")
}

func doOpsWebhookRequest(client opsHTTPDoer, req *http.Request, channel string) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err := validateOpsWebhookSuccessBody(channel, string(data)); err != nil {
			return err
		}
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("%s webhook returned %d: %s", channel, resp.StatusCode, strings.TrimSpace(string(data)))
}

func validateOpsWebhookSuccessBody(channel string, body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil
	}
	switch channel {
	case "feishu":
		if code, ok := jsonNumberAsFloat(payload["code"]); ok && code != 0 {
			return fmt.Errorf("feishu webhook returned code %.0f: %v", code, payload["msg"])
		}
		if code, ok := jsonNumberAsFloat(payload["StatusCode"]); ok && code != 0 {
			return fmt.Errorf("feishu webhook returned status %.0f: %v", code, payload["StatusMessage"])
		}
	case "telegram":
		if ok, exists := payload["ok"].(bool); exists && !ok {
			return fmt.Errorf("telegram webhook returned ok=false: %v", payload["description"])
		}
	}
	return nil
}

func jsonNumberAsFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
