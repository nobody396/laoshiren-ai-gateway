package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *IncidentControlService) ConfirmCandidate(ctx context.Context, command IncidentConfirmCommand) (*AdminIncident, error) {
	command.Title = strings.TrimSpace(command.Title)
	command.InternalSummary = strings.TrimSpace(command.InternalSummary)
	if s == nil || s.db == nil || command.CandidateID <= 0 || command.Title == "" || len(command.Title) > 200 || len(command.InternalSummary) > 5000 {
		return nil, fmt.Errorf("candidate, title, and bounded summary are required")
	}
	now := command.At.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('reliability-incident-control'))`); err != nil {
		return nil, err
	}
	var state string
	var firstObserved time.Time
	var existing sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT state,first_observed_at,confirmed_incident_id FROM reliability_incident_candidates WHERE id=$1 FOR UPDATE`, command.CandidateID).Scan(&state, &firstObserved, &existing); err != nil {
		return nil, err
	}
	if state == string(IncidentCandidateConfirmed) && existing.Valid {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.GetAdminIncident(ctx, existing.Int64)
	}
	if state != string(IncidentCandidateOpen) {
		return nil, fmt.Errorf("incident candidate is not open")
	}
	publicID := uuid.NewString()
	var incidentID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO reliability_incidents(public_id,candidate_id,phase,title,internal_summary,observation_started_at,created_by_user_id,updated_by_user_id)
VALUES($1,$2,'investigating',$3,$4,$5,NULLIF($6,0),NULLIF($6,0)) RETURNING id`, publicID, command.CandidateID, command.Title, command.InternalSummary, firstObserved, command.ActorUserID).Scan(&incidentID)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `
INSERT INTO reliability_incident_products(incident_id,product_id,affected_at,current_status)
SELECT $1,cp.product_id,cp.first_observed_at,cp.latest_status FROM reliability_incident_candidate_products cp WHERE cp.candidate_id=$2`, incidentID, command.CandidateID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reliability_incident_candidates SET state='confirmed',confirmed_incident_id=$2,updated_at=$3 WHERE id=$1`, command.CandidateID, incidentID, now); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reliability_incident_updates(incident_id,phase,kind,internal_message,created_by_user_id) VALUES($1,'investigating','transition','Incident confirmed from qualified candidate',NULLIF($2,0))`, incidentID, command.ActorUserID); err != nil {
		return nil, err
	}
	after, _ := json.Marshal(map[string]any{"phase": IncidentPhaseInvestigating, "title": command.Title})
	if _, err = tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(incident_id,candidate_id,action,actor_user_id,after_state) VALUES($1,$2,'confirm_candidate',NULLIF($3,0),$4)`, incidentID, command.CandidateID, command.ActorUserID, after); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if err := s.ReconcileAt(ctx, now); err != nil {
		return nil, err
	}
	return s.GetAdminIncident(ctx, incidentID)
}

func (s *IncidentControlService) DismissCandidate(ctx context.Context, command IncidentDismissCommand) error {
	command.Reason = strings.TrimSpace(command.Reason)
	if s == nil || s.db == nil || command.CandidateID <= 0 || command.Reason == "" || len(command.Reason) > 500 {
		return fmt.Errorf("candidate and reason are required")
	}
	now := command.At.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE reliability_incident_candidates SET state='dismissed',dismissed_reason=$2,dismissed_by_user_id=NULLIF($3,0),dismissed_at=$4,updated_at=$4 WHERE id=$1 AND state='open'`, command.CandidateID, command.Reason, command.ActorUserID, now)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("incident candidate is not open")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(candidate_id,action,actor_user_id,reason,after_state) VALUES($1,'dismiss_candidate',NULLIF($2,0),$3,'{"state":"dismissed"}')`, command.CandidateID, command.ActorUserID, command.Reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *IncidentControlService) Transition(ctx context.Context, command IncidentTransitionCommand) error {
	command.Message = strings.TrimSpace(command.Message)
	if s == nil || s.db == nil || command.IncidentID <= 0 || !command.Target.Valid() || command.Message == "" || len(command.Message) > 5000 {
		return fmt.Errorf("incident, target phase, and message are required")
	}
	now := command.At.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('reliability-incident-control'))`); err != nil {
		return err
	}
	state, _, err := loadIncidentLifecycleState(ctx, tx, command.IncidentID)
	if err != nil {
		return err
	}
	if err := ValidateIncidentPhaseTransition(state, command.Target, now); err != nil {
		return err
	}
	before, _ := json.Marshal(map[string]any{"phase": state.Phase})
	after, _ := json.Marshal(map[string]any{"phase": command.Target})
	var monitoring, resolved any
	if command.Target == IncidentPhaseMonitoring {
		monitoring = now
	}
	if command.Target == IncidentPhaseResolved {
		resolved = now
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE reliability_incidents SET phase=$2,monitoring_since=COALESCE($3::timestamptz,monitoring_since),resolved_at=$4::timestamptz,
 observation_ended_at=CASE WHEN $4::timestamptz IS NULL THEN observation_ended_at ELSE $4::timestamptz END,
 customer_impact_ended_at=CASE WHEN $4::timestamptz IS NULL THEN customer_impact_ended_at ELSE (SELECT MAX(ended_at) FROM reliability_incident_impact_segments WHERE incident_id=$1) END,
 updated_by_user_id=NULLIF($5,0),version=version+1,updated_at=$6 WHERE id=$1`, command.IncidentID, string(command.Target), monitoring, resolved, command.ActorUserID, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reliability_incident_updates(incident_id,phase,kind,internal_message,created_by_user_id) VALUES($1,$2,'transition',$3,NULLIF($4,0))`, command.IncidentID, string(command.Target), command.Message, command.ActorUserID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(incident_id,action,actor_user_id,reason,before_state,after_state) VALUES($1,'transition',NULLIF($2,0),$3,$4,$5)`, command.IncidentID, command.ActorUserID, command.Message, before, after); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *IncidentControlService) AddUpdate(ctx context.Context, command IncidentUpdateCommand) error {
	command.Message = strings.TrimSpace(command.Message)
	if s == nil || s.db == nil || command.IncidentID <= 0 || command.Message == "" || len(command.Message) > 5000 {
		return fmt.Errorf("incident and message are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
INSERT INTO reliability_incident_updates(incident_id,phase,kind,internal_message,created_by_user_id)
SELECT id,phase,'operator',$2,NULLIF($3,0) FROM reliability_incidents WHERE id=$1`, command.IncidentID, command.Message, command.ActorUserID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("incident not found")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(incident_id,action,actor_user_id,reason,after_state) VALUES($1,'add_internal_update',NULLIF($2,0),$3,'{}')`, command.IncidentID, command.ActorUserID, command.Message); err != nil {
		return err
	}
	return tx.Commit()
}

var approvedPublicIncidentMessages = map[IncidentPhase]map[string]struct{}{
	IncidentPhaseInvestigating: {"我们正在调查部分服务异常，用户暂时无需进行额外操作。": {}},
	IncidentPhaseIdentified:    {"我们已经定位到服务异常，正在进行处理。": {}},
	IncidentPhaseMitigating:    {"部分服务仍受到影响，我们正在继续处理。": {}},
	IncidentPhaseMonitoring:    {"相关服务正在恢复，我们将继续观察。": {}},
	IncidentPhaseResolved:      {"相关服务已经恢复，用户无需进行额外操作。": {}},
}

func validatePublicIncidentCopy(phase IncidentPhase, message string) error {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 1000 {
		return fmt.Errorf("public incident message is required and must be within 1000 characters")
	}
	if _, approved := approvedPublicIncidentMessages[phase][message]; !approved {
		return fmt.Errorf("public incident message must use an approved sanitized template")
	}
	return nil
}

func (s *IncidentControlService) PublishUpdate(ctx context.Context, command IncidentPublishCommand) error {
	command.Message = strings.TrimSpace(command.Message)
	if s == nil || s.db == nil || command.IncidentID <= 0 {
		return fmt.Errorf("incident is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var phase string
	var evidenceGap bool
	if err := tx.QueryRowContext(ctx, `SELECT phase,evidence_gap FROM reliability_incidents WHERE id=$1 FOR UPDATE`, command.IncidentID).Scan(&phase, &evidenceGap); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("incident not found")
		}
		return err
	}
	if evidenceGap && IncidentPhase(phase) == IncidentPhaseResolved {
		return fmt.Errorf("resolved public update is blocked by an incident evidence gap")
	}
	if err := validatePublicIncidentCopy(IncidentPhase(phase), command.Message); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_public_timeline(incident_id,phase,message,published_by_user_id) VALUES($1,$2,$3,NULLIF($4,0))`, command.IncidentID, phase, command.Message, command.ActorUserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(incident_id,action,actor_user_id,reason,after_state) VALUES($1,'publish_public_update',NULLIF($2,0),'','{}')`, command.IncidentID, command.ActorUserID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *IncidentControlService) AcknowledgeEvidenceGap(ctx context.Context, command IncidentEvidenceGapCommand) error {
	command.Reason = strings.TrimSpace(command.Reason)
	if s == nil || s.db == nil || command.IncidentID <= 0 || command.Reason == "" || len(command.Reason) > 500 {
		return fmt.Errorf("incident and evidence-gap review reason are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('reliability-incident-control'))`); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE reliability_incidents SET evidence_gap=FALSE,updated_by_user_id=NULLIF($2,0),version=version+1,updated_at=NOW() WHERE id=$1 AND phase<>'resolved' AND evidence_gap=TRUE`, command.IncidentID, command.ActorUserID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		return fmt.Errorf("active incident evidence gap not found")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(incident_id,action,actor_user_id,reason,before_state,after_state) VALUES($1,'acknowledge_evidence_gap',NULLIF($2,0),$3,'{"evidence_gap":true}','{"evidence_gap":false}')`, command.IncidentID, command.ActorUserID, command.Reason); err != nil {
		return err
	}
	return tx.Commit()
}
