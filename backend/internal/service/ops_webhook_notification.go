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
	opsTelegramAPIBaseURL     = "https://api.telegram.org"
)

func buildOpsWebhookTestMessage() string {
	return fmt.Sprintf("老实人AI 运维通知测试\n时间：%s\n结果：群通知通道已连通", time.Now().UTC().Format(time.RFC3339))
}

func buildOpsAlertWebhookText(rule *OpsAlertRule, event *OpsAlertEvent) string {
	if rule == nil || event == nil {
		return ""
	}
	value := "-"
	if event.MetricValue != nil {
		value = fmt.Sprintf("%.2f", *event.MetricValue)
	}
	threshold := fmt.Sprintf("%.2f", rule.Threshold)
	if event.ThresholdValue != nil {
		threshold = fmt.Sprintf("%.2f", *event.ThresholdValue)
	}

	lines := []string{
		"老实人AI 运维告警",
		"级别：" + strings.TrimSpace(rule.Severity),
		"规则：" + strings.TrimSpace(rule.Name),
		"状态：" + strings.TrimSpace(event.Status),
		"指标：" + strings.TrimSpace(rule.MetricType) + " " + strings.TrimSpace(rule.Operator) + " " + threshold,
		"当前值：" + value,
		"触发时间：" + event.FiredAt.Format(time.RFC3339),
	}
	if desc := strings.TrimSpace(event.Description); desc != "" {
		lines = append(lines, "说明："+desc)
	}
	return strings.Join(lines, "\n")
}

func sendOpsFeishuText(ctx context.Context, client opsHTTPDoer, cfg OpsFeishuNotificationConfig, text string) error {
	if client == nil {
		client = opsNotificationHTTPClient
	}
	webhookURL := strings.TrimSpace(cfg.WebhookURL)
	if webhookURL == "" {
		return errors.New("feishu webhook url is required")
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
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("%s webhook returned %d: %s", channel, resp.StatusCode, strings.TrimSpace(string(data)))
}
