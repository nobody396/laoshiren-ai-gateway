package service

import (
	"context"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type missingWebSearchSettingRepo struct{}

func (*missingWebSearchSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (*missingWebSearchSettingRepo) GetValue(context.Context, string) (string, error) {
	return "", ErrSettingNotFound
}
func (*missingWebSearchSettingRepo) Set(context.Context, string, string) error { return nil }
func (*missingWebSearchSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (*missingWebSearchSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (*missingWebSearchSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}
func (*missingWebSearchSettingRepo) Delete(context.Context, string) error { return nil }

func TestGetWebSearchEmulationConfig_MissingSettingUsesDisabledDefault(t *testing.T) {
	webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{expiresAt: 0})
	t.Cleanup(func() {
		webSearchEmulationCache.Store(&cachedWebSearchEmulationConfig{expiresAt: 0})
	})
	svc := NewSettingService(&missingWebSearchSettingRepo{}, &config.Config{})

	cfg, err := svc.GetWebSearchEmulationConfig(context.Background())

	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.Empty(t, cfg.Providers)
}
