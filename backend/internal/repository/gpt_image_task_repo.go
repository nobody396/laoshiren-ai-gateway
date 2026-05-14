package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
)

type gptImageTaskRepository struct {
	db *sql.DB
}

func NewGPTImageTaskRepository(db *sql.DB) service.GPTImageTaskRepository {
	return &gptImageTaskRepository{db: db}
}

func (r *gptImageTaskRepository) UpsertSubmitted(ctx context.Context, task *service.GPTImageTask) error {
	if task == nil {
		return nil
	}
	token := task.MediaToken
	if token == "" {
		generated, err := generateRepoRandomToken(24)
		if err != nil {
			return err
		}
		token = generated
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO gpt_image_tasks (
			task_id, user_id, api_key_id, account_id, model, upstream_model, resolution, size,
			status, storage_status, billing_status, media_token, image_count,
			inbound_endpoint, upstream_endpoint, user_agent, ip_address, request_payload_hash,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			NOW(), NOW()
		)
		ON CONFLICT (task_id) DO UPDATE SET
			status = EXCLUDED.status,
			updated_at = NOW()
	`, task.TaskID, task.UserID, task.APIKeyID, task.AccountID, task.Model, task.UpstreamModel, task.Resolution, task.Size,
		service.GPTImageTaskStatusSubmitted, service.GPTImageStorageStatusPending, service.GPTImageBillingStatusPending, token, maxInt(task.ImageCount, 1),
		task.InboundEndpoint, task.UpstreamEndpoint, task.UserAgent, task.IPAddress, task.RequestPayloadHash)
	if err != nil {
		return fmt.Errorf("upsert gpt image task: %w", err)
	}
	return nil
}

func (r *gptImageTaskRepository) GetByTaskID(ctx context.Context, taskID string) (*service.GPTImageTask, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, task_id, user_id, api_key_id, account_id, model, upstream_model, resolution, size,
			status, storage_status, billing_status, COALESCE(s3_object_keys, '[]'::jsonb), media_token,
			image_count, inbound_endpoint, upstream_endpoint, user_agent, ip_address, request_payload_hash,
			error_message, billed_at, completed_at, failed_at, created_at, updated_at
		FROM gpt_image_tasks
		WHERE task_id = $1
	`, taskID)
	task, err := scanGPTImageTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return task, nil
}

func (r *gptImageTaskRepository) ListStoredByTaskIDs(ctx context.Context, taskIDs []string) (map[string]*service.GPTImageTask, error) {
	out := make(map[string]*service.GPTImageTask)
	if len(taskIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, task_id, user_id, api_key_id, account_id, model, upstream_model, resolution, size,
			status, storage_status, billing_status, COALESCE(s3_object_keys, '[]'::jsonb), media_token,
			image_count, inbound_endpoint, upstream_endpoint, user_agent, ip_address, request_payload_hash,
			error_message, billed_at, completed_at, failed_at, created_at, updated_at
		FROM gpt_image_tasks
		WHERE task_id = ANY($1)
			AND storage_status = 'stored'
			AND media_token <> ''
	`, pq.Array(taskIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		task, scanErr := scanGPTImageTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out[task.TaskID] = task
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *gptImageTaskRepository) ListPendingSettlement(ctx context.Context, limit int) ([]*service.GPTImageTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, task_id, user_id, api_key_id, account_id, model, upstream_model, resolution, size,
			status, storage_status, billing_status, COALESCE(s3_object_keys, '[]'::jsonb), media_token,
			image_count, inbound_endpoint, upstream_endpoint, user_agent, ip_address, request_payload_hash,
			error_message, billed_at, completed_at, failed_at, created_at, updated_at
		FROM gpt_image_tasks
		WHERE billed_at IS NULL
			AND (billing_status <> 'billing' OR updated_at < NOW() - INTERVAL '10 minutes')
			AND status <> 'failed'
			AND (
				status IN ('submitted', 'pending', 'processing')
				OR storage_status <> 'stored'
				OR billing_status <> 'billed'
			)
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tasks := make([]*service.GPTImageTask, 0)
	for rows.Next() {
		task, scanErr := scanGPTImageTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *gptImageTaskRepository) UpdateUpstreamStatus(ctx context.Context, taskID, status, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE gpt_image_tasks
		SET status = $2,
			error_message = NULLIF($3, ''),
			completed_at = CASE WHEN $2 = 'completed' THEN COALESCE(completed_at, NOW()) ELSE completed_at END,
			failed_at = CASE WHEN $2 = 'failed' THEN COALESCE(failed_at, NOW()) ELSE failed_at END,
			updated_at = NOW()
		WHERE task_id = $1
	`, taskID, status, errorMessage)
	return err
}

func (r *gptImageTaskRepository) TryClaimStorage(ctx context.Context, taskID string) (*service.GPTImageTask, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE gpt_image_tasks
		SET storage_status = 'uploading', updated_at = NOW()
		WHERE task_id = $1
			AND storage_status <> 'stored'
			AND (storage_status <> 'uploading' OR updated_at < NOW() - INTERVAL '10 minutes')
		RETURNING id, task_id, user_id, api_key_id, account_id, model, upstream_model, resolution, size,
			status, storage_status, billing_status, COALESCE(s3_object_keys, '[]'::jsonb), media_token,
			image_count, inbound_endpoint, upstream_endpoint, user_agent, ip_address, request_payload_hash,
			error_message, billed_at, completed_at, failed_at, created_at, updated_at
	`, taskID)
	task, err := scanGPTImageTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return task, true, nil
}

func (r *gptImageTaskRepository) MarkStorageStored(ctx context.Context, taskID string, objectKeys []string, imageCount int, mediaToken string) error {
	if mediaToken == "" {
		var err error
		mediaToken, err = generateRepoRandomToken(24)
		if err != nil {
			return err
		}
	}
	keysJSON, err := json.Marshal(objectKeys)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE gpt_image_tasks
		SET status = 'completed',
			storage_status = 'stored',
			s3_object_keys = $2::jsonb,
			image_count = $3,
			media_token = $4,
			completed_at = COALESCE(completed_at, NOW()),
			updated_at = NOW()
		WHERE task_id = $1
	`, taskID, string(keysJSON), imageCount, mediaToken)
	return err
}

func (r *gptImageTaskRepository) MarkStorageFailed(ctx context.Context, taskID, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE gpt_image_tasks
		SET storage_status = 'failed',
			error_message = NULLIF($2, ''),
			updated_at = NOW()
		WHERE task_id = $1
	`, taskID, errorMessage)
	return err
}

func (r *gptImageTaskRepository) TryClaimBilling(ctx context.Context, taskID string) (*service.GPTImageTask, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE gpt_image_tasks
		SET billing_status = 'billing', updated_at = NOW()
		WHERE task_id = $1
			AND billed_at IS NULL
			AND (billing_status <> 'billing' OR updated_at < NOW() - INTERVAL '10 minutes')
		RETURNING id, task_id, user_id, api_key_id, account_id, model, upstream_model, resolution, size,
			status, storage_status, billing_status, COALESCE(s3_object_keys, '[]'::jsonb), media_token,
			image_count, inbound_endpoint, upstream_endpoint, user_agent, ip_address, request_payload_hash,
			error_message, billed_at, completed_at, failed_at, created_at, updated_at
	`, taskID)
	task, err := scanGPTImageTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return task, true, nil
}

func (r *gptImageTaskRepository) MarkBilled(ctx context.Context, taskID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE gpt_image_tasks
		SET billing_status = 'billed',
			billed_at = COALESCE(billed_at, NOW()),
			updated_at = NOW()
		WHERE task_id = $1
	`, taskID)
	return err
}

func (r *gptImageTaskRepository) ReleaseBillingClaim(ctx context.Context, taskID string, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE gpt_image_tasks
		SET billing_status = 'failed',
			error_message = NULLIF($2, ''),
			updated_at = NOW()
		WHERE task_id = $1 AND billed_at IS NULL
	`, taskID, errorMessage)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanGPTImageTask(row rowScanner) (*service.GPTImageTask, error) {
	var task service.GPTImageTask
	var keysRaw []byte
	var errMsg sql.NullString
	var billedAt, completedAt, failedAt sql.NullTime
	err := row.Scan(
		&task.ID, &task.TaskID, &task.UserID, &task.APIKeyID, &task.AccountID, &task.Model, &task.UpstreamModel, &task.Resolution, &task.Size,
		&task.Status, &task.StorageStatus, &task.BillingStatus, &keysRaw, &task.MediaToken,
		&task.ImageCount, &task.InboundEndpoint, &task.UpstreamEndpoint, &task.UserAgent, &task.IPAddress, &task.RequestPayloadHash,
		&errMsg, &billedAt, &completedAt, &failedAt, &task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(keysRaw) > 0 {
		_ = json.Unmarshal(keysRaw, &task.S3ObjectKeys)
	}
	if errMsg.Valid {
		task.ErrorMessage = errMsg.String
	}
	if billedAt.Valid {
		task.BilledAt = &billedAt.Time
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	if failedAt.Valid {
		task.FailedAt = &failedAt.Time
	}
	return &task, nil
}

func generateRepoRandomToken(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
