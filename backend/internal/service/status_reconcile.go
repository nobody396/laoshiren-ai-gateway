package service

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
)

const (
	statusEvaluationInterval = 30 * time.Second
	statusEvidenceTailLag    = time.Second
)

type statusBinding struct {
	ID               int64
	GroupID          *int64
	GroupName        string
	Platform         string
	ModelPattern     string
	RouteFingerprint string
	AccountIDs       map[int64]struct{}
}

type statusComponentDefinition struct {
	ID                  int64
	ProductID           int64
	Code                string
	ModelPattern        string
	AccessMode          string
	Previous            ServiceStatus
	MonitoringSince     time.Time
	RecoveryConfirmedAt time.Time
	Bindings            []statusBinding
}

type statusProductDefinition struct {
	ID         int64
	Code       string
	Critical   bool
	Components []statusComponentDefinition
}

type evaluatedStatusComponent struct {
	Definition          statusComponentDefinition
	Evaluation          StatusEvaluation
	EvidenceAt          *time.Time
	MonitoringSince     time.Time
	RecoveryConfirmedAt time.Time
}

func (s *StatusControlService) Name() string { return "status-control" }

func (s *StatusControlService) Start(context.Context) error {
	if s == nil || !s.settingEnabled(context.Background(), SettingKeyServiceStatusEnabled) {
		return nil
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return nil
	}
	if s.evidence != nil {
		s.readinessBaseline = s.evidence.Completeness()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()
	if err := s.Reconcile(ctx); err != nil {
		cancel()
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
		return err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(statusEvaluationInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Reconcile(ctx); err != nil && ctx.Err() == nil {
					logger.LegacyPrintf("service.status_control", "status reconciliation failed: %v", err)
				}
			}
		}
	}()
	return nil
}

func (s *StatusControlService) Stop(context.Context) error {
	if s == nil {
		return nil
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		s.wg.Wait()
	}
	return nil
}

func (s *StatusControlService) evidenceReadyAt(completeness ReliabilityEvidenceCompleteness, now time.Time) bool {
	s.mu.Lock()
	baseline := s.readinessBaseline
	incompleteUntil := s.incompleteUntil
	s.mu.Unlock()
	return completeness.Enabled && completeness.Running &&
		completeness.Dropped <= baseline.Dropped && completeness.Failed <= baseline.Failed &&
		!now.Before(incompleteUntil) &&
		(completeness.OldestPendingAt == nil || completeness.OldestPendingAt.After(now.Add(-statusEvidenceTailLag)))
}

func (s *StatusControlService) Reconcile(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	if !s.settingEnabled(ctx, SettingKeyServiceStatusEnabled) {
		return nil
	}
	if s.evidence == nil || s.db == nil {
		return fmt.Errorf("status evaluation dependencies are unavailable")
	}
	now := time.Now()
	evidenceEnd := now.Add(-statusEvidenceTailLag)
	products, err := s.loadStatusDefinitions(ctx)
	if err != nil {
		return err
	}
	evidence, err := s.loadStatusEvidence(ctx, products, now.Add(-5*time.Minute), evidenceEnd)
	if err != nil {
		return err
	}
	projected := projectStatusObservations(products, evidence.Observations)
	s.mu.Lock()
	baseline := s.readinessBaseline
	s.mu.Unlock()
	newCompletenessFailure := evidence.Completeness.Dropped > baseline.Dropped || evidence.Completeness.Failed > baseline.Failed
	ready := s.evidenceReadyAt(evidence.Completeness, now)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, product := range products {
		evaluated := make([]evaluatedStatusComponent, 0, len(product.Components))
		for _, component := range product.Components {
			observations := projected[component.ID]
			var evidenceAt *time.Time
			for _, observation := range observations {
				if evidenceAt == nil || observation.ObservedAt.After(*evidenceAt) {
					value := observation.ObservedAt
					evidenceAt = &value
				}
			}
			result := StatusEvaluation{Status: ServiceStatusMonitoring, Reason: "evidence_incomplete"}
			if ready {
				result = EvaluateComputedStatus(StatusEvaluationInput{
					Now: now, PreviousStatus: component.Previous, MonitoringSince: component.MonitoringSince,
					CriticalService: product.Critical, RecoveryConfirmedAt: component.RecoveryConfirmedAt, Observations: observations,
				})
			}
			monitoringSince := nextMonitoringSince(component.Previous, component.MonitoringSince, result.Status, result.MonitoringResetAt, now)
			recoveryConfirmedAt := nextRecoveryConfirmedAt(component, result)
			_, err = tx.ExecContext(ctx, `
INSERT INTO service_status_component_current(
 component_id,computed_status,reason,evidence_at,computed_at,monitoring_since,recovery_confirmed_at,
 customer_request_count,customer_success_count,customer_failure_count,
 probe_count,probe_success_count,probe_failure_count,updated_at
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$5)
ON CONFLICT(component_id) DO UPDATE SET
 computed_status=EXCLUDED.computed_status,reason=EXCLUDED.reason,evidence_at=EXCLUDED.evidence_at,
 computed_at=EXCLUDED.computed_at,monitoring_since=EXCLUDED.monitoring_since,recovery_confirmed_at=EXCLUDED.recovery_confirmed_at,
 customer_request_count=EXCLUDED.customer_request_count,customer_success_count=EXCLUDED.customer_success_count,
 customer_failure_count=EXCLUDED.customer_failure_count,probe_count=EXCLUDED.probe_count,
 probe_success_count=EXCLUDED.probe_success_count,probe_failure_count=EXCLUDED.probe_failure_count,
 updated_at=EXCLUDED.updated_at`,
				component.ID, string(result.Status), result.Reason, evidenceAt, now, nullableStatusTime(monitoringSince), nullableStatusTime(recoveryConfirmedAt),
				result.CustomerRequestCount, result.CustomerSuccessCount, result.CustomerFailureCount,
				result.ProbeCount, result.ProbeSuccessCount, result.ProbeFailureCount)
			if err != nil {
				return err
			}
			evaluated = append(evaluated, evaluatedStatusComponent{Definition: component, Evaluation: result, EvidenceAt: evidenceAt, MonitoringSince: monitoringSince, RecoveryConfirmedAt: recoveryConfirmedAt})
		}
		rollup := rollupProductStatus(evaluated)
		effective := rollup.Status
		effectiveReason := rollup.Reason
		var overrideStatus, overrideReason string
		err := tx.QueryRowContext(ctx, `
SELECT status,reason FROM service_status_overrides
WHERE product_id=$1 AND starts_at <= $2 AND expires_at > $2
ORDER BY created_at DESC, id DESC LIMIT 1`, product.ID, now).Scan(&overrideStatus, &overrideReason)
		if err == nil {
			effective = ServiceStatus(overrideStatus)
			effectiveReason = "manual_override"
		} else if err != sql.ErrNoRows {
			return err
		}
		_, err = tx.ExecContext(ctx, `
UPDATE service_status_current SET
 computed_status=$2,effective_status=$3,computed_reason=$4,effective_reason=$5,evidence_at=$6,computed_at=$7,monitoring_since=$8,recovery_confirmed_at=$9,
 customer_request_count=$10,customer_success_count=$11,customer_failure_count=$12,
 probe_count=$13,probe_success_count=$14,probe_failure_count=$15,updated_at=$7
WHERE product_id=$1`, product.ID, string(rollup.Status), string(effective), rollup.Reason, effectiveReason, rollup.EvidenceAt, now,
			nullableStatusTime(rollup.MonitoringSince), nullableStatusTime(rollup.RecoveryConfirmedAt), rollup.CustomerRequestCount, rollup.CustomerSuccessCount,
			rollup.CustomerFailureCount, rollup.ProbeCount, rollup.ProbeSuccessCount, rollup.ProbeFailureCount)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	s.lastSuccessfulReconcile = now
	s.lastReconcileReady = ready
	if newCompletenessFailure {
		// Completeness counters are cumulative. Advancing the baseline after one
		// fail-closed cycle makes readiness epoch-scoped instead of freezing all
		// products forever after one historical ingestion failure.
		s.readinessBaseline = evidence.Completeness
		contaminatedUntil := now.Add(5 * time.Minute)
		if contaminatedUntil.After(s.incompleteUntil) {
			s.incompleteUntil = contaminatedUntil
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *StatusControlService) loadStatusEvidence(ctx context.Context, products []statusProductDefinition, start, end time.Time) (*ReliabilityEvidenceSnapshot, error) {
	result := &ReliabilityEvidenceSnapshot{GeneratedAt: time.Now(), Observations: []*ReliabilityObservation{}}
	hasSnapshot := false
	seen := make(map[string]struct{})
	pendingCutoff := s.evidence.publishedPendingID.Load()
	for _, product := range products {
		for _, component := range product.Components {
			queryProduct := product
			queryProduct.Components = []statusComponentDefinition{component}
			query := statusEvidenceQueryForProduct(queryProduct, start, end)
			if len(query.AnyGroupIDs) == 0 && len(query.AnyAccountIDs) == 0 && len(query.AnyPlatforms) == 0 && len(query.AnyRouteFingerprints) == 0 && len(query.AnyModelPatterns) == 0 {
				continue
			}
			snapshot, err := s.evidence.snapshotAtPendingCutoff(ctx, query, pendingCutoff)
			if err != nil {
				return nil, err
			}
			if !hasSnapshot {
				result.Completeness = snapshot.Completeness
				hasSnapshot = true
			} else {
				result.Completeness = mergeReliabilityCompletenessConservative(result.Completeness, snapshot.Completeness)
			}
			for _, observation := range snapshot.Observations {
				if observation == nil {
					continue
				}
				key := observation.IdempotencyKey
				if key == "" {
					key = fmt.Sprintf("%s|%s|%s|%d", observation.FactType, observation.Source, observation.SourceID, observation.ObservedAt.UnixNano())
				}
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				result.Observations = append(result.Observations, observation)
			}
		}
	}
	if !hasSnapshot {
		result.Completeness = s.evidence.Completeness()
	}
	return result, nil
}

func statusEvidenceQueryForProduct(product statusProductDefinition, start, end time.Time) *ReliabilityEvidenceQuery {
	groups := map[int64]struct{}{}
	accounts := map[int64]struct{}{}
	platforms := map[string]struct{}{}
	routes := map[string]struct{}{}
	models := map[string]struct{}{}
	for _, component := range product.Components {
		if component.ModelPattern != "" {
			models[strings.ToLower(component.ModelPattern)] = struct{}{}
		}
		for _, binding := range component.Bindings {
			if binding.GroupID != nil {
				groups[*binding.GroupID] = struct{}{}
			}
			for accountID := range binding.AccountIDs {
				accounts[accountID] = struct{}{}
			}
			if binding.Platform != "" {
				platforms[strings.ToLower(binding.Platform)] = struct{}{}
			}
			if binding.RouteFingerprint != "" {
				routes[strings.ToLower(binding.RouteFingerprint)] = struct{}{}
			}
			if component.ModelPattern == "" && binding.ModelPattern != "" {
				models[strings.ToLower(binding.ModelPattern)] = struct{}{}
			}
		}
	}
	query := &ReliabilityEvidenceQuery{Start: start, End: end, FactTypes: []ReliabilityFactType{ReliabilityFactCustomerRequest, ReliabilityFactActiveProbe}, Limit: 5000}
	for value := range groups {
		query.AnyGroupIDs = append(query.AnyGroupIDs, value)
	}
	for value := range accounts {
		query.AnyAccountIDs = append(query.AnyAccountIDs, value)
	}
	for value := range platforms {
		query.AnyPlatforms = append(query.AnyPlatforms, value)
	}
	for value := range routes {
		query.AnyRouteFingerprints = append(query.AnyRouteFingerprints, value)
	}
	for value := range models {
		query.AnyModelPatterns = append(query.AnyModelPatterns, value)
	}
	sort.Slice(query.AnyGroupIDs, func(i, j int) bool { return query.AnyGroupIDs[i] < query.AnyGroupIDs[j] })
	sort.Slice(query.AnyAccountIDs, func(i, j int) bool { return query.AnyAccountIDs[i] < query.AnyAccountIDs[j] })
	sort.Strings(query.AnyPlatforms)
	sort.Strings(query.AnyRouteFingerprints)
	sort.Strings(query.AnyModelPatterns)
	return query
}

func nextMonitoringSince(previous ServiceStatus, previousSince time.Time, next ServiceStatus, resetAt time.Time, now time.Time) time.Time {
	if next == ServiceStatusMonitoring {
		if previous == ServiceStatusMonitoring && !previousSince.IsZero() {
			if resetAt.After(previousSince) {
				return resetAt
			}
			return previousSince
		}
		return now
	}
	return time.Time{}
}

func nextRecoveryConfirmedAt(component statusComponentDefinition, evaluation StatusEvaluation) time.Time {
	if evaluation.Status != ServiceStatusMonitoring {
		return time.Time{}
	}
	if !evaluation.RecoveryConfirmedAt.IsZero() {
		return evaluation.RecoveryConfirmedAt
	}
	if evaluation.Reason == "monitoring_observation" {
		return component.RecoveryConfirmedAt
	}
	return time.Time{}
}

func rollupProductStatus(components []evaluatedStatusComponent) rollupProductEvaluation {
	result := rollupProductEvaluation{StatusEvaluation: StatusEvaluation{Status: ServiceStatusMonitoring, Reason: "no_components"}}
	var evidenceAt *time.Time
	monitoringSince := time.Time{}
	recoveryConfirmedAt := time.Time{}
	allMonitoringConfirmed := len(components) > 0
	for index, component := range components {
		if index == 0 || statusSeverity(component.Evaluation.Status) > statusSeverity(result.Status) {
			result.Status = component.Evaluation.Status
			result.Reason = component.Evaluation.Reason
		}
		result.CustomerRequestCount += component.Evaluation.CustomerRequestCount
		result.CustomerSuccessCount += component.Evaluation.CustomerSuccessCount
		result.CustomerFailureCount += component.Evaluation.CustomerFailureCount
		result.ProbeCount += component.Evaluation.ProbeCount
		result.ProbeSuccessCount += component.Evaluation.ProbeSuccessCount
		result.ProbeFailureCount += component.Evaluation.ProbeFailureCount
		if component.EvidenceAt != nil && (evidenceAt == nil || component.EvidenceAt.After(*evidenceAt)) {
			value := *component.EvidenceAt
			evidenceAt = &value
		}
		if component.Evaluation.Status == ServiceStatusMonitoring && (monitoringSince.IsZero() || component.MonitoringSince.Before(monitoringSince)) {
			monitoringSince = component.MonitoringSince
		}
		if component.Evaluation.Status != ServiceStatusMonitoring || component.RecoveryConfirmedAt.IsZero() {
			allMonitoringConfirmed = false
		} else if component.RecoveryConfirmedAt.After(recoveryConfirmedAt) {
			recoveryConfirmedAt = component.RecoveryConfirmedAt
		}
	}
	if len(components) > 1 {
		result.Reason = "component_rollup"
	}
	// These transport fields are attached by the caller through a companion
	// object; keeping the evaluator result compact avoids a second DTO.
	result.EvidenceAt = evidenceAt
	result.MonitoringSince = monitoringSince
	if result.Status == ServiceStatusMonitoring && allMonitoringConfirmed {
		result.RecoveryConfirmedAt = recoveryConfirmedAt
	}
	return result
}

// rollupProductEvaluation carries timestamps alongside StatusEvaluation without
// exposing mutable package state.
type rollupProductEvaluation struct {
	StatusEvaluation
	EvidenceAt          *time.Time
	MonitoringSince     time.Time
	RecoveryConfirmedAt time.Time
}

func statusSeverity(status ServiceStatus) int {
	switch status {
	case ServiceStatusMajorOutage:
		return 6
	case ServiceStatusPartialOutage:
		return 5
	case ServiceStatusMaintenance:
		return 4
	case ServiceStatusDegradedPerformance:
		return 3
	case ServiceStatusMonitoring:
		return 2
	case ServiceStatusOperational:
		return 1
	default:
		return 0
	}
}

func (s *StatusControlService) loadStatusDefinitions(ctx context.Context) ([]statusProductDefinition, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT p.id,p.code,p.critical,c.id,c.code,c.model_pattern,c.access_mode,
 COALESCE(cc.computed_status,'monitoring'),cc.monitoring_since,cc.recovery_confirmed_at,
 b.id,b.group_id,b.group_name,b.platform,b.model_pattern,b.route_fingerprint,
 resolved_group.id,ag.account_id
FROM service_status_products p
JOIN service_status_components c ON c.product_id=p.id AND c.enabled=TRUE
LEFT JOIN service_status_component_current cc ON cc.component_id=c.id
LEFT JOIN service_status_bindings b ON b.component_id=c.id AND b.enabled=TRUE
LEFT JOIN groups resolved_group ON resolved_group.deleted_at IS NULL AND
 (resolved_group.id=b.group_id OR (b.group_id IS NULL AND b.group_name<>'' AND resolved_group.name=b.group_name))
LEFT JOIN account_groups ag ON ag.group_id=resolved_group.id
WHERE p.enabled=TRUE
ORDER BY p.id,c.id,b.id,ag.account_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	products := []statusProductDefinition{}
	productIndex := map[int64]int{}
	componentIndex := map[int64][2]int{}
	bindingIndex := map[int64][3]int{}
	for rows.Next() {
		var productID, componentID int64
		var productCode, componentCode, componentPattern, accessMode, previous string
		var critical bool
		var monitoring, recoveryConfirmed sql.NullTime
		var bindingID, groupID, resolvedGroupID, accountID sql.NullInt64
		var groupName, platform, bindingPattern, routeFingerprint sql.NullString
		if err := rows.Scan(&productID, &productCode, &critical, &componentID, &componentCode, &componentPattern, &accessMode,
			&previous, &monitoring, &recoveryConfirmed, &bindingID, &groupID, &groupName, &platform, &bindingPattern, &routeFingerprint,
			&resolvedGroupID, &accountID); err != nil {
			return nil, err
		}
		pi, ok := productIndex[productID]
		if !ok {
			pi = len(products)
			productIndex[productID] = pi
			products = append(products, statusProductDefinition{ID: productID, Code: productCode, Critical: critical})
		}
		location, ok := componentIndex[componentID]
		if !ok {
			ci := len(products[pi].Components)
			location = [2]int{pi, ci}
			componentIndex[componentID] = location
			component := statusComponentDefinition{ID: componentID, ProductID: productID, Code: componentCode, ModelPattern: componentPattern, AccessMode: accessMode, Previous: ServiceStatus(previous)}
			if monitoring.Valid {
				component.MonitoringSince = monitoring.Time
			}
			if recoveryConfirmed.Valid {
				component.RecoveryConfirmedAt = recoveryConfirmed.Time
			}
			products[pi].Components = append(products[pi].Components, component)
		}
		if !bindingID.Valid {
			continue
		}
		bl, ok := bindingIndex[bindingID.Int64]
		if !ok {
			binding := statusBinding{ID: bindingID.Int64, GroupName: groupName.String, Platform: platform.String, ModelPattern: bindingPattern.String, RouteFingerprint: routeFingerprint.String, AccountIDs: map[int64]struct{}{}}
			if resolvedGroupID.Valid {
				value := resolvedGroupID.Int64
				binding.GroupID = &value
			} else if groupID.Valid {
				value := groupID.Int64
				binding.GroupID = &value
			}
			bi := len(products[location[0]].Components[location[1]].Bindings)
			products[location[0]].Components[location[1]].Bindings = append(products[location[0]].Components[location[1]].Bindings, binding)
			bl = [3]int{location[0], location[1], bi}
			bindingIndex[bindingID.Int64] = bl
		}
		if accountID.Valid {
			products[bl[0]].Components[bl[1]].Bindings[bl[2]].AccountIDs[accountID.Int64] = struct{}{}
		}
	}
	return products, rows.Err()
}

func projectStatusObservations(products []statusProductDefinition, observations []*ReliabilityObservation) map[int64][]StatusObservation {
	result := make(map[int64][]StatusObservation)
	for _, observation := range observations {
		if observation == nil || observation.Outcome == ReliabilityOutcomeExcluded || observation.FactType == ReliabilityFactUpstreamAttempt {
			continue
		}
		type match struct{ componentID int64 }
		specific := map[int64]match{}
		generic := map[int64]match{}
		for _, product := range products {
			for _, component := range product.Components {
				if !statusComponentMatches(component, observation) {
					continue
				}
				for _, binding := range component.Bindings {
					matched, isSpecific := statusBindingMatches(binding, observation)
					if !matched {
						continue
					}
					if isSpecific {
						specific[component.ID] = match{componentID: component.ID}
					} else {
						generic[component.ID] = match{componentID: component.ID}
					}
				}
			}
		}
		selected := generic
		if len(specific) > 0 {
			selected = specific
			if observation.FactType == ReliabilityFactActiveProbe {
				// A unique route probe is shared evidence: fan it out to both
				// explicitly dependent group/route components and the generic
				// platform component. Customer requests keep exact-group precedence
				// so one invocation is never counted as two customer products.
				for id, item := range generic {
					selected[id] = item
				}
			}
		}
		ids := make([]int64, 0, len(selected))
		for id := range selected {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			result[id] = append(result[id], statusObservationFromEvidence(observation))
		}
	}
	return result
}

func statusComponentMatches(component statusComponentDefinition, observation *ReliabilityObservation) bool {
	if component.AccessMode != "http" || strings.HasPrefix(strings.ToLower(observation.Protocol), "ws") || strings.Contains(strings.ToLower(observation.Protocol), "websocket") {
		return false
	}
	return matchStatusModel(component.ModelPattern, observation.Model)
}

func statusBindingMatches(binding statusBinding, observation *ReliabilityObservation) (bool, bool) {
	if !matchStatusModel(binding.ModelPattern, observation.Model) {
		return false, false
	}
	if strings.EqualFold(observation.Protocol, "http_direct") && binding.RouteFingerprint == "" {
		// Direct-upstream diagnostics explain a gateway failure to operators but
		// are not an offered customer access mode. Only an explicit route binding
		// may publish them as status evidence.
		return false, false
	}
	if binding.RouteFingerprint != "" {
		return binding.RouteFingerprint == observation.RouteFingerprint, true
	}
	if binding.GroupID != nil {
		if observation.GroupID != nil {
			return *binding.GroupID == *observation.GroupID, true
		}
		if observation.FactType == ReliabilityFactActiveProbe && observation.AccountID != nil {
			_, ok := binding.AccountIDs[*observation.AccountID]
			return ok, true
		}
		return false, true
	}
	if binding.Platform != "" {
		return strings.EqualFold(binding.Platform, observation.Platform), false
	}
	return binding.ModelPattern != "", false
}

func matchStatusModel(pattern, model string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return true
	}
	matched, err := path.Match(pattern, strings.TrimSpace(model))
	return err == nil && matched
}

func statusObservationFromEvidence(observation *ReliabilityObservation) StatusObservation {
	return StatusObservation{Kind: observation.FactType, Outcome: observation.Outcome, CustomerImpact: observation.CustomerImpact, ObservedAt: observation.ObservedAt}
}

func nullableStatusTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

var _ LifecycleComponent = (*StatusControlService)(nil)
