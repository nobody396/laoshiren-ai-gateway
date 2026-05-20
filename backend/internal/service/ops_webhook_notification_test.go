//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type opsWebhookSettingRepoStub struct {
	values map[string]string
}

func (s *opsWebhookSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *opsWebhookSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.values == nil {
		s.values = map[string]string{}
	}
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *opsWebhookSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *opsWebhookSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (s *opsWebhookSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *opsWebhookSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *opsWebhookSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func TestOpsWebhookConfigRedactsAndPreservesSecrets(t *testing.T) {
	ctx := context.Background()
	repo := &opsWebhookSettingRepoStub{}
	svc := &OpsService{settingRepo: repo}

	updated, err := svc.UpdateWebhookNotificationConfig(ctx, &OpsWebhookNotificationConfigUpdateRequest{
		Feishu: &OpsFeishuNotificationConfig{
			Enabled:          true,
			Name:             "飞书主群",
			WebhookURL:       "https://open.feishu.cn/open-apis/bot/v2/hook/test",
			Secret:           "signing-secret",
			AppID:            "cli_test",
			AppSecret:        "app-secret",
			ChatID:           "oc_test",
			MinSeverity:      "warning",
			RateLimitPerHour: 5,
		},
		Telegram: &OpsTelegramNotificationConfig{
			Enabled:          true,
			Name:             "Telegram 主群",
			BotToken:         "123456:test-token",
			ChatID:           "-100123456",
			MinSeverity:      "critical",
			RateLimitPerHour: 3,
		},
	})
	require.NoError(t, err)
	require.True(t, updated.Feishu.WebhookURLConfigured)
	require.True(t, updated.Feishu.SecretConfigured)
	require.True(t, updated.Feishu.AppIDConfigured)
	require.True(t, updated.Feishu.AppSecretConfigured)
	require.Empty(t, updated.Feishu.WebhookURL)
	require.Empty(t, updated.Feishu.Secret)
	require.Empty(t, updated.Feishu.AppID)
	require.Empty(t, updated.Feishu.AppSecret)
	require.Equal(t, "oc_test", updated.Feishu.ChatID)
	require.True(t, updated.Telegram.BotTokenConfigured)
	require.Empty(t, updated.Telegram.BotToken)

	updated, err = svc.UpdateWebhookNotificationConfig(ctx, &OpsWebhookNotificationConfigUpdateRequest{
		Feishu: &OpsFeishuNotificationConfig{
			Enabled:          true,
			Name:             "飞书新名称",
			MinSeverity:      "info",
			RateLimitPerHour: 8,
		},
		Telegram: &OpsTelegramNotificationConfig{
			Enabled:          true,
			Name:             "Telegram 新名称",
			ChatID:           "-100999",
			MinSeverity:      "warning",
			RateLimitPerHour: 4,
		},
	})
	require.NoError(t, err)
	require.True(t, updated.Feishu.WebhookURLConfigured)
	require.True(t, updated.Feishu.SecretConfigured)
	require.True(t, updated.Telegram.BotTokenConfigured)

	raw := &OpsWebhookNotificationConfig{}
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingKeyOpsWebhookNotificationConfig]), raw))
	require.Equal(t, "https://open.feishu.cn/open-apis/bot/v2/hook/test", raw.Feishu.WebhookURL)
	require.Equal(t, "signing-secret", raw.Feishu.Secret)
	require.Equal(t, "cli_test", raw.Feishu.AppID)
	require.Equal(t, "app-secret", raw.Feishu.AppSecret)
	require.Equal(t, "oc_test", raw.Feishu.ChatID)
	require.Equal(t, "123456:test-token", raw.Telegram.BotToken)
	require.Equal(t, "-100999", raw.Telegram.ChatID)
}

func TestOpsWebhookConfigValidation(t *testing.T) {
	svc := &OpsService{settingRepo: &opsWebhookSettingRepoStub{}}
	_, err := svc.UpdateWebhookNotificationConfig(context.Background(), &OpsWebhookNotificationConfigUpdateRequest{
		Feishu: &OpsFeishuNotificationConfig{Enabled: true},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "feishu.webhook_url")
}

func TestOpsWebhookConfigAllowsFeishuAppBot(t *testing.T) {
	svc := &OpsService{settingRepo: &opsWebhookSettingRepoStub{}}
	updated, err := svc.UpdateWebhookNotificationConfig(context.Background(), &OpsWebhookNotificationConfigUpdateRequest{
		Feishu: &OpsFeishuNotificationConfig{
			Enabled:          true,
			Name:             "飞书告警群",
			AppID:            "cli_test",
			AppSecret:        "app-secret",
			ChatID:           "oc_test",
			MinSeverity:      "warning",
			RateLimitPerHour: 20,
		},
	})
	require.NoError(t, err)
	require.True(t, updated.Feishu.AppIDConfigured)
	require.True(t, updated.Feishu.AppSecretConfigured)
	require.Equal(t, "oc_test", updated.Feishu.ChatID)
}

func TestOpsAlertWebhookNotificationsSendToConfiguredGroups(t *testing.T) {
	var feishuHits int
	var telegramHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.URL.Path == "/feishu":
			feishuHits++
			require.Contains(t, string(body), "老实人AI 运维告警")
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/bot123456:test-token/sendMessage":
			telegramHits++
			require.Contains(t, string(body), "chat_id=-100123456")
			require.Contains(t, string(body), "text=")
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected webhook path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oldTelegramBaseURL := opsTelegramAPIBaseURL
	opsTelegramAPIBaseURL = server.URL
	defer func() { opsTelegramAPIBaseURL = oldTelegramBaseURL }()

	cfg := &OpsWebhookNotificationConfig{
		Feishu: OpsFeishuNotificationConfig{
			Enabled:          true,
			WebhookURL:       server.URL + "/feishu",
			MinSeverity:      "warning",
			RateLimitPerHour: 0,
		},
		Telegram: OpsTelegramNotificationConfig{
			Enabled:          true,
			BotToken:         "123456:test-token",
			ChatID:           "-100123456",
			MinSeverity:      "warning",
			RateLimitPerHour: 0,
		},
	}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)

	repo := &opsWebhookSettingRepoStub{values: map[string]string{
		SettingKeyOpsWebhookNotificationConfig: string(raw),
	}}
	svc := &OpsAlertEvaluatorService{
		opsService:      &OpsService{settingRepo: repo},
		feishuLimiter:   newSlidingWindowLimiter(0, time.Hour),
		telegramLimiter: newSlidingWindowLimiter(0, time.Hour),
	}

	value := 99.9
	threshold := 5.0
	sent := svc.maybeSendAlertWebhooks(context.Background(), nil, &OpsAlertRule{
		ID:          1,
		Name:        "错误率过高",
		MetricType:  "request_error_rate",
		Operator:    ">",
		Threshold:   threshold,
		Severity:    "P1",
		NotifyEmail: true,
	}, &OpsAlertEvent{
		ID:             9,
		Status:         OpsAlertStatusFiring,
		MetricValue:    &value,
		ThresholdValue: &threshold,
		FiredAt:        time.Now().UTC(),
		Description:    "request_error_rate > 5",
	})
	require.True(t, sent)
	require.Equal(t, 1, feishuHits)
	require.Equal(t, 1, telegramHits)
}

func TestBuildOpsAlertWebhookTextExplainsRootCause(t *testing.T) {
	value := 100.0
	threshold := 20.0
	text := buildOpsAlertWebhookText(&OpsAlertRule{
		Name:       "错误率极高",
		MetricType: "error_rate",
		Operator:   ">",
		Threshold:  threshold,
		Severity:   "P0",
	}, &OpsAlertEvent{
		Status:         OpsAlertStatusFiring,
		MetricValue:    &value,
		ThresholdValue: &threshold,
		FiredAt:        time.Date(2026, 5, 18, 5, 11, 0, 0, time.FixedZone("CST", 8*60*60)),
		Description:    "error_rate > 20.00 (current 100.00) over last 1m (overall)",
	})

	require.Contains(t, text, "结论：系统错误率高于阈值，当前 100.00%")
	require.Contains(t, text, "级别：P0（最高优先级，可能影响可用性）")
	require.Contains(t, text, "根因判断：平台或上游请求失败增多")
	require.Contains(t, text, "客户端 401/400 这类用户请求错误不会再计入系统错误率")
	require.Contains(t, text, "处理建议：先看运维面板的错误日志")
	require.Contains(t, text, "触发时间：2026-05-18 05:11:00 CST")
}

func TestBuildOpsAlertWebhookTextIncludesDynamicDiagnosis(t *testing.T) {
	value := 100.0
	threshold := 20.0
	text := buildOpsAlertWebhookTextWithDiagnosis(&OpsAlertRule{
		Name:       "错误率极高",
		MetricType: "error_rate",
		Operator:   ">",
		Threshold:  threshold,
		Severity:   "P0",
	}, &OpsAlertEvent{
		Status:         OpsAlertStatusFiring,
		MetricValue:    &value,
		ThresholdValue: &threshold,
		FiredAt:        time.Date(2026, 5, 20, 12, 34, 0, 0, time.UTC),
		Description:    "error_rate > 20.00",
	}, &OpsAlertDiagnosis{
		RootCause:        "账号/上游「KNA. 成本1.05r/1usd」：二级上游账号池无可用账号",
		Impact:           "告警窗口内错误样本 12 条，主要根因 10 条，集中账号/上游：KNA. 成本1.05r/1usd",
		SampleWindowText: "5m",
		Evidence: []string{
			"10条，二级上游账号池无可用账号，账号/上游=KNA. 成本1.05r/1usd，状态=503，责任=上游/供应商",
			"2条，请求或流式连接中途取消，账号/上游=dragoncode，状态=499，责任=客户端",
		},
		SuggestedAction: "先暂停或降权对应二级中转账号；到对方平台补充/恢复它后面的官方账号池；恢复后再重新启用。",
	})

	require.Contains(t, text, "根因判断：账号/上游「KNA. 成本1.05r/1usd」：二级上游账号池无可用账号")
	require.Contains(t, text, "影响范围：告警窗口内错误样本 12 条")
	require.Contains(t, text, "样本窗口：最近 5m")
	require.Contains(t, text, "根因证据：")
	require.Contains(t, text, "- 10条，二级上游账号池无可用账号")
	require.Contains(t, text, "处理建议：先暂停或降权对应二级中转账号")
}

func TestOpsAlertWebhookNotificationsRespectSeverity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("webhook should not be called for info alert")
	}))
	defer server.Close()

	cfg := &OpsWebhookNotificationConfig{
		Feishu: OpsFeishuNotificationConfig{
			Enabled:     true,
			WebhookURL:  server.URL,
			MinSeverity: "critical",
		},
	}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	repo := &opsWebhookSettingRepoStub{values: map[string]string{
		SettingKeyOpsWebhookNotificationConfig: string(raw),
	}}
	svc := &OpsAlertEvaluatorService{
		opsService:    &OpsService{settingRepo: repo},
		feishuLimiter: newSlidingWindowLimiter(0, time.Hour),
	}

	sent := svc.maybeSendAlertWebhooks(context.Background(), nil, &OpsAlertRule{
		ID:          1,
		Name:        "提示",
		Severity:    "P2",
		NotifyEmail: true,
	}, &OpsAlertEvent{
		ID:      10,
		Status:  OpsAlertStatusFiring,
		FiredAt: time.Now().UTC(),
	})
	require.False(t, sent)
}

func TestSendOpsTelegramTextRejectsMissingChat(t *testing.T) {
	err := sendOpsTelegramText(context.Background(), nil, OpsTelegramNotificationConfig{
		BotToken: "token",
		ChatID:   strings.TrimSpace(""),
	}, "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "chat id")
}

func TestSendOpsFeishuTextWithAppBot(t *testing.T) {
	var tokenHit bool
	var messageHit bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			tokenHit = true
			require.Contains(t, string(body), "cli_test")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token"}`))
		case "/open-apis/im/v1/messages":
			messageHit = true
			require.Equal(t, "Bearer tenant-token", r.Header.Get("Authorization"))
			require.Equal(t, "chat_id", r.URL.Query().Get("receive_id_type"))
			require.Contains(t, string(body), "oc_test")
			require.Contains(t, string(body), "老实人AI")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	oldBaseURL := opsFeishuAPIBaseURL
	opsFeishuAPIBaseURL = server.URL
	defer func() { opsFeishuAPIBaseURL = oldBaseURL }()

	err := sendOpsFeishuText(context.Background(), server.Client(), OpsFeishuNotificationConfig{
		AppID:     "cli_test",
		AppSecret: "app-secret",
		ChatID:    "oc_test",
	}, "老实人AI 测试")
	require.NoError(t, err)
	require.True(t, tokenHit)
	require.True(t, messageHit)
}

func TestValidateOpsWebhookSuccessBodyRejectsFeishuCode(t *testing.T) {
	err := validateOpsWebhookSuccessBody("feishu", `{"code":999,"msg":"bad request"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "999")
}
