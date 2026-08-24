package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

type CompensationControlService struct {
	db          *sql.DB
	settingRepo SettingRepository
	tiers       *CustomerTierService
	lifecycleMu sync.Mutex
	wg          sync.WaitGroup
	cancel      context.CancelFunc
}

func NewCompensationControlService(db *sql.DB, settingRepo SettingRepository, tiers *CustomerTierService) *CompensationControlService {
	return &CompensationControlService{db: db, settingRepo: settingRepo, tiers: tiers}
}
func (s *CompensationControlService) Name() string { return "compensation-shadow" }
func (s *CompensationControlService) enabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyCompensationShadowDraftEnabled)
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
func (s *CompensationControlService) Start(context.Context) error {
	if s == nil || !s.enabled(context.Background()) {
		return nil
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.cancel != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.GeneratePendingDrafts(ctx); err != nil {
		cancel()
		return err
	}
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.GeneratePendingDrafts(ctx); err != nil && ctx.Err() == nil {
					logger.LegacyPrintf("service.compensation_shadow", "draft generation failed: %v", err)
				}
			}
		}
	}()
	return nil
}
func (s *CompensationControlService) Stop(context.Context) error {
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

func (s *CompensationControlService) UpdateShadowEnabled(ctx context.Context, enabled bool, actorUserID int64) error {
	if s == nil || s.db == nil || s.settingRepo == nil {
		return fmt.Errorf("compensation shadow dependencies are unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-shadow-settings'))`); err != nil {
		return err
	}
	if enabled {
		policy, err := loadCompensationPolicy(ctx, tx, time.Now().UTC())
		if err != nil {
			return err
		}
		var active bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM compensation_shadow_periods WHERE ended_at IS NULL)`).Scan(&active); err != nil {
			return err
		}
		if !active {
			var actor any
			if actorUserID > 0 {
				actor = actorUserID
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO compensation_shadow_periods(activation_id,policy_version,started_at,created_by_user_id) VALUES($1,$2,NOW(),$3)`, uuid.New(), policy.Version, actor); err != nil {
				return err
			}
		}
	} else {
		if _, err = tx.ExecContext(ctx, `UPDATE compensation_shadow_periods SET ended_at=NOW() WHERE ended_at IS NULL`); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=EXCLUDED.updated_at`, SettingKeyCompensationShadowDraftEnabled, fmt.Sprintf("%t", enabled)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if enabled {
		if err := s.Start(ctx); err != nil {
			rollbackTx, beginErr := s.db.BeginTx(context.Background(), nil)
			if beginErr == nil {
				_, _ = rollbackTx.ExecContext(context.Background(), `UPDATE compensation_shadow_periods SET ended_at=NOW() WHERE ended_at IS NULL`)
				_, _ = rollbackTx.ExecContext(context.Background(), `UPDATE settings SET value='false',updated_at=NOW() WHERE key=$1`, SettingKeyCompensationShadowDraftEnabled)
				_ = rollbackTx.Commit()
			}
			return fmt.Errorf("compensation shadow preflight failed: %w", err)
		}
		return nil
	}
	return s.Stop(ctx)
}

func (s *CompensationControlService) GeneratePendingDrafts(ctx context.Context) error {
	if !s.enabled(ctx) {
		return nil
	}
	var periodID int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM compensation_shadow_periods WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`).Scan(&periodID); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT i.id FROM reliability_incidents i LEFT JOIN compensation_drafts d ON d.incident_id=i.id AND d.revision_number=1 LEFT JOIN compensation_draft_failures f ON f.incident_id=i.id AND f.shadow_period_id=$1 AND f.resolved_at IS NULL WHERE i.customer_impact_ended_at IS NOT NULL AND i.customer_impact_ended_at>=(SELECT started_at FROM compensation_shadow_periods WHERE id=$1) AND d.id IS NULL AND (f.incident_id IS NULL OR f.attempted_at<NOW()-INTERVAL '5 minutes') ORDER BY i.id LIMIT 50`, periodID)
	if err != nil {
		return err
	}
	var ids []int64
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
		if _, err := s.DraftIncident(ctx, id, 0); err != nil {
			_, recordErr := s.db.ExecContext(ctx, `INSERT INTO compensation_draft_failures(shadow_period_id,incident_id,error_message,attempted_at) VALUES($1,$2,$3,NOW()) ON CONFLICT(shadow_period_id,incident_id) DO UPDATE SET error_message=EXCLUDED.error_message,attempted_at=EXCLUDED.attempted_at,resolved_at=NULL`, periodID, id, truncateCustomerTierError(err))
			if recordErr != nil {
				return recordErr
			}
			continue
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE compensation_draft_failures SET resolved_at=NOW() WHERE shadow_period_id=$1 AND incident_id=$2 AND resolved_at IS NULL`, periodID, id)
	}
	return nil
}

var _ LifecycleComponent = (*CompensationControlService)(nil)
