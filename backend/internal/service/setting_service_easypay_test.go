//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestSettingService_GetEasyPayConfig(t *testing.T) {
	t.Run("returns stored config", func(t *testing.T) {
		repo := &settingPublicRepoStub{values: map[string]string{
			SettingKeyEasyPayEnabled: "true",
			SettingKeyEasyPayPID:     "1001",
			SettingKeyEasyPayKey:     "merchant-key",
			SettingKeyEasyPayAPIBase: "https://pay.example.com",
		}}
		svc := NewSettingService(repo, &config.Config{})

		pid, key, apiBase, enabled, err := svc.GetEasyPayConfig(context.Background())
		require.NoError(t, err)
		require.Equal(t, "1001", pid)
		require.Equal(t, "merchant-key", key)
		require.Equal(t, "https://pay.example.com", apiBase)
		require.True(t, enabled)
	})

	t.Run("defaults to disabled when unset", func(t *testing.T) {
		svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{})

		pid, key, apiBase, enabled, err := svc.GetEasyPayConfig(context.Background())
		require.NoError(t, err)
		require.Empty(t, pid)
		require.Empty(t, key)
		require.Empty(t, apiBase)
		require.False(t, enabled)
	})
}

func TestSettingService_GetTopupProvider(t *testing.T) {
	tests := []struct {
		name     string
		payType  string
		values   map[string]string
		want     string
		wantCode string
	}{
		{name: "alipay defaults to xunhu when unset", payType: "alipay", want: "xunhu"},
		{name: "wechat defaults to xunhu when unset", payType: "wechat", want: "xunhu"},
		{
			name:    "alipay easypay when explicitly set",
			payType: "alipay",
			values:  map[string]string{SettingKeyTopupAlipayProvider: "easypay"},
			want:    "easypay",
		},
		{
			name:    "wechat easypay when explicitly set",
			payType: "wechat",
			values:  map[string]string{SettingKeyTopupWechatProvider: "easypay"},
			want:    "easypay",
		},
		{
			name:    "channels are independent",
			payType: "wechat",
			values:  map[string]string{SettingKeyTopupAlipayProvider: "easypay"},
			want:    "xunhu",
		},
		{
			name:    "unknown value falls back to xunhu",
			payType: "alipay",
			values:  map[string]string{SettingKeyTopupAlipayProvider: "stripe"},
			want:    "xunhu",
		},
		{name: "invalid payType rejected", payType: "unionpay", wantCode: "INVALID_TOPUP_PAY_TYPE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSettingService(&settingPublicRepoStub{values: tt.values}, &config.Config{})

			got, err := svc.GetTopupProvider(context.Background(), tt.payType)
			if tt.wantCode != "" {
				require.Error(t, err)
				require.Equal(t, tt.wantCode, infraerrors.Reason(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSettingService_GetPublicSettings_TopupChannelEnabled(t *testing.T) {
	xunhuAlipayConfigured := map[string]string{
		SettingKeyXunhuAlipayEnabled: "true",
		SettingKeyXunhuAlipayAppID:   "xunhu-appid",
		SettingKeyXunhuAlipayKey:     "xunhu-key",
	}
	easyPayConfigured := map[string]string{
		SettingKeyEasyPayEnabled: "true",
		SettingKeyEasyPayPID:     "1001",
		SettingKeyEasyPayKey:     "merchant-key",
	}

	tests := []struct {
		name       string
		values     map[string]string
		wantAlipay bool
		wantWechat bool
	}{
		{name: "defaults: nothing configured", wantAlipay: false, wantWechat: false},
		{
			name:       "xunhu selected by default and fully configured",
			values:     xunhuAlipayConfigured,
			wantAlipay: true,
		},
		{
			name: "xunhu channel enabled but key missing",
			values: map[string]string{
				SettingKeyXunhuAlipayEnabled: "true",
				SettingKeyXunhuAlipayAppID:   "xunhu-appid",
			},
			wantAlipay: false,
		},
		{
			name: "easypay selected and configured, xunhu off",
			values: map[string]string{
				SettingKeyTopupAlipayProvider: "easypay",
				SettingKeyEasyPayEnabled:      "true",
				SettingKeyEasyPayPID:          "1001",
				SettingKeyEasyPayKey:          "merchant-key",
			},
			wantAlipay: true,
		},
		{
			name: "easypay selected but pid missing, xunhu configured",
			values: map[string]string{
				SettingKeyTopupAlipayProvider: "easypay",
				SettingKeyEasyPayEnabled:      "true",
				SettingKeyEasyPayKey:          "merchant-key",
				SettingKeyXunhuAlipayEnabled:  "true",
				SettingKeyXunhuAlipayAppID:    "xunhu-appid",
				SettingKeyXunhuAlipayKey:      "xunhu-key",
				SettingKeyXunhuWechatEnabled:  "true",
				SettingKeyXunhuWechatAppID:    "xunhu-wechat-appid",
				SettingKeyXunhuWechatKey:      "xunhu-wechat-key",
				SettingKeyTopupWechatProvider: "xunhu",
			},
			wantAlipay: false,
			wantWechat: true,
		},
		{
			name: "easypay shared config gates both channels",
			values: func() map[string]string {
				v := map[string]string{
					SettingKeyTopupAlipayProvider: "easypay",
					SettingKeyTopupWechatProvider: "easypay",
				}
				for k, val := range easyPayConfigured {
					v[k] = val
				}
				return v
			}(),
			wantAlipay: true,
			wantWechat: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSettingService(&settingPublicRepoStub{values: tt.values}, &config.Config{})

			settings, err := svc.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.wantAlipay, settings.TopupAlipayEnabled)
			require.Equal(t, tt.wantWechat, settings.TopupWechatEnabled)
		})
	}
}

func TestSettingService_UpdateSettings_EasyPay(t *testing.T) {
	t.Run("persists easypay settings and providers", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})

		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			EasyPayEnabled:      true,
			EasyPayPID:          " 1001 ",
			EasyPayKey:          "merchant-key",
			EasyPayAPIBase:      " https://pay.example.com/ ",
			TopupAlipayProvider: "easypay",
			TopupWechatProvider: "xunhu",
		})
		require.NoError(t, err)
		require.Equal(t, "true", repo.updates[SettingKeyEasyPayEnabled])
		require.Equal(t, "1001", repo.updates[SettingKeyEasyPayPID])
		require.Equal(t, "merchant-key", repo.updates[SettingKeyEasyPayKey])
		require.Equal(t, "https://pay.example.com/", repo.updates[SettingKeyEasyPayAPIBase])
		require.Equal(t, "easypay", repo.updates[SettingKeyTopupAlipayProvider])
		require.Equal(t, "xunhu", repo.updates[SettingKeyTopupWechatProvider])
	})

	t.Run("empty easypay key keeps persisted secret", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})

		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			EasyPayEnabled: true,
			EasyPayPID:     "1001",
		})
		require.NoError(t, err)
		_, written := repo.updates[SettingKeyEasyPayKey]
		require.False(t, written, "blank easypay key must not overwrite the persisted secret")
	})

	t.Run("empty provider normalizes to xunhu", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})

		err := svc.UpdateSettings(context.Background(), &SystemSettings{})
		require.NoError(t, err)
		require.Equal(t, "xunhu", repo.updates[SettingKeyTopupAlipayProvider])
		require.Equal(t, "xunhu", repo.updates[SettingKeyTopupWechatProvider])
	})

	t.Run("unknown provider rejected", func(t *testing.T) {
		repo := &settingUpdateRepoStub{}
		svc := NewSettingService(repo, &config.Config{})

		err := svc.UpdateSettings(context.Background(), &SystemSettings{
			TopupAlipayProvider: "stripe",
		})
		require.Error(t, err)
		require.Equal(t, "INVALID_TOPUP_PROVIDER", infraerrors.Reason(err))
		require.Nil(t, repo.updates)
	})
}

// settingEasyPayRepoStub supports GetAll for the masked-read test; the shared
// settingPublicRepoStub panics on GetAll.
type settingEasyPayRepoStub struct {
	values map[string]string
}

func (s *settingEasyPayRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingEasyPayRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *settingEasyPayRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingEasyPayRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *settingEasyPayRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingEasyPayRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out, nil
}

func (s *settingEasyPayRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_ParseSettings_EasyPayMaskedRead(t *testing.T) {
	repo := &settingEasyPayRepoStub{values: map[string]string{
		SettingKeyEasyPayEnabled:      "true",
		SettingKeyEasyPayPID:          "1001",
		SettingKeyEasyPayKey:          "merchant-key",
		SettingKeyEasyPayAPIBase:      "https://pay.example.com",
		SettingKeyTopupAlipayProvider: "easypay",
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.EasyPayEnabled)
	require.Equal(t, "1001", settings.EasyPayPID)
	require.Empty(t, settings.EasyPayKey, "masked admin read must never return the easypay key")
	require.True(t, settings.EasyPayKeyConfigured)
	require.Equal(t, "https://pay.example.com", settings.EasyPayAPIBase)
	require.Equal(t, "easypay", settings.TopupAlipayProvider)
	require.Equal(t, "xunhu", settings.TopupWechatProvider)
}
