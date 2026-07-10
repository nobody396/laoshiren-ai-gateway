package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type accountingCommandRepository struct {
	db *sql.DB
}

func NewAccountingCommandRepository(db *sql.DB) service.AccountingCommandRepository {
	return &accountingCommandRepository{db: db}
}

const accountingCommandColumns = `id, request_id, api_key_id, usage_log_id, version, payload,
	status, attempts, available_at, lease_owner, lease_expires_at,
	last_error_code, last_error_message, created_at, updated_at, completed_at`

const accountingCommandClaimColumns = `c.id, c.request_id, c.api_key_id, c.usage_log_id, c.version, c.payload,
	c.status, c.attempts, c.available_at, c.lease_owner, c.lease_expires_at,
	c.last_error_code, c.last_error_message, c.created_at, c.updated_at, c.completed_at`

func (r *accountingCommandRepository) Enqueue(ctx context.Context, usageLogID int64, cmd *service.UsageBillingCommand) (*service.AccountingCommand, bool, error) {
	exec := sqlExecutorFromContext(ctx, r.db)
	return enqueueAccountingCommand(ctx, exec, usageLogID, cmd)
}

func enqueueAccountingCommand(ctx context.Context, exec sqlExecutor, usageLogID int64, cmd *service.UsageBillingCommand) (*service.AccountingCommand, bool, error) {
	if cmd == nil {
		return nil, false, errors.New("accounting command is required")
	}
	if exec == nil {
		return nil, false, errors.New("accounting command executor is not configured")
	}
	cmd.Normalize()
	if cmd.RequestID == "" || cmd.APIKeyID <= 0 || usageLogID <= 0 {
		return nil, false, errors.New("accounting command request_id, api_key_id and usage_log_id are required")
	}
	cmd.UsageLogID = usageLogID
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, false, err
	}

	var id int64
	err = scanSingleRow(ctx, exec, `
		INSERT INTO usage_accounting_commands (
			request_id, api_key_id, usage_log_id, version, payload, status, available_at
		) VALUES ($1, $2, $3, $4, $5, 'pending', NOW())
		ON CONFLICT DO NOTHING
		RETURNING id
	`, []any{cmd.RequestID, cmd.APIKeyID, usageLogID, service.AccountingCommandVersion, payload}, &id)
	inserted := true
	if errors.Is(err, sql.ErrNoRows) {
		inserted = false
		err = scanSingleRow(ctx, exec, `
			SELECT id FROM usage_accounting_commands
			WHERE (request_id = $1 AND api_key_id = $2) OR usage_log_id = $3
			ORDER BY id LIMIT 1
		`, []any{cmd.RequestID, cmd.APIKeyID, usageLogID}, &id)
	}
	if err != nil {
		return nil, false, err
	}
	command, err := getAccountingCommandByID(ctx, exec, id)
	if err == nil && command != nil && command.Payload.RequestFingerprint != cmd.RequestFingerprint {
		return nil, false, service.ErrUsageBillingRequestConflict
	}
	return command, inserted, err
}

func (r *accountingCommandRepository) ClaimDue(ctx context.Context, workerID string, lease time.Duration, limit int) ([]service.AccountingCommand, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("accounting command database is not configured")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, errors.New("accounting worker id is required")
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	claimQuery := `
		WITH due AS (
			SELECT id
			FROM usage_accounting_commands
			WHERE (status = 'pending' AND available_at <= NOW())
			   OR (status = 'processing' AND lease_expires_at <= NOW())
			ORDER BY available_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE usage_accounting_commands AS c
		SET status = 'processing',
			attempts = c.attempts + 1,
			lease_owner = $2,
			lease_expires_at = NOW() + make_interval(secs => $3),
			updated_at = NOW()
		FROM due
		WHERE c.id = due.id
		RETURNING ` + accountingCommandClaimColumns
	rows, err := tx.QueryContext(ctx, claimQuery, limit, workerID, lease.Seconds())
	if err != nil {
		return nil, err
	}
	commands, err := scanAccountingCommands(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return commands, nil
}

func (r *accountingCommandRepository) Complete(ctx context.Context, id int64, workerID string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE usage_accounting_commands
		SET status = 'completed', completed_at = NOW(), updated_at = NOW(),
			lease_owner = NULL, lease_expires_at = NULL,
			last_error_code = NULL, last_error_message = NULL
		WHERE id = $1 AND status = 'processing' AND lease_owner = $2
	`, id, workerID)
	return requireAccountingCommandMutation(result, err)
}

func (r *accountingCommandRepository) Fail(ctx context.Context, id int64, workerID, code, message string, retryAt time.Time, dead bool) error {
	status := service.AccountingCommandStatusPending
	if dead {
		status = service.AccountingCommandStatusDead
	}
	code = truncateAccountingError(code, 64)
	message = truncateAccountingError(message, 512)
	result, err := r.db.ExecContext(ctx, `
		UPDATE usage_accounting_commands
		SET status = $3, available_at = $4, updated_at = NOW(),
			lease_owner = NULL, lease_expires_at = NULL,
			last_error_code = NULLIF($5, ''), last_error_message = NULLIF($6, '')
		WHERE id = $1 AND status = 'processing' AND lease_owner = $2
	`, id, workerID, status, retryAt, code, message)
	return requireAccountingCommandMutation(result, err)
}

func (r *accountingCommandRepository) Stats(ctx context.Context) (*service.AccountingCommandStats, error) {
	stats := &service.AccountingCommandStats{}
	var oldestSeconds float64
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status IN ('pending', 'processing')),
			COUNT(*) FILTER (WHERE status = 'dead'),
			COUNT(*) FILTER (WHERE attempts > 1 AND status <> 'completed'),
			COALESCE(EXTRACT(EPOCH FROM (NOW() - MIN(created_at) FILTER (WHERE status IN ('pending', 'processing')))), 0)
		FROM usage_accounting_commands
	`).Scan(&stats.PendingCount, &stats.DeadCount, &stats.RetryCount, &oldestSeconds)
	if err != nil {
		return nil, err
	}
	stats.OldestAge = time.Duration(oldestSeconds * float64(time.Second))
	return stats, nil
}

func (r *accountingCommandRepository) ReplayDead(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE usage_accounting_commands
		SET status = 'pending', available_at = NOW(), updated_at = NOW(),
			lease_owner = NULL, lease_expires_at = NULL
		WHERE id = $1 AND status = 'dead'
	`, id)
	return requireAccountingCommandMutation(result, err)
}

func (r *accountingCommandRepository) GetByUsageLogID(ctx context.Context, usageLogID int64) (*service.AccountingCommand, error) {
	return getAccountingCommand(ctx, r.db, `SELECT `+accountingCommandColumns+` FROM usage_accounting_commands WHERE usage_log_id = $1`, usageLogID)
}

func getAccountingCommandByID(ctx context.Context, exec sqlExecutor, id int64) (*service.AccountingCommand, error) {
	return getAccountingCommand(ctx, exec, `SELECT `+accountingCommandColumns+` FROM usage_accounting_commands WHERE id = $1`, id)
}

func getAccountingCommand(ctx context.Context, exec sqlExecutor, query string, arg any) (*service.AccountingCommand, error) {
	rows, err := exec.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	commands, err := scanAccountingCommands(rows)
	if err != nil {
		return nil, err
	}
	if len(commands) == 0 {
		return nil, sql.ErrNoRows
	}
	return &commands[0], nil
}

func scanAccountingCommands(rows *sql.Rows) ([]service.AccountingCommand, error) {
	defer func() { _ = rows.Close() }()
	commands := make([]service.AccountingCommand, 0)
	for rows.Next() {
		var command service.AccountingCommand
		var payload []byte
		var leaseOwner, errorCode, errorMessage sql.NullString
		var leaseExpires, completedAt sql.NullTime
		if err := rows.Scan(
			&command.ID, &command.RequestID, &command.APIKeyID, &command.UsageLogID,
			&command.Version, &payload, &command.Status, &command.Attempts,
			&command.AvailableAt, &leaseOwner, &leaseExpires, &errorCode, &errorMessage,
			&command.CreatedAt, &command.UpdatedAt, &completedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &command.Payload); err != nil {
			return nil, err
		}
		if leaseOwner.Valid {
			command.LeaseOwner = &leaseOwner.String
		}
		if leaseExpires.Valid {
			command.LeaseExpiresAt = &leaseExpires.Time
		}
		if errorCode.Valid {
			command.LastErrorCode = &errorCode.String
		}
		if errorMessage.Valid {
			command.LastErrorMessage = &errorMessage.String
		}
		if completedAt.Valid {
			command.CompletedAt = &completedAt.Time
		}
		commands = append(commands, command)
	}
	return commands, rows.Err()
}

func requireAccountingCommandMutation(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("accounting command lease/state changed concurrently")
	}
	return nil
}

func truncateAccountingError(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > limit {
		value = value[:limit]
	}
	return value
}
