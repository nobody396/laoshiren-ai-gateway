//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestStatusControlReconcilesCustomerImpactIntoProductState(t *testing.T) {
	ctx := context.Background()
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.Set(ctx, service.SettingKeyServiceStatusEnabled, "true"))
	require.NoError(t, settings.Set(ctx, service.SettingKeyReliabilityObservationEnabled, "true"))
	prefix := fmt.Sprintf("integration:status:%d:", time.Now().UnixNano())
	now := time.Now()
	for index := 0; index < 3; index++ {
		_, err := integrationDB.ExecContext(ctx, `
INSERT INTO reliability_observations(
 idempotency_key,fact_type,source,source_id,user_id,platform,model,request_class,protocol,
 outcome,status_code,error_owner,customer_impact,observed_at
) VALUES($1,'customer_request','integration',$2,900001,'openai','gpt-5.6-sol','text','responses','failure',503,'provider',TRUE,$3)`,
			prefix+fmt.Sprint(index), fmt.Sprint(index), now.Add(-time.Duration(index+2)*time.Second))
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_observations WHERE idempotency_key LIKE $1`, prefix+"%")
		_ = settings.Set(context.Background(), service.SettingKeyServiceStatusEnabled, "false")
		_ = settings.Set(context.Background(), service.SettingKeyReliabilityObservationEnabled, "false")
		_, _ = integrationDB.ExecContext(context.Background(), `UPDATE service_status_current SET computed_status='monitoring',effective_status='monitoring',computed_reason='no_evidence',effective_reason='no_evidence',monitoring_since=NULL,recovery_confirmed_at=NULL`)
		_, _ = integrationDB.ExecContext(context.Background(), `UPDATE service_status_component_current SET computed_status='monitoring',reason='no_evidence',monitoring_since=NULL,recovery_confirmed_at=NULL`)
	})
	evidence := service.NewReliabilityEvidenceService(integrationDB, settings)
	require.NoError(t, evidence.Start(ctx))
	t.Cleanup(func() { _ = evidence.Stop(context.Background()) })
	statusControl := service.NewStatusControlService(integrationDB, settings, evidence)

	require.NoError(t, statusControl.Reconcile(ctx))
	var status, reason string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT current.effective_status,current.effective_reason
FROM service_status_current current
JOIN service_status_products product ON product.id=current.product_id
WHERE product.code='openai-codex-api'`).Scan(&status, &reason))
	require.Equal(t, "major_outage", status)
	require.Equal(t, "broad_critical_loss", reason)
}

func TestStatusControlExpiresManualOverrideBackToComputedState(t *testing.T) {
	ctx := context.Background()
	settings := NewSettingRepository(integrationEntClient)
	require.NoError(t, settings.Set(ctx, service.SettingKeyServiceStatusEnabled, "true"))
	require.NoError(t, settings.Set(ctx, service.SettingKeyReliabilityObservationEnabled, "true"))
	prefix := fmt.Sprintf("integration:status-expiry:%d", time.Now().UnixNano())
	now := time.Now()
	_, err := integrationDB.ExecContext(ctx, `
INSERT INTO reliability_observations(
 idempotency_key,fact_type,source,source_id,platform,model,request_class,protocol,
 outcome,error_owner,customer_impact,observed_at
) VALUES($1,'active_probe','integration',$2,'openai','gpt-5.6-sol','text','http','success','provider',FALSE,$3)`, prefix, prefix, now.Add(-2*time.Second))
	require.NoError(t, err)
	var overrideID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO service_status_overrides(product_id,status,reason,starts_at,expires_at)
SELECT id,'maintenance',$1,$2,$3 FROM service_status_products WHERE code='openai-codex-api'
RETURNING id`, prefix, now.Add(-time.Minute), now.Add(time.Hour)).Scan(&overrideID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM service_status_overrides WHERE id=$1`, overrideID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM reliability_observations WHERE idempotency_key=$1`, prefix)
		_ = settings.Set(context.Background(), service.SettingKeyServiceStatusEnabled, "false")
		_ = settings.Set(context.Background(), service.SettingKeyReliabilityObservationEnabled, "false")
	})
	evidence := service.NewReliabilityEvidenceService(integrationDB, settings)
	require.NoError(t, evidence.Start(ctx))
	t.Cleanup(func() { _ = evidence.Stop(context.Background()) })
	statusControl := service.NewStatusControlService(integrationDB, settings, evidence)
	require.NoError(t, statusControl.Start(ctx))
	t.Cleanup(func() { _ = statusControl.Stop(context.Background()) })

	require.NoError(t, statusControl.Reconcile(ctx))
	var effective, reason string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT current.effective_status,current.effective_reason FROM service_status_current current
JOIN service_status_products product ON product.id=current.product_id
WHERE product.code='openai-codex-api'`).Scan(&effective, &reason))
	require.Equal(t, "maintenance", effective)
	require.Equal(t, "manual_override", reason)
	activeAdmin, err := statusControl.AdminSnapshot(ctx)
	require.NoError(t, err)
	activeProduct := findAdminStatusProduct(activeAdmin, "openai-codex-api")
	require.NotNil(t, activeProduct)
	require.NotEqual(t, activeProduct.ComputedStatus, activeProduct.EffectiveStatus)
	require.Equal(t, "manual_override", activeProduct.EffectiveReason)
	require.NotNil(t, activeProduct.LatestOverride)
	require.True(t, activeProduct.LatestOverride.Active)

	_, err = integrationDB.ExecContext(ctx, `UPDATE service_status_overrides SET expires_at=$2 WHERE id=$1`, overrideID, time.Now().Add(-time.Second))
	require.NoError(t, err)
	require.NoError(t, statusControl.Reconcile(ctx))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT current.effective_status,current.effective_reason FROM service_status_current current
JOIN service_status_products product ON product.id=current.product_id
WHERE product.code='openai-codex-api'`).Scan(&effective, &reason))
	require.NotEqual(t, "maintenance", effective)
	require.NotEqual(t, "manual_override", reason)
	admin, err := statusControl.AdminSnapshot(ctx)
	require.NoError(t, err)
	found := findAdminStatusProduct(admin, "openai-codex-api")
	require.NotNil(t, found)
	require.NotNil(t, found.LatestOverride)
	require.False(t, found.LatestOverride.Active)
	require.Equal(t, service.ServiceStatusMaintenance, found.LatestOverride.Status)
	require.Equal(t, found.ComputedStatus, found.EffectiveStatus)
}

func findAdminStatusProduct(snapshot *service.AdminStatusSnapshot, code string) *service.AdminStatusProduct {
	if snapshot == nil {
		return nil
	}
	for familyIndex := range snapshot.Families {
		for productIndex := range snapshot.Families[familyIndex].Products {
			if snapshot.Families[familyIndex].Products[productIndex].Code == code {
				return &snapshot.Families[familyIndex].Products[productIndex]
			}
		}
	}
	return nil
}

func TestReliabilityEvidenceSelectorQuerySupportsModelPatterns(t *testing.T) {
	now := time.Now()
	evidence := service.NewReliabilityEvidenceService(integrationDB, nil)
	snapshot, err := evidence.Snapshot(context.Background(), &service.ReliabilityEvidenceQuery{
		Start: now.Add(-5 * time.Minute), End: now, FactTypes: []service.ReliabilityFactType{service.ReliabilityFactCustomerRequest, service.ReliabilityFactActiveProbe},
		AnyPlatforms: []string{"openai"}, AnyModelPatterns: []string{"gpt-*"}, Limit: 10,
	})
	require.NoError(t, err)
	require.NotNil(t, snapshot)
}

func TestChannelMonitoringSnapshotRunsBoundedAggregateAgainstCatalog(t *testing.T) {
	statusControl := service.NewStatusControlService(integrationDB, NewSettingRepository(integrationEntClient), nil)
	snapshot, err := statusControl.MonitoringSnapshot(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, snapshot.Status)
	require.Equal(t, 15, snapshot.WindowMinutes)
	require.NotNil(t, snapshot.Evidence)
}
