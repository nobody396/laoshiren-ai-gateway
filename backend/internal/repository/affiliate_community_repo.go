package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type affiliateCommunityRepository struct {
	db *sql.DB
}

func NewAffiliateCommunityRepository(db *sql.DB) service.AffiliateCommunityRepository {
	return &affiliateCommunityRepository{db: db}
}

func (r *affiliateCommunityRepository) GetCommunitySettings(
	ctx context.Context,
) (*service.AffiliateCommunitySettings, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate community repository db is nil")
	}
	return scanAffiliateCommunity(r.db.QueryRowContext(ctx, `
		SELECT
			enabled, title, message,
			qr_object_key, qr_content_type,
			qr_original_filename, qr_size,
			revision, updated_by, created_at, updated_at
		FROM affiliate_community_settings
		WHERE id = 1
	`))
}

func (r *affiliateCommunityRepository) UpdateCommunitySettings(
	ctx context.Context,
	title, message string,
	enabled bool,
	updatedBy int64,
	expectedRevision int64,
) (*service.AffiliateCommunitySettings, error) {
	settings, err := scanAffiliateCommunity(r.db.QueryRowContext(ctx, `
		UPDATE affiliate_community_settings
		SET enabled = $1,
			title = $2,
			message = $3,
			revision = revision + 1,
			updated_by = $4,
			updated_at = NOW()
		WHERE id = 1
			AND revision = $5
		RETURNING
			enabled, title, message,
			qr_object_key, qr_content_type,
			qr_original_filename, qr_size,
			revision, updated_by, created_at, updated_at
	`, enabled, title, message, updatedBy, expectedRevision))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateCommunityRevisionConflict
	}
	return settings, err
}

func (r *affiliateCommunityRepository) UpdateCommunityQRCode(
	ctx context.Context,
	objectKey, contentType, originalName string,
	size int64,
	updatedBy int64,
) (*service.AffiliateCommunitySettings, error) {
	return scanAffiliateCommunity(r.db.QueryRowContext(ctx, `
		UPDATE affiliate_community_settings
		SET qr_object_key = $1,
			qr_content_type = $2,
			qr_original_filename = $3,
			qr_size = $4,
			revision = revision + 1,
			updated_by = $5,
			updated_at = NOW()
		WHERE id = 1
		RETURNING
			enabled, title, message,
			qr_object_key, qr_content_type,
			qr_original_filename, qr_size,
			revision, updated_by, created_at, updated_at
	`, objectKey, contentType, originalName, size, updatedBy))
}

func scanAffiliateCommunity(row *sql.Row) (*service.AffiliateCommunitySettings, error) {
	out := &service.AffiliateCommunitySettings{}
	err := row.Scan(
		&out.Enabled,
		&out.Title,
		&out.Message,
		&out.QRObjectKey,
		&out.QRContentType,
		&out.QROriginalFilename,
		&out.QRSize,
		&out.Revision,
		&out.UpdatedBy,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	return out, err
}
