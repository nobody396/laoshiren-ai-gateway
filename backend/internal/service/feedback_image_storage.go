package service

import (
	"context"
	"fmt"
	"io"
)

// DisabledFeedbackImageStorage keeps the feedback upload flow explicit until
// this project grows a dedicated object-storage configuration surface.
type DisabledFeedbackImageStorage struct{}

func NewFeedbackImageStorage() *DisabledFeedbackImageStorage {
	return &DisabledFeedbackImageStorage{}
}

func (s *DisabledFeedbackImageStorage) Enabled(context.Context) bool {
	return false
}

func (s *DisabledFeedbackImageStorage) UploadObject(context.Context, string, io.Reader, int64, string) error {
	return fmt.Errorf("feedback image storage is not configured")
}

func (s *DisabledFeedbackImageStorage) GetAccessURL(context.Context, string) (string, error) {
	return "", fmt.Errorf("feedback image storage is not configured")
}
