package service

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type statusSettingRepoStub struct{ monthlyStatusSettingRepoStub }

func (s *statusSettingRepoStub) SetMultiple(ctx context.Context, values map[string]string) error {
	for key, value := range values {
		if err := s.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func TestPublicStatusSnapshotIsEmptyWhenDefaultOff(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	settings := &statusSettingRepoStub{monthlyStatusSettingRepoStub{values: map[string]string{}}}
	svc := NewStatusControlService(db, settings, nil)

	snapshot, err := svc.PublicSnapshot(context.Background())

	require.NoError(t, err)
	require.False(t, snapshot.Enabled)
	require.Empty(t, snapshot.Families)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPublicStatusSnapshotContainsOnlySanitizedCatalogProjection(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT")).WillReturnRows(sqlmock.NewRows([]string{
		"family_code", "family_name", "product_code", "product_name", "critical", "status", "reason", "evidence_at", "computed_at",
		"component_code", "component_name", "model_pattern", "access_mode", "component_status", "component_reason", "component_evidence_at", "component_computed_at",
	}).AddRow("builder-pass", "Builder Pass", "builder-pass-gpt", "Builder Pass GPT", true, "operational", "fresh_success", now, now,
		"builder-pass-gpt-http", "Builder Pass GPT · HTTP", "", "http", "operational", "fresh_success", now, now))
	settings := &monthlyStatusSettingRepoStub{values: map[string]string{
		SettingKeyServiceStatusEnabled: "true", SettingKeyServiceStatusPublicEnabled: "true",
	}}
	svc := NewStatusControlService(db, settings, nil)

	snapshot, err := svc.PublicSnapshot(context.Background())

	require.NoError(t, err)
	require.True(t, snapshot.Enabled)
	require.Len(t, snapshot.Families, 1)
	require.Equal(t, "builder-pass-gpt", snapshot.Families[0].Products[0].Code)
	require.Equal(t, "http", snapshot.Families[0].Products[0].Components[0].AccessMode)
	require.Equal(t, ServiceStatusOperational, snapshot.Families[0].Products[0].Components[0].Status)
	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "model_pattern")
	require.NotContains(t, string(payload), "critical")
	require.NotContains(t, string(payload), "group_id")
	require.NotContains(t, string(payload), "account")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateStatusOverrideIsTimeBoundedAndAudited(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectExec("INSERT INTO service_status_overrides").
		WithArgs("openai-codex-api", "maintenance", "planned work", sqlmock.AnyArg(), sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	svc := NewStatusControlService(db, nil, nil)

	err = svc.CreateOverride(context.Background(), StatusOverrideCommand{
		ProductCode: "openai-codex-api", Status: ServiceStatusMaintenance,
		Reason: "planned work", Duration: time.Hour, CreatedByUserID: 7,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateStatusOverrideRejectsUnboundedDuration(t *testing.T) {
	svc := NewStatusControlService(nil, nil, nil)
	err := svc.CreateOverride(context.Background(), StatusOverrideCommand{
		ProductCode: "openai-codex-api", Status: ServiceStatusMaintenance,
		Reason: "planned work", Duration: 25 * time.Hour,
	})
	require.ErrorContains(t, err, "24 hours")
}

func TestUpdateStatusSettingsRefusesPublicWithoutEvaluator(t *testing.T) {
	settings := &statusSettingRepoStub{monthlyStatusSettingRepoStub{values: map[string]string{}}}
	svc := NewStatusControlService(nil, settings, nil)
	defer func() { _ = svc.Stop(context.Background()) }()
	err := svc.UpdateSettings(context.Background(), StatusSettings{Enabled: false, PublicEnabled: true})
	require.ErrorContains(t, err, "requires")
	require.Empty(t, settings.updates)

	err = svc.UpdateSettings(context.Background(), StatusSettings{Enabled: true, PublicEnabled: true})
	require.ErrorContains(t, err, "preflight reconciliation failed")
	require.Equal(t, "true", settings.values[SettingKeyServiceStatusEnabled])
	require.Equal(t, "false", settings.values[SettingKeyServiceStatusPublicEnabled])
}
