//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestIncidentControlConcurrentCandidateRelapseDurationResolutionAndPublicTimeline(t *testing.T) {
	ctx := context.Background()
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.SetMultiple(ctx, map[string]string{
		service.SettingKeyReliabilityObservationEnabled:     "true",
		service.SettingKeyServiceStatusEnabled:              "true",
		service.SettingKeyServiceStatusPublicEnabled:        "true",
		service.SettingKeyReliabilityIncidentsEnabled:       "true",
		service.SettingKeyReliabilityIncidentsPublicEnabled: "true",
	}))
	t.Cleanup(func() {
		_ = settings.SetMultiple(context.Background(), map[string]string{
			service.SettingKeyReliabilityObservationEnabled:     "false",
			service.SettingKeyServiceStatusEnabled:              "false",
			service.SettingKeyServiceStatusPublicEnabled:        "false",
			service.SettingKeyReliabilityIncidentsEnabled:       "false",
			service.SettingKeyReliabilityIncidentsPublicEnabled: "false",
		})
	})
	evidence := service.NewReliabilityEvidenceService(integrationDB, settings)
	require.NoError(t, evidence.Start(ctx))
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = evidence.Stop(stopCtx)
	})
	status := service.NewStatusControlService(integrationDB, settings, evidence)
	first := service.NewIncidentControlService(integrationDB, settings, status, evidence)
	second := service.NewIncidentControlService(integrationDB, settings, status, evidence)
	t.Cleanup(func() { _ = first.Stop(context.Background()); _ = second.Stop(context.Background()) })
	require.NoError(t, first.UpdateSettings(ctx, service.IncidentControlSettings{Enabled: true, PublicEnabled: true}, 42))

	prefix := "incident-it-" + uuid.NewString()
	modelA, modelB := prefix+"-model-a", prefix+"-model-b"
	familyID, productA, productB, groupA, groupB := seedIncidentControlCatalog(t, prefix)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_incidents WHERE candidate_id IN (SELECT DISTINCT cp.candidate_id FROM reliability_incident_candidate_products cp JOIN service_status_products p ON p.id=cp.product_id WHERE p.family_id=$1)`, familyID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_incident_candidates WHERE id IN (SELECT DISTINCT cp.candidate_id FROM reliability_incident_candidate_products cp JOIN service_status_products p ON p.id=cp.product_id WHERE p.family_id=$1)`, familyID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_incident_candidates WHERE candidate_key LIKE $1`, prefix+"%")
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_observations WHERE idempotency_key LIKE $1`, prefix+"%")
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM service_status_products WHERE family_id=$1`, familyID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM service_status_families WHERE id=$1`, familyID)
	})
	base := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Microsecond)
	var accountChangesBefore int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_change_records`).Scan(&accountChangesBefore))
	statusEvidenceAt := base.Add(2 * time.Minute)
	setIncidentProductStatus(t, productA, service.ServiceStatusPartialOutage, statusEvidenceAt, nil)
	_, err := integrationDB.ExecContext(ctx, `UPDATE service_status_current SET effective_status='operational',effective_reason='manual_override' WHERE product_id=$1`, productA)
	require.NoError(t, err)
	setIncidentProductStatus(t, productB, service.ServiceStatusMajorOutage, statusEvidenceAt, nil)
	insertIncidentCustomerFailure(t, prefix+"a1", groupA, modelA, base)
	insertIncidentCustomerFailure(t, prefix+"b1", groupB, modelB, base.Add(time.Minute))
	preIncidentSuccessID := insertIncidentCustomerSuccess(t, prefix+"pre-incident-success", groupA, modelA, base.Add(-5*time.Minute))
	_, err = integrationDB.ExecContext(ctx, `
	INSERT INTO reliability_observations(idempotency_key,fact_type,source,source_id,user_id,group_id,platform,model,request_class,protocol,outcome,status_code,error_owner,customer_impact,observed_at)
	SELECT $1 || gs::text,'customer_request','integration_incident',$1 || gs::text,900001,$2,'openai',$4,'text','responses','failure',503,'provider',TRUE,$3::timestamptz + (gs || ' milliseconds')::interval
FROM generate_series(1,5001) gs`, prefix+"bulk-a-", groupA, base.Add(30*time.Second), modelA)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, controller := range []*service.IncidentControlService{first, second} {
		wg.Add(1)
		go func(control *service.IncidentControlService) {
			defer wg.Done()
			errs <- control.ReconcileAt(ctx, base.Add(3*time.Minute))
		}(controller)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	snapshot, err := first.AdminSnapshot(ctx)
	require.NoError(t, err)
	require.Len(t, snapshot.Candidates, 1)
	require.Equal(t, service.ServiceStatusPartialOutage, snapshot.Candidates[0].Products[0].LatestStatus, "Manual Status Override must not change Incident input")
	require.Equal(t, service.IncidentCandidateOpen, snapshot.Candidates[0].State)
	require.Len(t, snapshot.Candidates[0].Products, 2)
	require.NoError(t, first.ReconcileAt(ctx, base.Add(3*time.Minute+30*time.Second)))
	var candidateWatermark time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT last_reconciled_at FROM reliability_incident_candidates WHERE id=$1`, snapshot.Candidates[0].ID).Scan(&candidateWatermark))
	require.False(t, candidateWatermark.Before(base.Add(3*time.Minute)), "open candidates must advance an incremental evidence watermark")

	incident, err := first.ConfirmCandidate(ctx, service.IncidentConfirmCommand{
		CandidateID: snapshot.Candidates[0].ID, Title: "Integration reliability incident",
		InternalSummary: "Two products affected", ActorUserID: 42, At: base.Add(3*time.Minute + 31*time.Second),
	})
	require.NoError(t, err)
	require.Equal(t, service.IncidentPhaseInvestigating, incident.Phase)
	require.Len(t, incident.Products, 2)
	require.Equal(t, base, incident.ObservationStartedAt)
	require.Equal(t, base, incident.Products[0].Segments[0].StartedAt)
	require.NoError(t, first.AddUpdate(ctx, service.IncidentUpdateCommand{IncidentID: incident.ID, Message: "Internal evidence reviewed", ActorUserID: 42}))
	require.NoError(t, first.Transition(ctx, service.IncidentTransitionCommand{IncidentID: incident.ID, Target: service.IncidentPhaseIdentified, Message: "Cause identified", ActorUserID: 42, At: base.Add(3*time.Minute + 32*time.Second)}))

	monitoringA := base.Add(5 * time.Minute)
	setIncidentProductStatus(t, productA, service.ServiceStatusMonitoring, monitoringA, &monitoringA)
	recoveryA1ID := insertIncidentCustomerSuccess(t, prefix+"a-recovery-1", groupA, modelA, monitoringA)
	lateRoutineSuccessID := insertIncidentCustomerSuccess(t, prefix+"a-routine-late", groupA, modelA, monitoringA.Add(20*time.Second))
	require.NoError(t, first.ReconcileAt(ctx, base.Add(6*time.Minute+2*time.Second)))
	incident, err = first.GetAdminIncident(ctx, incident.ID)
	require.NoError(t, err)
	require.Equal(t, int64(5*60), incident.Products[0].CompensableSeconds)
	require.Greater(t, incident.Products[0].Segments[0].EndObservationID, int64(0))
	require.Equal(t, recoveryA1ID, incident.Products[0].Segments[0].EndObservationID)
	require.NotEqual(t, lateRoutineSuccessID, incident.Products[0].Segments[0].EndObservationID, "success after Monitoring starts cannot close the earlier segment")
	require.Nil(t, incident.Products[1].Segments[0].EndedAt)

	monitoringB := base.Add(7 * time.Minute)
	setIncidentProductStatus(t, productB, service.ServiceStatusMonitoring, monitoringB, &monitoringB)
	insertIncidentCustomerSuccess(t, prefix+"b-recovery-1", groupB, modelB, monitoringB)
	require.NoError(t, first.ReconcileAt(ctx, base.Add(8*time.Minute+2*time.Second)))
	incident, err = first.GetAdminIncident(ctx, incident.ID)
	require.NoError(t, err)
	require.Equal(t, service.IncidentPhaseMonitoring, incident.Phase)
	_, err = integrationDB.ExecContext(ctx, `UPDATE reliability_incidents SET evidence_gap=TRUE WHERE id=$1`, incident.ID)
	require.NoError(t, err)
	require.Error(t, first.Transition(ctx, service.IncidentTransitionCommand{IncidentID: incident.ID, Target: service.IncidentPhaseResolved, Message: "premature resolution", ActorUserID: 42, At: monitoringB.Add(10 * time.Minute)}), "Monitoring without Operational products must not be manually resolved")
	require.NoError(t, first.AcknowledgeEvidenceGap(ctx, service.IncidentEvidenceGapCommand{IncidentID: incident.ID, Reason: "offline evidence reviewed", ActorUserID: 42}))
	var gapAfterReview bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT evidence_gap FROM reliability_incidents WHERE id=$1`, incident.ID).Scan(&gapAfterReview))
	require.False(t, gapAfterReview)
	probeRelapseAt := base.Add(10 * time.Minute)
	setIncidentProductStatus(t, productA, service.ServiceStatusDegradedPerformance, probeRelapseAt, nil)
	insertIncidentProbeFailure(t, prefix+"relapse-probe-1", modelA, probeRelapseAt.Add(-time.Minute))
	insertIncidentProbeFailure(t, prefix+"relapse-probe-2", modelA, probeRelapseAt.Add(-30*time.Second))
	insertIncidentProbeFailure(t, prefix+"relapse-probe-3", modelA, probeRelapseAt)
	require.NoError(t, first.ReconcileAt(ctx, base.Add(11*time.Minute+2*time.Second)))
	incident, err = first.GetAdminIncident(ctx, incident.ID)
	require.NoError(t, err)
	require.Equal(t, service.IncidentPhaseInvestigating, incident.Phase)
	require.Len(t, incident.Products[0].Segments, 1)
	var probeRelapseMessage string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT internal_message FROM reliability_incident_updates WHERE incident_id=$1 AND kind='system' ORDER BY id DESC LIMIT 1`, incident.ID).Scan(&probeRelapseMessage))
	require.Contains(t, probeRelapseMessage, "computed service status relapsed")
	require.NotContains(t, probeRelapseMessage, "customer impact relapsed")

	relapseAt := base.Add(12 * time.Minute)
	setIncidentProductStatus(t, productA, service.ServiceStatusPartialOutage, relapseAt, nil)
	insertIncidentCustomerFailure(t, prefix+"a2", groupA, modelA, relapseAt)
	require.NoError(t, first.ReconcileAt(ctx, relapseAt.Add(time.Minute)))
	incident, err = first.GetAdminIncident(ctx, incident.ID)
	require.NoError(t, err)
	require.Equal(t, service.IncidentPhaseInvestigating, incident.Phase)
	require.Len(t, incident.Products[0].Segments, 2)
	require.Equal(t, int64(5*60), incident.Products[0].CompensableSeconds, "Monitoring gap must remain excluded while relapse segment is open")

	monitoringAfterRelapse := base.Add(13 * time.Minute)
	setIncidentProductStatus(t, productA, service.ServiceStatusOperational, monitoringAfterRelapse, &monitoringAfterRelapse)
	setIncidentProductStatus(t, productB, service.ServiceStatusOperational, monitoringB, &monitoringB)
	insertIncidentCustomerSuccess(t, prefix+"a-recovery-2", groupA, modelA, monitoringAfterRelapse)
	require.NoError(t, first.ReconcileAt(ctx, monitoringAfterRelapse.Add(10*time.Minute)))
	incident, err = first.GetAdminIncident(ctx, incident.ID)
	require.NoError(t, err)
	require.Equal(t, service.IncidentPhaseResolved, incident.Phase)
	require.Equal(t, int64(6*60), incident.Products[0].CompensableSeconds)
	require.NotNil(t, incident.CustomerImpactEndedAt)

	require.Error(t, first.PublishUpdate(ctx, service.IncidentPublishCommand{IncidentID: incident.ID, Message: "供应商 route https://internal.invalid 已恢复", ActorUserID: 42}))
	require.NoError(t, first.PublishUpdate(ctx, service.IncidentPublishCommand{IncidentID: incident.ID, Message: "相关服务已经恢复，用户无需进行额外操作。", ActorUserID: 42}))
	public, err := first.PublicSnapshot(ctx)
	require.NoError(t, err)
	require.True(t, public.Enabled)
	require.Len(t, public.Incidents, 1)
	require.Len(t, public.Incidents[0].AffectedProducts, 2)
	require.Equal(t, "相关服务已经恢复，用户无需进行额外操作。", public.Incidents[0].Timeline[0].Message)

	var audits int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_audit_log WHERE incident_id=$1`, incident.ID).Scan(&audits))
	require.GreaterOrEqual(t, audits, 1)
	for _, action := range []string{"confirm_candidate", "add_internal_update", "transition", "publish_public_update", "acknowledge_evidence_gap"} {
		var count int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_audit_log WHERE incident_id=$1 AND action=$2`, incident.ID, action).Scan(&count))
		require.Equal(t, 1, count, action)
	}
	var settingsAudits int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_settings_audit WHERE actor_user_id=42`).Scan(&settingsAudits))
	require.GreaterOrEqual(t, settingsAudits, 1)
	var dismissCandidateID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incident_candidates(candidate_key,state,first_observed_at,last_observed_at) VALUES($1,'open',$2,$2) RETURNING id`, prefix+"dismiss", base).Scan(&dismissCandidateID))
	require.NoError(t, first.DismissCandidate(ctx, service.IncidentDismissCommand{CandidateID: dismissCandidateID, Reason: "unrelated alert", ActorUserID: 42, At: base.Add(4 * time.Minute)}))
	var dismissAudits int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_audit_log WHERE candidate_id=$1 AND action='dismiss_candidate'`, dismissCandidateID).Scan(&dismissAudits))
	require.Equal(t, 1, dismissAudits)
	var expiredCandidateID int64
	old := time.Now().UTC().Add(-8 * 24 * time.Hour)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO reliability_incident_candidates(candidate_key,state,first_observed_at,last_observed_at) VALUES($1,'open',$2,$2) RETURNING id`, prefix+"expired", old).Scan(&expiredCandidateID))
	require.NoError(t, first.ReconcileAt(ctx, time.Now().UTC()))
	var expiredState, expiredReason string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT state,dismissed_reason FROM reliability_incident_candidates WHERE id=$1`, expiredCandidateID).Scan(&expiredState, &expiredReason))
	require.Equal(t, "recovered", expiredState)
	require.Equal(t, "evidence_window_expired", expiredReason)
	var recoveryLinks, statusLinks, impactLinks int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND relation='recovery'`, incident.ID).Scan(&recoveryLinks))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND relation='status_evidence'`, incident.ID).Scan(&statusLinks))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND relation='customer_impact'`, incident.ID).Scan(&impactLinks))
	require.GreaterOrEqual(t, recoveryLinks, 3)
	require.GreaterOrEqual(t, statusLinks, 5)
	require.GreaterOrEqual(t, impactLinks, 5003, "customer-impact evidence must page beyond 5000 rows")
	var preIncidentLinks int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND observation_id=$2`, incident.ID, preIncidentSuccessID).Scan(&preIncidentLinks))
	require.Zero(t, preIncidentLinks, "pre-Incident healthy evidence must not enter the Observation Window")
	var accountChangesAfter int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_change_records`).Scan(&accountChangesAfter))
	require.Equal(t, accountChangesBefore, accountChangesAfter, "Incident Control must never mutate routing/account state")
}

func TestIncidentControlKeepsUnrelatedFamiliesInSeparateOverlappingIncidents(t *testing.T) {
	ctx := context.Background()
	var preexistingActive int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incidents WHERE phase<>'resolved'`).Scan(&preexistingActive))
	require.Zero(t, preexistingActive, "prior integration fixtures must clean active incidents")
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.SetMultiple(ctx, map[string]string{service.SettingKeyReliabilityObservationEnabled: "true", service.SettingKeyServiceStatusEnabled: "true", service.SettingKeyReliabilityIncidentsEnabled: "true"}))
	evidence := service.NewReliabilityEvidenceService(integrationDB, settings)
	require.NoError(t, evidence.Start(ctx))
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = evidence.Stop(stop)
		_ = settings.SetMultiple(context.Background(), map[string]string{service.SettingKeyReliabilityObservationEnabled: "false", service.SettingKeyServiceStatusEnabled: "false", service.SettingKeyReliabilityIncidentsEnabled: "false", service.SettingKeyReliabilityIncidentsPublicEnabled: "false"})
	})
	status := service.NewStatusControlService(integrationDB, settings, evidence)
	control := service.NewIncidentControlService(integrationDB, settings, status, evidence)
	prefixA, prefixB := "incident-family-a-"+uuid.NewString(), "incident-family-b-"+uuid.NewString()
	modelA1, modelA2 := prefixA+"-model-a", prefixA+"-model-b"
	modelB1, modelB2 := prefixB+"-model-a", prefixB+"-model-b"
	familyA, productA1, productA2, groupA1, groupA2 := seedIncidentControlCatalog(t, prefixA)
	familyB, productB1, productB2, groupB1, _ := seedIncidentControlCatalog(t, prefixB)
	for _, fixture := range []struct {
		prefix string
		family int64
	}{{prefixA, familyA}, {prefixB, familyB}} {
		fixture := fixture
		t.Cleanup(func() { cleanupIncidentControlCatalog(fixture.prefix, fixture.family) })
	}
	base := time.Now().UTC().Add(-20 * time.Minute).Truncate(time.Microsecond)
	setIncidentProductStatus(t, productA1, service.ServiceStatusPartialOutage, base.Add(time.Minute), nil)
	setIncidentProductStatus(t, productB1, service.ServiceStatusMajorOutage, base.Add(time.Minute), nil)
	insertIncidentCustomerFailure(t, prefixA+"failure-1", groupA1, modelA1, base)
	insertIncidentCustomerFailure(t, prefixB+"failure-1", groupB1, modelB1, base)
	require.NoError(t, control.ReconcileAt(ctx, base.Add(2*time.Minute+2*time.Second)))
	snapshot, err := control.AdminSnapshot(ctx)
	require.NoError(t, err)
	open := []service.AdminIncidentCandidate{}
	for _, candidate := range snapshot.Candidates {
		if candidate.State == service.IncidentCandidateOpen {
			open = append(open, candidate)
		}
	}
	require.Len(t, open, 2)
	for index, candidate := range open {
		_, err = control.ConfirmCandidate(ctx, service.IncidentConfirmCommand{CandidateID: candidate.ID, Title: fmt.Sprintf("Overlapping incident %d", index), ActorUserID: 42, At: base.Add(3 * time.Minute)})
		require.NoError(t, err)
	}
	snapshot, err = control.AdminSnapshot(ctx)
	require.NoError(t, err)
	active := []service.AdminIncident{}
	for _, incident := range snapshot.Incidents {
		if incident.Phase != service.IncidentPhaseResolved {
			active = append(active, incident)
		}
	}
	require.Len(t, active, 2)
	require.Len(t, active[0].Products, 1)
	require.Len(t, active[1].Products, 1)
	oldFailureID := insertIncidentCustomerFailure(t, prefixA+"old-unrelated", groupA2, modelA2, base.Add(-20*time.Minute))
	oldProbeID := insertIncidentProbeFailure(t, prefixB+"old-unrelated-probe", modelB2, base.Add(-20*time.Minute))
	_, err = integrationDB.ExecContext(ctx, `UPDATE reliability_incidents SET last_reconciled_at=$1 WHERE id=ANY($2)`, base.Add(-25*time.Minute), pq.Array([]int64{active[0].ID, active[1].ID}))
	require.NoError(t, err)
	lateFailureStart := base.Add(4 * time.Minute)
	setIncidentProductStatus(t, productA2, service.ServiceStatusPartialOutage, base.Add(6*time.Minute), nil)
	setIncidentProductStatus(t, productB2, service.ServiceStatusDegradedPerformance, base.Add(6*time.Minute), nil)
	lateProbeIDs := []int64{
		insertIncidentProbeFailure(t, prefixB+"late-probe-1", modelB2, lateFailureStart),
		insertIncidentProbeFailure(t, prefixB+"late-probe-2", modelB2, base.Add(5*time.Minute)),
		insertIncidentProbeFailure(t, prefixB+"late-probe-3", modelB2, base.Add(6*time.Minute)),
	}
	lateFailureIDs := []int64{
		insertIncidentCustomerFailure(t, prefixA+"late-1", groupA2, modelA2, lateFailureStart),
		insertIncidentCustomerFailure(t, prefixA+"late-2", groupA2, modelA2, base.Add(5*time.Minute)),
		insertIncidentCustomerFailure(t, prefixA+"late-3", groupA2, modelA2, base.Add(6*time.Minute)),
	}
	require.NoError(t, control.ReconcileAt(ctx, base.Add(7*time.Minute+2*time.Second)))
	probeSnapshot, err := control.AdminSnapshot(ctx)
	require.NoError(t, err)
	var probeIncident service.AdminIncident
	var probeProduct service.AdminIncidentProduct
	for _, incident := range probeSnapshot.Incidents {
		for _, product := range incident.Products {
			if product.ProductCode == prefixB+"-product-1" {
				probeIncident, probeProduct = incident, product
			}
		}
	}
	require.Greater(t, probeIncident.ID, int64(0))
	require.Empty(t, probeProduct.Segments)
	for _, observationID := range lateProbeIDs {
		var linked int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND observation_id=$2 AND relation='status_evidence'`, probeIncident.ID, observationID).Scan(&linked))
		require.Equal(t, 1, linked)
	}
	for _, oldObservation := range []struct{ incidentID, observationID int64 }{{probeIncident.ID, oldProbeID}} {
		var linked int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND observation_id=$2`, oldObservation.incidentID, oldObservation.observationID).Scan(&linked))
		require.Zero(t, linked, "evidence before the 15-minute event boundary must not attach")
	}
	lateMonitoring := base.Add(8 * time.Minute)
	setIncidentProductStatus(t, productA2, service.ServiceStatusMonitoring, lateMonitoring, &lateMonitoring)
	insertIncidentCustomerSuccess(t, prefixA+"late-recovery", groupA2, modelA2, lateMonitoring)
	require.NoError(t, control.ReconcileAt(ctx, base.Add(9*time.Minute+2*time.Second)))
	snapshot, err = control.AdminSnapshot(ctx)
	require.NoError(t, err)
	var attachedIncident service.AdminIncident
	var attachedProduct service.AdminIncidentProduct
	for _, incident := range snapshot.Incidents {
		for _, product := range incident.Products {
			if product.ProductCode == prefixA+"-product-1" {
				attachedIncident, attachedProduct = incident, product
			}
		}
	}
	require.Greater(t, attachedIncident.ID, int64(0))
	require.Len(t, attachedProduct.Segments, 1)
	require.Equal(t, lateFailureStart, attachedProduct.Segments[0].StartedAt)
	require.Equal(t, int64(4*60), attachedProduct.CompensableSeconds)
	for _, observationID := range lateFailureIDs {
		var linked int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND observation_id=$2 AND relation='customer_impact'`, attachedIncident.ID, observationID).Scan(&linked))
		require.Equal(t, 1, linked)
	}
	var oldFailureLinked int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND observation_id=$2`, attachedIncident.ID, oldFailureID).Scan(&oldFailureLinked))
	require.Zero(t, oldFailureLinked, "old unrelated customer failure must not move the late-attach impact boundary")
}

func TestIncidentControlReusesOneCandidateAcrossLongContinuousDegradation(t *testing.T) {
	ctx := context.Background()
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.SetMultiple(ctx, map[string]string{
		service.SettingKeyReliabilityObservationEnabled: "true",
		service.SettingKeyServiceStatusEnabled:          "true",
		service.SettingKeyReliabilityIncidentsEnabled:   "true",
	}))
	evidence := service.NewReliabilityEvidenceService(integrationDB, settings)
	require.NoError(t, evidence.Start(ctx))
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = evidence.Stop(stop)
		_ = settings.SetMultiple(context.Background(), map[string]string{
			service.SettingKeyReliabilityObservationEnabled: "false",
			service.SettingKeyServiceStatusEnabled:          "false",
			service.SettingKeyReliabilityIncidentsEnabled:   "false",
		})
	})
	status := service.NewStatusControlService(integrationDB, settings, evidence)
	control := service.NewIncidentControlService(integrationDB, settings, status, evidence)
	prefix := "incident-continuous-" + uuid.NewString()
	familyID, productID, _, groupID, _ := seedIncidentControlCatalog(t, prefix)
	t.Cleanup(func() { cleanupIncidentControlCatalog(prefix, familyID) })

	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)
	setIncidentProductStatus(t, productID, service.ServiceStatusDegradedPerformance, base, nil)
	insertIncidentCustomerFailure(t, prefix+"-failure", groupID, prefix+"-model", base)
	for _, offset := range []time.Duration{2 * time.Minute, 10 * time.Minute, 20 * time.Minute, 29 * time.Minute, 31 * time.Minute, 32 * time.Minute} {
		require.NoError(t, control.ReconcileAt(ctx, base.Add(offset)))
	}

	var candidateCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT COUNT(DISTINCT c.id)
FROM reliability_incident_candidates c
JOIN reliability_incident_candidate_products cp ON cp.candidate_id=c.id
JOIN service_status_products p ON p.id=cp.product_id
WHERE p.family_id=$1 AND c.state='open'`, familyID).Scan(&candidateCount))
	require.Equal(t, 1, candidateCount, "one continuous degradation must remain one Incident Candidate")
}

func TestIncidentControlLinksProbeOnlyAbnormalEvidenceWithoutInventingImpactSegment(t *testing.T) {
	ctx := context.Background()
	var leakedIncidentFixtures int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_observations WHERE source='integration_incident'`).Scan(&leakedIncidentFixtures))
	require.Zero(t, leakedIncidentFixtures, "prior Incident fixtures must clean linked observations")
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.SetMultiple(ctx, map[string]string{service.SettingKeyReliabilityObservationEnabled: "true", service.SettingKeyServiceStatusEnabled: "true", service.SettingKeyReliabilityIncidentsEnabled: "true"}))
	evidence := service.NewReliabilityEvidenceService(integrationDB, settings)
	require.NoError(t, evidence.Start(ctx))
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = evidence.Stop(stop)
		_ = settings.SetMultiple(context.Background(), map[string]string{service.SettingKeyReliabilityObservationEnabled: "false", service.SettingKeyServiceStatusEnabled: "false", service.SettingKeyReliabilityIncidentsEnabled: "false"})
	})
	status := service.NewStatusControlService(integrationDB, settings, evidence)
	control := service.NewIncidentControlService(integrationDB, settings, status, evidence)
	prefix := "probe-only-" + uuid.NewString()
	modelA := prefix + "-model-a"
	familyID, productID, _, _, _ := seedIncidentControlCatalog(t, prefix)
	t.Cleanup(func() { cleanupIncidentControlCatalog(prefix, familyID) })
	base := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Microsecond)
	setIncidentProductStatus(t, productID, service.ServiceStatusDegradedPerformance, base.Add(2*time.Minute), nil)
	probeIDs := []int64{
		insertIncidentProbeFailure(t, prefix+"probe-1", modelA, base),
		insertIncidentProbeFailure(t, prefix+"probe-2", modelA, base.Add(30*time.Second)),
		insertIncidentProbeFailure(t, prefix+"probe-3", modelA, base.Add(time.Minute)),
	}
	require.NoError(t, control.ReconcileAt(ctx, base.Add(3*time.Minute)))
	snapshot, err := control.AdminSnapshot(ctx)
	require.NoError(t, err)
	var candidate service.AdminIncidentCandidate
	for _, value := range snapshot.Candidates {
		if value.State == service.IncidentCandidateOpen {
			candidate = value
			break
		}
	}
	require.Greater(t, candidate.ID, int64(0))
	incident, err := control.ConfirmCandidate(ctx, service.IncidentConfirmCommand{CandidateID: candidate.ID, Title: "Probe-only degradation", ActorUserID: 42, At: base.Add(3*time.Minute + time.Second)})
	require.NoError(t, err)
	require.Len(t, incident.Products, 1)
	require.Empty(t, incident.Products[0].Segments)
	for _, observationID := range probeIDs {
		var linked int
		require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM reliability_incident_observation_links WHERE incident_id=$1 AND observation_id=$2 AND relation='status_evidence'`, incident.ID, observationID).Scan(&linked))
		require.Equal(t, 1, linked)
	}
}

func TestIncidentControlTargetedLookupFindsIncidentOlderThanAdminPage(t *testing.T) {
	prefix := "old-incident-" + uuid.NewString()[:8]
	_, err := integrationDB.ExecContext(context.Background(), `
INSERT INTO reliability_incidents(public_id,phase,title,observation_started_at,resolved_at,observation_ended_at)
SELECT $1 || '-' || gs::text,'resolved',$2 || gs::text,NOW()-INTERVAL '40 days',NOW()-INTERVAL '39 days',NOW()-INTERVAL '39 days'
FROM generate_series(1,101) gs`, prefix, prefix+"-")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_incidents WHERE title LIKE $1`, prefix+"-%")
	})
	var oldestID int64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), `SELECT id FROM reliability_incidents WHERE title LIKE $1 ORDER BY id LIMIT 1`, prefix+"-%").Scan(&oldestID))
	control := service.NewIncidentControlService(integrationDB, nil, nil, nil)
	incident, err := control.GetAdminIncident(context.Background(), oldestID)
	require.NoError(t, err)
	require.Equal(t, oldestID, incident.ID)
	require.Empty(t, incident.Products)
}

func TestIncidentControlSerializesConcurrentOppositeSettingsMutations(t *testing.T) {
	ctx := context.Background()
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.SetMultiple(ctx, map[string]string{service.SettingKeyReliabilityObservationEnabled: "true", service.SettingKeyServiceStatusEnabled: "true", service.SettingKeyServiceStatusPublicEnabled: "false", service.SettingKeyReliabilityIncidentsEnabled: "false", service.SettingKeyReliabilityIncidentsPublicEnabled: "false"}))
	evidence := service.NewReliabilityEvidenceService(integrationDB, settings)
	require.NoError(t, evidence.Start(ctx))
	status := service.NewStatusControlService(integrationDB, settings, evidence)
	first := service.NewIncidentControlService(integrationDB, settings, status, evidence)
	second := service.NewIncidentControlService(integrationDB, settings, status, evidence)
	t.Cleanup(func() {
		_ = first.Stop(ctx)
		_ = second.Stop(ctx)
		stop, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = evidence.Stop(stop)
		_ = settings.SetMultiple(context.Background(), map[string]string{service.SettingKeyReliabilityObservationEnabled: "false", service.SettingKeyServiceStatusEnabled: "false", service.SettingKeyReliabilityIncidentsEnabled: "false", service.SettingKeyReliabilityIncidentsPublicEnabled: "false"})
	})
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		errs <- first.UpdateSettings(ctx, service.IncidentControlSettings{Enabled: true}, 501)
	}()
	go func() { <-start; errs <- second.UpdateSettings(ctx, service.IncidentControlSettings{}, 502) }()
	close(start)
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	var enabled string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, service.SettingKeyReliabilityIncidentsEnabled).Scan(&enabled))
	var auditedEnabled string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT after_state->>'enabled' FROM reliability_incident_settings_audit WHERE actor_user_id IN (501,502) ORDER BY id DESC LIMIT 1`).Scan(&auditedEnabled))
	require.Equal(t, enabled, auditedEnabled, "last settings audit and committed runtime flag must agree")
}

func cleanupIncidentControlCatalog(prefix string, familyID int64) {
	_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_incidents WHERE candidate_id IN (SELECT DISTINCT cp.candidate_id FROM reliability_incident_candidate_products cp JOIN service_status_products p ON p.id=cp.product_id WHERE p.family_id=$1)`, familyID)
	_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_incident_candidates WHERE id IN (SELECT DISTINCT cp.candidate_id FROM reliability_incident_candidate_products cp JOIN service_status_products p ON p.id=cp.product_id WHERE p.family_id=$1)`, familyID)
	_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_observations WHERE idempotency_key LIKE $1`, prefix+"%")
	_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM service_status_products WHERE family_id=$1`, familyID)
	_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM service_status_families WHERE id=$1`, familyID)
}

func seedIncidentControlCatalog(t *testing.T, prefix string) (familyID, productA, productB, groupA, groupB int64) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO service_status_families(code,display_name,sort_order) VALUES($1,$2,999) RETURNING id`, prefix+"-family", "Incident integration").Scan(&familyID))
	for index, target := range []*int64{&productA, &productB} {
		code := fmt.Sprintf("%s-product-%d", prefix, index)
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO service_status_products(family_id,code,display_name,sort_order,public,critical) VALUES($1,$2,$3,$4,TRUE,TRUE) RETURNING id`, familyID, code, fmt.Sprintf("Incident Product %d", index+1), index).Scan(target))
		var componentID int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO service_status_components(product_id,code,display_name,model_pattern,access_mode) VALUES($1,$2,$3,$4,'http') RETURNING id`, *target, code+"-http", code+" HTTP", fmt.Sprintf("%s-model-%c", prefix, 'a'+index)).Scan(&componentID))
		groupID := int64(990000) + *target
		if index == 0 {
			groupA = groupID
		} else {
			groupB = groupID
		}
		_, err := integrationDB.ExecContext(ctx, `INSERT INTO service_status_bindings(binding_key,component_id,group_id,model_pattern) VALUES($1,$2,$3,$4)`, code+"-binding", componentID, groupID, fmt.Sprintf("%s-model-%c", prefix, 'a'+index))
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `INSERT INTO service_status_bindings(binding_key,component_id,platform,model_pattern) VALUES($1,$2,'openai',$3)`, code+"-platform-binding", componentID, fmt.Sprintf("%s-model-%c", prefix, 'a'+index))
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `INSERT INTO service_status_current(product_id) VALUES($1)`, *target)
		require.NoError(t, err)
	}
	return familyID, productA, productB, groupA, groupB
}

func setIncidentProductStatus(t *testing.T, productID int64, status service.ServiceStatus, evidenceAt time.Time, monitoringSince *time.Time) {
	t.Helper()
	_, err := integrationDB.ExecContext(context.Background(), `UPDATE service_status_current SET computed_status=$2,effective_status=$2,computed_reason='integration',effective_reason='integration',evidence_at=$3,computed_at=$3,monitoring_since=$4,updated_at=$3 WHERE product_id=$1`, productID, string(status), evidenceAt, monitoringSince)
	require.NoError(t, err)
}

func insertIncidentCustomerFailure(t *testing.T, identity string, groupID int64, model string, observedAt time.Time) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
INSERT INTO reliability_observations(idempotency_key,fact_type,source,source_id,user_id,group_id,platform,model,request_class,protocol,outcome,status_code,error_owner,customer_impact,observed_at)
VALUES($1,'customer_request','integration_incident',$1,900001,$2,'openai',$3,'text','responses','failure',503,'provider',TRUE,$4) RETURNING id`, identity, groupID, model, observedAt).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertIncidentCustomerSuccess(t *testing.T, identity string, groupID int64, model string, observedAt time.Time) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
INSERT INTO reliability_observations(idempotency_key,fact_type,source,source_id,user_id,group_id,platform,model,request_class,protocol,outcome,status_code,error_owner,customer_impact,observed_at)
VALUES($1,'customer_request','integration_incident',$1,900001,$2,'openai',$3,'text','responses','success',200,'',FALSE,$4) RETURNING id`, identity, groupID, model, observedAt).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertIncidentProbeFailure(t *testing.T, identity, model string, observedAt time.Time) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
INSERT INTO reliability_observations(idempotency_key,fact_type,source,source_id,platform,model,request_class,protocol,outcome,status_code,error_owner,customer_impact,observed_at)
VALUES($1,'active_probe','integration_incident',$1,'openai',$2,'text','http','failure',503,'provider',FALSE,$3) RETURNING id`, identity, model, observedAt).Scan(&id)
	require.NoError(t, err)
	return id
}
