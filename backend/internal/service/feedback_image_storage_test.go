package service

import (
	"context"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestFeedbackImageAccessTokenIsSigned(t *testing.T) {
	storage := NewFeedbackImageStorage(&config.Config{JWT: config.JWTConfig{Secret: strings.Repeat("a", 32)}})
	accessURL, err := storage.GetAccessURL(context.Background(), "feedbacks/42/screenshot.png")
	require.NoError(t, err)
	token := strings.TrimPrefix(accessURL, "/api/v1/feedback-images/")
	key, err := decodeFeedbackImageToken(token, strings.Repeat("a", 32))
	require.NoError(t, err)
	require.Equal(t, "feedback-images/feedbacks/42/screenshot.png", key)

	tampered := token[:len(token)-1] + "0"
	_, err = decodeFeedbackImageToken(tampered, strings.Repeat("a", 32))
	require.ErrorIs(t, err, ErrFeedbackImageTokenInvalid)
}
