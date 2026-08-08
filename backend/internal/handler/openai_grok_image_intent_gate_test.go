package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIResponses_GrokImageIntentPermissionGate(t *testing.T) {
	t.Run("native image tool is rejected", func(t *testing.T) {
		rec := runGrokImagePermissionGateTest(t, false, `{"model":"grok-4.5","tools":[{"type":"image_generation"}],"input":"draw"}`)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Contains(t, rec.Body.String(), service.ImageGenerationPermissionMessage())
	})

	t.Run("passive image namespace does not trigger gate", func(t *testing.T) {
		rec := runGrokImagePermissionGateTest(t, false, `{"model":"grok-4.5","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto","input":"write code"}`)
		require.NotEqual(t, http.StatusForbidden, rec.Code)
		require.NotContains(t, rec.Body.String(), service.ImageGenerationPermissionMessage())
	})

	t.Run("image model through chat is rejected", func(t *testing.T) {
		rec := runGrokImagePermissionGateTest(t, true, `{"model":"grok-imagine-image","messages":[{"role":"user","content":"draw"}]}`)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.Contains(t, rec.Body.String(), service.ImageGenerationPermissionMessage())
	})
}

func TestGrokImages_GroupMediaPermissionGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"grok-imagine-image","prompt":"draw"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(6401)
	userID := int64(6402)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      6403,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformGrok,
			AllowImageGeneration: false,
		},
		User: &service.User{ID: userID, Status: service.StatusActive},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID, Concurrency: 1})

	h := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
	h.GrokImages(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), service.ImageGenerationPermissionMessage())
}

func runGrokImagePermissionGateTest(t *testing.T, chat bool, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	path := "/v1/responses"
	if chat {
		path = "/v1/chat/completions"
	}
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(6301)
	userID := int64(6302)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      6303,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformGrok,
			AllowImageGeneration: false,
		},
		User: &service.User{ID: userID, Status: service.StatusActive},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID, Concurrency: 1})

	h := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
	if chat {
		h.ChatCompletions(c)
	} else {
		h.Responses(c)
	}
	return rec
}
