package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

const insertReliabilityObservationSQL = `
INSERT INTO reliability_observations (
  idempotency_key, fact_type, source, source_id,
  request_id, client_request_id, user_id, group_id, access_group_id, account_id,
  platform, model, request_class, protocol, transport, endpoint_hash, route_fingerprint, routing_fingerprint,
  outcome, status_code, error_owner, exclusion_reason, customer_impact,
  latency_ms, observed_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25
)
ON CONFLICT (idempotency_key) DO NOTHING`

func (s *ReliabilityEvidenceService) batchInsertPostgres(ctx context.Context, inputs []*ReliabilityObservation) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("nil reliability evidence database")
	}
	if len(inputs) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, insertReliabilityObservationSQL)
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()
	var inserted int64
	for _, input := range inputs {
		if input == nil {
			continue
		}
		result, execErr := stmt.ExecContext(ctx,
			input.IdempotencyKey, string(input.FactType), input.Source, input.SourceID,
			input.RequestID, input.ClientRequestID, reliabilityNullableInt64(input.UserID), reliabilityNullableInt64(input.GroupID), input.AccessGroupID, reliabilityNullableInt64(input.AccountID),
			input.Platform, input.Model, input.RequestClass, input.Protocol, input.Transport, input.EndpointHash, input.RouteFingerprint, input.RoutingFingerprint,
			string(input.Outcome), reliabilityNullableInt(input.StatusCode), input.ErrorOwner, input.ExclusionReason, input.CustomerImpact,
			input.LatencyMs, input.ObservedAt,
		)
		if execErr != nil {
			return 0, execErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return 0, rowsErr
		}
		inserted += rows
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

func (s *ReliabilityEvidenceService) tryClaimProbePostgres(ctx context.Context, claim *ReliabilityProbeClaim) (bool, error) {
	if s == nil || s.db == nil {
		return false, fmt.Errorf("nil reliability evidence database")
	}
	var claimed bool
	err := s.db.QueryRowContext(ctx, `
WITH cleanup AS (
  DELETE FROM reliability_probe_claims
  WHERE expires_at < NOW() - INTERVAL '1 day'
), claimed AS (
  INSERT INTO reliability_probe_claims (
    claim_key, route_fingerprint, interval_start, expires_at
  ) VALUES ($1, $2, $3, $4)
  ON CONFLICT DO NOTHING
  RETURNING 1
)
SELECT EXISTS (SELECT 1 FROM claimed)
`, claim.ClaimIdentity, claim.RouteFingerprint, claim.IntervalStart, claim.ExpiresAt).Scan(&claimed)
	return claimed, err
}

func (s *ReliabilityEvidenceService) listPostgres(ctx context.Context, query *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("nil reliability evidence database")
	}
	where := []string{"observed_at >= $1", "observed_at < $2"}
	args := []any{query.Start, query.End}
	if len(query.FactTypes) > 0 {
		values := make([]string, 0, len(query.FactTypes))
		for _, factType := range query.FactTypes {
			values = append(values, string(factType))
		}
		args = append(args, pq.Array(values))
		where = append(where, fmt.Sprintf("fact_type = ANY($%d)", len(args)))
	}
	if query.GroupID != nil {
		args = append(args, *query.GroupID)
		where = append(where, fmt.Sprintf("group_id = $%d", len(args)))
	}
	if query.RouteFingerprint != "" {
		args = append(args, query.RouteFingerprint)
		where = append(where, fmt.Sprintf("route_fingerprint = $%d", len(args)))
	}
	if query.CustomerImpact != nil {
		args = append(args, *query.CustomerImpact)
		where = append(where, fmt.Sprintf("customer_impact = $%d", len(args)))
	}
	if len(query.Outcomes) > 0 {
		values := make([]string, 0, len(query.Outcomes))
		for _, outcome := range query.Outcomes {
			values = append(values, string(outcome))
		}
		args = append(args, pq.Array(values))
		where = append(where, fmt.Sprintf("outcome = ANY($%d)", len(args)))
	}
	if query.BeforeObservedAt != nil {
		args = append(args, *query.BeforeObservedAt)
		observedArg := len(args)
		args = append(args, query.BeforeID)
		where = append(where, fmt.Sprintf("(observed_at,id) < ($%d,$%d)", observedArg, len(args)))
	}
	selectors := make([]string, 0, 4)
	if len(query.AnyGroupIDs) > 0 {
		args = append(args, pq.Array(query.AnyGroupIDs))
		selectors = append(selectors, fmt.Sprintf("group_id = ANY($%d)", len(args)))
	}
	if len(query.AnyAccountIDs) > 0 {
		args = append(args, pq.Array(query.AnyAccountIDs))
		selectors = append(selectors, fmt.Sprintf("account_id = ANY($%d)", len(args)))
	}
	if len(query.AnyPlatforms) > 0 {
		args = append(args, pq.Array(query.AnyPlatforms))
		selectors = append(selectors, fmt.Sprintf("platform = ANY($%d)", len(args)))
	}
	if len(query.AnyRouteFingerprints) > 0 {
		args = append(args, pq.Array(query.AnyRouteFingerprints))
		selectors = append(selectors, fmt.Sprintf("route_fingerprint = ANY($%d)", len(args)))
	}
	if len(selectors) > 0 {
		where = append(where, "("+strings.Join(selectors, " OR ")+")")
	}
	if len(query.AnyModelPatterns) > 0 {
		patterns := make([]string, 0, len(query.AnyModelPatterns))
		for _, pattern := range query.AnyModelPatterns {
			patterns = append(patterns, reliabilityModelGlobToLike(pattern))
		}
		args = append(args, pq.Array(patterns))
		where = append(where, fmt.Sprintf("LOWER(model) LIKE ANY($%d)", len(args)))
	}
	exactScopes := make([]string, 0, 2)
	if scope := query.AttemptScope; scope != nil {
		args = append(args, scope.GroupID)
		groupArg := len(args)
		args = append(args, scope.AccessGroupID)
		accessArg := len(args)
		args = append(args, scope.Protocol)
		protocolArg := len(args)
		args = append(args, pq.Array(scope.Transports))
		transportArg := len(args)
		args = append(args, pq.Array(scope.RoutingFingerprints))
		fingerprintArg := len(args)
		exactScopes = append(exactScopes, fmt.Sprintf(
			"(fact_type = 'upstream_attempt' AND group_id = $%d AND access_group_id = $%d AND protocol = $%d AND transport = ANY($%d) AND routing_fingerprint = ANY($%d))",
			groupArg, accessArg, protocolArg, transportArg, fingerprintArg,
		))
	}
	if scope := query.ProbeScope; scope != nil {
		args = append(args, scope.Protocol)
		protocolArg := len(args)
		routes := make([]string, 0, len(scope.Routes))
		for _, route := range scope.Routes {
			args = append(args, route.AccountID)
			accountArg := len(args)
			args = append(args, route.EndpointHash)
			endpointArg := len(args)
			args = append(args, route.Transport)
			transportArg := len(args)
			routes = append(routes, fmt.Sprintf("(account_id = $%d AND endpoint_hash = $%d AND transport = $%d)", accountArg, endpointArg, transportArg))
		}
		exactScopes = append(exactScopes, fmt.Sprintf(
			"(fact_type = 'active_probe' AND group_id IS NULL AND protocol = $%d AND (%s))",
			protocolArg, strings.Join(routes, " OR "),
		))
	}
	if len(exactScopes) > 0 {
		where = append(where, "("+strings.Join(exactScopes, " OR ")+")")
	}
	args = append(args, query.Limit)
	rows, err := s.db.QueryContext(ctx, `
SELECT
  id, idempotency_key, fact_type, source, source_id,
  request_id, client_request_id, user_id, group_id, access_group_id, account_id,
  platform, model, request_class, protocol, transport, endpoint_hash, route_fingerprint, routing_fingerprint,
  outcome, status_code, error_owner, exclusion_reason, customer_impact,
  latency_ms, observed_at
FROM reliability_observations
WHERE `+strings.Join(where, " AND ")+`
ORDER BY observed_at DESC, id DESC
LIMIT $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	observations := make([]*ReliabilityObservation, 0, query.Limit)
	for rows.Next() {
		item := &ReliabilityObservation{}
		var factType, outcome string
		var userID, groupID, accountID, statusCode sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.IdempotencyKey, &factType, &item.Source, &item.SourceID,
			&item.RequestID, &item.ClientRequestID, &userID, &groupID, &item.AccessGroupID, &accountID,
			&item.Platform, &item.Model, &item.RequestClass, &item.Protocol, &item.Transport, &item.EndpointHash, &item.RouteFingerprint, &item.RoutingFingerprint,
			&outcome, &statusCode, &item.ErrorOwner, &item.ExclusionReason, &item.CustomerImpact,
			&item.LatencyMs, &item.ObservedAt,
		); err != nil {
			return nil, err
		}
		item.FactType = ReliabilityFactType(factType)
		item.Outcome = ReliabilityOutcome(outcome)
		item.UserID = reliabilityNullInt64Pointer(userID)
		item.GroupID = reliabilityNullInt64Pointer(groupID)
		item.AccountID = reliabilityNullInt64Pointer(accountID)
		if statusCode.Valid {
			value := int(statusCode.Int64)
			item.StatusCode = &value
		}
		observations = append(observations, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return observations, nil
}

func reliabilityModelGlobToLike(pattern string) string {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	result := make([]rune, 0, len(pattern))
	for _, char := range pattern {
		switch char {
		case '\\', '%', '_':
			result = append(result, '\\', char)
		case '*':
			result = append(result, '%')
		case '?':
			result = append(result, '_')
		default:
			result = append(result, char)
		}
	}
	return string(result)
}

func reliabilityNullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func reliabilityNullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func reliabilityNullInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}
