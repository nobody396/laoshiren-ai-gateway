package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
)

const (
	SettingKeyReliabilityIncidentsEnabled       = "reliability_incidents_enabled"
	SettingKeyReliabilityIncidentsPublicEnabled = "reliability_incidents_public_enabled"
	incidentReconcileInterval                   = 30 * time.Second
	incidentCandidateJoinWindow                 = 15 * time.Minute
	incidentEvidenceLookback                    = 30 * time.Minute
)

type IncidentControlService struct {
	db              *sql.DB
	settingRepo     SettingRepository
	status          *StatusControlService
	evidence        *ReliabilityEvidenceService
	tierSnapshotter CustomerTierSnapshotter
	lifecycleMu     sync.Mutex
	settingsMu      sync.Mutex
	reconcileMu     sync.Mutex
	wg              sync.WaitGroup
	cancel          context.CancelFunc
}

type CustomerTierSnapshotter interface {
	EnsureIncidentSnapshots(context.Context, int64) error
	CustomerTierSnapshotsEnabled(context.Context) bool
}

func (s *IncidentControlService) SetCustomerTierSnapshotter(snapshotter CustomerTierSnapshotter) {
	if s != nil {
		s.tierSnapshotter = snapshotter
	}
}

func NewIncidentControlService(db *sql.DB, settingRepo SettingRepository, status *StatusControlService, evidence *ReliabilityEvidenceService) *IncidentControlService {
	return &IncidentControlService{db: db, settingRepo: settingRepo, status: status, evidence: evidence}
}

func (s *IncidentControlService) Name() string { return "incident-control" }

func (s *IncidentControlService) Start(context.Context) error {
	if s == nil || !s.settingEnabled(context.Background(), SettingKeyReliabilityIncidentsEnabled) {
		return nil
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.cancel != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.ReconcileAt(ctx, time.Now().UTC()); err != nil {
		cancel()
		return err
	}
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(incidentReconcileInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if err := s.ReconcileAt(ctx, now.UTC()); err != nil && ctx.Err() == nil {
					logger.LegacyPrintf("service.incident_control", "incident reconciliation failed: %v", err)
				}
			}
		}
	}()
	return nil
}

func (s *IncidentControlService) Stop(context.Context) error {
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

func (s *IncidentControlService) Settings(ctx context.Context) IncidentControlSettings {
	return IncidentControlSettings{Enabled: s.settingEnabled(ctx, SettingKeyReliabilityIncidentsEnabled), PublicEnabled: s.settingEnabled(ctx, SettingKeyReliabilityIncidentsPublicEnabled)}
}

func (s *IncidentControlService) UpdateSettings(ctx context.Context, settings IncidentControlSettings, actorUserID ...int64) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("incident settings repository is unavailable")
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	release, err := s.lockSettingsMutation(ctx)
	if err != nil {
		return err
	}
	defer release()
	if settings.PublicEnabled && !settings.Enabled {
		return fmt.Errorf("public incident timeline requires incident reconciliation enabled")
	}
	if settings.Enabled && (s.status == nil || !s.status.settingEnabled(ctx, SettingKeyServiceStatusEnabled)) {
		return fmt.Errorf("incident reconciliation requires computed Service Status enabled")
	}
	if settings.PublicEnabled && !s.status.settingEnabled(ctx, SettingKeyServiceStatusPublicEnabled) {
		return fmt.Errorf("public incident timeline requires public Service Status enabled")
	}
	before := s.Settings(ctx)
	actor := int64(0)
	if len(actorUserID) > 0 {
		actor = actorUserID[0]
	}
	if !settings.Enabled {
		if err := s.persistIncidentSettings(ctx, before, IncidentControlSettings{}, actor); err != nil {
			return err
		}
		return s.Stop(ctx)
	}
	wasEnabled := s.settingEnabled(ctx, SettingKeyReliabilityIncidentsEnabled)
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{SettingKeyReliabilityIncidentsEnabled: "true", SettingKeyReliabilityIncidentsPublicEnabled: "false"}); err != nil {
		return err
	}
	if !wasEnabled || !s.runtimeRunning() {
		if err := s.Start(ctx); err != nil {
			_ = s.settingRepo.SetMultiple(ctx, incidentSettingsValues(before))
			return fmt.Errorf("incident preflight reconciliation failed: %w", err)
		}
	} else if err := s.ReconcileAt(ctx, time.Now().UTC()); err != nil {
		_ = s.settingRepo.SetMultiple(ctx, incidentSettingsValues(before))
		return fmt.Errorf("incident preflight reconciliation failed: %w", err)
	}
	if err := s.persistIncidentSettings(ctx, before, settings, actor); err != nil {
		_ = s.settingRepo.SetMultiple(ctx, incidentSettingsValues(before))
		if !before.Enabled {
			_ = s.Stop(ctx)
		}
		return err
	}
	return nil
}

func (s *IncidentControlService) runtimeRunning() bool {
	if s == nil {
		return false
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.cancel != nil
}

func (s *IncidentControlService) lockSettingsMutation(ctx context.Context) (func(), error) {
	if s.db == nil {
		return func() {}, nil
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext('reliability-incident-settings'))`); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtext('reliability-incident-settings'))`)
		_ = conn.Close()
	}, nil
}

func incidentSettingsValues(settings IncidentControlSettings) map[string]string {
	return map[string]string{SettingKeyReliabilityIncidentsEnabled: fmt.Sprintf("%t", settings.Enabled), SettingKeyReliabilityIncidentsPublicEnabled: fmt.Sprintf("%t", settings.PublicEnabled)}
}

func (s *IncidentControlService) persistIncidentSettings(ctx context.Context, before, after IncidentControlSettings, actor int64) error {
	if s.db == nil {
		return s.settingRepo.SetMultiple(ctx, incidentSettingsValues(after))
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	values := incidentSettingsValues(after)
	for _, key := range []string{SettingKeyReliabilityIncidentsEnabled, SettingKeyReliabilityIncidentsPublicEnabled} {
		value := values[key]
		if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, key, value); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reliability_incident_settings_audit(actor_user_id,before_state,after_state) VALUES(NULLIF($1,0),$2,$3)`, actor, beforeJSON, afterJSON); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *IncidentControlService) settingEnabled(ctx context.Context, key string) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, key)
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

func nullTimePointerUTC(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func nullInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

var _ LifecycleComponent = (*IncidentControlService)(nil)
