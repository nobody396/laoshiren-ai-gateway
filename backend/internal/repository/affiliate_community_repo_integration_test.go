//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateCommunityRepository_ConfiguresPrivateAgentCard(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateCommunityRepository(integrationDB)
	communityService := service.NewAffiliateCommunityService(repo)
	admin := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("community-admin-%d@example.com", time.Now().UnixNano()),
		Role:  service.RoleAdmin,
	})

	initial, err := communityService.Get(ctx, false)
	require.NoError(t, err)
	require.False(t, initial.Enabled)

	withQR, err := repo.UpdateCommunityQRCode(
		ctx,
		"affiliate-community/integration.png",
		"image/png",
		"integration.png",
		256,
		admin.ID,
	)
	require.NoError(t, err)
	require.True(t, withQR.HasQRCode || withQR.QRObjectKey != "")

	enabled, err := communityService.Update(
		ctx,
		"代理商内测群",
		"扫码进群，获取运营支持。",
		true,
		admin.ID,
		withQR.Revision,
	)
	require.NoError(t, err)
	require.True(t, enabled.Enabled)
	require.True(t, enabled.HasQRCode)

	agentView, err := communityService.Get(ctx, true)
	require.NoError(t, err)
	require.Equal(t, "代理商内测群", agentView.Title)

	_, err = communityService.Update(
		ctx,
		"过期写入",
		"不应成功",
		false,
		admin.ID,
		withQR.Revision,
	)
	require.True(t, errors.Is(err, service.ErrAffiliateCommunityRevisionConflict), "unexpected error: %v", err)

	_, err = repo.UpdateCommunitySettings(
		ctx,
		enabled.Title,
		enabled.Message,
		false,
		admin.ID,
		enabled.Revision,
	)
	require.NoError(t, err)
	hidden, err := communityService.Get(ctx, true)
	require.NoError(t, err)
	require.False(t, hidden.Enabled)
	require.Empty(t, hidden.Title)
	require.False(t, hidden.HasQRCode)
}
