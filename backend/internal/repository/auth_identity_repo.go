package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type authIdentityRepository struct {
	db *sql.DB
}

func NewAuthIdentityRepository(sqlDB *sql.DB) service.AuthIdentityRepository {
	return &authIdentityRepository{db: sqlDB}
}

func (r *authIdentityRepository) ListByUserID(ctx context.Context, userID int64) ([]*service.AuthIdentity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, provider, provider_user_id, email, email_verified, display_name, avatar_url,
		       raw_profile, last_login_at, bound_at, created_at, updated_at
		FROM auth_identities
		WHERE user_id = $1
		ORDER BY provider ASC, id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*service.AuthIdentity
	for rows.Next() {
		item, err := scanAuthIdentity(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *authIdentityRepository) GetByProviderAccount(ctx context.Context, provider, providerUserID string) (*service.AuthIdentity, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, provider, provider_user_id, email, email_verified, display_name, avatar_url,
		       raw_profile, last_login_at, bound_at, created_at, updated_at
		FROM auth_identities
		WHERE provider = $1 AND provider_user_id = $2
	`, strings.TrimSpace(provider), strings.TrimSpace(providerUserID))
	item, err := scanAuthIdentity(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAuthIdentityNotFound
	}
	return item, err
}

func (r *authIdentityRepository) Upsert(ctx context.Context, input service.AuthIdentityUpsertInput) (*service.AuthIdentity, error) {
	provider := strings.TrimSpace(input.Provider)
	providerUserID := strings.TrimSpace(input.ProviderUserID)
	if input.UserID <= 0 || provider == "" || providerUserID == "" {
		return nil, errors.New("invalid auth identity upsert input")
	}

	existing, err := r.GetByProviderAccount(ctx, provider, providerUserID)
	if err != nil && !errors.Is(err, service.ErrAuthIdentityNotFound) {
		return nil, err
	}
	if existing != nil && existing.UserID != input.UserID {
		return nil, service.ErrAuthIdentityConflict
	}

	rawProfile, err := marshalJSONMap(input.RawProfile)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		row := r.db.QueryRowContext(ctx, `
			INSERT INTO auth_identities (
				user_id, provider, provider_user_id, email, email_verified,
				display_name, avatar_url, raw_profile, last_login_at, bound_at, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW(), NOW())
			RETURNING id, user_id, provider, provider_user_id, email, email_verified, display_name, avatar_url,
			          raw_profile, last_login_at, bound_at, created_at, updated_at
		`, input.UserID, provider, providerUserID, strings.TrimSpace(input.Email), input.EmailVerified,
			strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.AvatarURL), rawProfile, input.LastLoginAt)
		item, insertErr := scanAuthIdentity(row)
		if insertErr == nil {
			return item, nil
		}
		if existingAfterConflict, lookupErr := r.GetByProviderAccount(ctx, provider, providerUserID); lookupErr == nil {
			if existingAfterConflict.UserID != input.UserID {
				return nil, service.ErrAuthIdentityConflict
			}
			existing = existingAfterConflict
		} else {
			return nil, insertErr
		}
	}

	row := r.db.QueryRowContext(ctx, `
		UPDATE auth_identities
		SET email = $4,
		    email_verified = $5,
		    display_name = $6,
		    avatar_url = $7,
		    raw_profile = $8,
		    last_login_at = $9,
		    updated_at = NOW()
		WHERE user_id = $1 AND provider = $2 AND provider_user_id = $3
		RETURNING id, user_id, provider, provider_user_id, email, email_verified, display_name, avatar_url,
		          raw_profile, last_login_at, bound_at, created_at, updated_at
	`, input.UserID, provider, providerUserID, strings.TrimSpace(input.Email), input.EmailVerified,
		strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.AvatarURL), rawProfile, input.LastLoginAt)
	return scanAuthIdentity(row)
}

func (r *authIdentityRepository) DeleteByUserProvider(ctx context.Context, userID int64, provider string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM auth_identities
		WHERE user_id = $1 AND provider = $2
	`, userID, strings.TrimSpace(provider))
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return service.ErrAuthIdentityNotFound
	}
	return nil
}

type authIdentityScanner interface {
	Scan(dest ...any) error
}

func scanAuthIdentity(row authIdentityScanner) (*service.AuthIdentity, error) {
	var rawProfile []byte
	item := &service.AuthIdentity{}
	if err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Provider,
		&item.ProviderUserID,
		&item.Email,
		&item.EmailVerified,
		&item.DisplayName,
		&item.AvatarURL,
		&rawProfile,
		&item.LastLoginAt,
		&item.BoundAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	profile, err := unmarshalJSONMap(rawProfile)
	if err != nil {
		return nil, err
	}
	item.RawProfile = profile
	return item, nil
}

func marshalJSONMap(input map[string]any) ([]byte, error) {
	if len(input) == 0 {
		return []byte(`{}`), nil
	}
	return json.Marshal(input)
}

func unmarshalJSONMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
