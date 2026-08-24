package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type CompensationExecutionService struct {
	db           *sql.DB
	settingRepo  SettingRepository
	drafts       *CompensationControlService
	billingCache *BillingCacheService
}

func NewCompensationExecutionService(db *sql.DB, settings SettingRepository, drafts *CompensationControlService, billingCache *BillingCacheService) *CompensationExecutionService {
	return &CompensationExecutionService{db: db, settingRepo: settings, drafts: drafts, billingCache: billingCache}
}
func (s *CompensationExecutionService) enabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyCompensationExecutionEnabled)
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

func (s *CompensationExecutionService) UpdateEnabled(ctx context.Context, command CompensationExecutionSettingsCommand) error {
	if s == nil || s.db == nil || command.ActorUserID <= 0 || !rbacActorIsSuperAdmin(ctx) {
		return fmt.Errorf("execution setting actor is required")
	}
	var assessment any = map[string]any{"eligible_for_owner_review": false, "action": "owner_kill_switch"}
	if command.Enabled {
		current, err := s.drafts.Assessment(ctx)
		if err != nil {
			return err
		}
		if command.Confirmation != CompensationExecutionEnableConfirmation {
			return fmt.Errorf("exact execution confirmation is required")
		}
		if !current.EligibleForOwnerReview {
			return fmt.Errorf("compensation Shadow exit gates have not passed")
		}
		assessment = current
	}
	payload, err := canonicalCompensationJSON(assessment)
	if err != nil {
		return err
	}
	confirmationHash := sha256.Sum256([]byte(command.Confirmation))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-execution-permission'))`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO compensation_execution_permission_audit(enabled,actor_user_id,confirmation_hash,assessment) VALUES($1,$2,$3,$4)`, command.Enabled, command.ActorUserID, hex.EncodeToString(confirmationHash[:]), payload); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=EXCLUDED.updated_at`, SettingKeyCompensationExecutionEnabled, fmt.Sprintf("%t", command.Enabled)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *CompensationExecutionService) ApproveDraft(ctx context.Context, command CompensationApprovalCommand) (*CompensationDraftApproval, error) {
	command.ApprovalKey = strings.TrimSpace(command.ApprovalKey)
	command.Reason = strings.TrimSpace(command.Reason)
	command.PreviewHash = strings.TrimSpace(command.PreviewHash)
	if s == nil || s.db == nil || command.DraftID <= 0 || command.ActorUserID <= 0 || !rbacActorIsSuperAdmin(ctx) || command.ApprovalKey == "" || len(command.ApprovalKey) > 180 || command.Reason == "" || len(command.Reason) > 1000 || len(command.PreviewHash) != 64 || command.Confirmation != CompensationDraftApprovalConfirmation {
		return nil, fmt.Errorf("draft, owner confirmation, idempotency key, actor, and reason are required")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	assessment, err := s.drafts.Assessment(ctx)
	if err != nil {
		return nil, err
	}
	if !assessment.EligibleForOwnerReview {
		return nil, fmt.Errorf("compensation Shadow exit gates have not passed")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var incidentID int64
	if err = tx.QueryRowContext(ctx, `SELECT incident_id FROM compensation_drafts WHERE id=$1`, command.DraftID).Scan(&incidentID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-draft'),$1)`, incidentID); err != nil {
		return nil, err
	}
	var evidenceHash, series, state, phase, reviewResult string
	var high bool
	var latestID int64
	err = tx.QueryRowContext(ctx, `SELECT d.evidence_hash,d.series_id::text,d.state,d.high_value,i.phase,r.result FROM compensation_drafts d JOIN reliability_incidents i ON i.id=d.incident_id JOIN compensation_shadow_reviews r ON r.draft_id=d.id WHERE d.id=$1`, command.DraftID).Scan(&evidenceHash, &series, &state, &high, &phase, &reviewResult)
	if err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM compensation_drafts WHERE series_id=$1 ORDER BY revision_number DESC,id DESC LIMIT 1`, series).Scan(&latestID); err != nil {
		return nil, err
	}
	if latestID != command.DraftID || high || state != "shadow" || phase != "resolved" || reviewResult != "aligned" {
		return nil, fmt.Errorf("only the latest resolved aligned non-high-value draft can be approved")
	}
	existing := &CompensationDraftApproval{}
	err = tx.QueryRowContext(ctx, `SELECT id,draft_id,draft_evidence_hash,approved_preview_hash,approval_key,approval_reason,approved_by_user_id,approved_at FROM compensation_draft_approvals WHERE approval_key=$1 OR draft_id=$2 ORDER BY (approval_key=$1) DESC LIMIT 1`, command.ApprovalKey, command.DraftID).Scan(&existing.ID, &existing.DraftID, &existing.DraftEvidenceHash, &existing.ApprovedPreviewHash, &existing.ApprovalKey, &existing.ApprovalReason, &existing.ApprovedByUserID, &existing.ApprovedAt)
	if err == nil {
		if existing.DraftID != command.DraftID || existing.DraftEvidenceHash != evidenceHash || existing.ApprovedPreviewHash != command.PreviewHash || existing.ApprovalKey != command.ApprovalKey || existing.ApprovalReason != command.Reason || existing.ApprovedByUserID != command.ActorUserID {
			return nil, fmt.Errorf("draft approval already exists with different input")
		}
		frozen, loadErr := loadFrozenCompensationNotices(ctx, tx, existing.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		readHash, hashErr := compensationApprovalPreviewHash(command.DraftID, evidenceHash, frozen)
		if hashErr != nil || readHash != existing.ApprovedPreviewHash {
			return nil, fmt.Errorf("approved preview failed exact readback")
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	noticeSnapshots, err := buildCompensationNoticePreviews(ctx, tx, command.DraftID)
	if err != nil {
		return nil, err
	}
	notices := compensationNoticePreviews(noticeSnapshots)
	if len(notices) == 0 {
		return nil, fmt.Errorf("draft has no positive user benefit to approve")
	}
	previewHash, err := compensationApprovalPreviewHash(command.DraftID, evidenceHash, notices)
	if err != nil {
		return nil, err
	}
	if previewHash != command.PreviewHash {
		return nil, fmt.Errorf("approval preview changed; refresh and review again")
	}
	item := &CompensationDraftApproval{DraftID: command.DraftID, DraftEvidenceHash: evidenceHash, ApprovedPreviewHash: previewHash, ApprovalKey: command.ApprovalKey, ApprovalReason: command.Reason, ApprovedByUserID: command.ActorUserID}
	err = tx.QueryRowContext(ctx, `INSERT INTO compensation_draft_approvals(draft_id,draft_evidence_hash,approved_preview_hash,approval_key,approval_reason,owner_confirmation,approved_by_user_id) VALUES($1,$2,$3,$4,$5,TRUE,$6) RETURNING id,approved_at`, item.DraftID, item.DraftEvidenceHash, item.ApprovedPreviewHash, item.ApprovalKey, item.ApprovalReason, item.ApprovedByUserID).Scan(&item.ID, &item.ApprovedAt)
	if err != nil {
		return nil, err
	}
	if err = freezeCompensationApprovalNoticesTx(ctx, tx, item.ID, noticeSnapshots); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *CompensationExecutionService) Preview(ctx context.Context, draftID int64) (CompensationExecutionPreview, error) {
	result := CompensationExecutionPreview{DraftID: draftID, Blockers: []string{}}
	if s == nil || s.db == nil || draftID <= 0 {
		return result, fmt.Errorf("draft is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result.ExecutionEnabled = s.enabled(ctx)
	assessment, err := s.drafts.Assessment(ctx)
	if err != nil {
		return result, err
	}
	result.ShadowGateEligible = assessment.EligibleForOwnerReview
	if result.ExecutionEnabled {
		authorized, authErr := s.permissionAuthorized(ctx)
		if authErr != nil {
			return result, authErr
		}
		result.ShadowGateEligible = authorized
	}
	var series, phase, evidenceHash string
	var latestID int64
	err = s.db.QueryRowContext(ctx, `SELECT d.series_id::text,d.evidence_hash,i.phase,COALESCE(SUM(CASE WHEN u.eligible AND u.included AND u.evidence_complete THEN u.proposed_total_cny_fen ELSE 0 END),0)::bigint,COALESCE(SUM(CASE WHEN u.eligible AND u.included AND u.evidence_complete THEN u.balance_benefit_cny_fen ELSE 0 END),0)::bigint,COALESCE(SUM(CASE WHEN u.eligible AND u.included AND u.evidence_complete THEN u.builder_pass_benefit_cny_fen ELSE 0 END),0)::bigint,COUNT(*) FILTER(WHERE u.eligible AND u.included AND u.evidence_complete) FROM compensation_drafts d JOIN reliability_incidents i ON i.id=d.incident_id LEFT JOIN compensation_draft_users u ON u.draft_id=d.id WHERE d.id=$1 GROUP BY d.series_id,d.evidence_hash,i.phase`, draftID).Scan(&series, &evidenceHash, &phase, &result.TotalCNYFen, &result.BalanceCNYFen, &result.BuilderPassCNYFen, &result.UserCount)
	if err != nil {
		return result, err
	}
	result.IncidentResolved = phase == "resolved"
	if err = s.db.QueryRowContext(ctx, `SELECT id FROM compensation_drafts WHERE series_id=$1 ORDER BY revision_number DESC,id DESC LIMIT 1`, series).Scan(&latestID); err != nil {
		return result, err
	}
	result.Latest = latestID == draftID
	var approval sql.NullInt64
	var approvedPreviewHash sql.NullString
	if err = s.db.QueryRowContext(ctx, `SELECT id,approved_preview_hash FROM compensation_draft_approvals WHERE draft_id=$1`, draftID).Scan(&approval, &approvedPreviewHash); err != nil && err != sql.ErrNoRows {
		return result, err
	}
	if approval.Valid {
		v := approval.Int64
		result.ApprovalID = &v
	}
	if !result.Latest {
		result.Blockers = append(result.Blockers, "not_latest_revision")
	}
	if !result.IncidentResolved {
		result.Blockers = append(result.Blockers, "incident_not_resolved")
	}
	if !result.ShadowGateEligible {
		result.Blockers = append(result.Blockers, "shadow_gate_not_passed")
	}
	if !result.ExecutionEnabled {
		result.Blockers = append(result.Blockers, "execution_permission_disabled")
	}
	if !approval.Valid {
		result.Blockers = append(result.Blockers, "draft_not_approved")
	}
	if result.TotalCNYFen <= 0 {
		result.Blockers = append(result.Blockers, "no_positive_benefit")
	}
	if approval.Valid {
		result.NoticePreviews, err = loadFrozenCompensationNotices(ctx, s.db, approval.Int64)
	} else {
		result.NoticePreviews, err = s.previewCompensationNotices(ctx, draftID)
	}
	if err != nil {
		return result, err
	}
	result.ApprovalPreviewHash, err = compensationApprovalPreviewHash(draftID, evidenceHash, result.NoticePreviews)
	if err != nil {
		return result, err
	}
	if approvedPreviewHash.Valid && result.ApprovalPreviewHash != approvedPreviewHash.String {
		return result, fmt.Errorf("approved compensation preview failed exact readback")
	}
	result.Executable = len(result.Blockers) == 0
	return result, nil
}

func (s *CompensationExecutionService) permissionAuthorized(ctx context.Context) (bool, error) {
	var enabled, eligible bool
	err := s.db.QueryRowContext(ctx, `SELECT enabled,COALESCE((assessment->>'eligible_for_owner_review')::boolean,FALSE) FROM compensation_execution_permission_audit ORDER BY id DESC LIMIT 1`).Scan(&enabled, &eligible)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabled && eligible, nil
}

func (s *CompensationExecutionService) lockAndValidateApprovedDraftTx(ctx context.Context, tx *sql.Tx, draftID int64) (int64, int64, error) {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-execution-permission'))`); err != nil {
		return 0, 0, err
	}
	var setting string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, SettingKeyCompensationExecutionEnabled).Scan(&setting); err != nil {
		return 0, 0, err
	}
	if strings.ToLower(strings.TrimSpace(setting)) != "true" {
		return 0, 0, fmt.Errorf("compensation execution permission is disabled")
	}
	var permissionEnabled, permissionEligible bool
	if err := tx.QueryRowContext(ctx, `SELECT enabled,COALESCE((assessment->>'eligible_for_owner_review')::boolean,FALSE) FROM compensation_execution_permission_audit ORDER BY id DESC LIMIT 1`).Scan(&permissionEnabled, &permissionEligible); err != nil {
		return 0, 0, err
	}
	if !permissionEnabled || !permissionEligible {
		return 0, 0, fmt.Errorf("compensation execution permission audit is not authorized")
	}
	var incidentID int64
	if err := tx.QueryRowContext(ctx, `SELECT incident_id FROM compensation_drafts WHERE id=$1`, draftID).Scan(&incidentID); err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-draft'),$1)`, incidentID); err != nil {
		return 0, 0, err
	}
	var approvalID, latestID int64
	var evidenceHash, approvedEvidenceHash, approvedPreviewHash, series, state, phase, review string
	var high bool
	err := tx.QueryRowContext(ctx, `SELECT a.id,d.evidence_hash,a.draft_evidence_hash,a.approved_preview_hash,d.series_id::text,d.state,d.high_value,i.phase,r.result FROM compensation_drafts d JOIN compensation_draft_approvals a ON a.draft_id=d.id JOIN reliability_incidents i ON i.id=d.incident_id JOIN compensation_shadow_reviews r ON r.draft_id=d.id WHERE d.id=$1`, draftID).Scan(&approvalID, &evidenceHash, &approvedEvidenceHash, &approvedPreviewHash, &series, &state, &high, &phase, &review)
	if err != nil {
		return 0, 0, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM compensation_drafts WHERE series_id=$1 ORDER BY revision_number DESC,id DESC LIMIT 1`, series).Scan(&latestID); err != nil {
		return 0, 0, err
	}
	if latestID != draftID || evidenceHash != approvedEvidenceHash || state != "shadow" || high || phase != "resolved" || review != "aligned" {
		return 0, 0, fmt.Errorf("approved draft is no longer executable")
	}
	notices, err := loadFrozenCompensationNotices(ctx, tx, approvalID)
	if err != nil {
		return 0, 0, err
	}
	if len(notices) == 0 {
		return 0, 0, fmt.Errorf("approved draft has no frozen notice snapshot")
	}
	previewHash, err := compensationApprovalPreviewHash(draftID, evidenceHash, notices)
	if err != nil {
		return 0, 0, err
	}
	if previewHash != approvedPreviewHash {
		return 0, 0, fmt.Errorf("approved preview failed exact readback")
	}
	return approvalID, incidentID, nil
}
