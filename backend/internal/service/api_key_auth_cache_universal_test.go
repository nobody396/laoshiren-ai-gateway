//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthCachePreservesUniversalRoutes(t *testing.T) {
	group := &Group{ID: 90, Name: "Universal", Platform: PlatformUniversal, Status: StatusActive, Hydrated: true, UniversalRoutes: []UniversalRouteConfig{{
		PublicModel: "gpt-5.6-sol", MatchType: UniversalRouteMatchExact, InboundProtocol: APIProtocolResponses, TargetGroupID: 6, Priority: 10, Enabled: true,
	}}}
	user := &User{ID: 7, Status: StatusActive}
	key := &APIKey{ID: 8, Key: "secret-not-serialized", Status: StatusActive, User: user, Group: group}
	key.GroupID = &group.ID
	service := &APIKeyService{}

	snapshot := service.snapshotFromAPIKey(key)
	restored := service.snapshotToAPIKey("restored-key", snapshot)
	require.NotNil(t, restored)
	require.NotNil(t, restored.Group)
	require.Equal(t, group.UniversalRoutes, restored.Group.UniversalRoutes)
}
