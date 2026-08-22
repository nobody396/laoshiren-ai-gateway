//go:build unit

package dto

import (
	"encoding/json"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUniversalRoutesAreAdminOnly(t *testing.T) {
	group := &service.Group{ID: 90, Platform: service.PlatformUniversal, UniversalRoutes: []service.UniversalRouteConfig{{
		PublicModel: "gpt-5.6-sol", TargetGroupID: 6, Enabled: true,
	}}}
	publicJSON, err := json.Marshal(GroupFromServiceShallow(group))
	require.NoError(t, err)
	require.NotContains(t, string(publicJSON), "target_group_id")

	adminJSON, err := json.Marshal(GroupFromServiceAdmin(group))
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), "target_group_id")
}
