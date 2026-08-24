package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *CompensationExecutionService) ExecuteApprovedDraft(ctx context.Context, draftID int64) (CompensationExecutionBatch, error) {
	result := CompensationExecutionBatch{DraftID: draftID, Executions: []CompensationExecution{}}
	if !rbacActorIsSuperAdmin(ctx) {
		return result, fmt.Errorf("owner or super-admin execution authorization is required")
	}
	preview, err := s.Preview(ctx, draftID)
	if err != nil {
		return result, err
	}
	if !preview.Executable {
		return result, fmt.Errorf("draft execution is blocked: %s", strings.Join(preview.Blockers, ","))
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	result.ApprovalID, _, err = s.lockAndValidateApprovedDraftTx(ctx, tx, draftID)
	if err != nil {
		return result, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO compensation_executions(execution_key,approval_id,draft_user_id,incident_id,user_id,benefit_channel,amount_cny_fen) SELECT 'compensation:'||d.incident_id::text||':'||u.user_id::text||':balance',$1::bigint,u.id,d.incident_id,u.user_id,'balance',u.balance_benefit_cny_fen FROM compensation_draft_users u JOIN compensation_drafts d ON d.id=u.draft_id WHERE u.draft_id=$2 AND u.eligible=TRUE AND u.included=TRUE AND u.evidence_complete=TRUE AND u.balance_benefit_cny_fen>0 ON CONFLICT(execution_key) DO NOTHING`, result.ApprovalID, draftID)
	if err != nil {
		return result, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO compensation_executions(execution_key,approval_id,draft_user_id,incident_id,user_id,benefit_channel,amount_cny_fen) SELECT 'compensation:'||d.incident_id::text||':'||u.user_id::text||':builder_pass',$1::bigint,u.id,d.incident_id,u.user_id,'builder_pass',u.builder_pass_benefit_cny_fen FROM compensation_draft_users u JOIN compensation_drafts d ON d.id=u.draft_id WHERE u.draft_id=$2 AND u.eligible=TRUE AND u.included=TRUE AND u.evidence_complete=TRUE AND u.builder_pass_benefit_cny_fen>0 ON CONFLICT(execution_key) DO NOTHING`, result.ApprovalID, draftID)
	if err != nil {
		return result, err
	}
	var expectedCount, mismatchCount int
	err = tx.QueryRowContext(ctx, `WITH expected AS (
 SELECT 'compensation:'||d.incident_id::text||':'||u.user_id::text||':balance' execution_key,u.id draft_user_id,d.incident_id,u.user_id,'balance' benefit_channel,u.balance_benefit_cny_fen amount_cny_fen FROM compensation_draft_users u JOIN compensation_drafts d ON d.id=u.draft_id WHERE u.draft_id=$1 AND u.eligible AND u.included AND u.evidence_complete AND u.balance_benefit_cny_fen>0
 UNION ALL
 SELECT 'compensation:'||d.incident_id::text||':'||u.user_id::text||':builder_pass',u.id,d.incident_id,u.user_id,'builder_pass',u.builder_pass_benefit_cny_fen FROM compensation_draft_users u JOIN compensation_drafts d ON d.id=u.draft_id WHERE u.draft_id=$1 AND u.eligible AND u.included AND u.evidence_complete AND u.builder_pass_benefit_cny_fen>0
) SELECT COUNT(*),COUNT(*) FILTER(WHERE x.id IS NULL OR x.approval_id<>$2 OR x.draft_user_id<>e.draft_user_id OR x.incident_id<>e.incident_id OR x.user_id<>e.user_id OR x.benefit_channel<>e.benefit_channel OR x.amount_cny_fen<>e.amount_cny_fen) FROM expected e LEFT JOIN compensation_executions x ON x.execution_key=e.execution_key`, draftID, result.ApprovalID).Scan(&expectedCount, &mismatchCount)
	if err != nil {
		return result, err
	}
	if expectedCount == 0 || mismatchCount != 0 {
		return result, fmt.Errorf("compensation execution identity failed exact readback")
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM compensation_executions WHERE approval_id=$1 ORDER BY id`, result.ApprovalID)
	if err != nil {
		return result, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return result, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return result, err
	}
	if len(ids) != expectedCount {
		return result, fmt.Errorf("compensation execution rows failed exact readback")
	}
	for _, id := range ids {
		execution, applyErr := s.applyExecution(ctx, id)
		if applyErr != nil {
			result.FailedCount++
			execution, _ = s.GetExecution(ctx, id)
		} else {
			result.VerifiedCount++
		}
		if execution != nil {
			result.Executions = append(result.Executions, *execution)
		}
	}
	userRows, err := s.db.QueryContext(ctx, `SELECT id FROM compensation_draft_users WHERE draft_id=$1 AND eligible=TRUE AND included=TRUE AND evidence_complete=TRUE AND (balance_benefit_cny_fen+builder_pass_benefit_cny_fen)>0 ORDER BY user_id`, draftID)
	if err != nil {
		return result, err
	}
	var draftUserIDs []int64
	for userRows.Next() {
		var id int64
		if err := userRows.Scan(&id); err != nil {
			_ = userRows.Close()
			return result, err
		}
		draftUserIDs = append(draftUserIDs, id)
	}
	_ = userRows.Close()
	for _, draftUserID := range draftUserIDs {
		created, noticeErr := s.ensureCompensationNotice(ctx, draftUserID)
		if noticeErr != nil {
			result.FailedCount++
			continue
		}
		if created {
			result.NoticesCreated++
		}
	}
	if result.FailedCount > 0 {
		result.State = "partial"
		return result, fmt.Errorf("compensation execution partially completed; retry resumes incomplete channels")
	}
	result.State = "verified"
	return result, nil
}

func (s *CompensationExecutionService) applyExecution(ctx context.Context, executionID int64) (*CompensationExecution, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var draftID, expectedApprovalID int64
	if err = tx.QueryRowContext(ctx, `SELECT u.draft_id,e.approval_id FROM compensation_executions e JOIN compensation_draft_users u ON u.id=e.draft_user_id WHERE e.id=$1`, executionID).Scan(&draftID, &expectedApprovalID); err != nil {
		return nil, err
	}
	approvalID, _, err := s.lockAndValidateApprovedDraftTx(ctx, tx, draftID)
	if err != nil {
		return nil, err
	}
	if approvalID != expectedApprovalID {
		return nil, fmt.Errorf("execution approval identity changed")
	}
	var state, channel string
	var userID, amount int64
	if err = tx.QueryRowContext(ctx, `SELECT state,benefit_channel,user_id,amount_cny_fen FROM compensation_executions WHERE id=$1 FOR UPDATE`, executionID).Scan(&state, &channel, &userID, &amount); err != nil {
		return nil, err
	}
	if state == "verified" {
		_ = tx.Commit()
		return s.GetExecution(ctx, executionID)
	}
	var receipt map[string]any
	switch channel {
	case "balance":
		receipt, err = s.applyBalanceExecutionTx(ctx, tx, executionID, userID, amount)
	case "builder_pass":
		receipt, err = s.applyBuilderPassExecutionTx(ctx, tx, executionID, userID, amount)
	default:
		err = fmt.Errorf("unsupported compensation channel")
	}
	if err != nil {
		_ = tx.Rollback()
		s.recordExecutionFailure(executionID, err)
		return nil, err
	}
	payload, err := canonicalCompensationJSON(receipt)
	if err != nil {
		return nil, err
	}
	hash := sha256Hex(payload)
	now := time.Now().UTC()
	if _, err = tx.ExecContext(ctx, `UPDATE compensation_executions SET state='verified',attempt_count=attempt_count+1,applied_at=COALESCE(applied_at,$2),verified_at=$2,last_error='',updated_at=$2 WHERE id=$1`, executionID, now); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO compensation_execution_receipts(execution_id,payload,payload_hash,verified_at) VALUES($1,$2,$3,$4) ON CONFLICT(execution_id) DO NOTHING`, executionID, payload, hash, now); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO compensation_execution_events(execution_id,event_type,state,details) VALUES($1,'readback_verified','verified',$2)`, executionID, payload); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if s.billingCache != nil {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if channel == "balance" {
			_ = s.billingCache.InvalidateUserBalance(cacheCtx, userID)
		} else {
			rows, _ := s.db.QueryContext(cacheCtx, `SELECT group_id FROM compensation_execution_assets WHERE execution_id=$1 AND group_id IS NOT NULL`, executionID)
			if rows != nil {
				for rows.Next() {
					var groupID int64
					if rows.Scan(&groupID) == nil {
						_ = s.billingCache.InvalidateSubscription(cacheCtx, userID, groupID)
					}
				}
				_ = rows.Close()
			}
		}
	}
	return s.GetExecution(ctx, executionID)
}

func (s *CompensationExecutionService) recordExecutionFailure(executionID int64, cause error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	message := truncateCustomerTierError(cause)
	updated, err := tx.ExecContext(ctx, `UPDATE compensation_executions SET state='failed',attempt_count=attempt_count+1,last_error=$2,updated_at=NOW() WHERE id=$1 AND state<>'verified'`, executionID, message)
	if err != nil {
		return
	}
	rows, err := updated.RowsAffected()
	if err != nil || rows != 1 {
		return
	}
	details, _ := canonicalCompensationJSON(map[string]any{"error": message})
	if _, err = tx.ExecContext(ctx, `INSERT INTO compensation_execution_events(execution_id,event_type,state,details) VALUES($1,'attempt_failed','failed',$2)`, executionID, details); err != nil {
		return
	}
	_ = tx.Commit()
}
