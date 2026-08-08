//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupAllowsImageGeneration_GrokMediaGate(t *testing.T) {
	require.True(t, GroupAllowsImageGeneration(nil))
	require.True(t, GroupAllowsImageGeneration(&Group{AllowImageGeneration: true}))
	require.False(t, GroupAllowsImageGeneration(&Group{AllowImageGeneration: false}))
	require.Contains(t, ImageGenerationPermissionMessage(), "Image and video generation")
}
