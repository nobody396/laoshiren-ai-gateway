package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	SettingKeyServiceStatusEnabled       = "service_status_enabled"
	SettingKeyServiceStatusPublicEnabled = "service_status_public_enabled"
)

type StatusAvailabilityMetrics struct {
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Rate      *float64 `json:"rate,omitempty"`
}

func newStatusAvailability(total, succeeded, failed int) StatusAvailabilityMetrics {
	result := StatusAvailabilityMetrics{Total: total, Succeeded: succeeded, Failed: failed}
	if total > 0 {
		rate := float64(succeeded) / float64(total)
		result.Rate = &rate
	}
	return result
}

type StatusCatalogComponent struct {
	Code         string        `json:"code"`
	DisplayName  string        `json:"display_name"`
	ModelPattern string        `json:"-"`
	AccessMode   string        `json:"access_mode"`
	Status       ServiceStatus `json:"status"`
	Reason       string        `json:"reason"`
	EvidenceAt   *time.Time    `json:"evidence_at,omitempty"`
	ComputedAt   time.Time     `json:"computed_at"`
}

type StatusCatalogProduct struct {
	Code        string                   `json:"code"`
	DisplayName string                   `json:"display_name"`
	Critical    bool                     `json:"-"`
	Status      ServiceStatus            `json:"status"`
	Reason      string                   `json:"reason"`
	EvidenceAt  *time.Time               `json:"evidence_at,omitempty"`
	ComputedAt  time.Time                `json:"computed_at"`
	Components  []StatusCatalogComponent `json:"components"`
}

type StatusCatalogFamily struct {
	Code        string                 `json:"code"`
	DisplayName string                 `json:"display_name"`
	Products    []StatusCatalogProduct `json:"products"`
}

type PublicStatusSnapshot struct {
	Enabled     bool                  `json:"enabled"`
	GeneratedAt time.Time             `json:"generated_at"`
	Families    []StatusCatalogFamily `json:"families"`
}

type AdminStatusOverride struct {
	ID              int64         `json:"id"`
	Status          ServiceStatus `json:"status"`
	Reason          string        `json:"reason"`
	StartsAt        time.Time     `json:"starts_at"`
	ExpiresAt       time.Time     `json:"expires_at"`
	CreatedByUserID *int64        `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	Active          bool          `json:"active"`
}

type AdminStatusBinding struct {
	BindingKey       string `json:"binding_key"`
	GroupID          *int64 `json:"group_id,omitempty"`
	GroupName        string `json:"group_name,omitempty"`
	Platform         string `json:"platform,omitempty"`
	ModelPattern     string `json:"model_pattern,omitempty"`
	RouteFingerprint string `json:"route_fingerprint,omitempty"`
}

type AdminStatusComponent struct {
	Code                 string                    `json:"code"`
	DisplayName          string                    `json:"display_name"`
	ModelPattern         string                    `json:"model_pattern,omitempty"`
	AccessMode           string                    `json:"access_mode"`
	ComputedStatus       ServiceStatus             `json:"computed_status"`
	Reason               string                    `json:"reason"`
	EvidenceAt           *time.Time                `json:"evidence_at,omitempty"`
	ComputedAt           time.Time                 `json:"computed_at"`
	CustomerAvailability StatusAvailabilityMetrics `json:"customer_availability"`
	ProbeAvailability    StatusAvailabilityMetrics `json:"probe_availability"`
	Bindings             []AdminStatusBinding      `json:"bindings"`
}

type AdminStatusProduct struct {
	Code                 string                    `json:"code"`
	DisplayName          string                    `json:"display_name"`
	Critical             bool                      `json:"critical"`
	ComputedStatus       ServiceStatus             `json:"computed_status"`
	EffectiveStatus      ServiceStatus             `json:"effective_status"`
	ComputedReason       string                    `json:"computed_reason"`
	EffectiveReason      string                    `json:"effective_reason"`
	EvidenceAt           *time.Time                `json:"evidence_at,omitempty"`
	ComputedAt           time.Time                 `json:"computed_at"`
	CustomerAvailability StatusAvailabilityMetrics `json:"customer_availability"`
	ProbeAvailability    StatusAvailabilityMetrics `json:"probe_availability"`
	LatestOverride       *AdminStatusOverride      `json:"latest_override,omitempty"`
	Components           []AdminStatusComponent    `json:"components"`
}

type AdminStatusFamily struct {
	Code        string               `json:"code"`
	DisplayName string               `json:"display_name"`
	Products    []AdminStatusProduct `json:"products"`
}

type AdminStatusSnapshot struct {
	Enabled                 bool                            `json:"enabled"`
	PublicEnabled           bool                            `json:"public_enabled"`
	GeneratedAt             time.Time                       `json:"generated_at"`
	LastSuccessfulReconcile *time.Time                      `json:"last_successful_reconcile,omitempty"`
	EvaluationReady         bool                            `json:"evaluation_ready"`
	Completeness            ReliabilityEvidenceCompleteness `json:"completeness"`
	Families                []AdminStatusFamily             `json:"families"`
}

type StatusSettings struct {
	Enabled       bool `json:"enabled"`
	PublicEnabled bool `json:"public_enabled"`
}

type StatusControlService struct {
	db                      *sql.DB
	settingRepo             SettingRepository
	evidence                *ReliabilityEvidenceService
	mu                      sync.Mutex
	lifecycleMu             sync.Mutex
	reconcileMu             sync.Mutex
	wg                      sync.WaitGroup
	cancel                  context.CancelFunc
	readinessBaseline       ReliabilityEvidenceCompleteness
	lastSuccessfulReconcile time.Time
	lastReconcileReady      bool
	incompleteUntil         time.Time
}

func NewStatusControlService(db *sql.DB, settingRepo SettingRepository, evidence *ReliabilityEvidenceService) *StatusControlService {
	return &StatusControlService{db: db, settingRepo: settingRepo, evidence: evidence}
}

func (s *StatusControlService) PublicSnapshot(ctx context.Context) (*PublicStatusSnapshot, error) {
	enabled := s.settingEnabled(ctx, SettingKeyServiceStatusEnabled) && s.settingEnabled(ctx, SettingKeyServiceStatusPublicEnabled)
	result := &PublicStatusSnapshot{Enabled: enabled, GeneratedAt: time.Now(), Families: []StatusCatalogFamily{}}
	if !enabled {
		return result, nil
	}
	families, err := s.loadPublicCatalog(ctx)
	if err != nil {
		return nil, err
	}
	result.Families = families
	return result, nil
}

func (s *StatusControlService) AdminSnapshot(ctx context.Context) (*AdminStatusSnapshot, error) {
	families, err := s.loadAdminCatalog(ctx)
	if err != nil {
		return nil, err
	}
	result := &AdminStatusSnapshot{Enabled: s.settingEnabled(ctx, SettingKeyServiceStatusEnabled), PublicEnabled: s.settingEnabled(ctx, SettingKeyServiceStatusPublicEnabled), GeneratedAt: time.Now(), Families: families}
	if s.evidence != nil {
		result.Completeness = s.evidence.Completeness()
	}
	s.mu.Lock()
	if !s.lastSuccessfulReconcile.IsZero() {
		value := s.lastSuccessfulReconcile
		result.LastSuccessfulReconcile = &value
	}
	result.EvaluationReady = s.lastReconcileReady
	s.mu.Unlock()
	return result, nil
}

func (s *StatusControlService) Settings(ctx context.Context) StatusSettings {
	return StatusSettings{Enabled: s.settingEnabled(ctx, SettingKeyServiceStatusEnabled), PublicEnabled: s.settingEnabled(ctx, SettingKeyServiceStatusPublicEnabled)}
}

func (s *StatusControlService) UpdateSettings(ctx context.Context, settings StatusSettings) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("status settings repository is unavailable")
	}
	if settings.PublicEnabled && !settings.Enabled {
		return fmt.Errorf("public status requires status evaluation enabled")
	}
	if !settings.Enabled {
		if err := s.settingRepo.SetMultiple(ctx, map[string]string{SettingKeyServiceStatusEnabled: "false", SettingKeyServiceStatusPublicEnabled: "false"}); err != nil {
			return err
		}
		return s.Stop(ctx)
	}
	// Never publish a newly enabled evaluator before one synchronous, complete
	// reconciliation. A failed preflight leaves public visibility disabled.
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{SettingKeyServiceStatusEnabled: "true", SettingKeyServiceStatusPublicEnabled: "false"}); err != nil {
		return err
	}
	if err := s.Start(ctx); err != nil {
		_ = s.settingRepo.Set(ctx, SettingKeyServiceStatusPublicEnabled, "false")
		return fmt.Errorf("status preflight reconciliation failed: %w", err)
	}
	if settings.PublicEnabled {
		if err := s.Reconcile(ctx); err != nil {
			_ = s.settingRepo.Set(ctx, SettingKeyServiceStatusPublicEnabled, "false")
			return fmt.Errorf("status preflight reconciliation failed: %w", err)
		}
	}
	s.mu.Lock()
	preflightReady := s.lastReconcileReady
	s.mu.Unlock()
	if settings.PublicEnabled && !preflightReady {
		_ = s.settingRepo.Set(ctx, SettingKeyServiceStatusPublicEnabled, "false")
		return fmt.Errorf("status preflight reconciliation failed: reliability evidence is incomplete")
	}
	return s.settingRepo.Set(ctx, SettingKeyServiceStatusPublicEnabled, fmt.Sprintf("%t", settings.PublicEnabled))
}

type StatusOverrideCommand struct {
	ProductCode     string
	Status          ServiceStatus
	Reason          string
	Duration        time.Duration
	CreatedByUserID int64
}

func (s *StatusControlService) CreateOverride(ctx context.Context, command StatusOverrideCommand) error {
	command.ProductCode = strings.TrimSpace(command.ProductCode)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ProductCode == "" || command.Reason == "" || len(command.Reason) > 500 {
		return fmt.Errorf("status override product and reason are required")
	}
	switch command.Status {
	case ServiceStatusOperational, ServiceStatusDegradedPerformance, ServiceStatusPartialOutage, ServiceStatusMajorOutage, ServiceStatusMaintenance, ServiceStatusMonitoring:
	default:
		return fmt.Errorf("invalid status override")
	}
	if command.Duration <= 0 || command.Duration > 24*time.Hour {
		return fmt.Errorf("status override duration must be within 24 hours")
	}
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
INSERT INTO service_status_overrides(product_id,status,reason,starts_at,expires_at,created_by_user_id)
SELECT id,$2,$3,$4,$5,NULLIF($6,0) FROM service_status_products WHERE code=$1`,
		command.ProductCode, string(command.Status), command.Reason, now, now.Add(command.Duration), command.CreatedByUserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("status product not found")
	}
	return nil
}

func (s *StatusControlService) settingEnabled(ctx context.Context, key string) bool {
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

func (s *StatusControlService) loadPublicCatalog(ctx context.Context) ([]StatusCatalogFamily, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("status catalog database is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT f.code,f.display_name,p.code,p.display_name,p.critical,
 COALESCE(active_override.status,pc.computed_status,'monitoring'),
 CASE WHEN active_override.id IS NOT NULL THEN 'manual_override' ELSE COALESCE(pc.computed_reason,'no_evidence') END,
 pc.evidence_at,COALESCE(pc.computed_at,NOW()),
 c.code,c.display_name,c.model_pattern,c.access_mode,
 COALESCE(cc.computed_status,'monitoring'),COALESCE(cc.reason,'no_evidence'),cc.evidence_at,COALESCE(cc.computed_at,NOW())
FROM service_status_families f
JOIN service_status_products p ON p.family_id=f.id AND p.enabled=TRUE AND p.public=TRUE
JOIN service_status_components c ON c.product_id=p.id AND c.enabled=TRUE
LEFT JOIN service_status_current pc ON pc.product_id=p.id
LEFT JOIN service_status_component_current cc ON cc.component_id=c.id
LEFT JOIN LATERAL (
 SELECT o.id,o.status FROM service_status_overrides o
 WHERE o.product_id=p.id AND o.starts_at<=NOW() AND o.expires_at>NOW()
 ORDER BY o.created_at DESC,o.id DESC LIMIT 1
) active_override ON TRUE
WHERE f.enabled=TRUE
ORDER BY f.sort_order,f.id,p.sort_order,p.id,c.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	families := make([]StatusCatalogFamily, 0)
	familyIndex := map[string]int{}
	productIndex := map[string]int{}
	for rows.Next() {
		var familyCode, familyName, productCode, productName, productStatus, productReason string
		var componentStatus, componentReason string
		var critical bool
		var productEvidence, componentEvidence sql.NullTime
		var productComputed, componentComputed time.Time
		component := StatusCatalogComponent{}
		if err := rows.Scan(&familyCode, &familyName, &productCode, &productName, &critical, &productStatus, &productReason, &productEvidence, &productComputed,
			&component.Code, &component.DisplayName, &component.ModelPattern, &component.AccessMode, &componentStatus, &componentReason, &componentEvidence, &componentComputed); err != nil {
			return nil, err
		}
		component.Status, component.Reason, component.ComputedAt = ServiceStatus(componentStatus), componentReason, componentComputed
		if componentEvidence.Valid {
			value := componentEvidence.Time
			component.EvidenceAt = &value
		}
		fi, ok := familyIndex[familyCode]
		if !ok {
			fi = len(families)
			familyIndex[familyCode] = fi
			families = append(families, StatusCatalogFamily{Code: familyCode, DisplayName: familyName, Products: []StatusCatalogProduct{}})
		}
		lookup := familyCode + ":" + productCode
		pi, ok := productIndex[lookup]
		if !ok {
			pi = len(families[fi].Products)
			productIndex[lookup] = pi
			product := StatusCatalogProduct{Code: productCode, DisplayName: productName, Critical: critical, Status: ServiceStatus(productStatus), Reason: productReason, ComputedAt: productComputed, Components: []StatusCatalogComponent{}}
			if productEvidence.Valid {
				value := productEvidence.Time
				product.EvidenceAt = &value
			}
			families[fi].Products = append(families[fi].Products, product)
		}
		families[fi].Products[pi].Components = append(families[fi].Products[pi].Components, component)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}
	return families, rows.Err()
}

func (s *StatusControlService) loadAdminCatalog(ctx context.Context) ([]AdminStatusFamily, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("status catalog database is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT f.code,f.display_name,p.code,p.display_name,p.critical,
 COALESCE(pc.computed_status,'monitoring'),COALESCE(active_override.status,pc.computed_status,'monitoring'),
 COALESCE(pc.computed_reason,'no_evidence'),
 CASE WHEN active_override.id IS NOT NULL THEN 'manual_override' ELSE COALESCE(pc.computed_reason,'no_evidence') END,
 pc.evidence_at,COALESCE(pc.computed_at,NOW()),
 COALESCE(pc.customer_request_count,0),COALESCE(pc.customer_success_count,0),COALESCE(pc.customer_failure_count,0),
 COALESCE(pc.probe_count,0),COALESCE(pc.probe_success_count,0),COALESCE(pc.probe_failure_count,0),
 c.code,c.display_name,c.model_pattern,c.access_mode,COALESCE(cc.computed_status,'monitoring'),COALESCE(cc.reason,'no_evidence'),cc.evidence_at,COALESCE(cc.computed_at,NOW()),
 COALESCE(cc.customer_request_count,0),COALESCE(cc.customer_success_count,0),COALESCE(cc.customer_failure_count,0),
 COALESCE(cc.probe_count,0),COALESCE(cc.probe_success_count,0),COALESCE(cc.probe_failure_count,0),
 b.binding_key,b.group_id,b.group_name,b.platform,b.model_pattern,b.route_fingerprint,
 latest.id,latest.status,latest.reason,latest.starts_at,latest.expires_at,latest.created_by_user_id,latest.created_at
FROM service_status_families f
JOIN service_status_products p ON p.family_id=f.id AND p.enabled=TRUE
JOIN service_status_components c ON c.product_id=p.id AND c.enabled=TRUE
LEFT JOIN service_status_current pc ON pc.product_id=p.id
LEFT JOIN service_status_component_current cc ON cc.component_id=c.id
LEFT JOIN service_status_bindings b ON b.component_id=c.id AND b.enabled=TRUE
LEFT JOIN LATERAL (
 SELECT o.id,o.status FROM service_status_overrides o
 WHERE o.product_id=p.id AND o.starts_at<=NOW() AND o.expires_at>NOW()
 ORDER BY o.created_at DESC,o.id DESC LIMIT 1
) active_override ON TRUE
LEFT JOIN LATERAL (
 SELECT o.id,o.status,o.reason,o.starts_at,o.expires_at,o.created_by_user_id,o.created_at
 FROM service_status_overrides o WHERE o.product_id=p.id ORDER BY o.created_at DESC,o.id DESC LIMIT 1
) latest ON TRUE
WHERE f.enabled=TRUE
ORDER BY f.sort_order,f.id,p.sort_order,p.id,c.id,b.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	now := time.Now()
	families := []AdminStatusFamily{}
	familyIndex := map[string]int{}
	productIndex := map[string][2]int{}
	componentIndex := map[string][3]int{}
	for rows.Next() {
		var familyCode, familyName, productCode, productName, computedStatus, effectiveStatus, computedReason, effectiveReason string
		var critical bool
		var productEvidence, componentEvidence sql.NullTime
		var productComputed, componentComputed time.Time
		var prTotal, prSuccess, prFailure, ppTotal, ppSuccess, ppFailure int
		var crTotal, crSuccess, crFailure, cpTotal, cpSuccess, cpFailure int
		var componentCode, componentName, componentPattern, accessMode, componentStatus, componentReason string
		var bindingKey, groupName, platform, bindingPattern, routeFingerprint sql.NullString
		var bindingGroupID sql.NullInt64
		var overrideID, overrideActor sql.NullInt64
		var overrideStatus, overrideReason sql.NullString
		var overrideStarts, overrideExpires, overrideCreated sql.NullTime
		if err := rows.Scan(&familyCode, &familyName, &productCode, &productName, &critical, &computedStatus, &effectiveStatus, &computedReason, &effectiveReason, &productEvidence, &productComputed,
			&prTotal, &prSuccess, &prFailure, &ppTotal, &ppSuccess, &ppFailure,
			&componentCode, &componentName, &componentPattern, &accessMode, &componentStatus, &componentReason, &componentEvidence, &componentComputed,
			&crTotal, &crSuccess, &crFailure, &cpTotal, &cpSuccess, &cpFailure,
			&bindingKey, &bindingGroupID, &groupName, &platform, &bindingPattern, &routeFingerprint,
			&overrideID, &overrideStatus, &overrideReason, &overrideStarts, &overrideExpires, &overrideActor, &overrideCreated); err != nil {
			return nil, err
		}
		fi, ok := familyIndex[familyCode]
		if !ok {
			fi = len(families)
			familyIndex[familyCode] = fi
			families = append(families, AdminStatusFamily{Code: familyCode, DisplayName: familyName, Products: []AdminStatusProduct{}})
		}
		productKey := familyCode + ":" + productCode
		pl, ok := productIndex[productKey]
		if !ok {
			pi := len(families[fi].Products)
			pl = [2]int{fi, pi}
			productIndex[productKey] = pl
			product := AdminStatusProduct{Code: productCode, DisplayName: productName, Critical: critical, ComputedStatus: ServiceStatus(computedStatus), EffectiveStatus: ServiceStatus(effectiveStatus), ComputedReason: computedReason, EffectiveReason: effectiveReason, ComputedAt: productComputed,
				CustomerAvailability: newStatusAvailability(prTotal, prSuccess, prFailure), ProbeAvailability: newStatusAvailability(ppTotal, ppSuccess, ppFailure), Components: []AdminStatusComponent{}}
			if productEvidence.Valid {
				value := productEvidence.Time
				product.EvidenceAt = &value
			}
			if overrideID.Valid {
				o := &AdminStatusOverride{ID: overrideID.Int64, Status: ServiceStatus(overrideStatus.String), Reason: overrideReason.String, StartsAt: overrideStarts.Time, ExpiresAt: overrideExpires.Time, CreatedAt: overrideCreated.Time, Active: !now.Before(overrideStarts.Time) && now.Before(overrideExpires.Time)}
				if overrideActor.Valid {
					value := overrideActor.Int64
					o.CreatedByUserID = &value
				}
				product.LatestOverride = o
			}
			families[fi].Products = append(families[fi].Products, product)
		}
		componentKey := productKey + ":" + componentCode
		cl, ok := componentIndex[componentKey]
		if !ok {
			ci := len(families[pl[0]].Products[pl[1]].Components)
			cl = [3]int{pl[0], pl[1], ci}
			componentIndex[componentKey] = cl
			component := AdminStatusComponent{Code: componentCode, DisplayName: componentName, ModelPattern: componentPattern, AccessMode: accessMode, ComputedStatus: ServiceStatus(componentStatus), Reason: componentReason, ComputedAt: componentComputed,
				CustomerAvailability: newStatusAvailability(crTotal, crSuccess, crFailure), ProbeAvailability: newStatusAvailability(cpTotal, cpSuccess, cpFailure), Bindings: []AdminStatusBinding{}}
			if componentEvidence.Valid {
				value := componentEvidence.Time
				component.EvidenceAt = &value
			}
			families[cl[0]].Products[cl[1]].Components = append(families[cl[0]].Products[cl[1]].Components, component)
		}
		if bindingKey.Valid {
			binding := AdminStatusBinding{BindingKey: bindingKey.String, GroupName: groupName.String, Platform: platform.String, ModelPattern: bindingPattern.String, RouteFingerprint: routeFingerprint.String}
			if bindingGroupID.Valid {
				value := bindingGroupID.Int64
				binding.GroupID = &value
			}
			families[cl[0]].Products[cl[1]].Components[cl[2]].Bindings = append(families[cl[0]].Products[cl[1]].Components[cl[2]].Bindings, binding)
		}
	}
	if err := rows.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}
	return families, rows.Err()
}
