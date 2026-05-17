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
	require.Empty(t, updated.Feishu.WebhookURL)
	require.Empty(t, updated.Feishu.Secret)
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
