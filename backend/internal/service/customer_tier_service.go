package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

type CustomerTierService struct {
	db          *sql.DB
	settingRepo SettingRepository
	lifecycleMu sync.Mutex
	evaluateMu  sync.Mutex
	wg          sync.WaitGroup
	cancel      context.CancelFunc
}

func NewCustomerTierService(db *sql.DB, settingRepo SettingRepository) *CustomerTierService {
	return &CustomerTierService{db: db, settingRepo: settingRepo}
}

func (s *CustomerTierService) Name() string { return "customer-tier" }

func (s *CustomerTierService) Start(context.Context) error {
	if s == nil || !s.enabled(context.Background()) {
		return nil
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.cancel != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.EvaluateAll(ctx, time.Now().UTC()); err != nil {
		cancel()
		return err
	}
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		nextDaily := time.Now().UTC().Add(24 * time.Hour)
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				now = now.UTC()
				if err := s.RefreshExpiredOverrides(ctx, now); err != nil && ctx.Err() == nil {
					logger.LegacyPrintf("service.customer_tier", "override expiry refresh failed: %v", err)
				}
				if err := s.EnsureIncidentSnapshots(ctx, 0); err != nil && ctx.Err() == nil {
					logger.LegacyPrintf("service.customer_tier", "incident snapshot drain failed: %v", err)
				}
				if !now.Before(nextDaily) {
					if err := s.EvaluateAll(ctx, now); err != nil && ctx.Err() == nil {
						logger.LegacyPrintf("service.customer_tier", "daily evaluation failed: %v", err)
					}
					nextDaily = now.Add(24 * time.Hour)
				}
			}
		}
	}()
	return nil
}

func (s *CustomerTierService) Stop(context.Context) error {
	if s == nil {
		return nil
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	cancel := s.cancel
	s.cancel = nil
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}
	return nil
}

func (s *CustomerTierService) enabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyCustomerTierEvaluationEnabled)
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "enabled":
		return true
	default:
		return false
	}
}

func (s *CustomerTierService) CustomerTierSnapshotsEnabled(ctx context.Context) bool {
	return s.enabled(ctx)
}

func (s *CustomerTierService) UpdateEnabled(ctx context.Context, enabled bool) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("customer tier settings are unavailable")
	}
	if !enabled {
		if err := s.settingRepo.Set(ctx, SettingKeyCustomerTierEvaluationEnabled, "false"); err != nil {
			return err
		}
		return s.Stop(ctx)
	}
	before := s.enabled(ctx)
	if err := s.settingRepo.Set(ctx, SettingKeyCustomerTierEvaluationEnabled, "true"); err != nil {
		return err
	}
	if err := s.Start(ctx); err != nil {
		_ = s.settingRepo.Set(ctx, SettingKeyCustomerTierEvaluationEnabled, fmt.Sprintf("%t", before))
		return fmt.Errorf("customer tier preflight failed: %w", err)
	}
	return nil
}

func (s *CustomerTierService) EvaluateAll(ctx context.Context, at time.Time) error {
	if s == nil || !s.enabled(ctx) {
		return nil
	}
	if s.db == nil || at.IsZero() {
		return fmt.Errorf("customer tier evaluation dependencies are unavailable")
	}
	s.evaluateMu.Lock()
	defer s.evaluateMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtext('customer-tier-daily-evaluation'))`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtext('customer-tier-daily-evaluation'))`)
	}()
	// Incident snapshots are a separate safety obligation: a broken customer
	// evaluation must never prevent already-established impact from freezing.
	if err := s.EnsureIncidentSnapshots(ctx, 0); err != nil {
		return err
	}
	runID, cutoff, lastUserID, err := s.loadOrStartEvaluationRun(ctx, at.UTC())
	if err != nil {
		return err
	}
	for {
		rows, err := s.db.QueryContext(ctx, `SELECT id FROM users WHERE id>$1 AND created_at<=$2 ORDER BY id LIMIT 100`, lastUserID, cutoff)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, 100)
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(ids) == 0 {
			_, err = s.db.ExecContext(ctx, `UPDATE customer_tier_evaluator_state SET run_id=NULL,cutoff_at=NULL,last_user_id=0,updated_at=NOW() WHERE singleton=TRUE AND run_id=$1`, runID)
			if err != nil {
				return err
			}
			return s.EnsureIncidentSnapshots(ctx, 0)
		}
		for _, userID := range ids {
			if _, evalErr := s.EvaluateUserAt(ctx, userID, cutoff); evalErr != nil {
				_, recordErr := s.db.ExecContext(ctx, `INSERT INTO customer_tier_evaluation_failures(run_id,user_id,error_message,attempted_at) VALUES($1,$2,$3,NOW()) ON CONFLICT(run_id,user_id) DO UPDATE SET error_message=EXCLUDED.error_message,attempted_at=EXCLUDED.attempted_at`, runID, userID, truncateCustomerTierError(evalErr))
				if recordErr != nil {
					return recordErr
				}
				continue
			}
			_, _ = s.db.ExecContext(ctx, `UPDATE customer_tier_evaluation_failures SET resolved_at=NOW() WHERE user_id=$1 AND resolved_at IS NULL`, userID)
		}
		lastUserID = ids[len(ids)-1]
		if _, err := s.db.ExecContext(ctx, `UPDATE customer_tier_evaluator_state SET last_user_id=$2,updated_at=NOW() WHERE singleton=TRUE AND run_id=$1`, runID, lastUserID); err != nil {
			return err
		}
	}
}

func truncateCustomerTierError(err error) string {
	value := err.Error()
	if len(value) > 1000 {
		return value[:1000]
	}
	return value
}

func (s *CustomerTierService) loadOrStartEvaluationRun(ctx context.Context, requested time.Time) (uuid.UUID, time.Time, int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, time.Time{}, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var run sql.NullString
	var cutoff sql.NullTime
	var last int64
	if err := tx.QueryRowContext(ctx, `SELECT run_id::text,cutoff_at,last_user_id FROM customer_tier_evaluator_state WHERE singleton=TRUE FOR UPDATE`).Scan(&run, &cutoff, &last); err != nil {
		return uuid.Nil, time.Time{}, 0, err
	}
	var id uuid.UUID
	if run.Valid {
		id, err = uuid.Parse(run.String)
		if err != nil {
			return uuid.Nil, time.Time{}, 0, err
		}
	} else {
		id = uuid.New()
		cutoff = sql.NullTime{Time: requested, Valid: true}
		last = 0
		if _, err = tx.ExecContext(ctx, `UPDATE customer_tier_evaluator_state SET run_id=$1,cutoff_at=$2,last_user_id=0,updated_at=NOW() WHERE singleton=TRUE`, id, requested); err != nil {
			return uuid.Nil, time.Time{}, 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return uuid.Nil, time.Time{}, 0, err
	}
	return id, cutoff.Time.UTC(), last, nil
}

func (s *CustomerTierService) EvaluateUserAt(ctx context.Context, userID int64, at time.Time) (*CustomerTierEvaluation, error) {
	if s == nil || s.db == nil || userID <= 0 || at.IsZero() {
		return nil, fmt.Errorf("customer tier evaluation input is invalid")
	}
	policy, err := loadCustomerTierPolicy(ctx, s.db, at.UTC())
	if err != nil {
		return nil, err
	}
	evidence, err := s.loadEvidenceWithPolicy(ctx, userID, at.UTC(), policy)
	if err != nil {
		return nil, err
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(evidenceJSON)
	evidenceHash := hex.EncodeToString(hash[:])
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
		return nil, err
	}
	current, err := loadCustomerTierCurrentTx(ctx, tx, userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	override, err := loadActiveCustomerTierOverrideTx(ctx, tx, userID, at)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	calculated := policy.Classify(evidence.VerifiedPaidValueCNYFen)
	input := CustomerTierResolutionInput{Now: at.UTC(), CurrentTier: CustomerTierStandard, CalculatedTier: calculated, DowngradeGrace: policy.Grace()}
	if current != nil {
		input.CurrentTier = current.EffectiveTier
		input.GraceExpiresAt = current.GraceExpiresAt
		input.PreviousOverrideActive = current.OverrideID != nil && override == nil
	}
	if override != nil {
		input.OverrideActive = true
		input.OverrideTier = override.Tier
	}
	resolution := ResolveCustomerTier(input)
	keySource := fmt.Sprintf("%d|%d|%s|%s", userID, at.UnixNano(), evidenceHash, resolution.EffectiveTier)
	keyHash := sha256.Sum256([]byte(keySource))
	evaluationKey := "tier:" + hex.EncodeToString(keyHash[:])
	var evaluationID int64
	var overrideID any
	if override != nil {
		overrideID = override.ID
	}
	err = tx.QueryRowContext(ctx, `
INSERT INTO customer_tier_evaluations(evaluation_key,user_id,policy_version,window_started_at,window_ended_at,
 verified_paid_value_cny_fen,verified_paid_consumption_micros,calculated_tier,effective_tier,resolution_reason,
 override_id,grace_expires_at,evidence,policy_snapshot,evidence_hash,evaluated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$5)
ON CONFLICT(evaluation_key) DO NOTHING RETURNING id`, evaluationKey, userID, policy.Version,
		evidence.WindowStartedAt, evidence.WindowEndedAt, evidence.VerifiedPaidValueCNYFen, evidence.VerifiedPaidConsumptionMicros,
		string(calculated), string(resolution.EffectiveTier), resolution.Reason, overrideID, resolution.GraceExpiresAt, evidenceJSON, policyJSON, evidenceHash).Scan(&evaluationID)
	if err == sql.ErrNoRows {
		var existingID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM customer_tier_evaluations WHERE evaluation_key=$1 AND user_id=$2`, evaluationKey, userID).Scan(&existingID); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.loadCustomerTierEvaluation(ctx, existingID)
	}
	if err != nil {
		return nil, err
	}
	previousTier := any(nil)
	changed := current == nil || current.EffectiveTier != resolution.EffectiveTier
	if current != nil {
		previousTier = string(current.EffectiveTier)
	}
	if changed {
		if _, err := tx.ExecContext(ctx, `INSERT INTO customer_tier_history(user_id,evaluation_id,previous_tier,new_tier,change_reason,changed_at) VALUES($1,$2,$3,$4,$5,$6)`, userID, evaluationID, previousTier, string(resolution.EffectiveTier), resolution.Reason, at); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO customer_tier_current(user_id,policy_version,calculated_tier,effective_tier,verified_paid_value_cny_fen,
 verified_paid_consumption_micros,grace_expires_at,override_id,last_evaluation_id,evaluated_at,updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)
ON CONFLICT(user_id) DO UPDATE SET policy_version=EXCLUDED.policy_version,calculated_tier=EXCLUDED.calculated_tier,
 effective_tier=EXCLUDED.effective_tier,verified_paid_value_cny_fen=EXCLUDED.verified_paid_value_cny_fen,
 verified_paid_consumption_micros=EXCLUDED.verified_paid_consumption_micros,grace_expires_at=EXCLUDED.grace_expires_at,
 override_id=EXCLUDED.override_id,last_evaluation_id=EXCLUDED.last_evaluation_id,evaluated_at=EXCLUDED.evaluated_at,updated_at=EXCLUDED.updated_at`,
		userID, policy.Version, string(calculated), string(resolution.EffectiveTier), evidence.VerifiedPaidValueCNYFen,
		evidence.VerifiedPaidConsumptionMicros, resolution.GraceExpiresAt, overrideID, evaluationID, at)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result := &CustomerTierEvaluation{ID: evaluationID, UserID: userID, PolicyVersion: policy.Version, CalculatedTier: calculated,
		EffectiveTier: resolution.EffectiveTier, ResolutionReason: resolution.Reason, VerifiedPaidValueCNYFen: evidence.VerifiedPaidValueCNYFen,
		VerifiedPaidConsumptionMicros: evidence.VerifiedPaidConsumptionMicros, GraceExpiresAt: resolution.GraceExpiresAt, Evidence: evidence, EvidenceHash: evidenceHash, EvaluatedAt: at.UTC()}
	if override != nil {
		value := override.ID
		result.OverrideID = &value
	}
	return result, nil
}

type customerTierQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadCustomerTierPolicy(ctx context.Context, q customerTierQueryer, at time.Time) (CustomerTierPolicy, error) {
	var p CustomerTierPolicy
	err := q.QueryRowContext(ctx, `SELECT version,rolling_window_days,downgrade_grace_days,priority_threshold_cny_fen,strategic_threshold_cny_fen,standard_multiplier::double precision,priority_multiplier::double precision,strategic_multiplier::double precision,standard_cap_cny_fen,priority_cap_cny_fen,strategic_cap_cny_fen,effective_from FROM customer_tier_policy_versions WHERE effective_from<=$1 ORDER BY effective_from DESC,version DESC LIMIT 1`, at).Scan(&p.Version, &p.RollingWindowDays, &p.DowngradeGraceDays, &p.PriorityThresholdCNYFen, &p.StrategicThresholdCNYFen, &p.StandardMultiplier, &p.PriorityMultiplier, &p.StrategicMultiplier, &p.StandardCapCNYFen, &p.PriorityCapCNYFen, &p.StrategicCapCNYFen, &p.EffectiveFrom)
	if err != nil {
		return p, err
	}
	if !p.Valid() {
		return p, fmt.Errorf("customer tier policy version %d is invalid", p.Version)
	}
	p.EffectiveFrom = p.EffectiveFrom.UTC()
	return p, nil
}

func (s *CustomerTierService) loadCustomerTierEvaluation(ctx context.Context, id int64) (*CustomerTierEvaluation, error) {
	item := &CustomerTierEvaluation{}
	var calculated, effective string
	var grace sql.NullTime
	var override sql.NullInt64
	var evidence []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,policy_version,calculated_tier,effective_tier,resolution_reason,verified_paid_value_cny_fen,verified_paid_consumption_micros,grace_expires_at,override_id,evidence,evidence_hash,evaluated_at FROM customer_tier_evaluations WHERE id=$1`, id).Scan(&item.ID, &item.UserID, &item.PolicyVersion, &calculated, &effective, &item.ResolutionReason, &item.VerifiedPaidValueCNYFen, &item.VerifiedPaidConsumptionMicros, &grace, &override, &evidence, &item.EvidenceHash, &item.EvaluatedAt)
	if err != nil {
		return nil, err
	}
	item.CalculatedTier = CustomerTier(calculated)
	item.EffectiveTier = CustomerTier(effective)
	if grace.Valid {
		v := grace.Time.UTC()
		item.GraceExpiresAt = &v
	}
	if override.Valid {
		v := override.Int64
		item.OverrideID = &v
	}
	if err := json.Unmarshal(evidence, &item.Evidence); err != nil {
		return nil, err
	}
	item.EvaluatedAt = item.EvaluatedAt.UTC()
	return item, nil
}

func (s *CustomerTierService) loadEvidenceWithPolicy(ctx context.Context, userID int64, end time.Time, policy CustomerTierPolicy) (CustomerTierEvidence, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return CustomerTierEvidence{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := s.loadEvidenceWithPolicyTx(ctx, tx, userID, end, policy)
	if err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (s *CustomerTierService) loadEvidenceWithPolicyTx(ctx context.Context, tx *sql.Tx, userID int64, end time.Time, policy CustomerTierPolicy) (CustomerTierEvidence, error) {
	start := end.Add(-policy.Window())
	result := CustomerTierEvidence{WindowStartedAt: start, WindowEndedAt: end, PaidSources: []CustomerTierEvidenceItem{}, Policy: policy}
	var err error
	cursorTime, cursorType, cursorID := start, "", int64(0)
	for {
		rows, err := tx.QueryContext(ctx, `
WITH gross_sources AS (
 SELECT 'topup_order'::text source_type,id source_id,amount_cny_fen::bigint gross_amount_cny_fen,
  COALESCE(completed_at,updated_at,created_at) occurred_at,(status='completed' AND completed_at IS NOT NULL) included_base,
  CASE WHEN status='completed' AND completed_at IS NOT NULL THEN '' ELSE 'status_not_completed' END exclusion_reason
 FROM topup_orders WHERE user_id=$1 AND COALESCE(completed_at,updated_at,created_at)>=$2 AND COALESCE(completed_at,updated_at,created_at)<$3
 UNION ALL
 SELECT 'payment_order',id,amount_cents::bigint,COALESCE(completed_at,updated_at,created_at),
  (status='completed' AND completed_at IS NOT NULL),CASE WHEN status='completed' AND completed_at IS NOT NULL THEN '' ELSE 'status_not_completed' END
 FROM payment_orders WHERE user_id=$1 AND COALESCE(completed_at,updated_at,created_at)>=$2 AND COALESCE(completed_at,updated_at,created_at)<$3
 UNION ALL
 SELECT 'native_checkout_order',id,pay_amount_cny_fen,COALESCE(completed_at,updated_at,created_at),
  (status='completed' AND completed_at IS NOT NULL AND redeem_purpose='sale_recharge' AND redeem_sales_status='sold'),
  CASE WHEN status<>'completed' OR completed_at IS NULL THEN 'status_not_completed' WHEN redeem_purpose<>'sale_recharge' THEN 'non_sale_purpose' WHEN redeem_sales_status<>'sold' THEN 'not_sold' ELSE '' END
 FROM native_checkout_orders WHERE user_id=$1 AND COALESCE(completed_at,updated_at,created_at)>=$2 AND COALESCE(completed_at,updated_at,created_at)<$3
 UNION ALL
 SELECT 'redeem_code',r.id,ROUND(r.paid_value*100)::bigint,COALESCE(r.sold_at,r.used_at,r.updated_at,r.created_at),
  (r.status='used' AND r.used_by=$1 AND r.purpose='sale_recharge' AND r.sales_status='sold' AND r.paid_value>0 AND NOT EXISTS(SELECT 1 FROM native_checkout_orders n WHERE n.redeem_code_id=r.id AND n.status='completed')),
  CASE WHEN r.status<>'used' OR r.used_by IS DISTINCT FROM $1 THEN 'not_attributed_to_user'
       WHEN r.purpose<>'sale_recharge' THEN 'non_sale_purpose' WHEN r.sales_status<>'sold' THEN 'not_sold'
       WHEN r.paid_value<=0 THEN 'unresolved_paid_value'
       WHEN EXISTS(SELECT 1 FROM native_checkout_orders n WHERE n.redeem_code_id=r.id AND n.status='completed') THEN 'native_checkout_duplicate'
       ELSE '' END
 FROM redeem_codes r WHERE r.used_by=$1 AND COALESCE(r.sold_at,r.used_at,r.updated_at,r.created_at)>=$2 AND COALESCE(r.sold_at,r.used_at,r.updated_at,r.created_at)<$3
), refunds AS (
 SELECT source_type,source_id,SUM(amount_cny_fen)::bigint refunded_amount_cny_fen
 FROM customer_paid_value_refunds WHERE user_id=$1 AND refunded_at<$3 GROUP BY source_type,source_id
), paid_sources AS (
 SELECT g.*,COALESCE(r.refunded_amount_cny_fen,0)::bigint refunded_amount_cny_fen,
  GREATEST(g.gross_amount_cny_fen-COALESCE(r.refunded_amount_cny_fen,0),0)::bigint amount_cny_fen,
  (g.included_base AND g.gross_amount_cny_fen>COALESCE(r.refunded_amount_cny_fen,0)) included,
  CASE WHEN g.included_base AND g.gross_amount_cny_fen<=COALESCE(r.refunded_amount_cny_fen,0) THEN 'fully_refunded' ELSE g.exclusion_reason END final_exclusion_reason
 FROM gross_sources g LEFT JOIN refunds r USING(source_type,source_id)
)
SELECT source_type,source_id,gross_amount_cny_fen,refunded_amount_cny_fen,amount_cny_fen,occurred_at,included,final_exclusion_reason
FROM paid_sources WHERE (occurred_at,source_type,source_id)>($4,$5,$6)
ORDER BY occurred_at,source_type,source_id LIMIT 500`, userID, start, end, cursorTime, cursorType, cursorID)
		if err != nil {
			return result, err
		}
		count := 0
		for rows.Next() {
			var item CustomerTierEvidenceItem
			if err := rows.Scan(&item.SourceType, &item.SourceID, &item.GrossAmountCNYFen, &item.RefundedAmountCNYFen, &item.AmountCNYFen, &item.OccurredAt, &item.Included, &item.ExclusionReason); err != nil {
				_ = rows.Close()
				return result, err
			}
			item.OccurredAt = item.OccurredAt.UTC()
			result.PaidSources = append(result.PaidSources, item)
			count++
			cursorTime, cursorType, cursorID = item.OccurredAt, item.SourceType, item.SourceID
			if item.Included && item.AmountCNYFen > 0 {
				if result.VerifiedPaidValueCNYFen > math.MaxInt64-item.AmountCNYFen {
					_ = rows.Close()
					return result, fmt.Errorf("verified paid value exceeds supported range")
				}
				result.VerifiedPaidValueCNYFen += item.AmountCNYFen
			}
		}
		if err := rows.Close(); err != nil {
			return result, err
		}
		if count < 500 {
			break
		}
	}
	err = tx.QueryRowContext(ctx, `
SELECT
 COALESCE((SELECT SUM(c.amount_micros) FROM balance_lot_consumptions c JOIN balance_lots l ON l.id=c.balance_lot_id WHERE c.user_id=$1 AND l.source_type IN('paid_redeem','paid_topup') AND c.created_at>=$2 AND c.created_at<$3),0)::bigint,
 COALESCE((SELECT SUM(confirmed_amount_micros) FROM monthly_entitlement_consumptions WHERE user_id=$1 AND created_at>=$2 AND created_at<$3),0)::bigint,
 COALESCE((SELECT SUM(GREATEST(c.confirmed_consumption_micros-COALESCE(e.recorded,0),0)) FROM monthly_entitlement_cycles c LEFT JOIN (SELECT cycle_id,SUM(confirmed_amount_micros) recorded FROM monthly_entitlement_consumptions GROUP BY cycle_id) e ON e.cycle_id=c.id WHERE c.user_id=$1 AND c.source_type IN('paid_redeem','paid_topup') AND c.starts_at<$3 AND c.ends_at>$2),0)::bigint`, userID, start, end).Scan(&result.BalancePaidConsumptionMicros, &result.BuilderPassConsumptionMicros, &result.UnattributedBuilderPassConsumptionMicros)
	if err != nil {
		return result, err
	}
	result.VerifiedPaidConsumptionMicros = result.BalancePaidConsumptionMicros + result.BuilderPassConsumptionMicros
	return result, nil
}

func loadCustomerTierCurrentTx(ctx context.Context, tx *sql.Tx, userID int64) (*CustomerTierCurrent, error) {
	item := &CustomerTierCurrent{}
	var calculated, effective string
	var grace sql.NullTime
	var override sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT user_id,calculated_tier,effective_tier,verified_paid_value_cny_fen,verified_paid_consumption_micros,grace_expires_at,override_id,last_evaluation_id,evaluated_at FROM customer_tier_current WHERE user_id=$1 FOR UPDATE`, userID).Scan(&item.UserID, &calculated, &effective, &item.VerifiedPaidValueCNYFen, &item.VerifiedPaidConsumptionMicros, &grace, &override, &item.LastEvaluationID, &item.EvaluatedAt)
	if err != nil {
		return nil, err
	}
	item.CalculatedTier, item.EffectiveTier = CustomerTier(calculated), CustomerTier(effective)
	if grace.Valid {
		value := grace.Time.UTC()
		item.GraceExpiresAt = &value
	}
	if override.Valid {
		value := override.Int64
		item.OverrideID = &value
	}
	item.EvaluatedAt = item.EvaluatedAt.UTC()
	return item, nil
}

func loadActiveCustomerTierOverrideTx(ctx context.Context, tx *sql.Tx, userID int64, at time.Time) (*CustomerTierOverride, error) {
	item := &CustomerTierOverride{}
	var tier string
	var actor sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT id,tier,reason,starts_at,expires_at,created_by_user_id,created_at FROM customer_tier_overrides WHERE user_id=$1 AND starts_at<=$2 AND expires_at>$2 AND created_at<=$2 ORDER BY created_at DESC,id DESC LIMIT 1`, userID, at).Scan(&item.ID, &tier, &item.Reason, &item.StartsAt, &item.ExpiresAt, &actor, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	item.Tier = CustomerTier(tier)
	if actor.Valid {
		value := actor.Int64
		item.CreatedByUserID = &value
	}
	return item, nil
}

var _ LifecycleComponent = (*CustomerTierService)(nil)
