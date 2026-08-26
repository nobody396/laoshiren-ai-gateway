package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

type incidentStatusProduct struct {
	ID              int64
	FamilyID        int64
	Code            string
	DisplayName     string
	Status          ServiceStatus
	EvidenceAt      time.Time
	MonitoringSince time.Time
}

type incidentProductEvidence struct {
	Failures               []IncidentCustomerFailure
	AbnormalStatusFailures []IncidentCustomerFailure
	Recoveries             []IncidentCustomerFailure
}

func (s *IncidentControlService) ReconcileAt(ctx context.Context, now time.Time) error {
	if s == nil || !s.settingEnabled(ctx, SettingKeyReliabilityIncidentsEnabled) {
		return nil
	}
	if s.status == nil || !s.status.settingEnabled(ctx, SettingKeyServiceStatusEnabled) {
		return nil
	}
	if s.db == nil || s.status == nil || s.evidence == nil || now.IsZero() {
		return fmt.Errorf("incident reconciliation dependencies are unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	products, err := s.loadStatusProducts(ctx)
	if err != nil {
		return err
	}
	evidenceStart, evidenceGap, err := s.incidentEvidenceStart(ctx, now)
	if err != nil {
		return err
	}
	evidenceByProduct, err := s.loadIncidentProductEvidence(ctx, evidenceStart, now)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='15s'`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('reliability-incident-control'))`); err != nil {
		return err
	}
	if evidenceGap {
		if err := markIncidentEvidenceGap(ctx, tx, now.Add(-7*24*time.Hour), now); err != nil {
			return err
		}
	}
	activeIDs, err := loadActiveIncidentIDs(ctx, tx)
	if err != nil {
		return err
	}
	productIncident, err := loadActiveIncidentProductMap(ctx, tx)
	if err != nil {
		return err
	}
	abnormal := make([]incidentStatusProduct, 0)
	for _, product := range products {
		if incidentStatusIsCustomerAbnormal(product.Status) {
			abnormal = append(abnormal, product)
		}
	}
	recentIncidentByFamily, err := loadRecentIncidentFamilyMap(ctx, tx, now.Add(-incidentCandidateJoinWindow))
	if err != nil {
		return err
	}
	candidateProductsByFamily := map[int64][]incidentStatusProduct{}
	for _, product := range abnormal {
		if _, exists := productIncident[product.ID]; exists {
			continue
		}
		if correlation := recentIncidentByFamily[product.FamilyID]; correlation.IncidentID > 0 {
			if err := attachIncidentProduct(ctx, tx, correlation.IncidentID, product, evidenceByProduct[product.ID], correlation.EventFloor, now); err != nil {
				return err
			}
			continue
		}
		candidateProductsByFamily[product.FamilyID] = append(candidateProductsByFamily[product.FamilyID], product)
	}
	for familyID, familyProducts := range candidateProductsByFamily {
		if err := reconcileIncidentCandidate(ctx, tx, familyID, familyProducts, evidenceByProduct, now); err != nil {
			return err
		}
	}
	if err := recoverInactiveCandidates(ctx, tx, products, now); err != nil {
		return err
	}
	for _, incidentID := range activeIDs {
		if err := s.reconcileIncidentTx(ctx, tx, incidentID, products, evidenceByProduct, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if s.tierSnapshotter != nil && s.tierSnapshotter.CustomerTierSnapshotsEnabled(ctx) {
		if err := s.tierSnapshotter.EnsureIncidentSnapshots(ctx, 0); err != nil {
			return fmt.Errorf("freeze incident customer tiers: %w", err)
		}
	}
	return nil
}

func (s *IncidentControlService) loadStatusProducts(ctx context.Context) ([]incidentStatusProduct, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.id,p.family_id,p.code,p.display_name,COALESCE(c.computed_status,'monitoring'),c.evidence_at,c.monitoring_since
FROM service_status_products p
LEFT JOIN service_status_current c ON c.product_id=p.id
WHERE p.enabled=TRUE
ORDER BY p.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []incidentStatusProduct{}
	for rows.Next() {
		var item incidentStatusProduct
		var status string
		var evidenceAt, monitoringSince sql.NullTime
		if err := rows.Scan(&item.ID, &item.FamilyID, &item.Code, &item.DisplayName, &status, &evidenceAt, &monitoringSince); err != nil {
			return nil, err
		}
		item.Status = ServiceStatus(status)
		if evidenceAt.Valid {
			item.EvidenceAt = evidenceAt.Time.UTC()
		}
		if monitoringSince.Valid {
			item.MonitoringSince = monitoringSince.Time.UTC()
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *IncidentControlService) incidentEvidenceStart(ctx context.Context, now time.Time) (time.Time, bool, error) {
	var start sql.NullTime
	err := s.db.QueryRowContext(ctx, `
SELECT MIN(start_at) FROM (
 SELECT COALESCE(last_reconciled_at,observation_started_at) AS start_at FROM reliability_incidents WHERE phase<>'resolved'
 UNION ALL
 SELECT COALESCE(last_reconciled_at,first_observed_at) FROM reliability_incident_candidates WHERE state='open'
) windows`).Scan(&start)
	if err != nil {
		return time.Time{}, false, err
	}
	result := now.Add(-incidentEvidenceLookback)
	if start.Valid && start.Time.Before(result) {
		result = start.Time.UTC()
	}
	if result.Before(now.Add(-7 * 24 * time.Hour)) {
		return now.Add(-7 * 24 * time.Hour), true, nil
	}
	return result, false, nil
}

func markIncidentEvidenceGap(ctx context.Context, tx *sql.Tx, cutoff, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM reliability_incidents WHERE phase<>'resolved' AND evidence_gap=FALSE AND COALESCE(last_reconciled_at,observation_started_at)<$1 FOR UPDATE`, cutoff)
	if err != nil {
		return err
	}
	ids := []int64{}
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
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE reliability_incidents SET evidence_gap=TRUE,updated_at=$2 WHERE id=$1`, id, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(incident_id,action,reason,after_state) VALUES($1,'evidence_gap','reconciliation resumed after more than seven days','{"evidence_gap":true}')`, id); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE reliability_incident_candidates SET state='recovered',dismissed_reason='evidence_window_expired',dismissed_at=$2,updated_at=$2 WHERE state='open' AND COALESCE(last_reconciled_at,first_observed_at)<$1`, cutoff, now)
	return err
}

func (s *IncidentControlService) loadIncidentProductEvidence(ctx context.Context, start, now time.Time) (map[int64]incidentProductEvidence, error) {
	definitions, err := s.status.loadStatusDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	if len(definitions) == 0 {
		return map[int64]incidentProductEvidence{}, nil
	}
	end := now.Add(-statusEvidenceTailLag)
	if !start.Before(end) {
		return map[int64]incidentProductEvidence{}, nil
	}
	pendingCutoff := s.evidence.publishedPendingID.Load()
	all := make(map[string]*ReliabilityObservation)
	completeness := ReliabilityEvidenceCompleteness{}
	hasSnapshot := false
	impact := true
	for _, product := range definitions {
		for _, component := range product.Components {
			queryProduct := product
			queryProduct.Components = []statusComponentDefinition{component}
			failureQuery := statusEvidenceQueryForProduct(queryProduct, start, end)
			failureQuery.FactTypes = []ReliabilityFactType{ReliabilityFactCustomerRequest}
			failureQuery.Outcomes = []ReliabilityOutcome{ReliabilityOutcomeFailure}
			failureQuery.CustomerImpact = &impact
			observations, partCompleteness, pageErr := s.pageIncidentEvidence(ctx, failureQuery, pendingCutoff)
			if pageErr != nil {
				return nil, pageErr
			}
			completeness, hasSnapshot = mergeIncidentCompleteness(completeness, partCompleteness, hasSnapshot)
			for _, observation := range observations {
				all[observation.IdempotencyKey] = observation
			}

			recoveryQuery := statusEvidenceQueryForProduct(queryProduct, start, end)
			recoveryQuery.FactTypes = []ReliabilityFactType{ReliabilityFactCustomerRequest, ReliabilityFactActiveProbe}
			recoveryQuery.Outcomes = []ReliabilityOutcome{ReliabilityOutcomeSuccess, ReliabilityOutcomeRecovered}
			observations, partCompleteness, pageErr = s.pageIncidentEvidence(ctx, recoveryQuery, pendingCutoff)
			if pageErr != nil {
				return nil, pageErr
			}
			completeness, hasSnapshot = mergeIncidentCompleteness(completeness, partCompleteness, hasSnapshot)
			for _, observation := range observations {
				all[observation.IdempotencyKey] = observation
			}

			probeFailureQuery := statusEvidenceQueryForProduct(queryProduct, start, end)
			probeFailureQuery.FactTypes = []ReliabilityFactType{ReliabilityFactActiveProbe}
			probeFailureQuery.Outcomes = []ReliabilityOutcome{ReliabilityOutcomeFailure}
			observations, partCompleteness, pageErr = s.pageIncidentEvidence(ctx, probeFailureQuery, pendingCutoff)
			if pageErr != nil {
				return nil, pageErr
			}
			completeness, hasSnapshot = mergeIncidentCompleteness(completeness, partCompleteness, hasSnapshot)
			for _, observation := range observations {
				all[observation.IdempotencyKey] = observation
			}
		}
	}
	if hasSnapshot && !s.status.evidenceReadyAt(completeness, now) {
		return nil, fmt.Errorf("incident evidence is incomplete")
	}
	componentProduct := make(map[int64]int64)
	for _, product := range definitions {
		for _, component := range product.Components {
			componentProduct[component.ID] = product.ID
		}
	}
	result := make(map[int64]incidentProductEvidence)
	seenProductObservation := make(map[int64]map[int64]struct{})
	for _, observation := range all {
		if observation == nil || observation.ID <= 0 {
			continue
		}
		if observation.FactType == ReliabilityFactCustomerRequest && observation.CustomerImpact && (observation.UserID == nil || *observation.UserID <= 0) {
			// Preserve historical raw evidence, but do not let an unowned request
			// enter an Incident or compensation snapshot.
			continue
		}
		for _, componentID := range matchingStatusComponentIDs(definitions, observation, false) {
			productID := componentProduct[componentID]
			if productID <= 0 {
				continue
			}
			item := result[productID]
			if seenProductObservation[productID] == nil {
				seenProductObservation[productID] = map[int64]struct{}{}
			}
			if _, exists := seenProductObservation[productID][observation.ID]; exists {
				continue
			}
			seenProductObservation[productID][observation.ID] = struct{}{}
			if observation.FactType == ReliabilityFactCustomerRequest && observation.Outcome == ReliabilityOutcomeFailure && observation.CustomerImpact {
				item.Failures = append(item.Failures, IncidentCustomerFailure{ObservationID: observation.ID, ObservedAt: observation.ObservedAt.UTC()})
			} else if observation.FactType == ReliabilityFactActiveProbe && observation.Outcome == ReliabilityOutcomeFailure {
				item.AbnormalStatusFailures = append(item.AbnormalStatusFailures, IncidentCustomerFailure{ObservationID: observation.ID, ObservedAt: observation.ObservedAt.UTC()})
			} else if observation.Outcome == ReliabilityOutcomeSuccess || observation.Outcome == ReliabilityOutcomeRecovered {
				item.Recoveries = append(item.Recoveries, IncidentCustomerFailure{ObservationID: observation.ID, ObservedAt: observation.ObservedAt.UTC()})
			}
			result[productID] = item
		}
	}
	for productID, item := range result {
		sort.Slice(item.Failures, func(i, j int) bool { return item.Failures[i].ObservedAt.Before(item.Failures[j].ObservedAt) })
		sort.Slice(item.AbnormalStatusFailures, func(i, j int) bool {
			return item.AbnormalStatusFailures[i].ObservedAt.Before(item.AbnormalStatusFailures[j].ObservedAt)
		})
		sort.Slice(item.Recoveries, func(i, j int) bool { return item.Recoveries[i].ObservedAt.Before(item.Recoveries[j].ObservedAt) })
		result[productID] = item
	}
	return result, nil
}

func (s *IncidentControlService) pageIncidentEvidence(ctx context.Context, base *ReliabilityEvidenceQuery, pendingCutoff uint64) ([]*ReliabilityObservation, ReliabilityEvidenceCompleteness, error) {
	const pageSize = 1000
	query := *base
	query.Limit = pageSize
	result := []*ReliabilityObservation{}
	completeness := ReliabilityEvidenceCompleteness{}
	hasSnapshot := false
	for {
		snapshot, err := s.evidence.snapshotAtPendingCutoff(ctx, &query, pendingCutoff)
		if err != nil {
			return nil, completeness, err
		}
		completeness, hasSnapshot = mergeIncidentCompleteness(completeness, snapshot.Completeness, hasSnapshot)
		result = append(result, snapshot.Observations...)
		if len(snapshot.Observations) < pageSize {
			break
		}
		last := snapshot.Observations[len(snapshot.Observations)-1]
		if last == nil || last.ID <= 0 || last.ObservedAt.IsZero() {
			return nil, completeness, fmt.Errorf("incident evidence cursor is invalid")
		}
		cursorAt := last.ObservedAt.UTC()
		query.BeforeObservedAt = &cursorAt
		query.BeforeID = last.ID
	}
	return result, completeness, nil
}

func mergeIncidentCompleteness(current, next ReliabilityEvidenceCompleteness, hasCurrent bool) (ReliabilityEvidenceCompleteness, bool) {
	if !hasCurrent {
		return next, true
	}
	return mergeReliabilityCompletenessConservative(current, next), true
}

func reconcileIncidentCandidate(ctx context.Context, tx *sql.Tx, familyID int64, abnormal []incidentStatusProduct, evidence map[int64]incidentProductEvidence, now time.Time) error {
	var candidateID int64
	created := false
	err := tx.QueryRowContext(ctx, `
SELECT c.id FROM reliability_incident_candidates c
WHERE c.state='open' AND c.last_observed_at >= $1 AND EXISTS (
 SELECT 1 FROM reliability_incident_candidate_products cp JOIN service_status_products p ON p.id=cp.product_id
 WHERE cp.candidate_id=c.id AND p.family_id=$2
)
ORDER BY c.last_observed_at DESC,c.id DESC LIMIT 1 FOR UPDATE`, now.Add(-incidentCandidateJoinWindow), familyID).Scan(&candidateID)
	if err == sql.ErrNoRows {
		created = true
		key := incidentCandidateKey(now, familyID, abnormal)
		candidateStart := now
		for _, product := range abnormal {
			if !product.EvidenceAt.IsZero() && product.EvidenceAt.Before(candidateStart) {
				candidateStart = product.EvidenceAt
			}
			if values := evidence[product.ID].Failures; len(values) > 0 && values[0].ObservedAt.Before(candidateStart) {
				candidateStart = values[0].ObservedAt
			}
			if values := evidence[product.ID].AbnormalStatusFailures; len(values) > 0 && values[0].ObservedAt.Before(candidateStart) {
				candidateStart = values[0].ObservedAt
			}
		}
		err = tx.QueryRowContext(ctx, `
INSERT INTO reliability_incident_candidates(candidate_key,state,first_observed_at,last_observed_at)
VALUES($1,'open',$2,$3)
ON CONFLICT(candidate_key) DO UPDATE SET last_observed_at=GREATEST(reliability_incident_candidates.last_observed_at,EXCLUDED.last_observed_at),updated_at=NOW()
RETURNING id`, key, candidateStart, now).Scan(&candidateID)
	}
	if err != nil {
		return err
	}
	if created {
		if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(candidate_id,action,reason,after_state) VALUES($1,'create_candidate','qualified computed status','{"state":"open"}')`, candidateID); err != nil {
			return err
		}
	}
	for _, product := range abnormal {
		firstObserved := product.EvidenceAt
		if firstObserved.IsZero() {
			firstObserved = now
		}
		var firstFailure any
		if values := evidence[product.ID].Failures; len(values) > 0 {
			firstFailure = values[0].ObservedAt
			if values[0].ObservedAt.Before(firstObserved) {
				firstObserved = values[0].ObservedAt
			}
		}
		if values := evidence[product.ID].AbnormalStatusFailures; len(values) > 0 && values[0].ObservedAt.Before(firstObserved) {
			firstObserved = values[0].ObservedAt
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO reliability_incident_candidate_products(candidate_id,product_id,first_observed_at,last_observed_at,latest_status,first_customer_failure_at)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(candidate_id,product_id) DO UPDATE SET
 last_observed_at=GREATEST(reliability_incident_candidate_products.last_observed_at,EXCLUDED.last_observed_at),
 latest_status=EXCLUDED.latest_status,
 first_customer_failure_at=COALESCE(reliability_incident_candidate_products.first_customer_failure_at,EXCLUDED.first_customer_failure_at),updated_at=NOW()`,
			candidateID, product.ID, firstObserved, now, string(product.Status), firstFailure); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE reliability_incident_candidates SET first_observed_at=LEAST(first_observed_at,(SELECT MIN(first_observed_at) FROM reliability_incident_candidate_products WHERE candidate_id=$1)),last_observed_at=$2,last_reconciled_at=$2::timestamptz-INTERVAL '2 seconds',updated_at=NOW() WHERE id=$1`, candidateID, now)
	return err
}

func incidentCandidateKey(now time.Time, familyID int64, products []incidentStatusProduct) string {
	ids := make([]string, 0, len(products))
	for _, product := range products {
		ids = append(ids, fmt.Sprintf("%d", product.ID))
	}
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, ",")))
	return fmt.Sprintf("status:%d:%d:%s", familyID, now.UnixNano(), hex.EncodeToString(sum[:6]))
}

type incidentCorrelation struct {
	IncidentID int64
	EventFloor time.Time
}

func loadRecentIncidentFamilyMap(ctx context.Context, tx *sql.Tx, cutoff time.Time) (map[int64]incidentCorrelation, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT DISTINCT ON(p.family_id) p.family_id,i.id,i.observation_started_at
FROM reliability_incidents i
JOIN reliability_incident_products ip ON ip.incident_id=i.id
JOIN service_status_products p ON p.id=ip.product_id
WHERE i.phase<>'resolved' AND i.observation_started_at >= $1
ORDER BY p.family_id,i.updated_at DESC,i.id DESC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := map[int64]incidentCorrelation{}
	for rows.Next() {
		var familyID, incidentID int64
		var observationStarted time.Time
		if err := rows.Scan(&familyID, &incidentID, &observationStarted); err != nil {
			return nil, err
		}
		eventFloor := cutoff
		if observationStarted.After(eventFloor) {
			eventFloor = observationStarted
		}
		result[familyID] = incidentCorrelation{IncidentID: incidentID, EventFloor: eventFloor.UTC()}
	}
	return result, rows.Err()
}

func loadActiveIncidentIDs(ctx context.Context, tx *sql.Tx) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM reliability_incidents WHERE phase<>'resolved' ORDER BY updated_at DESC,id DESC FOR UPDATE`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func loadActiveIncidentProductMap(ctx context.Context, tx *sql.Tx) (map[int64]int64, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT ip.product_id,ip.incident_id FROM reliability_incident_products ip
JOIN reliability_incidents i ON i.id=ip.incident_id WHERE i.phase<>'resolved'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := map[int64]int64{}
	for rows.Next() {
		var productID, incidentID int64
		if err := rows.Scan(&productID, &incidentID); err != nil {
			return nil, err
		}
		result[productID] = incidentID
	}
	return result, rows.Err()
}

func attachIncidentProduct(ctx context.Context, tx *sql.Tx, incidentID int64, product incidentStatusProduct, evidence incidentProductEvidence, eventFloor, now time.Time) error {
	affectedAt := incidentProductEvidenceStart(product, evidence, eventFloor, now)
	_, err := tx.ExecContext(ctx, `
INSERT INTO reliability_incident_products(incident_id,product_id,affected_at,current_status)
VALUES($1,$2,$3,$4) ON CONFLICT(incident_id,product_id) DO NOTHING`, incidentID, product.ID, affectedAt, string(product.Status))
	return err
}

func incidentProductEvidenceStart(product incidentStatusProduct, evidence incidentProductEvidence, eventFloor, fallback time.Time) time.Time {
	result := fallback
	if !product.EvidenceAt.IsZero() && !product.EvidenceAt.Before(eventFloor) && product.EvidenceAt.Before(result) {
		result = product.EvidenceAt
	}
	for _, observation := range append(append([]IncidentCustomerFailure{}, evidence.Failures...), evidence.AbnormalStatusFailures...) {
		if !observation.ObservedAt.Before(eventFloor) && observation.ObservedAt.Before(result) {
			result = observation.ObservedAt
		}
	}
	return result
}

func recoverInactiveCandidates(ctx context.Context, tx *sql.Tx, products []incidentStatusProduct, now time.Time) error {
	statusByProduct := map[int64]ServiceStatus{}
	for _, product := range products {
		statusByProduct[product.ID] = product.Status
	}
	rows, err := tx.QueryContext(ctx, `
SELECT c.id,cp.product_id FROM reliability_incident_candidates c
JOIN reliability_incident_candidate_products cp ON cp.candidate_id=c.id
WHERE c.state='open' ORDER BY c.id,cp.product_id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	active := map[int64]bool{}
	seen := map[int64]bool{}
	for rows.Next() {
		var candidateID, productID int64
		if err := rows.Scan(&candidateID, &productID); err != nil {
			return err
		}
		seen[candidateID] = true
		if incidentStatusIsCustomerAbnormal(statusByProduct[productID]) {
			active[candidateID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for candidateID := range seen {
		if active[candidateID] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE reliability_incident_candidates SET state='recovered',dismissed_reason='status_recovered_before_confirmation',dismissed_at=$2,updated_at=$2 WHERE id=$1 AND state='open'`, candidateID, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *IncidentControlService) reconcileIncidentTx(ctx context.Context, tx *sql.Tx, incidentID int64, statuses []incidentStatusProduct, evidence map[int64]incidentProductEvidence, now time.Time) error {
	state, productAffectedAt, err := loadIncidentLifecycleState(ctx, tx, incidentID)
	if err != nil {
		return err
	}
	statusByProduct := map[int64]incidentStatusProduct{}
	for _, product := range statuses {
		statusByProduct[product.ID] = product
	}
	signals := make([]IncidentProductSignal, 0, len(state.Products))
	usedFailures := map[int64][]IncidentCustomerFailure{}
	usedRecovery := map[int64]*IncidentCustomerFailure{}
	for _, product := range state.Products {
		status, exists := statusByProduct[product.ProductID]
		if !exists {
			continue
		}
		threshold := productAffectedAt[product.ProductID]
		hasPriorFailure := !product.LastCustomerFailureAt.IsZero()
		if hasPriorFailure && product.LastCustomerFailureAt.After(threshold) {
			threshold = product.LastCustomerFailureAt
		}
		productEvidence := evidence[product.ProductID]
		for _, failure := range productEvidence.Failures {
			if failure.ObservedAt.After(threshold) || (!hasPriorFailure && failure.ObservedAt.Equal(threshold)) {
				usedFailures[product.ProductID] = append(usedFailures[product.ProductID], failure)
			}
		}
		if status.Status == ServiceStatusMonitoring || status.Status == ServiceStatusOperational {
			recoveryCeiling := status.MonitoringSince
			if recoveryCeiling.IsZero() {
				recoveryCeiling = now
			}
			recoveryFloor := recoveryCeiling.Add(-5 * time.Minute)
			if affectedAt := productAffectedAt[product.ProductID]; affectedAt.After(recoveryFloor) {
				recoveryFloor = affectedAt
			}
			for index := len(productEvidence.Recoveries) - 1; index >= 0; index-- {
				recovery := productEvidence.Recoveries[index]
				if !recovery.ObservedAt.Before(recoveryFloor) && !recovery.ObservedAt.After(recoveryCeiling) {
					value := recovery
					usedRecovery[product.ProductID] = &value
					break
				}
			}
			if (incidentStatusIsCustomerAbnormal(product.CurrentStatus) || hasOpenIncidentSegment(product.Segments)) && usedRecovery[product.ProductID] == nil {
				return fmt.Errorf("incident product %d recovery evidence is unavailable", product.ProductID)
			}
		}
		signals = append(signals, IncidentProductSignal{ProductID: product.ProductID, Status: status.Status, MonitoringSince: status.MonitoringSince, RecoveryObserved: usedRecovery[product.ProductID], CustomerFailures: usedFailures[product.ProductID]})
	}
	next, err := EvaluateIncidentLifecycle(state, IncidentLifecycleInput{Now: now, Products: signals})
	if err != nil {
		return err
	}
	for _, product := range next.Products {
		var monitoring, recovered, lastFailure any
		if !product.MonitoringSince.IsZero() {
			monitoring = product.MonitoringSince
		}
		if !product.RecoveredAt.IsZero() {
			recovered = product.RecoveredAt
		}
		if !product.LastCustomerFailureAt.IsZero() {
			lastFailure = product.LastCustomerFailureAt
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE reliability_incident_products SET current_status=$2,monitoring_since=$3,recovered_at=$4,last_customer_failure_at=$5,updated_at=$6 WHERE id=$1`,
			product.ID, string(product.CurrentStatus), monitoring, recovered, lastFailure, now); err != nil {
			return err
		}
		for _, segment := range product.Segments {
			if segment.ID > 0 {
				if _, err := tx.ExecContext(ctx, `UPDATE reliability_incident_impact_segments SET ended_at=$2::timestamptz,end_observation_id=NULLIF($3,0),close_reason=CASE WHEN $2::timestamptz IS NULL THEN close_reason ELSE 'monitoring' END,updated_at=$4 WHERE id=$1`, segment.ID, segment.EndedAt, segment.EndObservationID, now); err != nil {
					return err
				}
			} else {
				if _, err := tx.ExecContext(ctx, `
INSERT INTO reliability_incident_impact_segments(incident_id,incident_product_id,started_at,ended_at,start_observation_id,end_observation_id,close_reason)
VALUES($1,$2,$3,$4::timestamptz,NULLIF($5,0),NULLIF($6,0),CASE WHEN $4::timestamptz IS NULL THEN '' ELSE 'monitoring' END)`, incidentID, product.ID, segment.StartedAt, segment.EndedAt, segment.StartObservationID, segment.EndObservationID); err != nil {
					return err
				}
			}
		}
		failureIDs := make([]int64, 0, len(usedFailures[product.ProductID]))
		for _, failure := range usedFailures[product.ProductID] {
			failureIDs = append(failureIDs, failure.ObservationID)
		}
		if len(failureIDs) > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_observation_links(incident_id,observation_id,product_id,relation) SELECT $1,id,$3,'customer_impact' FROM unnest($2::bigint[]) id ON CONFLICT DO NOTHING`, incidentID, pq.Array(failureIDs), product.ProductID); err != nil {
				return err
			}
		}
		statusIDs := []int64{}
		for index, failure := range usedFailures[product.ProductID] {
			if index >= 3 {
				break
			}
			statusIDs = append(statusIDs, failure.ObservationID)
		}
		probeEvidenceAdded := 0
		for _, failure := range evidence[product.ProductID].AbnormalStatusFailures {
			if !failure.ObservedAt.Before(productAffectedAt[product.ProductID]) && !failure.ObservedAt.After(now) {
				statusIDs = append(statusIDs, failure.ObservationID)
				probeEvidenceAdded++
				if probeEvidenceAdded >= 3 {
					break
				}
			}
		}
		if recovery := usedRecovery[product.ProductID]; recovery != nil {
			statusIDs = append(statusIDs, recovery.ObservationID)
		}
		if len(statusIDs) > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_observation_links(incident_id,observation_id,product_id,relation) SELECT $1,id,$3,'status_evidence' FROM unnest($2::bigint[]) id ON CONFLICT DO NOTHING`, incidentID, pq.Array(statusIDs), product.ProductID); err != nil {
				return err
			}
		}
		if recovery := usedRecovery[product.ProductID]; recovery != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_observation_links(incident_id,observation_id,product_id,relation) VALUES($1,$2,$3,'recovery') ON CONFLICT DO NOTHING`, incidentID, recovery.ObservationID, product.ProductID); err != nil {
				return err
			}
		}
	}
	var monitoring, resolved any
	if !next.MonitoringSince.IsZero() {
		monitoring = next.MonitoringSince
	}
	if next.ResolvedAt != nil {
		resolved = *next.ResolvedAt
	}
	if state.Phase != next.Phase {
		message := fmt.Sprintf("incident phase changed automatically from %s to %s", state.Phase, next.Phase)
		if next.CustomerImpactRelapsed {
			message = "customer impact relapsed during Monitoring; opened a new impact segment"
		} else if next.Relapsed {
			message = "computed service status relapsed during Monitoring; reopened investigation without adding customer-impact duration"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_updates(incident_id,phase,kind,internal_message) VALUES($1,$2,'system',$3)`, incidentID, string(next.Phase), message); err != nil {
			return err
		}
		before, _ := json.Marshal(map[string]any{"phase": state.Phase})
		after, _ := json.Marshal(map[string]any{"phase": next.Phase, "relapsed": next.Relapsed})
		if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_audit_log(incident_id,action,reason,before_state,after_state) VALUES($1,'automatic_reconcile',$2,$3,$4)`, incidentID, message, before, after); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `
UPDATE reliability_incidents SET phase=$2,monitoring_since=$3,resolved_at=$4::timestamptz,
 observation_ended_at=CASE WHEN $4::timestamptz IS NULL THEN observation_ended_at ELSE $4::timestamptz END,
 customer_impact_started_at=(SELECT MIN(started_at) FROM reliability_incident_impact_segments WHERE incident_id=$1),
 customer_impact_ended_at=CASE WHEN $4::timestamptz IS NULL THEN customer_impact_ended_at ELSE (SELECT MAX(ended_at) FROM reliability_incident_impact_segments WHERE incident_id=$1) END,
 last_reconciled_at=$5::timestamptz-INTERVAL '2 seconds',version=version+1,updated_at=$5 WHERE id=$1`, incidentID, string(next.Phase), monitoring, resolved, now)
	return err
}

func loadIncidentLifecycleState(ctx context.Context, tx *sql.Tx, incidentID int64) (IncidentLifecycleState, map[int64]time.Time, error) {
	var phase string
	var monitoring sql.NullTime
	state := IncidentLifecycleState{}
	if err := tx.QueryRowContext(ctx, `SELECT phase,monitoring_since,evidence_gap FROM reliability_incidents WHERE id=$1 FOR UPDATE`, incidentID).Scan(&phase, &monitoring, &state.EvidenceGap); err != nil {
		return state, nil, err
	}
	state.Phase = IncidentPhase(phase)
	if monitoring.Valid {
		state.MonitoringSince = monitoring.Time.UTC()
	}
	rows, err := tx.QueryContext(ctx, `
SELECT ip.id,ip.product_id,ip.current_status,ip.affected_at,ip.monitoring_since,ip.recovered_at,ip.last_customer_failure_at,
 s.id,s.started_at,s.ended_at,s.start_observation_id,s.end_observation_id
FROM reliability_incident_products ip
LEFT JOIN reliability_incident_impact_segments s ON s.incident_product_id=ip.id
WHERE ip.incident_id=$1 ORDER BY ip.id,s.started_at,s.id`, incidentID)
	if err != nil {
		return state, nil, err
	}
	defer func() { _ = rows.Close() }()
	index := map[int64]int{}
	affected := map[int64]time.Time{}
	for rows.Next() {
		var productID, productRowID int64
		var status string
		var affectedAt time.Time
		var productMonitoring, recoveredAt, lastFailure sql.NullTime
		var segmentID, startObservationID, endObservationID sql.NullInt64
		var segmentStart, segmentEnd sql.NullTime
		if err := rows.Scan(&productRowID, &productID, &status, &affectedAt, &productMonitoring, &recoveredAt, &lastFailure, &segmentID, &segmentStart, &segmentEnd, &startObservationID, &endObservationID); err != nil {
			return state, nil, err
		}
		position, exists := index[productRowID]
		if !exists {
			position = len(state.Products)
			index[productRowID] = position
			product := IncidentProductLifecycle{ID: productRowID, ProductID: productID, CurrentStatus: ServiceStatus(status)}
			if productMonitoring.Valid {
				product.MonitoringSince = productMonitoring.Time.UTC()
			}
			if recoveredAt.Valid {
				product.RecoveredAt = recoveredAt.Time.UTC()
			}
			if lastFailure.Valid {
				product.LastCustomerFailureAt = lastFailure.Time.UTC()
			}
			state.Products = append(state.Products, product)
			affected[productID] = affectedAt.UTC()
		}
		if segmentID.Valid {
			segment := IncidentImpactSegment{ID: segmentID.Int64, StartedAt: segmentStart.Time.UTC(), StartObservationID: startObservationID.Int64, EndObservationID: endObservationID.Int64}
			if segmentEnd.Valid {
				value := segmentEnd.Time.UTC()
				segment.EndedAt = &value
			}
			state.Products[position].Segments = append(state.Products[position].Segments, segment)
		}
	}
	return state, affected, rows.Err()
}
