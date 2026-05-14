package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type pendingAuthSessionRepository struct {
	db *sql.DB
}

func NewPendingAuthSessionRepository(sqlDB *sql.DB) service.PendingAuthSessionRepository {
	return &pendingAuthSessionRepository{db: sqlDB}
}

func (r *pendingAuthSessionRepository) Create(ctx context.Context, input service.PendingAuthSessionCreateInput) (*service.PendingAuthSession, error) {
	claimsSnapshot, err := marshalJSONMap(input.ClaimsSnapshot)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO pending_auth_sessions (
			state, provider, intended_action, claims_snapshot, redirect_uri, user_id, expires_at, ip, user_agent
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, state, provider, provider_user_id, intended_action, claims_snapshot, redirect_uri,
		          user_id, expires_at, consumed_at, created_at, ip, user_agent
	`, strings.TrimSpace(input.State), strings.TrimSpace(input.Provider), strings.TrimSpace(input.IntendedAction),
		claimsSnapshot, strings.TrimSpace(input.RedirectURI), input.UserID, input.ExpiresAt.UTC(), strings.TrimSpace(input.IPAddress), strings.TrimSpace(input.UserAgent))
	return scanPendingAuthSession(row)
}

func (r *pendingAuthSessionRepository) GetByState(ctx context.Context, state string) (*service.PendingAuthSession, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, state, provider, provider_user_id, intended_action, claims_snapshot, redirect_uri,
		       user_id, expires_at, consumed_at, created_at, ip, user_agent
		FROM pending_auth_sessions
		WHERE state = $1
	`, strings.TrimSpace(state))
	item, err := scanPendingAuthSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPendingAuthSessionNotFound
	}
	return item, err
}

func (r *pendingAuthSessionRepository) Resolve(ctx context.Context, state string, input service.PendingAuthSessionResolveInput) (*service.PendingAuthSession, error) {
	claimsSnapshot, err := marshalJSONMap(input.ClaimsSnapshot)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE pending_auth_sessions
		SET provider_user_id = $2,
		    claims_snapshot = $3
		WHERE state = $1 AND consumed_at IS NULL
		RETURNING id, state, provider, provider_user_id, intended_action, claims_snapshot, redirect_uri,
		          user_id, expires_at, consumed_at, created_at, ip, user_agent
	`, strings.TrimSpace(state), strings.TrimSpace(input.ProviderUserID), claimsSnapshot)
	item, scanErr := scanPendingAuthSession(row)
	if errors.Is(scanErr, sql.ErrNoRows) {
		return nil, service.ErrPendingAuthSessionNotFound
	}
	return item, scanErr
}

func (r *pendingAuthSessionRepository) MarkConsumed(ctx context.Context, state string, consumedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE pending_auth_sessions
		SET consumed_at = $2
		WHERE state = $1 AND consumed_at IS NULL
	`, strings.TrimSpace(state), consumedAt.UTC())
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return service.ErrPendingAuthSessionConsumed
	}
	return nil
}

func (r *pendingAuthSessionRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM pending_auth_sessions
		WHERE expires_at < $1
	`, before.UTC())
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

type pendingAuthSessionScanner interface {
	Scan(dest ...any) error
}

func scanPendingAuthSession(row pendingAuthSessionScanner) (*service.PendingAuthSession, error) {
	var rawClaims []byte
	item := &service.PendingAuthSession{}
	if err := row.Scan(
		&item.ID,
		&item.State,
		&item.Provider,
		&item.ProviderUserID,
		&item.IntendedAction,
		&rawClaims,
		&item.RedirectURI,
		&item.UserID,
		&item.ExpiresAt,
		&item.ConsumedAt,
		&item.CreatedAt,
		&item.IPAddress,
		&item.UserAgent,
	); err != nil {
		return nil, err
	}
	claims, err := unmarshalJSONMap(rawClaims)
	if err != nil {
		return nil, err
	}
	item.ClaimsSnapshot = claims
	return item, nil
}
