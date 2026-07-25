package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/announcementread"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type announcementReadRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewAnnouncementReadRepository(client *dbent.Client, db *sql.DB) service.AnnouncementReadRepository {
	return newAnnouncementReadRepositoryWithSQL(client, db)
}

func newAnnouncementReadRepositoryWithSQL(client *dbent.Client, db *sql.DB) *announcementReadRepository {
	return &announcementReadRepository{client: client, db: db}
}

func (r *announcementReadRepository) MarkRead(ctx context.Context, announcementID, userID int64, readAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	return client.AnnouncementRead.Create().
		SetAnnouncementID(announcementID).
		SetUserID(userID).
		SetReadAt(readAt).
		OnConflictColumns(announcementread.FieldAnnouncementID, announcementread.FieldUserID).
		DoNothing().
		Exec(ctx)
}

func (r *announcementReadRepository) GetReadMapByUser(ctx context.Context, userID int64, announcementIDs []int64) (map[int64]time.Time, error) {
	if len(announcementIDs) == 0 {
		return map[int64]time.Time{}, nil
	}

	rows, err := r.client.AnnouncementRead.Query().
		Where(
			announcementread.UserIDEQ(userID),
			announcementread.AnnouncementIDIn(announcementIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[int64]time.Time, len(rows))
	for i := range rows {
		out[rows[i].AnnouncementID] = rows[i].ReadAt
	}
	return out, nil
}

func (r *announcementReadRepository) GetReadMapByUsers(ctx context.Context, announcementID int64, userIDs []int64) (map[int64]time.Time, error) {
	if len(userIDs) == 0 {
		return map[int64]time.Time{}, nil
	}

	rows, err := r.client.AnnouncementRead.Query().
		Where(
			announcementread.AnnouncementIDEQ(announcementID),
			announcementread.UserIDIn(userIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[int64]time.Time, len(rows))
	for i := range rows {
		out[rows[i].UserID] = rows[i].ReadAt
	}
	return out, nil
}

func (r *announcementReadRepository) CountByAnnouncementID(ctx context.Context, announcementID int64) (int64, error) {
	count, err := r.client.AnnouncementRead.Query().
		Where(announcementread.AnnouncementIDEQ(announcementID)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *announcementReadRepository) GetLastPromptedAnnouncementID(ctx context.Context, userID int64) (int64, error) {
	var lastPromptedID int64
	err := r.db.QueryRowContext(ctx, `
		SELECT last_prompted_announcement_id
		FROM user_announcement_states
		WHERE user_id = $1
	`, userID).Scan(&lastPromptedID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return lastPromptedID, nil
}

func (r *announcementReadRepository) MarkPopupBatchPrompted(
	ctx context.Context,
	userID, throughAnnouncementID int64,
	promptedAt time.Time,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_announcement_states (
			user_id,
			last_prompted_announcement_id,
			updated_at
		)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			last_prompted_announcement_id = GREATEST(
				user_announcement_states.last_prompted_announcement_id,
				EXCLUDED.last_prompted_announcement_id
			),
			updated_at = EXCLUDED.updated_at
	`, userID, throughAnnouncementID, promptedAt.UTC())
	return err
}
