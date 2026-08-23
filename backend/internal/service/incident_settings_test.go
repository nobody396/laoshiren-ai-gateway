package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type incidentSettingsStub struct{ values map[string]string }

func (s *incidentSettingsStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func TestIncidentSettingsAndAuditRollbackAtomicallyWhenAuditWriteFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &incidentSettingsStub{values: map[string]string{}}
	incidents := NewIncidentControlService(db, repo, nil, nil)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO settings(key,value,updated_at)")).WithArgs(SettingKeyReliabilityIncidentsEnabled, "true").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO settings(key,value,updated_at)")).WithArgs(SettingKeyReliabilityIncidentsPublicEnabled, "true").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO reliability_incident_settings_audit")).WillReturnError(errors.New("audit unavailable"))
	mock.ExpectRollback()

	err = incidents.persistIncidentSettings(context.Background(), IncidentControlSettings{}, IncidentControlSettings{Enabled: true, PublicEnabled: true}, 42)

	require.ErrorContains(t, err, "audit unavailable")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIncidentLifecycleConcurrentStopSerializesRestart(t *testing.T) {
	repo := &incidentSettingsStub{values: map[string]string{SettingKeyServiceStatusEnabled: "true", SettingKeyReliabilityIncidentsEnabled: "true"}}
	status := NewStatusControlService(nil, repo, nil)
	incidents := NewIncidentControlService(nil, repo, status, nil)
	workerCtx, cancel := context.WithCancel(context.Background())
	incidents.cancel = cancel
	incidents.wg.Add(1)
	cancelObserved := make(chan struct{})
	go func() {
		defer incidents.wg.Done()
		<-workerCtx.Done()
		close(cancelObserved)
		time.Sleep(20 * time.Millisecond)
	}()
	stopDone := make(chan error, 1)
	startDone := make(chan error, 1)
	go func() { stopDone <- incidents.Stop(context.Background()) }()
	<-cancelObserved
	go func() { startDone <- incidents.Start(context.Background()) }()
	require.NoError(t, <-stopDone)
	require.Error(t, <-startDone)
}
func (s *incidentSettingsStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (s *incidentSettingsStub) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}
func (s *incidentSettingsStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := map[string]string{}
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}
func (s *incidentSettingsStub) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		s.values[key] = value
	}
	return nil
}
func (s *incidentSettingsStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, nil
}
func (s *incidentSettingsStub) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func TestIncidentSettingsPreflightFailureRollsBackEnabledAndPublicFlags(t *testing.T) {
	repo := &incidentSettingsStub{values: map[string]string{SettingKeyServiceStatusEnabled: "true", SettingKeyServiceStatusPublicEnabled: "true", SettingKeyReliabilityIncidentsEnabled: "false", SettingKeyReliabilityIncidentsPublicEnabled: "false"}}
	status := NewStatusControlService(nil, repo, nil)
	incidents := NewIncidentControlService(nil, repo, status, nil)

	err := incidents.UpdateSettings(context.Background(), IncidentControlSettings{Enabled: true, PublicEnabled: true})

	require.ErrorContains(t, err, "preflight")
	require.Equal(t, "false", repo.values[SettingKeyReliabilityIncidentsEnabled])
	require.Equal(t, "false", repo.values[SettingKeyReliabilityIncidentsPublicEnabled])
}

func TestIncidentSettingsExistingPublicStateIsRestoredWhenPreflightFails(t *testing.T) {
	repo := &incidentSettingsStub{values: map[string]string{SettingKeyServiceStatusEnabled: "true", SettingKeyServiceStatusPublicEnabled: "true", SettingKeyReliabilityIncidentsEnabled: "true", SettingKeyReliabilityIncidentsPublicEnabled: "true"}}
	status := NewStatusControlService(nil, repo, nil)
	incidents := NewIncidentControlService(nil, repo, status, nil)
	_, cancel := context.WithCancel(context.Background())
	incidents.cancel = cancel
	t.Cleanup(cancel)

	err := incidents.UpdateSettings(context.Background(), IncidentControlSettings{Enabled: true, PublicEnabled: true})

	require.ErrorContains(t, err, "preflight")
	require.Equal(t, "true", repo.values[SettingKeyReliabilityIncidentsEnabled])
	require.Equal(t, "true", repo.values[SettingKeyReliabilityIncidentsPublicEnabled])
}

func TestIncidentSettingsExistingPublicStateIsRestoredWhenReplicaWasNotRunning(t *testing.T) {
	repo := &incidentSettingsStub{values: map[string]string{SettingKeyServiceStatusEnabled: "true", SettingKeyServiceStatusPublicEnabled: "true", SettingKeyReliabilityIncidentsEnabled: "true", SettingKeyReliabilityIncidentsPublicEnabled: "true"}}
	status := NewStatusControlService(nil, repo, nil)
	incidents := NewIncidentControlService(nil, repo, status, nil)

	err := incidents.UpdateSettings(context.Background(), IncidentControlSettings{Enabled: true, PublicEnabled: true})

	require.ErrorContains(t, err, "preflight")
	require.Equal(t, "true", repo.values[SettingKeyReliabilityIncidentsEnabled])
	require.Equal(t, "true", repo.values[SettingKeyReliabilityIncidentsPublicEnabled])
}
