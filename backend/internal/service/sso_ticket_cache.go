package service

import (
	"context"
	"errors"
	"time"
)

// ErrSSOTicketNotFound is returned when an SSO ticket is missing, expired, or already consumed.
var ErrSSOTicketNotFound = errors.New("sso ticket not found")

// SSOTicketData is the short-lived payload stored behind a one-time SSO ticket.
type SSOTicketData struct {
	Purpose    string    `json:"purpose"`
	UserID     int64     `json:"user_id"`
	APIKeyID   *int64    `json:"api_key_id,omitempty"`
	Audience   string    `json:"audience,omitempty"`
	TargetKind string    `json:"target_kind,omitempty"`
	TargetID   string    `json:"target_id,omitempty"`
	Delivery   string    `json:"delivery,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SSOTicketCache stores and atomically consumes short-lived SSO tickets.
type SSOTicketCache interface {
	StoreSSOTicket(ctx context.Context, ticket string, data SSOTicketData, ttl time.Duration) error
	ConsumeSSOTicket(ctx context.Context, ticket string) (*SSOTicketData, error)
}
