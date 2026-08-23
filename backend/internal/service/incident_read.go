package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

func (s *IncidentControlService) AdminSnapshot(ctx context.Context) (*AdminIncidentSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("incident control is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	candidates, err := s.loadAdminCandidates(ctx)
	if err != nil {
		return nil, err
	}
	incidents, err := s.loadAdminIncidents(ctx, 50)
	if err != nil {
		return nil, err
	}
	return &AdminIncidentSnapshot{Settings: s.Settings(ctx), Candidates: candidates, Incidents: incidents, GeneratedAt: time.Now().UTC()}, nil
}

func (s *IncidentControlService) GetAdminIncident(ctx context.Context, incidentID int64) (*AdminIncident, error) {
	if incidentID <= 0 {
		return nil, sql.ErrNoRows
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	incidents, err := s.loadAdminIncidentBases(ctx, 1, &incidentID)
	if err != nil {
		return nil, err
	}
	if len(incidents) != 1 {
		return nil, sql.ErrNoRows
	}
	if err := s.hydrateAdminIncidents(ctx, incidents); err != nil {
		return nil, err
	}
	return &incidents[0], nil
}

func (s *IncidentControlService) loadAdminCandidates(ctx context.Context) ([]AdminIncidentCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id,c.state,c.first_observed_at,c.last_observed_at,c.confirmed_incident_id,c.dismissed_reason,
 p.code,p.display_name,cp.latest_status,cp.first_observed_at,cp.last_observed_at,cp.first_customer_failure_at
FROM reliability_incident_candidates c
LEFT JOIN reliability_incident_candidate_products cp ON cp.candidate_id=c.id
LEFT JOIN service_status_products p ON p.id=cp.product_id
ORDER BY c.created_at DESC,c.id DESC,p.sort_order,p.id LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []AdminIncidentCandidate{}
	index := map[int64]int{}
	for rows.Next() {
		var id int64
		var state string
		var first, last time.Time
		var confirmed sql.NullInt64
		var dismissed string
		var code, name, status sql.NullString
		var productFirst, productLast, failure sql.NullTime
		if err := rows.Scan(&id, &state, &first, &last, &confirmed, &dismissed, &code, &name, &status, &productFirst, &productLast, &failure); err != nil {
			return nil, err
		}
		position, exists := index[id]
		if !exists {
			position = len(result)
			index[id] = position
			item := AdminIncidentCandidate{ID: id, State: IncidentCandidateState(state), FirstObservedAt: first.UTC(), LastObservedAt: last.UTC(), DismissedReason: dismissed, Products: []AdminIncidentCandidateProduct{}}
			if confirmed.Valid {
				value := confirmed.Int64
				item.ConfirmedIncidentID = &value
			}
			result = append(result, item)
		}
		if code.Valid {
			product := AdminIncidentCandidateProduct{ProductCode: code.String, ProductName: name.String, LatestStatus: ServiceStatus(status.String), FirstObservedAt: productFirst.Time.UTC(), LastObservedAt: productLast.Time.UTC()}
			if failure.Valid {
				value := failure.Time.UTC()
				product.FirstCustomerFailureAt = &value
			}
			result[position].Products = append(result[position].Products, product)
		}
	}
	return result, rows.Err()
}

func (s *IncidentControlService) loadAdminIncidents(ctx context.Context, limit int) ([]AdminIncident, error) {
	incidents, err := s.loadAdminIncidentBases(ctx, limit, nil)
	if err != nil {
		return nil, err
	}
	if err := s.hydrateAdminIncidents(ctx, incidents); err != nil {
		return nil, err
	}
	return incidents, nil
}

func (s *IncidentControlService) loadAdminIncidentBases(ctx context.Context, limit int, incidentID *int64) ([]AdminIncident, error) {
	query := `
SELECT id,public_id,phase,title,internal_summary,observation_started_at,observation_ended_at,
 customer_impact_started_at,customer_impact_ended_at,monitoring_since,resolved_at,version,evidence_gap,created_at,updated_at
FROM reliability_incidents`
	args := []any{}
	if incidentID != nil {
		query += ` WHERE id=$1`
		args = append(args, *incidentID)
	} else {
		if limit <= 0 || limit > 100 {
			limit = 50
		}
		query += ` ORDER BY created_at DESC,id DESC LIMIT $1`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []AdminIncident{}
	for rows.Next() {
		var item AdminIncident
		var phase string
		var observationEnd, impactStart, impactEnd, monitoring, resolved sql.NullTime
		if err := rows.Scan(&item.ID, &item.PublicID, &phase, &item.Title, &item.InternalSummary, &item.ObservationStartedAt, &observationEnd, &impactStart, &impactEnd, &monitoring, &resolved, &item.Version, &item.EvidenceGap, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Phase = IncidentPhase(phase)
		item.ObservationEndedAt = nullTimePointerUTC(observationEnd)
		item.CustomerImpactStartedAt = nullTimePointerUTC(impactStart)
		item.CustomerImpactEndedAt = nullTimePointerUTC(impactEnd)
		item.MonitoringSince = nullTimePointerUTC(monitoring)
		item.ResolvedAt = nullTimePointerUTC(resolved)
		item.Products = []AdminIncidentProduct{}
		item.Updates = []AdminIncidentUpdate{}
		item.PublicTimeline = []AdminIncidentPublicUpdate{}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *IncidentControlService) hydrateAdminIncidents(ctx context.Context, incidents []AdminIncident) error {
	if len(incidents) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(incidents))
	index := make(map[int64]int, len(incidents))
	for position := range incidents {
		ids = append(ids, incidents[position].ID)
		index[incidents[position].ID] = position
	}
	products, err := s.loadAdminIncidentProductsBatch(ctx, ids)
	if err != nil {
		return err
	}
	updates, err := s.loadAdminIncidentUpdatesBatch(ctx, ids)
	if err != nil {
		return err
	}
	publicUpdates, err := s.loadAdminIncidentPublicUpdatesBatch(ctx, ids)
	if err != nil {
		return err
	}
	for incidentID, values := range products {
		incidents[index[incidentID]].Products = values
	}
	for incidentID, values := range updates {
		incidents[index[incidentID]].Updates = values
	}
	for incidentID, values := range publicUpdates {
		incidents[index[incidentID]].PublicTimeline = values
	}
	return nil
}

func (s *IncidentControlService) loadAdminIncidentProductsBatch(ctx context.Context, incidentIDs []int64) (map[int64][]AdminIncidentProduct, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT ip.incident_id,ip.id,p.code,p.display_name,ip.current_status,ip.affected_at,ip.monitoring_since,ip.recovered_at,ip.last_customer_failure_at,
 seg.id,seg.started_at,seg.ended_at,seg.start_observation_id,seg.end_observation_id
FROM reliability_incident_products ip JOIN service_status_products p ON p.id=ip.product_id
LEFT JOIN reliability_incident_impact_segments seg ON seg.incident_product_id=ip.id
WHERE ip.incident_id=ANY($1) ORDER BY ip.incident_id,p.sort_order,p.id,seg.started_at,seg.id`, pq.Array(incidentIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := map[int64][]AdminIncidentProduct{}
	productIndex := map[int64]map[int64]int{}
	for rows.Next() {
		var incidentID, productRowID int64
		var code, name, status string
		var affected time.Time
		var monitoring, recovered, lastFailure sql.NullTime
		var segmentID, startObservation, endObservation sql.NullInt64
		var started, ended sql.NullTime
		if err := rows.Scan(&incidentID, &productRowID, &code, &name, &status, &affected, &monitoring, &recovered, &lastFailure, &segmentID, &started, &ended, &startObservation, &endObservation); err != nil {
			return nil, err
		}
		if productIndex[incidentID] == nil {
			productIndex[incidentID] = map[int64]int{}
		}
		position, exists := productIndex[incidentID][productRowID]
		if !exists {
			position = len(result[incidentID])
			productIndex[incidentID][productRowID] = position
			result[incidentID] = append(result[incidentID], AdminIncidentProduct{ID: productRowID, ProductCode: code, ProductName: name, CurrentStatus: ServiceStatus(status), AffectedAt: affected.UTC(), MonitoringSince: nullTimePointerUTC(monitoring), RecoveredAt: nullTimePointerUTC(recovered), LastCustomerFailureAt: nullTimePointerUTC(lastFailure), Segments: []AdminIncidentSegment{}})
		}
		if segmentID.Valid {
			segment := AdminIncidentSegment{ID: segmentID.Int64, StartedAt: started.Time.UTC(), EndedAt: nullTimePointerUTC(ended), StartObservationID: startObservation.Int64, EndObservationID: endObservation.Int64}
			if segment.EndedAt != nil {
				segment.DurationSeconds = int64(segment.EndedAt.Sub(segment.StartedAt).Seconds())
				result[incidentID][position].CompensableSeconds += segment.DurationSeconds
			}
			result[incidentID][position].Segments = append(result[incidentID][position].Segments, segment)
		}
	}
	return result, rows.Err()
}

func (s *IncidentControlService) loadAdminIncidentUpdatesBatch(ctx context.Context, incidentIDs []int64) (map[int64][]AdminIncidentUpdate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT incident_id,id,phase,kind,internal_message,created_by_user_id,created_at FROM reliability_incident_updates WHERE incident_id=ANY($1) ORDER BY incident_id,created_at,id`, pq.Array(incidentIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := map[int64][]AdminIncidentUpdate{}
	for rows.Next() {
		var incidentID int64
		var item AdminIncidentUpdate
		var phase string
		var actor sql.NullInt64
		if err := rows.Scan(&incidentID, &item.ID, &phase, &item.Kind, &item.InternalMessage, &actor, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Phase = IncidentPhase(phase)
		item.CreatedByUserID = nullInt64Pointer(actor)
		result[incidentID] = append(result[incidentID], item)
	}
	return result, rows.Err()
}

func (s *IncidentControlService) loadAdminIncidentPublicUpdatesBatch(ctx context.Context, incidentIDs []int64) (map[int64][]AdminIncidentPublicUpdate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT incident_id,id,phase,message,published_by_user_id,published_at FROM reliability_incident_public_timeline WHERE incident_id=ANY($1) ORDER BY incident_id,published_at,id`, pq.Array(incidentIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := map[int64][]AdminIncidentPublicUpdate{}
	for rows.Next() {
		var incidentID int64
		var item AdminIncidentPublicUpdate
		var phase string
		var actor sql.NullInt64
		if err := rows.Scan(&incidentID, &item.ID, &phase, &item.Message, &actor, &item.PublishedAt); err != nil {
			return nil, err
		}
		item.Phase = IncidentPhase(phase)
		item.PublishedByUserID = nullInt64Pointer(actor)
		result[incidentID] = append(result[incidentID], item)
	}
	return result, rows.Err()
}

func (s *IncidentControlService) PublicSnapshot(ctx context.Context) (*PublicIncidentSnapshot, error) {
	enabled := s != nil && s.settingEnabled(ctx, SettingKeyReliabilityIncidentsEnabled) && s.settingEnabled(ctx, SettingKeyReliabilityIncidentsPublicEnabled) && s.status.settingEnabled(ctx, SettingKeyServiceStatusPublicEnabled)
	result := &PublicIncidentSnapshot{Enabled: enabled, GeneratedAt: time.Now().UTC(), Incidents: []PublicIncident{}}
	if !enabled {
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
SELECT i.id,i.public_id,i.phase,i.observation_started_at,i.resolved_at
FROM reliability_incidents i
WHERE (i.phase<>'resolved' OR i.resolved_at >= NOW()-INTERVAL '30 days')
  AND EXISTS (SELECT 1 FROM reliability_incident_public_timeline t WHERE t.incident_id=i.id)
ORDER BY i.observation_started_at DESC,i.id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	index := map[int64]int{}
	for rows.Next() {
		var incidentID int64
		var item PublicIncident
		var phase string
		var resolved sql.NullTime
		if err := rows.Scan(&incidentID, &item.ID, &phase, &item.StartedAt, &resolved); err != nil {
			_ = rows.Close()
			return nil, err
		}
		item.Phase = IncidentPhase(phase)
		item.ResolvedAt = nullTimePointerUTC(resolved)
		item.AffectedProducts = []string{}
		item.Timeline = []PublicIncidentUpdate{}
		index[incidentID] = len(result.Incidents)
		ids = append(ids, incidentID)
		result.Incidents = append(result.Incidents, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return result, nil
	}
	productRows, err := s.db.QueryContext(ctx, `SELECT DISTINCT ip.incident_id,p.display_name,p.sort_order,p.id FROM reliability_incident_products ip JOIN service_status_products p ON p.id=ip.product_id AND p.public=TRUE AND p.enabled=TRUE WHERE ip.incident_id=ANY($1) ORDER BY ip.incident_id,p.sort_order,p.id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	for productRows.Next() {
		var incidentID, sortOrder, productID int64
		var name string
		if err := productRows.Scan(&incidentID, &name, &sortOrder, &productID); err != nil {
			_ = productRows.Close()
			return nil, err
		}
		result.Incidents[index[incidentID]].AffectedProducts = append(result.Incidents[index[incidentID]].AffectedProducts, name)
	}
	if err := productRows.Close(); err != nil {
		return nil, err
	}
	updateRows, err := s.db.QueryContext(ctx, `WITH ranked AS (SELECT incident_id,phase,message,published_at,id,ROW_NUMBER() OVER(PARTITION BY incident_id ORDER BY published_at DESC,id DESC) AS rn FROM reliability_incident_public_timeline WHERE incident_id=ANY($1)) SELECT incident_id,phase,message,published_at FROM ranked WHERE rn<=100 ORDER BY incident_id,published_at,id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	for updateRows.Next() {
		var incidentID int64
		var phase, message string
		var published time.Time
		if err := updateRows.Scan(&incidentID, &phase, &message, &published); err != nil {
			_ = updateRows.Close()
			return nil, err
		}
		result.Incidents[index[incidentID]].Timeline = append(result.Incidents[index[incidentID]].Timeline, PublicIncidentUpdate{Phase: IncidentPhase(phase), Message: message, PublishedAt: published.UTC()})
	}
	if err := updateRows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}
