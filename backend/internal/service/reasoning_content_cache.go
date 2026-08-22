package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	ReasoningContentDefaultTTL = 24 * time.Hour
	ReasoningContentMaxBytes   = 256 * 1024
)

var (
	ErrReasoningContentNotFound = errors.New("reasoning content not found")
	ErrReasoningContentTooLarge = errors.New("reasoning content exceeds cache limit")
)

// ReasoningCacheScope prevents an opaque Responses item id from becoming a
// cross-tenant lookup key. No credential material is stored in this scope.
type ReasoningCacheScope struct {
	UserID   int64
	APIKeyID int64
	Model    string
}

func (s ReasoningCacheScope) Valid() bool {
	return s.UserID > 0 && s.APIKeyID > 0 && strings.TrimSpace(s.Model) != ""
}

type ReasoningContentCache interface {
	SetReasoningContent(ctx context.Context, scope ReasoningCacheScope, itemID, content string, ttl time.Duration) error
	GetReasoningContent(ctx context.Context, scope ReasoningCacheScope, itemID string) (string, error)
}
