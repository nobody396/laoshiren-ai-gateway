package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type openAIRouteEvidenceEpochRepository struct {
	db *sql.DB
}

func newOpenAIRouteEvidenceEpochRepository(db *sql.DB) *openAIRouteEvidenceEpochRepository {
	return &openAIRouteEvidenceEpochRepository{db: db}
}

func (r *openAIRouteEvidenceEpochRepository) BeginOpenAIRouteEvidenceEpoch(
	ctx context.Context,
	epoch service.OpenAIRouteEvidenceEpoch,
) error {
	if r == nil || r.db == nil {
		return errors.New("nil OpenAI route evidence epoch repository")
	}
	if err := validateOpenAIRouteEvidenceEpoch(epoch); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
INSERT INTO openai_route_evidence_epochs (
  epoch_id, instance_id, component, started_at, heartbeat_at
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (epoch_id) DO NOTHING
`, epoch.EpochID, epoch.InstanceID, epoch.Component, epoch.StartedAt.UTC(), epoch.HeartbeatAt.UTC())
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	var instanceID, component string
	var startedAt time.Time
	if err := r.db.QueryRowContext(ctx, `
SELECT instance_id, component, started_at
FROM openai_route_evidence_epochs
WHERE epoch_id = $1
`, epoch.EpochID).Scan(&instanceID, &component, &startedAt); err != nil {
		return err
	}
	if instanceID != epoch.InstanceID || component != epoch.Component || !startedAt.UTC().Equal(epoch.StartedAt.UTC()) {
		return errors.New("OpenAI route evidence epoch identity mismatch")
	}
	return nil
}

func (r *openAIRouteEvidenceEpochRepository) CheckpointOpenAIRouteEvidenceEpoch(
	ctx context.Context,
	epoch service.OpenAIRouteEvidenceEpoch,
) error {
	if r == nil || r.db == nil {
		return errors.New("nil OpenAI route evidence epoch repository")
	}
	if err := validateOpenAIRouteEvidenceEpoch(epoch); err != nil {
		return err
	}
	var stoppedAt any
	if !epoch.StoppedAt.IsZero() {
		stoppedAt = epoch.StoppedAt.UTC()
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE openai_route_evidence_epochs SET
  heartbeat_at = GREATEST(heartbeat_at, $2),
  stopped_at = CASE
    WHEN $3::boolean THEN COALESCE(stopped_at, $4::timestamptz)
    ELSE stopped_at
  END,
  clean_shutdown = clean_shutdown OR $3,
  attempted = GREATEST(attempted, $5),
  written = GREATEST(written, $6),
  failed = GREATEST(failed, $7),
  dropped = GREATEST(dropped, $8),
  rejected = GREATEST(rejected, $9),
  outcome_expected = GREATEST(outcome_expected, $10),
  outcome_applied = GREATEST(outcome_applied, $11),
  outcome_failed = GREATEST(outcome_failed, $12),
  storage_checks = GREATEST(storage_checks, $13),
  storage_failed = GREATEST(storage_failed, $14),
  last_error = LEFT($15, 512),
  updated_at = NOW()
WHERE epoch_id = $1
  AND instance_id = $16
  AND component = $17
  AND started_at = $18
`,
		epoch.EpochID,
		epoch.HeartbeatAt.UTC(),
		epoch.CleanShutdown,
		stoppedAt,
		epoch.Counters.Attempted,
		epoch.Counters.Written,
		epoch.Counters.Failed,
		epoch.Counters.Dropped,
		epoch.Counters.Rejected,
		epoch.Counters.OutcomeExpected,
		epoch.Counters.OutcomeApplied,
		epoch.Counters.OutcomeFailed,
		epoch.Counters.StorageChecks,
		epoch.Counters.StorageFailed,
		strings.TrimSpace(epoch.Counters.LastError),
		epoch.InstanceID,
		epoch.Component,
		epoch.StartedAt.UTC(),
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("OpenAI route evidence epoch checkpoint identity mismatch")
	}
	return nil
}

func (r *openAIRouteEvidenceEpochRepository) ListOpenAIRouteEvidenceEpochs(
	ctx context.Context,
	start time.Time,
	end time.Time,
) ([]service.OpenAIRouteEvidenceEpoch, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil OpenAI route evidence epoch repository")
	}
	start = start.UTC()
	end = end.UTC()
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return nil, errors.New("invalid OpenAI route evidence window")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT
  epoch_id, instance_id, component, started_at, heartbeat_at, stopped_at, clean_shutdown,
  attempted, written, failed, dropped, rejected,
  outcome_expected, outcome_applied, outcome_failed,
  storage_checks, storage_failed, last_error
FROM openai_route_evidence_epochs
WHERE started_at < $2
  AND GREATEST(heartbeat_at, COALESCE(stopped_at, heartbeat_at)) > $1
ORDER BY started_at ASC, epoch_id ASC
`, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	epochs := make([]service.OpenAIRouteEvidenceEpoch, 0)
	for rows.Next() {
		var epoch service.OpenAIRouteEvidenceEpoch
		var stoppedAt sql.NullTime
		if err := rows.Scan(
			&epoch.EpochID,
			&epoch.InstanceID,
			&epoch.Component,
			&epoch.StartedAt,
			&epoch.HeartbeatAt,
			&stoppedAt,
			&epoch.CleanShutdown,
			&epoch.Counters.Attempted,
			&epoch.Counters.Written,
			&epoch.Counters.Failed,
			&epoch.Counters.Dropped,
			&epoch.Counters.Rejected,
			&epoch.Counters.OutcomeExpected,
			&epoch.Counters.OutcomeApplied,
			&epoch.Counters.OutcomeFailed,
			&epoch.Counters.StorageChecks,
			&epoch.Counters.StorageFailed,
			&epoch.Counters.LastError,
		); err != nil {
			return nil, err
		}
		if stoppedAt.Valid {
			epoch.StoppedAt = stoppedAt.Time.UTC()
		}
		epoch.StartedAt = epoch.StartedAt.UTC()
		epoch.HeartbeatAt = epoch.HeartbeatAt.UTC()
		epochs = append(epochs, epoch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return epochs, nil
}

func validateOpenAIRouteEvidenceEpoch(epoch service.OpenAIRouteEvidenceEpoch) error {
	if strings.TrimSpace(epoch.EpochID) == "" || len(epoch.EpochID) > 64 ||
		strings.TrimSpace(epoch.InstanceID) == "" || len(epoch.InstanceID) > 64 {
		return errors.New("invalid OpenAI route evidence epoch identity")
	}
	if epoch.Component != service.OpenAIRouteEvidenceComponentAudit &&
		epoch.Component != service.OpenAIRouteEvidenceComponentObservation {
		return errors.New("invalid OpenAI route evidence component")
	}
	if epoch.StartedAt.IsZero() || epoch.HeartbeatAt.IsZero() || epoch.HeartbeatAt.Before(epoch.StartedAt) {
		return errors.New("invalid OpenAI route evidence epoch timestamps")
	}
	if epoch.CleanShutdown && (epoch.StoppedAt.IsZero() || epoch.StoppedAt.Before(epoch.StartedAt)) {
		return fmt.Errorf("invalid clean OpenAI route evidence epoch stop")
	}
	return nil
}
