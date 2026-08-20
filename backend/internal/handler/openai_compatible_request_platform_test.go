package handler

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAICompatibleRequestPlatform(t *testing.T) {
	t.Parallel()

	require.Equal(t, service.PlatformOpenAI, openAICompatibleRequestPlatform(nil))
	require.Equal(t, service.PlatformOpenAI, openAICompatibleRequestPlatform(&service.APIKey{}))
	require.Equal(t, service.PlatformOpenAI, openAICompatibleRequestPlatform(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformOpenAI},
	}))
	require.Equal(t, service.PlatformOpenAI, openAICompatibleRequestPlatform(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformAnthropic},
	}))
	require.Equal(t, service.PlatformGrok, openAICompatibleRequestPlatform(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformGrok},
	}))
	require.Equal(t, service.PlatformGemini, openAICompatibleRequestPlatform(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformGemini},
	}))
}
