package service

import (
	"bytes"
	"context"
	"io"
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

func TestFeedbackImageLocalFallbackRoundTrip(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	storage := NewFeedbackImageStorage(&config.Config{JWT: config.JWTConfig{Secret: strings.Repeat("b", 32)}})
	png := []byte("\x89PNG\r\n\x1a\nlocal-feedback-image")

	require.True(t, storage.Enabled(context.Background()))
	require.NoError(t, storage.UploadObject(context.Background(), "feedbacks/7/screenshot.png", bytes.NewReader(png), int64(len(png)), "image/png"))
	accessURL, err := storage.GetAccessURL(context.Background(), "feedbacks/7/screenshot.png")
	require.NoError(t, err)
	token := strings.TrimPrefix(accessURL, "/api/v1/feedback-images/")
	access, err := storage.ResolveToken(context.Background(), token, 10)
	require.NoError(t, err)
	require.Empty(t, access.RedirectURL)
	require.Equal(t, "image/png", access.ContentType)
	require.Equal(t, int64(len(png)), access.ContentLength)
	t.Cleanup(func() { _ = access.Reader.Close() })
	readback, err := io.ReadAll(access.Reader)
	require.NoError(t, err)
	require.Equal(t, png, readback)
}
